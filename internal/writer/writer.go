package writer

import "os"

type SectionWriter struct {
	f    *os.File
	base int64
	pos  int64
}

func NewSectionWriter(f *os.File, base int64) *SectionWriter {
	return &SectionWriter{f: f, base: base}
}

func (sw *SectionWriter) Write(p []byte) (n int, err error) {
	n, err = sw.f.WriteAt(p, sw.base+sw.pos)
	sw.pos += int64(n)
	return
}

func (sw *SectionWriter) Pos() int64 {
	return sw.base + sw.pos
}
