// Package store хранит сессии, матчи и статистику игроков в SQLite.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound возвращается, когда запрошенная сущность не существует.
var ErrNotFound = errors.New("не найдено")

// ErrOrderMismatch — новый порядок не совпадает с набором матчей сессии.
var ErrOrderMismatch = errors.New("список матчей не совпадает с матчами сессии")

type Store struct {
	db  *sql.DB
	now func() time.Time
}

// Open открывает (или создаёт) БД и применяет миграции.
func Open(path string) (*Store, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	// immediate: транзакция сразу берёт блокировку на запись, иначе при
	// конкурентной записи API и воркера возможен SQLITE_BUSY без ожидания.
	q.Set("_txlock", "immediate")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, now: time.Now}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("миграции: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// migrationStep — Go-часть миграции: выполняется в той же транзакции до (Before) или после (After) SQL-файла.
// Нужна там, где SQL не справляется сам, например для генерации случайных id.
type migrationStep struct {
	Before func(ctx context.Context, tx *sql.Tx) error
	After  func(ctx context.Context, tx *sql.Tx) error
}

// migrationSteps — Go-шаги по номеру миграции.
var migrationSteps = map[int]migrationStep{
	8: {Before: stringIDsBefore, After: foreignKeyCheck},
}

// migrate применяет файлы migrations/NNN_*.sql с номером больше PRAGMA user_version.
func (s *Store) migrate(ctx context.Context) error {
	return s.migrateWith(ctx, migrationSteps)
}

// migrateWith применяет миграции с Go-шагами steps. Каждая миграция — одна транзакция:
// ошибка шага или SQL откатывает её целиком, и user_version не меняется.
func (s *Store) migrateWith(ctx context.Context, steps map[int]migrationStep) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var current int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	for _, e := range entries {
		num, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil {
			return fmt.Errorf("имя миграции %s: %w", e.Name(), err)
		}
		if num <= current {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		if err := s.applyMigration(ctx, num, string(body), steps[num]); err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	return nil
}

func (s *Store) applyMigration(ctx context.Context, num int, body string, step migrationStep) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if step.Before != nil {
		if err := step.Before(ctx, tx); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	if step.After != nil {
		if err := step.After(ctx, tx); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", num)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) timestamp() string { return s.now().UTC().Format(time.RFC3339) }
