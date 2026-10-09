// Package parser превращает демку CS2 в доменную модель матча, не зависящую от библиотеки парсинга.
package parser

import "time"

// Team — команда в матче. Не совпадает со стороной CT/T: команды меняются
// сторонами после половины, а игрок остаётся в своей команде.
type Team string

const (
	TeamA Team = "A" // команда, начавшая матч за CT
	TeamB Team = "B"
)

type Player struct {
	SteamID uint64
	Name    string
	Team    Team
}

type Kill struct {
	Time     time.Duration // время от начала демки
	Killer   uint64        // 0 — мир (падение, бомба и т.п.)
	Victim   uint64
	Assister uint64 // 0 — нет ассиста; флеш-ассисты не учитываются
	Headshot bool
}

// Damage — урон по здоровью, уже ограниченный оставшимся HP жертвы.
type Damage struct {
	Attacker uint64 // 0 — мир
	Victim   uint64
	Amount   int
}

type Round struct {
	Number  int
	Winner  Team
	Kills   []Kill
	Damages []Damage
}

type Match struct {
	Map     string
	Players []Player
	Rounds  []Round // только завершённые раунды после начала матча
	ScoreA  int
	ScoreB  int
}
