package api

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

// Больше 2^53: в JSON number SteamID потерял бы точность.
const (
	steamA uint64 = 76561198000000001
	steamB uint64 = 76561198000000002
)

// seedPlayers: сессия 2026-10-01 — матч A+B; сессия 2026-10-08 — матч A+B и матч только A.
func seedPlayers(t *testing.T, e *testEnv) (s1, s2 store.Session) {
	t.Helper()
	ctx := context.Background()
	s1, _ = e.store.CreateSession(ctx, "2026-10-01", "")
	s2, _ = e.store.CreateSession(ctx, "2026-10-08", "четверг")
	n := 0
	add := func(sess store.Session, ids ...uint64) {
		n++
		m, err := e.store.AddMatch(ctx, sess.ID, fmt.Sprint(n), "m.dem")
		if err != nil {
			t.Fatal(err)
		}
		var ps []stats.PlayerStats
		for i, id := range ids {
			team, res := "A", stats.Win
			if i == 1 {
				team, res = "B", stats.Loss
			}
			ps = append(ps, stats.PlayerStats{SteamID: id, Name: fmt.Sprintf("p%d_%d", id%10, n), Team: team, Result: res,
				Counters: stats.Counters{Rounds: 20, Kills: 10 * int(id%10), Deaths: 10, Damage: 1600}})
		}
		if err := e.store.SaveMatchResult(ctx, m.ID, store.MatchResult{Map: "de_mirage", Rounds: 20, ScoreA: 13, ScoreB: 7, Players: ps}, 1); err != nil {
			t.Fatal(err)
		}
	}
	add(s1, steamA, steamB)
	add(s2, steamA, steamB)
	add(s2, steamA)
	return s1, s2
}

func TestListPlayers(t *testing.T) {
	e := newTestEnv(t)
	s1, s2 := seedPlayers(t, e)

	get := func(query string) (int, playersResponse) {
		t.Helper()
		var resp playersResponse
		code := e.do(t, "GET", "/api/players?"+query, "", nil, &resp)
		return code, resp
	}

	code, resp := get("")
	if code != 200 || len(resp.Players) != 2 {
		t.Fatalf("без фильтров: %d %+v", code, resp)
	}
	// сортировка по rating: у B вдвое больше убийств
	a, b := resp.Players[1], resp.Players[0]
	if b.SteamID != fmt.Sprint(steamB) || a.SteamID != fmt.Sprint(steamA) {
		t.Errorf("порядок или SteamID строкой: %s, %s", b.SteamID, a.SteamID)
	}
	if a.Matches != 3 || a.Wins != 3 || a.ADR != 80 || a.Name != "p1_3" {
		t.Errorf("A: %+v", a)
	}

	cases := []struct {
		query string
		want  map[string]int // SteamID → матчей
	}{
		{"from=2026-10-08", map[string]int{fmt.Sprint(steamA): 2, fmt.Sprint(steamB): 1}},
		{"to=2026-10-01", map[string]int{fmt.Sprint(steamA): 1, fmt.Sprint(steamB): 1}},
		{fmt.Sprintf("session=%d&session=%d", s1.ID, s2.ID), map[string]int{fmt.Sprint(steamA): 3, fmt.Sprint(steamB): 2}},
		{fmt.Sprintf("player=%d", steamA), map[string]int{fmt.Sprint(steamA): 3}},
		{fmt.Sprintf("player=%d&player=%d&together=1", steamA, steamB), map[string]int{fmt.Sprint(steamA): 2, fmt.Sprint(steamB): 2}},
		{"minMatches=3", map[string]int{fmt.Sprint(steamA): 3}},
		{"from=2027-01-01", map[string]int{}},
	}
	for _, c := range cases {
		code, resp := get(c.query)
		got := map[string]int{}
		for _, p := range resp.Players {
			got[p.SteamID] = p.Matches
		}
		if code != 200 || fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: %d %v, ожидалось %v", c.query, code, got, c.want)
		}
	}
	if _, resp := get("from=2027-01-01"); resp.Players == nil {
		t.Error("пустая выборка должна быть пустым массивом, а не null")
	}
}

