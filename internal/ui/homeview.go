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
	elems [1]elemMesh
	pg    uint32
	ofC   int32
}

func CreateHomeView(pg uint32, ofC int32) *HomeView {
	hv := &HomeView{pg: pg, ofC: ofC}

	hv.elems[0].vao = render.CreateVAO([]float32{
		0.4, 0.0, 0.0,
		0.8, 0.8, 0.0,
		-0.4, 0.0, 0.0,
	})
	hv.elems[0].vtq = 3

	gl.ClearColor(0.0, 0.0, 0.0, 0.7)

	return hv
}

func (hv *HomeView) Render() {
	render.ElemRender(hv.pg, hv.elems[0].vao, hv.elems[0].vtq, hv.ofC)
}
