package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func addImport(t *testing.T, s *Store, sessionID int64, ext string) Import {
	t.Helper()
	x, err := s.AddImport(context.Background(), sessionID, "https://cs2.fastcup.net/matches/"+ext, "fastcup", ext)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestImportQueue(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")

	a := addImport(t, s, sess.ID, "1")
	b := addImport(t, s, sess.ID, "2")
	if a.Status != ImportQueued || string(a.Results) != "[]" {
		t.Fatalf("новая загрузка: %+v", a)
	}

	got, ok, err := s.ClaimNextImport(ctx)
	if err != nil || !ok || got.ID != a.ID || got.Status != ImportDownloading {
		t.Fatalf("claim #1: %+v %v %v", got, ok, err)
	}
	total := int64(100)
	if err := s.UpdateImportProgress(ctx, a.ID, 40, &total); err != nil {
		t.Fatal(err)
	}
	// рестарт: прерванная загрузка снова в очереди и снова первая
	if n, err := s.ResetDownloading(ctx); err != nil || n != 1 {
		t.Fatalf("reset: %d %v", n, err)
	}
	got, _, _ = s.ClaimNextImport(ctx)
	if got.ID != a.ID || got.BytesDone != 0 || got.BytesTotal != nil {
		t.Fatalf("после рестарта: %+v", got)
	}
	got, _, _ = s.ClaimNextImport(ctx)
	if got.ID != b.ID {
		t.Fatalf("claim #2: %+v", got)
	}
	if _, ok, _ := s.ClaimNextImport(ctx); ok {
		t.Fatal("очередь должна быть пуста")
	}

	// повтор только у упавшей
	if _, err := s.RetryImport(ctx, b.ID); !errors.Is(err, ErrImportState) {
		t.Fatalf("retry downloading: %v", err)
	}
	if err := s.FinishImport(ctx, b.ID, "сайт с демкой не ответил", json.RawMessage(`[{"status":"error"}]`)); err != nil {
		t.Fatal(err)
	}
	r, err := s.RetryImport(ctx, b.ID)
	if err != nil || r.Status != ImportQueued || r.Error != "" || string(r.Results) != "[]" || r.FinishedAt != nil {
		t.Fatalf("retry: %+v %v", r, err)
	}
	if _, err := s.RetryImport(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("retry неизвестной: %v", err)
	}
	s.FinishImport(ctx, b.ID, "ещё раз", json.RawMessage("null"))
	if x, _ := s.GetImport(ctx, b.ID); string(x.Results) != "[]" {
		t.Fatalf("null в итоге должен стать []: %s", x.Results)
	}
	if err := s.FinishImport(ctx, 999, "", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("finish неизвестной: %v", err)
	}
}

func TestListSessionImports(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	queued := addImport(t, s, sess.ID, "1")
	accepted := addImport(t, s, sess.ID, "2")
	dup := addImport(t, s, sess.ID, "3")
	failed := addImport(t, s, sess.ID, "4")
	s.FinishImport(ctx, accepted.ID, "", json.RawMessage(`[{"status":"accepted","matchId":1}]`))
	s.FinishImport(ctx, dup.ID, "", json.RawMessage(`[{"status":"accepted"},{"status":"duplicate"}]`))
	s.FinishImport(ctx, failed.ID, "ошибка", nil)

	list, err := s.ListSessionImports(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, x := range list {
		ids = append(ids, x.ID)
	}
	if want := []int64{queued.ID, dup.ID, failed.ID}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("видимые загрузки %v, ожидались %v", ids, want)
	}
}

func TestDeleteImportKeepsMatches(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	x := addImport(t, s, sess.ID, "27802385")
	m, err := s.AddMatchFrom(ctx, sess.ID, "sha", "a.dem", &MatchSource{ImportID: &x.ID, Platform: "fastcup", Number: 27802385, Map: 1, PlayedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteImport(ctx, x.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteImport(ctx, x.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("повторное удаление: %v", err)
	}
	var importID *int64
	s.db.QueryRow("SELECT import_id FROM matches WHERE id = ?", m.ID).Scan(&importID)
	if importID != nil {
		t.Fatalf("import_id не обнулён: %d", *importID)
	}
	// источник матча переживает удаление загрузки
	src, err := s.FindImportSource(ctx, sess.ID, "fastcup", "27802385", 27802385)
	if err != nil || src.Match == nil || src.Match.ID != m.ID {
		t.Fatalf("источник после удаления загрузки: %+v %v", src, err)
	}
}

func TestDeleteSessionDeletesImports(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	x := addImport(t, s, sess.ID, "1")
	if _, err := s.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetImport(ctx, x.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("загрузка удалённой сессии: %v", err)
	}
}

func TestFindImportSource(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	other, _ := s.CreateSession(ctx, "2026-10-09", "")

	src, err := s.FindImportSource(ctx, sess.ID, "fastcup", "1", 1)
	if err != nil || src.Active != nil || src.Match != nil || src.Failed != nil {
		t.Fatalf("пустой источник: %+v %v", src, err)
	}

	// уже скачивается в другой сессии
	active := addImport(t, s, other.ID, "1")
	src, _ = s.FindImportSource(ctx, sess.ID, "fastcup", "1", 1)
	if src.Active == nil || src.Active.ID != active.ID {
		t.Fatalf("активная загрузка: %+v", src)
	}

	// упавшая загрузка в этой сессии; в другой сессии упавшая не считается
	failed := addImport(t, s, sess.ID, "2")
	s.ClaimNextImport(ctx)
	s.ClaimNextImport(ctx)
	s.FinishImport(ctx, failed.ID, "ошибка", nil)
	src, _ = s.FindImportSource(ctx, sess.ID, "fastcup", "2", 2)
	if src.Failed == nil || src.Failed.ID != failed.ID || src.Active != nil {
		t.Fatalf("упавшая загрузка: %+v", src)
	}
	if src, _ := s.FindImportSource(ctx, other.ID, "fastcup", "2", 2); src.Failed != nil {
		t.Fatalf("упавшая загрузка чужой сессии: %+v", src)
	}

	// матч есть в обеих сессиях: сначала из запрошенной; другая платформа с тем же номером не мешает
	at := time.Date(2026, 10, 8, 20, 0, 0, 0, time.UTC)
	inOther, _ := s.AddMatchFrom(ctx, other.ID, "o", "o.dem", &MatchSource{Platform: "fastcup", Number: 3, Map: 1, PlayedAt: at})
	inThis, _ := s.AddMatchFrom(ctx, sess.ID, "t", "t.dem", &MatchSource{Platform: "fastcup", Number: 3, Map: 2, PlayedAt: at})
	s.AddMatchFrom(ctx, sess.ID, "c", "c.dem", &MatchSource{Platform: "cybershoke", Number: 4, Map: 1, PlayedAt: at})
	src, _ = s.FindImportSource(ctx, sess.ID, "fastcup", "3", 3)
	if src.Match == nil || src.Match.ID != inThis.ID {
		t.Fatalf("матч этой сессии: %+v", src.Match)
	}
	src, _ = s.FindImportSource(ctx, other.ID, "fastcup", "3", 3)
	if src.Match == nil || src.Match.ID != inOther.ID {
		t.Fatalf("матч другой сессии: %+v", src.Match)
	}
	if src, _ := s.FindImportSource(ctx, sess.ID, "fastcup", "4", 4); src.Match != nil {
		t.Fatalf("номер другой платформы: %+v", src.Match)
	}

	// после удаления матчей источник свободен
	s.DeleteMatch(ctx, inThis.ID)
	s.DeleteMatch(ctx, inOther.ID)
	if src, _ := s.FindImportSource(ctx, sess.ID, "fastcup", "3", 3); src.Match != nil {
		t.Fatalf("удалённый матч: %+v", src.Match)
	}
}

// sessionOrder возвращает sha матчей сессии по порядку и проверяет, что номера идут подряд с 1.
func sessionOrder(t *testing.T, s *Store, sessionID int64) []string {
	t.Helper()
	list, err := s.ListSessionMatches(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for i, m := range list {
		if m.Ordinal != i+1 {
			t.Fatalf("номера не подряд: %+v", list)
		}
		out = append(out, m.SHA256)
	}
	return out
}

func TestAddMatchByPlayedAt(t *testing.T) {
	ctx := context.Background()
	at := func(h, m int) time.Time { return time.Date(2026, 10, 8, h, m, 0, 0, time.UTC) }
	src := func(platform string, n int64, mapN int, t time.Time) *MatchSource {
		return &MatchSource{Platform: platform, Number: n, Map: mapN, PlayedAt: t}
	}
	type add struct {
		sha string
		src *MatchSource // nil — матч из файла
	}
	cases := []struct {
		name string
		adds []add
		want []string
	}{
		{"ссылки в обратном порядке",
			[]add{{"B", src("fastcup", 2, 1, at(21, 0))}, {"A", src("fastcup", 1, 1, at(20, 0))}},
			[]string{"A", "B"}},
		{"более ранний матч добавлен позже",
			[]add{{"A", src("fastcup", 1, 1, at(20, 0))}, {"B", src("fastcup", 2, 1, at(21, 0))}, {"C", src("fastcup", 3, 1, at(20, 30))}},
			[]string{"A", "C", "B"}},
		{"разные платформы по времени, а не по номеру",
			[]add{{"CS", src("cybershoke", 12000000, 1, at(20, 0))}, {"FC", src("fastcup", 27000000, 1, at(19, 0))}},
			[]string{"FC", "CS"}},
		{"матч между картами серии",
			[]add{{"M1", src("cybershoke", 5, 1, at(20, 0))}, {"M2", src("cybershoke", 5, 2, at(20, 40))}, {"X", src("fastcup", 9, 1, at(20, 20))}},
			[]string{"M1", "X", "M2"}},
		{"равное время: по номеру матча, затем карты",
			[]add{{"n2", src("fastcup", 2, 1, at(20, 0))}, {"n1m2", src("fastcup", 1, 2, at(20, 0))}, {"n1m1", src("fastcup", 1, 1, at(20, 0))}},
			[]string{"n1m1", "n1m2", "n2"}},
		{"матч из ссылки встаёт перед более поздним, файл остаётся первым",
			[]add{{"F", nil}, {"A", src("fastcup", 2, 1, at(21, 0))}, {"C", src("fastcup", 1, 1, at(20, 0))}},
			[]string{"F", "C", "A"}},
		{"более поздний матч из ссылки после файла идёт в конец",
			[]add{{"A", src("fastcup", 1, 1, at(20, 0))}, {"F", nil}, {"B", src("fastcup", 2, 1, at(21, 0))}},
			[]string{"A", "F", "B"}},
		{"файлы не двигаются друг относительно друга",
			[]add{{"F1", nil}, {"B", src("fastcup", 2, 1, at(21, 0))}, {"F2", nil}, {"A", src("fastcup", 1, 1, at(20, 0))}},
			[]string{"F1", "A", "B", "F2"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := openTest(t)
			sess, _ := s.CreateSession(ctx, "2026-10-08", "")
			for _, a := range c.adds {
				if _, err := s.AddMatchFrom(ctx, sess.ID, a.sha, a.sha+".dem", a.src); err != nil {
					t.Fatal(err)
				}
			}
			if got := sessionOrder(t, s, sess.ID); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("порядок %v, ожидался %v", got, c.want)
			}
		})
	}
}

// После ручной перестановки новая вставка ориентируется на текущий порядок.
func TestAddMatchByPlayedAtAfterReorder(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	sess, _ := s.CreateSession(ctx, "2026-10-08", "")
	at := func(h int) time.Time { return time.Date(2026, 10, 8, h, 0, 0, 0, time.UTC) }
	a, _ := s.AddMatchFrom(ctx, sess.ID, "A", "a", &MatchSource{Platform: "fastcup", Number: 1, Map: 1, PlayedAt: at(20)})
	b, _ := s.AddMatchFrom(ctx, sess.ID, "B", "b", &MatchSource{Platform: "fastcup", Number: 2, Map: 1, PlayedAt: at(22)})
	if _, err := s.ReorderMatches(ctx, sess.ID, []int64{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	// C (21:00): первый матч с более поздним временем — B на месте 1
	s.AddMatchFrom(ctx, sess.ID, "C", "c", &MatchSource{Platform: "fastcup", Number: 3, Map: 1, PlayedAt: at(21)})
	if got := sessionOrder(t, s, sess.ID); !reflect.DeepEqual(got, []string{"C", "B", "A"}) {
		t.Fatalf("порядок %v", got)
	}
}
