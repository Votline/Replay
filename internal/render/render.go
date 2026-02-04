package render

import "github.com/go-gl/gl/v4.1-core/gl"

func Setup() (uint32, int32) {
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	pg := gl.CreateProgram()

	shaders := attachShaders(pg)

	gl.LinkProgram(pg)
	gl.UseProgram(pg)

	ofC := gl.GetUniformLocation(pg, gl.Str("color\x00"))

	detachShaders(pg, shaders)

	return pg, ofC
}

func CreateVAO(vtc []float32) uint32 {
	var vao, vbo uint32

	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)

	gl.BindVertexArray(vao)

	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vtc)*4, gl.Ptr(vtc), gl.STATIC_DRAW)

	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 0, nil)
	gl.EnableVertexAttribArray(0)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	return vao
}

func ElemRender(pg uint32, vao uint32, vtq int32, ofC int32) {
	gl.UseProgram(pg)

	gl.Uniform4f(ofC, 1.0, 0.0, 0.0, 1.0)

	gl.BindVertexArray(vao)

	gl.DrawArrays(gl.TRIANGLES, 0, vtq)
}
