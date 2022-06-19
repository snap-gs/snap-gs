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

	Listen  string
	Listen1 string
	Listen2 string

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
	args := append(strings.Split(o.Exe, ","), "-nographics", "-batchmode", "--roomname", o.Session)
	if o.Password != "" {
		args = append(args, "--password", o.Password)
	}
	if o.Listen != "" {
		args = append(args, "--bind-address", o.Listen)
	}
	args[0], err = exec.LookPath(args[0])
	return args[0], args[1:], err
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
