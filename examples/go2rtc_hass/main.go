package main

import (
	"github.com/dadav/go2rtc/internal/api"
	"github.com/dadav/go2rtc/internal/app"
	"github.com/dadav/go2rtc/internal/hass"
	"github.com/dadav/go2rtc/internal/streams"
	"github.com/dadav/go2rtc/pkg/shell"
)

func main() {
	app.Init()
	streams.Init()

	api.Init()

	hass.Init()

	shell.RunUntilSignal()
}
