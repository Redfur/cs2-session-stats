package importer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// platformServer отвечает JSON-ом и запоминает тело последнего запроса.
func platformServer(t *testing.T, status int, body string) (*httptest.Server, *[]byte) {
	t.Helper()
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func fastcup(endpoint string, now time.Time) *FastCup {
	return &FastCup{Endpoint: endpoint, Now: func() time.Time { return now }}
}

var beforeExpiry = time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)

// Текст запроса не должен меняться: allowlist Hasura сверяет его дословно.
func TestFastCupQueryIsVerbatim(t *testing.T) {
	if !strings.HasPrefix(fastcupGetMatch, "query __GetMatch($matchId: Int!, $gameId: smallint!) {") ||
		strings.HasSuffix(fastcupGetMatch, "\n") || strings.Contains(fastcupGetMatch, "\r") {
		t.Fatal("fastcup_get_match.graphql изменён: нужна дословная копия запроса со страницы матча (без перевода строки в конце)")
	}
	srv, got := platformServer(t, 200, fixture(t, "fastcup_27802385.json"))
	if _, err := fastcup(srv.URL, beforeExpiry).Resolve(context.Background(), srv.Client(), "27802385"); err != nil {
		t.Fatal(err)
	}
	var req struct {
		OperationName string         `json:"operationName"`
		Variables     map[string]int `json:"variables"`
		Query         string         `json:"query"`
	}
	json.Unmarshal(*got, &req)
	if req.OperationName != "__GetMatch" || req.Query != fastcupGetMatch || req.Variables["matchId"] != 27802385 || req.Variables["gameId"] != 3 {
		t.Fatalf("запрос: %+v", req)
	}
}

func TestFastCupBo1(t *testing.T) {
	srv, _ := platformServer(t, 200, fixture(t, "fastcup_27802385.json"))
	m, err := fastcup(srv.URL, beforeExpiry).Resolve(context.Background(), srv.Client(), "27802385")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 7, 16, 17, 57, 278691000, time.UTC)
	if m.Number != 27802385 || !m.StartedAt.Equal(start) || len(m.Demos) != 1 {
		t.Fatalf("%+v", m)
	}
	d := m.Demos[0]
	if d.URL != "https://replays.fastcup.net/16324690/27802385_24844407_2610071617-de_ancient.dem" ||
		d.Name != "27802385_24844407_2610071617-de_ancient.dem" || len(d.Maps) != 1 || d.Maps[0].Number != 1 || !d.Maps[0].StartedAt.Equal(start) {
		t.Fatalf("демка: %+v", d)
	}
}

func TestFastCupBo3(t *testing.T) {
	// карты в перепутанном порядке; у карты 2 два replay — берётся последний; карта 3 без replay пропускается
	body := `{"data":{"match":{"status":"FINISHED","startedAt":"2026-10-08T19:00:00+00:00","maps":[
	 {"number":2,"startedAt":"2026-10-08T19:50:00+00:00","replays":[
	   {"url":"https://replays.fastcup.net/1/old.dem","createdAt":"2026-10-08T20:30:00+00:00"},
	   {"url":"https://replays.fastcup.net/1/m2.dem","createdAt":"2026-10-08T20:40:00+00:00"}]},
	 {"number":3,"startedAt":"2026-10-08T20:45:00+00:00","replays":[]},
	 {"number":1,"startedAt":null,"replays":[{"url":"https://replays.fastcup.net/1/m1.dem","createdAt":"2026-10-08T19:45:00+00:00"}]}
	]}}}`
	srv, _ := platformServer(t, 200, body)
	m, err := fastcup(srv.URL, beforeExpiry).Resolve(context.Background(), srv.Client(), "5")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Demos) != 2 || m.Demos[0].Name != "m1.dem" || m.Demos[1].Name != "m2.dem" {
		t.Fatalf("демки: %+v", m.Demos)
	}
	// у карты 1 нет своего времени — берётся время матча
	if !m.Demos[0].Maps[0].StartedAt.Equal(time.Date(2026, 10, 8, 19, 0, 0, 0, time.UTC)) || m.Demos[1].Maps[0].Number != 2 ||
		!m.Demos[1].Maps[0].StartedAt.Equal(time.Date(2026, 10, 8, 19, 50, 0, 0, time.UTC)) || !m.StartedAt.Equal(m.Demos[0].Maps[0].StartedAt) {
		t.Fatalf("время карт: %+v", m.Demos)
	}
}

