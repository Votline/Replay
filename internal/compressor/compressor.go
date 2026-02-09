package compressor

import (
	"fmt"

	"github.com/hraban/opus"
	"github.com/klauspost/compress/zstd"
)

type Compressor struct {
	opusEn *opus.Encoder
	opusDe *opus.Decoder

	zstdEn *zstd.Encoder
	zstdDe *zstd.Decoder

	ch         int
	btr        int
	smpR       int
	frameDurMs int
}

func NewCompressor(ch, btr, smpR, frameDurMs int) (*Compressor, error) {
	const op = "compressor.NewCompressor"

	opusEn, err := opus.NewEncoder(smpR, ch, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("%s: encoder init: %w", op, err)
	}
	opusEn.SetBitrate(btr)

	opusDe, err := opus.NewDecoder(smpR, ch, opus.AppVoIP)
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

func (c *Compressor) Compress(pcm []int16) ([]byte, error) {
	const op = "compressor.Compress"

	if len(pcm) == 0 {
		return nil, nil
	}

	encoded, err := c.encode(pcm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return c.zstdEn.EncodeAll(encoded, nil), nil
}

func (c *Compressor) encode(pcm []int16) ([]byte, error) {
	const op = "compressor.encode"

	data := make([]byte, len(pcm))
	n, err := c.opusEn.Encode(pcm, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	res := make([]byte, n)
	copy(res, data[:n])
	return res, nil
}

func (c *Compressor) Decompress(data []byte, bufSize int) ([]int16, error) {
	const op = "compressor.Decompress"

	if len(data) == 0 {
		return nil, nil
	}

	decoded, err := c.zstdDe.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	pcm, err := c.decode(decoded, bufSize)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
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

	res := make([]int16, n)
	copy(res, pcm[:n])
	return res, nil
}
