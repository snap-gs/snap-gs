package lobby

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/xattr"
	"github.com/snap-gs/snap-gs/internal/lobby"
	"github.com/snap-gs/snap-gs/internal/log"
	"github.com/snap-gs/snap-gs/internal/sync"
)

var ErrLobbyMaxFails = errors.New("lobby max fails")

func runc(ctx context.Context, stdout, stderr io.Writer, opts Options) error {
	exe, args, err := opts.ExeArgs()
	if err != nil {
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
			log.Debugf(stderr, "runc: stdout=%s", file)
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
			log.Debugf(stderr, "runc: stdout=%s", file)
		}
	}
	listen, addr, port := BindAddrPort(opts.Listen, "0.0.0.0", "27002")
	listen1, addr, port := BindAddrPort(opts.Listen1, addr, port)
	listen2, addr, port := BindAddrPort(opts.Listen2, addr, port)
	l := lobby.Lobby{
		Listen:       listen,
		Listen1:      listen1,
		Listen2:      listen2,
		Debug:        opts.Debug,
		SpecDir:      opts.SpecDir,
		StatDir:      opts.StatDir,
		MaxIdles:     opts.MaxIdles,
		MinUptime:    opts.MinUptime,
		Timeout:      opts.Timeout,
		AdminTimeout: opts.AdminTimeout,
	}
	if opts.LogState || opts.LogMatch || opts.LogClean {
		l.LogDir = opts.LogDir
		l.LogState = opts.LogState
		l.LogClean = opts.LogClean
	}
	err = l.Run(ctx, stdout, stderr, exe, args...)
	log.Errorf(stderr, "runc: error: %+v uptime=%s", err, l.Uptime())
	return err
}

func Run(ctx context.Context, stdout, stderr io.Writer, opts *Options) error {
	if opts.FlagDir != "" {
		cancel, err := opts.WatchFlagDir(ctx)
		if err != nil {
			return err
		}
		defer cancel()
	}
	if opts.Debug {
		log.Debugf(stderr, "lobby.Run: opts=%+v", opts)
	}
	var runs, fails int
	started := time.Now()
	const floor = 15 * time.Second
	for ctx.Err() == nil {
		var err error
		var stop, down, restart bool
		if opts.SpecDir != "" {
			_, err = os.Stat(filepath.Join(opts.SpecDir, "down"))
			down = err == nil
			fi, err := os.Stat(filepath.Join(opts.SpecDir, "stop"))
			stop = err == nil && started.Before(fi.ModTime())
			fi, err = os.Stat(filepath.Join(opts.SpecDir, "restart"))
			restart = err == nil && started.Before(fi.ModTime())
		}
		if opts.Debug {
			log.Debugf(stderr, "lobby.Run: specdir=%s stop=%t down=%t restart=%t", opts.SpecDir, stop, down, restart)
		}
		switch {
		case down, stop && runs != 0, restart && runs != 0:
			// TODO: Exit codes for the above.
			return ctx.Err()
		}
		runs++
		t := time.Now()
		err = runc(ctx, stdout, stderr, *opts)
		if err != nil {
			if fails >= opts.MaxFails {
				return err
			}
			fails++
			log.Errorf(stderr, "lobby.Run: error: %+v runs=%d fails=%d", err, runs, fails)
			// Fast retry transient errors.
			continue
		}
		uptime := time.Since(t).Round(time.Millisecond)
		if opts.Debug {
			log.Debugf(stderr, "lobby.Run: uptime=%s runs=%d fails=%d", uptime, runs, fails)
		}
		if opts.MaxIdles >= 0 {
			return nil
		}
		if uptime > floor {
			// Reset after healthy lobby.
			fails = 0
			continue
		}
		// Lobby ended too soon.
		if fails >= opts.MaxFails {
			return ErrLobbyMaxFails
		}
		fails++
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
