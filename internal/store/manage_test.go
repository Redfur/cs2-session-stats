package store

import (
	"context"
	"errors"
	"testing"

	"cs2stats/internal/stats"
)

// addMatches добавляет в сессию матчи с указанными sha и возвращает их id в том же порядке.
func addMatches(t *testing.T, s *Store, sessionID int64, shas ...string) []int64 {
	t.Helper()
	ids := make([]int64, 0, len(shas))
	for _, sha := range shas {
		m, err := s.AddMatch(context.Background(), sessionID, sha, sha+".dem")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}
	return ids
}

// order возвращает sha матчей сессии по порядку и проверяет, что номера идут 1..N.
func order(t *testing.T, s *Store, sessionID int64) []string {
	t.Helper()
	list, err := s.ListSessionMatches(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	shas := make([]string, 0, len(list))
	for i, m := range list {
		if m.Ordinal != i+1 {
			t.Fatalf("номера не подряд: %d-й матч имеет номер %d", i+1, m.Ordinal)
		}
		shas = append(shas, m.SHA256)
	}
	return shas
}

func equal(a, b []string) bool {
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

func countRows(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestUpdateSession(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	a, _ := s.CreateSession(ctx, "2026-10-08", "Четверг")
	b, _ := s.CreateSession(ctx, "2026-10-09", "")

	got, err := s.UpdateSession(ctx, a.ID, "2026-10-10", "Пятничный микс")
	if err != nil {
		t.Fatal(err)
	}
	if got.Date != "2026-10-10" || got.Title != "Пятничный микс" || got.CreatedAt != a.CreatedAt {
		t.Fatalf("неверный результат: %+v", got)
	}
	list, _ := s.ListSessions(ctx)
	if list[0].ID != a.ID || list[1].ID != b.ID {
		t.Fatalf("список не пересортирован по новой дате: %+v", list)
	}
	if _, err := s.UpdateSession(ctx, 999, "2026-10-10", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидался ErrNotFound, получено %v", err)
	}
}

func TestReorderMatches(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	other, _ := s.CreateSession(ctx, "2026-10-09", "")
	ids := addMatches(t, s, sess.ID, "A", "B", "C")
	foreign := addMatches(t, s, other.ID, "D")

	// сдвиг C вверх
	list, err := s.ReorderMatches(ctx, sess.ID, []int64{ids[0], ids[2], ids[1]})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[1].SHA256 != "C" {
		t.Fatalf("неверный ответ: %+v", list)
	}
	if got := order(t, s, sess.ID); !equal(got, []string{"A", "C", "B"}) {
		t.Fatalf("порядок %v", got)
	}
	// полный разворот: каждый матч меняет номер, UNIQUE не должен мешать
	if _, err := s.ReorderMatches(ctx, sess.ID, []int64{ids[1], ids[2], ids[0]}); err != nil {
		t.Fatal(err)
	}
	if got := order(t, s, sess.ID); !equal(got, []string{"B", "C", "A"}) {
		t.Fatalf("порядок %v", got)
	}

	bad := map[string][]int64{
		"неполный":    {ids[0], ids[1]},
		"чужой":       {ids[0], ids[1], foreign[0]},
		"повтор":      {ids[0], ids[0], ids[1]},
		"лишний":      {ids[0], ids[1], ids[2], foreign[0]},
		"пустой":      {},
		"неизвестный": {ids[0], ids[1], 999},
	}
	for name, list := range bad {
		if _, err := s.ReorderMatches(ctx, sess.ID, list); !errors.Is(err, ErrOrderMismatch) {
			t.Errorf("%s: ожидался ErrOrderMismatch, получено %v", name, err)
		}
	}
	if got := order(t, s, sess.ID); !equal(got, []string{"B", "C", "A"}) {
		t.Fatalf("отклонённый запрос изменил порядок: %v", got)
	}
	if got := order(t, s, other.ID); !equal(got, []string{"D"}) {
		t.Fatalf("затронута другая сессия: %v", got)
	}
	if _, err := s.ReorderMatches(ctx, 999, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестная сессия: ожидался ErrNotFound, получено %v", err)
	}
}

func TestDeleteMatch(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	ids := addMatches(t, s, sess.ID, "A", "B", "C")
	const alice = 76561198000000001
	for i, id := range ids {
		p := stats.PlayerStats{SteamID: alice, Name: "alice", Team: "A", Result: stats.Win,
			Counters: stats.Counters{Rounds: 10, Kills: i + 1}}
		if err := s.SaveMatchResult(ctx, id, MatchResult{Rounds: 10, Players: []stats.PlayerStats{p}}, 1); err != nil {
			t.Fatal(err)
		}
	}

	m, err := s.DeleteMatch(ctx, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if m.SHA256 != "B" {
		t.Fatalf("возвращён не тот матч: %+v", m)
	}
	if got := order(t, s, sess.ID); !equal(got, []string{"A", "C"}) {
		t.Fatalf("порядок после удаления %v", got)
	}
	if n := countRows(t, s, "SELECT count(*) FROM match_players WHERE match_id = ?", ids[1]); n != 0 {
		t.Fatalf("статистика удалённого матча осталась: %d строк", n)
	}
	players, _ := s.SessionPlayers(ctx, sess.ID)
	if len(players) != 1 || players[0].Matches != 2 || players[0].Kills != 1+3 {
		t.Fatalf("итоги сессии посчитаны с удалённым матчем: %+v", players)
	}

	// следующий загруженный файл получает номер 3; тот же sha снова принимается
	again, err := s.AddMatch(ctx, sess.ID, "B", "B.dem")
	if err != nil {
		t.Fatalf("повторная загрузка удалённой демки: %v", err)
	}
	if again.Ordinal != 3 || again.ID == ids[1] {
		t.Fatalf("неверный новый матч: %+v", again)
	}

	if _, err := s.DeleteMatch(ctx, ids[1]); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидался ErrNotFound, получено %v", err)
	}
}

func TestDeleteSession(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	other, _ := s.CreateSession(ctx, "2026-10-09", "")
	empty, _ := s.CreateSession(ctx, "2026-10-10", "")
	ids := addMatches(t, s, sess.ID, "A", "B")
	otherIDs := addMatches(t, s, other.ID, "C")
	p := stats.PlayerStats{SteamID: 1, Name: "x", Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 5}}
	for _, id := range append(ids, otherIDs...) {
		s.SaveMatchResult(ctx, id, MatchResult{Rounds: 5, Players: []stats.PlayerStats{p}}, 1)
	}

	shas, err := s.DeleteSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shas) != 2 || !(shas[0] == "A" && shas[1] == "B" || shas[0] == "B" && shas[1] == "A") {
		t.Fatalf("возвращены sha %v", shas)
	}
	if _, err := s.GetSession(ctx, sess.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("сессия не удалена: %v", err)
	}
	for _, id := range ids {
		if _, err := s.GetMatch(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("матч %d не удалён: %v", id, err)
		}
		if n := countRows(t, s, "SELECT count(*) FROM match_players WHERE match_id = ?", id); n != 0 {
			t.Fatalf("статистика матча %d осталась", id)
		}
	}
	if got := order(t, s, other.ID); !equal(got, []string{"C"}) {
		t.Fatalf("затронута другая сессия: %v", got)
	}
	if n := countRows(t, s, "SELECT count(*) FROM match_players WHERE match_id = ?", otherIDs[0]); n != 1 {
		t.Fatalf("затронута статистика другой сессии")
	}

	shas, err = s.DeleteSession(ctx, empty.ID)
	if err != nil || len(shas) != 0 {
		t.Fatalf("пустая сессия: %v, %v", shas, err)
	}
	if _, err := s.DeleteSession(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидался ErrNotFound, получено %v", err)
	}
}
