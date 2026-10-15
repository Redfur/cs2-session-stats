package store

import (
	"context"
	"database/sql"
	"fmt"

	"cs2stats/internal/ids"
)

// idTables — таблицы со строковыми id и тип id каждой.
var idTables = []struct {
	table string
	kind  ids.Kind
}{
	{"groups", ids.Group},
	{"sessions", ids.Session},
	{"matches", ids.Match},
	{"imports", ids.Import},
}

// stringIDsBefore — Go-шаг миграции 008: таблицы соответствия id_map_<таблица> (old → new)
// с новыми строковыми id для всех существующих строк. SQL миграции пересобирает по ним таблицы.
func stringIDsBefore(ctx context.Context, tx *sql.Tx) error {
	for _, t := range idTables {
		mapTable := "id_map_" + t.table
		if _, err := tx.ExecContext(ctx,
			"CREATE TABLE "+mapTable+" (old INTEGER PRIMARY KEY, new TEXT NOT NULL UNIQUE)"); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, "SELECT id FROM "+t.table)
		if err != nil {
			return err
		}
		var old []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			old = append(old, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range old {
			if err := insertWithNewID(t.kind, mapTable+".new", func(newID string) error {
				_, err := tx.ExecContext(ctx, "INSERT INTO "+mapTable+" (old, new) VALUES (?, ?)", id, newID)
				return err
			}); err != nil {
				return fmt.Errorf("id для %s %d: %w", t.table, id, err)
			}
		}
	}
	return nil
}

// foreignKeyCheck — проверка после пересборки таблиц: ни одной нарушенной ссылки.
func foreignKeyCheck(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var table, parent string
		var rowid sql.NullInt64
		var fkid int
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return err
		}
		return fmt.Errorf("нарушена ссылка %s → %s (строка %v)", table, parent, rowid.Int64)
	}
	return rows.Err()
}
