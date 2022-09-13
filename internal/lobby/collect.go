package lobby

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/xattr"
	"github.com/snap-gs/snap-gs/internal/match"
	"github.com/snap-gs/snap-gs/internal/sync"
)

func (l *Lobby) collect() {
	if l.m.MatchID == "" {
		l.m.Timestamp = time.Now().UTC()
		return
	}
	defer l.remstat("match")
	if len(l.m.KillData) == 0 {
		l.debugf("collect: discard (empty data): id=%s", l.m.MatchID)
		l.m = &match.Match{Timestamp: time.Now().UTC()}
		return
	}
	select {
	case l.matches <- l.m:
		l.debugf("collect: id=%s", l.m.MatchID)
	default:
		l.errorf("collect: discard (queue full): id=%s", l.m.MatchID)
	}
	l.m = &match.Match{Timestamp: time.Now().UTC()}
}

func (l *Lobby) collector() {
	defer l.wg.Done()
	defer l.debugf("collector: done")
	l.debugf("collector: logdir=%s", l.opts.LogDir)
	if l.opts.LogDir == "" {
		return
	}
	defer l.Cancel(ErrLobbyDone)
	for m := range l.matches {
		sm := &sync.Meta{
			ContentType:        "application/json",
			ContentDisposition: "inline",
			ContentLanguage:    "en-US",
			ContentEncoding:    "gzip",
			Metadata: map[string]string{
				"lobby": l.session,
			},
		}
		if fields := strings.Fields(l.session); len(fields) == 4 {
			switch fields[0] {
			case "VRML", "VXL":
				sm.Metadata["assoc"] = fields[0]
				sm.Metadata["team0"] = fields[1]
				sm.Metadata["team1"] = fields[3]
			}
		}
		m.Normalize()
		// Windows does not allow ':' in the filename.
		ts := strings.ReplaceAll(m.Timestamp.Format(time.RFC3339Nano), ":", "_")
		file := filepath.Join(l.opts.LogDir, ts+"-match.json.gz")
		l.debugf("collector: id=%s file=%s", m.MatchID, file)
		if err := writeMatchFile(m, sm, file); err != nil {
			l.errorf("collector: writeMatchFile: error: %+v id=%s file=%s", err, m.MatchID, file)
			l.Cancel(ErrLobbyBad)
		}
		m.Anonymize()
		file = filepath.Join(l.opts.LogDir, ts+"-clean.json.gz")
		if err := writeMatchFile(m, sm, file); err != nil {
			l.errorf("collector: writeMatchFile: error: %+v id=%s file=%s", err, m.MatchID, file)
			l.Cancel(ErrLobbyBad)
		}
	}
}

func writeMatchFile(m *match.Match, sm *sync.Meta, file string) error {
	bs, err := json.Marshal(sm)
	if err != nil {
		return err
	}
	lock := file + ".lock"
	w, err := os.Create(lock)
	if err != nil {
		return err
	}
	defer os.Rename(lock, file)
	defer xattr.Set(lock, "user.s3sync.meta", bs)
	defer w.Close()
	wz := gzip.NewWriter(w)
	defer wz.Close()
	bs, err = json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = wz.Write(bs)
	return err
}
