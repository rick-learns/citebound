package server

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rick-learns/citebound/internal/answer"
)

func uiServer(t *testing.T, files fs.FS) *httptest.Server {
	t.Helper()
	s, err := New(Config{Version: "v-test", Mode: "extractive", Pipeline: &answer.Pipeline{}})
	if err != nil {
		t.Fatal(err)
	}
	s.Handle("/", UI(files))
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return ts
}

func TestUI(t *testing.T) {
	ts := uiServer(t, fstest.MapFS{
		"index.html":           {Data: []byte(`<!doctype html><script type="module" src="/assets/index-abc.js"></script>`)},
		"assets/index-abc.js":  {Data: []byte("console.log(1)")},
		"assets/index-abc.css": {Data: []byte("body{}")},
	})
	for _, tc := range []struct {
		method, path string
		wantStatus   int
		wantType     string
		wantCache    string
		wantBody     string
	}{
		{"GET", "/", 200, "text/html; charset=utf-8", "no-cache", "<!doctype html>"},
		{"HEAD", "/", 200, "text/html; charset=utf-8", "no-cache", ""},
		{"GET", "/assets/index-abc.js", 200, "text/javascript; charset=utf-8", "public, max-age=31536000, immutable", "console.log(1)"},
		{"GET", "/assets/index-abc.css", 200, "text/css; charset=utf-8", "public, max-age=31536000, immutable", "body{}"},
		// Directories are never listed, and missing files are not index.html.
		{"GET", "/assets/", 404, "", "", "404 page not found"},
		{"GET", "/assets", 404, "", "", "404 page not found"},
		{"GET", "/missing.js", 404, "", "", "404 page not found"},
		{"POST", "/", 405, "", "", ""},
		// The API keeps its own routes and errors.
		{"GET", "/api/nope", 404, "application/json", "", `"not_found"`},
		{"GET", "/api/meta", 200, "application/json", "", `"version":"v-test"`},
	} {
		req, err := http.NewRequest(tc.method, ts.URL+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		name := tc.method + " " + tc.path
		if res.StatusCode != tc.wantStatus {
			t.Errorf("%s: status %d, want %d", name, res.StatusCode, tc.wantStatus)
		}
		if tc.wantType != "" && !strings.HasPrefix(res.Header.Get("Content-Type"), tc.wantType) {
			t.Errorf("%s: Content-Type %q, want %q", name, res.Header.Get("Content-Type"), tc.wantType)
		}
		if got := res.Header.Get("Cache-Control"); tc.wantCache != "" && got != tc.wantCache {
			t.Errorf("%s: Cache-Control %q, want %q", name, got, tc.wantCache)
		}
		if !strings.Contains(string(body), tc.wantBody) {
			t.Errorf("%s: body %q, want it to contain %q", name, body, tc.wantBody)
		}
		if tc.wantStatus == 405 && res.Header.Get("Allow") != "GET, HEAD" {
			t.Errorf("%s: Allow %q", name, res.Header.Get("Allow"))
		}
		// Every response, UI or API, carries the security headers.

		if res.Header.Get("Content-Security-Policy") == "" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing security headers", name)
		}
	}
}

func TestUINotBuilt(t *testing.T) {
	ts := uiServer(t, nil)
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 404 || !strings.Contains(string(body), "make build") {
		t.Errorf("status %d, body %q", res.StatusCode, body)
	}
}
