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
	Up       time.Time
	FlagUp   time.Time
	PeerFull time.Time

	Down          time.Time
	FlagDown      time.Time
	PeerIdle      time.Time
	ForceDown     time.Time
	FlagForceDown time.Time

	Restart          time.Time
	FlagRestart      time.Time
	ForceRestart     time.Time
	FlagForceRestart time.Time

	Stop          time.Time
	PeerUp        time.Time
	FlagStop      time.Time
	ForceStop     time.Time
	FlagForceStop time.Time
}

func (s *Spec) DownAfter(t time.Time) bool {
	switch {
	case s == nil:
		return false
	case s.Down.After(t):
		return true
	case s.FlagDown.After(t):
		return true
	case s.ForceDownAfter(t):
		return true
	default:
		return false
	}
}

func (s *Spec) ForceDownAfter(t time.Time) bool {
	switch {
	case s == nil:
		return false
	case s.ForceDown.After(t):
		return true
	case s.FlagForceDown.After(t):
		return true
	default:
		return false
	}
}

func (s *Spec) StopAfter(t time.Time) bool {
	switch {
	case s == nil:
		return false
	case s.Stop.After(t):
		return true
	case s.FlagStop.After(t):
		return true
	case s.ForceStopAfter(t):
		return true
	default:
		return false
	}
}

func (s *Spec) ForceStopAfter(t time.Time) bool {
	switch {
	case s == nil:
		return false
	case s.ForceStop.After(t):
		return true
	case s.FlagForceStop.After(t):
		return true
	default:
		return false
	}
}

func (s *Spec) RestartAfter(t time.Time) bool {
	switch {
	case s == nil:
		return false
	case s.Restart.After(t):
		return true
	case s.FlagRestart.After(t):
		return true
	case s.ForceRestartAfter(t):
		return true
	default:
		return false
	}
}

func (s *Spec) ForceRestartAfter(t time.Time) bool {
	switch {
	case s == nil:
		return false
	case s.ForceRestart.After(t):
		return true
	case s.FlagForceRestart.After(t):
		return true
	default:
		return false
	}
}

func (s *Spec) Watch(ctx context.Context, path string) (func(), error) {
	spec := *s
	update := func(name string, bs []byte) {
		var out, in *time.Time
		switch name {
		case "up":
			out, in = &s.Up, &spec.Up
		case "flag/up":
			out, in = &s.FlagUp, &spec.FlagUp
		case "peer/full":
			out, in = &s.PeerFull, &spec.PeerFull
		case "down":
			out, in = &s.Down, &spec.Down
		case "flag/down":
			out, in = &s.FlagDown, &spec.FlagDown
		case "peer/idle":
			out, in = &s.PeerIdle, &spec.PeerIdle
		case "forcedown":
			out, in = &s.ForceDown, &spec.ForceDown
		case "flag/forcedown":
			out, in = &s.FlagForceDown, &spec.FlagForceDown
		case "restart":
			out, in = &s.Restart, &spec.Restart
		case "flag/restart":
			out, in = &s.FlagRestart, &spec.FlagRestart
		case "forcerestart":
			out, in = &s.ForceRestart, &spec.ForceRestart
		case "flag/forcerestart":
			out, in = &s.FlagForceRestart, &spec.FlagForceRestart
		case "stop":
			out, in = &s.Stop, &spec.Stop
		case "peer/up":
			out, in = &s.PeerUp, &spec.PeerUp
		case "flag/stop":
			out, in = &s.FlagStop, &spec.FlagStop
		case "forcestop":
			out, in = &s.ForceStop, &spec.ForceStop
		case "flag/forcestop":
			out, in = &s.FlagForceStop, &spec.FlagForceStop
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
	return watch.Watch(ctx, path, 200*time.Millisecond, watch.LastNames, watch.LockNames, watch.SameNames,
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
