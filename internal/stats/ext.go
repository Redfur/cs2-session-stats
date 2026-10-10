package stats

import (
	"sort"
	"time"

	"cs2stats/internal/parser"
)

// ExtCounters — сырые счётчики расширенной статистики игрока в матче. Раунд, победитель
// которого восстановлен из счёта, в них не входит.
type ExtCounters struct {
	Rounds int `json:"rounds"` // раунды с расширенными данными

	TradedDeaths int `json:"tradedDeaths"` // смерти, которые союзник разменял
	TradeKills   int `json:"tradeKills"`   // убийства в размен

	DamageDealt int `json:"damageDealt"` // урон по здоровью противников
	DamageTaken int `json:"damageTaken"` // урон по здоровью от противников

	FlashAssists  int `json:"flashAssists"`
	DamageAssists int `json:"damageAssists"`

	RoundsT    int   `json:"roundsT"` // раунды с известным началом и концом, по сторонам
	RoundsCT   int   `json:"roundsCT"`
	SurvivedT  int   `json:"survivedT"`
	SurvivedCT int   `json:"survivedCT"`
	AliveMsT   int64 `json:"aliveMsT"` // суммарное время жизни
	AliveMsCT  int64 `json:"aliveMsCT"`

	HEDamage    int `json:"heDamage"`
	FireDamage  int `json:"fireDamage"`
	Flashed     int `json:"flashed"` // пары «флешка — противник» с длительностью > 0
	Smokes      int `json:"smokes"`
	HEKills     int `json:"heKills"`
	FireKills   int `json:"fireKills"`
	ImpactKills int `json:"impactKills"` // удар гранатой

	SmokeKills    int     `json:"smokeKills"`
	WallbangKills int     `json:"wallbangKills"`
	NoScopeKills  int     `json:"noScopeKills"`
	BlindKills    int     `json:"blindKills"`
	DistanceSum   float64 `json:"distanceSum"` // метры
	DistanceKills int     `json:"distanceKills"`

	Plants       int `json:"plants"`
	PlantStarts  int `json:"plantStarts"`
	Defuses      int `json:"defuses"`
	DefuseStarts int `json:"defuseStarts"`
}

// GrenadeKills — все убийства гранатами.
func (c ExtCounters) GrenadeKills() int { return c.HEKills + c.FireKills + c.ImpactKills }

// Add возвращает сумму счётчиков.
func (c ExtCounters) Add(o ExtCounters) ExtCounters {
	return ExtCounters{
		Rounds:        c.Rounds + o.Rounds,
		TradedDeaths:  c.TradedDeaths + o.TradedDeaths,
		TradeKills:    c.TradeKills + o.TradeKills,
		DamageDealt:   c.DamageDealt + o.DamageDealt,
		DamageTaken:   c.DamageTaken + o.DamageTaken,
		FlashAssists:  c.FlashAssists + o.FlashAssists,
		DamageAssists: c.DamageAssists + o.DamageAssists,
		RoundsT:       c.RoundsT + o.RoundsT,
		RoundsCT:      c.RoundsCT + o.RoundsCT,
		SurvivedT:     c.SurvivedT + o.SurvivedT,
		SurvivedCT:    c.SurvivedCT + o.SurvivedCT,
		AliveMsT:      c.AliveMsT + o.AliveMsT,
		AliveMsCT:     c.AliveMsCT + o.AliveMsCT,
		HEDamage:      c.HEDamage + o.HEDamage,
		FireDamage:    c.FireDamage + o.FireDamage,
		Flashed:       c.Flashed + o.Flashed,
		Smokes:        c.Smokes + o.Smokes,
		HEKills:       c.HEKills + o.HEKills,
		FireKills:     c.FireKills + o.FireKills,
		ImpactKills:   c.ImpactKills + o.ImpactKills,
		SmokeKills:    c.SmokeKills + o.SmokeKills,
		WallbangKills: c.WallbangKills + o.WallbangKills,
		NoScopeKills:  c.NoScopeKills + o.NoScopeKills,
		BlindKills:    c.BlindKills + o.BlindKills,
		DistanceSum:   c.DistanceSum + o.DistanceSum,
		DistanceKills: c.DistanceKills + o.DistanceKills,
		Plants:        c.Plants + o.Plants,
		PlantStarts:   c.PlantStarts + o.PlantStarts,
		Defuses:       c.Defuses + o.Defuses,
		DefuseStarts:  c.DefuseStarts + o.DefuseStarts,
	}
}

