package match

import "time"

const (
	BlueTeam = Team(0)
	PinkTeam = Team(1)
)

type Team int

type Match struct {
	Timestamp      time.Time `json:"@timestamp"`
	MatchID        string    `json:"matchId,omitempty"`
	ArenaName      string    `json:"arenaName"`
	Team0Score     int       `json:"team0Score"`
	Team1Score     int       `json:"team1Score"`
	MatchStartTime float64   `json:"matchStartTime,omitempty"`
	GameMode       int       `json:"gameMode"`
	Version        string    `json:"version,omitempty"`
	KillData       []Kill    `json:"killData"`
}

type Kill struct {
	ShooterID           string   `json:"shooterId,omitempty"`
	ShooterName         string   `json:"shooterName,omitempty"`
	ShooterTeam         Team     `json:"shooterTeam"`
	ShooterIsBot        bool     `json:"shooterIsBot"`
	EnemyID             string   `json:"enemyId,omitempty"`
	EnemyName           string   `json:"enemyName,omitempty"`
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
	for i := range m.KillData {
		m.KillData[i].ShooterName = ""
		m.KillData[i].ShooterID = ""
		m.KillData[i].EnemyName = ""
		m.KillData[i].EnemyID = ""
	}
	m.MatchID = ""
	m.Version = ""
}
