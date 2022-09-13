package watch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	ErrWatchFiltersUnconfigured = errors.New("watch filters unconfigured")
	ErrWatchPathUnconfigured    = errors.New("watch path unconfigured")
	ErrWatchTickUnconfigured    = errors.New("watch tick unconfigured")
	ErrWatchUnconfigured        = errors.New("watch unconfigured")
)

type Event fsnotify.Event

type Filter func([]Event, error) ([]Event, error)

func LockNames(events []Event, err error) ([]Event, error) {
	for i := range events {
		if name := strings.TrimSuffix(events[i].Name, ".lock"); name != events[i].Name {
			events[i].Name, events[i].Op = name, fsnotify.Create
		}
	}
	return events, err
}

func LastNames(events []Event, err error) ([]Event, error) {
	for i, j := 0, len(events); i < j; i++ {
		if name := strings.TrimPrefix(events[i].Name, "last"); name != events[i].Name {
			events = append(events, Event{Name: name, Op: fsnotify.Create})
		}
	}
	return events, err
}

func SameNames(events []Event, err error) ([]Event, error) {
	names := make(map[string]fsnotify.Op, len(events))
	for i := range events {
		names[events[i].Name] |= events[i].Op
	}
	events = events[:0]
	for name, op := range names {
		events = append(events, Event{Name: name, Op: op})
	}
	return events, err
}

func Watch(ctx context.Context, path string, tick time.Duration, filters ...Filter) (func(), error) {
	if path == "" {
		return nil, ErrWatchPathUnconfigured
	}

	if tick <= 0 {
		return nil, ErrWatchTickUnconfigured
	}

	if len(filters) == 0 {
		return nil, ErrWatchFiltersUnconfigured
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if path == "" {
		return nil, ErrWatchPathUnconfigured
	}

	if path[len(path)-1] == os.PathSeparator {
		return nil, ErrWatchUnconfigured
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	root := path + string(os.PathSeparator)
	watches := make(map[string]string, 10)
	events := make([]Event, 0, 10)
	paths := []string{path}
	for i := 0; i < len(paths); i++ {
		if err := filepath.WalkDir(paths[i], func(path string, d fs.DirEntry, err error) error {
			if err != nil || d == nil || path == "" || path[len(path)-1] == os.PathSeparator {
				return err
			}
			head, tail := filepath.Split(path)
			if head == "" || tail == "" {
				return nil
			}
			out := path
			if alias, ok := watches[head]; ok {
				out = alias + tail
			}
			if d.Type().IsRegular() {
				if path == paths[i] {
					return nil
				}
				name := strings.TrimPrefix(out, root)
				events = append(events, Event{Name: name, Op: fsnotify.Create})
				return nil
			}
			in := path + string(os.PathSeparator)
			out += string(os.PathSeparator)
			if watches[out] != "" {
				out = watches[out]
			}
			if d.Type().IsDir() {
				watches[in] = out
				return watcher.Add(path)
			}
			if d.Type()&fs.ModeSymlink == 0 {
				return nil
			}
			path, err = filepath.EvalSymlinks(path)
			if err != nil || path == "" || path[len(path)-1] == os.PathSeparator {
				return err
			}
			in = path + string(os.PathSeparator)
			if watches[in] != "" {
				return errors.New("watch duplicated: " + watches[in] + " -> " + in + " <- " + out)
			}
			paths = append(paths, path)
			watches[in] = out
			return nil
		}); err != nil {
			watcher.Close()
			return nil, err
		}
	}

	if len(watcher.WatchList()) == 0 {
		return nil, ErrWatchUnconfigured
	}

	ticker := time.NewTicker(tick)
	apply := func(evts []Event, err error, ok bool) {
		for i := range filters {
			evts, err = filters[i](evts, err)
			if ok && evts == nil && err == nil {
				break
			}
		}
	}
	wrap := func(err error, ok bool) {
		if len(events) != 0 {
			ticker.Stop()
			apply(events, nil, true)
			events = events[:0]
		}
		if err != nil {
			apply(nil, err, true)
		}
		if !ok {
			apply(nil, nil, false)
		}
	}

	wrap(nil, true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ticker.Stop()
		defer watcher.Close()
		for {
			var event fsnotify.Event
			var now time.Time
			var err error
			var ok bool
			select {
			case event, ok = <-watcher.Events:
			case err, ok = <-watcher.Errors:
			case now, ok = <-ticker.C:
			case <-ctx.Done():
				err = ctx.Err()
			}
			if !ok {
				wrap(err, false)
				return
			}
			if err != nil || !now.IsZero() {
				wrap(err, true)
				continue
			}
			head, tail := filepath.Split(event.Name)
			if head == "" || tail == "" {
				continue
			}
			head = watches[head]
			if !strings.HasPrefix(head, root) {
				continue
			}
			if len(events) == 0 {
				ticker.Reset(tick)
			}
			name := strings.TrimPrefix(head+tail, root)
			events = append(events, Event{Name: name, Op: event.Op})
		}
	}()

	cancel := func() {
		watcher.Close()
		<-done
		return
	}

	return cancel, nil
}
