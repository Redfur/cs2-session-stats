package store

import (
	"context"
	"database/sql"
	"errors"
)

type Session struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"` // YYYY-MM-DD
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

type SessionSummary struct {
	Session
	MatchCount int `json:"matchCount"`
}

// CreateSession создаёт сессию. Дату валидирует вызывающий код.
func (s *Store) CreateSession(ctx context.Context, date, title string) (Session, error) {
	sess := Session{Date: date, Title: title, CreatedAt: s.timestamp()}
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (date, title, created_at) VALUES (?, ?, ?)",
		sess.Date, sess.Title, sess.CreatedAt)
	if err != nil {
		return Session{}, err
	}
	sess.ID, err = res.LastInsertId()
	return sess, err
}

// ListSessions возвращает сессии от новых к старым.
func (s *Store) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.date, s.title, s.created_at, count(m.id)
		FROM sessions s LEFT JOIN matches m ON m.session_id = s.id
		GROUP BY s.id
		ORDER BY s.date DESC, s.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []SessionSummary{}
	for rows.Next() {
		var x SessionSummary
		if err := rows.Scan(&x.ID, &x.Date, &x.Title, &x.CreatedAt, &x.MatchCount); err != nil {
			return nil, err
		}
		list = append(list, x)
	}
	return list, rows.Err()
}

func (s *Store) GetSession(ctx context.Context, id int64) (Session, error) {
	var x Session
	err := s.db.QueryRowContext(ctx,
		"SELECT id, date, title, created_at FROM sessions WHERE id = ?", id).
		Scan(&x.ID, &x.Date, &x.Title, &x.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return x, err
}

// UpdateSession меняет дату и название сессии. Дату валидирует вызывающий код.
func (s *Store) UpdateSession(ctx context.Context, id int64, date, title string) (Session, error) {
	var x Session
	err := s.db.QueryRowContext(ctx,
		"UPDATE sessions SET date = ?, title = ? WHERE id = ? RETURNING id, date, title, created_at",
		date, title, id).
		Scan(&x.ID, &x.Date, &x.Title, &x.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return x, err
}

// DeleteSession удаляет сессию вместе с матчами и их статистикой и возвращает sha256
// демок удалённых матчей: файлы удаляет вызывающий код после коммита.
func (s *Store) DeleteSession(ctx context.Context, id int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, "SELECT sha256 FROM matches WHERE session_id = ?", id)
	if err != nil {
		return nil, err
	}
	shas := []string{}
	for rows.Next() {
		var sha string
		if err := rows.Scan(&sha); err != nil {
			rows.Close()
			return nil, err
		}
		shas = append(shas, sha)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// match_players удаляются каскадом
	if _, err := tx.ExecContext(ctx, "DELETE FROM matches WHERE session_id = ?", id); err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return shas, tx.Commit()
}
