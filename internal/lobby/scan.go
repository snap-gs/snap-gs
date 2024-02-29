package lobby

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/snap-gs/snap-gs/internal/match"
)

var stdoutIgnorePrefixes = [][]byte{
	[]byte(`(Filename:`),
	[]byte(`-- BOLT -- ArenaSidesSwapped Changed`),
	[]byte(`-- BOLT -- CountdownStartTime Changed`),
}

var stderrIgnorePrefixes = [][]byte{
	[]byte(`ALSA lib conf.c:`),
	[]byte(`ALSA lib confmisc.c:`),
	[]byte(`ALSA lib pcm.c:`),
	[]byte(`Unable to connect to 127.`),
	[]byte(`[S_API`),
	[]byte(`dlopen failed trying to load:`),
	[]byte(`with error:`),
}

var stderrIgnoreContains = [][]byte{
	[]byte(`.steam/sdk64/steamclient.so`),
}

func truncate(in []byte, n int) []byte {
	if len(in) < n {
		return in
	}
	out := make([]byte, n, n+20)
	copy(out, in)
	return append(out, []byte(fmt.Sprintf("... (%d bytes)", len(in)-n))...)
}

func (l *Lobby) filter(fd int, bs []byte) ([]byte, error) {
	var prefixes, contains [][]byte
	switch fd {
	case 1:
		prefixes, contains = stdoutIgnorePrefixes, nil
	case 2:
		prefixes, contains = stderrIgnorePrefixes, stderrIgnoreContains
	default:
		return bs, nil
	}
	sample := bytes.TrimSpace(bs)
	ignore := len(sample) == 0
	for i := range prefixes {
		if ignore = ignore || bytes.HasPrefix(sample, prefixes[i]); ignore {
			return nil, nil
		}
	}
	for i := range contains {
		if ignore = ignore || bytes.Contains(sample, contains[i]); ignore {
			return nil, nil
		}
	}
	if fd != 1 {
		return bs, nil
	}
	const (
		bolt   = "-- BOLT -- "
		upload = "Upload complete"
		ppool  = "Finished populating pool"
		arena  = "Received request for ArenaSpecName "
		nosess = "Failed to create session"
		disco  = "Disconnected"
	)
	var err error
	switch {
	case bs[0] == '{' && bs[len(bs)-1] == '}':
		bs, err = l.filterjson(fd, bs)
	case bytes.HasPrefix(bs, []byte(bolt)):
		bs, err = l.filterbolt(fd, bs)
	case bytes.HasPrefix(bs, []byte(upload)):
		// More timely (but less reliable) than waiting for 'matchId' to change.
		l.collect()
	case bytes.HasPrefix(bs, []byte(ppool)):
		if players, _ := l.players.Count(); players == 0 {
			l.newstat("idle")
		}
	case len(bs) != len(arena) && bytes.HasPrefix(bs, []byte(arena)):
		l.arena = string(bs[len(arena):])
		// TODO: Atomicity.
		l.remstat("arena")
		l.setstat("arena", l.arena)
		l.debugf("filter: arena=%s", l.arena)
	case bytes.HasPrefix(bs, []byte(nosess)), bytes.HasPrefix(bs, []byte(disco)):
		l.debugf("filter: reason=%+v", ErrLobbyDisconnected)
		l.Cancel(ErrLobbyDisconnected)
	}
	return bs, err
}

func (l *Lobby) filterjson(fd int, bs []byte) ([]byte, error) {
	const trunc = 66
	if l.opts.LogDir == "" {
		return truncate(bs, trunc), nil
	}
	if len(bs) < trunc {
		l.debugf("filterjson: ignored")
		return truncate(bs, trunc), nil
	}
	var v interface{}
	var k *match.Kill
	var m *match.Match
	switch {
	case bytes.Index(bs[:trunc], []byte(`"shooterId"`)) != -1:
		k = &match.Kill{}
		v = k
	case bytes.Index(bs[:trunc], []byte(`"matchId"`)) != -1:
		m = &match.Match{}
		v = m
	default:
		l.debugf("filterjson: unknown")
		return truncate(bs, trunc), nil
	}
	if err := json.Unmarshal(bs, v); err != nil {
		l.errorf("filterjson: json.Unmarshal: error: %+v", err)
		return truncate(bs, trunc), nil
	}
	if k != nil {
		l.m.KillData = append(l.m.KillData, *k)
		return truncate(bs, trunc), nil
	}
	if m.MatchID != l.m.MatchID {
		l.collect()
	}
	// Set match ASAP with current time.
	m.Timestamp = l.m.Timestamp
	// Advertise match before parsing.
	l.m = m
	defer l.newstat("match")
	const layout = "1/2/2006 3:04:05 PM"
	i := strings.Index(m.MatchID, l.session)
	if i == -1 || i == len(m.MatchID)-len(l.session) {
		l.errorf("filterjson: discard (mismatched): id=%s session=%s", m.MatchID, l.session)
		return truncate(bs, trunc), nil
	}
	t, err := time.Parse(layout, m.MatchID[i+len(l.session):])
	if err != nil {
		l.errorf("filterjson: time.Parse: error: %+v", err)
		return truncate(bs, trunc), nil
	}
	// Update match with parsed time.
	l.m.Timestamp = t
	return truncate(bs, trunc), nil
}

