package parser

import (
	"errors"
	"fmt"
	"io"
	"os"

	dem "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

// ParseFile разбирает демку с диска.
func ParseFile(path string) (Match, error) {
	f, err := os.Open(path)
	if err != nil {
		return Match{}, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse разбирает демку CS2 из потока.
func Parse(r io.Reader) (m Match, err error) {
	defer func() {
		// demoinfocs может паниковать на повреждённых демках
		if rec := recover(); rec != nil {
			err = fmt.Errorf("демка повреждена или не поддерживается: %v", rec)
		}
	}()

	p := dem.NewParser(r)
	defer p.Close()

	b := &builder{p: p, cur: -1, team: map[uint64]Team{}, name: map[uint64]string{}}
	p.RegisterNetMessageHandler(func(h *msg.CDemoFileHeader) {
		if b.mapName == "" {
			b.mapName = h.GetMapName()
		}
	})
	p.RegisterNetMessageHandler(func(si *msg.CSVCMsg_ServerInfo) {
		if b.mapName == "" {
			b.mapName = si.GetMapName()
		}
	})
	p.RegisterEventHandler(func(events.MatchStart) { b.reset() })
	p.RegisterEventHandler(func(events.RoundStart) { b.roundStart() })
	p.RegisterEventHandler(func(e events.MatchStartedChanged) {
		if !e.NewIsStarted {
			b.cur = -1 // матч окончен: события после него (например, суициды) не учитываем
		}
	})
	p.RegisterEventHandler(func(events.RoundFreezetimeEnd) { b.freezeEnd() })
	p.RegisterEventHandler(b.onKill)
	p.RegisterEventHandler(b.onHurt)
	p.RegisterEventHandler(b.onFire)
	p.RegisterEventHandler(b.onThrow)
	p.RegisterEventHandler(b.onFlash)
	p.RegisterEventHandler(func(e events.BombPlantBegin) { b.onBomb(BombPlantBegin, e.Player) })
	p.RegisterEventHandler(func(e events.BombPlanted) { b.onBomb(BombPlanted, e.Player) })
	p.RegisterEventHandler(func(e events.BombDefuseStart) { b.onBomb(BombDefuseBegin, e.Player) })
	p.RegisterEventHandler(func(e events.BombDefused) { b.onBomb(BombDefused, e.Player) })
	p.RegisterEventHandler(b.onRoundEnd)

	if err := p.ParseToEnd(); err != nil && !errors.Is(err, dem.ErrUnexpectedEndOfDemo) {
		return Match{}, fmt.Errorf("парсинг демки: %w", err)
	}
	return b.finish()
}

// builder собирает матч из событий демки.
type builder struct {
	p       dem.Parser
	mapName string

	rounds []Round
	cur    int // индекс текущего раунда в rounds; -1 — раунд не идёт (разминка)

	sideA     common.Team // сторона, за которую сейчас играет команда A
	sideAKnow bool

	team  map[uint64]Team
	name  map[uint64]string
	order []uint64 // порядок появления игроков
}

// reset отбрасывает всё собранное до (повторного) старта матча: разминку, ножевой раунд, рестарты.
func (b *builder) reset() {
	b.rounds = nil
	b.cur = -1
	b.sideAKnow = false
	b.team = map[uint64]Team{}
	b.name = map[uint64]string{}
	b.order = nil
}

func (b *builder) roundStart() {
	gs := b.p.GameState()
	if gs.IsWarmupPeriod() {
		b.cur = -1
		return
	}
	b.dropRestartedRounds(gs.TotalRoundsPlayed())
	playing := gs.Participants().Playing()
	if !b.sideAKnow {
		// команда A — та, что начала матч за CT
		b.sideA, b.sideAKnow = common.TeamCounterTerrorists, true
	} else {
		// после смены сторон определяем сторону A по большинству её известных игроков
		var ct, t int
		for _, pl := range playing {
			if b.team[pl.SteamID64] != TeamA {
				continue
			}
			switch pl.Team {
			case common.TeamCounterTerrorists:
				ct++
			case common.TeamTerrorists:
				t++
			}
		}
		if ct > t {
			b.sideA = common.TeamCounterTerrorists
		} else if t > ct {
			b.sideA = common.TeamTerrorists
		}
	}
	side := SideCT
	if b.sideA == common.TeamTerrorists {
		side = SideT
	}
	// номер раунда по данным игры: при записи не с начала матча первый раунд больше 1
	b.rounds = append(b.rounds, Round{Number: gs.TotalRoundsPlayed() + 1, SideA: side})
	b.cur = len(b.rounds) - 1
	for _, pl := range playing {
		b.id(pl)
	}
}

// dropRestartedRounds сверяет собранные раунды с числом сыгранных раундов по данным игры.
// Рестарт (mp_restartgame) не всегда сопровождается событием MatchStart: например, в демках
// FastCup первый раунд записи не засчитывается игрой. Если игра насчитала меньше раундов,
// чем собрано, лишние ранние раунды отбрасываются; при обнулении — сбрасываем всё, как при старте матча.
func (b *builder) dropRestartedRounds(played int) {
	completed := 0
	for _, r := range b.rounds {
		if r.Winner != "" {
			completed++
		}
	}
	switch {
	case played >= completed:
		return
	case played == 0:
		b.reset()
	default:
		b.rounds = b.rounds[len(b.rounds)-played:]
		b.cur = -1
	}
}

// id регистрирует игрока и возвращает его SteamID64; 0 — мир, бот или наблюдатель.
func (b *builder) id(pl *common.Player) uint64 {
	if pl == nil || pl.IsBot || pl.SteamID64 == 0 {
		return 0
	}
	if pl.Team != common.TeamCounterTerrorists && pl.Team != common.TeamTerrorists {
		return 0
	}
	sid := pl.SteamID64
	if _, ok := b.team[sid]; !ok {
		t := TeamB
		if pl.Team == b.sideA {
			t = TeamA
		}
		b.team[sid] = t
		b.order = append(b.order, sid)
	}
	b.name[sid] = pl.Name
	return sid
}

// freezeEnd отмечает начало активной части раунда.
func (b *builder) freezeEnd() {
	if b.cur >= 0 && b.rounds[b.cur].Start == 0 {
		b.rounds[b.cur].Start = b.p.CurrentTime()
	}
}

func (b *builder) onKill(e events.Kill) {
	if b.cur < 0 {
		return
	}
	k := Kill{
		Time:          b.p.CurrentTime(),
		Killer:        b.id(e.Killer),
		Victim:        b.id(e.Victim),
		Headshot:      e.IsHeadshot,
		Weapon:        WeaponCode(e.Weapon, ""),
		FlashAssist:   e.AssistedFlash,
		ThroughSmoke:  e.ThroughSmoke,
		Penetrated:    e.PenetratedObjects,
		NoScope:       e.NoScope,
		AttackerBlind: e.AttackerBlind,
		Distance:      float64(e.Distance),
	}
	k.Assister = b.id(e.Assister) // включая флеш-ассисты, как в табло CS2 и FastCup
	if k.Victim == 0 {
		return
	}
	b.rounds[b.cur].Kills = append(b.rounds[b.cur].Kills, k)
}

func (b *builder) onHurt(e events.PlayerHurt) {
	if b.cur < 0 || e.HealthDamageTaken <= 0 {
		return
	}
	d := Damage{
		Attacker: b.id(e.Attacker),
		Victim:   b.id(e.Player),
		Amount:   e.HealthDamageTaken,
		Weapon:   WeaponCode(e.Weapon, e.WeaponString),
		Head:     e.HitGroup == events.HitGroupHead,
	}
	if d.Victim == 0 {
		return
	}
	b.rounds[b.cur].Damages = append(b.rounds[b.cur].Damages, d)
}

func (b *builder) onFire(e events.WeaponFire) {
	if b.cur < 0 {
		return
	}
	if id := b.id(e.Shooter); id != 0 {
		b.rounds[b.cur].Shots = append(b.rounds[b.cur].Shots, Shot{Shooter: id, Weapon: WeaponCode(e.Weapon, "")})
	}
}

func (b *builder) onThrow(e events.GrenadeProjectileThrow) {
	if b.cur < 0 || e.Projectile == nil {
		return
	}
	id := b.id(e.Projectile.Thrower)
	if id == 0 {
		return
	}
	b.rounds[b.cur].Throws = append(b.rounds[b.cur].Throws, Throw{
		Thrower:    id,
		Weapon:     WeaponCode(e.Projectile.WeaponInstance, ""),
		Projectile: e.Projectile.UniqueID(),
	})
}

func (b *builder) onFlash(e events.PlayerFlashed) {
	if b.cur < 0 || e.Player == nil {
		return
	}
	f := Flash{Attacker: b.id(e.Attacker), Victim: b.id(e.Player), Duration: e.FlashDuration()}
	if e.Projectile != nil {
		f.Projectile = e.Projectile.UniqueID()
	}
	if f.Victim == 0 {
		return
	}
	b.rounds[b.cur].Flashes = append(b.rounds[b.cur].Flashes, f)
}

func (b *builder) onBomb(a BombAction, pl *common.Player) {
	if b.cur < 0 {
		return
	}
	if id := b.id(pl); id != 0 {
		b.rounds[b.cur].Bomb = append(b.rounds[b.cur].Bomb, BombEvent{Action: a, Player: id})
	}
}

func (b *builder) onRoundEnd(e events.RoundEnd) {
	if b.cur < 0 || b.rounds[b.cur].Winner != "" {
		return
	}
	switch e.Winner {
	case common.TeamCounterTerrorists, common.TeamTerrorists:
		r := &b.rounds[b.cur]
		r.Winner = b.teamOfSide(e.Winner)
		r.End = b.p.CurrentTime()
		r.Reason = roundReason(e.Reason)
	}
	// раунд остаётся текущим до следующего RoundStart, чтобы учесть убийства после окончания раунда
}

func roundReason(r events.RoundEndReason) Reason {
	switch r {
	case events.RoundEndReasonCTWin, events.RoundEndReasonTerroristsWin:
		return ReasonElimination
	case events.RoundEndReasonTargetBombed:
		return ReasonBomb
	case events.RoundEndReasonBombDefused:
		return ReasonDefuse
	case events.RoundEndReasonTargetSaved:
		return ReasonTime
	}
	return ReasonOther
}

func (b *builder) teamOfSide(side common.Team) Team {
	if side == b.sideA {
		return TeamA
	}
	return TeamB
}

func (b *builder) finish() (Match, error) {
	// В некоторых демках CS2 нет RoundEnd последнего раунда: восстанавливаем победителя по итоговому счёту.
	if n := len(b.rounds); n > 0 && b.rounds[n-1].Winner == "" && b.sideAKnow {
		var a, bb int
		for _, r := range b.rounds {
			switch r.Winner {
			case TeamA:
				a++
			case TeamB:
				bb++
			}
		}
		gs := b.p.GameState()
		scoreA, scoreB := gs.TeamCounterTerrorists().Score(), gs.TeamTerrorists().Score()
		if b.sideA == common.TeamTerrorists {
			scoreA, scoreB = scoreB, scoreA
		}
		switch {
		case scoreA > a:
			b.rounds[n-1].Winner = TeamA
		case scoreB > bb:
			b.rounds[n-1].Winner = TeamB
		}
		b.rounds[n-1].Restored = b.rounds[n-1].Winner != ""
	}

	m := Match{Map: b.mapName}
	first := 1
	for _, r := range b.rounds {
		if r.Winner == "" {
			continue // незавершённый раунд (запись оборвалась)
		}
		if len(m.Rounds) == 0 && r.Number > 1 {
			first = r.Number // запись началась посреди матча
		}
		r.Number = first + len(m.Rounds)
		m.Rounds = append(m.Rounds, r)
		if r.Winner == TeamA {
			m.ScoreA++
		} else {
			m.ScoreB++
		}
	}
	if len(m.Rounds) == 0 {
		return Match{}, errors.New("в демке нет сыгранных раундов матча")
	}
	for _, sid := range b.order {
		m.Players = append(m.Players, Player{SteamID: sid, Name: b.name[sid], Team: b.team[sid]})
	}
	m.HasDamageEvents, m.HasFlashEvents = observed(m.Rounds)
	return m, nil
}

// observed определяет, есть ли в записи события урона и ослепления. Если урона не могло
// быть (нет убийств игроками) или флешек не бросали, отсутствие событий — достоверный ноль.
func observed(rounds []Round) (damage, flash bool) {
	var hurt, kills, flashed, flashThrows bool
	for _, r := range rounds {
		hurt = hurt || len(r.Damages) > 0
		flashed = flashed || len(r.Flashes) > 0
		for _, k := range r.Kills {
			kills = kills || k.Killer != 0
		}
		for _, t := range r.Throws {
			flashThrows = flashThrows || t.Weapon == WeaponFlash
		}
	}
	return hurt || !kills, flashed || !flashThrows
}
