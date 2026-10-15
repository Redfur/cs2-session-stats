package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"cs2stats/internal/ids"
	"cs2stats/internal/store"
)

// Id неверного формата (числовой, с префиксом другого типа) отвечает так же, как отсутствующий
// объект верного формата: 404 и тот же текст.
func TestInvalidIDsAreNotFound(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	sess, _ := e.store.CreateSession(ctx, e.gid, "2026-10-08", "")
	m, _ := e.store.AddMatch(ctx, sess.ID, "a", "a.dem")
	imp, _ := e.store.AddImport(ctx, sess.ID, "https://cs2.fastcup.net/matches/1", "fastcup", "1")

	type request struct{ method, path, body string }
	cases := []struct {
		kind     ids.Kind
		foreign  string // существующий id другого типа
		notFound string
		requests func(id string) []request
	}{
		{ids.Group, sess.ID, "группа не найдена", func(id string) []request {
			return []request{
				{"GET", "/api/groups/" + id, ""},
				{"PATCH", "/api/groups/" + id, `{"name":"x"}`},
				{"DELETE", "/api/groups/" + id, ""},
				{"GET", "/api/groups/" + id + "/sessions", ""},
				{"POST", "/api/groups/" + id + "/sessions", `{}`},
				{"GET", "/api/groups/" + id + "/players", ""},
				{"GET", "/api/groups/" + id + "/players/1", ""},
				{"GET", "/api/groups/" + id + "/players/1/fight", ""},
			}
		}},
		{ids.Session, m.ID, "сессия не найдена", func(id string) []request {
			return []request{
				{"GET", "/api/sessions/" + id, ""},
				{"PATCH", "/api/sessions/" + id, `{"date":"2026-10-08"}`},
				{"DELETE", "/api/sessions/" + id, ""},
				{"PUT", "/api/sessions/" + id + "/order", `{"matchIds":[]}`},
				{"POST", "/api/sessions/" + id + "/demos", ""},
				{"POST", "/api/sessions/" + id + "/imports", `{"text":"x"}`},
				{"POST", "/api/sessions/" + id + "/reparse", ""},
				{"GET", "/api/sessions/" + id + "/duels", ""},
				{"GET", "/api/sessions/" + id + "/ext", ""},
			}
		}},
		{ids.Match, sess.ID, "матч не найден", func(id string) []request {
			return []request{
				{"GET", "/api/matches/" + id, ""},
				{"DELETE", "/api/matches/" + id, ""},
				{"POST", "/api/matches/" + id + "/reparse", ""},
				{"GET", "/api/matches/" + id + "/duels", ""},
				{"GET", "/api/matches/" + id + "/ext", ""},
			}
		}},
		{ids.Import, m.ID, "загрузка не найдена", func(id string) []request {
			return []request{
				{"POST", "/api/imports/" + id + "/retry", ""},
				{"DELETE", "/api/imports/" + id, ""},
			}
		}},
	}
	missing := map[ids.Kind]string{}
	for _, c := range cases {
		missing[c.kind] = string(c.kind) + "000000000000"
	}
	for _, c := range cases {
		for _, id := range []string{missing[c.kind], "3", "1", c.foreign, strings.ToUpper(missing[c.kind])} {
			for _, r := range c.requests(id) {
				var resp map[string]string
				code := e.do(t, r.method, r.path, "application/json", []byte(r.body), &resp)
				if code != http.StatusNotFound || resp["error"] != c.notFound {
					t.Errorf("%s %s: %d %q, ожидалось 404 %q", r.method, r.path, code, resp["error"], c.notFound)
				}
			}
		}
	}
	// неверный id сессии в фильтре периода — ошибка запроса
	var resp map[string]string
	if code := e.do(t, "GET", "/api/groups/"+e.gid+"/players?session=3", "", nil, &resp); code != 400 {
		t.Errorf("числовой id в фильтре: %d %v", code, resp)
	}
	// объекты на месте: запросы с чужими id ничего не задели
	if _, err := e.store.GetMatch(ctx, m.ID); err != nil {
		t.Errorf("матч: %v", err)
	}
	if _, err := e.store.GetImport(ctx, imp.ID); err != nil {
		t.Errorf("загрузка: %v", err)
	}
}

