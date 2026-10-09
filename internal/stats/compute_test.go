package stats

import (
	"math"
	"testing"
	"time"

	"cs2stats/internal/parser"
)

// Игроки: a1, a2 — команда A; b1, b2 — команда B.
const (
	a1 uint64 = 1
	a2 uint64 = 2
	b1 uint64 = 11
	b2 uint64 = 12
)

func players() []parser.Player {
	return []parser.Player{
		{SteamID: a1, Name: "a1", Team: parser.TeamA},
		{SteamID: a2, Name: "a2", Team: parser.TeamA},
		{SteamID: b1, Name: "b1", Team: parser.TeamB},
		{SteamID: b2, Name: "b2", Team: parser.TeamB},
	}
}

func sec(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

func kill(t float64, killer, victim uint64) parser.Kill {
	return parser.Kill{Time: sec(t), Killer: killer, Victim: victim}
}

func compute(t *testing.T, rounds []parser.Round, scoreA, scoreB int) map[uint64]PlayerStats {
	t.Helper()
	res := Compute(parser.Match{Players: players(), Rounds: rounds, ScoreA: scoreA, ScoreB: scoreB})
	byID := map[uint64]PlayerStats{}
	for _, p := range res {
		byID[p.SteamID] = p
	}
	if len(byID) != 4 {
		t.Fatalf("ожидалось 4 игрока, получено %d", len(byID))
	}
	return byID
}

func TestBasicCounters(t *testing.T) {
	hs := kill(10, a1, b1)
	hs.Headshot = true
	withAssist := kill(20, a1, b2)
	withAssist.Assister = a2
	r := compute(t, []parser.Round{{Kills: []parser.Kill{hs, withAssist}}}, 1, 0)

	got := r[a1]
	if got.Kills != 2 || got.HSKills != 1 || got.HSPercent() != 50 || got.Deaths != 0 {
		t.Errorf("a1: %+v", got.Counters)
	}
	if r[a2].Assists != 1 {
		t.Errorf("a2 assists = %d", r[a2].Assists)
	}
	if r[b1].Deaths != 1 || r[b2].Deaths != 1 {
		t.Errorf("смерти b1=%d b2=%d", r[b1].Deaths, r[b2].Deaths)
	}
	if r[a1].Result != Win || r[b1].Result != Loss {
		t.Errorf("исходы: a1=%s b1=%s", r[a1].Result, r[b1].Result)
	}
	if r[b1].HSPercent() != 0 {
		t.Errorf("HS%% без убийств = %v", r[b1].HSPercent())
	}
}

func TestTeamkillAndSuicide(t *testing.T) {
	assistOnTeamkill := kill(5, a1, a2)
	assistOnTeamkill.Assister = a2 // нелепый, но возможный в демке ассист — не засчитывается
	rounds := []parser.Round{{Kills: []parser.Kill{
		assistOnTeamkill,
		kill(6, b1, b1), // суицид
		kill(7, 0, b2),  // смерть от мира (падение)
	}}}
	r := compute(t, rounds, 0, 0)

	if r[a1].Kills != 0 || r[a2].Deaths != 1 || r[a2].Assists != 0 {
		t.Errorf("тимкилл: a1=%+v a2=%+v", r[a1].Counters, r[a2].Counters)
	}
	if r[b1].Kills != 0 || r[b1].Deaths != 1 || r[b2].Deaths != 1 {
		t.Errorf("суицид/мир: b1=%+v b2=%+v", r[b1].Counters, r[b2].Counters)
	}
	if r[a1].OpeningKills != 0 || r[a2].OpeningDeaths != 0 {
		t.Errorf("тимкилл не должен быть opening duel")
	}
	if r[a1].Result != Draw {
		t.Errorf("исход при 0:0 = %s", r[a1].Result)
	}
}

func TestADR(t *testing.T) {
	rounds := []parser.Round{
		{Damages: []parser.Damage{
			{Attacker: a1, Victim: b1, Amount: 40}, // уже ограничено оставшимся HP
			{Attacker: a1, Victim: a2, Amount: 50}, // по союзнику — не считается
			{Attacker: 0, Victim: a1, Amount: 10},  // мир
		}},
		{Damages: []parser.Damage{{Attacker: a1, Victim: b2, Amount: 100}}},
	}
	r := compute(t, rounds, 1, 1)
	if r[a1].Damage != 140 || r[a1].ADR() != 70 {
		t.Errorf("a1 damage=%d adr=%v", r[a1].Damage, r[a1].ADR())
	}
	if r[a1].Rounds != 2 {
		t.Errorf("rounds = %d", r[a1].Rounds)
	}
}

func TestKAST(t *testing.T) {
	rounds := []parser.Round{
		// раунд 1: a1 убит b1, через 3 с a2 убивает b1 — размен
		{Kills: []parser.Kill{kill(10, b1, a1), kill(13, a2, b1)}},
		// раунд 2: a1 убит b1, a2 мстит через 6 с — уже не размен
		{Kills: []parser.Kill{kill(10, b1, a1), kill(16, a2, b1)}},
		// раунд 3: никто не умер
		{},
	}
	r := compute(t, rounds, 2, 1)
	// a1: р1 размен, р2 ничего, р3 выжил
	if r[a1].KASTRounds != 2 || math.Abs(r[a1].KASTPercent()-200.0/3) > 1e-9 {
		t.Errorf("a1 KAST = %d (%v%%)", r[a1].KASTRounds, r[a1].KASTPercent())
	}
	// b1: р1 убийство, р2 убийство, р3 выжил
	if r[b1].KASTRounds != 3 {
		t.Errorf("b1 KAST = %d", r[b1].KASTRounds)
	}
	// b2: жив во всех раундах
	if r[b2].KASTRounds != 3 {
		t.Errorf("b2 KAST = %d", r[b2].KASTRounds)
	}
}

func TestMultiKillsAndOpenings(t *testing.T) {
	// команда B расширяется до 5 игроков, чтобы сделать эйс
	ps := players()
	for id := uint64(13); id <= 15; id++ {
		ps = append(ps, parser.Player{SteamID: id, Team: parser.TeamB})
	}
	ace := parser.Round{Kills: []parser.Kill{
		kill(1, a1, b1), kill(2, a1, b2), kill(3, a1, 13), kill(4, a1, 14), kill(5, a1, 15),
	}}
	double := parser.Round{Kills: []parser.Kill{kill(1, b1, a2), kill(2, a1, b1), kill(3, a1, b2)}}

	res := Compute(parser.Match{Players: ps, Rounds: []parser.Round{ace, double}, ScoreA: 2})
	got := map[uint64]PlayerStats{}
	for _, p := range res {
		got[p.SteamID] = p
	}
	c := got[a1].Counters
	if c.K5 != 1 || c.K2 != 1 || c.K3 != 0 || c.K4 != 0 || c.K1 != 0 {
		t.Errorf("мульти-киллы a1: %+v", c)
	}
	if c.OpeningKills != 1 || got[b1].OpeningDeaths != 1 {
		t.Errorf("opening: a1 ok=%d, b1 od=%d", c.OpeningKills, got[b1].OpeningDeaths)
	}
	if got[b1].OpeningKills != 1 || got[a2].OpeningDeaths != 1 {
		t.Errorf("opening раунда 2: b1 ok=%d, a2 od=%d", got[b1].OpeningKills, got[a2].OpeningDeaths)
	}
}

func TestDuplicatePlayersMerged(t *testing.T) {
	ps := append(players(), parser.Player{SteamID: a1, Name: "a1", Team: parser.TeamA})
	res := Compute(parser.Match{Players: ps, Rounds: []parser.Round{{Kills: []parser.Kill{kill(1, a1, b1)}}}})
	if len(res) != 4 {
		t.Fatalf("ожидалось 4 строки, получено %d", len(res))
	}
}

func TestRatingAverage(t *testing.T) {
	// средние значения формулы: 0.679 убийств, 0.317 выживаемости и 1.277 RMK на раунд
	c := Counters{Rounds: 1000, Kills: 679, Deaths: 683, K1: 1277}
	if math.Abs(c.Rating()-1) > 1e-9 {
		t.Errorf("rating = %v, ожидалось 1.00", c.Rating())
	}
	if (Counters{}).Rating() != 0 {
		t.Error("rating без раундов должен быть 0")
	}
}
