package api

import (
	"context"
	"fmt"
	"testing"

	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

const (
	duelA uint64 = 76561198000000011
	duelB uint64 = 76561198000000012
	duelC uint64 = 76561198000000013
)

type duelSeed struct {
	t     *testing.T
	e     *testEnv
	n     int
	names map[uint64]string
}

// match сохраняет матч: игрок «me» против соперников opp, kills[i] и deaths[i] — счёт me против opp[i].
func (d *duelSeed) match(sess store.Session, version int, me uint64, opp []uint64, kills, deaths []int) int64 {
	d.t.Helper()
	ctx := context.Background()
	d.n++
	m, err := d.e.store.AddMatch(ctx, sess.ID, fmt.Sprint("duel", d.n), "m.dem")
	if err != nil {
		d.t.Fatal(err)
	}
	r := store.MatchResult{Map: "de_mirage", Rounds: 20, ScoreA: 13, ScoreB: 7,
		Players: []stats.PlayerStats{{SteamID: me, Name: d.names[me], Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 20}}}}
	for i, o := range opp {
		r.Players = append(r.Players, stats.PlayerStats{SteamID: o, Name: d.names[o], Team: "B", Result: stats.Loss,
			Counters: stats.Counters{Rounds: 20, Kills: deaths[i]}})
		r.Duels = append(r.Duels, stats.Duel{Killer: me, Victim: o, Kills: kills[i]}, stats.Duel{Killer: o, Victim: me, Kills: deaths[i]})
	}
	if err := d.e.store.SaveMatchResult(ctx, m.ID, r, version); err != nil {
		d.t.Fatal(err)
	}
	return m.ID
}

