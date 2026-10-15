package store

import (
	"context"
	"database/sql"
	"errors"

	"cs2stats/internal/ids"
)

// ErrGroupNotEmpty — удалить можно только группу без сессий.
var ErrGroupNotEmpty = errors.New("в группе есть сессии — сначала удалите их")

// Group — компания друзей: свои сессии, статистика и рейтинг.
type Group struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CreatedAt    string `json:"createdAt"`
	SessionCount int    `json:"sessionCount"`
}

// GroupSummary — группа со сводкой для заголовка страницы группы.
type GroupSummary struct {
	Group
	MatchCount  int `json:"matchCount"`  // матчи с результатом
	PlayerCount int `json:"playerCount"` // разные игроки в матчах с результатом
}

const groupColumns = `g.id, g.name, g.created_at, (SELECT count(*) FROM sessions s WHERE s.group_id = g.id)`

func scanGroup(row scanner) (Group, error) {
	var g Group
	err := row.Scan(&g.ID, &g.Name, &g.CreatedAt, &g.SessionCount)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	return g, err
}

// ListGroups возвращает группы в порядке создания с числом сессий.
func (s *Store) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+groupColumns+" FROM groups g ORDER BY g.seq")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Group{}
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

// CreateGroup создаёт группу. Название проверяет вызывающий код.
func (s *Store) CreateGroup(ctx context.Context, name string) (Group, error) {
	g := Group{Name: name, CreatedAt: s.timestamp()}
	err := insertWithNewID(ids.Group, "groups.id", func(id string) error {
		g.ID = id
		_, err := s.db.ExecContext(ctx, "INSERT INTO groups (id, seq, name, created_at) VALUES (?, "+nextSeq("groups")+", ?, ?)",
			g.ID, g.Name, g.CreatedAt)
		return err
	})
	if err != nil {
		return Group{}, err
	}
	return g, nil
}

func (s *Store) GetGroup(ctx context.Context, id string) (Group, error) {
	return scanGroup(s.db.QueryRowContext(ctx, "SELECT "+groupColumns+" FROM groups g WHERE g.id = ?", id))
}

// GroupSummary возвращает группу с числом матчей с результатом и разных игроков в них.
func (s *Store) GroupSummary(ctx context.Context, id string) (GroupSummary, error) {
	g, err := s.GetGroup(ctx, id)
	if err != nil {
		return GroupSummary{}, err
	}
	x := GroupSummary{Group: g}
	err = s.db.QueryRowContext(ctx, `
		SELECT (SELECT count(*) FROM matches m JOIN sessions s ON s.id = m.session_id
		        WHERE s.group_id = ? AND m.has_result = 1),
		       (SELECT count(DISTINCT mp.steam_id) `+fromPlayerMatches+`
		        WHERE s.group_id = ? AND m.has_result = 1)`, id, id).
		Scan(&x.MatchCount, &x.PlayerCount)
	return x, err
}

// RenameGroup меняет название группы. Название проверяет вызывающий код.
func (s *Store) RenameGroup(ctx context.Context, id string, name string) (Group, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE groups SET name = ? WHERE id = ?", name, id)
	if err != nil {
		return Group{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Group{}, ErrNotFound
	}
	return s.GetGroup(ctx, id)
}

// DeleteGroup удаляет группу без сессий; у группы с сессиями — ErrGroupNotEmpty.
func (s *Store) DeleteGroup(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	g, err := scanGroup(tx.QueryRowContext(ctx, "SELECT "+groupColumns+" FROM groups g WHERE g.id = ?", id))
	if err != nil {
		return err
	}
	if g.SessionCount > 0 {
		return ErrGroupNotEmpty
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM groups WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}
