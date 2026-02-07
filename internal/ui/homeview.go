package ui

import (
	"os"

	"replay/internal/audio"
	"replay/internal/render"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type elemMesh struct {
	vao  uint32
	vtq  int32
	pos  [4]float32
	name string
}

type HomeView struct {
	ofC   int32
	ofTex int32
	pg    uint32
	texID uint32
	elems [7]elemMesh
	acl   *audio.AudioClient
	win   *glfw.Window
	f     *os.File
}

func CreateHomeView(pg uint32, ofC, ofTex int32, win *glfw.Window, f *os.File) (*HomeView, error) {
	acl, err := audio.Init()
	if err != nil {
		return nil, err
	}

	hv := &HomeView{pg: pg, ofC: ofC, ofTex: ofTex, win: win, f: f, acl: acl}
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
							hv.acl.StopReplay()
							hv.elems[5], hv.elems[0] = hv.elems[0], hv.elems[5]
							return
						} else {
							hv.elems[5], hv.elems[0] = hv.elems[0], hv.elems[5]
							go hv.acl.Replay(hv.f)
						}
					case "Prev":
					case "Next":
					case "Reset":
						hv.f.Seek(0, 0)
					case "Record":
						if hv.acl.IsRecording() {
							hv.acl.StopRecording()
							hv.elems[4], hv.elems[6] = hv.elems[6], hv.elems[4]
							return
						} else {
							hv.elems[6], hv.elems[4] = hv.elems[4], hv.elems[6]
							go hv.acl.Record(hv.f)
						}
					}
				}
			}
		}
	}
}
