package lobby

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (l *Lobby) newstat(name string) error {
	switch name {
	case "up", "idle", "full":
		return l.setstat(name, strconv.AppendInt(nil, int64(l.c.Process.Pid), 10))
	default:
		return l.setstat(name, []byte{})
	}
}

func (l *Lobby) remstats(names ...string) error {
	if l.StatDir == "" {
		return nil
	}
	if len(names) == 0 {
		dirents, err := os.ReadDir(l.StatDir)
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

func (l *Lobby) setstat(name string, data []byte) error {
	if l.StatDir == "" || name == "" || strings.HasPrefix(name, "last") {
		return nil
	}
	file := filepath.Join(l.StatDir, name)
	last := filepath.Join(l.StatDir, "last"+name)
	if data == nil {
		if os.Rename(file, last) == nil {
			return nil
		}
		return os.RemoveAll(file)
	}
	if err := os.WriteFile(file+".lock", data, 0o644); err != nil {
		return err
	}
	return os.Rename(file+".lock", file)
}
