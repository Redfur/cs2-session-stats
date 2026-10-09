package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"cs2stats/internal/api"
	"cs2stats/internal/config"
	"cs2stats/internal/ingest"
	"cs2stats/internal/parser"
	"cs2stats/internal/stats"
	"cs2stats/internal/store"
	"cs2stats/internal/webui"
	"cs2stats/internal/worker"
)

const usage = `использование:
  cs2stats                                        запустить сервис (HTTP API + воркер)
  cs2stats parse FILE                             разобрать демку и вывести статистику в JSON
  cs2stats reparse --match ID | --session ID | --all
                                                  поставить матчи в очередь на пересчёт
                                                  (их обработает запущенный сервис)`

func main() {
	args := os.Args[1:]
	switch {
	case len(args) == 0:
		if err := serve(); err != nil {
			log.Fatal(err)
		}
	case args[0] == "parse" && len(args) == 2:
		if err := parseCmd(args[1]); err != nil {
			log.Fatal(err)
		}
	case args[0] == "reparse":
		target, err := parseReparseArgs(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n\n%s\n", err, usage)
			os.Exit(2)
		}
		if err := reparseCmd(target); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}

func serve() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tmpDir := filepath.Join(cfg.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()
	demos, err := ingest.NewLocalStorage(cfg.DemosDir())
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	w := worker.New(st, demos, parser.Parse, logger)
	workerDone := make(chan error, 1)
	go func() { workerDone <- w.Run(ctx) }()

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: (&api.Server{
			Store:  st,
			Ingest: &ingest.Service{Store: st, Demos: demos, TmpDir: tmpDir, MaxSize: cfg.MaxDemoSize},
			Wake:   w.Wake,
			Log:    logger,
		}).Handler(webui.Default()),
		// без ReadTimeout: загрузка демок по медленному каналу может идти долго
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	logger.Info("cs2stats запущен", "addr", cfg.Addr, "data", cfg.DataDir)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-workerDone
}

func parseCmd(path string) error {
	m, err := parser.ParseFile(path)
	if err != nil {
		return err
	}
	players := stats.Compute(m)
	rows := make([]api.PlayerView, 0, len(players))
	for _, p := range players {
		rows = append(rows, api.MatchPlayerView(p))
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Team != rows[j].Team {
			return rows[i].Team < rows[j].Team
		}
		return rows[i].Rating > rows[j].Rating
	})
	out := struct {
		Map     string           `json:"map"`
		Rounds  int              `json:"rounds"`
		ScoreA  int              `json:"scoreA"`
		ScoreB  int              `json:"scoreB"`
		Players []api.PlayerView `json:"players"`
	}{m.Map, len(m.Rounds), m.ScoreA, m.ScoreB, rows}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
