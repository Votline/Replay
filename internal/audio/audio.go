package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
	"go.uber.org/zap"
)

func (a *AudioClient) Record(w io.Writer) error {
	const op = "audio.Record"

	a.inpRBuf.Reset()
	a.inpQueue.Reset()
	a.isRecording = true

	samplePerMs := int(a.sampleRate*a.duration/1000) * a.channels

	stream, err := portaudio.OpenStream(
		portaudio.StreamParameters{
			Input: portaudio.StreamDeviceParameters{
				Device:   a.inpDevice,
				Channels: a.channels,
			},
			SampleRate:      float64(a.sampleRate),
			FramesPerBuffer: samplePerMs,
		},
		func(in []float32) {
			a.inpRBuf.Write(in)
			a.log.Debug("Recorded",
				zap.String("op", op),
				zap.Int("len", len(in)))
		})
	if err != nil {
		return fmt.Errorf("%s: create rec stream: %w", op, err)
	}

	if err := stream.Start(); err != nil {
		return fmt.Errorf("%s: start rec stream: %w", op, err)
	}

	defer stream.Stop()
	defer stream.Close()

	var wg sync.WaitGroup
	wg.Go(func() {
		for a.isRecording {
			streamBuf := make([]float32, a.readSize)
			a.inpRBuf.ReadAll(streamBuf, len(streamBuf))
			a.inpQueue.Push(streamBuf)
			a.log.Debug("Pushed to input queue",
				zap.String("op", op),
				zap.Int("len", len(streamBuf)))
		}
	})

	fileBuf := make([]float32, samplePerMs)
	for a.isRecording {
		n := a.inpQueue.Pop(fileBuf)
		if n == 0 {
			time.Sleep(time.Millisecond)
			continue
		}

		a.log.Debug("Popped from input queue",
			zap.String("op", op),
			zap.Int("len", n))

		compressedChunk, err := a.inpCmpr.Compress(fileBuf[:n])
		if err != nil {
			a.log.Error("Compression error",
				zap.String("op", op),
				zap.Error(err))
			continue
		}

		size := uint32(len(compressedChunk))
		if err := binary.Write(w, binary.LittleEndian, size); err != nil {
			a.log.Error("Write error",
				zap.String("op", op),
				zap.Error(err))
			continue
		}

		if _, err := w.Write(compressedChunk); err != nil {
			a.log.Error("Write error",
				zap.String("op", op),
				zap.Error(err))
			continue
		}
	}

	wg.Wait()

	return nil
}

func (a *AudioClient) Replay(r io.Reader) error {
	const op = "audio.Play"

	a.outRBuf.Reset()
	a.isPlaying = true

	samplePerMs := int(a.sampleRate*a.duration/1000) * a.channels

	stream, err := portaudio.OpenStream(
		portaudio.StreamParameters{
			Output: portaudio.StreamDeviceParameters{
				Device:   a.outDevice,
				Channels: a.channels,
			},
			SampleRate:      float64(a.sampleRate),
			FramesPerBuffer: samplePerMs,
		},
		func(in, out []float32) {
			n := a.outRBuf.Read(out)
			for i := n; i < len(out); i++ {
				out[i] = 0
			}
			a.log.Debug("Playing",
				zap.String("op", op),
				zap.Int("len", n))
		})
	if err != nil {
		return fmt.Errorf("%s: create play stream: %w", op, err)
	}

	if err := stream.Start(); err != nil {
		return fmt.Errorf("%s: start play stream: %w", op, err)
	}

	defer stream.Stop()
	defer stream.Close()

	fromFile := make([]byte, samplePerMs)
	var size uint32
	for a.isPlaying {
		err := binary.Read(r, binary.LittleEndian, &size)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			a.log.Debug("Playing finished")
			break
		}
		if err != nil {
			a.log.Error("Read error",
				zap.String("op", op),
				zap.Error(err))
			continue
		}

		a.log.Debug("Read packet",
			zap.String("op", op),
			zap.Int("size", int(size)))

		if size > uint32(len(fromFile)) {
			fromFile = make([]byte, size)
		}
		buf := fromFile[:size]

		if _, err := io.ReadFull(r, buf); err != nil {
			a.log.Error("Read error",
				zap.String("op", op),
				zap.Error(err))
			continue
		}

		pcm, err := a.outCmpr.Decompress(buf)
		if err != nil {
			a.log.Error("Decompression error",
				zap.String("op", op),
				zap.Error(err))
			continue
		}

		a.outRBuf.Write(pcm)

		a.log.Debug("Write packet",
			zap.String("op", op),
			zap.Int("size", len(pcm)))
	}

	for a.outRBuf.Len() > 0 {
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(time.Duration(a.duration) * time.Millisecond)

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
