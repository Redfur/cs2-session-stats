package api

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"testing"

	"cs2stats/internal/ingest"
	"cs2stats/internal/store"
)

func TestUpdateSession(t *testing.T) {
	e := newTestEnv(t)
	var sess store.Session
	e.do(t, "POST", "/api/sessions", "application/json", []byte(`{"date":"2026-10-08","title":"Четверг"}`), &sess)
	path := "/api/sessions/" + strconv.FormatInt(sess.ID, 10)

	var got store.Session
	if code := e.do(t, "PATCH", path, "application/json", []byte(`{"date":"2026-10-09","title":"  Пятничный микс "}`), &got); code != 200 {
		t.Fatalf("редактирование: %d", code)
	}
	if got.ID != sess.ID || got.Date != "2026-10-09" || got.Title != "Пятничный микс" {
		t.Fatalf("неверный ответ: %+v", got)
	}

	bad := map[string]string{
		"неверная дата":    `{"date":"09.10.2026","title":""}`,
		"пустая дата":      `{"title":"x"}`,
		"длинное название": `{"date":"2026-10-09","title":"` + strings.Repeat("я", 201) + `"}`,
		"не JSON":          `{`,
	}
	for name, body := range bad {
		var errResp map[string]string
		if code := e.do(t, "PATCH", path, "application/json", []byte(body), &errResp); code != 400 || errResp["error"] == "" {
			t.Errorf("%s: %d %v", name, code, errResp)
		}
	}
	after, _ := e.store.GetSession(t.Context(), sess.ID)
	if after.Date != "2026-10-09" || after.Title != "Пятничный микс" {
		t.Fatalf("отклонённый запрос изменил сессию: %+v", after)
	}

	var errResp map[string]string
	if code := e.do(t, "PATCH", "/api/sessions/999", "application/json", []byte(`{"date":"2026-10-09"}`), &errResp); code != 404 {
		t.Errorf("неизвестная сессия: %d", code)
	}
}

func TestDeleteMatchAndSession(t *testing.T) {
	e := newTestEnv(t)
	var sess store.Session
	e.do(t, "POST", "/api/sessions", "application/json", []byte(`{}`), &sess)
	sid := strconv.FormatInt(sess.ID, 10)

	demos := map[string][]byte{"a.dem": []byte("HL2DEMO a"), "b.dem": []byte("HL2DEMO b"), "c.dem": []byte("HL2DEMO c")}
	upload := func(name string) ingest.FileResult {
		t.Helper()
		ct, body := multipartBody(t, map[string][]byte{name: demos[name]})
		var res []ingest.FileResult
		if code := e.do(t, "POST", "/api/sessions/"+sid+"/demos", ct, body, &res); code != 200 || len(res) != 1 {
			t.Fatalf("загрузка %s: %d %+v", name, code, res)
		}
		return res[0]
	}
	fileExists := func(name string) bool {
		sum := sha256.Sum256(demos[name])
		_, err := os.Stat(e.demos.Dir + "/" + hex.EncodeToString(sum[:]) + ".dem")
		return err == nil
	}
	a, b, c := upload("a.dem"), upload("b.dem"), upload("c.dem")

	// удаление матча: 204, файл удалён, номера сдвинулись
	if code := e.do(t, "DELETE", "/api/matches/"+strconv.FormatInt(b.MatchID, 10), "", nil, nil); code != 204 {
		t.Fatalf("удаление матча: %d", code)
	}
	if fileExists("b.dem") || !fileExists("a.dem") {
		t.Fatal("удалён не тот файл демки")
	}
	var errResp map[string]string
	if code := e.do(t, "GET", "/api/matches/"+strconv.FormatInt(b.MatchID, 10), "", nil, &errResp); code != 404 {
		t.Errorf("удалённый матч доступен: %d", code)
	}
	if code := e.do(t, "DELETE", "/api/matches/"+strconv.FormatInt(b.MatchID, 10), "", nil, &errResp); code != 404 {
		t.Errorf("повторное удаление матча: %d", code)
	}
	cm, _ := e.store.GetMatch(t.Context(), c.MatchID)
	if cm.Ordinal != 2 {
		t.Errorf("номер матча C после удаления B: %d", cm.Ordinal)
	}

	// та же демка после удаления принимается как новая
	if again := upload("b.dem"); again.Status != ingest.Accepted {
		t.Fatalf("повторная загрузка после удаления матча: %+v", again)
	}

	// удаление сессии: 204, все файлы удалены, сессия и матчи недоступны
	if code := e.do(t, "DELETE", "/api/sessions/"+sid, "", nil, nil); code != 204 {
		t.Fatalf("удаление сессии: %d", code)
	}
	for name := range demos {
		if fileExists(name) {
			t.Errorf("файл %s не удалён вместе с сессией", name)
		}
	}
	if code := e.do(t, "GET", "/api/sessions/"+sid, "", nil, &errResp); code != 404 {
		t.Errorf("удалённая сессия доступна: %d", code)
	}
	if code := e.do(t, "GET", "/api/matches/"+strconv.FormatInt(a.MatchID, 10), "", nil, &errResp); code != 404 {
		t.Errorf("матч удалённой сессии доступен: %d", code)
	}
	if code := e.do(t, "DELETE", "/api/sessions/"+sid, "", nil, &errResp); code != 404 {
		t.Errorf("повторное удаление сессии: %d", code)
	}

	// демка удалённой сессии загружается в другую как новая
	var other store.Session
	e.do(t, "POST", "/api/sessions", "application/json", []byte(`{}`), &other)
	sid = strconv.FormatInt(other.ID, 10)
	if again := upload("a.dem"); again.Status != ingest.Accepted {
		t.Fatalf("повторная загрузка после удаления сессии: %+v", again)
	}
}

