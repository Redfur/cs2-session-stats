package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cs2stats/internal/stats"
)

// ExtendedSinceVersion — версия обработки, с которой вместе с результатом сохраняется расширенная
// статистика. Она посчитана, если has_result = 1 и result_version не ниже этой версии.
const ExtendedSinceVersion = 3

var extCovered = fmt.Sprintf("m.result_version >= %d", ExtendedSinceVersion)

var extFields = []string{"rounds", "traded_deaths", "trade_kills", "damage_dealt", "damage_taken",
	"flash_assists", "damage_assists", "rounds_t", "rounds_ct", "survived_t", "survived_ct", "alive_ms_t", "alive_ms_ct",
	"he_damage", "fire_damage", "flashed", "smokes", "he_kills", "fire_kills", "impact_kills",
	"smoke_kills", "wallbang_kills", "noscope_kills", "blind_kills", "distance_sum", "distance_kills",
	"plants", "plant_starts", "defuses", "defuse_starts"}

func extTargets(c *stats.ExtCounters) []any {
	return []any{&c.Rounds, &c.TradedDeaths, &c.TradeKills, &c.DamageDealt, &c.DamageTaken,
		&c.FlashAssists, &c.DamageAssists, &c.RoundsT, &c.RoundsCT, &c.SurvivedT, &c.SurvivedCT, &c.AliveMsT, &c.AliveMsCT,
		&c.HEDamage, &c.FireDamage, &c.Flashed, &c.Smokes, &c.HEKills, &c.FireKills, &c.ImpactKills,
		&c.SmokeKills, &c.WallbangKills, &c.NoScopeKills, &c.BlindKills, &c.DistanceSum, &c.DistanceKills,
		&c.Plants, &c.PlantStarts, &c.Defuses, &c.DefuseStarts}
}

func extValues(c stats.ExtCounters) []any {
	return []any{c.Rounds, c.TradedDeaths, c.TradeKills, c.DamageDealt, c.DamageTaken,
		c.FlashAssists, c.DamageAssists, c.RoundsT, c.RoundsCT, c.SurvivedT, c.SurvivedCT, c.AliveMsT, c.AliveMsCT,
		c.HEDamage, c.FireDamage, c.Flashed, c.Smokes, c.HEKills, c.FireKills, c.ImpactKills,
		c.SmokeKills, c.WallbangKills, c.NoScopeKills, c.BlindKills, c.DistanceSum, c.DistanceKills,
		c.Plants, c.PlantStarts, c.Defuses, c.DefuseStarts}
}

