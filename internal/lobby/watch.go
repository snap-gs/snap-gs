package lobby

import (
	"os"
	"path/filepath"
	"time"
)

func (l *Lobby) watcher() {
	defer l.cwg.Done()
	defer l.debugf("watcher: done")
	l.debugf("watcher: timeout=%s admintimeout=%s", l.Timeout, l.AdminTimeout)
	if l.Timeout <= 0 && l.AdminTimeout <= 0 {
		return
	}
	every := time.Minute
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
		if l.m.id != "" {
			continue
		}
		since := time.Since(l.m.at).Round(100 * time.Microsecond)
		if since < l.MinUptime {
			continue
		}
		players, bots := l.players.Count()
		if players == 0 && l.Timeout > 0 && l.Timeout < since {
			l.reason = ErrLobbyTimeout
		} else if players != 0 && l.AdminTimeout > 0 && l.AdminTimeout < since {
			l.reason = ErrLobbyAdminTimeout
		} else if players == 0 && l.SpecDir != "" {
			_, err := os.Stat(filepath.Join(l.SpecDir, "up"))
			up := err == nil
			_, err = os.Stat(filepath.Join(l.SpecDir, "stop"))
			stop := err == nil
			_, err = os.Stat(filepath.Join(l.SpecDir, "down"))
			down := err == nil
			fi, err := os.Stat(filepath.Join(l.SpecDir, "restart"))
			restart := err == nil && l.t1.Before(fi.ModTime())
			switch {
			case down:
				l.reason = ErrLobbyDowned
			case stop && !up:
				l.reason = ErrLobbyStopped
			case restart && !up:
				l.reason = ErrLobbyRestarted
			default:
				continue
			}
		} else {
			continue
		}
		l.debugf("watcher: players=%d bots=%d since=%s reason=%s", players, bots, since, l.reason)
		l.cancel()
		return
	}
}
