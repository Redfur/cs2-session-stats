package api

import (
	"context"

	"cs2stats/internal/rating"
	"cs2stats/internal/stats"
)

// last20Matches — сколько последних матчей входит в «Среднее за 20».
const last20Matches = 20

// ratingBook считает историю OVR игроков группы по её матчам с результатом.
// Расчёт идёт на каждый запрос: матчей сотни, а перестановка и пересчёт сразу видны.
func (s *Server) ratingBook(ctx context.Context, groupID string) (*rating.Book, error) {
	rows, err := s.Store.RatingRows(ctx, groupID)
	if err != nil {
		return nil, err
	}
	swings, err := s.Store.RatingSwing(ctx, groupID)
	if err != nil {
		return nil, err
	}
	p := s.Rating
	if p.Alpha == 0 {
		p.Alpha = rating.DefaultAlpha
	}
	if p.Calibration == 0 {
		p.Calibration = 10
	}
	return rating.Build(rows, swings, p), nil
}

// OVRView — общий уровень игрока: баллы, R̄ до шкалы и признак калибровки.
type OVRView struct {
	Score       int     `json:"score"`
	Rating      float64 `json:"rating"`
	Matches     int     `json:"matches"`
	Calibrating bool    `json:"calibrating"`
	// порог калибровки — для счётчика «5/10»
	CalibrationMatches int `json:"calibrationMatches"`
}

func ovrView(b *rating.Book, st rating.State) *OVRView {
	return &OVRView{Score: st.Score(), Rating: round2(st.Rating), Matches: st.Matches,
		Calibrating: b.Calibrating(st.Matches), CalibrationMatches: b.CalibrationMatches()}
}

// playerOVR — текущий OVR игрока; nil, если матчей нет.
func playerOVR(b *rating.Book, steamID uint64) *OVRView {
	st, ok := b.OVR(steamID)
	if !ok {
		return nil
	}
	return ovrView(b, st)
}

// SessionChangeView — OVR игрока до и после сессии и оценка за неё. Разницы считаются
// по округлённым баллам, чтобы «65 → 67, +2» сходилось с показанными числами.
type SessionChangeView struct {
	SessionID   string `json:"sessionId"`
	Score       int    `json:"score"`             // оценка за сессию
	Before      *int   `json:"before,omitempty"`  // нет — сессия у игрока первая
	After       int    `json:"after"`             // OVR после последнего матча в сессии
	Delta       *int   `json:"delta,omitempty"`   // after − before
	VsUsual     *int   `json:"vsUsual,omitempty"` // score − before
	Matches     int    `json:"matches"`           // матчей игрока после сессии
	Calibrating bool   `json:"calibrating"`       // OVR после сессии ещё в калибровке
	// порог калибровки — для счётчика «8/10»
	CalibrationMatches int `json:"calibrationMatches"`
}

func sessionChangeView(b *rating.Book, ch rating.SessionChange) *SessionChangeView {
	v := &SessionChangeView{
		SessionID:   ch.SessionID,
		Score:       stats.ScorePoints(ch.Value),
		After:       ch.After.Score(),
		Matches:     ch.After.Matches,
		Calibrating: b.Calibrating(ch.After.Matches),

		CalibrationMatches: b.CalibrationMatches(),
	}
	if ch.HasBefore {
		before := ch.Before.Score()
		delta, vs := v.After-before, v.Score-before
		v.Before, v.Delta, v.VsUsual = &before, &delta, &vs
	}
	return v
}

// setMatchRating записывает в строку матча оценку за матч, OVR игрока перед матчем и разницу
// оценки с ним; ok = false, если перед матчем OVR не было (первый матч игрока).
func setMatchRating(p *PlayerView, b *rating.Book, steamID uint64, matchID string) (before rating.State, ok bool) {
	if st, found := b.Match(steamID, matchID); found {
		score := stats.ScorePoints(st.Value)
		p.Score = &score
	}
	before, ok = b.Before(steamID, matchID)
	if !ok {
		return before, false
	}
	v := before.Score()
	p.Before = &v
	if p.Score != nil {
		vs := *p.Score - v
		p.VsUsual = &vs
	}
	return before, true
}

// setSessionRating записывает в строку итогов сессии оценку за сессию и изменение OVR.
func setSessionRating(p *PlayerView, b *rating.Book, steamID uint64, sessionID string) {
	ch, ok := b.Session(steamID, sessionID)
	if !ok {
		return
	}
	p.Change = sessionChangeView(b, ch)
	score := p.Change.Score
	p.Score = &score
}

// TeamOVRView — средний OVR команды перед матчем по игрокам, у которых он был.
type TeamOVRView struct {
	Score int `json:"score"`
	Rated int `json:"rated"` // игроков с OVR перед матчем
	Total int `json:"total"` // игроков в команде
}

type teamsOVRView struct {
	A *TeamOVRView `json:"A,omitempty"`
	B *TeamOVRView `json:"B,omitempty"`
}

// withMatchRating дополняет строки scoreboard OVR перед матчем и сравнением с ним
// и считает средний OVR команд. Среднее берётся по R̄, а в баллы переводится потом:
// шкала нелинейна.
func withMatchRating(b *rating.Book, matchID string, players []PlayerView, ids []uint64) teamsOVRView {
	type acc struct {
		sum          float64
		rated, total int
	}
	teams := map[string]*acc{"A": {}, "B": {}}
	for i := range players {
		p := &players[i]
		t := teams[p.Team]
		if t != nil {
			t.total++
		}
		st, ok := setMatchRating(p, b, ids[i], matchID)
		if !ok {
			continue
		}
		if t != nil {
			t.sum += st.Rating
			t.rated++
		}
	}
	view := func(t *acc) *TeamOVRView {
		if t.rated == 0 {
			return nil
		}
		return &TeamOVRView{Score: stats.ScorePoints(t.sum / float64(t.rated)), Rated: t.rated, Total: t.total}
	}
	return teamsOVRView{A: view(teams["A"]), B: view(teams["B"])}
}
