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

// RatingSwing возвращает данные Round Swing матчей с результатом, обработанных версией не ниже
// SwingSinceVersion: раунды по номеру со снаряжением и временем установки бомбы и убийства каждого
// раунда в порядке времени. У остальных матчей данных нет, их значение для OVR — Rating 2.0.
func (s *Store) RatingSwing(ctx context.Context) (map[int64]rating.SwingMatch, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.match_id, r.number, r.winner, r.side_a, r.equip_a, r.equip_b, r.plant_ms, r.restored
		FROM match_rounds r JOIN matches m ON m.id = r.match_id
		WHERE m.has_result = 1 AND m.result_version >= ?
		ORDER BY r.match_id, r.number`, SwingSinceVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]rating.SwingMatch{}
	index := map[[2]int64]int{} // (матч, номер раунда) → позиция в Rounds
	for rows.Next() {
		var matchID, number int64
		var r rating.SwingRound
		if err := rows.Scan(&matchID, &number, &r.Winner, &r.SideA, &r.EquipA, &r.EquipB, &r.PlantMs, &r.Restored); err != nil {
			return nil, err
		}
		m := out[matchID]
		index[[2]int64{matchID, number}] = len(m.Rounds)
		m.Rounds = append(m.Rounds, r)
		out[matchID] = m
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	kills, err := s.db.QueryContext(ctx, `
		SELECT k.match_id, k.round, k.time_ms, k.killer_id, k.victim_id, k.assister_id
		FROM match_kills k JOIN matches m ON m.id = k.match_id
		WHERE m.has_result = 1 AND m.result_version >= ?
		ORDER BY k.match_id, k.round, k.time_ms, k.seq`, SwingSinceVersion)
	if err != nil {
		return nil, err
	}
	defer kills.Close()
	for kills.Next() {
		var matchID, round int64
		var k rating.SwingKill
		var killer, victim, assister int64
		if err := kills.Scan(&matchID, &round, &k.TimeMs, &killer, &victim, &assister); err != nil {
			return nil, err
		}
		k.Killer, k.Victim, k.Assister = uint64(killer), uint64(victim), uint64(assister)
		if i, ok := index[[2]int64{matchID, round}]; ok {
			m := out[matchID]
			m.Rounds[i].Kills = append(m.Rounds[i].Kills, k)
		}
	}
	return out, kills.Err()
}
