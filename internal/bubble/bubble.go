package bubble

import (
	"github.com/dadav/go2rtc/internal/streams"
	"github.com/dadav/go2rtc/pkg/bubble"
	"github.com/dadav/go2rtc/pkg/core"
)

func Init() {
	streams.HandleFunc("bubble", handle)
}

func handle(url string) (core.Producer, error) {
	conn := bubble.NewClient(url)
	if err := conn.Dial(); err != nil {
		return nil, err
	}
	return conn, nil
}
