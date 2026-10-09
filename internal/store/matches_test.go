package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"cs2stats/internal/stats"
)

func TestSessionsList(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	list, err := s.ListSessions(ctx)
	if err != nil || len(list) != 0 {
		t.Fatalf("пустой список: %v, %v", list, err)
	}

	old, _ := s.CreateSession(ctx, "2026-10-01", "")
	recent, _ := s.CreateSession(ctx, "2026-10-08", "Четверговый микс")
	if _, err := s.AddMatch(ctx, recent.ID, "sha-1", "a.dem"); err != nil {
		t.Fatal(err)
	}

	list, err = s.ListSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != recent.ID || list[1].ID != old.ID {
		t.Fatalf("неверный порядок: %+v", list)
	}
	if list[0].MatchCount != 1 || list[1].MatchCount != 0 || list[0].Title != "Четверговый микс" {
		t.Fatalf("неверные данные: %+v", list)
	}

	if _, err := s.GetSession(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидался ErrNotFound, получено %v", err)
	}
}

func TestAddMatchOrdinalAndDuplicate(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	other, _ := s.CreateSession(ctx, "2026-10-09", "")

	// первая загрузка: A, B; догрузка: C
	for i, sha := range []string{"A", "B", "C"} {
		m, err := s.AddMatch(ctx, sess.ID, sha, sha+".dem")
		if err != nil {
			t.Fatal(err)
		}
		if m.Ordinal != i+1 || m.Status != StatusPending {
			t.Fatalf("матч %s: ordinal=%d status=%s", sha, m.Ordinal, m.Status)
		}
	}
	// в другой сессии нумерация своя
	m, _ := s.AddMatch(ctx, other.ID, "D", "D.dem")
	if m.Ordinal != 1 {
		t.Fatalf("ordinal в другой сессии = %d", m.Ordinal)
	}

	_, err := s.AddMatch(ctx, other.ID, "B", "B-copy.dem")
	var dup *DuplicateError
	if !errors.As(err, &dup) {
		t.Fatalf("ожидался DuplicateError, получено %v", err)
	}
	if dup.Existing.SessionID != sess.ID || dup.Existing.Ordinal != 2 {
		t.Fatalf("дубликат указывает не туда: %+v", dup.Existing)
	}

	matches, _ := s.ListSessionMatches(ctx, sess.ID)
	if len(matches) != 3 || matches[0].SHA256 != "A" || matches[2].SHA256 != "C" {
		t.Fatalf("порядок матчей: %+v", matches)
	}
}

func TestQueueLifecycle(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	m1, _ := s.AddMatch(ctx, sess.ID, "1", "1.dem")
	m2, _ := s.AddMatch(ctx, sess.ID, "2", "2.dem")

	got, ok, err := s.ClaimNextPending(ctx)
	if err != nil || !ok || got.ID != m1.ID || got.Status != StatusParsing {
		t.Fatalf("claim #1: %+v ok=%v err=%v", got, ok, err)
	}

	// «перезапуск»: parsing возвращается в очередь
	if n, err := s.ResetParsing(ctx); err != nil || n != 1 {
		t.Fatalf("reset: n=%d err=%v", n, err)
	}
	got, _, _ = s.ClaimNextPending(ctx)
	if got.ID != m1.ID {
		t.Fatalf("после reset ожидался матч %d, получен %d", m1.ID, got.ID)
	}

	players := []stats.PlayerStats{{
		SteamID: 76561198000000001, Name: "alice", Team: "A", Result: stats.Win,
		Counters: stats.Counters{Rounds: 22, Kills: 20, Deaths: 15, Damage: 1800, K1: 8, K2: 3, K3: 2},
	}}
	if err := s.SaveMatchResult(ctx, m1.ID, MatchResult{Map: "de_mirage", Rounds: 22, ScoreA: 13, ScoreB: 9, Players: players}, 1); err != nil {
		t.Fatal(err)
	}
	done, _ := s.GetMatch(ctx, m1.ID)
	if done.Status != StatusDone || done.Map != "de_mirage" || done.ScoreA != 13 || done.ParsedAt == nil {
		t.Fatalf("матч после сохранения: %+v", done)
	}
	saved, _ := s.MatchPlayers(ctx, m1.ID)
	if len(saved) != 1 || saved[0] != players[0] {
		t.Fatalf("игроки: %+v", saved)
	}

	got, _, _ = s.ClaimNextPending(ctx)
	if got.ID != m2.ID {
		t.Fatalf("claim #2: %+v", got)
	}
	if err := s.FailMatch(ctx, m2.ID, "битая демка", 1); err != nil {
		t.Fatal(err)
	}
	failed, _ := s.GetMatch(ctx, m2.ID)
	if failed.Status != StatusFailed || failed.Error != "битая демка" {
		t.Fatalf("failed: %+v", failed)
	}

	if _, ok, _ := s.ClaimNextPending(ctx); ok {
		t.Fatal("очередь должна быть пуста")
	}
}

