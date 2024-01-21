package tapo

import (
	"github.com/dadav/go2rtc/internal/streams"
	"github.com/dadav/go2rtc/pkg/core"
	"github.com/dadav/go2rtc/pkg/kasa"
	"github.com/dadav/go2rtc/pkg/tapo"
)

func Init() {
	streams.HandleFunc("kasa", func(url string) (core.Producer, error) {
		return kasa.Dial(url)
	})

	streams.HandleFunc("tapo", func(url string) (core.Producer, error) {
		return tapo.Dial(url)
	})
}
