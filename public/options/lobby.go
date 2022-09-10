package options

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/snap-gs/snap-gs/internal/watch"
)

type Lobby struct {
	Debug bool

	Listen   string
	Session  string
	Password string

	LogDir  string
	SpecDir string
	StatDir string
	PidFile string

	Exe string

	Timeout      time.Duration
	AdminTimeout time.Duration

	MaxFails  int
	MinUptime time.Duration
}

const (
	ExeMinLen     = 1
	SessionMinLen = 1
	SessionMaxLen = 40
	MaxFailsMin   = 0
)

var (
	ErrExeMinLen     = errors.New(fmt.Sprintf("exe length must be %d or more", ExeMinLen))
	ErrSessionMinLen = errors.New(fmt.Sprintf("session length must be %d or more", SessionMinLen))
	ErrSessionMaxLen = errors.New(fmt.Sprintf("session length must be %d or less", SessionMaxLen))
	ErrMaxFailsMin   = errors.New(fmt.Sprintf("maxfails must be %d or more", MaxFailsMin))
)

func (o Lobby) Copy() *Lobby {
	return &o
}

func (o *Lobby) Validate() error {
	switch {
	case len(o.Exe) < ExeMinLen:
		return ErrExeMinLen
	case len(o.Session) < SessionMinLen:
		return ErrSessionMinLen
	case len(o.Session) > SessionMaxLen:
		return ErrSessionMaxLen
	case o.MaxFails < MaxFailsMin:
		return ErrMaxFailsMin
	default:
		return nil
	}
}

func (o *Lobby) Watch(ctx context.Context, path string) (func(), error) {
	in := *o
	update := func(key string, value []byte) {
		line := len(value) != 0 && value[len(value)-1] == '\n'
		if line {
			value = value[:len(value)-1]
		}
		switch key {
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
		case "exe":
			switch {
			case len(value) == 0:
				o.Exe = in.Exe
			case line:
				o.Exe = string(value)
			default:
				_ = json.Unmarshal(value, &o.Exe)
			}
		case "pidfile":
			switch {
			case len(value) == 0:
				o.PidFile = in.PidFile
			case line:
				o.PidFile = string(value)
			default:
				_ = json.Unmarshal(value, &o.PidFile)
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
		&watch.Options{
			Path: path,
			Tick: 200 * time.Millisecond,
		},
		watch.FilterSameNames,
		func(events []watch.Event, err error) ([]watch.Event, error) {
			for _, event := range events {
				if bs, _ := os.ReadFile(filepath.Join(path, event.Name)); len(bs) != 0 {
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