func (l *Lobby) filterbolt(fd int, bs []byte) ([]byte, error) {
	const (
		registeredPlayer      = "-- BOLT -- Registered player: "
		unregisteredPlayer    = "-- BOLT -- Unregistered player: "
		loadingArenaName      = "-- BOLT -- Loading arena name: "
		arenaSpecNameChanged  = "-- BOLT -- ArenaSpecName Changed"
		ballsPerSecondChanged = "-- BOLT -- BallsPerSecond Changed"
		playerAssigned        = "-- BOLT -- Player assigned "
		remoteCallbacks       = "-- BOLT -- REMOTE CALLBACKS "
	)
	switch {
	case len(bs) != len(loadingArenaName) && bytes.HasPrefix(bs, []byte(loadingArenaName)):
		l.arena = string(bs[len(loadingArenaName):])
		// TODO: Atomicity.
		l.remstat("arena")
		l.setstat("arena", l.arena)
		l.debugf("filterbolt: arena=%s", l.arena)
	case bytes.HasPrefix(bs, []byte(playerAssigned)) || bytes.HasPrefix(bs, []byte(remoteCallbacks)):
		// Player trying to register.
		l.remstat("idle")
	case len(bs) != len(registeredPlayer) && bytes.HasPrefix(bs, []byte(registeredPlayer)):
		id, _, _, admin := l.players.Add(string(bs[len(registeredPlayer):]))
		players, bots := l.players.Count()
		if id != -1 {
			if id < 1000 {
				// bots < 1000 <= players
				l.changed = true
			} else if players != 0 {
				l.setstat("players", players)
			}
		} else {
			defer l.Cancel(ErrLobbyBug)
		}
		l.debugf("filterbolt: players=%d bots=%d id=+%d admin=%t", players, bots, id, admin)
		switch players {
		case 0:
		case 1:
			l.remstat("idle")
			// Flush match and update timestamp.
			l.collect()
		case 2, 3, 4, 5, 6, 7, 8, 9:
		case 10:
			// Limit is 10 but 11 or even 12 people seen in the wild.
			l.newstat("full")
		}
	case len(bs) != len(unregisteredPlayer) && bytes.HasPrefix(bs, []byte(unregisteredPlayer)):
		id, _, _, admin := l.players.Remove(string(bs[len(unregisteredPlayer):]))
		players, bots := l.players.Count()
		if id != -1 {
			if id < 1000 {
				// bots < 1000 <= players
				l.changed = true
			} else if players != 0 {
				l.setstat("players", players)
			}
		} else {
			defer l.Cancel(ErrLobbyBug)
		}
		if admin && l.m.MatchID == "" {
			// Reset admin timeout when admin changes.
			l.collect()
		}
		l.debugf("filterbolt: players=%d bots=%d id=-%d admin=%t", players, bots, id, admin)
		switch players {
		case 0:
			l.remstat("players")
			// Flush match and update timestamp.
			l.collect()
			if !l.changed || l.opts.MaxFails == 0 {
				l.newstat("idle")
			} else if force, err := l.spec.ReasonAfter(l.t1, 0, 0); err != nil {
				l.debugf("filterbolt: players=%d bots=%d changed=%t reason=%s force=%t", players, bots, l.changed, err, force)
				l.Cancel(err)
			} else {
				l.debugf("filterbolt: players=%d bots=%d changed=%t reason=%s", players, bots, l.changed, ErrLobbyIdleTimeout)
				l.Cancel(ErrLobbyIdleTimeout)
			}
		case 9:
			// Limit is 10 but 11 or even 12 people seen in the wild.
			l.remstat("full")
		}
	case bytes.HasPrefix(bs, []byte(arenaSpecNameChanged)):
		// Fires once before players join to set default arena.
		if players, _ := l.players.Count(); players != 0 {
			l.changed = true
		}
	case bytes.HasPrefix(bs, []byte(ballsPerSecondChanged)):
		// Fires once before players join to set default bps.
		if players, _ := l.players.Count(); players != 0 {
			l.changed = true
		}
	}
	return bs, nil
}

func (l *Lobby) scanner(fd int) {
	defer l.wg.Done()
	defer l.debugf("scanner: done")
	defer l.Cancel(ErrLobbyDone)
	var r io.Reader
	var w func([]byte)
	switch fd {
	case 1:
		defer close(l.matches)
		defer l.collect()
		defer l.prout.Close()
		r, w = l.prout, l.logvout
	case 2:
		defer l.prerr.Close()
		r, w = l.prerr, l.logverr
	default:
		return
	}
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, pipesz), pipesz)
	for s.Scan() {
		bs, err := l.filter(fd, s.Bytes())
		if err != nil {
			l.errorf("scanner: filter: error: %+v fd=%d", err, fd)
			continue
		}
		if len(bs) == 0 {
			continue
		}
		w(bs)
	}
	if s.Err() != nil {
		l.errorf("scanner: error: %+v fd=%d", s.Err(), fd)
	}
}
