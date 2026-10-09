// Package worker в фоне обрабатывает загруженные демки: парсит, считает статистику, сохраняет результат.
package worker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"cs2stats/internal/parser"
	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

// DemoOpener открывает сохранённую демку по sha256.
type DemoOpener interface {
	Open(sha256 string) (io.ReadCloser, error)
}

type ParseFunc func(io.Reader) (parser.Match, error)

type Worker struct {
	store *store.Store
	demos DemoOpener
	parse ParseFunc
	log   *slog.Logger
	wake  chan struct{}
	// pollInterval — страховочный опрос очереди, если сигнал Wake потерялся.
	pollInterval time.Duration
}

func New(st *store.Store, demos DemoOpener, parse ParseFunc, log *slog.Logger) *Worker {
	return &Worker{store: st, demos: demos, parse: parse, log: log, wake: make(chan struct{}, 1), pollInterval: 30 * time.Second}
}

// Wake сообщает воркеру, что в очереди появились матчи. Не блокирует.
func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run обрабатывает очередь, пока не отменён ctx.
func (w *Worker) Run(ctx context.Context) error {
	n, err := w.store.ResetParsing(ctx)
	if err != nil {
		return fmt.Errorf("возврат прерванных матчей в очередь: %w", err)
	}
	if n > 0 {
		w.log.Info("прерванные матчи возвращены в очередь", "count", n)
	}
	for {
		if err := w.drain(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.log.Error("ошибка очереди", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-w.wake:
		case <-time.After(w.pollInterval):
		}
	}
}

// drain обрабатывает все pending-матчи по одному.
func (w *Worker) drain(ctx context.Context) error {
	for ctx.Err() == nil {
		m, ok, err := w.store.ClaimNextPending(ctx)
		if err != nil || !ok {
			return err
		}
		start := time.Now()
		if err := w.process(ctx, m); err != nil {
			if ctx.Err() != nil {
				// сервис останавливается: матч остаётся в parsing и вернётся в очередь при следующем запуске
				w.log.Info("обработка прервана остановкой сервиса", "match", m.ID)
				return ctx.Err()
			}
			w.log.Warn("матч не обработан", "match", m.ID, "file", m.OriginalName, "err", err)
			if ferr := w.store.FailMatch(context.WithoutCancel(ctx), m.ID, err.Error()); ferr != nil {
				return ferr
			}
			continue
		}
		w.log.Info("матч обработан", "match", m.ID, "file", m.OriginalName, "took", time.Since(start).Round(time.Millisecond))
	}
	return ctx.Err()
}

func (w *Worker) process(ctx context.Context, m store.Match) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("внутренняя ошибка обработки: %v", rec)
		}
	}()
	f, err := w.demos.Open(m.SHA256)
	if err != nil {
		return fmt.Errorf("открытие демки: %w", err)
	}
	defer f.Close()

	match, err := w.parse(f)
	if err != nil {
		return err
	}
	return w.store.SaveMatchResult(ctx, m.ID, store.MatchResult{
		Map:     match.Map,
		Rounds:  len(match.Rounds),
		ScoreA:  match.ScoreA,
		ScoreB:  match.ScoreB,
		Players: stats.Compute(match),
	})
}
