package ivideon

import (
	"github.com/dadav/go2rtc/internal/streams"
	"github.com/dadav/go2rtc/pkg/core"
	"github.com/dadav/go2rtc/pkg/ivideon"
	"strings"
)

func Init() {
	streams.HandleFunc("ivideon", func(url string) (core.Producer, error) {
		id := strings.Replace(url[8:], "/", ":", 1)
		prod := ivideon.NewClient(id)
		if err := prod.Dial(); err != nil {
			return nil, err
		}
		return prod, nil
	})
}
