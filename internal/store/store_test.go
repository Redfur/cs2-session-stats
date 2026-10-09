package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMigrationsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for i := 0; i < 2; i++ {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("открытие #%d: %v", i+1, err)
		}
		var version int
		if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version != 2 {
			t.Fatalf("user_version = %d, ожидалось 2", version)
		}
		for _, table := range []string{"sessions", "matches", "match_players"} {
			var n int
			if err := s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&n); err != nil || n != 1 {
				t.Fatalf("таблица %s не создана (err=%v)", table, err)
			}
		}
		s.Close()
	}
}

// БД, созданная версией 1 схемы, мигрирует с корректным заполнением новых колонок.
func TestMigration002Backfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	v1, _ := migrationsFS.ReadFile("migrations/001_init.sql")
	if _, err := raw.Exec(string(v1)); err != nil {
		t.Fatal(err)
	}
	raw.Exec("PRAGMA user_version = 1")
	raw.Exec("INSERT INTO sessions (id, date, created_at) VALUES (1, '2026-10-08', 'x')")
	for i, st := range []string{"done", "failed", "pending"} {
		raw.Exec("INSERT INTO matches (session_id, ordinal, sha256, original_name, status, created_at) VALUES (1, ?, ?, 'f', ?, 'x')", i+1, st, st)
	}
	raw.Close()

	for run := 0; run < 2; run++ {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("запуск %d: %v", run+1, err)
		}
		want := map[string][2]int{"done": {1, 1}, "failed": {0, 1}, "pending": {0, 0}}
		for sha, w := range want {
			var hasResult, version int
			if err := s.db.QueryRow("SELECT has_result, processed_version FROM matches WHERE sha256 = ?", sha).Scan(&hasResult, &version); err != nil {
				t.Fatal(err)
			}
			if hasResult != w[0] || version != w[1] {
				t.Errorf("%s: has_result=%d processed_version=%d, ожидалось %v", sha, hasResult, version, w)
			}
		}
		s.Close()
	}
}
