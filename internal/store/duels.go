package store

import (
	"context"
	"fmt"
)

// DuelsSinceVersion — версия обработки, с которой вместе с результатом сохраняются личные дуэли.
// Дуэли матча посчитаны, если has_result = 1 и result_version не ниже этой версии.
const DuelsSinceVersion = 2

var duelsCovered = fmt.Sprintf("m.result_version >= %d", DuelsSinceVersion)

// DuelCell — личный счёт пары за выборку: Kills — убийства Killer против Victim, Deaths —
// обратные, Maps — число матчей, где пара была соперниками.
type DuelCell struct {
	Killer uint64
	Victim uint64
	Kills  int
	Deaths int
	Maps   int
}

// duelCells суммирует дуэли матчей, выбранных условием where по псевдониму m (matches).
func (s *Store) duelCells(ctx context.Context, from, where string, args ...any) ([]DuelCell, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.killer_id, d.victim_id, sum(d.kills), count(*)
		FROM match_duels d JOIN matches m ON m.id = d.match_id `+from+`
		WHERE `+where+`
		GROUP BY d.killer_id, d.victim_id
		ORDER BY d.killer_id, d.victim_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type pair struct{ k, v uint64 }
	kills := map[pair]int{}
	cells := []DuelCell{}
	for rows.Next() {
		var c DuelCell
		var killer, victim int64
		if err := rows.Scan(&killer, &victim, &c.Kills, &c.Maps); err != nil {
			return nil, err
		}
		c.Killer, c.Victim = uint64(killer), uint64(victim)
		kills[pair{c.Killer, c.Victim}] = c.Kills
		cells = append(cells, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range cells {
		cells[i].Deaths = kills[pair{cells[i].Victim, cells[i].Killer}]
	}
	return cells, nil
}

// MatchDuels возвращает дуэли матча. covered=false, если у матча нет результата или он
// получен версией обработки без дуэлей.
func (s *Store) MatchDuels(ctx context.Context, matchID int64) (covered bool, cells []DuelCell, err error) {
	err = s.db.QueryRowContext(ctx,
		"SELECT m.has_result = 1 AND "+duelsCovered+" FROM matches m WHERE m.id = ?", matchID).Scan(&covered)
	if err != nil || !covered {
		return covered, []DuelCell{}, err
	}
	cells, err = s.duelCells(ctx, "", "m.id = ?", matchID)
	return covered, cells, err
}

// SessionDuels — дуэли сессии по матчам с посчитанными дуэлями.
type SessionDuels struct {
	Eligible  int     // матчи сессии с результатом
	Uncovered []Match // матчи с результатом, но без дуэлей, по порядку
	Cells     []DuelCell
}

func (s *Store) SessionDuels(ctx context.Context, sessionID int64) (SessionDuels, error) {
	var r SessionDuels
	if err := s.db.QueryRowContext(ctx,
		"SELECT count(*) FROM matches WHERE session_id = ? AND has_result = 1", sessionID).Scan(&r.Eligible); err != nil {
		return r, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+matchColumns+` FROM matches m
		WHERE session_id = ? AND has_result = 1 AND NOT (`+duelsCovered+`) ORDER BY ordinal`, sessionID)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	r.Uncovered = []Match{}
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return r, err
		}
		r.Uncovered = append(r.Uncovered, m)
	}
	if err := rows.Err(); err != nil {
		return r, err
	}
	r.Cells, err = s.duelCells(ctx, "", "m.session_id = ? AND m.has_result = 1 AND "+duelsCovered, sessionID)
	return r, err
}

// Opponent — личный счёт игрока с соперником за выборку.
type Opponent struct {
	SteamID uint64
	Name    string // ник из последнего матча соперника без учёта выборки
	Kills   int
	Deaths  int
	Maps    int
}

// PlayerDuels — дуэли игрока за выборку.
type PlayerDuels struct {
	Eligible        int // матчи игрока в выборке
	Covered         int // из них с посчитанными дуэлями
	CoveredSessions int // сессии, в которых есть такие матчи
	Opponents       []Opponent
}

// PlayerDuels суммирует личный счёт игрока со всеми соперниками по матчам выборки f
// с посчитанными дуэлями. Используются только период f (даты или сессии).
func (s *Store) PlayerDuels(ctx context.Context, steamID uint64, f MatchFilter) (PlayerDuels, error) {
	var r PlayerDuels
	where, args := MatchFilter{From: f.From, To: f.To, SessionIDs: f.SessionIDs}.where("m", "s")
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*), coalesce(sum(`+duelsCovered+`), 0),
			count(DISTINCT CASE WHEN `+duelsCovered+` THEN m.session_id END)
		`+fromPlayerMatches+`
		WHERE mp.steam_id = ? AND `+where, append([]any{int64(steamID)}, args...)...).
		Scan(&r.Eligible, &r.Covered, &r.CoveredSessions)
	if err != nil {
		return r, err
	}
	cells, err := s.duelCells(ctx, "JOIN sessions s ON s.id = m.session_id",
		"d.killer_id = ? AND "+where+" AND "+duelsCovered, append([]any{int64(steamID)}, args...)...)
	if err != nil {
		return r, err
	}
	// обратные убийства: соперник против игрока за те же матчи
	back, err := s.duelCells(ctx, "JOIN sessions s ON s.id = m.session_id",
		"d.victim_id = ? AND "+where+" AND "+duelsCovered, append([]any{int64(steamID)}, args...)...)
	if err != nil {
		return r, err
	}
	deaths := map[uint64]int{}
	for _, c := range back {
		deaths[c.Killer] = c.Kills
	}
	r.Opponents = make([]Opponent, 0, len(cells))
	for _, c := range cells {
		name, err := s.latestName(ctx, c.Victim)
		if err != nil {
			return r, err
		}
		r.Opponents = append(r.Opponents, Opponent{SteamID: c.Victim, Name: name, Kills: c.Kills, Deaths: deaths[c.Victim], Maps: c.Maps})
	}
	return r, nil
}

// latestName — ник игрока из его последнего матча с результатом (как в заголовке профиля).
func (s *Store) latestName(ctx context.Context, steamID uint64) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT mp.name `+fromPlayerMatches+`
		WHERE mp.steam_id = ? AND m.has_result = 1
		ORDER BY s.date DESC, s.id DESC, m.ordinal DESC LIMIT 1`, int64(steamID)).Scan(&name)
	return name, err
}
