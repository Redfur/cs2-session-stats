package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"cs2stats/internal/store"
)

func TestGroupsAPI(t *testing.T) {
	e := newTestEnv(t)
	var list []store.Group
	if code := e.do(t, "GET", "/api/groups", "", nil, &list); code != 200 || len(list) != 1 || list[0].Name != "Основная" || list[0].SessionCount != 0 {
		t.Fatalf("список новой базы: %d %+v", code, list)
	}

	var g store.Group
	if code := e.do(t, "POST", "/api/groups", "application/json", []byte(`{"name":"  Четверг "}`), &g); code != 201 || g.Name != "Четверг" || g.ID == "" {
		t.Fatalf("создание: %d %+v", code, g)
	}
	var errResp map[string]string
	for _, body := range []string{`{"name":"   "}`, `{}`, `{"name":"` + strings.Repeat("я", 101) + `"}`, `nope`} {
		if code := e.do(t, "POST", "/api/groups", "application/json", []byte(body), &errResp); code != 400 || errResp["error"] == "" {
			t.Errorf("%s: %d %v", body, code, errResp)
		}
	}
	var long store.Group
	if code := e.do(t, "POST", "/api/groups", "application/json", []byte(`{"name":"`+strings.Repeat("я", 100)+`"}`), &long); code != 201 {
		t.Errorf("название из 100 символов: %d", code)
	}

	path := fmt.Sprintf("/api/groups/%s", g.ID)
	if code := e.do(t, "PATCH", path, "application/json", []byte(`{"name":"Пятница"}`), &g); code != 200 || g.Name != "Пятница" {
		t.Errorf("переименование: %d %+v", code, g)
	}
	if code := e.do(t, "PATCH", path, "application/json", []byte(`{"name":""}`), &errResp); code != 400 {
		t.Errorf("переименование в пустое: %d", code)
	}

	// сводка: сессия с матчем с результатом
	ctx := context.Background()
	sess, _ := e.store.CreateSession(ctx, g.ID, "2026-10-08", "")
	addRatedMatch(t, e, sess, ratingPlayer{steamA, "A", 1}, ratingPlayer{steamB, "B", 1})
	var sum store.GroupSummary
	if code := e.do(t, "GET", path, "", nil, &sum); code != 200 || sum.Name != "Пятница" || sum.SessionCount != 1 || sum.MatchCount != 1 || sum.PlayerCount != 2 {
		t.Errorf("сводка: %d %+v", code, sum)
	}

	if code := e.do(t, "DELETE", path, "", nil, &errResp); code != 409 || errResp["error"] != "в группе есть сессии — сначала удалите их" {
		t.Errorf("удаление группы с сессиями: %d %v", code, errResp)
	}
	if code := e.do(t, "DELETE", fmt.Sprintf("/api/groups/%s", long.ID), "", nil, nil); code != 204 {
		t.Errorf("удаление пустой: %d", code)
	}

	for _, req := range [][2]string{
		{"GET", fmt.Sprintf("/api/groups/%s", long.ID)}, {"GET", "/api/groups/abc"},
		{"PATCH", "/api/groups/999"}, {"DELETE", "/api/groups/999"},
		{"GET", "/api/groups/999/sessions"}, {"POST", "/api/groups/999/sessions"},
		{"GET", "/api/groups/999/players"}, {"GET", fmt.Sprintf("/api/groups/999/players/%d", steamA)},
		{"GET", fmt.Sprintf("/api/groups/999/players/%d/fight", steamA)},
	} {
		if code := e.do(t, req[0], req[1], "application/json", []byte(`{"name":"x"}`), &errResp); code != 404 || errResp["error"] != "группа не найдена" {
			t.Errorf("%s %s: %d %v", req[0], req[1], code, errResp)
		}
	}
}

