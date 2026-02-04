package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"replay/internal/audio"
	"replay/internal/render"
	"replay/internal/ui"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: replay <mode>(record|replay")
		return
	}

	acl, err := audio.Init()
	if err != nil {
		fmt.Printf("Init audio error: %s\n", err.Error())
		return
	}

	go uiStart()

	f, err := os.OpenFile("file.bak", 0o666, os.FileMode(os.O_RDWR))
	if err != nil {
		panic(err)
	}

	mode := os.Args[1]
	switch mode {
	case "record":
		acl.Record(f)
	case "replay":
		for {
		_, err := f.Seek(0, 0)
			if err != nil {
				fmt.Printf("Seek failed: %s\n", err.Error())
				os.Exit(1)
			}
			acl.Replay(f)
		}
	default:
		fmt.Println("Usage: replay <mode>(record|replay")
	}
}

func init() {
	runtime.LockOSThread()
}

func uiStart() {
	const op = "uiSetup"

	if err := glfw.Init(); err != nil {
		fmt.Printf("Failed to initialize GLFW: %v", err)
		os.Exit(1)
	}
	defer glfw.Terminate()

	if err := gl.Init(); err != nil {
		fmt.Printf("Failed to initialize OpenGL: %v", err)
		os.Exit(1)
	}

	win, err := ui.PrimaryWindow()
	if err != nil {
		fmt.Printf("%s: Failed to initialize window: %v", op, err)
		os.Exit(1)
	}

	win.MakeContextCurrent()
	win.SetAttrib(glfw.Floating, glfw.True)
	glfw.SwapInterval(1)
	glfw.WaitEventsTimeout(0.1)

	pg, ofC := render.Setup()
	defer gl.DeleteProgram(pg)

	view := ui.CreateHomeView(pg, ofC)

	for !win.ShouldClose() {
		gl.Clear(gl.COLOR_BUFFER_BIT)

		time.Sleep(time.Millisecond)
		view.Render()

		glfw.PollEvents()
		win.SwapBuffers()
	}
}
