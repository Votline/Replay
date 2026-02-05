package ui

import (
	"replay/internal/render"

	"github.com/go-gl/gl/v4.1-core/gl"
)

type elemMesh struct {
	vao uint32
	vtq int32
}

type HomeView struct {
	ofC   int32
	ofTex int32
	pg    uint32
	texID uint32
	elems [5]elemMesh
}

func CreateHomeView(pg uint32, ofC, ofTex int32) *HomeView {
	hv := &HomeView{pg: pg, ofC: ofC, ofTex: ofTex}
	hv.texID = render.LoadTexture("assets/texture.png")

	addRect := func(x, y, w, h, u1, v1, u2, v2 float32, idx int) {
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
	}

	rect := func(x, y, w, h, px, py, pw, ph float32, idx int) {
		u1, v1, u2, v2 := getUV(px, py, pw, ph)
		addRect(x, y, w, h, u1, v1, u2, v2, idx)
	}

	// rect(x, y, w, h, px, py, pw, ph, idx)
	rect(-0.3, -0.9, 0.6, 0.9, 64+12, 4, 64-4, 64-2, 0)     // Play&Pause
	rect(-0.8, -0.9, 0.3, 0.6, 6, 64+8, 64-4, 64-2, 1)      // Previous
	rect(0.5, -0.9, 0.3, 0.6, 64+14, 64+12, 64-6, 64-8, 2)  // Next
	rect(-0.8, 0.2, 0.3, 0.6, 128+22, 64+12, 64-4, 64-8, 3) // Reset
	rect(0.5, 0.2, 0.3, 0.6, 128+22, 8, 64-4, 64-12, 4)     // Record

	gl.ClearColor(0.0, 0.0, 0.0, 0.7)

	return hv
}

func (hv *HomeView) Render() {
	for i := range hv.elems {
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
