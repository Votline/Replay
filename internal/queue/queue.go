package queue

import (
	"sync"
)

type Queue struct {
	mu sync.Mutex
	buf []float32
}

func NewQueue(bufSize uint64) *Queue {
	return &Queue{
		buf: make([]float32, 0, bufSize),
	}
}

func (q *Queue) Push(data []float32) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.buf = append(q.buf, data...)
}

func (q *Queue) Pop(data []float32) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.buf) == 0 {
		return 0
	}

	n := len(data)
	if n > (len(q.buf)) {
		n = len(q.buf)
	}

	copy(data[:n], q.buf[:n])
	q.buf = q.buf[n:]

	return n
}
