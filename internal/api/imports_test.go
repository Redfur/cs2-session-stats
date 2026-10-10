package api

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"

	"cs2stats/internal/importer"
	"cs2stats/internal/store"
)

// fakeImports — сервис импорта для тестов API: возвращает заданный итог и считает вызовы.
type fakeImports struct {
	mu       sync.Mutex
	results  []importer.AddResult
	err      error
	text     string
	wakes    int
	canceled []int64
	sessions []int64
}

func (f *fakeImports) Add(_ context.Context, _ int64, text string) ([]importer.AddResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = text
	return f.results, f.err
}
func (f *fakeImports) Wake() { f.mu.Lock(); f.wakes++; f.mu.Unlock() }
func (f *fakeImports) Cancel(id int64) {
	f.mu.Lock()
	f.canceled = append(f.canceled, id)
	f.mu.Unlock()
}
func (f *fakeImports) CancelSession(id int64) {
	f.mu.Lock()
	f.sessions = append(f.sessions, id)
	f.mu.Unlock()
}

func TestAddImports(t *testing.T) {
	e := newTestEnv(t)
	sess, _ := e.store.CreateSession(context.Background(), "2026-10-09", "")
	path := "/api/sessions/" + strconv.FormatInt(sess.ID, 10) + "/imports"

	e.imports.results = []importer.AddResult{
		{URL: "https://cs2.fastcup.net/matches/1", Status: importer.AddAccepted, ImportID: 1},
		{URL: "https://example.com", Status: importer.AddError, Error: "Поддерживаются ссылки на матчи"},
	}
	var res []importer.AddResult
	if code := e.do(t, "POST", path, "application/json", []byte(`{"text":"a\nb"}`), &res); code != 200 || len(res) != 2 || res[1].Error == "" {
		t.Fatalf("добавление: %d %+v", code, res)
	}
	if e.imports.text != "a\nb" || e.imports.wakes != 1 {
		t.Fatalf("ввод %q, wake %d", e.imports.text, e.imports.wakes)
	}

	// без принятых ссылок скачивание не будим
	e.imports.results = []importer.AddResult{{Status: importer.AddExists, MatchID: 5, Ordinal: 2, SessionID: sess.ID}}
	e.do(t, "POST", path, "application/json", []byte(`{"text":"a"}`), &res)
	if e.imports.wakes != 1 || res[0].Ordinal != 2 {
		t.Fatalf("уже есть: wake %d, %+v", e.imports.wakes, res)
	}

	var errResp map[string]string
	for _, c := range []struct {
		err  error
		want string
	}{{importer.ErrEmptyInput, importer.ErrEmptyInput.Error()}, {importer.ErrTooMany, importer.ErrTooMany.Error()}} {
		e.imports.err = c.err
		if code := e.do(t, "POST", path, "application/json", []byte(`{"text":""}`), &errResp); code != 400 || errResp["error"] != c.want {
			t.Errorf("%v: %d %v", c.err, code, errResp)
		}
	}
	e.imports.err = nil
	if code := e.do(t, "POST", path, "application/json", []byte(`не json`), &errResp); code != 400 {
		t.Errorf("не JSON: %d", code)
	}
	if code := e.do(t, "POST", "/api/sessions/999/imports", "application/json", []byte(`{"text":"a"}`), &errResp); code != 404 {
		t.Errorf("нет сессии: %d", code)
	}
}

func TestImportRetryDeleteAndSessionView(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	sess, _ := e.store.CreateSession(ctx, "2026-10-09", "")
	queued, _ := e.store.AddImport(ctx, sess.ID, "https://cs2.fastcup.net/matches/1", "fastcup", "1")
	failed, _ := e.store.AddImport(ctx, sess.ID, "https://cs2.fastcup.net/matches/2", "fastcup", "2")
	done, _ := e.store.AddImport(ctx, sess.ID, "https://cs2.fastcup.net/matches/3", "fastcup", "3")
	e.store.FinishImport(ctx, failed.ID, "Сайт с демкой не ответил. Файл не скачан.", nil)
	e.store.FinishImport(ctx, done.ID, "", json.RawMessage(`[{"fileName":"a.dem","status":"accepted","matchId":1}]`))
	id := func(x store.Import) string { return strconv.FormatInt(x.ID, 10) }

	// в ответе сессии — загрузки в работе и с ошибкой; завершённая без пометок скрыта
	var view struct {
		Imports []store.Import `json:"imports"`
	}
	e.do(t, "GET", "/api/sessions/"+strconv.FormatInt(sess.ID, 10), "", nil, &view)
	if len(view.Imports) != 2 || view.Imports[0].ID != queued.ID || view.Imports[1].Error == "" {
		t.Fatalf("загрузки сессии: %+v", view.Imports)
	}

	var x store.Import
	var errResp map[string]string
	if code := e.do(t, "POST", "/api/imports/"+id(queued)+"/retry", "", nil, &errResp); code != 409 {
		t.Errorf("повтор не упавшей: %d", code)
	}
	if code := e.do(t, "POST", "/api/imports/"+id(failed)+"/retry", "", nil, &x); code != 202 || x.Status != store.ImportQueued || e.imports.wakes != 1 {
		t.Errorf("повтор: %d %+v wake=%d", code, x, e.imports.wakes)
	}
	if code := e.do(t, "POST", "/api/imports/999/retry", "", nil, &errResp); code != 404 {
		t.Errorf("повтор неизвестной: %d", code)
	}

	if code := e.do(t, "DELETE", "/api/imports/"+id(queued), "", nil, nil); code != 204 || len(e.imports.canceled) != 1 || e.imports.canceled[0] != queued.ID {
		t.Errorf("удаление: %d %v", code, e.imports.canceled)
	}
	if code := e.do(t, "DELETE", "/api/imports/"+id(queued), "", nil, &errResp); code != 404 {
		t.Errorf("повторное удаление: %d", code)
	}

	if code := e.do(t, "DELETE", "/api/sessions/"+strconv.FormatInt(sess.ID, 10), "", nil, nil); code != 204 ||
		len(e.imports.sessions) != 1 || e.imports.sessions[0] != sess.ID {
		t.Errorf("удаление сессии: %d %v", code, e.imports.sessions)
	}
	if _, err := e.store.GetImport(ctx, failed.ID); err != store.ErrNotFound {
		t.Errorf("загрузки сессии не удалены: %v", err)
	}
}
