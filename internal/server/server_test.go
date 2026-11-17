package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rick-learns/citebound/internal/corpus"
)

var (
	pinnedOnce sync.Once
	pinnedRFCs []*corpus.RFC
	pinnedErr  error
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	pinnedOnce.Do(func() { pinnedRFCs, pinnedErr = corpus.Load("../../corpus") })
	if pinnedErr != nil {
		t.Fatal(pinnedErr)
	}
	s, err := New(Config{Version: "v-test", Mode: "extractive", RFCs: pinnedRFCs})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return ts
}

func getJSON(t *testing.T, url string, wantStatus int, v any) {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx // test
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s: status %d, want %d", url, resp.StatusCode, wantStatus)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func TestMeta(t *testing.T) {
	ts := newServer(t)
	var m metaResponse
	getJSON(t, ts.URL+"/api/meta", http.StatusOK, &m)
	if m.Version != "v-test" || m.Mode != "extractive" || len(m.RFCs) != 21 || m.RFCs[0].Number != 6749 {
		t.Errorf("meta = %+v", m)
	}
}

func TestSection(t *testing.T) {
	ts := newServer(t)
	var s sectionResponse
	getJSON(t, ts.URL+"/api/section?rfc=6749&section=10.4", http.StatusOK, &s)
	if s.RFCTitle != "The OAuth 2.0 Authorization Framework" || s.SectionTitle != "Refresh Tokens" || len(s.Paragraphs) == 0 || s.Paragraphs[0].Ref != "RFC6749 §10.4 ¶1" {
		t.Errorf("section = %+v", s)
	}
	for _, q := range []string{"rfc=9999&section=1", "rfc=6749&section=99.9", "rfc=abc&section=1", ""} {
		var e struct{ Error errorBody }
		getJSON(t, ts.URL+"/api/section?"+q, http.StatusNotFound, &e)
		if e.Error.Code != "not_found" {
			t.Errorf("%q: error = %+v", q, e)
		}
	}
	var e struct{ Error errorBody }
	getJSON(t, ts.URL+"/api/nope", http.StatusNotFound, &e)
}

func TestWrongMethod(t *testing.T) {
	ts := newServer(t)
	resp, err := http.Post(ts.URL+"/api/meta", "application/json", strings.NewReader("{}")) //nolint:noctx // test
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != "GET, HEAD" {
		t.Errorf("POST /api/meta: status %d, Allow %q; want 405, GET, HEAD", resp.StatusCode, resp.Header.Get("Allow"))
	}
	head, err := http.Head(ts.URL + "/api/meta") //nolint:noctx // test
	if err != nil {
		t.Fatal(err)
	}
	head.Body.Close()
	if head.StatusCode != http.StatusOK {
		t.Errorf("HEAD /api/meta: status %d, want 200", head.StatusCode)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP = %q", csp)
	}
}
