package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cs2stats/internal/parser"
	"cs2stats/internal/store"
)

type fakeDemos struct{}

func (fakeDemos) Open(sha string) (io.ReadCloser, error) {
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
