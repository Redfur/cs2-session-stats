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
	p.RegisterEventHandler(b.onKill)
	p.RegisterEventHandler(b.onHurt)
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
	b.rounds = append(b.rounds, Round{Number: len(b.rounds) + 1})
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

func (b *builder) onKill(e events.Kill) {
	if b.cur < 0 {
		return
	}
	k := Kill{
		Time:     b.p.CurrentTime(),
		Killer:   b.id(e.Killer),
		Victim:   b.id(e.Victim),
		Headshot: e.IsHeadshot,
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
	d := Damage{Attacker: b.id(e.Attacker), Victim: b.id(e.Player), Amount: e.HealthDamageTaken}
	if d.Victim == 0 {
		return
	}
	b.rounds[b.cur].Damages = append(b.rounds[b.cur].Damages, d)
}

func (b *builder) onRoundEnd(e events.RoundEnd) {
	if b.cur < 0 || b.rounds[b.cur].Winner != "" {
		return
	}
	switch e.Winner {
	case common.TeamCounterTerrorists, common.TeamTerrorists:
		b.rounds[b.cur].Winner = b.teamOfSide(e.Winner)
	}
	// раунд остаётся текущим до следующего RoundStart, чтобы учесть убийства после окончания раунда
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
	}

	m := Match{Map: b.mapName}
	for _, r := range b.rounds {
		if r.Winner == "" {
			continue // незавершённый раунд (запись оборвалась)
		}
		r.Number = len(m.Rounds) + 1
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
	return m, nil
}
