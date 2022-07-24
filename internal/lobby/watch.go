package lobby

import (
	"os"
	"path/filepath"
	"time"
)

func (l *Lobby) watcher() {
	defer l.cwg.Done()
	defer l.debugf("watcher: done")
	l.debugf("watcher: minuptime=%s timeout=%s admintimeout=%s", l.MinUptime, l.Timeout, l.AdminTimeout)
	if l.Timeout <= 0 && l.AdminTimeout <= 0 && l.SpecDir == "" {
		return
	}
	every := 15 * time.Second
	if l.Timeout > 0 && l.Timeout < every {
		every = l.Timeout
	}
	if l.AdminTimeout > 0 && l.AdminTimeout < every {
		every = l.AdminTimeout
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-l.ctx.Done():
			return
		case <-t.C:
		}
		if l.m.MatchID != "" {
			l.savestate()
			continue
		}
		since := time.Since(l.m.Timestamp).Round(100 * time.Microsecond)
		players, bots := l.players.Count()
		switch {
		case players != 0 && l.AdminTimeout > 0 && l.AdminTimeout < since:
			l.reason = ErrLobbyAdminTimeout
		case players == 0 && l.Timeout > 0 && l.Timeout < since:
			l.reason = ErrLobbyTimeout
		case players == 0 && l.SpecDir != "":
			for _, key := range []string{"down", "stop", "restart"} {
				fi, err := os.Stat(filepath.Join(l.SpecDir, key))
				if err == nil && (key != "restart" || l.t1.Before(fi.ModTime())) {
					l.updatestate(l.states.Spec, key, fi.ModTime().UTC().Format(time.RFC3339))
				} else {
					l.updatestate(l.states.Spec, key, nil)
				}
			}
			switch {
			case l.hasstate(l.states.Spec, "down"):
				l.reason = ErrLobbyDowned
			case l.hasstate(l.states.Spec, "stop") && since > l.MinUptime:
				l.reason = ErrLobbyStopped
			case l.hasstate(l.states.Spec, "restart") && since > l.MinUptime:
				l.reason = ErrLobbyRestarted
			default:
				l.savestate()
				continue
			}
		default:
			l.savestate()
			continue
		}
		l.debugf("watcher: players=%d bots=%d since=%s reason=%s", players, bots, since, l.reason)
		l.cancel()
		return
	}
}
