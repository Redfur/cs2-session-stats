package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cs2stats/internal/stats"
)

type MatchStatus string

const (
	StatusPending MatchStatus = "pending"
	StatusParsing MatchStatus = "parsing"
	StatusDone    MatchStatus = "done"
	StatusFailed  MatchStatus = "failed"
)

type Match struct {
	ID           int64       `json:"id"`
	SessionID    int64       `json:"sessionId"`
	Ordinal      int         `json:"ordinal"`
	SHA256       string      `json:"sha256"`
	OriginalName string      `json:"originalName"`
	Status       MatchStatus `json:"status"`
	Error        string      `json:"error,omitempty"`
	Map          string      `json:"map"`
	Rounds       int         `json:"rounds"`
	ScoreA       int         `json:"scoreA"`
	ScoreB       int         `json:"scoreB"`
	CreatedAt    string      `json:"createdAt"`
	ParsedAt     *string     `json:"parsedAt,omitempty"`
	// HasResult — у матча есть посчитанный результат; он сохраняется, пока матч
	// пересчитывается, и при неудачном пересчёте.
	HasResult bool `json:"hasResult"`
	// ProcessedVersion — версия обработки последней завершённой попытки (0 — не обрабатывался).
	ProcessedVersion int `json:"processedVersion"`
}

// MatchResult — результат обработки демки.
type MatchResult struct {
	Map     string
	Rounds  int
	ScoreA  int
	ScoreB  int
	Players []stats.PlayerStats
}

// DuplicateError — демка с таким sha256 уже есть в системе.
type DuplicateError struct{ Existing Match }

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("демка уже загружена: матч %d в сессии %d", e.Existing.ID, e.Existing.SessionID)
}

const matchColumns = `id, session_id, ordinal, sha256, original_name, status, error, map, rounds,
	score_a, score_b, created_at, parsed_at, has_result, processed_version`

type scanner interface{ Scan(dest ...any) error }

func scanMatch(row scanner) (Match, error) {
	var m Match
	var parsedAt sql.NullString
	err := row.Scan(&m.ID, &m.SessionID, &m.Ordinal, &m.SHA256, &m.OriginalName, &m.Status, &m.Error,
		&m.Map, &m.Rounds, &m.ScoreA, &m.ScoreB, &m.CreatedAt, &parsedAt, &m.HasResult, &m.ProcessedVersion)
	if parsedAt.Valid {
		m.ParsedAt = &parsedAt.String
	}
	return m, err
}

// AddMatch добавляет матч в конец сессии в статусе pending.
// Если демка с таким sha256 уже есть, возвращает *DuplicateError.
func (s *Store) AddMatch(ctx context.Context, sessionID int64, sha256, originalName string) (Match, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Match{}, err
	}
	defer tx.Rollback()

	existing, err := scanMatch(tx.QueryRowContext(ctx, "SELECT "+matchColumns+" FROM matches WHERE sha256 = ?", sha256))
	switch {
	case err == nil:
		return Match{}, &DuplicateError{Existing: existing}
	case !errors.Is(err, sql.ErrNoRows):
		return Match{}, err
	}

	m, err := scanMatch(tx.QueryRowContext(ctx, `
		INSERT INTO matches (session_id, ordinal, sha256, original_name, status, created_at)
		VALUES (?, (SELECT coalesce(max(ordinal), 0) + 1 FROM matches WHERE session_id = ?), ?, ?, ?, ?)
		RETURNING `+matchColumns,
		sessionID, sessionID, sha256, originalName, StatusPending, s.timestamp()))
	if err != nil {
		return Match{}, err
	}
	return m, tx.Commit()
}

func (s *Store) FindMatchBySHA(ctx context.Context, sha256 string) (Match, error) {
	m, err := scanMatch(s.db.QueryRowContext(ctx, "SELECT "+matchColumns+" FROM matches WHERE sha256 = ?", sha256))
	if errors.Is(err, sql.ErrNoRows) {
		return Match{}, ErrNotFound
	}
	return m, err
}