// guardedIDKey — ключ JSON, который ссылается на группу, сессию, матч или загрузку.
func guardedIDKey(key string) bool {
	switch key {
	case "steamId", "killerId", "victimId", "assisterId", "externalId":
		return false
	}
	return key == "id" || strings.HasSuffix(key, "Id") || strings.HasSuffix(key, "Ids")
}

// numericIDs обходит JSON и возвращает пути ключей id, у которых значение — число.
func numericIDs(path string, v any, checked *int) []string {
	var bad []string
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			p := path + "." + k
			if guardedIDKey(k) {
				*checked++
				switch id := val.(type) {
				case float64:
					bad = append(bad, p)
				case []any:
					for i, item := range id {
						if _, ok := item.(float64); ok {
							bad = append(bad, fmt.Sprintf("%s[%d]", p, i))
						}
					}
				}
			}
			bad = append(bad, numericIDs(p, val, checked)...)
		}
	case []any:
		for i, item := range x {
			bad = append(bad, numericIDs(fmt.Sprintf("%s[%d]", path, i), item, checked)...)
		}
	}
	return bad
}

// Ни один ответ основных GET-эндпоинтов не отдаёт числовой id группы, сессии, матча или загрузки.
// Новый эндпоинт — повод дописать его сюда.
func TestNoNumericIDsInResponses(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	s1, _ := e.store.CreateSession(ctx, e.gid, "2026-10-01", "")
	s2, _ := e.store.CreateSession(ctx, e.gid, "2026-10-08", "")
	newID := saveParsed(t, e, s1, "new", extMatch(), store.ExtendedSinceVersion)
	oldID := saveParsed(t, e, s2, "old", extMatch(), store.DuelsSinceVersion-1)
	if err := e.store.FailMatch(ctx, oldID, "битая демка", store.ExtendedSinceVersion); err != nil {
		t.Fatal(err)
	}
	// упавшая загрузка видна на странице сессии
	imp, _ := e.store.AddImport(ctx, s2.ID, "https://cs2.fastcup.net/matches/1", "fastcup", "1")
	e.store.ClaimNextImport(ctx)
	results := json.RawMessage(fmt.Sprintf(`[{"fileName":"a.dem","status":"duplicate","matchId":%q,"sessionId":%q}]`, newID, s1.ID))
	if err := e.store.FinishImport(ctx, imp.ID, "ошибка", results); err != nil {
		t.Fatal(err)
	}

	player := fmt.Sprint(extA1)
	group := "/api/groups/" + e.gid
	paths := []string{
		"/api/groups", group, group + "/sessions",
		"/api/sessions/" + s1.ID, "/api/sessions/" + s2.ID,
		"/api/sessions/" + s1.ID + "/ext", "/api/sessions/" + s2.ID + "/ext",
		"/api/sessions/" + s1.ID + "/duels", "/api/sessions/" + s2.ID + "/duels",
		"/api/matches/" + newID, "/api/matches/" + oldID,
		"/api/matches/" + newID + "/ext", "/api/matches/" + newID + "/duels", "/api/matches/" + oldID + "/duels",
		group + "/players", group + "/players?session=" + s2.ID,
	}
	for _, tab := range []string{"", "/duels", "/fight", "/weapons", "/utility"} {
		paths = append(paths, group+"/players/"+player+tab, group+"/players/"+player+tab+"?session="+s2.ID)
	}

	checked := 0
	for _, path := range paths {
		var body any
		if code := e.do(t, "GET", path, "", nil, &body); code != http.StatusOK {
			t.Fatalf("GET %s: %d", path, code)
		}
		for _, p := range numericIDs("", body, &checked) {
			t.Errorf("GET %s: числовой id в %s", path, p)
		}
	}

	// ответ на загрузку демки
	ct, upload := multipartBody(t, map[string][]byte{"x.dem": []byte("HL2DEMO fake")})
	var uploaded any
	e.do(t, "POST", "/api/sessions/"+s1.ID+"/demos", ct, upload, &uploaded)
	for _, p := range numericIDs("", uploaded, &checked) {
		t.Errorf("загрузка демки: числовой id в %s", p)
	}
	if checked < 50 {
		t.Fatalf("проверено ключей id: %d — обход не дошёл до данных", checked)
	}
}
