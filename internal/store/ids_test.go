package store

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"cs2stats/internal/ids"
)

// n объектов подряд: при сортировке по случайному id порядок вставки совпал бы с вероятностью 1/n!.
const orderN = 12

// Очередь разбора идёт в порядке загрузки, а не по id.
func TestClaimNextPendingInsertOrder(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, mainGroup(t, s), "2026-10-08", "")
	var want []string
	for i := range orderN {
		m, err := s.AddMatch(ctx, sess.ID, fmt.Sprint("sha", i), "f.dem")
		if err != nil || !ids.Valid(ids.Match, m.ID) {
			t.Fatalf("матч %d: %+v, %v", i, m, err)
		}
		want = append(want, m.ID)
	}
	var got []string
	for {
		m, ok, err := s.ClaimNextPending(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		got = append(got, m.ID)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("порядок разбора %v, ожидался порядок загрузки %v", got, want)
	}
}

// Очередь скачивания и список загрузок сессии идут в порядке добавления.
func TestImportQueueInsertOrder(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, mainGroup(t, s), "2026-10-08", "")
	var want []string
	for i := range orderN {
		x := addImport(t, s, sess.ID, fmt.Sprint(100+i))
		if !ids.Valid(ids.Import, x.ID) {
			t.Fatalf("id загрузки %q", x.ID)
		}
		want = append(want, x.ID)
	}
	list, err := s.ListSessionImports(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, x := range list {
		listed = append(listed, x.ID)
	}
	if !slices.Equal(listed, want) {
		t.Fatalf("список загрузок %v, ожидалось %v", listed, want)
	}
	var got []string
	for {
		x, ok, err := s.ClaimNextImport(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		got = append(got, x.ID)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("порядок скачивания %v, ожидалось %v", got, want)
	}
}

// Сессии одной даты идут в порядке создания: в списке — от новых к старым, в хронологии рейтинга —
// от старых к новым. Группы — в порядке создания.
func TestSameDateSessionsCreationOrder(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	group := mainGroup(t, s)
	var sessions, matches []string
	for i := range orderN {
		sess, err := s.CreateSession(ctx, group, "2026-10-08", fmt.Sprint(i))
		if err != nil || !ids.Valid(ids.Session, sess.ID) {
			t.Fatalf("сессия %d: %+v, %v", i, sess, err)
		}
		sessions = append(sessions, sess.ID)
	}
	// матчи добавляются в обратном порядке сессий: порядок рейтинга от него не зависит
	for i := orderN - 1; i >= 0; i-- {
		m := addMatches(t, s, sessions[i], fmt.Sprint("sha", i))[0]
		saveResult(t, s, m)
		matches = append([]string{m}, matches...)
	}

	list, err := s.ListSessions(ctx, group)
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, x := range list {
		listed = append(listed, x.ID)
	}
	newestFirst := slices.Clone(sessions)
	slices.Reverse(newestFirst)
	if !slices.Equal(listed, newestFirst) {
		t.Fatalf("список сессий %v, ожидалось %v", listed, newestFirst)
	}
	if got := matchOrder(t, s); !slices.Equal(got, matches) {
		t.Fatalf("хронология рейтинга %v, ожидалось %v", got, matches)
	}

	wantGroups := []string{group}
	for i := range orderN {
		g, err := s.CreateGroup(ctx, fmt.Sprint("г", i))
		if err != nil || !ids.Valid(ids.Group, g.ID) {
			t.Fatalf("группа %d: %+v, %v", i, g, err)
		}
		wantGroups = append(wantGroups, g.ID)
	}
	groups, err := s.ListGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var gotGroups []string
	for _, g := range groups {
		gotGroups = append(gotGroups, g.ID)
	}
	if !slices.Equal(gotGroups, wantGroups) {
		t.Fatalf("порядок групп %v, ожидалось %v", gotGroups, wantGroups)
	}
}

// Совпадение нового id с существующим повторяет вставку с другим id; другие ошибки не повторяются.
func TestInsertWithNewIDRetries(t *testing.T) {
	calls := 0
	err := insertWithNewID(ids.Match, "matches.id", func(id string) error {
		calls++
		if calls < 3 {
			return fmt.Errorf("constraint failed: UNIQUE constraint failed: matches.id (1555)")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("повторы: calls=%d, %v", calls, err)
	}
	calls = 0
	err = insertWithNewID(ids.Match, "matches.id", func(id string) error {
		calls++
		return fmt.Errorf("UNIQUE constraint failed: matches.sha256")
	})
	if err == nil || calls != 1 {
		t.Fatalf("чужое нарушение уникальности повторено: calls=%d, %v", calls, err)
	}
}
