package ui

import (
	"io"
	"os"
	"time"

	"replay/internal/audio"
	"replay/internal/render"
	"replay/internal/writer"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"go.uber.org/zap"
)

type elemMesh struct {
	vao  uint32
	vtq  int32
	pos  [4]float32
	name string
}

type HomeView struct {
	ofC       int32
	ofTex     int32
	pg        uint32
	texID     uint32
	elems     [7]elemMesh
	acl       *audio.AudioClient
	win       *glfw.Window
	f         *os.File
	segments  [3]audio.AudioSegment
	curIdx    int
	log       *zap.Logger
	recWriter *writer.SectionWriter
	recReader *io.SectionReader
}

func CreateHomeView(pg uint32, ofC, ofTex int32, win *glfw.Window, f *os.File, log *zap.Logger) (*HomeView, error) {
	acl, err := audio.Init(log)
	if err != nil {
		return nil, err
	}

	hv := &HomeView{pg: pg, ofC: ofC, ofTex: ofTex, win: win, f: f, acl: acl, log: log}
	hv.texID = render.LoadTexture("assets/texture.png")

	addRect := func(x, y, w, h, u1, v1, u2, v2 float32, idx int, name string) {
		vertices := []float32{
			x, y, 0.0, u1, v2,
			x, y + h, 0.0, u1, v1,
			x + w, y + h, 0.0, u2, v1,
			x + w, y, 0.0, u2, v2,
		}
		indices := []uint32{
			0, 1, 2,
			2, 3, 0,
		}

		hv.elems[idx].vao = render.CreateVAO(vertices, indices)
		hv.elems[idx].vtq = int32(len(indices))
		hv.elems[idx].pos = [4]float32{x, y, x + w, y + h}
		hv.elems[idx].name = name
	}

	rect := func(x, y, w, h, px, py, pw, ph float32, idx int, name string) {
		u1, v1, u2, v2 := getUV(px, py, pw, ph)
		addRect(x, y, w, h, u1, v1, u2, v2, idx, name)
	}

	// rect(x, y, w, h, px, py, pw, ph, idx)
	rect(-0.3, -0.9, 0.6, 0.9, 12-8, 4, 64-4, 64, 5, "Pause")         // Pause
	rect(-0.3, -0.9, 0.6, 0.9, 64+12, 4, 64-4, 64-2, 0, "Play&Pause") // Play&Pause
	rect(-0.8, -0.9, 0.3, 0.6, 6, 64+8, 64-4, 64-2, 1, "Prev")        // Previous
	rect(0.5, -0.9, 0.3, 0.6, 64+14, 64+12, 64-6, 64-8, 2, "Next")    // Next
	rect(-0.8, 0.2, 0.3, 0.6, 128+22, 64+12, 64-4, 64-8, 3, "Reset")  // Reset
	rect(0.5, 0.2, 0.3, 0.6, 128+22, 8, 64-4, 64-12, 4, "Record")     // Record
	rect(0.5, 0.2, 0.3, 0.6, 12-8, 9, 64-4, 64-12, 6, "StopRecord")   // StopRecord

	gl.ClearColor(0.0, 0.0, 0.0, 0.7)

	win.SetMouseButtonCallback(hv.btnCallback())

	return hv, nil
}

func (hv *HomeView) Render() {
	for i := range len(hv.elems) - 2 {
		render.ElemRender(hv.pg, hv.elems[i].vao, hv.texID,
			hv.elems[i].vtq, hv.ofC, hv.ofTex)
	}
}

func getUV(px, py, pw, ph float32) (u1, v1, u2, v2 float32) {
	const size = 256.0
	u1 = px / size
	v1 = py / size
	u2 = (px + pw) / size
	v2 = (py + ph) / size
	return u1, v1, u2, v2
}

func (hv *elemMesh) hover(w *glfw.Window, x, y float32) bool {
	mX, mY := w.GetCursorPos()
	glX := float32(mX)/x*2 - 1
	glY := 1 - float32(mY)/y*2

	if glX >= hv.pos[0] && glX <= hv.pos[2] &&
		glY >= hv.pos[1] && glY <= hv.pos[3] {
		return true
	}

	return false
}

