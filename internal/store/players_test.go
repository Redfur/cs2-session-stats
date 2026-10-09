package store

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"cs2stats/internal/stats"
)

const (
	pA uint64 = 76561198000000001
	pB uint64 = 76561198000000002
	pG uint64 = 76561198000000003 // гость
)

type fixture struct {
	s          *Store
	s1, s2, s3 Session // 2026-09-20, 2026-10-01, 2026-10-08
	matches    []int64 // id матчей m1..m6 по порядку создания
}

// newFixture: A сыграл 5 матчей, B — 3 (2 из них вместе с A), гость — 1.
//
//	s1 (09-20): m1 de_inferno A, G
//	s2 (10-01): m2 de_mirage A(alpha), B; m3 de_mirage A(alpha)
//	s3 (10-08): m4 de_mirage A(beta), B; m5 de_nuke A(beta); m6 de_nuke B; m7 failed без результата
func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	f := &fixture{s: openTest(t)}
	f.s1, _ = f.s.CreateSession(ctx, "2026-09-20", "")
	f.s2, _ = f.s.CreateSession(ctx, "2026-10-01", "")
	f.s3, _ = f.s.CreateSession(ctx, "2026-10-08", "четверг")
	type pl struct {
		id   uint64
		name string
	}
	add := func(sess Session, mapName string, players ...pl) {
		n := len(f.matches) + 1
		m, err := f.s.AddMatch(ctx, sess.ID, fmt.Sprintf("m%d", n), "m.dem")
		if err != nil {
			t.Fatal(err)
		}
		var ps []stats.PlayerStats
		for i, p := range players {
			team, res := "A", stats.Win
			if i%2 == 1 {
				team, res = "B", stats.Loss
			}
			ps = append(ps, stats.PlayerStats{SteamID: p.id, Name: p.name, Team: team, Result: res,
				Counters: stats.Counters{Rounds: 10, Kills: n, Damage: 100 * n}})
		}
		r := MatchResult{Map: mapName, Rounds: 10, ScoreA: 13, ScoreB: n, Players: ps}
		if err := f.s.SaveMatchResult(ctx, m.ID, r, 1); err != nil {
			t.Fatal(err)
		}
		f.matches = append(f.matches, m.ID)
	}
	add(f.s1, "de_inferno", pl{pA, "a_old"}, pl{pG, "guest"})
	add(f.s2, "de_mirage", pl{pA, "alpha"}, pl{pB, "bob"})
	add(f.s2, "de_mirage", pl{pA, "alpha"})
	add(f.s3, "de_mirage", pl{pA, "beta"}, pl{pB, "bob"})
	add(f.s3, "de_nuke", pl{pA, "beta"})
	add(f.s3, "de_nuke", pl{pB, "bob"})
	m7, _ := f.s.AddMatch(ctx, f.s3.ID, "m7", "m.dem")
	f.s.FailMatch(ctx, m7.ID, "ошибка", 1)
	return f
}

// totals возвращает число матчей по SteamID.
func (f *fixture) totals(t *testing.T, flt MatchFilter, minMatches int) map[uint64]int {
	t.Helper()
	list, err := f.s.PlayerTotals(context.Background(), flt, minMatches)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uint64]int{}
	for _, p := range list {
		got[p.SteamID] = p.Matches
	}
	return got
}

func TestPlayerTotalsFilters(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name string
		flt  MatchFilter
		min  int
		want map[uint64]int
	}{
		{"всё время", MatchFilter{}, 1, map[uint64]int{pA: 5, pB: 3, pG: 1}},
		{"диапазон дат", MatchFilter{From: "2026-10-01", To: "2026-10-08"}, 1, map[uint64]int{pA: 4, pB: 3}},
		{"только нижняя граница", MatchFilter{From: "2026-10-08"}, 1, map[uint64]int{pA: 2, pB: 2}},
		{"только верхняя граница", MatchFilter{To: "2026-09-20"}, 1, map[uint64]int{pA: 1, pG: 1}},
		{"несколько сессий", MatchFilter{SessionIDs: []int64{f.s1.ID, f.s3.ID}}, 1, map[uint64]int{pA: 3, pB: 2, pG: 1}},
		{"выбраны игроки", MatchFilter{PlayerIDs: []uint64{pA, pB}}, 1, map[uint64]int{pA: 5, pB: 3}},
		{"совместные матчи", MatchFilter{PlayerIDs: []uint64{pA, pB}, Together: true}, 1, map[uint64]int{pA: 2, pB: 2}},
		{"совместные без игроков", MatchFilter{Together: true}, 1, map[uint64]int{pA: 5, pB: 3, pG: 1}},
		{"повтор игрока в фильтре", MatchFilter{PlayerIDs: []uint64{pA, pA}, Together: true}, 1, map[uint64]int{pA: 5}},
		{"порог отсекает гостя", MatchFilter{}, 3, map[uint64]int{pA: 5, pB: 3}},
		{"порог с учётом периода", MatchFilter{From: "2026-10-08"}, 3, map[uint64]int{}},
		{"пустой период", MatchFilter{From: "2027-01-01"}, 1, map[uint64]int{}},
	}
	for _, c := range cases {
		if got := f.totals(t, c.flt, c.min); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, ожидалось %v", c.name, got, c.want)
		}
	}
}

func TestPlayerTotalsSums(t *testing.T) {
	f := newFixture(t)
	list, _ := f.s.PlayerTotals(context.Background(), MatchFilter{PlayerIDs: []uint64{pB}}, 1)
	if len(list) != 1 {
		t.Fatalf("%+v", list)
	}
	// B: матчи 2, 4, 6 по 10 раундов, урон 100*n, в m2 и m4 — команда B (поражение), в m6 — A (победа)
	p := list[0]
	if p.Rounds != 30 || p.Kills != 12 || p.Damage != 1200 || p.Wins != 1 || p.ADR() != 40 {
		t.Errorf("%+v", p)
	}
}

