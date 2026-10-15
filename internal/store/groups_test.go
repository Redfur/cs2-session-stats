package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestGroups(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	g, err := s.CreateGroup(ctx, "Четверг")
	if err != nil || g.ID == "" || g.Name != "Четверг" || g.CreatedAt == "" {
		t.Fatalf("создание: %+v, %v", g, err)
	}
	s.CreateSession(ctx, mainGroup(t, s), "2026-10-08", "")
	s.CreateSession(ctx, mainGroup(t, s), "2026-10-09", "")
	list, err := s.ListGroups(ctx)
	if err != nil || len(list) != 2 || list[0].ID != mainGroup(t, s) || list[0].SessionCount != 2 || list[1].ID != g.ID || list[1].SessionCount != 0 {
		t.Fatalf("список: %+v, %v", list, err)
	}

	renamed, err := s.RenameGroup(ctx, g.ID, "Пятница")
	if err != nil || renamed.Name != "Пятница" || renamed.ID != g.ID {
		t.Fatalf("переименование: %+v, %v", renamed, err)
	}
	if got, _ := s.GetGroup(ctx, g.ID); got.Name != "Пятница" {
		t.Errorf("после переименования: %+v", got)
	}

	if err := s.DeleteGroup(ctx, mainGroup(t, s)); !errors.Is(err, ErrGroupNotEmpty) {
		t.Errorf("удаление группы с сессиями: %v", err)
	}
	if err := s.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatalf("удаление пустой группы: %v", err)
	}
	if _, err := s.GetGroup(ctx, g.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("удалённая группа: %v", err)
	}

	const unknown = "none"
	if _, err := s.GetGroup(ctx, unknown); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetGroup неизвестной: %v", err)
	}
	if _, err := s.GroupSummary(ctx, unknown); !errors.Is(err, ErrNotFound) {
		t.Errorf("GroupSummary неизвестной: %v", err)
	}
	if _, err := s.RenameGroup(ctx, unknown, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("RenameGroup неизвестной: %v", err)
	}
	if err := s.DeleteGroup(ctx, unknown); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteGroup неизвестной: %v", err)
	}
}

