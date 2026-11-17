package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/llm"
	"github.com/rick-learns/citebound/internal/verify"
)

var (
	pinnedOnce sync.Once
	pinnedRFCs []*corpus.RFC
	pinnedIx   *index.Index
	pinnedErr  error
)

func newServer(t *testing.T, a answer.Answerer) *httptest.Server {
	t.Helper()
	pinnedOnce.Do(func() {
		if pinnedRFCs, pinnedErr = corpus.Load("../../corpus"); pinnedErr != nil {
			return
		}
		pinnedIx, pinnedErr = index.Build(context.Background(), pinnedRFCs)
	})
	if pinnedErr != nil {
		t.Fatal(pinnedErr)
	}
	s, err := New(Config{
		Version: "v-test",
		Mode:    "extractive",
		RFCs:    pinnedRFCs,
		Pipeline: &answer.Pipeline{
			Index: pinnedIx, Lookup: verify.NewCorpusLookup(pinnedRFCs), Answerer: a, Policy: answer.DefaultPolicy,
		},
		AskTimeout: 5 * time.Second,
	})
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
	ts := newServer(t, answer.Extractive{})
	var m metaResponse
	getJSON(t, ts.URL+"/api/meta", http.StatusOK, &m)
	if m.Version != "v-test" || m.Mode != "extractive" || len(m.RFCs) != 21 || m.RFCs[0].Number != 6749 {
		t.Errorf("meta = %+v", m)
	}
}

func TestSection(t *testing.T) {
	ts := newServer(t, answer.Extractive{})
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
	ts := newServer(t, answer.Extractive{})
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

type event struct {
	name string
	data map[string]any
}

// reply is what ask saw. The body is already read and closed.
type reply struct {
	status int
	header http.Header
}

// ask posts a question and reads every event from the stream.
func ask(t *testing.T, ts *httptest.Server, body string) (reply, []event) {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/ask", "application/json", strings.NewReader(body)) //nolint:noctx // test
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := reply{status: resp.StatusCode, header: resp.Header}
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		return r, nil
	}
	var events []event
	var cur event
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &cur.data); err != nil {
				t.Fatalf("bad data %q: %v", line, err)
			}
		case line == "":
			events = append(events, cur)
			cur = event{}
		}
	}
	return r, events
}

func names(events []event) string {
	var n []string
	for _, e := range events {
		n = append(n, e.name)
	}
	return strings.Join(n, ",")
}

func TestAskStreamsAnAnswer(t *testing.T) {
	ts := newServer(t, answer.Extractive{})
	r, events := ask(t, ts, `{"question":"Can a refresh token be bound to a client?"}`)
	if r.status != http.StatusOK {
		t.Fatalf("status %d", r.status)
	}
	if got := names(events); got != "retrieval,claim,verdict,claim,verdict,claim,verdict,done" {
		t.Fatalf("events = %s", got)
	}
	if n := len(events[0].data["passages"].([]any)); n != 8 {
		t.Errorf("%d passages, want 8", n)
	}
	for i := range 3 {
		c, v := events[1+2*i].data, events[2+2*i].data
		if c["id"] != float64(i) || v["id"] != float64(i) || v["status"] != "verified" || v["reason"] != "" {
			t.Errorf("claim %d: %v / %v", i, c, v)
		}
	}
	done := events[len(events)-1].data
	if done["claims"] != 3.0 || done["verified"] != 3.0 || done["struck"] != 0.0 || done["mode"] != "extractive" {
		t.Errorf("done = %v", done)
	}
	if csp := r.header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP = %q", csp)
	}
}

func TestAskAbstains(t *testing.T) {
	ts := newServer(t, answer.Extractive{})
	_, events := ask(t, ts, `{"question":"How do I bake sourdough bread?"}`)
	if got := names(events); got != "retrieval,abstain,done" {
		t.Fatalf("events = %s", got)
	}
	if a := events[1].data; a["code"] != "answerer-abstained" || a["reason"] == "" {
		t.Errorf("abstain = %v", a)
	}

	_, events = ask(t, ts, `{"question":"what is the"}`)
	if got := names(events); got != "retrieval,abstain,done" {
		t.Fatalf("stopwords: events = %s", got)
	}
	if p, ok := events[0].data["passages"].([]any); !ok || len(p) != 0 {
		t.Errorf("stopwords: passages = %v, want []", events[0].data["passages"])
	}
	if a := events[1].data; a["code"] != "no-passages" {
		t.Errorf("stopwords: abstain = %v", a)
	}
}

// fixed answers with the same text every time.
type fixed string

