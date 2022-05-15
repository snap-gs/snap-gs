package lobby

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/xattr"
)

type syncmeta struct {
	ContentType        string `json:"content_type,omitempty"`
	ContentDisposition string `json:"content_disposition,omitempty"`
	ContentLanguage    string `json:"content_language,omitempty"`
	ContentEncoding    string `json:"content_encoding,omitempty"`
	CacheControl       string `json:"cache_control,omitempty"`
}

func (l *Lobby) collect() {
	if l.m.id == "" {
		l.m.at = time.Now().In(time.UTC)
		return
	}
	select {
	case l.matches <- l.m:
		l.debugf("collect: id=%s", l.m.id)
	default:
		l.errorf("collect: discard (queue full): id=%s", l.m.id)
	}
	l.m = Match{at: time.Now().In(time.UTC)}
	l.remstat("match")
}

func (l *Lobby) enqueue(id string, match []byte) {
	defer func() { l.setstat("match", l.m.bs) }()
	bs := make([]byte, len(match)+36)
	copy(bs[36:], match)
	if l.m.id == id {
		copy(bs[:37], l.m.bs)
		l.m.bs = bs
		return
	}
	l.collect()
	l.m.id = id
	const layout = "01/02/2006 15:04:05"
	i := strings.Index(l.m.id, l.session)
	if i == -1 || i == len(l.m.id)-len(l.session) {
		l.m.at = time.Now().In(time.UTC)
	} else if t, err := time.ParseInLocation(layout, l.m.id[i+len(l.session):], time.Local); err != nil {
		l.errorf("enqueue: time.ParseInLocation: err=%+v", err)
		l.m.at = time.Now().In(time.UTC)
	} else {
		l.m.at = t.In(time.UTC)
	}
	copy(bs[:37], []byte(`{"@timestamp":"`+l.m.at.Format(time.RFC3339)+`",`))
	l.m.bs = bs
}

func (l *Lobby) collector() {
	defer l.mwg.Done()
	defer l.debugf("collector: done")
	l.debugf("collector: matchdir=%s", l.MatchDir)
	if l.MatchDir == "" {
		return
	}
	defer l.cancel()
	sm, err := json.Marshal(syncmeta{
		ContentType:        "application/json",
		ContentDisposition: "inline",
		ContentLanguage:    "en-US",
		ContentEncoding:    "gzip",
	})
	if err != nil {
		l.errorf("collector: json.Marshal: err=%+v", err)
		return
	}
	for m := range l.matches {
		// Windows does not allow ':' in the filename.
		ts := strings.ReplaceAll(m.at.Format(time.RFC3339Nano), ":", "_")
		file := filepath.Join(l.MatchDir, ts+".match.json.gz")
		l.debugf("collector: id=%s match=%s", m.id, file)
		w, err := os.Create(file + ".lock")
		if err != nil {
			l.errorf("collector: os.Create: err=%+v", err)
			return
		}
		wz := gzip.NewWriter(w)
		errs := make([]error, 5)
		_, errs[0] = wz.Write(m.bs)
		errs[1] = wz.Close()
		errs[2] = w.Close()
		errs[3] = xattr.Set(file+".lock", "user.s3sync.meta", sm)
		errs[4] = os.Rename(file+".lock", file)
		for i := range errs {
			if errs[i] != nil {
				l.errorf("collector: wz.Write: err=%+v", errs[0])
				l.errorf("collector: wz.Close: err=%+v", errs[1])
				l.errorf("collector: w.Close: err=%+v", errs[2])
				l.errorf("collector: xattr.Set: err=%+v", errs[3])
				l.errorf("collector: os.Rename: err=%+v", errs[4])
				l.cancel()
				break
			}
		}
	}
}
