package lobby

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/xattr"
	"github.com/snap-gs/snap-gs/internal/sync"
)

func (l *Lobby) newstat(name string) error {
	switch name {
	case "match":
		return l.setstat(name, l.m.Timestamp.UTC().Format(time.RFC3339))
	default:
		return l.setstat(name, time.Now().UTC().Format(time.RFC3339))
	}
}

func (l *Lobby) remstats(names ...string) error {
	if l.StatDir != "" && len(names) == 0 {
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

func (l *Lobby) setstat(name string, data interface{}) error {
	if name == "" || strings.HasPrefix(name, "last") {
		return nil
	}
	l.updatestate(l.states.Stat, name, data)
	if l.StatDir == "" {
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
	var bs []byte
	switch value := data.(type) {
	case []byte:
		bs = value
	case string:
		bs = []byte(value)
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

func (l *Lobby) updatestate(state map[string]interface{}, name string, data interface{}) {
	l.states.x.Lock()
	defer l.states.x.Unlock()
	l.states.Timestamp = time.Now().UTC().Truncate(time.Second)
	if data != nil {
		state[name] = data
	} else {
		delete(state, name)
	}
}

func (l *Lobby) hasstate(state map[string]interface{}, name string) bool {
	l.states.x.Lock()
	defer l.states.x.Unlock()
	_, ok := state[name]
	return ok
}

func (l *Lobby) loadstate() error {
	if l.SpecDir == "" {
		return nil
	}
	dirents, err := os.ReadDir(l.SpecDir)
	if err != nil {
		return err
	}
	for i := range dirents {
		fi, err := dirents[i].Info()
		if err != nil {
			l.errorf("loadstate: error: %+v", err)
			continue
		}
		if strings.HasPrefix(dirents[i].Name(), "last") {
			continue
		}
		if strings.HasSuffix(dirents[i].Name(), ".lock") {
			continue
		}
		if dirents[i].Name() != "restart" || l.t1.Before(fi.ModTime()) {
			l.updatestate(l.states.Spec, dirents[i].Name(), fi.ModTime().UTC().Format(time.RFC3339))
		} else {
			l.updatestate(l.states.Spec, dirents[i].Name(), nil)
		}
	}
	return nil
}

func (l *Lobby) savestate() {
	if l.session == "" || l.LogDir == "" || !l.LogState {
		return
	}
	sm := &sync.Meta{
		ContentType:        "application/json",
		ContentDisposition: "inline",
		ContentLanguage:    "en-US",
		ContentEncoding:    "gzip",
		Metadata: map[string]string{
			"lobby": l.session,
		},
	}
	bs, err := json.Marshal(sm)
	if err != nil {
		l.errorf("savestate: json.Marshal: error: %+v", err)
		return
	}
	lock := filepath.Join(l.LogDir, "state.json.gz.lock")
	file := filepath.Join(l.LogDir, "state.json.gz")
	w, err := os.Create(lock)
	if err != nil {
		l.errorf("savestate: os.Create: error: %+v", err)
		return
	}
	defer os.Rename(lock, file)
	defer xattr.Set(lock, "user.s3sync.meta", bs)
	defer w.Close()
	wz := gzip.NewWriter(w)
	defer wz.Close()
	if bs, err = json.Marshal(&l.states); err != nil {
		l.errorf("savestate: json.Marshal: error: %+v", err)
	} else if _, err = wz.Write(bs); err != nil {
		l.errorf("savestate: wz.Write: error: %+v", err)
	}
}