func TestGroupSummary(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, mainGroup(t, s), "2026-10-08", "")
	ids := addMatches(t, s, sess.ID, "a", "b", "pending")
	// игроки 1 и 2 в обоих матчах — по одному разу
	for _, id := range ids[:2] {
		if err := s.SaveMatchResult(ctx, id, duelResult(3), 1); err != nil {
			t.Fatal(err)
		}
	}
	other, _ := s.CreateGroup(ctx, "Четверг")
	osess, _ := s.CreateSession(ctx, other.ID, "2026-10-08", "")
	if err := s.SaveMatchResult(ctx, addMatches(t, s, osess.ID, "c")[0], duelResult(1), 1); err != nil {
		t.Fatal(err)
	}

	x, err := s.GroupSummary(ctx, mainGroup(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if x.SessionCount != 1 || x.MatchCount != 2 || x.PlayerCount != 2 || x.Name != "Основная" {
		t.Errorf("сводка: %+v", x)
	}
	empty, _ := s.CreateGroup(ctx, "Пустая")
	if x, _ := s.GroupSummary(ctx, empty.ID); x.SessionCount != 0 || x.MatchCount != 0 || x.PlayerCount != 0 {
		t.Errorf("сводка пустой группы: %+v", x)
	}
}

func TestSessionsByGroup(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	other, _ := s.CreateGroup(ctx, "Четверг")
	main, _ := s.CreateSession(ctx, mainGroup(t, s), "2026-10-08", "")
	thu, err := s.CreateSession(ctx, other.ID, "2026-10-09", "четверг")
	if err != nil || thu.GroupID != other.ID {
		t.Fatalf("сессия группы: %+v, %v", thu, err)
	}
	for _, sess := range []Session{main, thu} {
		if err := s.SaveMatchResult(ctx, addMatches(t, s, sess.ID, sess.Date)[0], duelResult(1), 1); err != nil {
			t.Fatal(err)
		}
	}
	for gid, want := range map[string]Session{mainGroup(t, s): main, other.ID: thu} {
		list, err := s.ListSessions(ctx, gid)
		if err != nil || len(list) != 1 || list[0].ID != want.ID || list[0].GroupID != gid || list[0].MatchCount != 1 ||
			!reflect.DeepEqual(list[0].Maps, []string{"de_nuke"}) {
			t.Errorf("сессии группы %s: %+v, %v", gid, list, err)
		}
	}
	if got, _ := s.GetSession(ctx, thu.ID); got.GroupID != other.ID {
		t.Errorf("GetSession: %+v", got)
	}
	if got, _ := s.UpdateSession(ctx, thu.ID, "2026-10-10", "x"); got.GroupID != other.ID {
		t.Errorf("UpdateSession: %+v", got)
	}
}

// Игрок 1 сыграл матч в «Основной» и матч в группе «Четверг» под другим ником.
func TestPlayerDataByGroup(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	other, _ := s.CreateGroup(ctx, "Четверг")
	main, _ := s.CreateSession(ctx, mainGroup(t, s), "2026-10-01", "")
	thu, _ := s.CreateSession(ctx, other.ID, "2026-10-08", "")

	r := extResult(1)
	r.Players[0].Name = "alpha"
	r.Players[0].Rounds = 20
	m1 := addMatches(t, s, main.ID, "main")[0]
	if err := s.SaveMatchResult(ctx, m1, r, SwingSinceVersion); err != nil {
		t.Fatal(err)
	}
	r = extResult(1)
	r.Players[0].Name = "beta"
	r.Players[0].Rounds = 30
	m2 := addMatches(t, s, thu.ID, "thu")[0]
	if err := s.SaveMatchResult(ctx, m2, r, SwingSinceVersion); err != nil {
		t.Fatal(err)
	}

	// свои агрегаты и ник в каждой группе
	for gid, want := range map[string]struct {
		name   string
		rounds int
	}{mainGroup(t, s): {"alpha", 20}, other.ID: {"beta", 30}} {
		list, err := s.PlayerTotals(ctx, MatchFilter{GroupID: gid, PlayerIDs: []uint64{1}}, 1)
		if err != nil || len(list) != 1 || list[0].Matches != 1 || list[0].Rounds != want.rounds || list[0].Name != want.name {
			t.Errorf("группа %s: %+v, %v", gid, list, err)
		}
		if name, err := s.latestName(ctx, 1, gid); err != nil || name != want.name {
			t.Errorf("latestName группы %s: %q, %v", gid, name, err)
		}
	}

	// сессия чужой группы в фильтре не даёт строк
	if list, _ := s.PlayerTotals(ctx, MatchFilter{GroupID: other.ID, SessionIDs: []string{main.ID}}, 1); len(list) != 0 {
		t.Errorf("сессия чужой группы: %+v", list)
	}
	for name, get := range map[string]func(MatchFilter) (int, error){
		"PlayerMatches":  func(f MatchFilter) (int, error) { x, err := s.PlayerMatches(ctx, 1, f); return len(x), err },
		"PlayerSessions": func(f MatchFilter) (int, error) { x, err := s.PlayerSessions(ctx, 1, f); return len(x), err },
		"PlayerMaps":     func(f MatchFilter) (int, error) { x, err := s.PlayerMaps(ctx, 1, f); return len(x), err },
		"PlayerExtRows":  func(f MatchFilter) (int, error) { x, err := s.PlayerExtRows(ctx, 1, f); return len(x), err },
		"PlayerWeapons":  func(f MatchFilter) (int, error) { x, err := s.PlayerWeapons(ctx, 1, f); return len(x), err },
		"PlayerClutches": func(f MatchFilter) (int, error) { x, err := s.PlayerClutches(ctx, 1, f); return len(x), err },
		"PlayerDuels": func(f MatchFilter) (int, error) {
			x, err := s.PlayerDuels(ctx, 1, f)
			return x.Eligible, err
		},
	} {
		if n, err := get(MatchFilter{GroupID: other.ID}); err != nil || n != 1 {
			t.Errorf("%s группы: %d, %v", name, n, err)
		}
		if n, err := get(MatchFilter{GroupID: other.ID, SessionIDs: []string{main.ID}}); err != nil || n != 0 {
			t.Errorf("%s с сессией чужой группы: %d, %v", name, n, err)
		}
	}

	// OVR группы не видит матчи другой группы
	for gid, want := range map[string]string{mainGroup(t, s): m1, other.ID: m2} {
		rows, err := s.RatingRows(ctx, gid)
		if err != nil || len(rows) != 2 || rows[0].MatchID != want {
			t.Errorf("RatingRows группы %s: %+v, %v", gid, rows, err)
		}
		swing, err := s.RatingSwing(ctx, gid)
		if _, ok := swing[want]; err != nil || len(swing) != 1 || !ok {
			t.Errorf("RatingSwing группы %s: %v, %v", gid, swing, err)
		}
	}
}

// При одной группе выборка по группе совпадает с выборкой без группы.
func TestSingleGroupUnchanged(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	all, _ := f.s.PlayerTotals(ctx, MatchFilter{}, 1)
	byGroup, err := f.s.PlayerTotals(ctx, MatchFilter{GroupID: mainGroup(t, f.s)}, 1)
	if err != nil || !reflect.DeepEqual(all, byGroup) {
		t.Errorf("PlayerTotals: %+v против %+v (%v)", byGroup, all, err)
	}
	rowsAll, _ := f.s.RatingRows(ctx, "")
	rowsGroup, err := f.s.RatingRows(ctx, mainGroup(t, f.s))
	if err != nil || !reflect.DeepEqual(rowsAll, rowsGroup) {
		t.Errorf("RatingRows отличаются (%v)", err)
	}
}
