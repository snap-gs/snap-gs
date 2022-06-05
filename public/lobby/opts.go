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

	Listen string

	Session  string
	Password string

	LogDir  string
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
	SessionMaxLen = 26
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

func (o *Options) BindAddrPort() (string, string, string) {
	bind := o.Listen
	if !strings.Contains(bind, ":") {
		bind += ":"
	}
	if strings.HasSuffix(bind, ":") {
		bind += "0"
	}
	if strings.HasPrefix(bind, ":") {
		bind = "0.0.0.0" + bind
	}
	i := strings.Index(bind, ":")
	return bind, bind[:i], bind[i+1:]
}

func (o *Options) ExeArgs() (string, []string, error) {
	err := o.Validate()
	if err != nil {
		return "", nil, err
	}
	bind, _, _ := o.BindAddrPort()
	args := append(strings.Split(o.Exe, ","), "-nographics", "-batchmode", "--bind-address", bind, "--roomname", o.Session)
	if o.Password != "" {
		args = append(args, "--password", o.Password)
	}
	args[0], err = exec.LookPath(args[0])
	return args[0], args[1:], err
}
