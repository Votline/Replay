package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"replay/internal/buffer"
	"replay/internal/compressor"
	"replay/internal/queue"

	"github.com/gordonklaus/portaudio"
	"go.uber.org/zap"
)

const bufSize = 8192

type AudioClient struct {
	bitrate     int
	channels    int
	sampleRate  float64
	isPlaying   bool
	isRecording bool
	duration    time.Duration

	inpBuf   *buffer.RingBuffer
	inpQueue *queue.Queue
	outBuf   *buffer.RingBuffer

	inputDevice  *portaudio.DeviceInfo
	outputDevice *portaudio.DeviceInfo

	inpCmpr *compressor.Compressor
	outCmpr *compressor.Compressor

	log *zap.Logger
}

type AudioSegment struct {
	Start int64
	End   int64
}

func Init(log *zap.Logger) (*AudioClient, error) {
	const op = "audio.Init"

	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("Init portaudio: %w", err)
	}

	acl := &AudioClient{
		bitrate:    48000,
		channels:   2,
		sampleRate: 48000.0,
		duration:   20,

		inpBuf:   buffer.NewRB(bufSize),
		inpQueue: queue.NewQueue(bufSize),
		outBuf:   buffer.NewRB(bufSize),

		inputDevice:  nil,
		outputDevice: nil,

		log: log,
	}

	errs := make([]string, 0, 20)
	maxAttempts := 0
	for (acl.inputDevice == nil || acl.outputDevice == nil) && maxAttempts <= 10 {
		input, err := acl.initInputDevice()
		if err == nil {
			acl.inputDevice = input
		} else {
			errs = append(errs, err.Error()+"\n")
		}

		output, err := acl.initOutputDevice()
		if err == nil {
			acl.outputDevice = output
		} else {
			errs = append(errs, err.Error()+"\n")
		}

		maxAttempts++
		time.Sleep(100 * time.Millisecond)
	}

	if acl.inputDevice == nil || acl.outputDevice == nil {
		return nil, fmt.Errorf("%s: Failed to find input or output device. Errors:\n%v", op, errs)
	}

	inpCmpr, err := compressor.NewCompressor(acl.channels, acl.bitrate, int(acl.sampleRate), int(acl.duration), log)
	if err != nil {
		return nil, fmt.Errorf("%s: Failed to create compressor: %w", op, err)
	}
	outCmpr, err := compressor.NewCompressor(acl.channels, acl.bitrate, int(acl.sampleRate), int(acl.duration), log)
	if err != nil {
		return nil, fmt.Errorf("%s: Failed to create compressor: %w", op, err)
	}

	acl.inpCmpr = inpCmpr
	acl.outCmpr = outCmpr

	return acl, nil
}

func (acl *AudioClient) initInputDevice() (*portaudio.DeviceInfo, error) {
	defer func() {
		if r := recover(); r != nil {
			acl.log.Error("initInputDevice recovered", zap.Any("recover", r))
		}
	}()

	return portaudio.DefaultInputDevice()
}

func (acl *AudioClient) initOutputDevice() (*portaudio.DeviceInfo, error) {
	defer func() {
		if r := recover(); r != nil {
			acl.log.Error("initOutputDevice recovered", zap.Any("recover", r))
		}
	}()

	return portaudio.DefaultOutputDevice()
}