// saveExt заменяет расширенные данные матча в транзакции сохранения результата.
func saveExt(ctx context.Context, tx *sql.Tx, matchID int64, e *stats.Ext) error {
	for _, table := range []string{"match_player_ext", "match_player_weapons", "match_rounds", "match_kills", "match_clutches"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE match_id = ?", matchID); err != nil {
			return err
		}
	}
	if e == nil {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE matches SET first_round = ?, restored_round = ?, has_damage_events = ?, has_flash_events = ?, duration_ms = ?
		WHERE id = ?`, e.FirstRound, e.RestoredRound, e.HasDamageEvents, e.HasFlashEvents, e.Duration.Milliseconds(), matchID); err != nil {
		return err
	}
	insertExt := "INSERT INTO match_player_ext (match_id, steam_id, " + strings.Join(extFields, ", ") +
		") VALUES (?, ?" + strings.Repeat(", ?", len(extFields)) + ")"
	for _, p := range e.Players {
		if _, err := tx.ExecContext(ctx, insertExt, append([]any{matchID, int64(p.SteamID)}, extValues(p.ExtCounters)...)...); err != nil {
			return fmt.Errorf("расширенные счётчики игрока %d: %w", p.SteamID, err)
		}
	}
	for _, w := range e.Weapons {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO match_player_weapons (match_id, steam_id, weapon, kills, hs_kills, damage, shots, hits, hs_hits)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			matchID, int64(w.SteamID), w.Weapon, w.Kills, w.HSKills, w.Damage, w.Shots, w.Hits, w.HSHits)
		if err != nil {
			return fmt.Errorf("оружие %s игрока %d: %w", w.Weapon, w.SteamID, err)
		}
	}
	for _, r := range e.Rounds {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO match_rounds (match_id, number, winner, side_a, reason, start_ms, end_ms, restored)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			matchID, r.Number, r.Winner, r.SideA, r.Reason, r.Start.Milliseconds(), r.End.Milliseconds(), r.Restored)
		if err != nil {
			return fmt.Errorf("раунд %d: %w", r.Number, err)
		}
	}
	for _, k := range e.Kills {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO match_kills (match_id, round, seq, time_ms, killer_id, victim_id, assister_id, weapon, headshot,
				flash_assist, through_smoke, wallbang, noscope, attacker_blind, distance, trade)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			matchID, k.Round, k.Seq, k.Time.Milliseconds(), int64(k.Killer), int64(k.Victim), int64(k.Assister), k.Weapon,
			k.Headshot, k.FlashAssist, k.ThroughSmoke, k.Wallbang, k.NoScope, k.AttackerBlind, k.Distance, k.Trade)
		if err != nil {
			return fmt.Errorf("убийство %d/%d: %w", k.Round, k.Seq, err)
		}
	}
	for _, c := range e.Clutches {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO match_clutches (match_id, round, steam_id, vs, outcome) VALUES (?, ?, ?, ?, ?)",
			matchID, c.Round, int64(c.SteamID), c.Vs, c.Outcome)
		if err != nil {
			return fmt.Errorf("клатч раунда %d: %w", c.Round, err)
		}
	}
	return nil
}

// ExtRow — строка игрока в матче выборки для расширенной статистики. У матча без расширенных
// данных Covered = false, а Ext нулевой; базовые K, D, A есть всегда.
type ExtRow struct {
	MatchID   int64
	SessionID int64
	Ordinal   int
	Map       string
	SteamID   uint64
	Name      string
	Team      string
	Covered   bool
	HasDamage bool // в демке есть события урона
	HasFlash  bool // в демке есть события ослепления
	Kills     int
	Deaths    int
	Assists   int
	Rounds    int // базовые раунды
	Damage    int // базовый урон
	Ext       stats.ExtCounters
}

func (s *Store) extRows(ctx context.Context, where string, args ...any) ([]ExtRow, error) {
	cols := make([]string, len(extFields))
	for i, f := range extFields {
		cols[i] = "coalesce(x." + f + ", 0)"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.session_id, m.ordinal, m.map, mp.steam_id, mp.name, mp.team, `+extCovered+` AND x.match_id IS NOT NULL,
			m.has_damage_events, m.has_flash_events, mp.kills, mp.deaths, mp.assists, mp.rounds, mp.damage,
			`+strings.Join(cols, ", ")+`
		`+fromPlayerMatches+`
		LEFT JOIN match_player_ext x ON x.match_id = mp.match_id AND x.steam_id = mp.steam_id
		WHERE `+where+`
		ORDER BY s.date, s.id, m.ordinal`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ExtRow{}
	for rows.Next() {
		var r ExtRow
		var steamID int64
		dest := append([]any{&r.MatchID, &r.SessionID, &r.Ordinal, &r.Map, &steamID, &r.Name, &r.Team, &r.Covered,
			&r.HasDamage, &r.HasFlash, &r.Kills, &r.Deaths, &r.Assists, &r.Rounds, &r.Damage}, extTargets(&r.Ext)...)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		r.SteamID = uint64(steamID)
		list = append(list, r)
	}
	return list, rows.Err()
}

// PlayerExtRows — строки игрока по матчам периода f (даты или сессии).
func (s *Store) PlayerExtRows(ctx context.Context, steamID uint64, f MatchFilter) ([]ExtRow, error) {
	where, args := MatchFilter{From: f.From, To: f.To, SessionIDs: f.SessionIDs}.where("m", "s")
	return s.extRows(ctx, "mp.steam_id = ? AND "+where, append([]any{int64(steamID)}, args...)...)
}

