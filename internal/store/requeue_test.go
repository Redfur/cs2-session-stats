package store

import (
	"context"
	"errors"
	"testing"
)

// setupStatuses создаёт матчи в заданных состояниях: done (v1), failed (v1), parsing, pending и done (v2).
func setupStatuses(t *testing.T, s *Store, sessionID int64) map[string]int64 {
	t.Helper()
	ctx := context.Background()
	ids := map[string]int64{}
	for _, sha := range []string{"done-v1", "failed-v1", "parsing", "pending", "done-v2"} {
		m, err := s.AddMatch(ctx, sessionID, sha, sha+".dem")
		if err != nil {
			t.Fatal(err)
		}
		ids[sha] = m.ID
	}
	exec := func(q string, args ...any) {
		if _, err := s.db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE matches SET status='done', has_result=1, processed_version=1 WHERE id=?", ids["done-v1"])
	exec("UPDATE matches SET status='failed', error='x', processed_version=1 WHERE id=?", ids["failed-v1"])
	exec("UPDATE matches SET status='parsing' WHERE id=?", ids["parsing"])
	exec("UPDATE matches SET status='done', has_result=1, processed_version=2 WHERE id=?", ids["done-v2"])
	return ids
}

func statusOf(t *testing.T, s *Store, id int64) MatchStatus {
	t.Helper()
	m, err := s.GetMatch(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return m.Status
}

func TestRequeueOutdated(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	ids := setupStatuses(t, s, sess.ID)

	// версия не менялась: ничего не ставится, включая упавшие
	if n, err := s.RequeueOutdated(ctx, 1); err != nil || n != 0 {
		t.Fatalf("версия 1: n=%d err=%v", n, err)
	}
	// новая версия: done и failed версии 1 — в очередь; parsing/pending/актуальные не трогаем
	n, err := s.RequeueOutdated(ctx, 2)
	if err != nil || n != 2 {
		t.Fatalf("версия 2: n=%d err=%v", n, err)
	}
	want := map[string]MatchStatus{"done-v1": StatusPending, "failed-v1": StatusPending, "parsing": StatusParsing, "pending": StatusPending, "done-v2": StatusDone}
	for sha, st := range want {
		if got := statusOf(t, s, ids[sha]); got != st {
			t.Errorf("%s: %s, ожидалось %s", sha, got, st)
		}
	}
	if m, _ := s.GetMatch(ctx, ids["failed-v1"]); m.Error != "" {
		t.Errorf("ошибка не сброшена при постановке в очередь: %q", m.Error)
	}
}

func TestRequeueManual(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	other, _ := s.CreateSession(ctx, "2026-10-09", "")
	ids := setupStatuses(t, s, sess.ID)
	foreign, _ := s.AddMatch(ctx, other.ID, "foreign", "f.dem")
	s.db.Exec("UPDATE matches SET status='done', has_result=1, processed_version=1 WHERE id=?", foreign.ID)

	// отдельный матч: done и failed переходят в pending, parsing — без изменений
	for _, sha := range []string{"done-v2", "failed-v1"} {
		m, err := s.RequeueMatch(ctx, ids[sha])
		if err != nil || m.Status != StatusPending {
			t.Fatalf("%s: %+v err=%v", sha, m, err)
		}
	}
	if m, err := s.RequeueMatch(ctx, ids["parsing"]); err != nil || m.Status != StatusParsing {
		t.Fatalf("parsing: %+v err=%v", m, err)
	}
	if _, err := s.RequeueMatch(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестный матч: %v", err)
	}

	// сессия: остался один done-v1 вне очереди; чужая сессия не затрагивается
	if n, err := s.RequeueSession(ctx, sess.ID); err != nil || n != 1 {
		t.Fatalf("сессия: n=%d err=%v", n, err)
	}
	if statusOf(t, s, foreign.ID) != StatusDone {
		t.Fatal("затронут матч другой сессии")
	}
	if n, err := s.RequeueAll(ctx); err != nil || n != 1 {
		t.Fatalf("все: n=%d err=%v", n, err)
	}
}
