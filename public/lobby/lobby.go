package lobby

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/snap-gs/snap-gs/internal/lobby"
	"github.com/snap-gs/snap-gs/internal/log"
)

var ErrLobbyMaxFails = errors.New("lobby max fails")

func runc(ctx context.Context, stdout, stderr io.Writer, opts *Options, up bool) error {
	exe, args, err := opts.ExeArgs()
	if err != nil {
		return err
	}
	if opts.LogDir != "" {
		// Windows does not allow ':' in the filename.
		ts := strings.ReplaceAll(time.Now().In(time.UTC).Format(time.RFC3339), ":", "_")
		file := filepath.Join(opts.LogDir, ts+".lobby.log.gz")
		w, err := os.Create(file + ".lock")
		if err != nil {
			return err
		}
		// TODO: Errors.
		defer os.Rename(file+".lock", file)
		defer w.Close()
		wgz := gzip.NewWriter(w)
		defer wgz.Close()
		stdout = wgz
		if opts.Debug {
			log.Debugf(stderr, "runc: stdout=%s", file)
		}
	}
	if opts.StatDir != "" {
		file := filepath.Join(opts.StatDir, "log")
		w, err := os.Create(file)
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
	l := lobby.Lobby{
		Debug:    opts.Debug,
		MatchDir: opts.MatchDir,
		SpecDir:  opts.SpecDir,
		StatDir:  opts.StatDir,

		Timeout:      opts.Timeout,
		AdminTimeout: opts.AdminTimeout,

		MaxIdles:  opts.MaxIdles,
		MinUptime: opts.MinUptime,
	}
	if up {
		l.MinUptime = opts.MinUpUptime
		l.MaxIdles = -1
	}
	err = l.Run(ctx, stdout, stderr, exe, args...)
	log.Errorf(stderr, "runc: err=%+v minuptime=%s uptime=%s", err, l.MinUptime, l.Uptime())
	if !up {
		return err
	}
	switch err {
	case lobby.ErrLobbyTimeout, lobby.ErrLobbyIdleTimeout, lobby.ErrLobbyAdminTimeout:
		return nil
	case lobby.ErrLobbyRestarted, lobby.ErrLobbyStopped, lobby.ErrLobbyDowned:
		return nil
	default:
		return err
	}
}

func Run(ctx context.Context, stdout, stderr io.Writer, opts *Options) error {
	var tries, fails int
	started := time.Now()
	for ctx.Err() == nil {
		var err error
		var up, stop, down, restart bool
		if opts.SpecDir != "" {
			_, err = os.Stat(filepath.Join(opts.SpecDir, "up"))
			up = err == nil
			_, err = os.Stat(filepath.Join(opts.SpecDir, "stop"))
			stop = err == nil
			_, err = os.Stat(filepath.Join(opts.SpecDir, "down"))
			down = err == nil
			fi, err := os.Stat(filepath.Join(opts.SpecDir, "restart"))
			restart = err == nil && started.Before(fi.ModTime())
		}
		if opts.Debug {
			log.Debugf(stderr, "lobby.Run: specdir=%s up=%t stop=%t down=%t restart=%t", opts.SpecDir, up, stop, down, restart)
		}
		switch {
		case down, stop && !up, restart && !up:
			// TODO: Exit codes for the above.
			return ctx.Err()
		}
		tries++
		t := time.Now()
		err = runc(ctx, stdout, stderr, opts, up)
		if err != nil {
			if fails >= opts.MaxFails {
				return err
			}
			fails++
			log.Errorf(stderr, "lobby.Run: err=%+v tries=%d fails=%d", err, tries, fails)
			// Fast retry transient errors.
			continue
		}
		uptime := time.Since(t).Round(time.Millisecond)
		if opts.Debug {
			log.Debugf(stderr, "lobby.Run: uptime=%s minuptime=%s tries=%d fails=%d", uptime, opts.MinUptime, tries, fails)
		}
		if opts.MaxIdles >= 0 {
			return nil
		}
		if opts.MinUptime < uptime {
			// Reset after healthy lobby.
			fails = 0
			continue
		}
		// Lobby ended too soon.
		fails++
		if fails >= opts.MaxFails {
			return ErrLobbyMaxFails
		}
		if opts.Debug {
			log.Debugf(stderr, "lobby.Run: sleep: secs=%s tries=%d fails=%d", opts.MinUptime-uptime, tries, fails)
		}
		// Avoid busy loops from unknown bugs.
		select {
		case <-time.After(opts.MinUptime - uptime):
			continue
		case <-ctx.Done():
			break
		}
	}
	return ctx.Err()
}