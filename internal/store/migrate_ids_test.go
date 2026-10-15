package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"cs2stats/internal/ids"
)

// rawAtVersion создаёт базу, к которой применены миграции 001..version без Go-шагов, и
// возвращает её путь и открытое соединение с включёнными внешними ключами.
func rawAtVersion(t *testing.T, version int) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	raw, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { raw.Close() })
	entries, _ := migrationsFS.ReadDir("migrations")
	for _, e := range entries[:version] {
		body, _ := migrationsFS.ReadFile("migrations/" + e.Name())
		if _, err := raw.Exec(string(body)); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
	}
	if _, err := raw.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		t.Fatal(err)
	}
	return path, raw
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Go-шаги вызываются по одному разу в транзакции своей миграции; ошибка шага откатывает её целиком.
func TestMigrateSteps(t *testing.T) {
	// не через Open: он сразу применил бы все миграции со штатными шагами
	path, raw := rawAtVersion(t, 6)
	raw.Close()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := &Store{db: db}

	boom := errors.New("шаг упал")
	var before, after int
	failing := map[int]migrationStep{7: {
		Before: func(ctx context.Context, tx *sql.Tx) error { before++; return nil },
		After:  func(ctx context.Context, tx *sql.Tx) error { after++; return boom },
	}}
	if err := st.migrateWith(t.Context(), failing); !errors.Is(err, boom) {
		t.Fatalf("ошибка шага не вернулась: %v", err)
	}
	if before != 1 || after != 1 {
		t.Fatalf("шаги вызваны before=%d after=%d", before, after)
	}
	if v := userVersion(t, db); v != 6 {
		t.Fatalf("user_version после ошибки = %d, ожидалось 6", v)
	}
	var n int
	db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = 'groups'").Scan(&n)
	if n != 0 {
		t.Fatal("SQL миграции 007 не откатился")
	}

	before, after = 0, 0
	var sawGroups bool
	ok := map[int]migrationStep{8: migrationSteps[8], 7: {
		Before: func(ctx context.Context, tx *sql.Tx) error { before++; return nil },
		After: func(ctx context.Context, tx *sql.Tx) error {
			after++
			// After видит результат SQL в той же транзакции
			var c int
			err := tx.QueryRowContext(ctx, "SELECT count(*) FROM groups").Scan(&c)
			sawGroups = err == nil && c == 1
			return err
		},
	}}
	if err := st.migrateWith(t.Context(), ok); err != nil {
		t.Fatal(err)
	}
	if before != 1 || after != 1 || !sawGroups {
		t.Fatalf("шаги вызваны before=%d after=%d, группа видна: %v", before, after, sawGroups)
	}
	if err := st.migrateWith(t.Context(), ok); err != nil || before != 1 || after != 1 {
		t.Fatalf("повторный запуск вызвал шаги: before=%d after=%d, %v", before, after, err)
	}
}

