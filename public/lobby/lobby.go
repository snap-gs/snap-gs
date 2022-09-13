package lobby

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/xattr"
	"github.com/snap-gs/snap-gs/internal/lobby"
	"github.com/snap-gs/snap-gs/internal/log"
	"github.com/snap-gs/snap-gs/internal/sync"
	"github.com/snap-gs/snap-gs/public/options"
)

func runc(ctx context.Context, opts *options.Lobby, stdout, stderr io.Writer) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	sm, err := json.Marshal(sync.Meta{
		ContentType:        "text/plain",
		ContentDisposition: "inline",
		ContentLanguage:    "en-US",
		ContentEncoding:    "gzip",
		Metadata: map[string]string{
			"lobby": opts.Session,
		},
	})
	if err != nil {
		return err
	}
	if opts.LogDir != "" {
		// Windows does not allow ':' in the filename.
		ts := strings.ReplaceAll(time.Now().In(time.UTC).Format(time.RFC3339), ":", "_")
		file := filepath.Join(opts.LogDir, ts+"-lobby.log.gz")
		w, err := os.Create(file + ".lock")
		if err != nil {
			return err
		}
		// TODO: Errors.
		defer os.Rename(file+".lock", file)
		defer xattr.Set(file+".lock", "user.s3sync.meta", sm)
		defer w.Close()
		wgz := gzip.NewWriter(w)
		defer wgz.Close()
		stdout = wgz
		stderr = io.MultiWriter(stderr, stdout)
		if opts.Debug {
			log.Debugf(stderr, "runc: stdout: %s", file)
		}
		file = filepath.Join(opts.LogDir, "lobby.log")
		w, err = os.Create(file)
		if err != nil {
			return err
		}
		// TODO: Errors.
		defer w.Close()
		stdout = io.MultiWriter(w, stdout)
		if opts.Debug {
			log.Debugf(stderr, "runc: stdout: %s", file)
		}
	}
	l, err := lobby.Run(ctx, opts, stdout, stderr)
	log.Errorf(stderr, "runc: error: %+v uptime=%s", err, l.Uptime())
	return err
}

func Run(ctx context.Context, opts *options.Lobby, stdout, stderr io.Writer) error {
	var runs, fails int
	const floor = 15 * time.Second
	for ctx.Err() == nil {
		runs++
		t := time.Now()
		err := runc(ctx, opts.Copy(), stdout, stderr)
		switch err {
		case nil, lobby.ErrLobbyIdleTimeout, lobby.ErrLobbyAdminTimeout:
		case lobby.ErrLobbyDowned, lobby.ErrLobbyRestarted, lobby.ErrLobbyStopped:
			return err
		default:
			fails++
			if fails >= opts.MaxFails {
				return err
			}
			uptime := time.Since(t).Round(time.Millisecond)
			log.Errorf(stderr, "lobby.Run: error: %+v uptime=%s runs=%d fails=%d", err, uptime, runs, fails)
			// Fast retry transient errors.
			continue
		}
		uptime := time.Since(t).Round(time.Millisecond)
		if uptime >= floor {
			// Reset after healthy lobby.
			fails = 0
		} else {
			// Lobby ended too soon.
			fails++
		}
		if opts.Debug {
			log.Debugf(stderr, "lobby.Run: uptime=%s runs=%d fails=%d", uptime, runs, fails)
		}
		if fails >= opts.MaxFails {
			if err != nil || opts.MaxFails == 0 {
				// Return timeout to caller.
				return err
			}
			return lobby.ErrLobbyMaxFails
		}
		if fails == 0 {
			// Fast restart healthy lobby.
			continue
		}
		if opts.Debug {
			log.Debugf(stderr, "lobby.Run: sleep: secs=%s runs=%d fails=%d", floor-uptime, runs, fails)
		}
		// Avoid busy loops from unknown bugs.
		select {
		case <-time.After(floor - uptime):
			continue
		case <-ctx.Done():
			break
		}
	}
	return ctx.Err()
}