// PlayerExt — расширенные счётчики игрока матча.
type PlayerExt struct {
	SteamID uint64
	ExtCounters
}

// WeaponStats — счётчики игрока по одному оружию (без гранат и огня).
type WeaponStats struct {
	SteamID uint64
	Weapon  string
	Kills   int
	HSKills int
	Damage  int
	Shots   int
	Hits    int
	HSHits  int
}

// RoundInfo — раунд для хронологии.
type RoundInfo struct {
	Number   int
	Winner   parser.Team
	SideA    parser.Side
	Reason   parser.Reason
	Start    time.Duration
	End      time.Duration
	Restored bool
}

// KillEvent — убийство в ходе раунда. Time — от начала демки.
type KillEvent struct {
	Round         int
	Seq           int
	Time          time.Duration
	Killer        uint64
	Victim        uint64
	Assister      uint64
	Weapon        string
	Headshot      bool
	FlashAssist   bool
	ThroughSmoke  bool
	Wallbang      bool
	NoScope       bool
	AttackerBlind bool
	Distance      float64
	Trade         bool // убийство в размен
}

// ClutchOutcome — исход попытки клатча.
type ClutchOutcome string

const (
	ClutchWin     ClutchOutcome = "win"
	ClutchLoss    ClutchOutcome = "loss"
	ClutchDraw    ClutchOutcome = "draw"
	ClutchUnknown ClutchOutcome = "unknown"
)

// Clutch — попытка клатча: игрок остался один против Vs противников.
type Clutch struct {
	Round   int
	SteamID uint64
	Vs      int
	Outcome ClutchOutcome
}

// Ext — расширенный результат матча.
type Ext struct {
	Players  []PlayerExt
	Weapons  []WeaponStats
	Rounds   []RoundInfo
	Kills    []KillEvent
	Clutches []Clutch

	FirstRound      int
	RestoredRound   bool
	HasDamageEvents bool
	HasFlashEvents  bool
	Duration        time.Duration // от начала первого раунда до конца последнего наблюдаемого
}