func (hv *HomeView) btnCallback() func(w *glfw.Window, b glfw.MouseButton, a glfw.Action, m glfw.ModifierKey) {
	return func(w *glfw.Window, b glfw.MouseButton, a glfw.Action, m glfw.ModifierKey) {
		if a == glfw.Press && b == glfw.MouseButtonLeft {
			for _, el := range hv.elems {
				if el.hover(w, winW, winH) {
					switch el.name {
					case "Play&Pause":
						if hv.acl.IsPlaying() {
							hv.log.Info("Stop replay")

							hv.acl.StopReplay()
							hv.elems[5], hv.elems[0] = hv.elems[0], hv.elems[5]

							hv.log.Info("Swapped buttons")
						} else {
							hv.log.Info("Start replay")

							hv.elems[5], hv.elems[0] = hv.elems[0], hv.elems[5]

							go func() {
								hv.restartReplay()
								hv.acl.Replay(hv.recReader)
								for hv.acl.IsPlaying() {
									hv.restartReplay()
									hv.acl.Replay(hv.recReader)
								}
							}()

							hv.log.Info("Swapped buttons")
						}
					case "Next":
						hv.curIdx++
						if hv.curIdx > len(hv.segments)-1 {
							hv.curIdx = 0
							hv.segments[hv.curIdx].Start = 0
							hv.log.Info("Next",
								zap.Int("curIdx", hv.curIdx))
							return
						}
						hv.segments[hv.curIdx].Start = hv.segments[hv.curIdx-1].End
						hv.log.Info("Next",
							zap.Int("curIdx", hv.curIdx))
					case "Prev":
						hv.curIdx--
						if hv.curIdx-1 < 0 {
							hv.curIdx = 0
							hv.segments[hv.curIdx].Start = 0
							hv.log.Info("Prev",
								zap.Int("curIdx", hv.curIdx))
							return
						}
						hv.segments[hv.curIdx].Start = hv.segments[hv.curIdx-1].End
						hv.log.Info("Prev",
							zap.Int("curIdx", hv.curIdx))
					case "Reset":
						hv.curIdx = 0
						hv.segments = [3]audio.AudioSegment{}
						hv.f.Seek(0, 0)
					case "Record":
						if hv.acl.IsRecording() {
							hv.log.Info("Stop record")

							hv.acl.StopRecording()
							hv.segments[hv.curIdx].End = hv.recWriter.Pos()
							hv.recWriter = nil

							hv.log.Info("Indexies", zap.Any("seg", hv.segments[hv.curIdx]))

							hv.elems[4], hv.elems[6] = hv.elems[6], hv.elems[4]

							hv.log.Info("Swapped buttons")
							return
						} else {
							hv.log.Info("Start record")

							hv.recWriter = writer.NewSectionWriter(hv.f, hv.segments[hv.curIdx].Start)

							hv.log.Info("Indexies", zap.Any("seg", hv.segments[hv.curIdx]))

							hv.elems[6], hv.elems[4] = hv.elems[4], hv.elems[6]
							time.Sleep(200 * time.Millisecond)
							go hv.acl.Record(hv.recWriter)

							hv.log.Info("Start record")
						}
					}
				}
			}
		}
	}
}

func (hv *HomeView) getAudioIdx() int64 {
	hv.log.Info("Get audio file size")

	info, err := hv.f.Stat()
	if err != nil {
		hv.log.Error("Get audio file size error. Zero should be returned", zap.Error(err))
		return 0
	}

	hv.log.Info("Get audio file size", zap.Int64("size", info.Size()))

	return info.Size()
}

func (hv *HomeView) restartReplay() {
	if hv.curIdx >= len(hv.segments) {
		return
	}
	seg := hv.segments[hv.curIdx]

	if seg.End <= seg.Start {
		hv.log.Error("Empty segment", zap.Any("seg", seg))
		return
	}

	hv.log.Info("Restart replay", zap.Any("seg", seg))

	hv.recReader = io.NewSectionReader(
		hv.f,
		int64(seg.Start),
		int64(seg.End-seg.Start),
	)
}
