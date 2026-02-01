package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"replay/internal/buffer"

	"github.com/gordonklaus/portaudio"
)

const bufSize uint64 = 512

type AudioClient struct {
	bitrate    int
	channels   int
	sampleRate float64
	duration   time.Duration

	inpBuf *buffer.RingBuffer
	outBuf *buffer.RingBuffer

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
		duration: 20,

		inpBuf: buffer.NewRB(bufSize),
		outBuf: buffer.NewRB(bufSize),

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
			errs = append(errs, err.Error() + "\n")
		}

		output, err := initOutputDevice()
		if err == nil {
			acl.outputDevice = output
		} else {
			errs = append(errs, err.Error() + "\n")
		}

		maxAttempts++
		time.Sleep(100*time.Millisecond)
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
	return portaudio.DefaultInputDevice()
}

func (acl *AudioClient) Record(w io.Writer) error {
	const op = "audio.Record"

	samplePerMs := int(acl.sampleRate * float64(acl.duration) / 1000 ) * acl.channels

	stream, err := portaudio.OpenStream(
		portaudio.StreamParameters{
			Input: portaudio.StreamDeviceParameters{
				Device: acl.inputDevice,
				Channels: acl.channels,
			},
			Output: portaudio.StreamDeviceParameters{
				Channels: 0,
			},
			SampleRate: acl.sampleRate,
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

	transferBuf := buffer.NewRB(bufSize)
	go func(){
		localBuf := make([]float32, bufSize)
		ticker := time.NewTicker(acl.duration * time.Millisecond)
		for range ticker.C {
			n := transferBuf.Read(localBuf)
			binary.Write(w, binary.LittleEndian, localBuf[:n])
		}
	}()

	localBuf := make([]float32, bufSize)
	ticker := time.NewTicker(acl.duration * time.Millisecond)
	for range ticker.C {
		n := acl.inpBuf.Read(localBuf)
		transferBuf.Write(localBuf[:n])
	}

	return nil
}
