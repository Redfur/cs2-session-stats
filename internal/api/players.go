package api

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

// parsePeriod разбирает период выборки: либо даты from/to, либо список session.
func parsePeriod(q url.Values) (store.MatchFilter, error) {
	var f store.MatchFilter
	f.From, f.To = q.Get("from"), q.Get("to")
	for _, d := range []string{f.From, f.To} {
		if d == "" {
			continue
		}
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			return f, errors.New("некорректная дата, ожидается ГГГГ-ММ-ДД")
		}
	}
	for _, v := range q["session"] {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return f, errors.New("некорректный идентификатор сессии: " + v)
		}
		f.SessionIDs = append(f.SessionIDs, id)
	}
	if (f.From != "" || f.To != "") && len(f.SessionIDs) > 0 {
		return f, errors.New("укажите период либо датами, либо сессиями")
	}
	return f, nil
}

// parsePlayersQuery разбирает фильтры общей таблицы игроков.
func parsePlayersQuery(q url.Values) (f store.MatchFilter, minMatches int, err error) {
	if f, err = parsePeriod(q); err != nil {
		return f, 0, err
	}
	for _, v := range q["player"] {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil || id == 0 {
			return f, 0, errors.New("некорректный SteamID: " + v)
		}
		f.PlayerIDs = append(f.PlayerIDs, id)
	}
	switch q.Get("together") {
	case "", "0", "false":
	case "1", "true":
		f.Together = true
	default:
		return f, 0, errors.New("параметр together принимает 1 или 0")
	}
	minMatches = 1
	if v := q.Get("minMatches"); v != "" {
		if minMatches, err = strconv.Atoi(v); err != nil || minMatches < 1 {
			return f, 0, errors.New("минимум матчей должен быть целым числом не меньше 1")
		}
	}
	return f, minMatches, nil
}

func aggregateView(steamID uint64, name string, a store.Aggregate) PlayerView {
	v := newPlayerView(steamID, name, a.Counters)
	v.Matches, v.Wins = a.Matches, a.Wins
	return v
}

func totalsView(list []store.PlayerTotal) []PlayerView {
	players := make([]PlayerView, 0, len(list))
	for _, p := range list {
		players = append(players, aggregateView(p.SteamID, p.Name, p.Aggregate))
	}
	sort.SliceStable(players, func(i, j int) bool { return players[i].Rating > players[j].Rating })
	return players
}

type playersResponse struct {
	Players []PlayerView `json:"players"`
}

func (s *Server) listPlayers(w http.ResponseWriter, r *http.Request) {
	f, minMatches, err := parsePlayersQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := s.Store.PlayerTotals(r.Context(), f, minMatches)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, playersResponse{Players: totalsView(list)})
}

type profileSession struct {
	Session store.Session `json:"session"`
	PlayerView
}

type profileMap struct {
	Map string `json:"map"`
	PlayerView
}

type profileMatch struct {
	MatchID      int64  `json:"matchId"`
	SessionID    int64  `json:"sessionId"`
	SessionDate  string `json:"sessionDate"`
	SessionTitle string `json:"sessionTitle"`
	Ordinal      int    `json:"ordinal"`
	Map          string `json:"map"`
	ScoreA       int    `json:"scoreA"`
	ScoreB       int    `json:"scoreB"`
	PlayerView
}

type profileResponse struct {
	SteamID  string           `json:"steamId"`
	Name     string           `json:"name"`
	Totals   PlayerView       `json:"totals"`
	Sessions []profileSession `json:"sessions"`
	Maps     []profileMap     `json:"maps"`
	Matches  []profileMatch   `json:"matches"`
}

func (s *Server) getPlayer(w http.ResponseWriter, r *http.Request) {
	steamID, err := strconv.ParseUint(r.PathValue("steamId"), 10, 64)
	if err != nil || steamID == 0 {
		writeError(w, http.StatusBadRequest, "некорректный SteamID")
		return
	}
	f, err := parsePeriod(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	// ник для заголовка — из последнего матча игрока без учёта периода
	all, err := s.Store.PlayerTotals(ctx, store.MatchFilter{PlayerIDs: []uint64{steamID}}, 1)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if len(all) == 0 {
		writeError(w, http.StatusNotFound, "игрок не найден")
		return
	}
	name := all[0].Name

	f.PlayerIDs = []uint64{steamID}
	totals, err := s.Store.PlayerTotals(ctx, f, 1)
	if err != nil {
		s.internalError(w, err)
		return
	}
	var total store.Aggregate
	if len(totals) > 0 {
		total = totals[0].Aggregate
	}
	sessions, err := s.Store.PlayerSessions(ctx, steamID, f)
	if err != nil {
		s.internalError(w, err)
		return
	}
	maps, err := s.Store.PlayerMaps(ctx, steamID, f)
	if err != nil {
		s.internalError(w, err)
		return
	}
	matches, err := s.Store.PlayerMatches(ctx, steamID, f)
	if err != nil {
		s.internalError(w, err)
		return
	}

	resp := profileResponse{
		SteamID:  strconv.FormatUint(steamID, 10),
		Name:     name,
		Totals:   aggregateView(steamID, name, total),
		Sessions: make([]profileSession, 0, len(sessions)),
		Maps:     make([]profileMap, 0, len(maps)),
		Matches:  make([]profileMatch, 0, len(matches)),
	}
	for _, p := range sessions {
		resp.Sessions = append(resp.Sessions, profileSession{Session: p.Session, PlayerView: aggregateView(steamID, name, p.Aggregate)})
	}
	for _, p := range maps {
		resp.Maps = append(resp.Maps, profileMap{Map: p.Map, PlayerView: aggregateView(steamID, name, p.Aggregate)})
	}
	for _, p := range matches {
		v := MatchPlayerView(stats.PlayerStats{SteamID: steamID, Name: name, Team: p.Team, Result: p.Result, Counters: p.Counters})
		resp.Matches = append(resp.Matches, profileMatch{
			MatchID: p.MatchID, SessionID: p.SessionID, SessionDate: p.SessionDate, SessionTitle: p.SessionTitle,
			Ordinal: p.Ordinal, Map: p.Map, ScoreA: p.ScoreA, ScoreB: p.ScoreB, PlayerView: v,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
