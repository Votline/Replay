package main

import (
	"embed"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"replay/internal/audio"
	"replay/internal/render"
	"replay/internal/ui"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const helpMsg = `
Usage: replay <command> <path> <debug>

Commands:
	play <path> play audio from file
	record <path> record audio to file
	ui <path> start user interface. Play and record from file
Args:
	<debug> turn on debug mode
Aliases:
	play: p, play, rp, replay, -p, --play, -rp, --replay
	record: r, record, -r, --record
	debug: d, debug, dbg, -d, --debug, -dbg, --dbg
	ui: u, ui, -u, --ui
	help: h, help, -h, --help
`

//go:embed assets
var assets embed.FS

func parseArgs() (mode, path string, dbg bool) {
	args := os.Args[1:] // <mode> <path> <dbg>

	switch args[0] {
	case "p", "play", "-p", "--play", "rp", "replay", "-rp", "--replay":
		mode = "play"
	case "r", "record", "-r", "--record":
		mode = "record"
	case "h", "help", "-h", "--help":
		fmt.Print(helpMsg)
		return "", "", false
	case "u", "ui", "-u", "--ui":
		mode = "ui"
	default:
		fmt.Println("Invalid mode. Use 'help' for help.")
		return "", "", false
	}

	if len(args) < 2 {
		fmt.Println("Usage: replay <mode> <path>")
		return
	}

	path = args[1]
	if len(args) > 2 {
		switch args[2] {
		case "d", "debug", "-d", "--debug", "dbg", "-dbg", "--dbg":
			dbg = true
		}
	}

	return mode, path, dbg
}

func initLog(dbg bool) *zap.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.Encoding = "console"
	cfg.EncoderConfig.TimeKey = ""
	cfg.DisableStacktrace = true
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.ConsoleSeparator = " | "
	cfg.Level.SetLevel(zap.ErrorLevel)

	if dbg {
		cfg.Level.SetLevel(zap.DebugLevel)
	}
	log, _ := cfg.Build()

	return log
}

func main() {
	mode, path, dbg := parseArgs()
	if mode == "" {
		return
	}

	log := initLog(dbg)
	defer log.Sync()

	acl, err := audio.Init(log)
	if err != nil {
		log.Fatal("Init error", zap.Error(err))
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0o666)
	if err != nil {
		log.Error("Open file error", zap.Error(err))
		f, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o666)
		if err != nil {
			log.Error("Create file error", zap.Error(err))
			return
		}
		if _, err := f.Write(make([]byte, 48)); err != nil {
			log.Error("Write segments header error", zap.Error(err))
			return
		}
	}
	defer f.Close()

	log.Debug("Received data",
		zap.String("mode", mode),
		zap.String("path", path),
		zap.Bool("debug", dbg))

	switch mode {
	case "play":
		for {
			if _, err := f.Seek(48, io.SeekStart); err != nil {
				log.Fatal("Seek error", zap.Error(err))
			}
			log.Debug("playing")
			acl.Replay(f)
		}
	case "record":
		if _, err := f.Seek(48, io.SeekStart); err != nil {
			log.Fatal("Seek error", zap.Error(err))
		}
		log.Debug("recording")
		acl.Record(f)
	case "ui":
		log.Debug("ui")
		uiStart(f, log)
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

	view, err := ui.CreateHomeView(pg, ofC, ofTex, win, f, log, assets)
	if err != nil {
		log.Error("Failed to create home view",
			zap.String("op", op),
			zap.Error(err))
		os.Exit(1)
	}

	exitChan := make(chan struct{}, 1)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
		<-sigChan
		exitChan <- struct{}{}
	}()

	go func() {
		<-exitChan
		log.Info("Graceful shutdown")
		view.SaveSegments()
		os.Exit(0)
	}()

	for !win.ShouldClose() {
		gl.Clear(gl.COLOR_BUFFER_BIT)

		view.Render()

		glfw.PollEvents()
		win.SwapBuffers()
	}
	exitChan <- struct{}{}
}
