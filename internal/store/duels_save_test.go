package store

import (
	"context"
	"errors"
	"testing"

	"cs2stats/internal/stats"
)

func duelResult(kills int) MatchResult {
	return MatchResult{
		Map: "de_nuke", Rounds: 13, ScoreA: 13,
		Players: []stats.PlayerStats{
			{SteamID: 1, Name: "a", Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 13, Kills: kills}},
			{SteamID: 2, Name: "b", Team: "B", Result: stats.Loss, Counters: stats.Counters{Rounds: 13}},
		},
		Duels: []stats.Duel{{Killer: 1, Victim: 2, Kills: kills}, {Killer: 2, Victim: 1, Kills: 0}},
	}
}

func TestSaveMatchResultDuels(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	id := addMatches(t, s, sess.ID, "x")[0]

	killsOf := func() (n, rows int) {
		rows = countRows(t, s, "SELECT count(*) FROM match_duels WHERE match_id = ?", id)
		n = countRows(t, s, "SELECT coalesce(sum(kills), 0) FROM match_duels WHERE match_id = ? AND killer_id = 1", id)
		return
	}
	resultVersion := func() int {
		return countRows(t, s, "SELECT result_version FROM matches WHERE id = ?", id)
	}

	if err := s.SaveMatchResult(ctx, id, duelResult(5), 2); err != nil {
		t.Fatal(err)
	}
	if n, rows := killsOf(); n != 5 || rows != 2 || resultVersion() != 2 {
		t.Fatalf("после сохранения: kills=%d rows=%d result_version=%d", n, rows, resultVersion())
	}

	// ошибка пересчёта не трогает дуэли и версию результата
	if err := s.FailMatch(ctx, id, "битая демка", 3); err != nil {
		t.Fatal(err)
	}
	if n, rows := killsOf(); n != 5 || rows != 2 || resultVersion() != 2 {
		t.Fatalf("после ошибки: kills=%d rows=%d result_version=%d", n, rows, resultVersion())
	}

	// два успешных пересчёта дают один набор счётчиков
	for i := 0; i < 2; i++ {
		if err := s.SaveMatchResult(ctx, id, duelResult(7), 3); err != nil {
			t.Fatal(err)
		}
	}
	if n, rows := killsOf(); n != 7 || rows != 2 || resultVersion() != 3 {
		t.Fatalf("после пересчётов: kills=%d rows=%d result_version=%d", n, rows, resultVersion())
	}

	// удаление матча удаляет дуэли; сохранение удалённого матча ничего не воскрешает
	if _, err := s.DeleteMatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMatchResult(ctx, id, duelResult(9), 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("сохранение удалённого матча: %v", err)
	}
	if n := countRows(t, s, "SELECT count(*) FROM match_duels"); n != 0 {
		t.Fatalf("дуэли удалённого матча остались: %d", n)
	}
	if n := countRows(t, s, "SELECT count(*) FROM match_players WHERE match_id = ?", id); n != 0 {
		t.Fatalf("игроки удалённого матча появились: %d", n)
	}
}