func TestDuelsAPI(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	d := &duelSeed{t: t, e: e, names: map[uint64]string{duelA: "alpha", duelB: "bravo", duelC: "charlie"}}
	s1, _ := e.store.CreateSession(ctx, "2026-10-01", "")
	s2, _ := e.store.CreateSession(ctx, "2026-10-08", "")

	// сессия 1: A против B и C — 1:0 и 12:3, затем 1:8 и 0:12; итог B 2:8 (доля 0.2), C 12:15
	m1 := d.match(s1, store.DuelsSinceVersion, duelA, []uint64{duelB, duelC}, []int{1, 12}, []int{0, 3})
	d.match(s1, store.DuelsSinceVersion, duelA, []uint64{duelB, duelC}, []int{1, 0}, []int{8, 12})
	old := d.match(s1, store.DuelsSinceVersion-1, duelA, []uint64{duelB}, []int{9}, []int{9})
	// сессия 2: A против B 0:0
	d.match(s2, store.DuelsSinceVersion, duelA, []uint64{duelB}, []int{0}, []int{0})

	// матч
	var md matchDuelsResponse
	if code := e.do(t, "GET", fmt.Sprintf("/api/matches/%d/duels", m1), "", nil, &md); code != 200 {
		t.Fatalf("дуэли матча: %d", code)
	}
	if md.Status != "complete" || len(md.Players) != 3 || md.Players[0].SteamID != fmt.Sprint(duelA) || md.Players[0].Team != "A" || len(md.Cells) != 4 {
		t.Errorf("дуэли матча: %+v", md)
	}
	e.do(t, "GET", fmt.Sprintf("/api/matches/%d/duels", old), "", nil, &md)
	if md.Status != "unavailable" || len(md.Cells) != 0 {
		t.Errorf("матч старой версии: %+v", md)
	}
	var errResp map[string]string
	if code := e.do(t, "GET", "/api/matches/999/duels", "", nil, &errResp); code != 404 {
		t.Errorf("неизвестный матч: %d", code)
	}

	// сессия: частичное покрытие
	var sd sessionDuelsResponse
	if code := e.do(t, "GET", fmt.Sprintf("/api/sessions/%d/duels", s1.ID), "", nil, &sd); code != 200 {
		t.Fatalf("дуэли сессии: %d", code)
	}
	if sd.Status != "partial" || sd.EligibleMatches != 3 || sd.CoveredMatches != 2 ||
		len(sd.UncoveredMatches) != 1 || sd.UncoveredMatches[0].ID != old || sd.UncoveredMatches[0].Ordinal != 3 || len(sd.Players) != 3 {
		t.Errorf("дуэли сессии: %+v", sd)
	}
	for _, c := range sd.Cells {
		if c.KillerID == fmt.Sprint(duelA) && c.VictimID == fmt.Sprint(duelB) {
			if c.Kills != 2 || c.Deaths != 8 || c.Maps != 2 || c.Share == nil || *c.Share != 0.2 {
				t.Errorf("A→B в сессии: %+v", c)
			}
		}
	}
	if code := e.do(t, "GET", "/api/sessions/999/duels", "", nil, &errResp); code != 404 {
		t.Errorf("неизвестная сессия: %d", code)
	}
	empty, _ := e.store.CreateSession(ctx, "2026-10-09", "")
	e.do(t, "GET", fmt.Sprintf("/api/sessions/%d/duels", empty.ID), "", nil, &sd)
	if sd.Status != "no_matches" || sd.Players == nil || sd.Cells == nil || sd.UncoveredMatches == nil {
		t.Errorf("пустая сессия: %+v", sd)
	}

	// профиль: равный максимум убийств B и C нет — 2 против 12; погибает чаще от C (15), один соперник в обоих
	path := fmt.Sprintf("/api/players/%d/duels", duelA)
	var pd playerDuelsResponse
	if code := e.do(t, "GET", path+fmt.Sprintf("?session=%d", s1.ID), "", nil, &pd); code != 200 {
		t.Fatalf("дуэли профиля: %d", code)
	}
	if pd.Status != "partial" || pd.EligibleMatches != 3 || pd.CoveredMatches != 2 || pd.CoveredSessions != 1 {
		t.Errorf("покрытие профиля: %+v", pd)
	}
	if len(pd.MostKilled) != 1 || pd.MostKilled[0].Name != "charlie" || pd.MostKilled[0].Kills != 12 || pd.MostKilled[0].Deaths != 15 ||
		len(pd.MostKilledBy) != 1 || pd.MostKilledBy[0].Name != "charlie" {
		t.Errorf("максимумы: %+v / %+v", pd.MostKilled, pd.MostKilledBy)
	}

	// равенство: в сессии 2026-10-01 только по первому матчу B и C — 1 и 12; добавим матч, где у B тоже 12
	s3, _ := e.store.CreateSession(ctx, "2026-10-10", "")
	d.match(s3, store.DuelsSinceVersion, duelA, []uint64{duelB, duelC}, []int{12, 12}, []int{15, 3})
	e.do(t, "GET", path+fmt.Sprintf("?session=%d", s3.ID), "", nil, &pd)
	if len(pd.MostKilled) != 2 || pd.MostKilled[0].Name != "bravo" || pd.MostKilled[1].Name != "charlie" ||
		len(pd.MostKilledBy) != 1 || pd.MostKilledBy[0].Name != "bravo" || pd.Status != "complete" {
		t.Errorf("равенство: %+v / %+v", pd.MostKilled, pd.MostKilledBy)
	}

	// все пары 0:0 — максимумы пусты, статус complete
	e.do(t, "GET", path+fmt.Sprintf("?session=%d", s2.ID), "", nil, &pd)
	if pd.Status != "complete" || len(pd.MostKilled) != 0 || len(pd.MostKilledBy) != 0 || len(pd.Opponents) != 1 || pd.Opponents[0].Share != nil {
		t.Errorf("0:0: %+v", pd)
	}
	// пустой период и только непокрытые матчи
	e.do(t, "GET", path+"?from=2027-01-01", "", nil, &pd)
	if pd.Status != "no_matches" || pd.MostKilled == nil || pd.Opponents == nil {
		t.Errorf("пустой период: %+v", pd)
	}

	if code := e.do(t, "GET", path+"?from=2026-10-01&session=1", "", nil, &errResp); code != 400 {
		t.Errorf("даты и сессии вместе: %d", code)
	}
	if code := e.do(t, "GET", "/api/players/76561198000000099/duels", "", nil, &errResp); code != 404 {
		t.Errorf("неизвестный игрок: %d", code)
	}
}

func TestDuelsStatusUnavailable(t *testing.T) {
	e := newTestEnv(t)
	d := &duelSeed{t: t, e: e, names: map[uint64]string{duelA: "alpha", duelB: "bravo"}}
	s, _ := e.store.CreateSession(context.Background(), "2026-10-01", "")
	d.match(s, store.DuelsSinceVersion-1, duelA, []uint64{duelB}, []int{3}, []int{1})
	var pd playerDuelsResponse
	e.do(t, "GET", fmt.Sprintf("/api/players/%d/duels", duelA), "", nil, &pd)
	if pd.Status != "unavailable" || len(pd.Opponents) != 0 {
		t.Errorf("только старые матчи: %+v", pd)
	}
}
