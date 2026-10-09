package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"cs2stats/internal/store"
)

// Состояние расчёта дуэлей выборки.
const (
	duelsNoMatches   = "no_matches"  // нет матчей с результатом
	duelsUnavailable = "unavailable" // ни у одного матча нет дуэлей
	duelsPartial     = "partial"     // дуэли есть не у всех матчей
	duelsComplete    = "complete"
)

func duelsStatus(eligible, covered int) string {
	switch {
	case eligible == 0:
		return duelsNoMatches
	case covered == 0:
		return duelsUnavailable
	case covered < eligible:
		return duelsPartial
	}
	return duelsComplete
}

// share — доля K/(K+D) из сумм; без убийств в паре — null.
func share(kills, deaths int) *float64 {
	if kills+deaths == 0 {
		return nil
	}
	v := float64(kills) / float64(kills+deaths)
	return &v
}

type duelPlayer struct {
	SteamID string `json:"steamId"`
	Name    string `json:"name"`
	Team    string `json:"team,omitempty"`
}

type duelCell struct {
	KillerID string   `json:"killerId"`
	VictimID string   `json:"victimId"`
	Kills    int      `json:"kills"`
	Deaths   int      `json:"deaths"`
	Share    *float64 `json:"share"`
	Maps     int      `json:"maps"`
}

func cellsView(cells []store.DuelCell) []duelCell {
	out := make([]duelCell, 0, len(cells))
	for _, c := range cells {
		out = append(out, duelCell{
			KillerID: strconv.FormatUint(c.Killer, 10), VictimID: strconv.FormatUint(c.Victim, 10),
			Kills: c.Kills, Deaths: c.Deaths, Share: share(c.Kills, c.Deaths), Maps: c.Maps,
		})
	}
	return out
}

type matchDuelsResponse struct {
	Status  string       `json:"status"`
	Players []duelPlayer `json:"players"`
	Cells   []duelCell   `json:"cells"`
}

func (s *Server) getMatchDuels(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.GetMatch(r.Context(), id); err != nil {
		s.storeError(w, err, "матч не найден")
		return
	}
	covered, cells, err := s.Store.MatchDuels(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	resp := matchDuelsResponse{Status: duelsUnavailable, Players: []duelPlayer{}, Cells: cellsView(cells)}
	if !covered {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.Status = duelsComplete
	ps, err := s.Store.MatchPlayers(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	// порядок как в scoreboard: команда A, затем B, внутри — по rating
	for _, p := range sortScoreboard(ps) {
		resp.Players = append(resp.Players, duelPlayer{SteamID: p.SteamID, Name: p.Name, Team: p.Team})
	}
	writeJSON(w, http.StatusOK, resp)
}

type duelMatchRef struct {
	ID      int64 `json:"id"`
	Ordinal int   `json:"ordinal"`
}

type sessionDuelsResponse struct {
	Status           string         `json:"status"`
	EligibleMatches  int            `json:"eligibleMatches"`
	CoveredMatches   int            `json:"coveredMatches"`
	UncoveredMatches []duelMatchRef `json:"uncoveredMatches"`
	Players          []duelPlayer   `json:"players"`
	Cells            []duelCell     `json:"cells"`
}

func (s *Server) getSessionDuels(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.GetSession(r.Context(), id); err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	d, err := s.Store.SessionDuels(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	agg, err := s.Store.SessionPlayers(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	covered := d.Eligible - len(d.Uncovered)
	resp := sessionDuelsResponse{
		Status: duelsStatus(d.Eligible, covered), EligibleMatches: d.Eligible, CoveredMatches: covered,
		UncoveredMatches: make([]duelMatchRef, 0, len(d.Uncovered)), Players: []duelPlayer{}, Cells: cellsView(d.Cells),
	}
	for _, m := range d.Uncovered {
		resp.UncoveredMatches = append(resp.UncoveredMatches, duelMatchRef{ID: m.ID, Ordinal: m.Ordinal})
	}
	// порядок игроков как в итогах сессии
	for _, p := range totalsView(agg) {
		resp.Players = append(resp.Players, duelPlayer{SteamID: p.SteamID, Name: p.Name})
	}
	writeJSON(w, http.StatusOK, resp)
}

type opponentView struct {
	SteamID string   `json:"steamId"`
	Name    string   `json:"name"`
	Kills   int      `json:"kills"`
	Deaths  int      `json:"deaths"`
	Share   *float64 `json:"share"`
	Maps    int      `json:"maps"`
}

type playerDuelsResponse struct {
	Status          string         `json:"status"`
	EligibleMatches int            `json:"eligibleMatches"`
	CoveredMatches  int            `json:"coveredMatches"`
	CoveredSessions int            `json:"coveredSessions"`
	MostKilled      []opponentView `json:"mostKilled"`
	MostKilledBy    []opponentView `json:"mostKilledBy"`
	Opponents       []opponentView `json:"opponents"`
}

// maxBy возвращает всех соперников с наибольшим положительным значением.
func maxBy(list []opponentView, value func(opponentView) int) []opponentView {
	best := 0
	for _, o := range list {
		best = max(best, value(o))
	}
	out := []opponentView{}
	if best == 0 {
		return out
	}
	for _, o := range list {
		if value(o) == best {
			out = append(out, o)
		}
	}
	return out
}

func (s *Server) getPlayerDuels(w http.ResponseWriter, r *http.Request) {
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
	all, err := s.Store.PlayerTotals(ctx, store.MatchFilter{PlayerIDs: []uint64{steamID}}, 1)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if len(all) == 0 {
		writeError(w, http.StatusNotFound, "игрок не найден")
		return
	}
	d, err := s.Store.PlayerDuels(ctx, steamID, f)
	if err != nil {
		s.internalError(w, err)
		return
	}
	opps := make([]opponentView, 0, len(d.Opponents))
	for _, o := range d.Opponents {
		opps = append(opps, opponentView{SteamID: strconv.FormatUint(o.SteamID, 10), Name: o.Name,
			Kills: o.Kills, Deaths: o.Deaths, Share: share(o.Kills, o.Deaths), Maps: o.Maps})
	}
	// детерминированный порядок, в том числе при равных максимумах: по нику, затем по SteamID
	sort.SliceStable(opps, func(i, j int) bool {
		a, b := strings.ToLower(opps[i].Name), strings.ToLower(opps[j].Name)
		if a != b {
			return a < b
		}
		return opps[i].SteamID < opps[j].SteamID
	})
	writeJSON(w, http.StatusOK, playerDuelsResponse{
		Status:          duelsStatus(d.Eligible, d.Covered),
		EligibleMatches: d.Eligible, CoveredMatches: d.Covered, CoveredSessions: d.CoveredSessions,
		MostKilled:   maxBy(opps, func(o opponentView) int { return o.Kills }),
		MostKilledBy: maxBy(opps, func(o opponentView) int { return o.Deaths }),
		Opponents:    opps,
	})
}