// ComputeExt считает расширенную статистику матча. Команды игроков — команды матча,
// как у K и личных дуэлей.
func ComputeExt(m parser.Match) Ext {
	team := teams(m)
	ext := Ext{
		FirstRound:      m.FirstRound(),
		HasDamageEvents: m.HasDamageEvents,
		HasFlashEvents:  m.HasFlashEvents,
	}
	acc := make(map[uint64]*ExtCounters, len(m.Players))
	var order []uint64
	for _, p := range m.Players {
		if acc[p.SteamID] == nil {
			acc[p.SteamID] = &ExtCounters{}
			order = append(order, p.SteamID)
		}
	}
	type wkey struct {
		id     uint64
		weapon string
	}
	weapons := map[wkey]*WeaponStats{}
	weapon := func(id uint64, w string) *WeaponStats {
		k := wkey{id, w}
		if weapons[k] == nil {
			weapons[k] = &WeaponStats{SteamID: id, Weapon: w}
		}
		return weapons[k]
	}
	enemies := func(a, b uint64) bool {
		ta, ok1 := team[a]
		tb, ok2 := team[b]
		return ok1 && ok2 && a != b && ta != tb
	}
	isEnemyKill := func(k parser.Kill) bool { return enemyKill(team, k) }

	var firstStart, lastEnd time.Duration
	for _, r := range m.Rounds {
		ext.Rounds = append(ext.Rounds, RoundInfo{
			Number: r.Number, Winner: r.Winner, SideA: r.SideA, Reason: r.Reason,
			Start: r.Start, End: r.End, Restored: r.Restored,
		})
		if firstStart == 0 && r.Start > 0 {
			firstStart = r.Start
		}
		if r.End > lastEnd {
			lastEnd = r.End
		}
		kills := append([]parser.Kill(nil), r.Kills...)
		sort.SliceStable(kills, func(i, j int) bool { return kills[i].Time < kills[j].Time })

		if r.Restored {
			ext.RestoredRound = true
			ext.Clutches = append(ext.Clutches, clutches(r, kills, team, m.Players)...)
			continue
		}
		for _, c := range acc {
			c.Rounds++
		}

		// размены — до конца раунда, правило KAST
		active := activeKills(r, kills)
		trade := make([]bool, len(active))
		for i, k := range active {
			if !isEnemyKill(k) {
				continue
			}
			for j := i + 1; j < len(active); j++ {
				later := active[j]
				if later.Time-k.Time > TradeWindow {
					break
				}
				if later.Victim == k.Killer && isEnemyKill(later) && team[later.Killer] == team[k.Victim] {
					acc[k.Victim].TradedDeaths++
					trade[j] = true
					break
				}
			}
		}
		for j, k := range active {
			if trade[j] {
				acc[k.Killer].TradeKills++
			}
		}

		for i, k := range kills {
			ext.Kills = append(ext.Kills, KillEvent{
				Round: r.Number, Seq: i + 1, Time: k.Time, Killer: k.Killer, Victim: k.Victim, Assister: k.Assister,
				Weapon: k.Weapon, Headshot: k.Headshot, FlashAssist: k.FlashAssist, ThroughSmoke: k.ThroughSmoke,
				Wallbang: k.Penetrated > 0, NoScope: k.NoScope, AttackerBlind: k.AttackerBlind, Distance: k.Distance,
				Trade: i < len(trade) && trade[i], // active — префикс kills
			})
			if !isEnemyKill(k) {
				continue
			}
			c := acc[k.Killer]
			if a := acc[k.Assister]; a != nil && team[k.Assister] != team[k.Victim] && k.Assister != k.Killer {
				if k.FlashAssist {
					a.FlashAssists++
				} else {
					a.DamageAssists++
				}
			}
			switch {
			case k.Weapon == parser.WeaponHE:
				c.HEKills++
			case parser.IsFire(k.Weapon):
				c.FireKills++
			case parser.IsGrenade(k.Weapon):
				c.ImpactKills++
			default:
				w := weapon(k.Killer, k.Weapon)
				w.Kills++
				if k.Headshot {
					w.HSKills++
				}
			}
			if k.ThroughSmoke {
				c.SmokeKills++
			}
			if k.Penetrated > 0 {
				c.WallbangKills++
			}
			if k.NoScope {
				c.NoScopeKills++
			}
			if k.AttackerBlind {
				c.BlindKills++
			}
			if k.Distance > 0 {
				c.DistanceSum += k.Distance
				c.DistanceKills++
			}
		}

		for _, d := range r.Damages {
			if !enemies(d.Attacker, d.Victim) {
				continue
			}
			acc[d.Attacker].DamageDealt += d.Amount
			acc[d.Victim].DamageTaken += d.Amount
			switch {
			case d.Weapon == parser.WeaponHE:
				acc[d.Attacker].HEDamage += d.Amount
			case parser.IsFire(d.Weapon):
				acc[d.Attacker].FireDamage += d.Amount
			case parser.IsGrenade(d.Weapon):
			default:
				w := weapon(d.Attacker, d.Weapon)
				w.Damage += d.Amount
				w.Hits++
				if d.Head {
					w.HSHits++
				}
			}
		}
		for _, s := range r.Shots {
			// броски гранат и взмахи ножом — не выстрелы
			if acc[s.Shooter] != nil && !parser.IsGrenade(s.Weapon) && s.Weapon != "knife" {
				weapon(s.Shooter, s.Weapon).Shots++
			}
		}
		smokes := map[int64]bool{}
		for _, t := range r.Throws {
			if c := acc[t.Thrower]; c != nil && t.Weapon == parser.WeaponSmoke && !smokes[t.Projectile] {
				smokes[t.Projectile] = true
				c.Smokes++
			}
		}
		type pair struct {
			projectile int64
			victim     uint64
		}
		flashed := map[pair]bool{}
		for _, f := range r.Flashes {
			if f.Duration <= 0 || !enemies(f.Attacker, f.Victim) {
				continue
			}
			p := pair{f.Projectile, f.Victim}
			if f.Projectile != 0 && flashed[p] {
				continue
			}
			flashed[p] = true
			acc[f.Attacker].Flashed++
		}
		for _, b := range r.Bomb {
			c := acc[b.Player]
			if c == nil {
				continue
			}
			switch b.Action {
			case parser.BombPlantBegin:
				c.PlantStarts++
			case parser.BombPlanted:
				c.Plants++
			case parser.BombDefuseBegin:
				c.DefuseStarts++
			case parser.BombDefused:
				c.Defuses++
			}
		}

		survival(r, active, acc, team)
		ext.Clutches = append(ext.Clutches, clutches(r, active, team, m.Players)...)
	}
	if lastEnd > firstStart {
		ext.Duration = lastEnd - firstStart
	}

	for _, id := range order {
		ext.Players = append(ext.Players, PlayerExt{SteamID: id, ExtCounters: *acc[id]})
	}
	for _, w := range weapons {
		ext.Weapons = append(ext.Weapons, *w)
	}
	sort.Slice(ext.Weapons, func(i, j int) bool {
		a, b := ext.Weapons[i], ext.Weapons[j]
		if a.SteamID != b.SteamID {
			return a.SteamID < b.SteamID
		}
		return a.Weapon < b.Weapon
	})
	return ext
}