func (f fixed) Answer(context.Context, string, []index.Hit) (string, error) { return string(f), nil }

func TestAskStreamsStruckClaims(t *testing.T) {
	ts := newServer(t, fixed("Tokens never expire. [RFC6749 §10.4 ¶1: \"refresh tokens never expire under any circumstances\"]\nUncited claim."))
	_, events := ask(t, ts, `{"question":"refresh token expiry"}`)
	if got := names(events); got != "retrieval,claim,verdict,claim,verdict,abstain,done" {
		t.Fatalf("events = %s", got)
	}
	if c := events[1].data; c["ref"] != "RFC6749 §10.4 ¶1" {
		t.Errorf("claim = %v", c)
	}
	if v := events[2].data; v["status"] != "struck" || v["reason"] != "quote-not-found" {
		t.Errorf("verdict = %v", v)
	}
	if c, v := events[3].data, events[4].data; c["ref"] != "" || v["reason"] != "uncited" {
		t.Errorf("uncited claim = %v / %v", c, v)
	}
	if done := events[6].data; done["struck"] != 2.0 {
		t.Errorf("done = %v", done)
	}
}

type failing struct{ err error }

func (f failing) Answer(context.Context, string, []index.Hit) (string, error) { return "", f.err }

func TestAskReportsFailures(t *testing.T) {
	for err, code := range map[error]string{
		errors.New("boom"):                           "internal",
		context.DeadlineExceeded:                     "unavailable",
		fmt.Errorf("replay: %w", llm.ErrNotRecorded): "not_recorded",
	} {
		ts := newServer(t, failing{err})
		_, events := ask(t, ts, `{"question":"refresh tokens"}`)
		if got := names(events); got != "retrieval,error" {
			t.Fatalf("%v: events = %s", err, got)
		}
		if e := events[1].data; e["code"] != code || strings.Contains(e["message"].(string), "boom") {
			t.Errorf("%v: error = %v", err, e)
		}
	}
}

// slow answers after a delay.
type slow time.Duration

func (d slow) Answer(ctx context.Context, q string, p []index.Hit) (string, error) {
	time.Sleep(time.Duration(d))
	return answer.Extractive{}.Answer(ctx, q, p)
}

func TestAskOutlastsTheServerWriteTimeout(t *testing.T) {
	newServer(t, answer.Extractive{}) // loads the corpus
	s, err := New(Config{
		Mode: "extractive", RFCs: pinnedRFCs, AskTimeout: 5 * time.Second,
		Pipeline: &answer.Pipeline{Index: pinnedIx, Lookup: verify.NewCorpusLookup(pinnedRFCs), Answerer: slow(500 * time.Millisecond), Policy: answer.DefaultPolicy},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(s)
	ts.Config.WriteTimeout = 200 * time.Millisecond
	ts.Start()
	t.Cleanup(ts.Close)
	if _, events := ask(t, ts, `{"question":"refresh token rotation"}`); !strings.HasSuffix(names(events), ",done") {
		t.Errorf("events = %s, want the stream to finish", names(events))
	}
}

func TestAskRejectsBadRequests(t *testing.T) {
	ts := newServer(t, answer.Extractive{})
	for name, body := range map[string]string{
		"not json":       `question`,
		"unknown field":  `{"question":"q","model":"x"}`,
		"two objects":    `{"question":"q"}{"question":"r"}`,
		"empty":          `{"question":"   "}`,
		"too long":       `{"question":"` + strings.Repeat("a", MaxQuestionRunes+1) + `"}`,
		"too big a body": `{"question":"` + strings.Repeat("a", 20<<10) + `"}`,
		"invalid utf-8":  "{\"question\":\"refresh token \xff rotation\"}",
	} {
		if r, _ := ask(t, ts, body); r.status != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, r.status)
		}
	}

	for ct, want := range map[string]int{
		"text/plain":                      http.StatusUnsupportedMediaType,
		"application/jsonp":               http.StatusUnsupportedMediaType,
		"application/json; broken":        http.StatusUnsupportedMediaType,
		"":                                http.StatusUnsupportedMediaType,
		"Application/JSON":                http.StatusOK,
		"application/json; charset=utf-8": http.StatusOK,
	} {
		resp, err := http.Post(ts.URL+"/api/ask", ct, strings.NewReader(`{"question":"refresh token rotation"}`)) //nolint:noctx // test
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("Content-Type %q: status %d, want %d", ct, resp.StatusCode, want)
		}
	}

	resp, err := http.Get(ts.URL + "/api/ask") //nolint:noctx // test
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/ask: status %d, want 405", resp.StatusCode)
	}
}
