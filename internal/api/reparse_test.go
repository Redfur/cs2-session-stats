package api

import (
	"context"
	"strconv"
	"testing"

	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

func TestReparse(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	sess, _ := e.store.CreateSession(ctx, "2026-10-08", "")
	done, _ := e.store.AddMatch(ctx, sess.ID, "a", "a.dem")
	queued, _ := e.store.AddMatch(ctx, sess.ID, "b", "b.dem") // остаётся pending
	e.store.ClaimNextPending(ctx)
	e.store.SaveMatchResult(ctx, done.ID, store.MatchResult{Map: "de_nuke", Rounds: 1, Players: []stats.PlayerStats{
		{SteamID: 1, Name: "a", Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 1, Kills: 1}},
	}}, 1)
	path := func(id int64) string { return strconv.FormatInt(id, 10) }

	// поля hasResult/processedVersion в ответе матча
	var mr struct {
		Match   store.Match  `json:"match"`
		Players []PlayerView `json:"players"`
	}
	e.do(t, "GET", "/api/matches/"+path(done.ID), "", nil, &mr)
	if !mr.Match.HasResult || mr.Match.ProcessedVersion != 1 {
		t.Fatalf("матч: %+v", mr.Match)
	}

	var m store.Match
	if code := e.do(t, "POST", "/api/matches/"+path(done.ID)+"/reparse", "", nil, &m); code != 202 || m.Status != store.StatusPending || !m.HasResult {
		t.Fatalf("пересчёт матча: %d %+v", code, m)
	}
	if e.woken != 1 {
		t.Errorf("воркер разбужен %d раз", e.woken)
	}
	// во время пересчёта прежние игроки отдаются
	e.do(t, "GET", "/api/matches/"+path(done.ID), "", nil, &mr)
	if len(mr.Players) != 1 {
		t.Errorf("игроки во время пересчёта: %+v", mr.Players)
	}
	// идемпотентность для pending
	if code := e.do(t, "POST", "/api/matches/"+path(queued.ID)+"/reparse", "", nil, &m); code != 202 || m.Status != store.StatusPending {
		t.Fatalf("pending: %d %+v", code, m)
	}

	var errResp map[string]string
	for _, p := range []string{"/api/matches/999/reparse", "/api/sessions/999/reparse"} {
		if code := e.do(t, "POST", p, "", nil, &errResp); code != 404 {
			t.Errorf("%s: %d", p, code)
		}
	}

	// все матчи сессии уже в очереди — ставить нечего
	var q map[string]int64
	if code := e.do(t, "POST", "/api/sessions/"+path(sess.ID)+"/reparse", "", nil, &q); code != 202 || q["queued"] != 0 {
		t.Fatalf("сессия: %d %v", code, q)
	}
	e.store.ClaimNextPending(ctx)
	e.store.FailMatch(ctx, queued.ID, "x", 1)
	if e.do(t, "POST", "/api/sessions/"+path(sess.ID)+"/reparse", "", nil, &q); q["queued"] != 1 {
		t.Fatalf("сессия после ошибки: %v", q)
	}
}
