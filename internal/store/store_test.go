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
		if version != 5 {
			t.Fatalf("user_version = %d, ожидалось 5", version)
		}
		for _, table := range []string{"sessions", "matches", "match_players", "match_duels", "imports",
			"match_player_ext", "match_player_weapons", "match_rounds", "match_kills", "match_clutches"} {
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

// БД версии 2 схемы мигрирует: результат старых матчей помечается версией 1, дуэлей у них нет.
func TestMigration003Backfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_init.sql", "002_reprocessing.sql"} {
		body, _ := migrationsFS.ReadFile("migrations/" + name)
		if _, err := raw.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	raw.Exec("PRAGMA user_version = 2")
	raw.Exec("INSERT INTO sessions (id, date, created_at) VALUES (1, '2026-10-08', 'x')")
	raw.Exec("INSERT INTO matches (session_id, ordinal, sha256, original_name, status, created_at, has_result, processed_version) VALUES (1, 1, 'done', 'f', 'done', 'x', 1, 1)")
	raw.Exec("INSERT INTO matches (session_id, ordinal, sha256, original_name, status, created_at, has_result, processed_version) VALUES (1, 2, 'failed', 'f', 'failed', 'x', 0, 1)")
	raw.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for sha, want := range map[string]int{"done": 1, "failed": 0} {
		var v int
		if err := s.db.QueryRow("SELECT result_version FROM matches WHERE sha256 = ?", sha).Scan(&v); err != nil {
			t.Fatal(err)
		}
		if v != want {
			t.Errorf("%s: result_version=%d, ожидалось %d", sha, v, want)
		}
	}
}

// БД версии 3 схемы мигрирует: у старых матчей нет загрузки и времени игры, они читаются как раньше.
func TestMigration004Backfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_init.sql", "002_reprocessing.sql", "003_duels.sql"} {
		body, _ := migrationsFS.ReadFile("migrations/" + name)
		if _, err := raw.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	raw.Exec("PRAGMA user_version = 3")
	raw.Exec("INSERT INTO sessions (id, date, created_at) VALUES (1, '2026-10-08', 'x')")
	raw.Exec("INSERT INTO matches (session_id, ordinal, sha256, original_name, status, created_at, has_result, processed_version, result_version) VALUES (1, 1, 'old', 'f.dem', 'done', 'x', 1, 2, 2)")
	raw.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var importID, playedAt, platform, source, mapNumber sql.NullString
	if err := s.db.QueryRow("SELECT import_id, played_at, source_platform, source_number, map_number FROM matches WHERE sha256 = 'old'").
		Scan(&importID, &playedAt, &platform, &source, &mapNumber); err != nil {
		t.Fatal(err)
	}
	if importID.Valid || playedAt.Valid || platform.Valid || source.Valid || mapNumber.Valid {
		t.Errorf("новые колонки старого матча не NULL: %v %v %v %v %v", importID, playedAt, platform, source, mapNumber)
	}
	m, err := s.FindMatchBySHA(t.Context(), "old")
	if err != nil || m.Ordinal != 1 || !m.HasResult {
		t.Fatalf("старый матч читается неверно: %+v, %v", m, err)
	}
}

// БД версии 4 схемы мигрирует: старые матчи читаются как раньше, расширенных данных у них нет.
func TestMigration005Backfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001_init.sql", "002_reprocessing.sql", "003_duels.sql", "004_imports.sql"} {
		body, _ := migrationsFS.ReadFile("migrations/" + name)
		if _, err := raw.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	raw.Exec("PRAGMA user_version = 4")
	raw.Exec("INSERT INTO sessions (id, date, created_at) VALUES (1, '2026-10-08', 'x')")
	raw.Exec("INSERT INTO matches (id, session_id, ordinal, sha256, original_name, status, created_at, has_result, processed_version, result_version) VALUES (1, 1, 1, 'old', 'f.dem', 'done', 'x', 1, 2, 2)")
	raw.Exec("INSERT INTO match_players (match_id, steam_id, name, team, result, rounds, kills, deaths, assists, hs_kills, damage, kast_rounds, k1, k2, k3, k4, k5, opening_kills, opening_deaths) VALUES (1, 7, 'p', 'A', 'win', 13, 10, 5, 3, 4, 900, 10, 0, 0, 0, 0, 0, 0, 0)")
	raw.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	m, err := s.FindMatchBySHA(t.Context(), "old")
	if err != nil || !m.HasResult {
		t.Fatalf("старый матч читается неверно: %+v, %v", m, err)
	}
	e, err := s.GetMatchExt(t.Context(), m.ID)
	if err != nil || e.Covered {
		t.Fatalf("у старого матча расширенные данные посчитаны: %+v, %v", e, err)
	}
	rows, err := s.PlayerExtRows(t.Context(), 7, MatchFilter{})
	if err != nil || len(rows) != 1 || rows[0].Covered || rows[0].Kills != 10 || rows[0].Assists != 3 {
		t.Fatalf("строка старого матча: %+v, %v", rows, err)
	}
}
