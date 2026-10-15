package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"cs2stats/internal/ids"
)

type Session struct {
	ID        string `json:"id"`
	GroupID   string `json:"groupId"`
	Date      string `json:"date"` // YYYY-MM-DD
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

type SessionSummary struct {
	Session
	MatchCount  int      `json:"matchCount"`
	FailedCount int      `json:"failedCount"` // матчи в статусе failed
	Maps        []string `json:"maps"`        // карты матчей с результатом по порядку, без повторов
}

// CreateSession создаёт сессию в группе. Дату и существование группы проверяет вызывающий код.
func (s *Store) CreateSession(ctx context.Context, groupID string, date, title string) (Session, error) {
	sess := Session{GroupID: groupID, Date: date, Title: title, CreatedAt: s.timestamp()}
	err := insertWithNewID(ids.Session, "sessions.id", func(id string) error {
		sess.ID = id
		_, err := s.db.ExecContext(ctx,
			"INSERT INTO sessions (id, seq, group_id, date, title, created_at) VALUES (?, "+nextSeq("sessions")+", ?, ?, ?, ?)",
			sess.ID, sess.GroupID, sess.Date, sess.Title, sess.CreatedAt)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

// ListSessions возвращает сессии группы от новых к старым со сводкой по матчам.
func (s *Store) ListSessions(ctx context.Context, groupID string) ([]SessionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+sessionColumns+`, count(m.id), coalesce(sum(m.status = ?), 0)
		FROM sessions s LEFT JOIN matches m ON m.session_id = s.id
		WHERE s.group_id = ?
		GROUP BY s.id
		ORDER BY s.date DESC, s.seq DESC`, StatusFailed, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []SessionSummary{}
	byID := map[string]*SessionSummary{}
	for rows.Next() {
		x := SessionSummary{Maps: []string{}}
		if err := rows.Scan(append(sessionTargets(&x.Session), &x.MatchCount, &x.FailedCount)...); err != nil {
			return nil, err
		}
		list = append(list, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range list {
		byID[list[i].ID] = &list[i]
	}
	return list, s.fillSessionMaps(ctx, groupID, byID)
}

// fillSessionMaps добавляет сессиям карты матчей с результатом в порядке номеров, без повторов.
func (s *Store) fillSessionMaps(ctx context.Context, groupID string, byID map[string]*SessionSummary) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.session_id, m.map FROM matches m JOIN sessions s ON s.id = m.session_id
		WHERE m.has_result = 1 AND s.group_id = ? ORDER BY s.seq, m.ordinal`, groupID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, m string
		if err := rows.Scan(&id, &m); err != nil {
			return err
		}
		x := byID[id]
		if x == nil || m == "" || slices.Contains(x.Maps, m) {
			continue
		}
		x.Maps = append(x.Maps, m)
	}
	return rows.Err()
}

// sessionColumns — колонки Session по псевдониму s; читаются через sessionTargets.
const sessionColumns = "s.id, s.group_id, s.date, s.title, s.created_at"

func sessionTargets(x *Session) []any {
	return []any{&x.ID, &x.GroupID, &x.Date, &x.Title, &x.CreatedAt}
}

func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	var x Session
	err := s.db.QueryRowContext(ctx,
		"SELECT "+sessionColumns+" FROM sessions s WHERE s.id = ?", id).
		Scan(sessionTargets(&x)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return x, err
}

// UpdateSession меняет дату и название сессии. Дату валидирует вызывающий код.
func (s *Store) UpdateSession(ctx context.Context, id string, date, title string) (Session, error) {
	var x Session
	err := s.db.QueryRowContext(ctx,
		"UPDATE sessions SET date = ?, title = ? WHERE id = ? RETURNING id, group_id, date, title, created_at",
		date, title, id).
		Scan(sessionTargets(&x)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return x, err
}

// DeleteSession удаляет сессию вместе с матчами и их статистикой и возвращает sha256
// демок удалённых матчей: файлы удаляет вызывающий код после коммита.
func (s *Store) DeleteSession(ctx context.Context, id string) ([]string, error) {
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