func (acl *AudioClient) Record(w io.Writer) error {
	const op = "audio.Record"

	acl.inpBuf.Reset()
	acl.inpQueue.Reset()

	acl.isRecording = true

	go acl.AutoRouteToMonitor()

	samplePerMs := int(acl.sampleRate*float64(acl.duration)/1000) * acl.channels

	stream, err := portaudio.OpenStream(
		portaudio.StreamParameters{
			Input: portaudio.StreamDeviceParameters{
				Device:   acl.inputDevice,
				Channels: acl.channels,
			},
			SampleRate:      acl.sampleRate,
			FramesPerBuffer: samplePerMs,
		},
		func(in []float32) {
			acl.inpBuf.Write(in)
			acl.log.Info("Recorded", zap.Int("len", len(in)))
		})
	if err != nil {
		return fmt.Errorf("%s: error during record: %w", op, err)
	}

	if err := stream.Start(); err != nil {
		return fmt.Errorf("%s: start stream: %w", op, err)
	}

	defer stream.Stop()
	defer stream.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		localBuf := make([]float32, bufSize)
		for acl.isRecording {
			n := acl.inpBuf.Read(localBuf)
			acl.log.Info("Read", zap.Int("len", n))
			if n > 0 {
				acl.inpQueue.Push(localBuf[:n])
			} else {
				time.Sleep(time.Millisecond)
			}
		}
	})

	localBuf := make([]float32, samplePerMs)
	for acl.isRecording {
		n := acl.inpQueue.Pop(localBuf)
		acl.log.Info("Popped", zap.Int("len", n))
		if n > 0 {
			compressed, err := acl.inpCmpr.Compress(localBuf[:n])
			if err != nil {
				acl.log.Error("Compress error. Drop chunk", zap.Error(err))
				continue
			}
			acl.log.Info("Compressed", zap.Int("len", len(compressed)))

			size := uint32(len(compressed))
			if err := binary.Write(w, binary.LittleEndian, size); err != nil {
				acl.log.Error("Write size error", zap.Error(err))
				continue
			}

			if _, err := w.Write(compressed); err != nil {
				acl.log.Error("Write compressed error", zap.Error(err))
				continue
			}
		} else {
			time.Sleep(time.Millisecond)
		}
	}

	return nil
}

func (acl *AudioClient) Replay(r io.Reader) error {
	const op = "audio.Replay"

	acl.outBuf.Reset()
	acl.isPlaying = true

	samplePerMs := int(acl.sampleRate*float64(acl.duration)/1000) * acl.channels

	stream, err := portaudio.OpenStream(
		portaudio.StreamParameters{
			Output: portaudio.StreamDeviceParameters{
				Device:   acl.outputDevice,
				Channels: acl.channels,
			},
			SampleRate:      acl.sampleRate,
			FramesPerBuffer: samplePerMs,
		},
		func(in, out []float32) {
			n := acl.outBuf.Read(out)
			for i := n; i < len(out); i++ {
				out[i] = 0
			}
		})
	if err != nil {
		return fmt.Errorf("%s: error during record: %w", op, err)
	}
	if err := stream.Start(); err != nil {
		return fmt.Errorf("%s: start stream: %w", op, err)
	}

	defer stream.Stop()
	defer stream.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		for acl.isPlaying {
			var size uint32

			err := binary.Read(r, binary.LittleEndian, &size)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				acl.log.Info("End of file")
				break
			}
			if err != nil {
				acl.log.Error("Read error", zap.Error(err))
				return
			}

			packet := make([]byte, size)
			_, err = io.ReadFull(r, packet)
			if err != nil {
				acl.log.Error("Read packet error", zap.Error(err))
				return
			}

			pcm, err := acl.outCmpr.Decompress(packet, bufSize)
			if err != nil {
				acl.log.Error("Decompress error. Drop chunk", zap.Error(err))
				continue
			}

			acl.outBuf.Write(pcm)
		}

		time.Sleep(acl.duration * time.Millisecond)
	})

	wg.Wait()

	return nil
}

func (acl *AudioClient) AutoRouteToMonitor() error {
	out, _ := exec.Command("pactl", "get-default-sink").Output()
	monitorName := strings.TrimSpace(string(out)) + ".monitor"

	time.Sleep(200 * time.Millisecond)

	out, err := exec.Command("pactl", "list", "short", "source-outputs").Output()
	if err != nil {
		return err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			streamID := fields[0]
			exec.Command("pactl", "move-source-output", streamID, monitorName).Run()
		}
	}
	return nil
}

func (acl *AudioClient) StopReplay() {
	acl.isPlaying = false
}

func (acl *AudioClient) IsPlaying() bool {
	return acl.isPlaying
}

func (acl *AudioClient) StopRecording() {
	acl.isRecording = false
}

func (acl *AudioClient) IsRecording() bool {
	return acl.isRecording
}
