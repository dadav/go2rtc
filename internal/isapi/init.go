package isapi

import (
	"github.com/dadav/go2rtc/internal/streams"
	"github.com/dadav/go2rtc/pkg/core"
	"github.com/dadav/go2rtc/pkg/isapi"
)

func Init() {
	streams.HandleFunc("isapi", handle)
}

func handle(url string) (core.Producer, error) {
	conn, err := isapi.NewClient(url)
	if err != nil {
		return nil, err
	}
	if err = conn.Dial(); err != nil {
		return nil, err
	}
	return conn, nil
}