// Миграция 008 на базе версии 7: все строки, связи и порядок сохраняются, ключи — строковые id.
func TestMigration008StringIDs(t *testing.T) {
	path, raw := rawAtVersion(t, 7)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := raw.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec("INSERT INTO groups (id, name, created_at) VALUES (2, 'Вторая', 'x')")
	// сессии 1 и 2 одной даты в группе 1, сессия 3 — в группе 2
	exec(`INSERT INTO sessions (id, group_id, date, title, created_at) VALUES
		(1, 1, '2026-10-08', 'а', 'x'), (2, 1, '2026-10-08', 'б', 'x'), (3, 2, '2026-10-01', 'в', 'x')`)
	exec(`INSERT INTO imports (id, session_id, url, platform, external_id, status, created_at) VALUES
		(1, 1, 'u1', 'fastcup', '100', 'done', 'x'), (2, 2, 'u2', 'fastcup', '101', 'queued', 'x'),
		(3, 2, 'u3', 'cybershoke', '7', 'queued', 'x')`)
	exec(`INSERT INTO matches (id, session_id, ordinal, sha256, original_name, status, created_at, has_result,
			processed_version, result_version, import_id, played_at, source_platform, source_number, map_number, duration_ms) VALUES
		(1, 1, 1, 'a', 'a.dem', 'done', 'x', 1, 5, 5, 1, '2026-10-08T18:00:00Z', 'fastcup', 100, 1, 1000),
		(2, 1, 2, 'b', 'b.dem', 'pending', 'x', 1, 4, 4, NULL, NULL, NULL, NULL, NULL, 0),
		(3, 2, 1, 'c', 'c.dem', 'failed', 'x', 0, 5, 0, NULL, NULL, NULL, NULL, NULL, 0),
		(4, 3, 1, 'd', 'd.dem', 'done', 'x', 1, 5, 5, NULL, NULL, NULL, NULL, NULL, 0)`)
	for _, m := range []int{1, 2, 4} {
		for _, p := range []int{7, 8} {
			exec(`INSERT INTO match_players (match_id, steam_id, name, team, result, rounds, kills, deaths, assists, hs_kills,
				damage, kast_rounds, k1, k2, k3, k4, k5, opening_kills, opening_deaths)
				VALUES (?, ?, 'p', 'A', 'win', 13, ?, 5, 3, 4, 900, 10, 0, 0, 0, 0, 0, 0, 0)`, m, p, m*10+p)
		}
		exec("INSERT INTO match_duels (match_id, killer_id, victim_id, kills) VALUES (?, 7, 8, 2), (?, 8, 7, 1)", m, m)
		exec(`INSERT INTO match_player_ext (match_id, steam_id, rounds, traded_deaths, trade_kills, damage_dealt, damage_taken,
			flash_assists, damage_assists, rounds_t, rounds_ct, survived_t, survived_ct, alive_ms_t, alive_ms_ct, he_damage,
			fire_damage, flashed, smokes, he_kills, fire_kills, impact_kills, smoke_kills, wallbang_kills, noscope_kills,
			blind_kills, distance_sum, distance_kills, plants, plant_starts, defuses, defuse_starts)
			VALUES (?, 7, 13, 1, 1, 900, 800, 0, 0, 6, 7, 2, 3, 1000, 2000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 12.5, 1, 0, 0, 0, 0)`, m)
		exec(`INSERT INTO match_player_weapons (match_id, steam_id, weapon, kills, hs_kills, damage, shots, hits, hs_hits)
			VALUES (?, 7, 'ak47', 5, 2, 500, 40, 10, 3)`, m)
		exec(`INSERT INTO match_rounds (match_id, number, winner, side_a, reason, start_ms, end_ms, restored, equip_a, equip_b, plant_ms)
			VALUES (?, 1, 'A', 'CT', 'elimination', 100, 200, 0, 4000, 3500, 0), (?, 2, 'B', 'CT', 'bomb', 300, 400, 0, 1000, 2000, 350)`, m, m)
		exec(`INSERT INTO match_kills (match_id, round, seq, time_ms, killer_id, victim_id, assister_id, weapon, headshot,
			flash_assist, through_smoke, wallbang, noscope, attacker_blind, distance, trade)
			VALUES (?, 1, 1, 150, 7, 8, 0, 'ak47', 1, 0, 0, 0, 0, 0, 10.5, 0)`, m)
		exec("INSERT INTO match_clutches (match_id, round, steam_id, vs, outcome) VALUES (?, 1, 7, 1, 'win')", m)
	}
	tables := []string{"groups", "sessions", "imports", "matches", "match_players", "match_duels", "match_player_ext",
		"match_player_weapons", "match_rounds", "match_kills", "match_clutches"}
	countRows := func(db *sql.DB) map[string]int {
		out := map[string]int{}
		for _, table := range tables {
			var n int
			if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
				t.Fatal(err)
			}
			out[table] = n
		}
		return out
	}
	before := countRows(raw)
	raw.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v := userVersion(t, s.db); v != 8 {
		t.Fatalf("user_version = %d", v)
	}
	if after := countRows(s.db); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("число строк изменилось: до %v, после %v", before, after)
	}
	rows, err := s.db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		t.Fatal("foreign_key_check не пуст")
	}
	rows.Close()

	// все ключи и ссылки — строковые id своего типа
	for _, c := range []struct {
		query string
		kind  ids.Kind
	}{
		{"SELECT id FROM groups", ids.Group},
		{"SELECT id FROM sessions", ids.Session},
		{"SELECT group_id FROM sessions", ids.Group},
		{"SELECT id FROM imports", ids.Import},
		{"SELECT session_id FROM imports", ids.Session},
		{"SELECT id FROM matches", ids.Match},
		{"SELECT session_id FROM matches", ids.Session},
		{"SELECT import_id FROM matches WHERE import_id IS NOT NULL", ids.Import},
		{"SELECT match_id FROM match_players", ids.Match},
		{"SELECT match_id FROM match_duels", ids.Match},
		{"SELECT match_id FROM match_player_ext", ids.Match},
		{"SELECT match_id FROM match_player_weapons", ids.Match},
		{"SELECT match_id FROM match_rounds", ids.Match},
		{"SELECT match_id FROM match_kills", ids.Match},
		{"SELECT match_id FROM match_clutches", ids.Match},
	} {
		rows, err := s.db.Query(c.query)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var v string
			rows.Scan(&v)
			if !ids.Valid(c.kind, v) {
				t.Errorf("%s: %q не id типа %s", c.query, v, c.kind)
			}
		}
		rows.Close()
	}

	// связи и seq: восстанавливаем старые номера по seq и сверяем с исходными связями
	for _, c := range []struct{ query, want string }{
		{"SELECT group_concat(seq, ',') FROM (SELECT seq FROM groups ORDER BY seq)", "1,2"},
		{`SELECT group_concat(x, ',') FROM (SELECT s.seq || ':' || g.seq AS x FROM sessions s JOIN groups g ON g.id = s.group_id ORDER BY s.seq)`,
			"1:1,2:1,3:2"},
		{`SELECT group_concat(x, ',') FROM (SELECT i.seq || ':' || s.seq || ':' || i.status AS x FROM imports i JOIN sessions s ON s.id = i.session_id ORDER BY i.seq)`,
			"1:1:done,2:2:queued,3:2:queued"},
		{`SELECT group_concat(x, ',') FROM (SELECT m.seq || ':' || s.seq || ':' || m.ordinal || ':' || coalesce(i.seq, '-') || ':' || m.status || ':' || m.result_version AS x
			FROM matches m JOIN sessions s ON s.id = m.session_id LEFT JOIN imports i ON i.id = m.import_id ORDER BY m.seq)`,
			"1:1:1:1:done:5,2:1:2:-:pending:4,3:2:1:-:failed:0,4:3:1:-:done:5"},
		{`SELECT group_concat(x, ',') FROM (SELECT m.seq || ':' || p.steam_id || ':' || p.kills AS x
			FROM match_players p JOIN matches m ON m.id = p.match_id ORDER BY m.seq, p.steam_id)`,
			"1:7:17,1:8:18,2:7:27,2:8:28,4:7:47,4:8:48"},
		{`SELECT group_concat(x, ',') FROM (SELECT m.seq || ':' || d.killer_id || '>' || d.victim_id || '=' || d.kills AS x
			FROM match_duels d JOIN matches m ON m.id = d.match_id ORDER BY m.seq, d.killer_id)`,
			"1:7>8=2,1:8>7=1,2:7>8=2,2:8>7=1,4:7>8=2,4:8>7=1"},
		{`SELECT group_concat(x, ',') FROM (SELECT m.seq || ':' || r.number || ':' || r.equip_a || ':' || r.plant_ms AS x
			FROM match_rounds r JOIN matches m ON m.id = r.match_id ORDER BY m.seq, r.number)`,
			"1:1:4000:0,1:2:1000:350,2:1:4000:0,2:2:1000:350,4:1:4000:0,4:2:1000:350"},
		{`SELECT group_concat(x, ',') FROM (SELECT m.seq || ':' || x.distance_sum || ':' || w.weapon || ':' || k.distance || ':' || c.outcome AS x
			FROM matches m JOIN match_player_ext x ON x.match_id = m.id JOIN match_player_weapons w ON w.match_id = m.id
			JOIN match_kills k ON k.match_id = m.id JOIN match_clutches c ON c.match_id = m.id ORDER BY m.seq)`,
			"1:12.5:ak47:10.5:win,2:12.5:ak47:10.5:win,4:12.5:ak47:10.5:win"},
	} {
		var got string
		if err := s.db.QueryRow(c.query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", c.query, err)
		}
		if got != c.want {
			t.Errorf("%s\nполучено %s\nожидалось %s", c.query, got, c.want)
		}
	}

	// в схеме нет целых колонок-ключей объектов и таблиц миграции
	keyCols := map[string][]string{
		"groups": {"id"}, "sessions": {"id", "group_id"}, "imports": {"id", "session_id"},
		"matches": {"id", "session_id", "import_id"}, "match_players": {"match_id"}, "match_duels": {"match_id"},
		"match_player_ext": {"match_id"}, "match_player_weapons": {"match_id"}, "match_rounds": {"match_id"},
		"match_kills": {"match_id"}, "match_clutches": {"match_id"},
	}
	for table, cols := range keyCols {
		for _, col := range cols {
			var typ string
			var notNull int
			if err := s.db.QueryRow("SELECT type, \"notnull\" FROM pragma_table_info(?) WHERE name = ?", table, col).Scan(&typ, &notNull); err != nil {
				t.Fatalf("%s.%s: %v", table, col, err)
			}
			if typ != "TEXT" {
				t.Errorf("%s.%s: тип %s", table, col, typ)
			}
			if table == "sessions" && col == "group_id" && notNull != 1 {
				t.Error("sessions.group_id допускает NULL")
			}
		}
	}
	var leftovers []string
	rows, err = s.db.Query("SELECT name FROM sqlite_master WHERE name LIKE 'id\\_map\\_%' ESCAPE '\\' OR sql LIKE '%\\_old%' ESCAPE '\\'")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n string
		rows.Scan(&n)
		leftovers = append(leftovers, n)
	}
	rows.Close()
	if len(leftovers) > 0 {
		t.Errorf("остались служебные таблицы: %s", strings.Join(leftovers, ", "))
	}
	var indexes int
	s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'index' AND sql IS NOT NULL").Scan(&indexes)
	if indexes != 9 {
		t.Errorf("индексов %d, ожидалось 9", indexes)
	}
}
