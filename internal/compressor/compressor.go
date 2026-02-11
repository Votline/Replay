package compressor

import (
	"fmt"

	"github.com/hraban/opus"
	"github.com/klauspost/compress/zstd"
	"go.uber.org/zap"
)

type Compressor struct {
	log *zap.Logger

	opusEn *opus.Encoder
	opusDe *opus.Decoder

	zstdEn *zstd.Encoder
	zstdDe *zstd.Decoder

	ch         int
	btr        int
	smpR       int
	frameDurMs int
}

func NewCompressor(ch, btr, smpR, frameDurMs int, log *zap.Logger) (*Compressor, error) {
	const op = "compressor.NewCompressor"

	opusEn, err := opus.NewEncoder(smpR, ch, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("%s: encoder init: %w", op, err)
	}
	opusEn.SetBitrate(btr)

	opusDe, err := opus.NewDecoder(smpR, ch)
	if err != nil {
		return nil, fmt.Errorf("%s: decoder init: %w", op, err)
	}

	zstdEn, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, fmt.Errorf("%s: zstd encoder init: %w", op, err)
	}

	zstdDe, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("%s: zstd decoder init: %w", op, err)
	}

	return &Compressor{
		log: log,

		opusEn: opusEn,
		opusDe: opusDe,

		zstdEn: zstdEn,
		zstdDe: zstdDe,

		ch:         ch,
		btr:        btr,
		smpR:       smpR,
		frameDurMs: frameDurMs,
	}, nil
}

func (c *Compressor) Compress(pcm []float32) ([]byte, error) {
	const op = "compressor.Compress"

	if len(pcm) == 0 {
		c.log.Warn("Empty pcm")
		return nil, nil
	}

	pcmI16 := make([]int16, len(pcm))
	for i, v := range pcm {
		pcmI16[i] = int16(v * 32767)
	}

	encoded, err := c.encode(pcmI16)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return c.zstdEn.EncodeAll(encoded, nil), nil
}

func (c *Compressor) encode(pcm []int16) ([]byte, error) {
	const op = "compressor.encode"

	samplesPerFrame := (c.smpR * c.frameDurMs / 1000) * c.ch
	if len(pcm) < samplesPerFrame {
		return nil, fmt.Errorf("%s: pcm too short: %d < %d", op, len(pcm), samplesPerFrame)
	}

	frame := pcm[:samplesPerFrame]

	data := make([]byte, 1275)
	n, err := c.opusEn.Encode(frame, data)
	if err != nil {
		return nil, fmt.Errorf("%s: encode: %w", op, err)
	}

	res := make([]byte, n)
	copy(res, data[:n])
	return res, nil
}

func (c *Compressor) Decompress(data []byte, bufSize int) ([]float32, error) {
	const op = "compressor.Decompress"

	if len(data) == 0 {
		return nil, nil
	}

	decoded, err := c.zstdDe.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: decode all: %w", op, err)
	}

	pcmI16, err := c.decode(decoded, bufSize)
	if err != nil {
		return nil, fmt.Errorf("%s: decode to int16: %w", op, err)
	}

	pcm := make([]float32, len(pcmI16))
	for i, v := range pcmI16 {
		pcm[i] = float32(v) / 32767
	}

	return pcm, nil
}

func (c *Compressor) decode(data []byte, bufSize int) ([]int16, error) {
	const op = "compressor.decode"

	pcm := make([]int16, bufSize)
	n, err := c.opusDe.Decode(data, pcm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	totalSamples := n * c.ch

	res := make([]int16, totalSamples)
	copy(res, pcm[:totalSamples])
	return res, nil
}
