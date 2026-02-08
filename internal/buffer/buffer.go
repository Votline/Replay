package buffer

import (
	"runtime"
	"sync/atomic"
	"time"
)

type RingBuffer struct {
	wPos, rPos uint64
	bufSize    uint64
	buf        []float32
}

func NewRB(bufSize uint64) *RingBuffer {
	return &RingBuffer{
		wPos:    0,
		rPos:    0,
		bufSize: bufSize,
		buf:     make([]float32, bufSize),
	}
}

func (b *RingBuffer) Write(val []float32) {
	idx := 0
	for _, v := range val {
		for {
			w := atomic.LoadUint64(&b.wPos)
			r := atomic.LoadUint64(&b.rPos)

			if w-r >= b.bufSize {
				spin(&idx)
				continue
			}

			b.buf[w%b.bufSize] = v
			atomic.AddUint64(&b.wPos, 1)
			break
		}
	}
}

func (b *RingBuffer) Read(p []float32) int {
	w := atomic.LoadUint64(&b.wPos)
	r := atomic.LoadUint64(&b.rPos)

	available := w - r
	toRead := uint64(len(p))
	if toRead > available {
		toRead = available
	}

	for i := range toRead {
		p[i] = b.buf[(r+i)%b.bufSize]
	}

	atomic.AddUint64(&b.rPos, toRead)

	return int(toRead)
}

func spin(idx *int) {
	*idx++
	if *idx < 10 {
		runtime.Gosched()
	} else {
		time.Sleep(time.Millisecond)
		*idx = 0
	}
}

func (b *RingBuffer) Reset() {
	atomic.StoreUint64(&b.wPos, 0)
	atomic.StoreUint64(&b.rPos, 0)
}