func TestPlayerTotalsName(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	name := func(flt MatchFilter) string {
		t.Helper()
		flt.PlayerIDs = []uint64{pA}
		list, err := f.s.PlayerTotals(ctx, flt, 1)
		if err != nil || len(list) != 1 {
			t.Fatalf("%+v err=%v", list, err)
		}
		return list[0].Name
	}
	if n := name(MatchFilter{}); n != "beta" {
		t.Errorf("без фильтров: %q", n)
	}
	if n := name(MatchFilter{From: "2026-10-01", To: "2026-10-01"}); n != "alpha" {
		t.Errorf("за 2026-10-01: %q", n)
	}
	if n := name(MatchFilter{SessionIDs: []int64{f.s1.ID}}); n != "a_old" {
		t.Errorf("за первую сессию: %q", n)
	}

	// при равной дате последней считается сессия, созданная позже, даже если её матч загружен раньше
	later, _ := f.s.CreateSession(ctx, "2026-10-08", "")
	m, _ := f.s.AddMatch(ctx, later.ID, "late", "m.dem")
	f.s.SaveMatchResult(ctx, m.ID, MatchResult{Rounds: 1, Players: []stats.PlayerStats{{SteamID: pA, Name: "gamma"}}}, 1)
	if n := name(MatchFilter{}); n != "gamma" {
		t.Errorf("сессии одной даты: %q", n)
	}
}

func TestPlayerProfileQueries(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	sessions, err := f.s.PlayerSessions(ctx, pA, MatchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var gotSess []string
	for _, p := range sessions {
		gotSess = append(gotSess, fmt.Sprintf("%s:%d", p.Session.Date, p.Matches))
	}
	if want := []string{"2026-10-08:2", "2026-10-01:2", "2026-09-20:1"}; !reflect.DeepEqual(gotSess, want) {
		t.Errorf("по сессиям: %v", gotSess)
	}
	if sessions[0].Session.Title != "четверг" || sessions[0].Damage != 900 {
		t.Errorf("сессия s3: %+v", sessions[0])
	}

	maps, err := f.s.PlayerMaps(ctx, pA, MatchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var gotMaps []string
	for _, p := range maps {
		gotMaps = append(gotMaps, fmt.Sprintf("%s:%d", p.Map, p.Matches))
	}
	if want := []string{"de_mirage:3", "de_inferno:1", "de_nuke:1"}; !reflect.DeepEqual(gotMaps, want) {
		t.Errorf("по картам: %v", gotMaps)
	}

	matches, err := f.s.PlayerMatches(ctx, pA, MatchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var gotIDs []int64
	for _, p := range matches {
		gotIDs = append(gotIDs, p.MatchID)
	}
	m := f.matches
	if want := []int64{m[4], m[3], m[2], m[1], m[0]}; !reflect.DeepEqual(gotIDs, want) {
		t.Errorf("матчи: %v, ожидалось %v", gotIDs, want)
	}
	if p := matches[0]; p.Map != "de_nuke" || p.Ordinal != 2 || p.SessionDate != "2026-10-08" ||
		p.Result != stats.Win || p.Kills != 5 || p.ScoreB != 5 {
		t.Errorf("матч m5: %+v", p)
	}

	// период фильтрует все разбивки
	period := MatchFilter{SessionIDs: []int64{f.s2.ID}}
	sessions, _ = f.s.PlayerSessions(ctx, pA, period)
	maps, _ = f.s.PlayerMaps(ctx, pA, period)
	matches, _ = f.s.PlayerMatches(ctx, pA, period)
	if len(sessions) != 1 || len(maps) != 1 || len(matches) != 2 {
		t.Errorf("за сессию s2: %d сессий, %d карт, %d матчей", len(sessions), len(maps), len(matches))
	}

	// пустой период — пустые разбивки, не nil
	empty := MatchFilter{From: "2027-01-01"}
	sessions, _ = f.s.PlayerSessions(ctx, pA, empty)
	maps, _ = f.s.PlayerMaps(ctx, pA, empty)
	matches, _ = f.s.PlayerMatches(ctx, pA, empty)
	if sessions == nil || maps == nil || matches == nil || len(sessions)+len(maps)+len(matches) != 0 {
		t.Errorf("пустой период: %v %v %v", sessions, maps, matches)
	}
}

// Игрок, у которого есть только матч без результата, не находится.
func TestPlayerTotalsOnlyFailed(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	m1, _ := s.AddMatch(ctx, sess.ID, "ok", "m.dem")
	s.SaveMatchResult(ctx, m1.ID, MatchResult{Rounds: 1, Players: []stats.PlayerStats{{SteamID: pA, Name: "a"}}}, 1)
	// матч, пересчёт которого упал, сохраняет результат и остаётся в выборке
	s.RequeueMatch(ctx, m1.ID)
	s.ClaimNextPending(ctx)
	s.FailMatch(ctx, m1.ID, "ошибка", 2)
	m2, _ := s.AddMatch(ctx, sess.ID, "bad", "m.dem")
	s.FailMatch(ctx, m2.ID, "ошибка", 1)

	list, err := s.PlayerTotals(ctx, MatchFilter{PlayerIDs: []uint64{pB}}, 1)
	if err != nil || len(list) != 0 {
		t.Errorf("игрок без матчей с результатом: %+v err=%v", list, err)
	}
	list, _ = s.PlayerTotals(ctx, MatchFilter{PlayerIDs: []uint64{pA}}, 1)
	if len(list) != 1 || list[0].Matches != 1 {
		t.Errorf("матч с неудачным пересчётом: %+v", list)
	}
}
