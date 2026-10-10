package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cs2stats/internal/ingest"
	"cs2stats/internal/store"
)

// fakePlatform — платформа fake.test, данные матчей задаёт тест.
type fakePlatform struct {
	mu      sync.Mutex
	matches map[string]Match
	errs    map[string]error
	calls   atomic.Int32
	delay   time.Duration
}

func (*fakePlatform) Name() string              { return "fake" }
func (*fakePlatform) Title() string             { return "Fake" }
func (*fakePlatform) Example() string           { return "https://fake.test/match/1" }
func (*fakePlatform) MatchURL(id string) string { return "https://fake.test/match/" + id }
func (*fakePlatform) PageHosts() []string       { return []string{"fake.test"} }
func (*fakePlatform) Hosts() []string           { return []string{"fake.test"} }
func (p *fakePlatform) MatchID(u *url.URL) (string, error) {
	seg := pathSegments(u)
	if len(seg) == 2 && seg[0] == "match" {
		return seg[1], nil
	}
	return "", notMatchPage(p)
}
func (p *fakePlatform) Resolve(ctx context.Context, _ *http.Client, id string) (Match, error) {
	p.calls.Add(1)
	time.Sleep(p.delay)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.errs[id]; err != nil {
		return Match{}, err
	}
	m, ok := p.matches[id]
	if !ok {
		return Match{}, notFound("Fake")
	}
	return m, nil
}

func (p *fakePlatform) set(id string, m Match) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.matches[id] = m
}

type env struct {
	im      *Importer
	st      *store.Store
	fake    *fakePlatform
	session int64
	files   map[string]http.HandlerFunc // раздача демок по пути
	srv     *httptest.Server
	woken   atomic.Int32
}

func newEnv(t *testing.T, maxSize int64) *env {
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
	sess, _ := st.CreateSession(context.Background(), "2026-10-08", "Вечер")

	e := &env{st: st, fake: &fakePlatform{matches: map[string]Match{}, errs: map[string]error{}}, session: sess.ID, files: map[string]http.HandlerFunc{}}
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := e.files[r.URL.Path]; ok {
			h(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(e.srv.Close)

	ing := &ingest.Service{Store: st, Demos: demos, TmpDir: tmp, MaxSize: maxSize}
	e.im = New(st, ing, tmp, maxSize, func() { e.woken.Add(1) }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.im.Platforms = []Platform{e.fake}
	e.im.Hosts = []string{"127.0.0.1"}
	e.im.Client = e.im.NewClient()
	e.im.IdleTimeout = 200 * time.Millisecond
	e.im.ProgressEvery = 0
	return e
}

func (e *env) url(p string) string { return e.srv.URL + p }

// serve раздаёт данные; chunked — без Content-Length.
func (e *env) serve(p string, data []byte, chunked bool) {
	e.files[p] = func(w http.ResponseWriter, r *http.Request) {
		if !chunked {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		}
		w.Write(data[:len(data)/2])
		w.(http.Flusher).Flush()
		w.Write(data[len(data)/2:])
	}
}

func at(h, m int) time.Time { return time.Date(2026, 10, 8, h, m, 0, 0, time.UTC) }

func demoBytes(tag string) []byte { return []byte("HL2DEMO\x00" + strings.Repeat(tag, 100)) }

func zipOf(t *testing.T, files map[string][]byte, order []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		w, _ := zw.Create(name)
		w.Write(files[name])
	}
	zw.Close()
	return buf.Bytes()
}

// add добавляет ссылку на fake-матч и возвращает id загрузки.
func (e *env) add(t *testing.T, id string) int64 {
	t.Helper()
	res, err := e.im.Add(context.Background(), e.session, "https://fake.test/match/"+id)
	if err != nil || res[0].Status != AddAccepted {
		t.Fatalf("добавление %s: %+v %v", id, res, err)
	}
	return res[0].ImportID
}

func (e *env) importOf(t *testing.T, id int64) (store.Import, []ingest.FileResult) {
	t.Helper()
	x, err := e.st.GetImport(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var results []ingest.FileResult
	json.Unmarshal(x.Results, &results)
	return x, results
}

func (e *env) order(t *testing.T) []string {
	t.Helper()
	list, _ := e.st.ListSessionMatches(context.Background(), e.session)
	var out []string
	for i, m := range list {
		if m.Ordinal != i+1 {
			t.Fatalf("номера не подряд: %+v", list)
		}
		out = append(out, m.OriginalName)
	}
	return out
}

func (e *env) assertTmpEmpty(t *testing.T) {
	t.Helper()
	entries, _ := os.ReadDir(e.im.TmpDir)
	if len(entries) != 0 {
		t.Errorf("во временном каталоге остались файлы: %d", len(entries))
	}
}

func TestDownloadDemWithoutLength(t *testing.T) {
	e := newEnv(t, 1<<20)
	data := demoBytes("a")
	e.serve("/r/1_de_ancient.dem", data, true)
	e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0), Demos: []Demo{{URL: e.url("/r/1_de_ancient.dem"), Name: "x.dem", Maps: []MapInfo{{1, at(20, 0)}}}}})
	id := e.add(t, "1")
	e.im.drain(context.Background())

	x, results := e.importOf(t, id)
	if x.Status != store.ImportDone || x.BytesDone != int64(len(data)) || x.BytesTotal != nil {
		t.Fatalf("загрузка: %+v", x)
	}
	if len(results) != 1 || results[0].Status != ingest.Accepted || results[0].FileName != "1_de_ancient.dem" {
		t.Fatalf("итог: %+v", results)
	}
	if e.woken.Load() != 1 {
		t.Fatalf("воркер разбора не разбужен")
	}
	if got := e.order(t); !reflect.DeepEqual(got, []string{"1_de_ancient.dem"}) {
		t.Fatalf("матчи: %v", got)
	}
	e.assertTmpEmpty(t)
}

