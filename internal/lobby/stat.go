package lobby

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (l *Lobby) newstat(name string) error {
	switch name {
	case "match":
		return l.setstat(name, l.m.Timestamp.UTC())
	default:
		return l.setstat(name, time.Now().UTC())
	}
}

func (l *Lobby) remstats(names ...string) error {
	if l.opts.StatDir != "" && len(names) == 0 {
		dirents, err := os.ReadDir(l.opts.StatDir)
		if err != nil {
			return err
		}
		for i := range dirents {
			if !strings.HasPrefix(dirents[i].Name(), "last") {
				names = append(names, dirents[i].Name())
			}
		}
	}
	for i := range names {
		if err := l.setstat(names[i], nil); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lobby) remstat(name string) error {
	return l.setstat(name, nil)
}

func (l *Lobby) setstat(name string, data interface{}) error {
	if name == "" || strings.HasPrefix(name, "last") {
		return nil
	}
	if (name == "up" || name == "idle") && l.c != nil && l.c.Process != nil && l.c.ProcessState == nil {
		pidfile := strings.Split(l.opts.PidFile, ",")
		var err error
		switch {
		case name == "up" && data != nil && pidfile[1] != "":
			err = os.WriteFile(pidfile[1], []byte(strconv.Itoa(l.c.Process.Pid)), 0o644)
		case name == "idle" && data == nil && len(pidfile) > 2 && pidfile[2] != "":
			err = os.WriteFile(pidfile[1], []byte(strconv.Itoa(l.c.Process.Pid)), 0o644)
		case name == "idle" && data != nil && len(pidfile) > 2 && pidfile[2] != "":
			err = os.WriteFile(pidfile[2], []byte(strconv.Itoa(l.c.Process.Pid)), 0o644)
		}
		if err != nil {
			l.errorf("stat: os.WriteFile: error: %+v", err)
		}
	}
	if l.opts.StatDir == "" {
		return nil
	}
	file := filepath.Join(l.opts.StatDir, name)
	last := filepath.Join(l.opts.StatDir, "last"+name)
	if data == nil {
		if os.Rename(file, last) == nil {
			return nil
		}
		return os.RemoveAll(file)
	}
	var bs []byte
	switch value := data.(type) {
	case []byte:
		bs = value
	default:
		var err error
		if bs, err = json.Marshal(data); err != nil {
			return err
		}
	}
	if err := os.WriteFile(file+".lock", bs, 0o644); err != nil {
		return err
	}
	return os.Rename(file+".lock", file)
}
