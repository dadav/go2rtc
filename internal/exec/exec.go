package exec

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AlexxIT/go2rtc/internal/app"
	"github.com/AlexxIT/go2rtc/internal/rtsp"
	"github.com/AlexxIT/go2rtc/internal/streams"
	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/magic"
	pkg "github.com/AlexxIT/go2rtc/pkg/rtsp"
	"github.com/AlexxIT/go2rtc/pkg/shell"
	"github.com/rs/zerolog"
)

func parseSignal(signalString string) (os.Signal, error) {
	signalMap := map[string]os.Signal{
		"sighup":    syscall.SIGHUP,
		"sigint":    syscall.SIGINT,
		"sigquit":   syscall.SIGQUIT,
		"sigill":    syscall.SIGILL,
		"sigtrap":   syscall.SIGTRAP,
		"sigabrt":   syscall.SIGABRT,
		"sigbus":    syscall.SIGBUS,
		"sigfpe":    syscall.SIGFPE,
		"sigkill":   syscall.SIGKILL,
		"sigusr1":   syscall.SIGUSR1,
		"sigsegv":   syscall.SIGSEGV,
		"sigusr2":   syscall.SIGUSR2,
		"sigpipe":   syscall.SIGPIPE,
		"sigalrm":   syscall.SIGALRM,
		"sigterm":   syscall.SIGTERM,
		"sigchld":   syscall.SIGCHLD,
		"sigcont":   syscall.SIGCONT,
		"sigstop":   syscall.SIGSTOP,
		"sigtstp":   syscall.SIGTSTP,
		"sigttin":   syscall.SIGTTIN,
		"sigttou":   syscall.SIGTTOU,
		"sigurg":    syscall.SIGURG,
		"sigxcpu":   syscall.SIGXCPU,
		"sigxfsz":   syscall.SIGXFSZ,
		"sigvtalrm": syscall.SIGVTALRM,
		"sigprof":   syscall.SIGPROF,
		"sigwinch":  syscall.SIGWINCH,
		"sigio":     syscall.SIGIO,
		"sigpoll":   syscall.SIGPOLL,
		"sigpwr":    syscall.SIGPWR,
		"sigsys":    syscall.SIGSYS,
	}

	signalValue, ok := signalMap[strings.ToLower(signalString)]
	if !ok {
		return nil, fmt.Errorf("invalid signal: %s", signalString)
	}

	return signalValue, nil
}

var defaults = map[string]string{
	"signal": "sigkill",
}

func Init() {
	var cfg struct {
		Mod map[string]string `yaml:"exec"`
	}

	cfg.Mod = defaults // will be overriden from yaml

	app.LoadConfig(&cfg)

	s, err := parseSignal(defaults["signal"])
	if err != nil {
		log.Error().Err(err).Str("signal", defaults["signal"]).Msg("[exec] bad value")
		panic(err)
	}

	killSignal = s

	rtsp.HandleFunc(func(conn *pkg.Conn) bool {
		waitersMu.Lock()
		waiter := waiters[conn.URL.Path]
		waitersMu.Unlock()

		if waiter == nil {
			return false
		}

		// unblocking write to channel
		select {
		case waiter <- conn:
			return true
		default:
			return false
		}
	})

	streams.HandleFunc("exec", execHandle)

	log = app.GetLogger("exec")
}

func execHandle(url string) (core.Producer, error) {
	var path string

	args := shell.QuoteSplit(url[5:]) // remove `exec:`
	for i, arg := range args {
		if arg == "{output}" {
			if rtsp.Port == "" {
				return nil, errors.New("rtsp module disabled")
			}

			sum := md5.Sum([]byte(url))
			path = "/" + hex.EncodeToString(sum[:])
			args[i] = "rtsp://127.0.0.1:" + rtsp.Port + path
			break
		}
	}

	cmd := exec.Command(args[0], args[1:]...)
	if log.Debug().Enabled() {
		cmd.Stderr = os.Stderr
	}

	if path == "" {
		return handlePipe(url, cmd)
	}

	return handleRTSP(url, path, cmd)
}

func handlePipe(url string, cmd *exec.Cmd) (core.Producer, error) {
	r, err := PipeCloser(cmd)
	if err != nil {
		return nil, err
	}

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	prod, err := magic.Open(r)
	if err != nil {
		_ = r.Close()
	}

	return prod, err
}

func handleRTSP(url, path string, cmd *exec.Cmd) (core.Producer, error) {
	if log.Trace().Enabled() {
		cmd.Stdout = os.Stdout
	}

	ch := make(chan core.Producer)

	waitersMu.Lock()
	waiters[path] = ch
	waitersMu.Unlock()

	defer func() {
		waitersMu.Lock()
		delete(waiters, path)
		waitersMu.Unlock()
	}()

	log.Debug().Str("url", url).Msg("[exec] run")

	ts := time.Now()

	if err := cmd.Start(); err != nil {
		log.Error().Err(err).Str("url", url).Msg("[exec]")
		return nil, err
	}

	chErr := make(chan error)

	go func() {
		err := cmd.Wait()
		// unblocking write to channel
		select {
		case chErr <- err:
		default:
			log.Trace().Str("url", url).Msg("[exec] close")
		}
	}()

	select {
	case <-time.After(time.Second * 60):
		_ = cmd.Process.Kill()
		log.Error().Str("url", url).Msg("[exec] timeout")
		return nil, errors.New("timeout")
	case err := <-chErr:
		return nil, fmt.Errorf("exec: %s", err)
	case prod := <-ch:
		log.Debug().Stringer("launch", time.Since(ts)).Msg("[exec] run")
		return prod, nil
	}
}

// internal

var (
	log        zerolog.Logger
	waiters    = map[string]chan core.Producer{}
	waitersMu  sync.Mutex
	killSignal os.Signal
)
