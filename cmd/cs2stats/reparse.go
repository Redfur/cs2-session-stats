package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"cs2stats/internal/config"
	"cs2stats/internal/ids"
	"cs2stats/internal/store"
)

// reparseTarget — что ставить на пересчёт: ровно одно из полей.
type reparseTarget struct {
	match   string
	session string
	all     bool
}

func parseReparseArgs(args []string) (reparseTarget, error) {
	var t reparseTarget
	fs := flag.NewFlagSet("reparse", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&t.match, "match", "", "")
	fs.StringVar(&t.session, "session", "", "")
	fs.BoolVar(&t.all, "all", false, "")
	if err := fs.Parse(args); err != nil {
		return t, err
	}
	if fs.NArg() > 0 {
		return t, fmt.Errorf("лишние аргументы: %v", fs.Args())
	}
	set := 0
	fs.Visit(func(*flag.Flag) { set++ })
	if set != 1 {
		return t, errors.New("укажите ровно один из параметров --match, --session или --all")
	}
	switch {
	case t.all:
	case isSet(fs, "session"):
		if !ids.Valid(ids.Session, t.session) {
			return t, fmt.Errorf("неверный id сессии %q: ожидается вида s_Vf8nR2pQxa7c", t.session)
		}
	case !ids.Valid(ids.Match, t.match):
		return t, fmt.Errorf("неверный id матча %q: ожидается вида m_Vf8nR2pQxa7c", t.match)
	}
	return t, nil
}

func isSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

func reparseCmd(t reparseTarget) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	// не создаём пустую базу, если DATA_DIR указан неверно
	if _, err := os.Stat(cfg.DBPath()); err != nil {
		return fmt.Errorf("база не найдена: %s (проверьте DATA_DIR)", cfg.DBPath())
	}
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()

	n, err := requeue(context.Background(), st, t)
	if err != nil {
		return err
	}
	fmt.Printf("поставлено в очередь: %d\n", n)
	return nil
}

func requeue(ctx context.Context, st *store.Store, t reparseTarget) (int64, error) {
	switch {
	case t.all:
		return st.RequeueAll(ctx)
	case t.session != "":
		if _, err := st.GetSession(ctx, t.session); err != nil {
			return 0, fmt.Errorf("сессия %s: %w", t.session, err)
		}
		return st.RequeueSession(ctx, t.session)
	default:
		before, err := st.GetMatch(ctx, t.match)
		if err != nil {
			return 0, fmt.Errorf("матч %s: %w", t.match, err)
		}
		if before.Status == store.StatusPending || before.Status == store.StatusParsing {
			return 0, nil // уже в очереди
		}
		_, err = st.RequeueMatch(ctx, t.match)
		return 1, err
	}
}
