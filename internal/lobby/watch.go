package lobby

import (
	"context"
	"time"
)

func (l *Lobby) watcher(ctx context.Context) {
	defer l.wg.Done()
	defer l.debugf("watcher: done")
	l.debugf("watcher: minuptime=%s timeout=%s admintimeout=%s", l.opts.MinUptime, l.opts.Timeout, l.opts.AdminTimeout)
	if l.opts.Timeout <= 0 && l.opts.AdminTimeout <= 0 {
		return
	}
	defer l.Cancel(ErrLobbyDone)
	every := time.Second
	floor := 200 * time.Millisecond
	if l.opts.Timeout > 0 && l.opts.Timeout < every {
		every = l.opts.Timeout
	}
	if l.opts.AdminTimeout > 0 && l.opts.AdminTimeout < every {
		every = l.opts.AdminTimeout
	}
	if every < floor {
		every = floor
	}
	lastup := l.m.Timestamp
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		var now time.Time
		var ok bool
		select {
		case now, ok = <-ticker.C:
		case <-l.done:
		}
		if !ok {
			return
		}
		lastidle := l.m.Timestamp
		if !l.spec.Up.IsZero() || !l.spec.PeerFull.IsZero() {
			lastup = now.UTC()
		}
		if lastidle.Before(lastup) {
			lastidle = lastup
		}
		players, bots := l.players.Count()
		since := time.Since(lastidle).Round(100 * time.Microsecond)
		if l.spec.ForceDownAfter(l.t1) {
			l.debugf("watcher: cancel: %s players=%d bots=%d since=%s", ErrLobbyDowned, players, bots, since)
			l.Cancel(ErrLobbyDowned)
			return
		}
		if l.m.MatchID != "" {
			continue
		}
		force, reason := l.spec.ReasonAfter(l.t1, since, l.opts.MinUptime, l.opts.Timeout)
		if players != 0 && !force {
			continue
		}
		if players != 0 && reason == nil && l.opts.AdminTimeout > 0 && l.opts.AdminTimeout < since {
			reason = ErrLobbyAdminTimeout
		}
		if reason != nil {
			l.debugf("watcher: cancel: %s players=%d bots=%d since=%s", reason, players, bots, since)
			l.Cancel(reason)
			return
		}
	}
}
