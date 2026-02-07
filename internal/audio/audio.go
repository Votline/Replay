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
	"replay/internal/queue"

	"github.com/gordonklaus/portaudio"
)

const bufSize uint64 = 8192

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
}

func Init() (*AudioClient, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("Init portaudio: %w", err)
	}

	acl := &AudioClient{
		bitrate:    64000,
		channels:   2,
		sampleRate: 44100.0,
		duration:   20,

		inpBuf:   buffer.NewRB(bufSize),
		inpQueue: queue.NewQueue(bufSize),
		outBuf:   buffer.NewRB(bufSize),

		inputDevice:  nil,
		outputDevice: nil,
	}

	errs := make([]string, 0, 20)
	maxAttempts := 0
	for (acl.inputDevice == nil || acl.outputDevice == nil) && maxAttempts <= 10 {
		input, err := initInputDevice()
		if err == nil {
			acl.inputDevice = input
		} else {
			errs = append(errs, err.Error()+"\n")
		}

		output, err := initOutputDevice()
		if err == nil {
			acl.outputDevice = output
		} else {
			errs = append(errs, err.Error()+"\n")
		}

		maxAttempts++
		time.Sleep(100 * time.Millisecond)
	}

	if acl.inputDevice == nil || acl.outputDevice == nil {
		return nil, fmt.Errorf("Failed to find input or output device. Errors:\n%v", errs)
	}

	return acl, nil
}

func initInputDevice() (*portaudio.DeviceInfo, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("initInputDevice recovered", r)
		}
	}()

	return portaudio.DefaultInputDevice()
}

func initOutputDevice() (*portaudio.DeviceInfo, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("initOutputDevice recovered", r)
		}
	}()

	return portaudio.DefaultOutputDevice()
}

func (acl *AudioClient) Record(w io.Writer) error {
	const op = "audio.Record"

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
		})
	if err != nil {
		return fmt.Errorf("%s: error during record: %w", op, err)
	}

	if err := stream.Start(); err != nil {
		return fmt.Errorf("%s: start stream: %w", op, err)
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		localBuf := make([]float32, bufSize)
		for {
			n := acl.inpBuf.Read(localBuf)
			if n > 0 {
				acl.inpQueue.Push(localBuf[:n])
			} else {
				time.Sleep(time.Millisecond)
			}
		}
	})

	localBuf := make([]float32, bufSize)
	for acl.isRecording {
		n := acl.inpQueue.Pop(localBuf)
		if n > 0 {
			if err := binary.Write(w, binary.LittleEndian, localBuf[:n]); err != nil {
				fmt.Printf("%v\n", err.Error())
			}
		} else {
			time.Sleep(time.Millisecond)
		}
	}

	return nil
}

func (acl *AudioClient) Replay(r io.Reader) error {
	const op = "audio.Replay"
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

	var wg sync.WaitGroup
	wg.Go(func() {
		defer stream.Stop()
		defer stream.Close()

		localBuf := make([]float32, bufSize)
		for acl.isPlaying {
			err := binary.Read(r, binary.LittleEndian, localBuf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				fmt.Printf("%v\n", err.Error())
				return
			}

			acl.outBuf.Write(localBuf)
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
