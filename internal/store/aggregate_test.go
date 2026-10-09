package store

import (
	"context"
	"math"
	"testing"

	"cs2stats/internal/stats"
)

func TestSessionPlayers(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	const alice = 76561198000000001

	addDone := func(sha string, rounds int, p stats.PlayerStats) {
		m, err := s.AddMatch(ctx, sess.ID, sha, sha+".dem")
		if err != nil {
			t.Fatal(err)
		}
		p.Rounds = rounds
		if err := s.SaveMatchResult(ctx, m.ID, MatchResult{Rounds: rounds, Players: []stats.PlayerStats{p}}, 1); err != nil {
			t.Fatal(err)
		}
	}

	// матч 1: 20 раундов, урон 1600, KAST 10, 10 убийств (5 в голову), rating-счётчики
	addDone("m1", 20, stats.PlayerStats{SteamID: alice, Name: "alice_old", Team: "A", Result: stats.Win,
		Counters: stats.Counters{Kills: 10, HSKills: 5, Deaths: 12, Damage: 1600, KASTRounds: 10, K1: 6, K2: 2}})
	// матч 2: 30 раундов, урон 2400, KAST 27, 30 убийств (3 в голову), ник сменился
	addDone("m2", 30, stats.PlayerStats{SteamID: alice, Name: "alice", Team: "B", Result: stats.Loss,
		Counters: stats.Counters{Kills: 30, HSKills: 3, Deaths: 18, Damage: 2400, KASTRounds: 27, K1: 10, K2: 5, K3: 2, K5: 1}})
	// матч 3: failed при первой обработке (результата нет) — не должен попасть в агрегаты
	m3, _ := s.AddMatch(ctx, sess.ID, "m3", "m3.dem")
	s.FailMatch(ctx, m3.ID, "ошибка", 1)

	players, err := s.SessionPlayers(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 1 {
		t.Fatalf("ожидался 1 игрок, получено %d", len(players))
	}
	p := players[0]
	if p.Name != "alice" {
		t.Errorf("ник = %q, ожидался ник из последнего матча", p.Name)
	}
	if p.Matches != 2 || p.Wins != 1 || p.Rounds != 50 {
		t.Errorf("matches=%d wins=%d rounds=%d", p.Matches, p.Wins, p.Rounds)
	}
	if p.ADR() != 80 {
		t.Errorf("ADR = %v, ожидалось 80", p.ADR())
	}
	// взвешенные проценты: (10+27)/50 и (5+3)/40, а не среднее 50% и 90%
	if p.KASTPercent() != 74 || p.HSPercent() != 20 {
		t.Errorf("KAST=%v HS=%v", p.KASTPercent(), p.HSPercent())
	}
	// rating за сессию = среднее рейтингов матчей, взвешенное по раундам
	r1 := stats.Counters{Rounds: 20, Kills: 10, Deaths: 12, K1: 6, K2: 2}.Rating()
	r2 := stats.Counters{Rounds: 30, Kills: 30, Deaths: 18, K1: 10, K2: 5, K3: 2, K5: 1}.Rating()
	want := (r1*20 + r2*30) / 50
	if math.Abs(p.Rating()-want) > 1e-9 {
		t.Errorf("rating = %v, ожидалось %v", p.Rating(), want)
	}
}

// Матч с результатом входит в итоги, пока пересчитывается и после неудачного пересчёта.
func TestSessionPlayersDuringReprocessing(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	p := stats.PlayerStats{SteamID: 1, Name: "a", Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 10, Kills: 7}}
	m, _ := s.AddMatch(ctx, sess.ID, "m", "m.dem")
	s.SaveMatchResult(ctx, m.ID, MatchResult{Rounds: 10, Players: []stats.PlayerStats{p}}, 1)

	check := func(stage string) {
		t.Helper()
		players, err := s.SessionPlayers(ctx, sess.ID)
		if err != nil || len(players) != 1 || players[0].Kills != 7 {
			t.Fatalf("%s: %+v err=%v", stage, players, err)
		}
	}
	s.RequeueMatch(ctx, m.ID)
	check("в очереди на пересчёт")
	s.ClaimNextPending(ctx)
	check("пересчитывается")
	s.FailMatch(ctx, m.ID, "ошибка", 2)
	check("пересчёт не удался")
}