func TestSessionsByGroupAPI(t *testing.T) {
	e := newTestEnv(t)
	g, _ := e.store.CreateGroup(context.Background(), "Четверг")
	var sess store.Session
	if code := e.do(t, "POST", fmt.Sprintf("/api/groups/%s/sessions", g.ID), "application/json", []byte(`{"date":"2026-10-08"}`), &sess); code != 201 || sess.GroupID != g.ID {
		t.Fatalf("создание в группе: %d %+v", code, sess)
	}
	var list []sessionSummaryView
	if e.do(t, "GET", "/api/groups/"+e.gid+"/sessions", "", nil, &list); len(list) != 0 {
		t.Errorf("сессии «Основной»: %+v", list)
	}
	if e.do(t, "GET", fmt.Sprintf("/api/groups/%s/sessions", g.ID), "", nil, &list); len(list) != 1 || list[0].ID != sess.ID || list[0].GroupID != g.ID {
		t.Errorf("сессии группы: %+v", list)
	}
	var sr sessionResponse
	if e.do(t, "GET", fmt.Sprintf("/api/sessions/%s", sess.ID), "", nil, &sr); sr.Session.GroupID != g.ID {
		t.Errorf("groupId сессии: %+v", sr.Session)
	}
}

// Одни и те же игроки сыграли матч в «Основной», затем матч в группе «Четверг».
func TestRatingByGroupAPI(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	g, _ := e.store.CreateGroup(ctx, "Четверг")
	main, _ := e.store.CreateSession(ctx, e.gid, "2026-10-01", "")
	thu, _ := e.store.CreateSession(ctx, g.ID, "2026-10-08", "")
	ps := []ratingPlayer{{steamA, "A", 1.2}, {3, "A", 1}, {steamB, "B", 0.9}, {4, "B", 1}}
	addRatedMatch(t, e, main, ps...)
	addRatedMatch(t, e, main, ps...)
	id := addRatedMatch(t, e, thu, ps...)

	// перед матчем «Четверга» у игроков нет OVR в этой группе
	var m matchResponse
	e.do(t, http.MethodGet, fmt.Sprintf("/api/matches/%s", id), "", nil, &m)
	if m.GroupID != g.ID || m.Teams.A != nil || m.Teams.B != nil || byID(m.Players, steamA).Before != nil {
		t.Errorf("матч группы «Четверг»: groupId %s, команды %+v", m.GroupID, m.Teams)
	}

	var main1, thu1 playersResponse
	e.do(t, http.MethodGet, "/api/groups/"+e.gid+"/players", "", nil, &main1)
	e.do(t, http.MethodGet, fmt.Sprintf("/api/groups/%s/players", g.ID), "", nil, &thu1)
	a1, a2 := byID(main1.Players, steamA), byID(thu1.Players, steamA)
	if a1.Matches != 2 || a2.Matches != 1 || a1.OVR == nil || a2.OVR == nil || a1.OVR.Matches != 2 || a2.OVR.Matches != 1 {
		t.Errorf("игрок в двух группах: %+v / %+v", a1, a2)
	}

	// игрок без матчей в группе — 404, даже если матчи есть в другой
	other, _ := e.store.CreateGroup(ctx, "Пусто")
	var errResp map[string]string
	for _, tab := range []string{"", "/duels", "/fight", "/weapons", "/utility"} {
		path := fmt.Sprintf("/api/groups/%s/players/%d%s", other.ID, steamA, tab)
		if code := e.do(t, http.MethodGet, path, "", nil, &errResp); code != 404 || errResp["error"] != "игрок не найден" {
			t.Errorf("%s: %d %v", path, code, errResp)
		}
	}
	var prof profileResponse
	if code := e.do(t, http.MethodGet, fmt.Sprintf("/api/groups/%s/players/%d", g.ID, steamA), "", nil, &prof); code != 200 ||
		len(prof.Matches) != 1 || prof.OVR == nil || prof.OVR.Matches != 1 {
		t.Errorf("профиль в «Четверге»: %d %+v", code, prof.OVR)
	}
}