// SessionExtRows — строки всех игроков по матчам сессии с результатом.
func (s *Store) SessionExtRows(ctx context.Context, sessionID int64) ([]ExtRow, error) {
	return s.extRows(ctx, "m.session_id = ? AND m.has_result = 1", sessionID)
}

// ClutchCount — число попыток клатча игрока с данным N и исходом.
type ClutchCount struct {
	SteamID uint64
	Vs      int
	Outcome stats.ClutchOutcome
	Count   int
}

func (s *Store) clutchCounts(ctx context.Context, where string, args ...any) ([]ClutchCount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.steam_id, c.vs, c.outcome, count(*)
		FROM match_clutches c JOIN matches m ON m.id = c.match_id JOIN sessions s ON s.id = m.session_id
		WHERE m.has_result = 1 AND `+extCovered+` AND `+where+`
		GROUP BY c.steam_id, c.vs, c.outcome
		ORDER BY c.steam_id, c.vs, c.outcome`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ClutchCount{}
	for rows.Next() {
		var c ClutchCount
		var steamID int64
		if err := rows.Scan(&steamID, &c.Vs, &c.Outcome, &c.Count); err != nil {
			return nil, err
		}
		c.SteamID = uint64(steamID)
		list = append(list, c)
	}
	return list, rows.Err()
}

// PlayerClutches — попытки клатча игрока за период.
func (s *Store) PlayerClutches(ctx context.Context, steamID uint64, f MatchFilter) ([]ClutchCount, error) {
	where, args := MatchFilter{From: f.From, To: f.To, SessionIDs: f.SessionIDs}.where("m", "s")
	return s.clutchCounts(ctx, "c.steam_id = ? AND "+where, append([]any{int64(steamID)}, args...)...)
}

// SessionClutches — попытки клатча игроков сессии.
func (s *Store) SessionClutches(ctx context.Context, sessionID int64) ([]ClutchCount, error) {
	return s.clutchCounts(ctx, "m.session_id = ?", sessionID)
}

// WeaponTotal — счётчики по оружию. Урон и попадания — только по матчам с событиями урона.
type WeaponTotal struct {
	SteamID uint64
	Weapon  string
	Kills   int
	HSKills int
	Damage  int
	Shots   int
	Hits    int
	HSHits  int
}

func (s *Store) weaponTotals(ctx context.Context, where string, args ...any) ([]WeaponTotal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT w.steam_id, w.weapon, sum(w.kills), sum(w.hs_kills),
			sum(CASE WHEN m.has_damage_events THEN w.damage ELSE 0 END), sum(w.shots),
			sum(CASE WHEN m.has_damage_events THEN w.hits ELSE 0 END),
			sum(CASE WHEN m.has_damage_events THEN w.hs_hits ELSE 0 END)
		FROM match_player_weapons w JOIN matches m ON m.id = w.match_id JOIN sessions s ON s.id = m.session_id
		WHERE m.has_result = 1 AND `+extCovered+` AND `+where+`
		GROUP BY w.steam_id, w.weapon
		ORDER BY w.steam_id, sum(w.kills) DESC, sum(w.damage) DESC, w.weapon`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []WeaponTotal{}
	for rows.Next() {
		var w WeaponTotal
		var steamID int64
		if err := rows.Scan(&steamID, &w.Weapon, &w.Kills, &w.HSKills, &w.Damage, &w.Shots, &w.Hits, &w.HSHits); err != nil {
			return nil, err
		}
		w.SteamID = uint64(steamID)
		list = append(list, w)
	}
	return list, rows.Err()
}

// PlayerWeapons — оружие игрока за период.
func (s *Store) PlayerWeapons(ctx context.Context, steamID uint64, f MatchFilter) ([]WeaponTotal, error) {
	where, args := MatchFilter{From: f.From, To: f.To, SessionIDs: f.SessionIDs}.where("m", "s")
	return s.weaponTotals(ctx, "w.steam_id = ? AND "+where, append([]any{int64(steamID)}, args...)...)
}

