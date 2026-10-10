package worker

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cs2stats/internal/parser"
	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

type fakeDemos struct{}

func (fakeDemos) Open(sha string) (io.ReadCloser, error) {
	if sha == "missing" {
		return nil, fs.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(sha)), nil
}

// fakeParse ведёт себя по содержимому «демки», которое равно её sha.
func fakeParse(r io.Reader) (parser.Match, error) {
	b, _ := io.ReadAll(r)
	switch string(b) {
	case "broken":
		return parser.Match{}, errors.New("битая демка")
	case "panic":
		panic("что-то сломалось")
	}
	return parser.Match{
		Map:     "de_mirage",
		Players: []parser.Player{{SteamID: 1, Name: "a", Team: parser.TeamA}, {SteamID: 2, Name: "b", Team: parser.TeamB}},
		Rounds:  []parser.Round{{Winner: parser.TeamA, Kills: []parser.Kill{{Killer: 1, Victim: 2}}}},
		ScoreA:  1,
	}, nil
}

func waitStatus(t *testing.T, st *store.Store, id int64, want store.MatchStatus) store.Match {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m, err := st.GetMatch(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if m.Status == want {
			return m
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("матч %d не перешёл в %s", id, want)
	return store.Match{}
}

func TestWorker(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	sess, _ := st.CreateSession(ctx, "2026-10-08", "")

	// матч «застрял» в parsing из-за перезапуска
	stuck, _ := st.AddMatch(ctx, sess.ID, "ok-1", "1.dem")
	if _, ok, _ := st.ClaimNextPending(ctx); !ok {
		t.Fatal("claim")
	}

	w := New(st, fakeDemos{}, fakeParse, slog.New(slog.DiscardHandler))
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error)
	go func() { done <- w.Run(runCtx) }()

	got := waitStatus(t, st, stuck.ID, store.StatusDone)
	if got.Map != "de_mirage" || got.ScoreA != 1 || got.Rounds != 1 {
		t.Errorf("результат: %+v", got)
	}
	players, _ := st.MatchPlayers(ctx, stuck.ID)
	if len(players) != 2 || players[0].Kills != 1 {
		t.Errorf("игроки: %+v", players)
	}
	if ext, err := st.GetMatchExt(ctx, stuck.ID); err != nil || !ext.Covered || len(ext.Rounds) != 1 || len(ext.Kills) != 1 {
		t.Errorf("расширенные данные: %+v, %v", ext, err)
	}

	broken, _ := st.AddMatch(ctx, sess.ID, "broken", "2.dem")
	panicky, _ := st.AddMatch(ctx, sess.ID, "panic", "3.dem")
	w.Wake()
	if m := waitStatus(t, st, broken.ID, store.StatusFailed); m.Error != "битая демка" {
		t.Errorf("ошибка: %q", m.Error)
	}
	if m := waitStatus(t, st, panicky.ID, store.StatusFailed); !strings.Contains(m.Error, "что-то сломалось") {
		t.Errorf("ошибка паники: %q", m.Error)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// Остановка сервиса во время парсинга не должна помечать матч failed:
// он остаётся parsing и дообрабатывается после следующего запуска.
func TestShutdownDuringParse(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	sess, _ := st.CreateSession(ctx, "2026-10-08", "")
	m, _ := st.AddMatch(ctx, sess.ID, "slow", "slow.dem")

	started, release := make(chan struct{}), make(chan struct{})
	slowParse := func(r io.Reader) (parser.Match, error) {
		close(started)
		<-release // парсинг не отменяется контекстом, как и настоящий
		return fakeParse(strings.NewReader("ok"))
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error)
	go func() { done <- New(st, fakeDemos{}, slowParse, slog.New(slog.DiscardHandler)).Run(runCtx) }()

	<-started
	cancel()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetMatch(ctx, m.ID); got.Status != store.StatusParsing {
		t.Fatalf("после остановки статус %s, ожидался parsing", got.Status)
	}

	// следующий запуск дообрабатывает матч
	runCtx, cancel = context.WithCancel(ctx)
	defer cancel()
	go New(st, fakeDemos{}, fakeParse, slog.New(slog.DiscardHandler)).Run(runCtx)
	waitStatus(t, st, m.ID, store.StatusDone)
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func runWorker(t *testing.T, st *store.Store) *Worker {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	w := New(st, fakeDemos{}, fakeParse, slog.New(slog.DiscardHandler))
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return w
}

// При старте матчи, обработанные старой версией, пересчитываются; актуальные — нет.
func TestRequeueOutdatedOnStart(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	sess, _ := st.CreateSession(ctx, "2026-10-08", "")
	old, _ := st.AddMatch(ctx, sess.ID, "ok-old", "old.dem")
	cur, _ := st.AddMatch(ctx, sess.ID, "ok-cur", "cur.dem")
	for _, x := range []struct {
		id      int64
		version int
	}{{old.ID, ProcessingVersion - 1}, {cur.ID, ProcessingVersion}} {
		st.ClaimNextPending(ctx)
		st.SaveMatchResult(ctx, x.id, store.MatchResult{Map: "stale", Rounds: 1}, x.version)
	}

	runWorker(t, st)
	got := waitStatus(t, st, old.ID, store.StatusDone)
	deadline := time.Now().Add(5 * time.Second)
	for got.Map != "de_mirage" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		got, _ = st.GetMatch(ctx, old.ID)
	}
	if got.Map != "de_mirage" || got.ProcessedVersion != ProcessingVersion {
		t.Fatalf("устаревший матч не пересчитан: %+v", got)
	}
	if c, _ := st.GetMatch(ctx, cur.ID); c.Map != "stale" {
		t.Fatalf("актуальный матч пересчитан: %+v", c)
	}
	// после пересчёта у матча появились дуэли
	covered, cells, err := st.MatchDuels(ctx, old.ID)
	if err != nil || !covered || len(cells) != 2 {
		t.Fatalf("дуэли пересчитанного матча: covered=%v cells=%+v err=%v", covered, cells, err)
	}
}

// Пересчёт без исходной демки: ошибка, прежний результат сохранён.
func TestMissingDemoKeepsResult(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	sess, _ := st.CreateSession(ctx, "2026-10-08", "")
	m, _ := st.AddMatch(ctx, sess.ID, "missing", "m.dem")
	st.ClaimNextPending(ctx)
	players := []stats.PlayerStats{{SteamID: 1, Name: "a", Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 5, Kills: 4}}}
	st.SaveMatchResult(ctx, m.ID, store.MatchResult{Map: "de_nuke", Rounds: 5, ScoreA: 5, Players: players}, ProcessingVersion)

	w := runWorker(t, st)
	st.RequeueMatch(ctx, m.ID)
	w.Wake()
	got := waitStatus(t, st, m.ID, store.StatusFailed)
	if got.Error != "исходная демка не найдена" || !got.HasResult || got.Map != "de_nuke" {
		t.Fatalf("матч: %+v", got)
	}
	if ps, _ := st.MatchPlayers(ctx, m.ID); len(ps) != 1 || ps[0].Kills != 4 {
		t.Fatalf("прежние игроки потеряны: %+v", ps)
	}
}

// Матч удалён, пока воркер его парсит: результат не сохраняется, матч не появляется снова,
// очередь обрабатывается дальше.
func TestMatchDeletedDuringProcessing(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	sess, _ := st.CreateSession(ctx, "2026-10-08", "")
	deleted, _ := st.AddMatch(ctx, sess.ID, "block", "1.dem")
	next, _ := st.AddMatch(ctx, sess.ID, "ok-2", "2.dem")

	started, release := make(chan struct{}), make(chan struct{})
	parse := func(r io.Reader) (parser.Match, error) {
		b, _ := io.ReadAll(r)
		if string(b) == "block" {
			close(started)
			<-release
		}
		return fakeParse(strings.NewReader("ok"))
	}
	w := New(st, fakeDemos{}, parse, slog.New(slog.DiscardHandler))
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error)
	go func() { done <- w.Run(runCtx) }()
	defer func() { cancel(); <-done }()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("воркер не начал обработку")
	}
	if _, err := st.DeleteMatch(ctx, deleted.ID); err != nil {
		t.Fatal(err)
	}
	close(release)

	waitStatus(t, st, next.ID, store.StatusDone)
	if _, err := st.GetMatch(ctx, deleted.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("удалённый матч появился снова: %v", err)
	}
	if ps, err := st.MatchPlayers(ctx, deleted.ID); err != nil || len(ps) != 0 {
		t.Fatalf("статистика удалённого матча сохранена: %d игроков, %v", len(ps), err)
	}
	list, _ := st.ListSessionMatches(ctx, sess.ID)
	if len(list) != 1 || list[0].ID != next.ID || list[0].Ordinal != 1 {
		t.Fatalf("матчи сессии: %+v", list)
	}
}

// Текущая версия обработки сохраняет дуэли: иначе новые матчи считались бы непокрытыми.
func TestProcessingVersionHasDuels(t *testing.T) {
	if ProcessingVersion < store.DuelsSinceVersion {
		t.Fatalf("ProcessingVersion %d ниже store.DuelsSinceVersion %d", ProcessingVersion, store.DuelsSinceVersion)
	}
	if ProcessingVersion < store.ExtendedSinceVersion {
		t.Fatalf("ProcessingVersion %d ниже store.ExtendedSinceVersion %d", ProcessingVersion, store.ExtendedSinceVersion)
	}
}
