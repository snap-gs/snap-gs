package lobby

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Options struct {
	Debug bool

	Roomname string
	Password string

	LogDir   string
	MatchDir string
	SpecDir  string
	StatDir  string

	Exe string

	Timeout      time.Duration
	AdminTimeout time.Duration

	MaxIdles    int
	MaxFails    int
	MinUptime   time.Duration
	MinUpUptime time.Duration
}

const (
	ExeMinLen      = 1
	RoomnameMinLen = 1
	RoomnameMaxLen = 26
)

var (
	ErrExeMinLen      = errors.New(fmt.Sprintf("exe length must be %d or more", ExeMinLen))
	ErrRoomnameMinLen = errors.New(fmt.Sprintf("roomname length must be %d or more", RoomnameMinLen))
	ErrRoomnameMaxLen = errors.New(fmt.Sprintf("roomname length must be %d or less", RoomnameMaxLen))
)

func (o *Options) Validate() error {
	switch {
	case len(o.Exe) < ExeMinLen:
		return ErrExeMinLen
	case len(o.Roomname) < RoomnameMinLen:
		return ErrRoomnameMinLen
	case len(o.Roomname) > RoomnameMaxLen:
		return ErrRoomnameMaxLen
	default:
		return nil
	}
}

func (o *Options) ExeArgs() (string, []string, error) {
	err := o.Validate()
	if err != nil {
		return "", nil, err
	}
	args := append(strings.Split(o.Exe, ","), "-nographics", "-batchmode", "--roomname", o.Roomname)
	if o.Password != "" {
		args = append(args, "--password", o.Password)
	}
	args[0], err = exec.LookPath(args[0])
	return args[0], args[1:], err
}
