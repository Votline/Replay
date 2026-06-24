package compressor

import (
	"fmt"

	"github.com/hraban/opus"
	"github.com/klauspost/compress/zstd"
	"go.uber.org/zap"
)

type Compressor struct {
	bitrate         int
	channels        int
	sampleRate      int
	samplerPerFrame int
	duration        int

	opusEn *opus.Encoder
	zstdEn *zstd.Encoder
	encBuf []byte

	opusDe *opus.Decoder
	zstdDe *zstd.Decoder
	decBuf []int16

	log *zap.Logger
}

func Init(bitrate, channels, sampleRate, duration int, encBuf []byte, decBuf []int16, log *zap.Logger) (*Compressor, error) {
	const op = "compressor.Init"

	opusEn, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("%s: opus.NewEncoder: %w", op, err)
	}

	opusDe, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("%s: opus.NewDecoder: %w", op, err)
	}

	zstdEn, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, fmt.Errorf("%s: zstd.NewWriter: %w", op, err)
	}

	zstdDe, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("%s: zstd.NewReader: %w", op, err)
	}

	samplerPerFrame := (sampleRate * duration / 1000) * channels

	return &Compressor{
		bitrate:         bitrate,
		channels:        channels,
		sampleRate:      sampleRate,
		duration:        duration,
		samplerPerFrame: samplerPerFrame,

		opusEn: opusEn,
		zstdEn: zstdEn,
		encBuf: encBuf,

		opusDe: opusDe,
		zstdDe: zstdDe,
		decBuf: decBuf,

		log: log,
	}, nil
}

func (c *Compressor) Compress(data []float32) ([]byte, error) {
	const op = "compressor.Compress"

	pcm := make([]int16, len(data))
	for i, v := range data {
		pcm[i] = int16(v * 32767)
	}

	enc, err := c.encode(pcm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return c.zstdEn.EncodeAll(enc, nil), nil
}

func (c *Compressor) Decompress(data []byte) ([]float32, error) {
	const op = "compressor.Decompress"

	if len(data) == 0 {
		return nil, nil
	}

	dec, err := c.zstdDe.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: zstd.DecodeAll: %w", op, err)
	}

	pcmI16, err := c.decode(dec)
	if err != nil {
		return nil, fmt.Errorf("%s: opus.Decode: %w", op, err)
	}

	pcm := make([]float32, len(pcmI16))
	for i, v := range pcmI16 {
		pcm[i] = float32(v) / 32767
	}

	return pcm, nil
}

func (c *Compressor) encode(pcm []int16) ([]byte, error) {
	const op = "compressor.encode"

	n, err := c.opusEn.Encode(pcm, c.encBuf)
	if err != nil {
		return nil, fmt.Errorf("%s: opus.Encode: %w", op, err)
	}

	return c.encBuf[:n], nil
}

func (c *Compressor) decode(data []byte) ([]int16, error) {
	const op = "compressor.decode"

	n, err := c.opusDe.Decode(data, c.decBuf)
	if err != nil {
		return nil, fmt.Errorf("%s: opus.Decode: %w", op, err)
	}
	return c.decBuf[:n*c.channels], nil
}
