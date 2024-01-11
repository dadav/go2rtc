package exec

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
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

type Params struct {
	KillSignal  os.Signal
	Command     string
	KillTimeout time.Duration
}

func parseParams(s string) *Params {
	args := &Params{
		KillSignal:  syscall.SIGKILL,
		KillTimeout: 5 * time.Second,
		Command:     s,
	}

	var query url.Values
	if i := strings.IndexByte(s, '#'); i > 0 {
		query = streams.ParseQuery(s[i+1:])
		args.Command = s[:i]
	}

	if val, ok := query["killsignal"]; ok {
		if sig, err := parseSignal(val[0]); err == nil {
			args.KillSignal = sig
		} else {
			log.Error().Err(err).Str("signal", val[0]).Msg("[exec] bad value")
			panic(err)
		}
	}

	if val, ok := query["killtimeout"]; ok {
		if i, err := strconv.Atoi(val[0]); err == nil {
			args.KillTimeout = time.Duration(i) * time.Second
		}
	}

	return args
}

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

func Init() {
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

	params := parseParams(url)
	args := shell.QuoteSplit(params.Command[5:]) // remove `exec:`
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
		return handlePipe(url, cmd, params)
	}

	return handleRTSP(url, path, cmd)
}

func handlePipe(_ string, cmd *exec.Cmd, params *Params) (core.Producer, error) {
	r, err := PipeCloser(cmd, params)
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
	log       zerolog.Logger
	waiters   = map[string]chan core.Producer{}
	waitersMu sync.Mutex
)