func TestDownloadProgressWithLength(t *testing.T) {
	e := newEnv(t, 1<<20)
	data := demoBytes("b")
	e.files["/d"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Content-Disposition", `attachment; filename="named.dem"`)
		w.Write(data)
	}
	e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0), Demos: []Demo{{URL: e.url("/d"), Name: "x.dem", Maps: []MapInfo{{1, at(20, 0)}}}}})
	id := e.add(t, "1")
	e.im.drain(context.Background())
	x, results := e.importOf(t, id)
	if x.BytesTotal == nil || *x.BytesTotal != int64(len(data)) || x.BytesDone != int64(len(data)) || results[0].FileName != "named.dem" {
		t.Fatalf("прогресс и имя: %+v %+v", x, results)
	}
}

func TestDownloadErrors(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *env) string // возвращает адрес демки
		want  string
	}{
		{"редирект на чужой хост", func(e *env) string {
			e.files["/redir"] = func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, strings.Replace(e.url("/r/a.dem"), "127.0.0.1", "localhost", 1), http.StatusFound)
			}
			e.serve("/r/a.dem", demoBytes("c"), false)
			return e.url("/redir")
		}, "Адрес демки ведёт на сайт вне FastCup и Cybershoke"},
		{"адрес вне списка", func(e *env) string { return "https://evil.example/a.dem" }, "Адрес демки ведёт на сайт вне"},
		{"больше лимита без Content-Length", func(e *env) string {
			e.serve("/big.dem", bytes.Repeat([]byte("x"), 5000), true)
			return e.url("/big.dem")
		}, "Файл демки больше лимита"},
		{"больше лимита по Content-Length", func(e *env) string {
			e.serve("/big.dem", bytes.Repeat([]byte("x"), 5000), false)
			return e.url("/big.dem")
		}, "Файл демки больше лимита"},
		{"ошибка сервера", func(e *env) string { return e.url("/missing.dem") }, "Сайт с демкой ответил ошибкой 404"},
		{"сайт замолчал", func(e *env) string {
			e.files["/slow.dem"] = func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("HL2DEMO"))
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
				case <-time.After(2 * time.Second):
				}
			}
			return e.url("/slow.dem")
		}, "Сайт с демкой не ответил"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, 1000)
			u := c.setup(e)
			e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0), Demos: []Demo{{URL: u, Name: "a.dem", Maps: []MapInfo{{1, at(20, 0)}}}}})
			id := e.add(t, "1")
			e.im.drain(context.Background())
			x, results := e.importOf(t, id)
			if x.Status != store.ImportFailed || !strings.HasPrefix(x.Error, c.want) {
				t.Fatalf("загрузка: %+v", x)
			}
			if len(results) != 1 || results[0].Status != ingest.Failed {
				t.Fatalf("итог: %+v", results)
			}
			if got := e.order(t); len(got) != 0 {
				t.Fatalf("матчи созданы: %v", got)
			}
			e.assertTmpEmpty(t)
		})
	}
}

