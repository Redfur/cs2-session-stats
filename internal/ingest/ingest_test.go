package ingest

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

	"cs2stats/internal/store"
)

// sampleDemo совпадает с содержимым testdata/sample.dem.bz2 (в stdlib нет bzip2-кодировщика).
var sampleDemo = append([]byte("HL2DEMO\x00"), bytes.Repeat(func() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}(), 40)...)

type env struct {
	svc     *Service
	store   *store.Store
	demos   *LocalStorage
	session int64
}

func newEnv(t *testing.T, maxSize int64) env {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	demos, _ := NewLocalStorage(filepath.Join(dir, "demos"))
	tmp := filepath.Join(dir, "tmp")
	os.MkdirAll(tmp, 0o755)
	sess, _ := st.CreateSession(context.Background(), "2026-10-08", "")
	return env{svc: &Service{Store: st, Demos: demos, TmpDir: tmp, MaxSize: maxSize}, store: st, demos: demos, session: sess.ID}
}

func (e env) ingest(name string, data []byte) FileResult {
	return e.svc.Ingest(context.Background(), e.session, name, bytes.NewReader(data))
}

func (e env) assertTmpEmpty(t *testing.T) {
	t.Helper()
	entries, _ := os.ReadDir(e.svc.TmpDir)
	if len(entries) != 0 {
		t.Errorf("во временном каталоге остались файлы: %d", len(entries))
	}
}

func gz(t *testing.T, data []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

func zst(t *testing.T, data []byte) []byte {
	var buf bytes.Buffer
	w, _ := zstd.NewWriter(&buf)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range files {
		f, _ := w.Create(name)
		f.Write(data)
	}
	w.Close()
	return buf.Bytes()
}

func TestFormats(t *testing.T) {
	bz2, err := os.ReadFile("testdata/sample.dem.bz2")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"match.dem":     sampleDemo,
		"match.dem.gz":  gz(t, sampleDemo),
		"match.dem.bz2": bz2,
		"MATCH.DEM.ZST": zst(t, sampleDemo),
		"match.zip":     zipOf(t, map[string][]byte{"readme.txt": []byte("x"), "inner/match.dem": sampleDemo}),
	}
	sum := sha256.Sum256(sampleDemo)
	wantSHA := hex.EncodeToString(sum[:])

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, 1<<20)
			res := e.ingest(name, data)
			if res.Status != Accepted || res.MatchID == 0 {
				t.Fatalf("результат: %+v", res)
			}
			m, _ := e.store.GetMatch(context.Background(), res.MatchID)
			if m.SHA256 != wantSHA || m.OriginalName != name || m.Status != store.StatusPending {
				t.Errorf("матч: %+v", m)
			}
			rc, err := e.demos.Open(wantSHA)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := io.ReadAll(rc)
			rc.Close()
			if !bytes.Equal(got, sampleDemo) {
				t.Error("сохранённое содержимое не совпадает с распакованной демкой")
			}
			e.assertTmpEmpty(t)
		})
	}
}

func TestRejected(t *testing.T) {
	cases := []struct {
		name, file string
		data       []byte
		errPart    string
	}{
		{"неподдерживаемый формат", "notes.txt", []byte("hi"), "неподдерживаемый формат"},
		{"zip с двумя демками", "two.zip", zipOf(t, map[string][]byte{"a.dem": sampleDemo, "b.dem": []byte("x")}), "ровно одну демку"},
		{"zip без демок", "none.zip", zipOf(t, map[string][]byte{"a.txt": []byte("x")}), "ровно одну демку"},
		{"битый gzip", "bad.dem.gz", []byte("not gzip"), "gzip"},
		{"превышение лимита", "big.dem", bytes.Repeat([]byte{1}, 2048), "превышает лимит"},
		{"превышение лимита после распаковки", "big.dem.gz", gz(t, bytes.Repeat([]byte{1}, 2048)), "превышает лимит"},
		{"пустой файл", "empty.dem", nil, "пуст"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, 1024)
			res := e.ingest(c.file, c.data)
			if res.Status != Failed || !strings.Contains(res.Error, c.errPart) {
				t.Fatalf("ожидалась ошибка с %q, получено %+v", c.errPart, res)
			}
			e.assertTmpEmpty(t)
			entries, _ := os.ReadDir(e.demos.Dir)
			if len(entries) != 0 {
				t.Error("отклонённый файл попал в хранилище")
			}
		})
	}
}

func TestDuplicate(t *testing.T) {
	e := newEnv(t, 1<<20)
	first := e.ingest("match.dem", sampleDemo)
	if first.Status != Accepted {
		t.Fatalf("первая загрузка: %+v", first)
	}

	// та же демка в архиве и в другой сессии
	other, _ := e.store.CreateSession(context.Background(), "2026-10-09", "")
	res := e.svc.Ingest(context.Background(), other.ID, "match.dem.gz", bytes.NewReader(gz(t, sampleDemo)))
	if res.Status != Duplicate || res.MatchID != first.MatchID || res.SessionID != e.session {
		t.Fatalf("ожидался duplicate со ссылкой на матч %d, получено %+v", first.MatchID, res)
	}
	matches, _ := e.store.ListSessionMatches(context.Background(), other.ID)
	if len(matches) != 0 {
		t.Error("дубликат создал новый матч")
	}
	e.assertTmpEmpty(t)

	// сохранённая демка не должна пострадать от повторной загрузки
	rc, err := e.demos.Open(func() string { s := sha256.Sum256(sampleDemo); return hex.EncodeToString(s[:]) }())
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
}

func TestRemoveDemos(t *testing.T) {
	e := newEnv(t, 1<<20)
	ctx := context.Background()
	shaOf := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	otherDemo := append([]byte("HL2DEMO\x00"), "другая демка"...)

	deleted := e.ingest("a.dem", sampleDemo)
	kept := e.ingest("b.dem", otherDemo)
	if deleted.Status != Accepted || kept.Status != Accepted {
		t.Fatalf("загрузка: %+v %+v", deleted, kept)
	}
	if _, err := e.store.DeleteMatch(ctx, deleted.MatchID); err != nil {
		t.Fatal(err)
	}

	// у второго sha матч есть — файл остаётся; отсутствующий файл ошибкой не считается
	if err := e.svc.RemoveDemos(ctx, []string{shaOf(sampleDemo), shaOf(otherDemo), shaOf([]byte("нет такой"))}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.demos.Open(shaOf(sampleDemo)); !os.IsNotExist(err) {
		t.Fatalf("файл удалённого матча не удалён: %v", err)
	}
	rc, err := e.demos.Open(shaOf(otherDemo))
	if err != nil {
		t.Fatalf("файл живого матча удалён: %v", err)
	}
	rc.Close()

	// после удаления та же демка принимается как новая
	again := e.ingest("a.dem", sampleDemo)
	if again.Status != Accepted || again.MatchID == deleted.MatchID {
		t.Fatalf("повторная загрузка: %+v", again)
	}
	rc, err = e.demos.Open(shaOf(sampleDemo))
	if err != nil {
		t.Fatalf("файл повторной загрузки не сохранён: %v", err)
	}
	rc.Close()
}
