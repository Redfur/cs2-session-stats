package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"testing"

	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

// ratingCounters — счётчики матча на 24 раунда с rating около target (rating линеен по урону).
func ratingCounters(target float64) stats.Counters {
	c := stats.Counters{Rounds: 24, Kills: 17, Deaths: 17, Assists: 3, KASTRounds: 17}
	c.Damage = int(math.Round((target - c.Rating()) / 0.0032 * 24))
	return c
}

type ratingPlayer struct {
	id     uint64
	team   string
	rating float64
}

// ratedSeq делает sha матчей уникальными.
var ratedSeq int

// addRatedMatch сохраняет матч с результатом и возвращает его id.
func addRatedMatch(t *testing.T, e *testEnv, sess store.Session, players ...ratingPlayer) int64 {
	t.Helper()
	ctx := context.Background()
	ratedSeq++
	m, err := e.store.AddMatch(ctx, sess.ID, fmt.Sprintf("rated-%d", ratedSeq), "m.dem")
	if err != nil {
		t.Fatal(err)
	}
	var ps []stats.PlayerStats
	for _, p := range players {
		ps = append(ps, stats.PlayerStats{SteamID: p.id, Name: strconv.FormatUint(p.id, 10), Team: p.team, Result: stats.Draw, Counters: ratingCounters(p.rating)})
	}
	if err := e.store.SaveMatchResult(ctx, m.ID, store.MatchResult{Map: "de_dust2", Rounds: 24, ScoreA: 12, ScoreB: 12, Players: ps}, 1); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func byID(players []PlayerView, id uint64) PlayerView {
	for _, p := range players {
		if steamIDOf(p) == id {
			return p
		}
	}
	return PlayerView{}
}

func TestRatingInViews(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	early, _ := e.store.CreateSession(ctx, "2026-09-01", "")
	late, _ := e.store.CreateSession(ctx, "2026-10-01", "")
	addRatedMatch(t, e, early, ratingPlayer{steamA, "A", 1.40}, ratingPlayer{steamB, "B", 0.80})
	m := addRatedMatch(t, e, late, ratingPlayer{steamA, "A", 1.00}, ratingPlayer{steamB, "B", 1.00})

	// оценка есть в scoreboard, итогах сессии, таблице игроков и разбивках профиля
	var match matchResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/matches/%d", m), "", nil, &match)
	a := byID(match.Players, steamA)
	if a.Score == nil || *a.Score != stats.ScorePoints(a.Rating) {
		t.Fatalf("оценка за матч: %+v", a.Score)
	}
	var sess sessionResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/sessions/%d", late.ID), "", nil, &sess)
	var prof profileResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/players/%d", steamA), "", nil, &prof)
	if byID(sess.Players, steamA).Score == nil || prof.Totals.Score == nil || prof.Sessions[0].Score == nil || prof.Matches[0].Score == nil || prof.Maps[0].Score == nil {
		t.Fatal("оценка должна быть в итогах сессии и разбивках профиля")
	}

	// OVR не зависит от периода таблицы игроков
	var all, period playersResponse
	e.do(t, http.MethodGet, "/api/players", "", nil, &all)
	e.do(t, http.MethodGet, "/api/players?from=2026-10-01", "", nil, &period)
	pa, pp := byID(all.Players, steamA), byID(period.Players, steamA)
	if pa.OVR == nil || pp.OVR == nil || *pa.OVR != *pp.OVR || pa.OVR.Matches != 2 || !pa.OVR.Calibrating {
		t.Fatalf("OVR за всё время %+v, за период %+v", pa.OVR, pp.OVR)
	}
	if pp.Matches != 1 {
		t.Errorf("агрегаты периода должны считаться по одному матчу: %d", pp.Matches)
	}
	// пустой период профиля: нулевые итоги без оценки, OVR остаётся
	var empty profileResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/players/%d?from=2027-01-01", steamA), "", nil, &empty)
	if empty.Totals.Score != nil || empty.OVR == nil || *empty.OVR != *pa.OVR {
		t.Errorf("пустой период: оценка %v, OVR %+v", empty.Totals.Score, empty.OVR)
	}
}

func TestProfileRating(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	first, _ := e.store.CreateSession(ctx, "2026-09-01", "")
	second, _ := e.store.CreateSession(ctx, "2026-10-01", "")
	for i := 0; i < 5; i++ {
		addRatedMatch(t, e, first, ratingPlayer{steamA, "A", 0.60})
	}
	var prof profileResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/players/%d", steamA), "", nil, &prof)
	ls := prof.LastSession
	if ls == nil || ls.SessionID != first.ID || ls.Before != nil || ls.Delta != nil || ls.VsUsual != nil || !ls.Calibrating || ls.Matches != 5 {
		t.Fatalf("первая сессия: %+v", ls)
	}
	// ещё 20 матчей: «Среднее за 20» — только по ним
	for i := 0; i < 20; i++ {
		addRatedMatch(t, e, second, ratingPlayer{steamA, "A", 1.20})
	}
	e.do(t, http.MethodGet, fmt.Sprintf("/api/players/%d", steamA), "", nil, &prof)
	if prof.Last20 == nil || *prof.Last20 != stats.ScorePoints(ratingCounters(1.20).Rating()) {
		t.Errorf("среднее за 20 = %v, ожидалась оценка последних 20 матчей", prof.Last20)
	}
	ls = prof.LastSession
	if ls == nil || ls.SessionID != second.ID || ls.Before == nil || *ls.Delta != ls.After-*ls.Before || ls.Calibrating || prof.OVR.Calibrating {
		t.Errorf("последняя сессия: %+v, OVR %+v", ls, prof.OVR)
	}
	if prof.Sessions[1].Change == nil || prof.Sessions[1].Change.Before != nil || prof.Sessions[0].Change.VsUsual == nil {
		t.Errorf("сравнение в разбивке по сессиям: %+v, %+v", prof.Sessions[0].Change, prof.Sessions[1].Change)
	}
	if prof.Matches[0].VsUsual == nil || prof.Matches[len(prof.Matches)-1].VsUsual != nil {
		t.Error("в списке матчей сравнение есть у всех, кроме первого матча")
	}
}

func TestSessionChange(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	first, _ := e.store.CreateSession(ctx, "2026-09-01", "")
	second, _ := e.store.CreateSession(ctx, "2026-10-01", "")
	addRatedMatch(t, e, first, ratingPlayer{steamA, "A", 1.00})
	low := addRatedMatch(t, e, second, ratingPlayer{steamA, "A", 0.70}, ratingPlayer{steamB, "B", 1.00})
	high := addRatedMatch(t, e, second, ratingPlayer{steamA, "A", 1.60})

	var sess sessionResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/sessions/%d", second.ID), "", nil, &sess)
	ch := byID(sess.Players, steamA).Change
	if ch == nil || ch.Before == nil || *ch.Delta != ch.After-*ch.Before || *ch.VsUsual != ch.Score-*ch.Before {
		t.Fatalf("изменение за сессию: %+v", ch)
	}
	if *ch.Before != stats.ScorePoints(ratingCounters(1.00).Rating()) {
		t.Errorf("OVR перед сессией = %d", *ch.Before)
	}
	if b := byID(sess.Players, steamB).Change; b == nil || b.Before != nil || b.Delta != nil {
		t.Errorf("у игрока, для которого сессия первая, изменения нет: %+v", b)
	}

	// перестановка матчей меняет OVR перед матчем; итог сессии при малом числе матчей — среднее, он тот же
	var m matchResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/matches/%d", low), "", nil, &m)
	before := *byID(m.Players, steamA).Before
	if _, err := e.store.ReorderMatches(ctx, second.ID, []int64{high, low}); err != nil {
		t.Fatal(err)
	}
	e.do(t, http.MethodGet, fmt.Sprintf("/api/matches/%d", low), "", nil, &m)
	if got := *byID(m.Players, steamA).Before; got <= before {
		t.Errorf("после перестановки перед матчем учтён сильный матч: OVR %d, было %d", got, before)
	}
	e.do(t, http.MethodGet, fmt.Sprintf("/api/sessions/%d", second.ID), "", nil, &sess)
	if got := byID(sess.Players, steamA).Change; got.After != ch.After || *got.Before != *ch.Before {
		t.Errorf("до и после сессии при перестановке внутри неё не меняются: %+v, было %+v", got, ch)
	}
}

func TestMatchTeamsOVR(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	sess, _ := e.store.CreateSession(ctx, "2026-10-01", "")
	ids := []uint64{steamA, steamB, 3, 4, 5, 6, 7, 8, 9, 10}
	var ps []ratingPlayer
	for i, id := range ids {
		team := "A"
		if i >= 5 {
			team = "B"
		}
		ps = append(ps, ratingPlayer{id, team, 0.8 + 0.05*float64(i)})
	}
	// первый матч у всех: среднего нет
	first := addRatedMatch(t, e, sess, ps...)
	var m matchResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/matches/%d", first), "", nil, &m)
	if m.Teams.A != nil || m.Teams.B != nil || m.Players[0].Before != nil || m.Players[0].VsUsual != nil {
		t.Fatalf("первый матч у всех: %+v, %+v", m.Teams, m.Players[0])
	}

	// во втором матче в команде A новичок вместо игрока 5
	second := append([]ratingPlayer{}, ps...)
	second[4] = ratingPlayer{11, "A", 1.0}
	id := addRatedMatch(t, e, sess, second...)
	e.do(t, http.MethodGet, fmt.Sprintf("/api/matches/%d", id), "", nil, &m)
	if m.Teams.A == nil || m.Teams.A.Rated != 4 || m.Teams.A.Total != 5 {
		t.Fatalf("команда A: %+v, ожидалось «по 4 из 5»", m.Teams.A)
	}
	var sum float64
	for _, p := range ps[:4] {
		sum += ratingCounters(p.rating).Rating()
	}
	if m.Teams.A.Score != stats.ScorePoints(sum/4) {
		t.Errorf("средний OVR A = %d, ожидалось по R̄ четырёх игроков %d", m.Teams.A.Score, stats.ScorePoints(sum/4))
	}
	if m.Teams.B == nil || m.Teams.B.Rated != 5 {
		t.Errorf("команда B: %+v", m.Teams.B)
	}
	a := byID(m.Players, steamA)
	if a.Before == nil || a.VsUsual == nil || *a.VsUsual != *a.Score-*a.Before {
		t.Errorf("сравнение с обычным: %+v %+v %+v", a.Before, a.VsUsual, a.Score)
	}
	if n := byID(m.Players, 11); n.Before != nil || n.VsUsual != nil {
		t.Error("у новичка сравнения нет")
	}
}
