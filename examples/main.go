package main

import (
	"fmt"
	"github.com/c-bata/go-prompt"
	mirvpgl "github.com/fython/hlae-server-kit-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	srv    *mirvpgl.HLAEServer
	logger *zap.Logger
)

// ExampleHandler for HLAE Server
func ExampleHandler(cmd string) {
	fmt.Printf("Received %s\n", cmd)
	if cmd == "hello" {
		srv.BroadcastRCON("echo Hello from hlae-server-kit-go")
	}
}

// ExampleCamHandler for cam datas
func ExampleCamHandler(cam *mirvpgl.CamData) {
	// fmt.Printf("Received cam data %v\n", cam)
}

// ExampleEventHandler for cam datas
func ExampleEventHandler(ev *mirvpgl.GameEventData) {
	fmt.Printf("Received event data %v\n", ev)
}

func completer(in prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{}
	return prompt.FilterHasPrefix(s, in.GetWordBeforeCursor(), true)
}

func newLogger() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.Level.SetLevel(zap.DebugLevel)
	cfg.Encoding = "console"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	logger, _ := cfg.Build()
	return logger
}

func main() {
	var err error
	logger = newLogger()
	srv, err = mirvpgl.New(mirvpgl.HLAEServerArguments{
		Logger: logger,
	})
	if err != nil {
		panic(err)
	}
	srv.RegisterHandler(ExampleHandler)
	srv.RegisterCamHandler(ExampleCamHandler)
	srv.RegisterEventHandler(ExampleEventHandler)

	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()
	engine.GET("/", func(c *gin.Context) {
		srv.ServeHTTP(c.Writer, c.Request)
	})
	go func() {
		err := engine.Run(":65535")
		if err != nil {
			panic(err)
		}
	}()

	// NOTE : enclose ws URL with double quotes...
	// mirv_pgl url "ws://localhost:65535/"
	// mirv_pgl start
	// mirv_pgl datastart
	for {
		cmd := prompt.Input("HLAE >>> ", completer)
		if cmd == "exit" {
			break
		}
		srv.BroadcastRCON(cmd)
	}
}
