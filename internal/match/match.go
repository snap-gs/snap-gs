package match

import (
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const (
	BlueTeam = Team(0)
	PinkTeam = Team(1)
)

type Team int

type Match struct {
	Timestamp      time.Time  `json:"@timestamp"`
	MatchID        string     `json:"matchId,omitempty"`
	ArenaName      string     `json:"arenaName"`
	Team0Score     int        `json:"team0Score"`
	Team1Score     int        `json:"team1Score"`
	MatchStartTime float64    `json:"matchStartTime,omitempty"`
	GameMode       int        `json:"gameMode"`
	Version        string     `json:"version,omitempty"`
	KillData       []KillData `json:"killData"`
}

type KillData struct {
	ShooterID           string   `json:"shooterId,omitempty"`
	ShooterName         string   `json:"shooterName"`
	ShooterTeam         Team     `json:"shooterTeam"`
	ShooterIsBot        bool     `json:"shooterIsBot"`
	EnemyID             string   `json:"enemyId,omitempty"`
	EnemyName           string   `json:"enemyName"`
	EnemyTeam           Team     `json:"enemyTeam"`
	EnemyIsBot          bool     `json:"enemyIsBot"`
	ShooterLocation     Location `json:"shooterLocation"`
	ShotOrigin          Location `json:"shotOrigin"`
	ImpactLocation      Location `json:"impactLocation"`
	ImpactLocationLocal Location `json:"impactLocationLocal"`
	ImpactCollider      string   `json:"impactCollider"`
	EnemyLocation       Location `json:"enemyLocation"`
	RoundNumber         int      `json:"roundNumber"`
	RoundStartTime      float64  `json:"roundStartTime"`
	KillTime            float64  `json:"killTime"`
}

type Location struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func (m *Match) Normalize() {
	for i := range m.KillData {
		m.KillData[i].RoundStartTime -= m.MatchStartTime
		m.KillData[i].KillTime -= m.MatchStartTime
	}
	m.MatchStartTime = 0
}

func (m *Match) Anonymize() {
	renames := make(map[string]string, len(m.KillData)/2)
	indexes := make([]int, len(m.KillData))
	players := 1
	for i := range indexes {
		indexes[i] = i
	}
	rand.Shuffle(len(indexes), func(i, j int) {
		indexes[i], indexes[j] = indexes[j], indexes[i]
	})
	for _, i := range indexes {
		for _, name := range []string{m.KillData[i].ShooterName, m.KillData[i].EnemyName} {
			if _, ok := renames[name]; ok {
				continue
			}
			if fields := strings.Fields(name); len(fields) != 2 {
			} else if fields[0] != "Player" && fields[0] != "Bot" {
			} else if _, err := strconv.ParseInt(fields[1], 10, 64); err != nil {
			} else {
				renames[name] = name
				continue
			}
			renames[name] = "Player " + strconv.Itoa(players)
			players++
		}
		m.KillData[i].ShooterName = renames[m.KillData[i].ShooterName]
		m.KillData[i].EnemyName = renames[m.KillData[i].EnemyName]
		m.KillData[i].ShooterID = ""
		m.KillData[i].EnemyID = ""
	}
	m.MatchID = ""
	m.Version = ""
}
