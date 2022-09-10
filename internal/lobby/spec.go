package lobby

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/snap-gs/snap-gs/internal/watch"
)

type Spec struct {
	Up         time.Time
	Down       time.Time
	Stop       time.Time
	Start      time.Time
	ForceUp    time.Time
	ForceDown  time.Time
	ForceStop  time.Time
	ForceStart time.Time
}

func (s *Spec) Watch(ctx context.Context, path string) (func(), error) {
	spec := *s
	update := func(name string, bs []byte) {
		var out, in *time.Time
		switch name {
		case "up", "peer/full":
			out, in = &s.Up, &spec.Up
		case "down", "peer/idle":
			out, in = &s.Down, &spec.Down
		case "stop", "peer/up":
			out, in = &s.Stop, &spec.Stop
		case "start":
			out, in = &s.Start, &spec.Start
		case "forceup":
			out, in = &s.ForceUp, &spec.ForceUp
		case "forcedown":
			out, in = &s.ForceDown, &spec.ForceDown
		case "forcestop":
			out, in = &s.ForceStop, &spec.ForceStop
		case "forcestart":
			out, in = &s.ForceStart, &spec.ForceStart
		default:
			return
		}
		nl := len(bs) != 0 && bs[len(bs)-1] == '\n'
		if nl {
			bs = bs[:len(bs)-1]
		}
		switch {
		case !nl && len(bs) == 0:
			*out = *in
		case nl && len(bs) == 0:
			*out = time.Now().UTC()
		default:
			_ = json.Unmarshal(bs, out)
		}
	}
	return watch.Watcher(
		ctx,
		&watch.Options{
			Path: path,
			Tick: 200 * time.Millisecond,
		},
		watch.FilterSameNames,
		func(events []watch.Event, err error) ([]watch.Event, error) {
			for _, event := range events {
				if bs, _ := os.ReadFile(filepath.Join(path, event.Name)); len(bs) != 0 {
					update(event.Name, bs)
				} else {
					update(event.Name, nil)
				}
			}
			if events == nil && err == nil {
				*s = spec
			}
			return events, err
		},
	)
}
