package store

import (
	"context"

	"cs2stats/internal/stats"
)

// SessionPlayer — суммарная статистика игрока за сессию.
type SessionPlayer struct {
	SteamID uint64
	Name    string // ник из последнего по порядку матча
	Matches int
	Wins    int
	stats.Counters
}

// SessionPlayers суммирует сырые счётчики игроков по матчам сессии в статусе done.
// Производные показатели считаются из сумм, поэтому ADR, KAST%, HS% и rating
// автоматически взвешены по раундам и убийствам.
func (s *Store) SessionPlayers(ctx context.Context, sessionID int64) ([]SessionPlayer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT mp.steam_id,
			(SELECT mp2.name FROM match_players mp2 JOIN matches m2 ON m2.id = mp2.match_id
			 WHERE mp2.steam_id = mp.steam_id AND m2.session_id = m.session_id AND m2.status = ?
			 ORDER BY m2.ordinal DESC LIMIT 1),
			count(*), sum(mp.result = 'win'),
			sum(mp.rounds), sum(mp.kills), sum(mp.deaths), sum(mp.assists), sum(mp.hs_kills), sum(mp.damage),
			sum(mp.kast_rounds), sum(mp.k1), sum(mp.k2), sum(mp.k3), sum(mp.k4), sum(mp.k5),
			sum(mp.opening_kills), sum(mp.opening_deaths)
		FROM match_players mp JOIN matches m ON m.id = mp.match_id
		WHERE m.session_id = ? AND m.status = ?
		GROUP BY mp.steam_id`, StatusDone, sessionID, StatusDone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []SessionPlayer{}
	for rows.Next() {
		var p SessionPlayer
		var steamID int64
		err := rows.Scan(&steamID, &p.Name, &p.Matches, &p.Wins,
			&p.Rounds, &p.Kills, &p.Deaths, &p.Assists, &p.HSKills, &p.Damage,
			&p.KASTRounds, &p.K1, &p.K2, &p.K3, &p.K4, &p.K5, &p.OpeningKills, &p.OpeningDeaths)
		if err != nil {
			return nil, err
		}
		p.SteamID = uint64(steamID)
		list = append(list, p)
	}
	return list, rows.Err()
}
