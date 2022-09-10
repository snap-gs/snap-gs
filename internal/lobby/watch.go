package lobby

import (
	"context"
	"time"
)

func (l *Lobby) watcher(ctx context.Context) {
	defer l.cwg.Done()
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
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-l.done:
			return
		case <-t.C:
		}
		if l.t1.IsZero() {
			continue
		}
		if l.m.MatchID != "" {
			continue
		}
		lastidle := l.m.Timestamp
		if !l.spec.Up.IsZero() {
			lastup = time.Now().UTC()
		}
		if lastidle.Before(lastup) {
			lastidle = lastup
		}
		since := time.Since(lastidle).Round(100 * time.Microsecond)
		players, bots := l.players.Count()
		var reason error
		switch {
		case players == 0 && !l.spec.Down.IsZero():
			reason = ErrLobbyDowned
		case players == 0 && !l.spec.Stop.IsZero() && l.opts.MinUptime < since:
			reason = ErrLobbyStopped
		case players == 0 && l.spec.Start.After(l.t1):
			reason = ErrLobbyRestarted
		case players == 0 && l.opts.Timeout > 0 && l.opts.Timeout < since:
			reason = ErrLobbyTimeout
		case players != 0 && l.opts.AdminTimeout > 0 && l.opts.AdminTimeout < since:
			reason = ErrLobbyAdminTimeout
		default:
			continue
		}
		l.Cancel(reason)
		l.debugf("watcher: players=%d bots=%d since=%s reason=%s", players, bots, since, l.reason)
		return
	}
}