func TestCancelDuringDownload(t *testing.T) {
	for _, bySession := range []bool{false, true} {
		e := newEnv(t, 1<<20)
		started := make(chan struct{})
		e.files["/d.dem"] = func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("HL2DEMO"))
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
		}
		e.im.IdleTimeout = 10 * time.Second
		e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0), Demos: []Demo{{URL: e.url("/d.dem"), Name: "d.dem", Maps: []MapInfo{{1, at(20, 0)}}}}})
		id := e.add(t, "1")
		done := make(chan struct{})
		go func() { e.im.drain(context.Background()); close(done) }()
		<-started
		ctx := context.Background()
		if bySession {
			e.st.DeleteSession(ctx, e.session)
			e.im.CancelSession(e.session)
		} else {
			e.st.DeleteImport(ctx, id)
			e.im.Cancel(id)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("скачивание не прервано")
		}
		if _, err := e.st.GetImport(ctx, id); err != store.ErrNotFound {
			t.Fatalf("загрузка осталась: %v", err)
		}
		if got := e.order(t); len(got) != 0 {
			t.Fatalf("матч создан: %v", got)
		}
		e.assertTmpEmpty(t)
	}
}

// Прерванное остановкой сервиса скачивание возвращается в очередь и выполняется заново.
func TestRunResumesInterrupted(t *testing.T) {
	e := newEnv(t, 1<<20)
	e.serve("/a.dem", demoBytes("r"), false)
	e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0), Demos: []Demo{{URL: e.url("/a.dem"), Name: "a.dem", Maps: []MapInfo{{1, at(20, 0)}}}}})
	id := e.add(t, "1")
	if _, ok, _ := e.st.ClaimNextImport(context.Background()); !ok { // «сервис остановился посреди скачивания»
		t.Fatal("claim")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.im.Run(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if x, _ := e.importOf(t, id); x.Status == store.ImportDone {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("прерванная загрузка не выполнена после старта")
}

func TestSeriesArchive(t *testing.T) {
	e := newEnv(t, 1<<20)
	m1, m2 := demoBytes("1"), demoBytes("2")
	archive := zipOf(t, map[string][]byte{"match_5_map2.dem": m2, "match_5_map1.dem": m1, "readme.txt": []byte("x")},
		[]string{"match_5_map2.dem", "readme.txt", "match_5_map1.dem"})
	e.files["/demos/5"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="match_5.zip"`)
		w.Write(archive)
	}
	e.fake.set("5", Match{Number: 5, StartedAt: at(20, 0), Demos: []Demo{{URL: e.url("/demos/5"), Name: "match_5.zip",
		Maps: []MapInfo{{1, at(20, 0)}, {2, at(20, 40)}}}}})
	// другой матч, сыгранный между картами серии, скачан раньше
	e.serve("/x.dem", demoBytes("x"), false)
	e.fake.set("9", Match{Number: 9, StartedAt: at(20, 20), Demos: []Demo{{URL: e.url("/x.dem"), Name: "x.dem", Maps: []MapInfo{{1, at(20, 20)}}}}})
	e.add(t, "9")
	e.im.drain(context.Background())
	id := e.add(t, "5")
	e.im.drain(context.Background())

	x, results := e.importOf(t, id)
	if x.Status != store.ImportDone || len(results) != 2 || results[0].FileName != "match_5_map1.dem" || results[1].FileName != "match_5_map2.dem" {
		t.Fatalf("загрузка: %+v %+v", x, results)
	}
	if got := e.order(t); !reflect.DeepEqual(got, []string{"match_5_map1.dem", "x.dem", "match_5_map2.dem"}) {
		t.Fatalf("порядок: %v", got)
	}
	e.assertTmpEmpty(t)

	// повтор той же серии после ошибки отмечает карты как уже загруженные
	e.st.FinishImport(context.Background(), id, "ошибка", nil)
	if _, err := e.st.RetryImport(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	e.im.drain(context.Background())
	_, results = e.importOf(t, id)
	if len(results) != 2 || results[0].Status != ingest.Duplicate || results[1].Status != ingest.Duplicate {
		t.Fatalf("повтор: %+v", results)
	}
}

func TestSeriesPartial(t *testing.T) {
	e := newEnv(t, 1000)
	small := demoBytes("s")
	big := bytes.Repeat([]byte("y"), 5000) // сжимается хорошо, но после распаковки больше лимита
	archive := zipOf(t, map[string][]byte{"m_map1.dem": small, "m_map2.dem": big}, []string{"m_map1.dem", "m_map2.dem"})
	e.serve("/z.zip", archive, false)
	e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0), Demos: []Demo{{URL: e.url("/z.zip"), Name: "m.zip", Maps: []MapInfo{{1, at(20, 0)}, {2, at(20, 40)}}}}})
	id := e.add(t, "1")
	e.im.drain(context.Background())
	x, results := e.importOf(t, id)
	if x.Status != store.ImportFailed || !strings.Contains(x.Error, "лимит") || len(results) != 2 ||
		results[0].Status != ingest.Accepted || results[1].Status != ingest.Failed {
		t.Fatalf("частичный итог: %+v %+v", x, results)
	}
	if got := e.order(t); !reflect.DeepEqual(got, []string{"m_map1.dem"}) {
		t.Fatalf("первая карта должна остаться: %v", got)
	}
	if e.woken.Load() != 1 {
		t.Fatal("принятая карта должна разбудить воркер")
	}
}

func TestSeriesAlreadyUploadedMap(t *testing.T) {
	e := newEnv(t, 1<<20)
	m1, m2 := demoBytes("1"), demoBytes("2")
	manual := e.im.Ingest.Ingest(context.Background(), e.session, "manual.dem", bytes.NewReader(m2))
	archive := zipOf(t, map[string][]byte{"s_map1.dem": m1, "s_map2.dem": m2}, []string{"s_map1.dem", "s_map2.dem"})
	e.serve("/s.zip", archive, false)
	e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0), Demos: []Demo{{URL: e.url("/s.zip"), Name: "s.zip", Maps: []MapInfo{{1, at(20, 0)}, {2, at(20, 40)}}}}})
	id := e.add(t, "1")
	e.im.drain(context.Background())
	x, results := e.importOf(t, id)
	if x.Status != store.ImportDone || results[0].Status != ingest.Accepted || results[1].Status != ingest.Duplicate || results[1].MatchID != manual.MatchID {
		t.Fatalf("итог: %+v %+v", x, results)
	}
}

func TestArchiveWithoutDemos(t *testing.T) {
	e := newEnv(t, 1<<20)
	e.serve("/e.zip", zipOf(t, map[string][]byte{"a.txt": []byte("x")}, []string{"a.txt"}), false)
	e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0), Demos: []Demo{{URL: e.url("/e.zip"), Name: "e.zip", Maps: []MapInfo{{1, at(20, 0)}}}}})
	id := e.add(t, "1")
	e.im.drain(context.Background())
	if x, _ := e.importOf(t, id); x.Status != store.ImportFailed || x.Error != "В архиве нет демок." {
		t.Fatalf("%+v", x)
	}
}

func TestAddLinks(t *testing.T) {
	e := newEnv(t, 1<<20)
	ctx := context.Background()
	e.fake.set("2", Match{Number: 2, StartedAt: at(21, 0)})
	e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0)})
	e.fake.errs["3"] = errNotFinished

	// порядок ввода: 2 (21:00), чужой сайт, 1 (20:00), не законченный 3, повтор 2 в другом виде
	res, err := e.im.Add(ctx, e.session, "https://fake.test/match/2\nhttps://example.com/x  http://fake.test/match/1\nhttps://fake.test/match/3 fake.test/match/2")
	if err != nil {
		t.Fatal(err)
	}
	var statuses []string
	for _, r := range res {
		statuses = append(statuses, r.Status)
	}
	if !reflect.DeepEqual(statuses, []string{AddAccepted, AddError, AddAccepted, AddError, AddAccepted}) {
		t.Fatalf("статусы: %v (%+v)", statuses, res)
	}
	// загрузки созданы по времени игры: 1 раньше 2
	if res[2].ImportID >= res[0].ImportID || res[4].ImportID != res[0].ImportID || res[4].URL != "https://fake.test/match/2" {
		t.Fatalf("очередь: %+v", res)
	}
	if !strings.HasPrefix(res[3].Error, "Матч ещё не закончен") || !strings.HasPrefix(res[1].Error, "Поддерживаются ссылки") {
		t.Fatalf("ошибки: %+v", res)
	}
	list, _ := e.st.ListSessionImports(ctx, e.session)
	if len(list) != 2 {
		t.Fatalf("загрузок %d, ожидалось 2", len(list))
	}

	// уже скачивается: без обращения к платформе
	calls := e.fake.calls.Load()
	res, _ = e.im.Add(ctx, e.session, "https://fake.test/match/1")
	if res[0].Status != AddDownloading || res[0].SessionID != e.session || res[0].SessionTitle != "Вечер" || e.fake.calls.Load() != calls {
		t.Fatalf("уже скачивается: %+v", res)
	}

	// уже есть матч: ordinal и без обращения к платформе
	e.st.AddMatchFrom(ctx, e.session, "sha", "a.dem", &store.MatchSource{Platform: "fake", Number: 7, Map: 1, PlayedAt: at(19, 0)})
	res, _ = e.im.Add(ctx, e.session, "https://fake.test/match/7")
	if res[0].Status != AddExists || res[0].Ordinal != 1 || res[0].MatchID == 0 || e.fake.calls.Load() != calls {
		t.Fatalf("уже есть: %+v", res)
	}

	// упавшая загрузка этой сессии ставится на повтор
	x, _, _ := e.st.ClaimNextImport(ctx)
	e.st.FinishImport(ctx, x.ID, "ошибка", nil)
	res, _ = e.im.Add(ctx, e.session, x.URL)
	if res[0].Status != AddRetried || res[0].ImportID != x.ID {
		t.Fatalf("повтор: %+v", res)
	}

	if _, err := e.im.Add(ctx, e.session, " \n "); err != ErrEmptyInput {
		t.Fatalf("пустой ввод: %v", err)
	}
	var many []string
	for i := 0; i <= MaxLinks; i++ {
		many = append(many, "https://fake.test/match/"+strconv.Itoa(100+i))
	}
	if _, err := e.im.Add(ctx, e.session, strings.Join(many, " ")); err != ErrTooMany {
		t.Fatalf("много ссылок: %v", err)
	}
}

func TestAddConcurrentSameLink(t *testing.T) {
	e := newEnv(t, 1<<20)
	e.fake.delay = 50 * time.Millisecond
	e.fake.set("1", Match{Number: 1, StartedAt: at(20, 0)})
	var wg sync.WaitGroup
	results := make([]string, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := e.im.Add(context.Background(), e.session, "https://fake.test/match/1")
			if err == nil {
				results[i] = res[0].Status
			}
		}()
	}
	wg.Wait()
	if !(results[0] == AddAccepted && results[1] == AddDownloading) && !(results[1] == AddAccepted && results[0] == AddDownloading) {
		t.Fatalf("результаты: %v", results)
	}
	if list, _ := e.st.ListSessionImports(context.Background(), e.session); len(list) != 1 {
		t.Fatalf("загрузок %d", len(list))
	}
}
