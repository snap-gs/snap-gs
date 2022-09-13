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
		var reason error
		switch {
		case l.spec.ForceStopAfter(l.t1):
			reason = ErrLobbyStopped
		case l.spec.ForceRestartAfter(l.t1):
			reason = ErrLobbyRestarted
		case players != 0:
			switch {
			case l.opts.AdminTimeout > 0 && l.opts.AdminTimeout < since:
				reason = ErrLobbyAdminTimeout
			}
		case !l.spec.PeerIdle.IsZero() || l.spec.DownAfter(l.t1):
			reason = ErrLobbyDowned
		case since > l.opts.MinUptime && (!l.spec.PeerUp.IsZero() || l.spec.StopAfter(l.t1)):
			reason = ErrLobbyStopped
		case l.spec.RestartAfter(l.t1):
			reason = ErrLobbyRestarted
		case l.opts.Timeout > 0 && l.opts.Timeout < since:
			reason = ErrLobbyTimeout
		}
		if reason != nil {
			l.debugf("watcher: cancel: %s players=%d bots=%d since=%s", reason, players, bots, since)
			l.Cancel(reason)
			return
		}
	}
}
