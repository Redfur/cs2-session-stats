package store

import (
	"context"

	"cs2stats/internal/rating"
)

// RatingRows возвращает строки всех игроков во всех матчах с результатом в порядке игры:
// по дате сессии, порядку создания сессий с одной датой и номеру матча. Строки одного матча
// идут подряд. Фильтры не применяются: OVR всегда считается по всей истории.
func (s *Store) RatingRows(ctx context.Context) ([]rating.MatchRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.session_id, mp.steam_id, mp.team, `+counterColumns+`
		`+fromPlayerMatches+`
		WHERE m.has_result = 1
		ORDER BY s.date, s.id, m.ordinal, m.id, mp.steam_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []rating.MatchRow
	for rows.Next() {
		var r rating.MatchRow
		var steamID int64
		if err := rows.Scan(append([]any{&r.MatchID, &r.SessionID, &steamID, &r.Team}, counterTargets(&r.Counters)...)...); err != nil {
			return nil, err
		}
		r.SteamID = uint64(steamID)
		list = append(list, r)
	}
	return list, rows.Err()
}
