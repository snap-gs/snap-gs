package lobby

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/snap-gs/snap-gs/internal/log"
	"github.com/snap-gs/snap-gs/internal/match"
	"github.com/snap-gs/snap-gs/public/options"
)

var (
	ErrLobbyBad          = errors.New("lobby bad")
	ErrLobbyDone         = errors.New("lobby done")
	ErrLobbyIdleTimeout  = errors.New("lobby idle timeout")
	ErrLobbyDisconnected = errors.New("lobby disconnected")
	ErrLobbyMaxFails     = errors.New("lobby max fails")
)

type Lobby struct {
	arena   string
	session string
	changed bool

	opts *options.Lobby
	spec Spec

	c     *exec.Cmd
	prout *os.File
	pwout *os.File
	prerr *os.File
	pwerr *os.File

	m       *match.Match
	matches chan *match.Match
	players Players

	stdx   sync.Mutex
	stdout io.Writer
	stderr io.Writer

	t1     time.Time
	t2     time.Time
	wg     sync.WaitGroup
	runx   sync.Mutex
	errx   sync.Mutex
	done   chan struct{}
	reason error
}

const pipesz = 1 << 20 // 1MiB

var setpipesz = func(uintptr) error {
	return nil
}

func Run(ctx context.Context, opts *options.Lobby, stdout, stderr io.Writer) (*Lobby, error) {
	if opts == nil {
		opts = &options.Lobby{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l := Lobby{
		opts:   opts,
		stdout: stdout,
		stderr: stderr,
	}
	l.runx.Lock()
	defer l.runx.Unlock()
	l.debugf("Run: opts: %+v", opts)
	return &l, l.runc(ctx)
}

func (l *Lobby) Uptime() time.Duration {
	if l == nil || l.t1.IsZero() {
		return 0
	}
	if l.t2.IsZero() {
		return time.Since(l.t1)
	}
	return l.t2.Sub(l.t1)
}

func (l *Lobby) Cancel(reason error) error {
	if l == nil {
		return ErrLobbyBad
	}
	if reason == nil {
		return l.reason
	}
	l.errx.Lock()
	defer l.errx.Unlock()
	if l.reason == nil {
		l.debugf("Cancel: reason: %+v", reason)
		if l.done != nil {
			defer close(l.done)
		}
	} else if l.reason != ErrLobbyDone {
		if reason != ErrLobbyDone {
			l.errorf("Cancel: error: %+v (%+v)", reason, l.reason)
		}
		return l.reason
	}
	l.reason = reason
	if reason == ErrLobbyDone {
		reason = nil
	}
	if l.c == nil || l.c.Process == nil || l.c.ProcessState != nil {
		return reason
	}
	p, err := process.NewProcess(int32(l.c.Process.Pid))
	if err != nil {
		l.debugf("Cancel: process.NewProcess: error: %+v pid=%d", err, l.c.Process.Pid)
		if err := l.c.Process.Kill(); err != nil {
			l.debugf("Cancel: l.c.Process.Kill: error: %+v pid=%d", err, l.c.Process.Pid)
		}
		return reason
	}
	if err := p.Terminate(); err != nil {
		l.debugf("Cancel: p.Terminate: error: %+v pid=%d", err, l.c.Process.Pid)
	}
	return reason
}

func (l *Lobby) alloc(ctx context.Context) (func(), error) {
	var err error
	args := append(
		strings.Split(l.opts.Exe, ","),
		"-nographics", "-batchmode",
		"--roomname", strings.ReplaceAll(l.opts.Session, " ", "\u00a0"),
	)
	if l.opts.LogDir != "" {
		args = append(args, "-logMatchData")
	}
	if l.opts.Password != "" {
		args = append(args, "--password", l.opts.Password)
	}
	if l.opts.Listen != "" {
		listen := strings.Split(l.opts.Listen, ",")
		if listen[0] != "" {
			args = append(args, "--bind-address", listen[0])
		}
	}
	if args[0], err = exec.LookPath(args[0]); err != nil {
		return nil, err
	}
	if pidfile := strings.Split(l.opts.PidFile, ","); pidfile[0] != "" {
		if err := os.WriteFile(pidfile[0], []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
			return nil, err
		}
	}
	l.prout, l.pwout, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	if err := setpipesz(l.pwout.Fd()); err != nil {
		_, _ = l.prout.Close(), l.pwout.Close()
		return nil, err
	}
	l.prerr, l.pwerr, err = os.Pipe()
	if err != nil {
		_, _ = l.prout.Close(), l.pwout.Close()
		return nil, err
	}
	if err := setpipesz(l.prerr.Fd()); err != nil {
		_, _ = l.prout.Close(), l.pwout.Close()
		_, _ = l.prerr.Close(), l.pwerr.Close()
		return nil, err
	}
	specdone := func() {}
	if l.opts.SpecDir != "" {
		specdone, err = l.spec.Watch(ctx, l.opts.SpecDir)
		if err != nil {
			_, _ = l.prout.Close(), l.pwout.Close()
			_, _ = l.prerr.Close(), l.pwerr.Close()
			return nil, err
		}
		l.debugf("alloc: spec: %+v", l.spec)
	}
	timer := func() { l.t2 = time.Now().UTC() }
	done := func() { specdone(); timer() }
	var outfile *os.File
	if l.opts.LogDir != "" && l.opts.Debug {
		file := filepath.Join(l.opts.LogDir, "Player.log")
		prev := filepath.Join(l.opts.LogDir, "Player-prev.log")
		if outfile, err = os.Create(file); err != nil {
			_, _ = l.prout.Close(), l.pwout.Close()
			_, _ = l.prerr.Close(), l.pwerr.Close()
			specdone()
			return nil, err
		}
		done = func() {
			outfile.Close()
			os.Rename(file, prev)
			specdone()
			timer()
		}
	}
	// Committed to run from here.
	l.done = make(chan struct{})
	l.session, l.players = "", Players{}
	l.reason, l.matches = nil, make(chan *match.Match, 10)
	// Empty 'id' with nonempty 'at' time informs idle lobby watchers of the
	// most-recent push time when no match is currently in progress.
	l.m = &match.Match{Timestamp: time.Now().UTC()}
	l.t1, l.t2 = time.Now().UTC(), time.Time{}
	l.c = exec.CommandContext(ctx, args[0], args[1:]...)
	l.c.Stdout, l.c.Stderr = l.pwout, l.pwerr
	if outfile != nil {
		l.c.Stdout = io.MultiWriter(l.pwout, outfile)
	}
	self, err := os.Executable()
	if err != nil {
		return done, nil
	}
	preload := self + "-preload.so"
	if _, err := os.Stat(preload); err != nil {
		return done, nil
	}
	listen := strings.Split(l.opts.Listen, ",")
	if len(listen) != 3 || listen[0] == "" || listen[1] == "" || listen[2] == "" {
		return done, nil
	}
	l.c.Env = append(
		os.Environ(),
		"LD_PRELOAD="+preload,
		"SNAPGS_LOBBY_LISTEN="+listen[0],
		"SNAPGS_LOBBY_LISTEN1="+listen[1],
		"SNAPGS_LOBBY_LISTEN2="+listen[2],
	)
	l.debugf("alloc: preload=%s listen=%s", preload, l.opts.Listen)
	return done, nil
}

func (l *Lobby) runc(ctx context.Context) error {
	done, err := l.alloc(ctx)
	if err != nil {
		return l.Cancel(err)
	}
	defer done()
	l.remstats()
	defer l.remstats()
	l.wg.Add(5)
	go l.collector()
	go l.collector()
	go l.watcher(ctx)
	go l.scanner(1)
	go l.scanner(2)
	defer l.wg.Wait()
	defer l.pwerr.Close()
	defer l.pwout.Close()
	defer l.Cancel(ErrLobbyDone)
	l.debugf("runc: c=%s", l.c)
	if err := l.c.Start(); err != nil {
		return l.Cancel(err)
	}
	l.newstat("up")
	defer l.remstat("up")
	return l.Cancel(l.c.Wait())
}

func (l *Lobby) errorf(format string, a ...interface{}) {
	if l.opts.Debug {
		l.debugf(format, a...)
		return
	}
	l.stdx.Lock()
	defer l.stdx.Unlock()
	log.Errorf(l.stderr, "Lobby."+format, a...)
}

func (l *Lobby) debugf(format string, a ...interface{}) {
	if !l.opts.Debug {
		return
	}
	l.stdx.Lock()
	defer l.stdx.Unlock()
	log.Debugf(l.stderr, "Lobby."+format, a...)
}

func (l *Lobby) infof(format string, a ...interface{}) {
	l.stdx.Lock()
	defer l.stdx.Unlock()
	log.Infof(l.stderr, "Lobby."+format, a...)
}

func (l *Lobby) logvout(bs []byte) {
	l.stdx.Lock()
	defer l.stdx.Unlock()
	log.Logv(l.stdout, log.N1, bs)
}

func (l *Lobby) logverr(bs []byte) {
	l.stdx.Lock()
	defer l.stdx.Unlock()
	log.Logv(l.stderr, log.N2, bs)
}
