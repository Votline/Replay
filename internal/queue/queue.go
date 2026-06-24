package queue

type Queue struct {
	buf []float32
}

func New(bufLen int) *Queue {
	return &Queue{
		buf: make([]float32, 0, bufLen),
	}
}

func (q *Queue) Push(v []float32) {
	q.buf = append(q.buf, v...)
}

func (q *Queue) Pop(p []float32) int {
	if len(q.buf) == 0 {
		return 0
	}

	n := len(p)
	n = min(n, len(q.buf))

	copy(p[:n], q.buf[:n])
	q.buf = q.buf[n:]

	return n
}

func (q *Queue) Reset() {
	q.buf = q.buf[:0]
}

func (q *Queue) Len() int {
	return len(q.buf)
}
