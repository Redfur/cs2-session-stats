package store

import (
	"context"
	"testing"

	"cs2stats/internal/stats"
)

// saveDuels сохраняет матч с игроками команд A и B и счётом каждой пары.
// k[[2]uint64{x, y}] — сколько раз x убил y; пары соперников без записи — нули.
func saveDuels(t *testing.T, s *Store, id int64, teamA, teamB []uint64, k map[[2]uint64]int, version int) {
	t.Helper()
	var r MatchResult
	r.Rounds = 13
	team := map[uint64]string{}
	for _, p := range teamA {
		team[p] = "A"
	}
	for _, p := range teamB {
		team[p] = "B"
	}
	for p, tm := range team {
		r.Players = append(r.Players, stats.PlayerStats{SteamID: p, Name: "p" + string(rune('0'+p%10)), Team: tm, Result: stats.Draw})
	}
	for x := range team {
		for y := range team {
			if team[x] != team[y] {
				r.Duels = append(r.Duels, stats.Duel{Killer: x, Victim: y, Kills: k[[2]uint64{x, y}]})
			}
		}
	}
	if err := s.SaveMatchResult(context.Background(), id, r, version); err != nil {
		t.Fatal(err)
	}
}

func cellOf(cells []DuelCell, killer, victim uint64) (DuelCell, bool) {
	for _, c := range cells {
		if c.Killer == killer && c.Victim == victim {
			return c, true
		}
	}
	return DuelCell{}, false
}

const (
	dA uint64 = 1
	dB uint64 = 2
	dC uint64 = 3
)

func TestSessionAndPlayerDuels(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	s1, _ := s.CreateSession(ctx, "2026-10-01", "")
	s2, _ := s.CreateSession(ctx, "2026-10-08", "")
	s3, _ := s.CreateSession(ctx, "2026-10-09", "")
	ids := addMatches(t, s, s1.ID, "m1", "m2", "m3", "m4", "old")
	// A против B три раза: 3:1, 0:0, 2:4; в m4 A и B вместе против C
	saveDuels(t, s, ids[0], []uint64{dA}, []uint64{dB}, map[[2]uint64]int{{dA, dB}: 3, {dB, dA}: 1}, DuelsSinceVersion)
	saveDuels(t, s, ids[1], []uint64{dA}, []uint64{dB}, nil, DuelsSinceVersion)
	saveDuels(t, s, ids[2], []uint64{dB}, []uint64{dA}, map[[2]uint64]int{{dA, dB}: 2, {dB, dA}: 4}, DuelsSinceVersion)
	saveDuels(t, s, ids[3], []uint64{dA, dB}, []uint64{dC}, map[[2]uint64]int{{dA, dC}: 1}, DuelsSinceVersion)
	// матч старой версии: результат есть, дуэли не учитываются
	saveDuels(t, s, ids[4], []uint64{dA}, []uint64{dB}, map[[2]uint64]int{{dA, dB}: 50}, DuelsSinceVersion-1)
	// матч другой сессии вне периода
	other := addMatches(t, s, s2.ID, "late")
	saveDuels(t, s, other[0], []uint64{dA}, []uint64{dB}, map[[2]uint64]int{{dA, dB}: 7}, DuelsSinceVersion)
	addMatches(t, s, s3.ID, "pending") // без результата

	sd, err := s.SessionDuels(ctx, s1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sd.Eligible != 5 || len(sd.Uncovered) != 1 || sd.Uncovered[0].Ordinal != 5 {
		t.Fatalf("покрытие сессии: eligible=%d uncovered=%+v", sd.Eligible, sd.Uncovered)
	}
	if c, ok := cellOf(sd.Cells, dA, dB); !ok || c.Kills != 5 || c.Deaths != 5 || c.Maps != 3 {
		t.Fatalf("A→B в сессии: %+v ok=%v", c, ok)
	}
	if _, ok := cellOf(sd.Cells, dA, dA); ok {
		t.Fatal("ячейка игрока с самим собой")
	}
	if c, ok := cellOf(sd.Cells, dC, dA); !ok || c.Kills != 0 || c.Deaths != 1 || c.Maps != 1 {
		t.Fatalf("C→A: %+v ok=%v", c, ok)
	}

	pd, err := s.PlayerDuels(ctx, dA, MatchFilter{From: "2026-10-01", To: "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	if pd.Eligible != 5 || pd.Covered != 4 || pd.CoveredSessions != 1 {
		t.Fatalf("покрытие профиля: %+v", pd)
	}
	got := map[uint64]Opponent{}
	for _, o := range pd.Opponents {
		got[o.SteamID] = o
	}
	if o := got[dB]; o.Kills != 5 || o.Deaths != 5 || o.Maps != 3 || o.Name == "" {
		t.Fatalf("A против B за период: %+v", o)
	}
	if o := got[dC]; o.Kills != 1 || o.Deaths != 0 || o.Maps != 1 {
		t.Fatalf("A против C: %+v", o)
	}

	// без периода учитывается и матч второй сессии; границы дат включительные
	all, _ := s.PlayerDuels(ctx, dA, MatchFilter{})
	for _, o := range all.Opponents {
		if o.SteamID == dB && (o.Kills != 12 || o.Maps != 4) {
			t.Fatalf("A против B за всё время: %+v", o)
		}
	}
	if all.CoveredSessions != 2 {
		t.Fatalf("сессий с дуэлями %d, ожидалось 2", all.CoveredSessions)
	}
	bySession, _ := s.PlayerDuels(ctx, dA, MatchFilter{SessionIDs: []int64{s2.ID}})
	if bySession.Eligible != 1 || len(bySession.Opponents) != 1 || bySession.Opponents[0].Kills != 7 {
		t.Fatalf("по сессии: %+v", bySession)
	}

	covered, cells, _ := s.MatchDuels(ctx, ids[4])
	if covered || len(cells) != 0 {
		t.Fatalf("матч старой версии покрыт: %v %+v", covered, cells)
	}
	covered, cells, _ = s.MatchDuels(ctx, ids[0])
	if c, ok := cellOf(cells, dB, dA); !covered || !ok || c.Kills != 1 || c.Deaths != 3 {
		t.Fatalf("матч: covered=%v %+v", covered, cells)
	}
}