func TestFastCupErrors(t *testing.T) {
	noReplays := `{"data":{"match":{"status":"FINISHED","replayExpirationDate":"2026-11-06T16:12:10+00:00","maps":[{"number":1,"replays":[]}]}}}`
	cases := []struct {
		name   string
		status int
		body   string
		now    time.Time
		want   string
	}{
		{"не найден", 200, `{"data":{"match":null}}`, beforeExpiry, "Матч не найден на FastCup"},
		{"не закончен", 200, `{"data":{"match":{"status":"LIVE","maps":[]}}}`, beforeExpiry, "Матч ещё не закончен"},
		{"демку ещё не выложили", 200, noReplays, beforeExpiry, "Демку ещё не выложили"},
		{"срок хранения истёк", 200, noReplays, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), "FastCup удалил демку"},
		{"query is not allowed", 200, `{"errors":[{"message":"query is not allowed"}]}`, beforeExpiry, "FastCup изменил формат ответа"},
		{"не JSON", 200, `<html>`, beforeExpiry, "FastCup изменил формат ответа"},
		{"ошибка сервера", 502, `bad gateway`, beforeExpiry, "Сайт FastCup ответил ошибкой 502"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := platformServer(t, c.status, c.body)
			_, err := fastcup(srv.URL, c.now).Resolve(context.Background(), srv.Client(), "1")
			if msg := UserMessage(err, ""); !strings.HasPrefix(msg, c.want) {
				t.Fatalf("ошибка %v, ожидалось начало %q", err, c.want)
			}
		})
	}
}

func TestPlatformTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // висим, пока клиент не бросит запрос, но не дольше секунды: иначе Close ждёт вечно
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// таймаут самого запроса (а не отмена загрузки) показывается как «не ответил»
	client := &http.Client{Timeout: 20 * time.Millisecond}
	_, err := fastcup(srv.URL, beforeExpiry).Resolve(ctx, client, "1")
	if msg := UserMessage(err, ""); !strings.HasPrefix(msg, "Сайт FastCup не ответил") {
		t.Fatalf("таймаут: %v", err)
	}
	// отмена загрузки возвращается как отмена
	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()
	if _, err := fastcup(srv.URL, beforeExpiry).Resolve(cctx, srv.Client(), "1"); err != context.Canceled {
		t.Fatalf("отмена: %v", err)
	}
}

func TestCybershoke(t *testing.T) {
	cs := func(url string) *Cybershoke { return &Cybershoke{Endpoint: url} }

	srv, got := platformServer(t, 200, fixture(t, "cybershoke_12552678.json"))
	m, err := cs(srv.URL).Resolve(context.Background(), srv.Client(), "12552678")
	if err != nil {
		t.Fatal(err)
	}
	if string(*got) != `{"id_lobby":12552678}` {
		t.Fatalf("запрос: %s", *got)
	}
	if m.Number != 12552678 || len(m.Demos) != 1 || m.Demos[0].URL != "https://cdn-de-1.cybershoke.net/demos/12552678?series=1" ||
		m.Demos[0].Name != "match_12552678.zip" || len(m.Demos[0].Maps) != 1 || m.Demos[0].Maps[0].Number != 1 {
		t.Fatalf("Bo1: %+v", m)
	}

	srv, _ = platformServer(t, 200, fixture(t, "cybershoke_12578063.json"))
	m, err = cs(srv.URL).Resolve(context.Background(), srv.Client(), "12578063")
	if err != nil {
		t.Fatal(err)
	}
	maps := m.Demos[0].Maps
	if len(maps) != 2 || maps[0].Number != 1 || maps[1].Number != 2 ||
		!maps[0].StartedAt.Equal(time.Unix(1791581424, 0)) || !maps[1].StartedAt.Equal(time.Unix(1791583791, 0)) ||
		!m.StartedAt.Equal(maps[0].StartedAt) {
		t.Fatalf("Bo3: %+v", maps)
	}

	cases := []struct{ name, body, want string }{
		{"не найден", `{"result":"error","code":100}`, "Матч не найден на Cybershoke"},
		{"не закончен", `{"result":"success","data":{"dates":{"unixtime_match_end":null},"demo":{"status":0}}}`, "Матч ещё не закончен"},
		{"не готова", `{"result":"success","data":{"match_stats":{"bo":[]},"dates":{"unixtime_match_end":1791554015},"demo":{"status":1,"url_download":""}}}`, "Демку ещё не выложили"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := platformServer(t, 200, c.body)
			_, err := cs(srv.URL).Resolve(context.Background(), srv.Client(), "1")
			if msg := UserMessage(err, ""); !strings.HasPrefix(msg, c.want) {
				t.Fatalf("ошибка %v, ожидалось %q", err, c.want)
			}
		})
	}

	// карт в статистике нет ([] от PHP) — одна карта со временем начала матча
	srv, _ = platformServer(t, 200, `{"result":"success","data":{"match_stats":[],"dates":{"unixtime_start_match":100,"unixtime_match_end":200},"demo":{"status":4,"url_download":"https://cdn-de-1.cybershoke.net/demos/1?series=1"}}}`)
	if m, err := cs(srv.URL).Resolve(context.Background(), srv.Client(), "1"); err == nil {
		t.Fatalf("match_stats: [] должен разбираться как пустой, а не ломать ответ: %+v", m)
	}
}