func TestFailKeepsPreviousResult(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	m, _ := s.AddMatch(ctx, sess.ID, "x", "x.dem")
	if m.HasResult || m.ProcessedVersion != 0 {
		t.Fatalf("новый матч: %+v", m)
	}

	players := []stats.PlayerStats{{SteamID: 1, Name: "a", Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 13, Kills: 9}}}
	s.SaveMatchResult(ctx, m.ID, MatchResult{Map: "de_nuke", Rounds: 13, ScoreA: 13, Players: players}, 1)
	got, _ := s.GetMatch(ctx, m.ID)
	if !got.HasResult || got.ProcessedVersion != 1 {
		t.Fatalf("после успеха: %+v", got)
	}

	// пересчёт новой версией упал
	if err := s.FailMatch(ctx, m.ID, "исходная демка не найдена", 2); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetMatch(ctx, m.ID)
	if got.Status != StatusFailed || !got.HasResult || got.ProcessedVersion != 2 || got.Map != "de_nuke" || got.ScoreA != 13 {
		t.Fatalf("после ошибки: %+v", got)
	}
	if saved, _ := s.MatchPlayers(ctx, m.ID); len(saved) != 1 || saved[0].Kills != 9 {
		t.Fatalf("игроки после ошибки: %+v", saved)
	}
}

// Новая загрузка обрабатывается раньше матчей, стоящих в очереди на пересчёт.
func TestClaimPrefersNewUploads(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	for i := 0; i < 20; i++ {
		m, _ := s.AddMatch(ctx, sess.ID, fmt.Sprintf("old-%d", i), "old.dem")
		s.SaveMatchResult(ctx, m.ID, MatchResult{Rounds: 1}, 1)
	}
	if n, _ := s.RequeueAll(ctx); n != 20 {
		t.Fatalf("в очередь поставлено %d", n)
	}
	fresh, _ := s.AddMatch(ctx, sess.ID, "fresh", "fresh.dem")

	got, ok, err := s.ClaimNextPending(ctx)
	if err != nil || !ok || got.ID != fresh.ID {
		t.Fatalf("первым взят матч %d, ожидался новый %d (err=%v)", got.ID, fresh.ID, err)
	}
	if next, _, _ := s.ClaimNextPending(ctx); !next.HasResult {
		t.Fatalf("затем должен идти пересчёт: %+v", next)
	}
}

func TestSessionsListSummary(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	full, _ := s.CreateSession(ctx, "2026-10-08", "")
	mixed, _ := s.CreateSession(ctx, "2026-10-07", "")
	empty, _ := s.CreateSession(ctx, "2026-10-06", "")

	done := func(sessionID int64, sha, mapName string) {
		t.Helper()
		m, err := s.AddMatch(ctx, sessionID, sha, "m.dem")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SaveMatchResult(ctx, m.ID, MatchResult{Map: mapName, Rounds: 1}, 1); err != nil {
			t.Fatal(err)
		}
	}
	done(full.ID, "f1", "de_dust2")
	done(full.ID, "f2", "de_mirage")
	done(full.ID, "f3", "de_dust2")
	done(mixed.ID, "x1", "de_nuke")
	failed, _ := s.AddMatch(ctx, mixed.ID, "x2", "m.dem")
	s.FailMatch(ctx, failed.ID, "ошибка", 1)
	s.AddMatch(ctx, mixed.ID, "x3", "m.dem") // ещё в очереди

	list, err := s.ListSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]SessionSummary{}
	for _, x := range list {
		got[x.ID] = x
	}
	if x := got[full.ID]; !reflect.DeepEqual(x.Maps, []string{"de_dust2", "de_mirage"}) || x.FailedCount != 0 || x.MatchCount != 3 {
		t.Fatalf("сессия с повтором карты: %+v", x)
	}
	if x := got[mixed.ID]; !reflect.DeepEqual(x.Maps, []string{"de_nuke"}) || x.FailedCount != 1 || x.MatchCount != 3 {
		t.Fatalf("сессия с ошибкой: %+v", x)
	}
	if x := got[empty.ID]; x.Maps == nil || len(x.Maps) != 0 || x.FailedCount != 0 {
		t.Fatalf("пустая сессия: %+v", x)
	}
}