func (s *Store) GetMatch(ctx context.Context, id int64) (Match, error) {
	m, err := scanMatch(s.db.QueryRowContext(ctx, "SELECT "+matchColumns+" FROM matches WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Match{}, ErrNotFound
	}
	return m, err
}

// ListSessionMatches возвращает матчи сессии по порядку.
func (s *Store) ListSessionMatches(ctx context.Context, sessionID int64) ([]Match, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+matchColumns+" FROM matches WHERE session_id = ? ORDER BY ordinal", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Match{}
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// ClaimNextPending переводит следующий pending-матч в parsing и возвращает его.
// Матчи без результата (новые загрузки) идут раньше пересчётов, чтобы массовый
// пересчёт не задерживал свежие демки. ok=false, если очередь пуста.
func (s *Store) ClaimNextPending(ctx context.Context) (m Match, ok bool, err error) {
	m, err = scanMatch(s.db.QueryRowContext(ctx, `
		UPDATE matches SET status = ?
		WHERE id = (SELECT id FROM matches WHERE status = ? ORDER BY has_result, id LIMIT 1)
		RETURNING `+matchColumns, StatusParsing, StatusPending))
	if errors.Is(err, sql.ErrNoRows) {
		return Match{}, false, nil
	}
	return m, err == nil, err
}

// ResetParsing возвращает в очередь матчи, обработка которых прервалась перезапуском.
func (s *Store) ResetParsing(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE matches SET status = ? WHERE status = ?", StatusPending, StatusParsing)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SaveMatchResult атомарно заменяет результат матча, полученный версией обработки version, и переводит матч в done.
func (s *Store) SaveMatchResult(ctx context.Context, matchID int64, r MatchResult, version int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM match_players WHERE match_id = ?", matchID); err != nil {
		return err
	}
	for _, p := range r.Players {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO match_players (match_id, steam_id, name, team, result, rounds, kills, deaths, assists,
				hs_kills, damage, kast_rounds, k1, k2, k3, k4, k5, opening_kills, opening_deaths)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			matchID, int64(p.SteamID), p.Name, p.Team, p.Result, p.Rounds, p.Kills, p.Deaths, p.Assists,
			p.HSKills, p.Damage, p.KASTRounds, p.K1, p.K2, p.K3, p.K4, p.K5, p.OpeningKills, p.OpeningDeaths)
		if err != nil {
			return fmt.Errorf("игрок %d: %w", p.SteamID, err)
		}
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE matches SET status = ?, error = '', map = ?, rounds = ?, score_a = ?, score_b = ?, parsed_at = ?,
			has_result = 1, processed_version = ?
		WHERE id = ?`, StatusDone, r.Map, r.Rounds, r.ScoreA, r.ScoreB, s.timestamp(), version, matchID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// FailMatch переводит матч в failed с текстом ошибки. Прежний результат матча
// (игроки, счёт, has_result) не трогается.
func (s *Store) FailMatch(ctx context.Context, matchID int64, msg string, version int) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE matches SET status = ?, error = ?, parsed_at = ?, processed_version = ? WHERE id = ?",
		StatusFailed, msg, s.timestamp(), version, matchID)
	return err
}

const playerColumns = `steam_id, name, team, result, rounds, kills, deaths, assists, hs_kills, damage,
	kast_rounds, k1, k2, k3, k4, k5, opening_kills, opening_deaths`

func scanPlayer(row scanner) (stats.PlayerStats, error) {
	var p stats.PlayerStats
	var steamID int64
	err := row.Scan(&steamID, &p.Name, &p.Team, &p.Result, &p.Rounds, &p.Kills, &p.Deaths, &p.Assists,
		&p.HSKills, &p.Damage, &p.KASTRounds, &p.K1, &p.K2, &p.K3, &p.K4, &p.K5, &p.OpeningKills, &p.OpeningDeaths)
	p.SteamID = uint64(steamID)
	return p, err
}

// MatchPlayers возвращает статистику игроков матча.
func (s *Store) MatchPlayers(ctx context.Context, matchID int64) ([]stats.PlayerStats, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+playerColumns+" FROM match_players WHERE match_id = ?", matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []stats.PlayerStats{}
	for rows.Next() {
		p, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// DeleteMatch удаляет матч вместе со статистикой и перенумеровывает оставшиеся матчи сессии.
// Возвращает удалённый матч: файл его демки удаляет вызывающий код после коммита.
func (s *Store) DeleteMatch(ctx context.Context, id int64) (Match, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Match{}, err
	}
	defer tx.Rollback()

	m, err := scanMatch(tx.QueryRowContext(ctx, "DELETE FROM matches WHERE id = ? RETURNING "+matchColumns, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Match{}, ErrNotFound
	}
	if err != nil {
		return Match{}, err
	}
	ids, err := sessionMatchIDs(ctx, tx, m.SessionID)
	if err != nil {
		return Match{}, err
	}
	if err := renumber(ctx, tx, m.SessionID, ids); err != nil {
		return Match{}, err
	}
	return m, tx.Commit()
}

// ReorderMatches задаёт новый порядок матчей сессии. ids — все матчи сессии в новом порядке,
// иначе ErrOrderMismatch. Неизвестная сессия — ErrNotFound.
func (s *Store) ReorderMatches(ctx context.Context, sessionID int64, ids []int64) ([]Match, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM sessions WHERE id = ?", sessionID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	current, err := sessionMatchIDs(ctx, tx, sessionID)
	if err != nil {
		return nil, err
	}
	if len(ids) != len(current) {
		return nil, ErrOrderMismatch
	}
	known := make(map[int64]bool, len(current))
	for _, id := range current {
		known[id] = true
	}
	for _, id := range ids {
		if !known[id] {
			return nil, ErrOrderMismatch // чужой матч или повтор
		}
		delete(known, id)
	}
	if err := renumber(ctx, tx, sessionID, ids); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.ListSessionMatches(ctx, sessionID)
}

func sessionMatchIDs(ctx context.Context, tx *sql.Tx, sessionID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM matches WHERE session_id = ? ORDER BY ordinal", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// renumber нумерует матчи сессии 1..N в порядке ids. Сначала номера уходят в отрицательные,
// иначе промежуточное состояние нарушает UNIQUE (session_id, ordinal).
func renumber(ctx context.Context, tx *sql.Tx, sessionID int64, ids []int64) error {
	if _, err := tx.ExecContext(ctx, "UPDATE matches SET ordinal = -ordinal WHERE session_id = ?", sessionID); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, "UPDATE matches SET ordinal = ? WHERE id = ?", i+1, id); err != nil {
			return err
		}
	}
	return nil
}
