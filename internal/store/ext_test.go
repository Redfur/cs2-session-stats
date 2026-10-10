package store

import (
	"context"
	"testing"
	"time"

	"cs2stats/internal/parser"
	"cs2stats/internal/stats"
)

func extResult(trades int) MatchResult {
	r := duelResult(5)
	r.Ext = &stats.Ext{
		FirstRound: 1, HasDamageEvents: true, HasFlashEvents: true, Duration: 40 * time.Minute,
		Players: []stats.PlayerExt{
			{SteamID: 1, ExtCounters: stats.ExtCounters{Rounds: 13, TradeKills: trades, HEDamage: 50}},
			{SteamID: 2, ExtCounters: stats.ExtCounters{Rounds: 13}},
		},
		Weapons:  []stats.WeaponStats{{SteamID: 1, Weapon: "ak47", Kills: 5, HSKills: 2, Damage: 500, Shots: 60, Hits: 20, HSHits: 4}},
		Rounds:   []stats.RoundInfo{{Number: 1, Winner: parser.TeamA, SideA: parser.SideCT, Reason: parser.ReasonBomb, Start: time.Second, End: time.Minute}},
		Kills:    []stats.KillEvent{{Round: 1, Seq: 1, Time: 30 * time.Second, Killer: 1, Victim: 2, Weapon: "ak47", Headshot: true, Trade: true}},
		Clutches: []stats.Clutch{{Round: 1, SteamID: 1, Vs: 2, Outcome: stats.ClutchWin}},
	}
	return r
}

func TestSaveMatchResultExt(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	id := addMatches(t, s, sess.ID, "x")[0]
	tables := []string{"match_player_ext", "match_player_weapons", "match_rounds", "match_kills", "match_clutches"}
	counts := func() []int {
		var out []int
		for _, table := range tables {
			out = append(out, countRows(t, s, "SELECT count(*) FROM "+table+" WHERE match_id = ?", id))
		}
		return out
	}

	// два сохранения той же версией — один набор строк
	for i := 0; i < 2; i++ {
		if err := s.SaveMatchResult(ctx, id, extResult(3), ExtendedSinceVersion); err != nil {
			t.Fatal(err)
		}
	}
	want := []int{2, 1, 1, 1, 1}
	for i, n := range counts() {
		if n != want[i] {
			t.Errorf("%s: %d строк, ожидалось %d", tables[i], n, want[i])
		}
	}
	e, err := s.GetMatchExt(ctx, id)
	if err != nil || !e.Covered || len(e.Rounds) != 1 || len(e.Kills) != 1 || !e.Kills[0].Trade || e.Duration != 40*time.Minute {
		t.Fatalf("расширенные данные матча: %+v, %v", e, err)
	}
	if e.Rounds[0].Reason != parser.ReasonBomb || e.Rounds[0].End != time.Minute || e.Clutches[0].Outcome != stats.ClutchWin {
		t.Errorf("раунд и клатч: %+v %+v", e.Rounds[0], e.Clutches[0])
	}

	// ошибка пересчёта сохраняет прежние данные
	if err := s.FailMatch(ctx, id, "битая демка", ExtendedSinceVersion); err != nil {
		t.Fatal(err)
	}
	rows, err := s.PlayerExtRows(ctx, 1, MatchFilter{})
	if err != nil || len(rows) != 1 || !rows[0].Covered || rows[0].Ext.TradeKills != 3 || rows[0].Ext.HEDamage != 50 {
		t.Fatalf("после ошибки: %+v, %v", rows, err)
	}

	// запись старым бинарём (версия 2) делает расширенные данные непосчитанными
	if err := s.SaveMatchResult(ctx, id, duelResult(5), 2); err != nil {
		t.Fatal(err)
	}
	if e, _ := s.GetMatchExt(ctx, id); e.Covered {
		t.Error("после записи версией 2 расширенные данные считаются посчитанными")
	}
	if rows, _ := s.PlayerExtRows(ctx, 1, MatchFilter{}); rows[0].Covered {
		t.Error("строка игрока после записи версией 2 считается покрытой")
	}
	if c, _ := s.PlayerClutches(ctx, 1, MatchFilter{}); len(c) != 0 {
		t.Errorf("клатчи после записи версией 2: %+v", c)
	}
}

func TestExtAggregates(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	ids := addMatches(t, s, sess.ID, "a", "b", "c")
	if err := s.SaveMatchResult(ctx, ids[0], extResult(3), ExtendedSinceVersion); err != nil {
		t.Fatal(err)
	}
	noDamage := extResult(1)
	noDamage.Ext.HasDamageEvents = false
	if err := s.SaveMatchResult(ctx, ids[1], noDamage, ExtendedSinceVersion); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMatchResult(ctx, ids[2], duelResult(5), 2); err != nil { // старая версия
		t.Fatal(err)
	}

	rows, err := s.SessionExtRows(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	covered := map[int64]bool{}
	for _, r := range rows {
		if r.SteamID == 1 {
			covered[r.MatchID] = r.Covered
		}
	}
	if !covered[ids[0]] || !covered[ids[1]] || covered[ids[2]] {
		t.Errorf("покрытие матчей: %v", covered)
	}
	w, err := s.PlayerWeapons(ctx, 1, MatchFilter{})
	if err != nil || len(w) != 1 {
		t.Fatalf("оружие: %+v, %v", w, err)
	}
	// убийства — по обоим посчитанным матчам, урон и попадания — только где есть события урона
	if w[0].Kills != 10 || w[0].Damage != 500 || w[0].Hits != 20 || w[0].Shots != 120 {
		t.Errorf("AK-47 за период: %+v", w[0])
	}
	c, err := s.SessionClutches(ctx, sess.ID)
	if err != nil || len(c) != 1 || c[0].Count != 2 || c[0].Vs != 2 {
		t.Errorf("клатчи сессии: %+v, %v", c, err)
	}
}

func TestDeleteMatchRemovesExt(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	id := addMatches(t, s, sess.ID, "x")[0]
	if err := s.SaveMatchResult(ctx, id, extResult(1), ExtendedSinceVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteMatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"match_player_ext", "match_player_weapons", "match_rounds", "match_kills", "match_clutches"} {
		if n := countRows(t, s, "SELECT count(*) FROM "+table+" WHERE match_id = ?", id); n != 0 {
			t.Errorf("%s: осталось %d строк", table, n)
		}
	}
	// удалённый во время обработки матч не воскресает
	if err := s.SaveMatchResult(ctx, id, extResult(1), ExtendedSinceVersion); err != ErrNotFound {
		t.Errorf("сохранение удалённого матча: %v", err)
	}
	if n := countRows(t, s, "SELECT count(*) FROM match_kills WHERE match_id = ?", id); n != 0 {
		t.Errorf("журнал удалённого матча записан: %d", n)
	}
}
