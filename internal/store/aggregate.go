package store

import (
	"context"
	"strings"

	"cs2stats/internal/stats"
)

// MatchFilter — выборка матчей для агрегатов. В выборку всегда входят только матчи
// с посчитанным результатом, включая пересчитываемые и матчи с неудачным пересчётом.
type MatchFilter struct {
	From, To   string   // границы по дате сессии YYYY-MM-DD включительно; пустая строка — без границы
	SessionIDs []int64  // только эти сессии; пусто — все
	PlayerIDs  []uint64 // в PlayerTotals: только строки этих игроков
	Together   bool     // только матчи, в которых сыграли все PlayerIDs
}

// where возвращает условие выборки для запроса, где m — псевдоним matches, s — sessions.
func (f MatchFilter) where(m, s string) (string, []any) {
	conds := []string{m + ".has_result = 1"}
	var args []any
	if f.From != "" {
		conds = append(conds, s+".date >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		conds = append(conds, s+".date <= ?")
		args = append(args, f.To)
	}
	if len(f.SessionIDs) > 0 {
		conds = append(conds, m+".session_id IN ("+placeholders(len(f.SessionIDs))+")")
		for _, id := range f.SessionIDs {
			args = append(args, id)
		}
	}
	if ids := uniqueIDs(f.PlayerIDs); f.Together && len(ids) > 0 {
		conds = append(conds, m+".id IN (SELECT match_id FROM match_players WHERE steam_id IN ("+
			placeholders(len(ids))+") GROUP BY match_id HAVING count(DISTINCT steam_id) = ?)")
		args = append(args, ids...)
		args = append(args, len(ids))
	}
	return strings.Join(conds, " AND "), args
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// uniqueIDs возвращает SteamID без повторов в виде аргументов запроса.
func uniqueIDs(ids []uint64) []any {
	seen := make(map[uint64]bool, len(ids))
	var out []any
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, int64(id))
		}
	}
	return out
}

const fromPlayerMatches = `FROM match_players mp
	JOIN matches m ON m.id = mp.match_id
	JOIN sessions s ON s.id = m.session_id`

var counterFields = []string{"rounds", "kills", "deaths", "assists", "hs_kills", "damage", "kast_rounds",
	"k1", "k2", "k3", "k4", "k5", "opening_kills", "opening_deaths"}

// sumColumns — число матчей, побед и суммы сырых счётчиков; читается через aggTargets.
var sumColumns = "count(*), sum(mp.result = 'win'), " + joinFields("sum(mp.", ")")

// counterColumns — сырые счётчики одной строки match_players; читаются через counterTargets.
var counterColumns = joinFields("mp.", "")

func joinFields(prefix, suffix string) string {
	cols := make([]string, len(counterFields))
	for i, f := range counterFields {
		cols[i] = prefix + f + suffix
	}
	return strings.Join(cols, ", ")
}

func counterTargets(c *stats.Counters) []any {
	return []any{&c.Rounds, &c.Kills, &c.Deaths, &c.Assists, &c.HSKills, &c.Damage, &c.KASTRounds,
		&c.K1, &c.K2, &c.K3, &c.K4, &c.K5, &c.OpeningKills, &c.OpeningDeaths}
}

func aggTargets(a *Aggregate) []any {
	return append([]any{&a.Matches, &a.Wins}, counterTargets(&a.Counters)...)
}

// Aggregate — суммы по нескольким матчам игрока. Производные показатели считаются из сумм,
// поэтому ADR, KAST%, HS% и rating автоматически взвешены по раундам и убийствам.
type Aggregate struct {
	Matches int
	Wins    int
	stats.Counters
}

// PlayerTotal — суммарная статистика игрока за выборку.
type PlayerTotal struct {
	SteamID uint64
	Name    string // ник из последнего матча выборки
	Aggregate
}

