package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"replay/internal/audio"
	"replay/internal/render"
	"replay/internal/ui"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func initLog(d bool) *zap.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.Encoding = "console"
	cfg.EncoderConfig.TimeKey = ""
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.DisableStacktrace = true

	lvl := zapcore.ErrorLevel
	if d {
		lvl = zapcore.DebugLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)

	log, _ := cfg.Build()

	return log
}

func main() {
	debug := flag.Bool("debug", false, "debug mode")
	path := flag.String("path", "", "path to file")
	mode := flag.String("mode", "", "mode(record|replay)")
	flag.Parse()

	log := initLog(*debug)

	if *path == "" {
		log.Error("Path is empty")
		return
	}

	f, err := os.OpenFile(*path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		log.Error("Open file error", zap.Error(err))
		return
	}

	acl, err := audio.Init(log)
	if err != nil {
		log.Error("Init audio error", zap.Error(err))
		return
	}

	switch *mode {
	case "record":
		acl.Record(f)
	case "replay":
		for {
			_, err := f.Seek(0, 0)
			if err != nil {
				log.Error("Seek file error", zap.Error(err))
				os.Exit(1)
			}
			acl.Replay(f)
		}
	case "":
		uiStart(f, log)
	default:
		fmt.Println("Usage: replay --path=file.bak --mode=record|replay")
	}
}

func init() {
	runtime.LockOSThread()
}

func uiStart(f *os.File, log *zap.Logger) {
	const op = "uiSetup"

	if err := glfw.Init(); err != nil {
		log.Error("Failed to initialize GLFW",
			zap.String("op", op),
			zap.Error(err))
		os.Exit(1)
	}
	defer glfw.Terminate()

	if err := gl.Init(); err != nil {
		log.Error("Failed to initialize OpenGL",
			zap.String("op", op),
			zap.Error(err))
		os.Exit(1)
	}

	win, err := ui.PrimaryWindow()
	if err != nil {
		log.Error("Failed to create window",
			zap.String("op", op),
			zap.Error(err))
		os.Exit(1)
	}

	win.MakeContextCurrent()
	win.SetAttrib(glfw.Floating, glfw.True)
	glfw.SwapInterval(1)
	glfw.WaitEventsTimeout(0.1)

	pg, ofC, ofTex := render.Setup()
	defer gl.DeleteProgram(pg)

	view, err := ui.CreateHomeView(pg, ofC, ofTex, win, f, log)
	if err != nil {
		log.Error("Failed to create home view",
			zap.String("op", op),
			zap.Error(err))
		os.Exit(1)
	}

	for !win.ShouldClose() {
		gl.Clear(gl.COLOR_BUFFER_BIT)

		view.Render()

		glfw.PollEvents()
		win.SwapBuffers()
	}
}
