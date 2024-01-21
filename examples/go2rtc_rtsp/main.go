package main

import (
	"github.com/dadav/go2rtc/internal/app"
	"github.com/dadav/go2rtc/internal/rtsp"
	"github.com/dadav/go2rtc/internal/streams"
	"github.com/dadav/go2rtc/pkg/shell"
)

func main() {
	app.Init()
	streams.Init()

	rtsp.Init()

	shell.RunUntilSignal()
}