// activeKills — убийства раунда до RoundEnd включительно (kills отсортированы по времени).
// Если RoundEnd не наблюдался, — все убийства.
func activeKills(r parser.Round, kills []parser.Kill) []parser.Kill {
	if r.End == 0 {
		return kills
	}
	n := sort.Search(len(kills), func(i int) bool { return kills[i].Time > r.End })
	return kills[:n]
}

// survival считает выживание и время жизни по сторонам. Раунд без известного начала
// или конца не учитывается.
func survival(r parser.Round, active []parser.Kill, acc map[uint64]*ExtCounters, team map[uint64]parser.Team) {
	if r.Start == 0 || r.End == 0 || r.End < r.Start {
		return
	}
	died := map[uint64]time.Duration{}
	for _, k := range active {
		if _, ok := died[k.Victim]; !ok {
			died[k.Victim] = k.Time
		}
	}
	for id, c := range acc {
		end, dead := died[id]
		if !dead {
			end = r.End
		}
		alive := end - r.Start
		if alive < 0 {
			alive = 0
		}
		if r.SideOf(team[id]) == parser.SideT {
			c.RoundsT++
			c.AliveMsT += alive.Milliseconds()
			if !dead {
				c.SurvivedT++
			}
		} else {
			c.RoundsCT++
			c.AliveMsCT += alive.Milliseconds()
			if !dead {
				c.SurvivedCT++
			}
		}
	}
}

// clutches находит попытки клатча раунда: после всех смертей одного момента игрок остался
// единственным живым в команде против N ≥ 1 противников. kills — отсортированные убийства
// до конца раунда. Не больше одной попытки на игрока, N фиксируется при начале.
func clutches(r parser.Round, kills []parser.Kill, team map[uint64]parser.Team, players []parser.Player) []Clutch {
	alive := map[parser.Team]map[uint64]bool{parser.TeamA: {}, parser.TeamB: {}}
	for _, p := range players {
		alive[p.Team][p.SteamID] = true
	}
	var out []Clutch
	started := map[uint64]bool{}
	for i := 0; i < len(kills); {
		t := kills[i].Time
		for ; i < len(kills) && kills[i].Time == t; i++ {
			if tm, ok := team[kills[i].Victim]; ok {
				delete(alive[tm], kills[i].Victim)
			}
		}
		for _, tm := range []parser.Team{parser.TeamA, parser.TeamB} {
			other := parser.TeamB
			if tm == parser.TeamB {
				other = parser.TeamA
			}
			if len(alive[tm]) != 1 || len(alive[other]) == 0 {
				continue
			}
			for id := range alive[tm] {
				if !started[id] {
					started[id] = true
					out = append(out, Clutch{Round: r.Number, SteamID: id, Vs: min(len(alive[other]), 5), Outcome: clutchOutcome(r, tm)})
				}
			}
		}
	}
	return out
}

func clutchOutcome(r parser.Round, t parser.Team) ClutchOutcome {
	switch {
	case r.Restored:
		return ClutchUnknown
	case r.Winner == "":
		return ClutchDraw
	case r.Winner == t:
		return ClutchWin
	}
	return ClutchLoss
}