// MatchExt — расширенные данные матча.
type MatchExt struct {
	Covered         bool
	FirstRound      int
	RestoredRound   bool
	HasDamageEvents bool
	HasFlashEvents  bool
	Duration        time.Duration
	Rounds          []stats.RoundInfo
	Kills           []stats.KillEvent
	Clutches        []stats.Clutch
	Players         []ExtRow
	Weapons         []WeaponTotal
}

// GetMatchExt возвращает расширенные данные матча. Covered = false, если у матча нет
// результата или он получен версией без расширенной статистики.
func (s *Store) GetMatchExt(ctx context.Context, matchID int64) (MatchExt, error) {
	var e MatchExt
	var durMs int64
	err := s.db.QueryRowContext(ctx, `
		SELECT m.has_result = 1 AND `+extCovered+`, m.first_round, m.restored_round, m.has_damage_events,
			m.has_flash_events, m.duration_ms
		FROM matches m WHERE m.id = ?`, matchID).
		Scan(&e.Covered, &e.FirstRound, &e.RestoredRound, &e.HasDamageEvents, &e.HasFlashEvents, &durMs)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	if err != nil || !e.Covered {
		return e, err
	}
	e.Duration = time.Duration(durMs) * time.Millisecond

	rows, err := s.db.QueryContext(ctx, `
		SELECT number, winner, side_a, reason, start_ms, end_ms, restored FROM match_rounds
		WHERE match_id = ? ORDER BY number`, matchID)
	if err != nil {
		return e, err
	}
	for rows.Next() {
		var r stats.RoundInfo
		var start, end int64
		if err := rows.Scan(&r.Number, &r.Winner, &r.SideA, &r.Reason, &start, &end, &r.Restored); err != nil {
			rows.Close()
			return e, err
		}
		r.Start, r.End = time.Duration(start)*time.Millisecond, time.Duration(end)*time.Millisecond
		e.Rounds = append(e.Rounds, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return e, err
	}

	rows, err = s.db.QueryContext(ctx, `
		SELECT round, seq, time_ms, killer_id, victim_id, assister_id, weapon, headshot, flash_assist,
			through_smoke, wallbang, noscope, attacker_blind, distance, trade
		FROM match_kills WHERE match_id = ? ORDER BY round, seq`, matchID)
	if err != nil {
		return e, err
	}
	for rows.Next() {
		var k stats.KillEvent
		var t, killer, victim, assister int64
		if err := rows.Scan(&k.Round, &k.Seq, &t, &killer, &victim, &assister, &k.Weapon, &k.Headshot, &k.FlashAssist,
			&k.ThroughSmoke, &k.Wallbang, &k.NoScope, &k.AttackerBlind, &k.Distance, &k.Trade); err != nil {
			rows.Close()
			return e, err
		}
		k.Time = time.Duration(t) * time.Millisecond
		k.Killer, k.Victim, k.Assister = uint64(killer), uint64(victim), uint64(assister)
		e.Kills = append(e.Kills, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return e, err
	}

	rows, err = s.db.QueryContext(ctx,
		"SELECT round, steam_id, vs, outcome FROM match_clutches WHERE match_id = ? ORDER BY round, steam_id", matchID)
	if err != nil {
		return e, err
	}
	for rows.Next() {
		var c stats.Clutch
		var steamID int64
		if err := rows.Scan(&c.Round, &steamID, &c.Vs, &c.Outcome); err != nil {
			rows.Close()
			return e, err
		}
		c.SteamID = uint64(steamID)
		e.Clutches = append(e.Clutches, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return e, err
	}

	if e.Players, err = s.extRows(ctx, "m.id = ?", matchID); err != nil {
		return e, err
	}
	e.Weapons, err = s.weaponTotals(ctx, "m.id = ?", matchID)
	return e, err
}
