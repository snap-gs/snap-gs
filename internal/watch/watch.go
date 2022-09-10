package watch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Event fsnotify.Event

type Filter func([]Event, error) ([]Event, error)

type Options struct {
	Tick time.Duration
	Path string
}

func FilterLockNames(events []Event, err error) ([]Event, error) {
	for i := range events {
		if name := strings.TrimSuffix(events[i].Name, ".lock"); name != events[i].Name {
			events[i].Name, events[i].Op = name, fsnotify.Create
		}
	}
	return events, err
}

func FilterLastNames(events []Event, err error) ([]Event, error) {
	for i, j := 0, len(events); i < j; i++ {
		if name := strings.TrimPrefix(events[i].Name, "last"); name != events[i].Name {
			events = append(events, Event{Name: name, Op: fsnotify.Create})
		}
	}
	return events, err
}

func FilterSameNames(events []Event, err error) ([]Event, error) {
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

func Watcher(ctx context.Context, opts *Options, filters ...Filter) (func(), error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	path, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, err
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}

	if path != "" && path[len(path)-1] != os.PathSeparator {
		path += string(os.PathSeparator)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	err = watcher.Add(path)
	if err != nil {
		watcher.Close()
		return nil, err
	}

	files, err := os.ReadDir(path)
	if err != nil {
		watcher.Close()
		return nil, err
	}

	events := make([]Event, len(files))
	renames := make(map[string]string, 10)
	for i := range files {
		events[i] = Event{
			Name: files[i].Name(),
			Op:   fsnotify.Create,
		}
		p1 := path + events[i].Name
		p2, err := filepath.EvalSymlinks(p1)
		if err != nil || p1 == p2 {
			continue
		}
		_ = watcher.Add(p2)
		renames[p2] = p1
	}

	ticker := time.NewTicker(opts.Tick)
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
			name := event.Name
			if rename, ok := renames[name]; ok {
				name = rename
			}
			head, tail := filepath.Split(name)
			if head == "" || tail == "" {
				continue
			}
			if head != path {
				if rename, ok := renames[head[:len(head)-1]]; ok {
					head = rename + string(head[len(head)-1])
				}
			}
			if head != path {
				var dir string
				head, dir = filepath.Split(head[:len(head)-1])
				if head == "" || dir == "" {
					continue
				}
				tail = dir + string(os.PathSeparator) + tail
			}
			if head != path {
				continue
			}
			_ = watcher.Add(head + tail)
			if len(events) == 0 {
				ticker.Reset(opts.Tick)
			}
			events = append(events, Event{Name: tail, Op: event.Op})
		}
	}()

	cancel := func() {
		watcher.Close()
		<-done
		return
	}

	return cancel, nil
}
