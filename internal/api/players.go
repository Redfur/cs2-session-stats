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
	gid, ok := s.groupFromPath(w, r)
	if !ok {
		return
	}
	f, minMatches, err := parsePlayersQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	f.GroupID = gid
	list, err := s.Store.PlayerTotals(r.Context(), f, minMatches)
	if err != nil {
		s.internalError(w, err)
		return
	}
	book, err := s.ratingBook(r.Context(), gid)
	if err != nil {
		s.internalError(w, err)
		return
	}
	players := totalsView(list)
	for i := range players {
		players[i].OVR = playerOVR(book, steamIDOf(players[i]))
	}
	writeJSON(w, http.StatusOK, playersResponse{Players: players})
}

// playerRequest разбирает запрос профиля игрока в группе: группу, SteamID и период.
// name — ник из последнего матча игрока в группе без учёта периода. Игрок без матчей
// с результатом в группе — 404. ok=false — ответ уже записан.
func (s *Server) playerRequest(w http.ResponseWriter, r *http.Request) (steamID uint64, f store.MatchFilter, name string, ok bool) {
	gid, ok := s.groupFromPath(w, r)
	if !ok {
		return
	}
	steamID, err := strconv.ParseUint(r.PathValue("steamId"), 10, 64)
	if err != nil || steamID == 0 {
		writeError(w, http.StatusBadRequest, "некорректный SteamID")
		return 0, f, "", false
	}
	if f, err = parsePeriod(r.URL.Query()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return 0, f, "", false
	}
	f.GroupID = gid
	all, err := s.Store.PlayerTotals(r.Context(), store.MatchFilter{GroupID: gid, PlayerIDs: []uint64{steamID}}, 1)
	if err != nil {
		s.internalError(w, err)
		return 0, f, "", false
	}
	if len(all) == 0 {
		writeError(w, http.StatusNotFound, "игрок не найден")
		return 0, f, "", false
	}
	return steamID, f, all[0].Name, true
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
	SteamID string     `json:"steamId"`
	Name    string     `json:"name"`
	Totals  PlayerView `json:"totals"`
	// OVR, «Среднее за 20» и последняя сессия не зависят от периода.
	OVR         *OVRView           `json:"ovr,omitempty"`
	Last20      *int               `json:"last20,omitempty"`
	LastSession *SessionChangeView `json:"lastSession,omitempty"`
	Sessions    []profileSession   `json:"sessions"`
	Maps        []profileMap       `json:"maps"`
	Matches     []profileMatch     `json:"matches"`
}

func (s *Server) getPlayer(w http.ResponseWriter, r *http.Request) {
	steamID, f, name, ok := s.playerRequest(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

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

	book, err := s.ratingBook(ctx, f.GroupID)
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
		OVR:      playerOVR(book, steamID),
	}
	if v, ok := book.Last(steamID, last20Matches); ok {
		score := stats.ScorePoints(v)
		resp.Last20 = &score
	}
	if ch, ok := book.LastSession(steamID); ok {
		resp.LastSession = sessionChangeView(book, ch)
	}
	for _, p := range sessions {
		v := aggregateView(steamID, name, p.Aggregate)
		setSessionRating(&v, book, steamID, p.Session.ID)
		resp.Sessions = append(resp.Sessions, profileSession{Session: p.Session, PlayerView: v})
	}
	for _, p := range maps {
		resp.Maps = append(resp.Maps, profileMap{Map: p.Map, PlayerView: aggregateView(steamID, name, p.Aggregate)})
	}
	for _, p := range matches {
		v := MatchPlayerView(stats.PlayerStats{SteamID: steamID, Name: name, Team: p.Team, Result: p.Result, Counters: p.Counters})
		setMatchRating(&v, book, steamID, p.MatchID)
		resp.Matches = append(resp.Matches, profileMatch{
			MatchID: p.MatchID, SessionID: p.SessionID, SessionDate: p.SessionDate, SessionTitle: p.SessionTitle,
			Ordinal: p.Ordinal, Map: p.Map, ScoreA: p.ScoreA, ScoreB: p.ScoreB, PlayerView: v,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
