package lobby

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/snap-gs/snap-gs/internal/log"
)

var stdoutIgnorePrefixes = [][]byte{
	[]byte(`(Filename:`),
	[]byte(`-- BOLT -- ArenaSidesSwapped Changed`),
	[]byte(`-- BOLT -- CountdownStartTime Changed`),
	[]byte(`-- BOLT -- Ignoring Teleport event`),
	[]byte(`-- BOLT -- Snapping client body`),
	[]byte(`IK detected on teleported transform`),
	[]byte(`Preview:`),
	[]byte(`Received RegisterHitWitness`),
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
		upload = "Upload complete!"
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
		l.setstat("arena", bs[len(arena):])
		l.debugf("filter: arena=%s", l.arena)
	case bytes.HasPrefix(bs, []byte(nosess)), bytes.HasPrefix(bs, []byte(disco)):
		l.debugf("filter: reason=%+v", ErrLobbyDisconnected)
		l.reason = ErrLobbyDisconnected
		l.cancel()
	}
	return bs, err
}

func (l *Lobby) filterjson(fd int, bs []byte) ([]byte, error) {
	const trunc = 66
	if l.MatchDir == "" {
		l.debugf("filterjson: discard")
		return truncate(bs, trunc), nil
	}
	const mid = `"matchId":"`
	i := bytes.Index(bs, []byte(mid))
	if i == -1 || i == len(bs)-len(mid) {
		l.errorf("filterjson: discard (ignored)")
		return truncate(bs, trunc), nil
	}
	bid := bs[i+len(mid):]
	i = bytes.IndexByte(bid, '"')
	if i == -1 {
		l.errorf("filterjson: discard (malformed)")
		return truncate(bs, trunc), nil
	}
	if i == 0 {
		// no-match-id:enemyLocation != match-id:enemyLocation for unknown reasons.
		return truncate(bs, trunc), nil
	}
	id := string(bid[:i])
	if !strings.Contains(id, l.session) {
		l.errorf("filterjson: discard (mismatched): id=%s session=%s", id, l.session)
		return truncate(bs, trunc), nil
	}
	l.enqueue(id, bs)
	return truncate(bs, trunc), nil
}

func (l *Lobby) filterbolt(fd int, bs []byte) ([]byte, error) {
	const (
		creatingSession       = "-- BOLT -- Creating session: "
		registeredPlayer      = "-- BOLT -- Registered player: "
		unregisteredPlayer    = "-- BOLT -- Unregistered player: "
		loadingArenaName      = "-- BOLT -- Loading arena name: "
		arenaSpecNameChanged  = "-- BOLT -- ArenaSpecName Changed"
		ballsPerSecondChanged = "-- BOLT -- BallsPerSecond Changed"
		gameStateChanged      = "-- BOLT -- Game State Changed to "
		playerAssigned        = "-- BOLT -- Player assigned "
		remoteCallbacks       = "-- BOLT -- REMOTE CALLBACKS "
	)
	switch {
	case len(bs) != len(creatingSession) && bytes.HasPrefix(bs, []byte(creatingSession)):
		l.session = string(bs[len(creatingSession):])
		l.setstat("session", bs[len(creatingSession):])
		l.debugf("filterbolt: session=%s", l.session)
	case len(bs) != len(loadingArenaName) && bytes.HasPrefix(bs, []byte(loadingArenaName)):
		l.arena = string(bs[len(loadingArenaName):])
		l.setstat("arena", bs[len(loadingArenaName):])
		l.debugf("filterbolt: arena=%s", l.arena)
	case bytes.HasPrefix(bs, []byte(playerAssigned)) || bytes.HasPrefix(bs, []byte(remoteCallbacks)):
		// Player trying to register.
		l.remstat("idle")
	case len(bs) != len(registeredPlayer) && bytes.HasPrefix(bs, []byte(registeredPlayer)):
		id, _, _, admin := l.players.Add(string(bs[len(registeredPlayer):]))
		if id != -1 && id < 1000 {
			// bots < 1000 < players
			l.changed = true
		}
		players, bots := l.players.Count()
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
		if id != -1 && id < 1000 {
			// bots < 1000 < players
			l.changed = true
		}
		if admin && l.m.id == "" {
			// Reset admin timeout when admin changes.
			l.collect()
		}
		players, bots := l.players.Count()
		l.debugf("filterbolt: players=%d bots=%d id=-%d admin=%t", players, bots, id, admin)
		switch players {
		case 0:
			// Flush match and update timestamp.
			l.collect()
			// Ignore meaningless player joins.
			if l.changed {
				l.idles++
			}
			// Allow lobby to idle unlimited (0) or abs(MaxIdles) times.
			if l.MaxIdles != 0 && (l.MaxIdles > 0 && l.idles >= l.MaxIdles || l.idles >= -l.MaxIdles) {
				l.debugf("filterbolt: players=%d bots=%d idles=%d maxidles=%d reason=%s", players, bots, l.idles, l.MaxIdles, ErrLobbyIdleTimeout)
				l.reason = ErrLobbyIdleTimeout
				l.cancel()
			} else {
				l.newstat("idle")
			}
		case 1, 2, 3, 4, 5, 6, 7, 8:
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
	case bytes.HasPrefix(bs, []byte(gameStateChanged)):
		if l.m.id == "" {
			// Admin timeout otherwise relies on a speedy first kill in round 1.
			l.collect()
		}
		l.changed = true
	}
	return bs, nil
}

func (l *Lobby) scanner(fd int) {
	defer l.cwg.Done()
	defer l.debugf("scanner: done")
	defer l.cancel()
	var r io.Reader
	var w io.Writer
	var p log.Prefix
	switch fd {
	case 1:
		defer close(l.matches)
		defer l.collect()
		defer l.prout.Close()
		r, w, p = l.prout, l.stdout, log.N1
	case 2:
		defer l.prerr.Close()
		r, w, p = l.prerr, l.stderr, log.N2
	default:
		return
	}
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, SetPipeSize), SetPipeSize)
	for s.Scan() {
		bs, err := l.filter(fd, s.Bytes())
		if err != nil {
			l.errorf("scanner: filter: err=%+v fd=%d", err, fd)
			continue
		}
		if len(bs) == 0 {
			continue
		}
		log.Log(w, p, log.Uptime(), bs)
	}
	if s.Err() != nil {
		l.errorf("scanner: err=%+v fd=%d", s.Err(), fd)
	}
}
