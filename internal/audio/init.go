package audio

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"
	"unsafe"

	"replay/internal/buffer"
	"replay/internal/compressor"
	"replay/internal/queue"

	"github.com/gordonklaus/portaudio"
	"go.uber.org/zap"
)

type AudioSegment struct {
	Start int64
	End   int64
}

type AudioClient struct {
	bitrate    int
	channels   int
	sampleRate int

	readSize int
	duration int

	inpRBuf     *buffer.RingBuffer
	inpQueue    *queue.Queue
	inpCmpr     *compressor.Compressor
	inpDevice   *portaudio.DeviceInfo
	isRecording bool

	outRBuf   *buffer.RingBuffer
	outCmpr   *compressor.Compressor
	outDevice *portaudio.DeviceInfo
	isPlaying bool

	log *zap.Logger
}

func Init(log *zap.Logger) (*AudioClient, error) {
	const op = "audio.Init"

	const bufSize = 1920 * 4
	const queueSize = 1920 * 4
	const readSize = 1920 * 2
	const cmprSize = 1920

	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("%s: portaudio init: %w", op, err)
	}

	acl := &AudioClient{
		bitrate:    48000,
		channels:   2,
		sampleRate: 48000,

		readSize: readSize,
		duration: 20,

		inpRBuf:   buffer.NewRB(bufSize),
		inpQueue:  queue.New(queueSize),
		inpDevice: nil,

		outRBuf:   buffer.NewRB(bufSize),
		outDevice: nil,

		log: log,
	}

	if err := acl.initDevices(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	acl.autoRouteMonitor()

	encInpBuf := make([]byte, cmprSize)
	encOutBuf := make([]byte, cmprSize)

	decInpBuf := make([]int16, cmprSize)
	decOutBuf := make([]int16, cmprSize)

	inpCmr, err := compressor.Init(acl.bitrate, acl.channels, acl.sampleRate, acl.duration, encInpBuf, decInpBuf, log)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	outCmr, err := compressor.Init(acl.bitrate, acl.channels, acl.sampleRate, acl.duration, encOutBuf, decOutBuf, log)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	acl.inpCmpr = inpCmr
	acl.outCmpr = outCmr

	return acl, nil
}

func (a *AudioClient) initDevices() error {
	const op = "audio.initDevices"

	var err error
	maxAttempts := 10
	for i := range maxAttempts {
		if a.inpDevice == nil {
			a.inpDevice, err = a.initInputDevice()
			if err != nil {
				a.log.Error("Input Initialize error",
					zap.Any("attempt", i),
					zap.Error(err))
				a.inpDevice = nil
				continue
			}
		}
		if a.outDevice == nil {
			a.outDevice, err = a.initOutputDevice()
			if err != nil {
				a.log.Error("Output Initialize error",
					zap.Any("attempt", i),
					zap.Error(err))
				a.outDevice = nil
				continue
			}
		}
	}

	return nil
}

func (a *AudioClient) initInputDevice() (*portaudio.DeviceInfo, error) {
	const op = "audio.initInputDevice"

	defer func() {
		if r := recover(); r != nil {
			a.log.Error("Input Initialize error",
				zap.Any("recover", r))
		}
	}()

	return portaudio.DefaultInputDevice()
}

func (a *AudioClient) initOutputDevice() (*portaudio.DeviceInfo, error) {
	const op = "audio.initOutputDevice"
	defer func() {
		if r := recover(); r != nil {
			a.log.Error("Output Initialize error",
				zap.Any("recover", r))
		}
	}()

	return portaudio.DefaultOutputDevice()
}

func (acl *AudioClient) autoRouteMonitor() error {
	const op = "audio.autoRouteMonitor"

	out, _ := exec.Command("pactl", "get-default-sink").Output()
	trimSpaceBytes(&out)

	monitorNameBytes := append(out, []byte(".monitor")...)
	monitorName := unsafe.String(unsafe.SliceData(monitorNameBytes), len(monitorNameBytes))

	time.Sleep(200 * time.Millisecond)

	out, err := exec.Command("pactl", "list", "short", "source-outputs").Output()
	if err != nil {
		return err
	}

	rangeByByte(out, '\n', func(start, end int) {
		line := out[start:end]
		spaceIdx := bytes.IndexByte(line, ' ')
		if spaceIdx == -1 {
			return
		}
		streamIDBytes := line[:spaceIdx]
		streamID := unsafe.String(unsafe.SliceData(streamIDBytes), len(streamIDBytes))

		exec.Command("pactl", "move-source-output", streamID, monitorName).Run()
	})

	return nil
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func trimSpaceBytes(b *[]byte) {
	tempB := *b

	start := 0
	end := len(tempB) - 1
	for start < end && isSpace(tempB[start]) {
		start++
	}
	for end > start && isSpace(tempB[end]) {
		end--
	}

	*b = tempB[start : end+1]
}

func rangeByByte(b []byte, sep byte, yield func(start, end int)) {
	start, end, sepIdx := 0, len(b)-1, 0

	for start < end {
		sepIdx = bytes.IndexByte(b[start:], sep)
		if sepIdx == -1 {
			break
		}
		start += sepIdx + 1 // jump to separator and skip it

		sepIdx = bytes.IndexByte(b[start:], sep)
		if sepIdx == -1 {
			break
		}
		end = start + sepIdx

		if start == end {
			break
		}

		yield(start, end)
	}
}