func TestListPlayersValidation(t *testing.T) {
	e := newTestEnv(t)
	for _, q := range []string{
		"from=08.10.2026",
		"to=2026-13-01",
		"session=abc",
		"session=0",
		"player=abc",
		"player=-1",
		"minMatches=0",
		"minMatches=x",
		"together=yes",
		"from=2026-10-01&session=1",
	} {
		var resp map[string]string
		if code := e.do(t, "GET", "/api/players?"+q, "", nil, &resp); code != 400 || resp["error"] == "" {
			t.Errorf("%s: %d %v", q, code, resp)
		}
	}
}

func TestGetPlayer(t *testing.T) {
	e := newTestEnv(t)
	s1, s2 := seedPlayers(t, e)
	path := fmt.Sprintf("/api/players/%d", steamA)

	var p profileResponse
	if code := e.do(t, "GET", path, "", nil, &p); code != 200 {
		t.Fatalf("профиль: %d", code)
	}
	if p.SteamID != fmt.Sprint(steamA) || p.Name != "p1_3" || p.Totals.Matches != 3 {
		t.Errorf("профиль: %+v", p)
	}
	if len(p.Sessions) != 2 || p.Sessions[0].Session.ID != s2.ID || p.Sessions[0].Matches != 2 || p.Sessions[0].Session.Title != "четверг" {
		t.Errorf("по сессиям: %+v", p.Sessions)
	}
	if len(p.Maps) != 1 || p.Maps[0].Map != "de_mirage" || p.Maps[0].Matches != 3 {
		t.Errorf("по картам: %+v", p.Maps)
	}
	if len(p.Matches) != 3 || p.Matches[0].SessionID != s2.ID || p.Matches[0].Ordinal != 2 ||
		p.Matches[0].Result != stats.Win || p.Matches[0].ScoreA != 13 || p.Matches[2].SessionID != s1.ID {
		t.Errorf("матчи: %+v", p.Matches)
	}

	// итоги профиля совпадают со строкой общей таблицы за тот же период
	period := url.Values{"session": {fmt.Sprint(s2.ID)}}.Encode()
	var list playersResponse
	e.do(t, "GET", "/api/players?"+period+fmt.Sprintf("&player=%d", steamA), "", nil, &list)
	e.do(t, "GET", path+"?"+period, "", nil, &p)
	if len(list.Players) != 1 || fmt.Sprint(list.Players[0].Counters) != fmt.Sprint(p.Totals.Counters) ||
		list.Players[0].Rating != p.Totals.Rating || p.Totals.Matches != 2 {
		t.Errorf("итоги профиля %+v != строка таблицы %+v", p.Totals, list.Players)
	}

	// пустой период: ник без учёта периода, нулевые итоги, пустые массивы
	var raw map[string]any
	if code := e.do(t, "GET", path+"?from=2027-01-01", "", nil, &raw); code != 200 {
		t.Fatalf("пустой период: %d", code)
	}
	totals := raw["totals"].(map[string]any)
	if raw["name"] != "p1_3" || totals["kills"] != 0.0 || totals["rating"] != 0.0 {
		t.Errorf("пустой период: %v", raw)
	}
	for _, k := range []string{"sessions", "maps", "matches"} {
		if arr, ok := raw[k].([]any); !ok || len(arr) != 0 {
			t.Errorf("%s: %v, ожидался пустой массив", k, raw[k])
		}
	}

	var errResp map[string]string
	if code := e.do(t, "GET", "/api/players/76561198000000099", "", nil, &errResp); code != 404 {
		t.Errorf("неизвестный игрок: %d", code)
	}
	if code := e.do(t, "GET", "/api/players/abc", "", nil, &errResp); code != 400 {
		t.Errorf("нечисловой SteamID: %d", code)
	}
	if code := e.do(t, "GET", path+"?from=x", "", nil, &errResp); code != 400 {
		t.Errorf("неверная дата в профиле: %d", code)
	}
}