func TestReorderMatches(t *testing.T) {
	e := newTestEnv(t)
	ctx := t.Context()
	sess, _ := e.store.CreateSession(ctx, "2026-10-08", "")
	other, _ := e.store.CreateSession(ctx, "2026-10-09", "")
	var ids []string
	for _, sha := range []string{"A", "B", "C"} {
		m, _ := e.store.AddMatch(ctx, sess.ID, sha, sha+".dem")
		ids = append(ids, strconv.FormatInt(m.ID, 10))
	}
	foreign, _ := e.store.AddMatch(ctx, other.ID, "D", "D.dem")
	path := "/api/sessions/" + strconv.FormatInt(sess.ID, 10) + "/order"

	var list []store.Match
	body := `{"matchIds":[` + ids[0] + `,` + ids[2] + `,` + ids[1] + `]}`
	if code := e.do(t, "PUT", path, "application/json", []byte(body), &list); code != 200 {
		t.Fatalf("перестановка: %d", code)
	}
	if len(list) != 3 || list[0].SHA256 != "A" || list[1].SHA256 != "C" || list[2].SHA256 != "B" || list[1].Ordinal != 2 {
		t.Fatalf("неверный порядок: %+v", list)
	}

	bad := []string{
		`{"matchIds":[` + ids[0] + `,` + ids[1] + `]}`,
		`{"matchIds":[` + ids[0] + `,` + ids[1] + `,` + strconv.FormatInt(foreign.ID, 10) + `]}`,
		`{"matchIds":"x"}`,
		`{`,
	}
	for _, b := range bad {
		var errResp map[string]string
		if code := e.do(t, "PUT", path, "application/json", []byte(b), &errResp); code != 400 || errResp["error"] == "" {
			t.Errorf("%s: %d %v", b, code, errResp)
		}
	}
	var errResp map[string]string
	if code := e.do(t, "PUT", "/api/sessions/999/order", "application/json", []byte(`{"matchIds":[]}`), &errResp); code != 404 {
		t.Errorf("неизвестная сессия: %d", code)
	}
}

// Файл не удалось удалить: данные в БД уже удалены, поэтому ответ всё равно 204.
func TestDeleteMatchFileError(t *testing.T) {
	e := newTestEnv(t)
	var sess store.Session
	e.do(t, "POST", "/api/sessions", "application/json", []byte(`{}`), &sess)
	ct, body := multipartBody(t, map[string][]byte{"a.dem": []byte("HL2DEMO a")})
	var res []ingest.FileResult
	e.do(t, "POST", "/api/sessions/"+strconv.FormatInt(sess.ID, 10)+"/demos", ct, body, &res)

	if err := os.Chmod(e.demos.Dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(e.demos.Dir, 0o755) })
	if code := e.do(t, "DELETE", "/api/matches/"+strconv.FormatInt(res[0].MatchID, 10), "", nil, nil); code != 204 {
		t.Fatalf("удаление матча при ошибке файла: %d", code)
	}
	if _, err := e.store.GetMatch(t.Context(), res[0].MatchID); err == nil {
		t.Fatal("матч не удалён")
	}
	if entries, _ := os.ReadDir(e.demos.Dir); len(entries) != 1 {
		t.Fatalf("файл должен был остаться (права), найдено %d", len(entries))
	}
}
