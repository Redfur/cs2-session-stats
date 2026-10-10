package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cs2stats/internal/parser"
	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

const (
	extA1 uint64 = 76561198000000101
	extA2 uint64 = 76561198000000102
	extB1 uint64 = 76561198000000111
	extB2 uint64 = 76561198000000112
)

func extPlayers() []parser.Player {
	return []parser.Player{
		{SteamID: extA1, Name: "a1", Team: parser.TeamA}, {SteamID: extA2, Name: "a2", Team: parser.TeamA},
		{SteamID: extB1, Name: "b1", Team: parser.TeamB}, {SteamID: extB2, Name: "b2", Team: parser.TeamB},
	}
}

func at(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// extMatch — матч из одного раунда: a1 убил b1 через смок, b2 в клатче 1v2 убил a1 (размен за b1),
// a2 в клатче 1v1 добил b2 с флеш-ассистом a1. Команда A выиграла.
func extMatch() parser.Match {
	r := parser.Round{Number: 1, Winner: parser.TeamA, SideA: parser.SideCT, Start: at(10), End: at(70), Reason: parser.ReasonElimination}
	r.Kills = []parser.Kill{
		{Time: at(34), Killer: extA1, Victim: extB1, Weapon: "ak47", Headshot: true, ThroughSmoke: true, Distance: 1000},
		{Time: at(38), Killer: extB2, Victim: extA1, Weapon: "awp"},
		{Time: at(60), Killer: extA2, Victim: extB2, Weapon: "m4a1_silencer", Assister: extA1, FlashAssist: true},
	}
	r.Damages = []parser.Damage{
		{Attacker: extA1, Victim: extB1, Amount: 100, Weapon: "ak47", Head: true},
		{Attacker: extB2, Victim: extA1, Amount: 100, Weapon: "awp"},
		{Attacker: extA2, Victim: extB2, Amount: 40, Weapon: parser.WeaponHE},
		{Attacker: extA2, Victim: extB2, Amount: 60, Weapon: "m4a1_silencer"},
	}
	r.Throws = []parser.Throw{{Thrower: extA2, Weapon: parser.WeaponSmoke, Projectile: 1}}
	return parser.Match{Map: "de_mirage", Players: extPlayers(), Rounds: []parser.Round{r}, ScoreA: 1, HasDamageEvents: true, HasFlashEvents: true}
}

func saveParsed(t *testing.T, e *testEnv, sess store.Session, sha string, m parser.Match, version int) int64 {
	t.Helper()
	ctx := context.Background()
	added, err := e.store.AddMatch(ctx, sess.ID, sha, sha+".dem")
	if err != nil {
		t.Fatal(err)
	}
	res := store.MatchResult{Map: m.Map, Rounds: len(m.Rounds), ScoreA: m.ScoreA, ScoreB: m.ScoreB,
		Players: stats.Compute(m), Duels: stats.Duels(m)}
	if version >= store.ExtendedSinceVersion {
		ext := stats.ComputeExt(m)
		res.Ext = &ext
	}
	if err := e.store.SaveMatchResult(ctx, added.ID, res, version); err != nil {
		t.Fatal(err)
	}
	return added.ID
}

func TestPlayerExtTabs(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	s1, _ := e.store.CreateSession(ctx, "2026-10-01", "")
	s2, _ := e.store.CreateSession(ctx, "2026-10-08", "")
	saveParsed(t, e, s1, "new", extMatch(), store.ExtendedSinceVersion)
	old := saveParsed(t, e, s2, "old", extMatch(), store.ExtendedSinceVersion-1)
	a1 := fmt.Sprint(extA1)
	if err := e.store.FailMatch(ctx, old, "битая демка", store.ExtendedSinceVersion); err != nil {
		t.Fatal(err)
	}

	var f fightResponse
	if code := e.do(t, "GET", "/api/players/"+a1+"/fight", "", nil, &f); code != 200 {
		t.Fatalf("бой: %d", code)
	}
	if f.Status != "partial" || f.EligibleMatches != 2 || f.CoveredMatches != 1 ||
		len(f.UncoveredMatches) != 1 || f.UncoveredMatches[0].ID != old || f.UncoveredMatches[0].Reason != missingOld {
		t.Errorf("заголовок: %+v", f.extHeader)
	}
	if len(f.Failed) != 1 || f.Failed[0].ID != old || f.Failed[0].Error != "битая демка" || len(f.Reparsing) != 0 {
		t.Errorf("ошибки пересчёта: %+v %+v", f.Failed, f.Reparsing)
	}
	if f.Trades.TradedDeaths != 0 || f.Trades.Deaths != 1 || f.Trades.Kills != 1 || f.Trades.Covered != 1 || f.Trades.Total != 2 {
		t.Errorf("размены: %+v", f.Trades)
	}
	if f.Damage.DealtPerRound == nil || *f.Damage.DealtPerRound != 100 || *f.Damage.TakenPerRound != 100 || *f.Damage.DiffPerRound != 0 {
		t.Errorf("урон: %+v", f.Damage)
	}
	// a1 погиб на 38 с от начала демки, раунд начался на 10 с: 28 с жизни за CT
	if ct := f.Survival.Rows[1]; ct.Side != "CT" || ct.Rounds != 1 || ct.Survived != 0 || ct.AvgAliveSec == nil || *ct.AvgAliveSec != 28 {
		t.Errorf("выживание CT: %+v", ct)
	}
	if tr := f.Survival.Rows[0]; tr.Rounds != 0 || tr.Share != nil || tr.AvgAliveSec != nil {
		t.Errorf("выживание T без раундов: %+v", tr)
	}
	if f.Assists.Flash != 1 || f.Assists.Damage != 0 || f.Assists.Unknown != 1 || f.Assists.Total != 2 {
		t.Errorf("ассисты: %+v", f.Assists)
	}
	if f.Clutches.Sum.Attempts != 0 || f.Clutches.Sum.WinRate != nil || len(f.Clutches.Rows) != 5 {
		t.Errorf("клатчи a1: %+v", f.Clutches)
	}

	// клатч a2 1v1 — победа
	var f2 fightResponse
	e.do(t, "GET", fmt.Sprintf("/api/players/%d/fight", extA2), "", nil, &f2)
	if r := f2.Clutches.Rows[0]; r.Vs != 1 || r.Attempts != 1 || r.Wins != 1 || r.WinRate == nil || *r.WinRate != 1 {
		t.Errorf("клатч a2: %+v", r)
	}

	var wr weaponsResponse
	e.do(t, "GET", "/api/players/"+a1+"/weapons", "", nil, &wr)
	if len(wr.Weapons.Rows) != 1 || wr.Weapons.Rows[0].Weapon != "ak47" || *wr.Weapons.Rows[0].HSKillsPct != 100 || wr.Weapons.Rows[0].HSHitsPct == nil {
		t.Errorf("оружие: %+v", wr.Weapons.Rows)
	}
	if wr.KillDetails.Smoke != 1 || wr.KillDetails.Kills != 1 || wr.KillDetails.AvgDistance == nil || *wr.KillDetails.AvgDistance != 1000 {
		t.Errorf("детали убийств: %+v", wr.KillDetails)
	}

	var u utilityResponse
	e.do(t, "GET", fmt.Sprintf("/api/players/%d/utility", extA2), "", nil, &u)
	if u.Grenades.HE.Value == nil || *u.Grenades.HE.Value != 40 || u.Grenades.HE.Covered != 1 || u.Grenades.HE.Total != 2 {
		t.Errorf("урон HE: %+v", u.Grenades.HE)
	}
	if u.Grenades.Smokes.Value == nil || *u.Grenades.Smokes.Value != 1 || u.Grenades.Flashed.Value == nil || *u.Grenades.Flashed.Value != 0 {
		t.Errorf("смоки и ослепления: %+v %+v", u.Grenades.Smokes, u.Grenades.Flashed)
	}
	if len(u.ByMap) != 1 || u.ByMap[0].Map != "de_mirage" || u.ByMap[0].Matches != 2 || len(u.ByMap[0].HE.Missing) != 1 {
		t.Errorf("по картам: %+v", u.ByMap)
	}

	// период только со старым матчем — расширенные данные недоступны
	e.do(t, "GET", fmt.Sprintf("/api/players/%s/fight?session=%d", a1, s2.ID), "", nil, &f)
	if f.Status != "unavailable" || f.Damage.DealtPerRound != nil {
		t.Errorf("только старый матч: %+v", f)
	}
	// пустой период
	e.do(t, "GET", "/api/players/"+a1+"/fight?from=2030-01-01", "", nil, &f)
	if f.Status != "no_matches" {
		t.Errorf("пустой период: %s", f.Status)
	}
	var errResp map[string]string
	if code := e.do(t, "GET", "/api/players/76561198999999999/utility", "", nil, &errResp); code != 404 {
		t.Errorf("неизвестный игрок: %d", code)
	}
}

func TestMatchAndSessionExt(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	sess, _ := e.store.CreateSession(ctx, "2026-10-08", "")
	id := saveParsed(t, e, sess, "new", extMatch(), store.ExtendedSinceVersion)
	old := saveParsed(t, e, sess, "old", extMatch(), store.ExtendedSinceVersion-1)

	var m matchExtResponse
	if code := e.do(t, "GET", fmt.Sprintf("/api/matches/%d/ext", id), "", nil, &m); code != 200 {
		t.Fatalf("матч: %d", code)
	}
	if m.Status != "complete" || len(m.Rounds) != 1 || m.ExtRounds != 1 || m.DurationSec == nil || *m.DurationSec != 60 {
		t.Fatalf("матч: %+v", m)
	}
	r := m.Rounds[0]
	if r.Reason != parser.ReasonElimination || r.DurationSec == nil || *r.DurationSec != 60 || r.ScoreA != 1 || len(r.Kills) != 3 {
		t.Errorf("раунд: %+v", r)
	}
	if k := r.Kills[1]; k.TimeSec == nil || *k.TimeSec != 28 || !k.Trade || k.KillerID != fmt.Sprint(extB2) {
		t.Errorf("размен в ходе раунда: %+v", k)
	}
	if len(r.Alive["A"]) != 1 || r.Alive["A"][0] != fmt.Sprint(extA2) || len(r.Alive["B"]) != 0 {
		t.Errorf("дожили до конца: %+v", r.Alive)
	}
	if len(r.Clutches) != 2 || r.Clutches[0].SteamID != fmt.Sprint(extA2) || r.Clutches[0].Vs != 1 ||
		r.Clutches[1].Vs != 2 || r.Clutches[1].Outcome != stats.ClutchLoss {
		t.Errorf("клатч раунда: %+v", r.Clutches)
	}
	if len(m.Players) != 4 || len(m.Weapons) == 0 {
		t.Errorf("игроки и оружие: %d %d", len(m.Players), len(m.Weapons))
	}
	e.do(t, "GET", fmt.Sprintf("/api/matches/%d/ext", old), "", nil, &m)
	if m.Status != "unavailable" || len(m.Rounds) != 0 {
		t.Errorf("старый матч: %+v", m)
	}
	var errResp map[string]string
	if code := e.do(t, "GET", "/api/matches/999/ext", "", nil, &errResp); code != 404 {
		t.Errorf("неизвестный матч: %d", code)
	}

	var s sessionExtResponse
	e.do(t, "GET", fmt.Sprintf("/api/sessions/%d/ext", sess.ID), "", nil, &s)
	if s.Status != "partial" || s.EligibleMatches != 2 || s.CoveredMatches != 1 || len(s.Players) != 4 {
		t.Errorf("сессия: %+v", s)
	}
	for _, p := range s.Players {
		if p.SteamID == fmt.Sprint(extA2) && (p.ClutchWins != 1 || p.ClutchAttempts != 1 || p.Matches != 1 || p.HEDamage == nil || *p.HEDamage != 40) {
			t.Errorf("a2 в сессии: %+v", p)
		}
	}
}
