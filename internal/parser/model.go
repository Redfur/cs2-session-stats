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

// Side — сторона в раунде.
type Side string

const (
	SideCT Side = "CT"
	SideT  Side = "T"
)

// Other возвращает противоположную сторону.
func (s Side) Other() Side {
	if s == SideCT {
		return SideT
	}
	return SideCT
}

// Reason — причина конца раунда.
type Reason string

const (
	ReasonElimination Reason = "elimination" // все противники убиты
	ReasonBomb        Reason = "bomb"        // взрыв бомбы
	ReasonDefuse      Reason = "defuse"      // бомба разминирована
	ReasonTime        Reason = "time"        // время раунда вышло
	ReasonOther       Reason = "other"       // сдача и прочее
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
	Assister uint64 // 0 — нет ассиста; флеш-ассист тоже считается ассистом
	Headshot bool
	Weapon   string // код оружия, см. WeaponCode

	FlashAssist   bool    // ассист флешкой
	ThroughSmoke  bool    // убийство через дым
	Penetrated    int     // число пробитых препятствий; больше 0 — прострел
	NoScope       bool    // без прицела
	AttackerBlind bool    // убийца был ослеплён
	Distance      float64 // дистанция в единицах игры
}

// Damage — урон по здоровью, уже ограниченный оставшимся HP жертвы.
type Damage struct {
	Attacker uint64 // 0 — мир
	Victim   uint64
	Amount   int
	Weapon   string // код оружия
	Head     bool   // попадание в голову
}

// Shot — выстрел из оружия (событие WeaponFire).
type Shot struct {
	Shooter uint64
	Weapon  string
}

// Throw — брошенная граната.
type Throw struct {
	Thrower    uint64
	Weapon     string // код гранаты
	Projectile int64  // уникальный номер снаряда
}

// Flash — ослепление игрока флешкой.
type Flash struct {
	Attacker   uint64
	Victim     uint64
	Projectile int64 // уникальный номер снаряда; 0 — неизвестен
	Duration   time.Duration
}

// BombAction — действие игрока с бомбой.
type BombAction string

const (
	BombPlantBegin  BombAction = "plant_begin"
	BombPlanted     BombAction = "planted"
	BombDefuseBegin BombAction = "defuse_begin"
	BombDefused     BombAction = "defused"
)

type BombEvent struct {
	Action BombAction
	Player uint64
}

type Round struct {
	Number  int // номер раунда матча; при записи не с начала матча первый номер больше 1
	Winner  Team
	Kills   []Kill
	Damages []Damage

	SideA    Side          // сторона команды A в раунде
	Start    time.Duration // конец freeze time; 0 — неизвестен
	End      time.Duration // событие RoundEnd; 0 — не наблюдалось
	Reason   Reason        // пусто, если RoundEnd не наблюдался
	Restored bool          // победитель восстановлен из итогового счёта

	Shots   []Shot
	Throws  []Throw
	Flashes []Flash
	Bomb    []BombEvent
}

// SideOf возвращает сторону команды в раунде.
func (r Round) SideOf(t Team) Side {
	if t == TeamA {
		return r.SideA
	}
	return r.SideA.Other()
}

type Match struct {
	Map     string
	Players []Player
	Rounds  []Round // только завершённые раунды после начала матча
	ScoreA  int
	ScoreB  int

	// Признаки записи.
	HasDamageEvents bool // в демке есть события урона (или урона быть не могло)
	HasFlashEvents  bool // в демке есть события ослепления (или флешек не бросали)
}

// FirstRound — номер первого записанного раунда; больше 1, если запись началась посреди матча.
func (m Match) FirstRound() int {
	if len(m.Rounds) == 0 {
		return 1
	}
	return m.Rounds[0].Number
}