// PlayerTotals суммирует сырые счётчики игроков по матчам выборки и оставляет игроков,
// сыгравших не меньше minMatches матчей. Последним матчем для ника считается матч сессии
// с более поздней датой (при равных — созданной позже) и с большим порядковым номером.
func (s *Store) PlayerTotals(ctx context.Context, f MatchFilter, minMatches int) ([]PlayerTotal, error) {
	nameWhere, nameArgs := f.where("m2", "s2")
	where, args := f.where("m", "s")
	if ids := uniqueIDs(f.PlayerIDs); len(ids) > 0 {
		where += " AND mp.steam_id IN (" + placeholders(len(ids)) + ")"
		args = append(args, ids...)
	}
	query := `
		SELECT mp.steam_id,
			(SELECT mp2.name FROM match_players mp2
			 JOIN matches m2 ON m2.id = mp2.match_id JOIN sessions s2 ON s2.id = m2.session_id
			 WHERE mp2.steam_id = mp.steam_id AND ` + nameWhere + `
			 ORDER BY s2.date DESC, s2.id DESC, m2.ordinal DESC LIMIT 1),
			` + sumColumns + `
		` + fromPlayerMatches + `
		WHERE ` + where + `
		GROUP BY mp.steam_id
		HAVING count(*) >= ?`
	all := append(append(nameArgs, args...), minMatches)
	rows, err := s.db.QueryContext(ctx, query, all...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []PlayerTotal{}
	for rows.Next() {
		var p PlayerTotal
		var steamID int64
		if err := rows.Scan(append([]any{&steamID, &p.Name}, aggTargets(&p.Aggregate)...)...); err != nil {
			return nil, err
		}
		p.SteamID = uint64(steamID)
		list = append(list, p)
	}
	return list, rows.Err()
}

// SessionPlayers суммирует статистику игроков по матчам сессии с результатом.
func (s *Store) SessionPlayers(ctx context.Context, sessionID int64) ([]PlayerTotal, error) {
	return s.PlayerTotals(ctx, MatchFilter{SessionIDs: []int64{sessionID}}, 1)
}

// PlayerSession — статистика игрока за одну сессию.
type PlayerSession struct {
	Session Session
	Aggregate
}

// PlayerSessions возвращает статистику игрока по сессиям выборки, от новых к старым.
func (s *Store) PlayerSessions(ctx context.Context, steamID uint64, f MatchFilter) ([]PlayerSession, error) {
	where, args := f.where("m", "s")
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.date, s.title, s.created_at, `+sumColumns+`
		`+fromPlayerMatches+`
		WHERE mp.steam_id = ? AND `+where+`
		GROUP BY s.id
		ORDER BY s.date DESC, s.id DESC`, append([]any{int64(steamID)}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []PlayerSession{}
	for rows.Next() {
		var p PlayerSession
		sess := &p.Session
		dest := append([]any{&sess.ID, &sess.Date, &sess.Title, &sess.CreatedAt}, aggTargets(&p.Aggregate)...)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// PlayerMap — статистика игрока на одной карте.
type PlayerMap struct {
	Map string
	Aggregate
}

// PlayerMaps возвращает статистику игрока по картам выборки, от самых частых к редким.
func (s *Store) PlayerMaps(ctx context.Context, steamID uint64, f MatchFilter) ([]PlayerMap, error) {
	where, args := f.where("m", "s")
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.map, `+sumColumns+`
		`+fromPlayerMatches+`
		WHERE mp.steam_id = ? AND `+where+`
		GROUP BY m.map
		ORDER BY count(*) DESC, m.map`, append([]any{int64(steamID)}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []PlayerMap{}
	for rows.Next() {
		var p PlayerMap
		if err := rows.Scan(append([]any{&p.Map}, aggTargets(&p.Aggregate)...)...); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// PlayerMatch — строка игрока в одном матче вместе с данными матча и сессии.
type PlayerMatch struct {
	MatchID      int64
	SessionID    int64
	SessionDate  string
	SessionTitle string
	Ordinal      int
	Map          string
	ScoreA       int
	ScoreB       int
	Team         string
	Result       stats.Result
	stats.Counters
}

// PlayerMatches возвращает матчи игрока из выборки, от новых к старым.
func (s *Store) PlayerMatches(ctx context.Context, steamID uint64, f MatchFilter) ([]PlayerMatch, error) {
	where, args := f.where("m", "s")
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.session_id, s.date, s.title, m.ordinal, m.map, m.score_a, m.score_b,
			mp.team, mp.result, `+counterColumns+`
		`+fromPlayerMatches+`
		WHERE mp.steam_id = ? AND `+where+`
		ORDER BY s.date DESC, s.id DESC, m.ordinal DESC`, append([]any{int64(steamID)}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []PlayerMatch{}
	for rows.Next() {
		var p PlayerMatch
		dest := append([]any{&p.MatchID, &p.SessionID, &p.SessionDate, &p.SessionTitle, &p.Ordinal, &p.Map,
			&p.ScoreA, &p.ScoreB, &p.Team, &p.Result}, counterTargets(&p.Counters)...)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}
