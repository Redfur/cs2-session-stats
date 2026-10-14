package api

import (
	"math"
	"strconv"

	"cs2stats/internal/stats"
)

// PlayerView — строка scoreboard: сырые счётчики и производные показатели.
// SteamID передаётся строкой: 64-битное число не помещается в number JavaScript без потерь.
type PlayerView struct {
	SteamID string       `json:"steamId"`
	Name    string       `json:"name"`
	Team    string       `json:"team,omitempty"`
	Result  stats.Result `json:"result,omitempty"`
	Matches int          `json:"matches,omitempty"`
	Wins    int          `json:"wins,omitempty"`
	stats.Counters
	KD      float64 `json:"kd"`
	ADR     float64 `json:"adr"`
	HSPct   float64 `json:"hsPct"`
	KASTPct float64 `json:"kastPct"`
	Rating  float64 `json:"rating"`
	// Score — оценка 0–100 по rating; нет, если раундов нет.
	Score *int `json:"score,omitempty"`

	// Поля рейтинга, которые заполняются только в своих ответах.
	OVR     *OVRView           `json:"ovr,omitempty"`     // таблица игроков: текущий OVR
	Change  *SessionChangeView `json:"change,omitempty"`  // итоги сессии: OVR до и после
	Before  *int               `json:"before,omitempty"`  // scoreboard: OVR перед матчем
	VsUsual *int               `json:"vsUsual,omitempty"` // scoreboard: оценка за матч − before
}

func newPlayerView(steamID uint64, name string, c stats.Counters) PlayerView {
	var score *int
	if c.Rounds > 0 {
		v := stats.ScorePoints(c.Rating())
		score = &v
	}
	return PlayerView{
		Score:    score,
		SteamID:  strconv.FormatUint(steamID, 10),
		Name:     name,
		Counters: c,
		KD:       round2(c.KD()),
		ADR:      round2(c.ADR()),
		HSPct:    round2(c.HSPercent()),
		KASTPct:  round2(c.KASTPercent()),
		Rating:   round2(c.Rating()),
	}
}

// MatchPlayerView — строка scoreboard матча.
func MatchPlayerView(p stats.PlayerStats) PlayerView {
	v := newPlayerView(p.SteamID, p.Name, p.Counters)
	v.Team, v.Result = p.Team, p.Result
	return v
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// steamIDOf возвращает SteamID строки, который в JSON передаётся строкой.
func steamIDOf(p PlayerView) uint64 {
	id, _ := strconv.ParseUint(p.SteamID, 10, 64)
	return id
}
