package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"cs2stats/internal/ingest"
	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

type testEnv struct {
	srv     *httptest.Server
	store   *store.Store
	demos   *ingest.LocalStorage
	woken   int
	imports *fakeImports
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	demos, _ := ingest.NewLocalStorage(filepath.Join(dir, "demos"))
	tmp := filepath.Join(dir, "tmp")
	os.MkdirAll(tmp, 0o755)

	e := &testEnv{store: st, demos: demos, imports: &fakeImports{}}
	s := &Server{
		Store:   st,
		Ingest:  &ingest.Service{Store: st, Demos: demos, TmpDir: tmp, MaxSize: 1 << 20},
		Wake:    func() { e.woken++ },
		Imports: e.imports,
		Log:     slog.New(slog.DiscardHandler),
		Now:     func() time.Time { return time.Date(2026, 10, 9, 20, 0, 0, 0, time.Local) },
	}
	e.srv = httptest.NewServer(s.Handler(nil))
	t.Cleanup(e.srv.Close)
	return e
}

func (e *testEnv) do(t *testing.T, method, path, contentType string, body []byte, out any) int {
	t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: ответ не JSON: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func TestSessions(t *testing.T) {
	e := newTestEnv(t)

	var sess store.Session
	if code := e.do(t, "POST", "/api/sessions", "application/json", nil, &sess); code != 201 {
		t.Fatalf("создание без параметров: %d", code)
	}
	if sess.Date != "2026-10-09" || sess.Title != "" {
		t.Errorf("сессия по умолчанию: %+v", sess)
	}

	var errResp map[string]string
	if code := e.do(t, "POST", "/api/sessions", "application/json", []byte(`{"date":"08.10.2026"}`), &errResp); code != 400 || errResp["error"] == "" {
		t.Errorf("неверная дата: %d %v", code, errResp)
	}

	e.do(t, "POST", "/api/sessions", "application/json", []byte(`{"date":"2026-10-10","title":" Микс "}`), &sess)
	if sess.Title != "Микс" {
		t.Errorf("название не обрезано: %q", sess.Title)
	}

	var list []store.SessionSummary
	if code := e.do(t, "GET", "/api/sessions", "", nil, &list); code != 200 || len(list) != 2 || list[0].Date != "2026-10-10" {
		t.Errorf("список: %d %+v", code, list)
	}

	for _, path := range []string{"/api/sessions/999", "/api/sessions/abc", "/api/matches/999", "/api/nope"} {
		if code := e.do(t, "GET", path, "", nil, &errResp); code != 404 {
			t.Errorf("GET %s: %d, ожидалось 404", path, code)
		}
	}
}

func multipartBody(t *testing.T, files map[string][]byte) (string, []byte) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("comment", "игнорируется")
	for name, data := range files {
		fw, _ := w.CreateFormFile("files", name)
		fw.Write(data)
	}
	w.Close()
	return w.FormDataContentType(), buf.Bytes()
}

func TestUploadAndViews(t *testing.T) {
	e := newTestEnv(t)
	var sess store.Session
	e.do(t, "POST", "/api/sessions", "application/json", []byte(`{}`), &sess)

	ct, body := multipartBody(t, map[string][]byte{"a.dem": []byte("HL2DEMO fake"), "notes.txt": []byte("x")})
	var errResp map[string]string
	if code := e.do(t, "POST", "/api/sessions/999/demos", ct, body, &errResp); code != 404 {
		t.Errorf("загрузка в неизвестную сессию: %d", code)
	}

	var results []ingest.FileResult
	if code := e.do(t, "POST", "/api/sessions/1/demos", ct, body, &results); code != 200 {
		t.Fatalf("загрузка: %d", code)
	}
	byName := map[string]ingest.FileResult{}
	for _, r := range results {
		byName[r.FileName] = r
	}
	if len(results) != 2 || byName["a.dem"].Status != ingest.Accepted || byName["notes.txt"].Status != ingest.Failed {
		t.Fatalf("частичный успех: %+v", results)
	}
	if e.woken != 1 {
		t.Errorf("воркер разбужен %d раз", e.woken)
	}

	if code := e.do(t, "POST", "/api/sessions/1/demos", "application/json", []byte(`{}`), &errResp); code != 400 {
		t.Errorf("не multipart: %d", code)
	}

	// имитируем обработку воркером
	matchID := byName["a.dem"].MatchID
	ctx := context.Background()
	e.store.ClaimNextPending(ctx)
	e.store.SaveMatchResult(ctx, matchID, store.MatchResult{Map: "de_nuke", Rounds: 20, ScoreA: 13, ScoreB: 7, Players: []stats.PlayerStats{
		{SteamID: 76561198000000001, Name: "alice", Team: "A", Result: stats.Win, Counters: stats.Counters{Rounds: 20, Kills: 20, Damage: 1600}},
		{SteamID: 76561198000000002, Name: "bob", Team: "B", Result: stats.Loss, Counters: stats.Counters{Rounds: 20, Deaths: 20}},
	}}, 1)

	var sr struct {
		Session store.Session `json:"session"`
		Matches []store.Match `json:"matches"`
		Players []PlayerView  `json:"players"`
	}
	if code := e.do(t, "GET", "/api/sessions/1", "", nil, &sr); code != 200 {
		t.Fatalf("сессия: %d", code)
	}
	if len(sr.Matches) != 1 || sr.Matches[0].Map != "de_nuke" || len(sr.Players) != 2 {
		t.Fatalf("сессия: %+v", sr)
	}
	if sr.Players[0].SteamID != "76561198000000001" || sr.Players[0].ADR != 80 || sr.Players[0].Wins != 1 || sr.Players[0].Matches != 1 {
		t.Errorf("сводная строка: %+v", sr.Players[0])
	}

	var mr struct {
		Match   store.Match  `json:"match"`
		Players []PlayerView `json:"players"`
	}
	if code := e.do(t, "GET", "/api/matches/"+strconv.FormatInt(matchID, 10), "", nil, &mr); code != 200 || mr.Match.ScoreA != 13 || len(mr.Players) != 2 || mr.Players[0].Team != "A" {
		t.Errorf("матч: %d %+v", code, mr)
	}
}
