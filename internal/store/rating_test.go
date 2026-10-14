package store

import (
	"context"
	"testing"
	"time"

	"cs2stats/internal/stats"
)

// matchOrder — порядок матчей в RatingRows (каждый матч один раз).
func matchOrder(t *testing.T, s *Store) []int64 {
	t.Helper()
	rows, err := s.RatingRows(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range rows {
		if len(ids) == 0 || ids[len(ids)-1] != r.MatchID {
			ids = append(ids, r.MatchID)
		}
	}
	return ids
}

func TestRatingRows(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// m7 без результата не входит, остальные — по дате сессии и номеру матча
	if got := matchOrder(t, f.s); !equalIDs(got, f.matches) {
		t.Fatalf("порядок матчей %v, ожидалось %v", got, f.matches)
	}
	rows, _ := f.s.RatingRows(ctx, 1)
	if len(rows) != 9 || rows[0].SteamID != pA || rows[0].Team != "A" || rows[0].Rounds != 10 || rows[0].Kills != 1 {
		t.Fatalf("строки: %d, первая %+v", len(rows), rows[0])
	}

	// матч с неудачным пересчётом остаётся в истории
	if _, err := f.s.RequeueMatch(ctx, f.matches[0]); err != nil {
		t.Fatal(err)
	}
	f.s.ClaimNextPending(ctx)
	f.s.FailMatch(ctx, f.matches[0], "ошибка", 2)
	if got := matchOrder(t, f.s); !equalIDs(got, f.matches) {
		t.Fatalf("после неудачного пересчёта порядок %v", got)
	}

	// перестановка матчей сессии меняет порядок
	if _, err := f.s.ReorderMatches(ctx, f.s2.ID, []int64{f.matches[2], f.matches[1]}); err != nil {
		t.Fatal(err)
	}
	want := []int64{f.matches[0], f.matches[2], f.matches[1], f.matches[3], f.matches[4], f.matches[5]}
	if got := matchOrder(t, f.s); !equalIDs(got, want) {
		t.Fatalf("после перестановки %v, ожидалось %v", got, want)
	}
}

func TestRatingRowsSameDate(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	// вторая сессия той же даты создана позже, а её матч загружен раньше
	first, _ := s.CreateSession(ctx, 1, "2026-10-08", "первая")
	second, _ := s.CreateSession(ctx, 1, "2026-10-08", "вторая")
	b := addMatches(t, s, second.ID, "B")
	a := addMatches(t, s, first.ID, "A")
	for _, id := range []int64{a[0], b[0]} {
		saveResult(t, s, id)
	}
	if got := matchOrder(t, s); !equalIDs(got, []int64{a[0], b[0]}) {
		t.Fatalf("порядок %v: сначала матч первой созданной сессии", got)
	}
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// saveResult сохраняет результат матча с одним игроком.
func saveResult(t *testing.T, s *Store, matchID int64) {
	t.Helper()
	r := MatchResult{Map: "de_dust2", Rounds: 10, ScoreA: 13, ScoreB: 5,
		Players: []stats.PlayerStats{{SteamID: pA, Name: "a", Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 10}}}}
	if err := s.SaveMatchResult(context.Background(), matchID, r, 1); err != nil {
		t.Fatal(err)
	}
}

func TestRatingSwing(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, 1, "2026-10-08", "")
	ids := addMatches(t, s, sess.ID, "old", "new")
	if err := s.SaveMatchResult(ctx, ids[0], extResult(1), SwingSinceVersion-1); err != nil {
		t.Fatal(err)
	}
	r := extResult(1)
	r.Ext.Kills = append([]stats.KillEvent{{Round: 1, Seq: 2, Time: 20 * time.Second, Killer: 2, Victim: 1, Assister: 1}}, r.Ext.Kills...)
	if err := s.SaveMatchResult(ctx, ids[1], r, SwingSinceVersion); err != nil {
		t.Fatal(err)
	}

	got, err := s.RatingSwing(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got[ids[0]]; ok {
		t.Error("у матча прежней версии данных Round Swing нет")
	}
	m, ok := got[ids[1]]
	if !ok || len(m.Rounds) != 1 {
		t.Fatalf("матч версии %d: %+v", SwingSinceVersion, m)
	}
	rd := m.Rounds[0]
	if rd.Winner != "A" || rd.SideA != "CT" || rd.EquipA != 23500 || rd.EquipB != 4100 || rd.PlantMs != 40000 || len(rd.Kills) != 2 {
		t.Fatalf("раунд: %+v", rd)
	}
	// убийства в порядке времени, а не номера
	if rd.Kills[0].TimeMs != 20000 || rd.Kills[0].Killer != 2 || rd.Kills[0].Assister != 1 || rd.Kills[1].Victim != 2 {
		t.Errorf("убийства: %+v", rd.Kills)
	}
}
