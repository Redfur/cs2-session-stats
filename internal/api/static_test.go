package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"cs2stats/internal/webui"
)

func TestStaticAndAPIRouting(t *testing.T) {
	e := newTestEnv(t)
	fsys := fstest.MapFS{
		"index.html":      {Data: []byte("<!doctype html><div id=root></div>")},
		"assets/app-1.js": {Data: []byte("console.log(1)")},
	}
	srv := httptest.NewServer((&Server{Store: e.store, Log: nil}).Handler(webui.New(fsys)))
	defer srv.Close()

	get := func(path string) (int, string, string) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header.Get("Content-Type"), string(b)
	}

	for _, path := range []string{"/", "/sessions/1", "/matches/5"} {
		code, ct, body := get(path)
		if code != 200 || !strings.HasPrefix(ct, "text/html") || !strings.Contains(body, "id=root") {
			t.Errorf("GET %s: %d %s", path, code, ct)
		}
	}
	if code, _, body := get("/assets/app-1.js"); code != 200 || body != "console.log(1)" {
		t.Errorf("статический файл: %d %q", code, body)
	}
	if code, ct, body := get("/api/sessions"); code != 200 || !strings.HasPrefix(ct, "application/json") || strings.TrimSpace(body) != "[]" {
		t.Errorf("API: %d %s %q", code, ct, body)
	}
}
