package lobby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/snap-gs/snap-gs/internal/watch"
)

type Options struct {
	Debug bool

	Listen  string
	Listen1 string
	Listen2 string

	Session  string
	Password string

	LogDir  string
	FlagDir string
	SpecDir string
	StatDir string

	LogState bool
	LogMatch bool
	LogClean bool

	Exe string

	Timeout      time.Duration
	AdminTimeout time.Duration

	MaxIdles    int
	MaxFails    int
	MinUptime   time.Duration
	MinUpUptime time.Duration
}

const (
	ExeMinLen     = 1
	SessionMinLen = 1
	SessionMaxLen = 40
)

var (
	ErrExeMinLen     = errors.New(fmt.Sprintf("exe length must be %d or more", ExeMinLen))
	ErrSessionMinLen = errors.New(fmt.Sprintf("session length must be %d or more", SessionMinLen))
	ErrSessionMaxLen = errors.New(fmt.Sprintf("session length must be %d or less", SessionMaxLen))
)

func (o *Options) Validate() error {
	switch {
	case len(o.Exe) < ExeMinLen:
		return ErrExeMinLen
	case len(o.Session) < SessionMinLen:
		return ErrSessionMinLen
	case len(o.Session) > SessionMaxLen:
		return ErrSessionMaxLen
	default:
		return nil
	}
}

func (o *Options) ExeArgs() (string, []string, error) {
	err := o.Validate()
	if err != nil {
		return "", nil, err
	}
	args := append(
		strings.Split(o.Exe, ","),
		"-nographics", "-batchmode",
		"--roomname", strings.ReplaceAll(o.Session, " ", "\u00a0"),
	)
	if o.Password != "" {
		args = append(args, "--password", o.Password)
	}
	if o.Listen != "" {
		args = append(args, "--bind-address", o.Listen)
	}
	args[0], err = exec.LookPath(args[0])
	return args[0], args[1:], err
}

func (o *Options) WatchFlagDir(ctx context.Context) (func(), error) {
	in := *o
	update := func(key string, value []byte) {
		line := len(value) != 0 && value[len(value)-1] == '\n'
		if line {
			value = value[:len(value)-1]
		}
		switch key {
		case "flagdir":
			// Cannot set --flagdir from within flagdir.
		case "session":
			switch {
			case len(value) == 0:
				o.Session = in.Session
			case line:
				o.Session = string(value)
			default:
				_ = json.Unmarshal(value, &o.Session)
			}
		case "password":
			switch {
			case len(value) == 0:
				o.Password = in.Password
			case line:
				o.Password = string(value)
			default:
				_ = json.Unmarshal(value, &o.Password)
			}
		case "specdir":
			switch {
			case len(value) == 0:
				o.SpecDir = in.SpecDir
			case line:
				o.SpecDir = string(value)
			default:
				_ = json.Unmarshal(value, &o.SpecDir)
			}
		case "statdir":
			switch {
			case len(value) == 0:
				o.StatDir = in.StatDir
			case line:
				o.StatDir = string(value)
			default:
				_ = json.Unmarshal(value, &o.StatDir)
			}
		case "logdir":
			switch {
			case len(value) == 0:
				o.LogDir = in.LogDir
			case line:
				o.LogDir = string(value)
			default:
				_ = json.Unmarshal(value, &o.LogDir)
			}
		case "logstate":
			switch {
			case len(value) == 0:
				o.LogState = in.LogState
			default:
				_ = json.Unmarshal(value, &o.LogState)
			}
		case "logmatch":
			switch {
			case len(value) == 0:
				o.LogMatch = in.LogMatch
			default:
				_ = json.Unmarshal(value, &o.LogMatch)
			}
		case "logclean":
			switch {
			case len(value) == 0:
				o.LogClean = in.LogClean
			default:
				_ = json.Unmarshal(value, &o.LogClean)
			}
		case "maxidles":
			switch {
			case len(value) == 0:
				o.MaxIdles = in.MaxIdles
			default:
				_ = json.Unmarshal(value, &o.MaxIdles)
			}
		case "maxfails":
			switch {
			case len(value) == 0:
				o.MaxFails = in.MaxFails
			default:
				_ = json.Unmarshal(value, &o.MaxFails)
			}
		case "minuptime":
			switch {
			case len(value) == 0:
				o.MinUptime = in.MinUptime
			case line:
				o.MinUptime, _ = time.ParseDuration(string(value))
			default:
				_ = json.Unmarshal(value, &o.MinUptime)
			}
		case "minupuptime":
			switch {
			case len(value) == 0:
				o.MinUpUptime = in.MinUpUptime
			case line:
				o.MinUpUptime, _ = time.ParseDuration(string(value))
			default:
				_ = json.Unmarshal(value, &o.MinUpUptime)
			}
		case "admintimeout":
			switch {
			case len(value) == 0:
				o.AdminTimeout = in.AdminTimeout
			case line:
				o.AdminTimeout, _ = time.ParseDuration(string(value))
			default:
				_ = json.Unmarshal(value, &o.AdminTimeout)
			}
		case "timeout":
			switch {
			case len(value) == 0:
				o.Timeout = in.Timeout
			case line:
				o.Timeout, _ = time.ParseDuration(string(value))
			default:
				_ = json.Unmarshal(value, &o.Timeout)
			}
		case "listen":
			switch {
			case len(value) == 0:
				o.Listen = in.Listen
			case line:
				o.Listen = string(value)
			default:
				_ = json.Unmarshal(value, &o.Listen)
			}
		case "listen1":
			switch {
			case len(value) == 0:
				o.Listen1 = in.Listen1
			case line:
				o.Listen1 = string(value)
			default:
				_ = json.Unmarshal(value, &o.Listen1)
			}
		case "listen2":
			switch {
			case len(value) == 0:
				o.Listen2 = in.Listen2
			case line:
				o.Listen2 = string(value)
			default:
				_ = json.Unmarshal(value, &o.Listen2)
			}
		case "exe":
			switch {
			case len(value) == 0:
				o.Exe = in.Exe
			case line:
				o.Exe = string(value)
			default:
				_ = json.Unmarshal(value, &o.Exe)
			}
		case "debug":
			switch {
			case len(value) == 0:
				o.Debug = in.Debug
			default:
				_ = json.Unmarshal(value, &o.Debug)
			}
		}
	}
	return watch.Watcher(
		ctx,
		watch.Options{
			Path: o.FlagDir,
			Tick: 200 * time.Millisecond,
		},
		watch.Duplicates,
		func(events []watch.Event, err error) ([]watch.Event, error) {
			for _, event := range events {
				if bs, _ := os.ReadFile(filepath.Join(o.FlagDir, event.Name)); len(bs) != 0 {
					update(event.Name, bs)
				} else {
					update(event.Name, nil)
				}
			}
			if events == nil && err == nil {
				*o = in
			}
			return events, err
		},
	)
}

func BindAddrPort(bind, addr, port string) (string, string, string) {
	const colon = ":"
	if !strings.Contains(bind, colon) {
		bind += colon
	}
	if bind[0] == colon[0] {
		bind = addr + bind
	}
	if bind[len(bind)-1] == colon[0] {
		bind += port
	}
	if bind == colon {
		return "", "", ""
	}
	i := strings.IndexByte(bind, colon[0])
	return bind, bind[:i], bind[i+1:]
}
