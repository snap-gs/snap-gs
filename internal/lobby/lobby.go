package lobby

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/snap-gs/snap-gs/internal/log"
	"github.com/snap-gs/snap-gs/internal/match"
)

var (
	ErrLobbyNil          = errors.New("lobby nil")
	ErrLobbyTimeout      = errors.New("lobby timeout")
	ErrLobbyIdleTimeout  = errors.New("lobby idle timeout")
	ErrLobbyAdminTimeout = errors.New("lobby admin timeout")
	ErrLobbyDisconnected = errors.New("lobby disconnected")
	ErrLobbyRestarted    = errors.New("lobby restarted")
	ErrLobbyStopped      = errors.New("lobby stopped")
	ErrLobbyDowned       = errors.New("lobby downed")
)

type Lobby struct {
	Debug bool

	SpecDir string
	StatDir string

	LogDir   string
	LogClean bool

	Timeout      time.Duration
	AdminTimeout time.Duration

	MaxIdles  int
	MinUptime time.Duration

	arena   string
	session string
	changed bool
	idles   int

	c     *exec.Cmd
	p     *process.Process
	cwg   sync.WaitGroup
	prout *os.File
	pwout *os.File
	prerr *os.File
	pwerr *os.File

	m       *match.Match
	mbs     []byte
	mwg     sync.WaitGroup
	matches chan *match.Match
	players Players

	stdx   sync.Mutex
	stdout io.Writer
	stderr io.Writer

	x      sync.Mutex
	t1     time.Time
	t2     time.Time
	ctx    context.Context
	cancel func()
	reason error
}

const pipesz = 1 << 20 // 1MiB

var setpipesz = func(uintptr) error {
	return nil
}

func (l *Lobby) Run(ctx context.Context, stdout, stderr io.Writer, exe string, args ...string) error {
	if l == nil {
		return ErrLobbyNil
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
	// Early opportunistic check without lock.
	if err := ctx.Err(); err != nil {
		return err
	}
	l.x.Lock()
	defer l.x.Unlock()
	// Checked again after acquiring lock.
	if err := ctx.Err(); err != nil {
		return err
	}
	return l.runc(ctx, stdout, stderr, exe, args...)
}

func (l *Lobby) Stop() error {
	if l == nil || l.cancel == nil {
		return ErrLobbyNil
	}
	l.cancel()
	l.x.Lock()
	defer l.x.Unlock()
	return l.Err()
}

func (l *Lobby) Err() error {
	if l == nil {
		return ErrLobbyNil
	}
	// -MaxIdles ignores timeout errors.
	if l.MaxIdles >= 0 {
		return l.reason
	}
	switch l.reason {
	case ErrLobbyTimeout, ErrLobbyIdleTimeout, ErrLobbyAdminTimeout, ErrLobbyRestarted, ErrLobbyStopped, ErrLobbyDowned:
		return nil
	default:
		return l.reason
	}
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

func (l *Lobby) alloc(ctx context.Context, stdout, stderr io.Writer, exe string, args ...string) error {
	var err error
	l.prout, l.pwout, err = os.Pipe()
	if err != nil {
		return err
	}
	if err := setpipesz(l.prout.Fd()); err != nil {
		_, _ = l.prout.Close(), l.pwout.Close()
		return err
	}
	l.prerr, l.pwerr, err = os.Pipe()
	if err != nil {
		_, _ = l.prout.Close(), l.pwout.Close()
		return err
	}
	if err := setpipesz(l.prerr.Fd()); err != nil {
		_, _ = l.prout.Close(), l.pwout.Close()
		_, _ = l.prerr.Close(), l.pwerr.Close()
		return err
	}
	// Committed to run from here.
	l.session, l.players = "", Players{}
	l.reason, l.matches = nil, make(chan *match.Match, 10)
	// Empty 'id' with nonempty 'at' time informs idle lobby watchers of the
	// most-recent push time when no match is currently in progress.
	l.m, l.mbs = &match.Match{Timestamp: time.Now().In(time.UTC)}, nil
	l.t1, l.t2 = time.Time{}, time.Time{}
	l.ctx, l.cancel = context.WithCancel(ctx)
	l.p, l.c = nil, exec.CommandContext(l.ctx, exe, args...)
	l.c.Stdout, l.c.Stderr = l.pwout, l.pwerr
	l.stdout, l.stderr = stdout, stderr
	return nil
}

func (l *Lobby) runc(ctx context.Context, stdout, stderr io.Writer, exe string, args ...string) error {
	defer l.remstats()
	if err := l.alloc(ctx, stdout, stderr, exe, args...); err != nil {
		return err
	}
	l.mwg.Add(2)
	go l.collector()
	go l.collector()
	l.cwg.Add(3)
	go l.watcher()
	go l.scanner(1)
	go l.scanner(2)
	defer l.mwg.Wait()
	defer l.cwg.Wait()
	defer l.pwerr.Close()
	defer l.pwout.Close()
	defer l.cancel()
	l.debugf("runc: c=%s", l.c)
	l.t1 = time.Now()
	err := l.c.Start()
	if err == nil {
		l.newstat("up")
		defer l.remstat("up")
		l.p, err = process.NewProcess(int32(l.c.Process.Pid))
		if err == nil {
			err = l.c.Wait()
		} else {
			_ = l.c.Process.Kill()
			_ = l.c.Wait()
		}
	}
	if l.reason == nil {
		l.reason = err
	}
	l.t2 = time.Now()
	return l.Err()
}

func (l *Lobby) errorf(format string, a ...interface{}) {
	if l.Debug {
		l.debugf(format, a...)
		return
	}
	l.stdx.Lock()
	defer l.stdx.Unlock()
	log.Errorf(l.stderr, "Lobby."+format, a...)
}

func (l *Lobby) debugf(format string, a ...interface{}) {
	if !l.Debug {
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
