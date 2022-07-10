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

type Func func([]Event, error)

type Filter func([]Event, error) ([]Event, error)

type Options struct {
	Tick time.Duration
	Path string
}

func Lockables(events []Event, err error) ([]Event, error) {
	for i := range events {
		if name := strings.TrimSuffix(events[i].Name, ".lock"); name != events[i].Name {
			events[i].Name, events[i].Op = name, fsnotify.Create
		}
	}
	return events, err
}

func Originals(events []Event, err error) ([]Event, error) {
	for i, j := 0, len(events); i < j; i++ {
		if name := strings.TrimPrefix(events[i].Name, "last"); name != events[i].Name {
			events = append(events, Event{Name: name, Op: fsnotify.Create})
		}
	}
	return events, err
}

func Duplicates(events []Event, err error) ([]Event, error) {
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

func Dir(ctx context.Context, path string, fn Func) (context.CancelFunc, error) {
	return Watcher(
		ctx, Options{Tick: 10 * time.Millisecond, Path: path},
		Lockables, Originals, Duplicates, func(events []Event, err error) ([]Event, error) {
			fn(events, err)
			return nil, nil
		},
	)
}

func Watcher(ctx context.Context, opts Options, filters ...Filter) (context.CancelFunc, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	path, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, err
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
	for i := range files {
		events[i] = Event{Name: files[i].Name(), Op: fsnotify.Create}
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
			head, tail := filepath.Split(event.Name)
			if head == "" || tail == "" || path != head[:len(head)-1] {
				continue
			}
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
