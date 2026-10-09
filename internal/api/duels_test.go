package api

import (
	"context"
	"fmt"
	"strings"
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

	// профиль: B 2:8 (20%), C 12:15 (44%) — выигранных пар нет, проигрывает хуже всего B
	path := fmt.Sprintf("/api/players/%d/duels", duelA)
	var pd playerDuelsResponse
	if code := e.do(t, "GET", path+fmt.Sprintf("?session=%d", s1.ID), "", nil, &pd); code != 200 {
		t.Fatalf("дуэли профиля: %d", code)
	}
	if pd.Status != "partial" || pd.EligibleMatches != 3 || pd.CoveredMatches != 2 || pd.CoveredSessions != 1 {
		t.Errorf("покрытие профиля: %+v", pd)
	}
	if len(pd.Beats) != 0 || len(pd.LosesTo) != 1 || pd.LosesTo[0].Name != "bravo" || pd.LosesTo[0].Kills != 2 || pd.LosesTo[0].Deaths != 8 {
		t.Errorf("по доле: %+v / %+v", pd.Beats, pd.LosesTo)
	}

	// все пары 0:0 — показатели пусты, статус complete
	e.do(t, "GET", path+fmt.Sprintf("?session=%d", s2.ID), "", nil, &pd)
	if pd.Status != "complete" || len(pd.Beats) != 0 || len(pd.LosesTo) != 0 || len(pd.Opponents) != 1 || pd.Opponents[0].Share != nil {
		t.Errorf("0:0: %+v", pd)
	}
	// пустой период и только непокрытые матчи
	e.do(t, "GET", path+"?from=2027-01-01", "", nil, &pd)
	if pd.Status != "no_matches" || pd.Beats == nil || pd.LosesTo == nil || pd.Opponents == nil {
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

// Выбор соперников профиля по доле дуэлей (спека personal-duels, «Выигранные и проигранные дуэли в профиле»).
func TestPlayerDuelsByShare(t *testing.T) {
	const (
		duelD uint64 = 76561198000000014
		duelE uint64 = 76561198000000015
	)
	e := newTestEnv(t)
	ctx := context.Background()
	d := &duelSeed{t: t, e: e, names: map[uint64]string{duelA: "alpha", duelB: "bravo", duelC: "charlie", duelD: "delta", duelE: "echo"}}
	path := fmt.Sprintf("/api/players/%d/duels", duelA)
	get := func(opp []uint64, kills, deaths []int) playerDuelsResponse {
		t.Helper()
		sess, _ := e.store.CreateSession(ctx, "2026-10-01", "")
		d.match(sess, store.DuelsSinceVersion, duelA, opp, kills, deaths)
		var pd playerDuelsResponse
		if code := e.do(t, "GET", path+fmt.Sprintf("?session=%d", sess.ID), "", nil, &pd); code != 200 {
			t.Fatalf("дуэли профиля: %d", code)
		}
		return pd
	}
	names := func(list []opponentView) string {
		var out []string
		for _, o := range list {
			out = append(out, o.Name)
		}
		return strings.Join(out, ",")
	}

	// доля важнее объёма: B 30:25 (55%), C 12:7 (63%), D 8:15 (35%)
	pd := get([]uint64{duelB, duelC, duelD}, []int{30, 12, 8}, []int{25, 7, 15})
	if names(pd.Beats) != "charlie" || names(pd.LosesTo) != "delta" {
		t.Errorf("доля важнее объёма: %s / %s", names(pd.Beats), names(pd.LosesTo))
	}

	// равные доли 2:1 и 4:2 — оба, сначала пара с большим числом убийств; D 1:1 — 50%, никуда
	pd = get([]uint64{duelB, duelC, duelD}, []int{2, 4, 1}, []int{1, 2, 1})
	if names(pd.Beats) != "charlie,bravo" || len(pd.LosesTo) != 0 {
		t.Errorf("равные доли: %s / %s", names(pd.Beats), names(pd.LosesTo))
	}

	// маленькая выборка: 1:0 даёт 100% и обходит 20:5; 5:5 не попадает никуда
	pd = get([]uint64{duelB, duelC, duelD}, []int{1, 20, 5}, []int{0, 5, 5})
	if names(pd.Beats) != "bravo" || len(pd.LosesTo) != 0 {
		t.Errorf("маленькая выборка: %s / %s", names(pd.Beats), names(pd.LosesTo))
	}

	// равные проигрыши: 1:3 и 2:6 — оба, сначала пара с большим числом убийств; 0:0 не участвует
	pd = get([]uint64{duelB, duelC, duelE}, []int{1, 2, 0}, []int{3, 6, 0})
	if names(pd.LosesTo) != "charlie,bravo" || len(pd.Beats) != 0 {
		t.Errorf("равные проигрыши: %s / %s", names(pd.Beats), names(pd.LosesTo))
	}
}
