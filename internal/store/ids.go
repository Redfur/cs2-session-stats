package store

import (
	"strings"

	"cs2stats/internal/ids"
)

// nextSeq — выражение для seq новой строки: следующий номер вставки в таблице. Записи в SQLite
// идут по одной, поэтому два INSERT не получат один номер. seq задаёт только порядок и наружу не отдаётся.
func nextSeq(table string) string {
	return "(SELECT coalesce(max(seq), 0) + 1 FROM " + table + ")"
}

// insertAttempts — сколько раз повторить вставку при совпадении нового id с существующим.
const insertAttempts = 3

// insertWithNewID вызывает insert с новым id типа kind; если id совпал с существующим
// (нарушена уникальность колонки column, например "matches.id"), повторяет с другим id.
func insertWithNewID(kind ids.Kind, column string, insert func(id string) error) error {
	var err error
	for range insertAttempts {
		if err = insert(ids.New(kind)); err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed: "+column) {
			return err
		}
	}
	return err
}
