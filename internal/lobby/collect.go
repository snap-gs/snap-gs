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

// rfc3339nano is a fixed-width version of time.RFC3339Nano.
const rfc3339nano = "2006-01-02T15:04:05.000000000Z07:00"

func (l *Lobby) enqueue(id string, match []byte) {
	const prefix, suffix = `{"@timestamp":"`, `",`
	// Extra -5 drops unused TZ and -1 overwrites '{' with ','.
	const offset = len(prefix) + len(rfc3339nano) + len(suffix) - 6
	defer func() { l.setstat("match", l.mbs) }()
	bs := make([]byte, len(match)+offset)
	copy(bs[offset:], match)
	if l.m.MatchID == id {
		copy(bs[:offset+1], l.mbs)
		l.mbs = bs
		return
	}
	l.collect()
	l.m.MatchID = id
	const layout = "01/02/2006 15:04:05"
	i := strings.Index(l.m.MatchID, l.session)
	if i == -1 || i == len(l.m.MatchID)-len(l.session) {
		l.m.Timestamp = time.Now().UTC()
	} else if t, err := time.Parse(layout, l.m.MatchID[i+len(l.session):]); err != nil {
		l.errorf("enqueue: time.Parse: error: %+v", err)
		l.m.Timestamp = time.Now().UTC()
	} else {
		l.m.Timestamp = t.UTC()
	}
	copy(bs[:offset+1], []byte(prefix+l.m.Timestamp.Format(rfc3339nano)+suffix))
	l.mbs = bs
}

func (l *Lobby) collect() {
	if l.m.MatchID == "" {
		l.m.Timestamp = time.Now().In(time.UTC)
		return
	}
	if err := json.Unmarshal(l.mbs, l.m); err != nil {
		l.errorf("collect: json.Unmarshal: error: %+v id=%s", err, l.m.MatchID)
		return
	}
	if len(l.m.KillData) == 0 {
		l.debugf("collect: discard (empty data): id=%s", l.m.MatchID)
	} else {
		select {
		case l.matches <- l.m:
			l.debugf("collect: id=%s", l.m.MatchID)
		default:
			l.errorf("collect: discard (queue full): id=%s", l.m.MatchID)
		}
	}
	l.m, l.mbs = &match.Match{Timestamp: time.Now().In(time.UTC)}, nil
	l.remstat("match")
}

func (l *Lobby) collector() {
	defer l.mwg.Done()
	defer l.debugf("collector: done")
	l.debugf("collector: logdir=%s clean=%t", l.LogDir, l.LogClean)
	if l.LogDir == "" {
		return
	}
	defer l.cancel()
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
		file := filepath.Join(l.LogDir, ts+"-match.json.gz")
		l.debugf("collector: id=%s file=%s", m.MatchID, file)
		if err := writeMatchFile(m, sm, file); err != nil {
			l.errorf("collector: writeMatchFile: error: %+v id=%s file=%s", err, m.MatchID, file)
			l.cancel()
		}
		if !l.LogClean {
			continue
		}
		m.Anonymize()
		file = filepath.Join(l.LogDir, ts+"-clean.json.gz")
		if err := writeMatchFile(m, sm, file); err != nil {
			l.errorf("collector: writeMatchFile: error: %+v id=%s file=%s", err, m.MatchID, file)
			l.cancel()
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
