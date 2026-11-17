package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeAPI serves one canned reply and records the request it got.
type fakeAPI struct {
	status int
	reply  string
	got    *http.Request
	body   map[string]any
}

func (f *fakeAPI) start(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.got = r
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &f.body)
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.reply)
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

var req = Request{System: "Answer with citations.", User: "What is PKCE?", MaxTokens: 512}

func TestAnthropic(t *testing.T) {
	api := &fakeAPI{status: 200, reply: `{"model":"claude-x-1","content":[{"type":"text","text":"PKCE is "},{"type":"text","text":"a proof key."}],"stop_reason":"end_turn","usage":{"input_tokens":120,"output_tokens":8}}`}
	p := &Anthropic{APIKey: "sk-test", Model: "claude-x", BaseURL: api.start(t) + "/"}
	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	want := Response{Text: "PKCE is a proof key.", Model: "claude-x-1", InputTokens: 120, OutputTokens: 8}
	if *resp != want {
		t.Errorf("response = %+v, want %+v", *resp, want)
	}
	if api.got.URL.Path != "/v1/messages" || api.got.Header.Get("x-api-key") != "sk-test" || api.got.Header.Get("anthropic-version") != anthropicVersion {
		t.Errorf("request %s, headers %v", api.got.URL.Path, api.got.Header)
	}
	msgs := api.body["messages"].([]any)
	if api.body["system"] != req.System || api.body["max_tokens"] != 512.0 || len(msgs) != 1 || msgs[0].(map[string]any)["content"] != req.User {
		t.Errorf("body = %v", api.body)
	}
	if p.Name() != "anthropic/claude-x" {
		t.Errorf("Name = %q", p.Name())
	}
}

func TestOpenAI(t *testing.T) {
	api := &fakeAPI{status: 200, reply: `{"model":"gpt-x-2","choices":[{"message":{"content":"PKCE is a proof key."},"finish_reason":"length"}],"usage":{"prompt_tokens":100,"completion_tokens":512}}`}
	p := &OpenAI{APIKey: "sk-test", Model: "gpt-x", BaseURL: api.start(t), Label: "openrouter"}
	resp, err := p.Complete(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	want := Response{Text: "PKCE is a proof key.", Model: "gpt-x-2", InputTokens: 100, OutputTokens: 512, Truncated: true}
	if *resp != want {
		t.Errorf("response = %+v, want %+v", *resp, want)
	}
	if api.got.URL.Path != "/chat/completions" || api.got.Header.Get("Authorization") != "Bearer sk-test" {
		t.Errorf("request %s, headers %v", api.got.URL.Path, api.got.Header)
	}
	msgs := api.body["messages"].([]any)
	if len(msgs) != 2 || msgs[0].(map[string]any)["role"] != "system" || api.body["max_completion_tokens"] != 512.0 {
		t.Errorf("body = %v", api.body)
	}
	if p.Name() != "openrouter/gpt-x" {
		t.Errorf("Name = %q", p.Name())
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name      string
		provider  func(url string) Provider
		status    int
		reply     string
		wantType  string
		wantMsg   string
		retryable bool
	}{
		{"anthropic overloaded", anthropicAt, 529, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, "overloaded_error", "Overloaded", true},
		{"anthropic bad request", anthropicAt, 400, `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens: too large"}}`, "invalid_request_error", "max_tokens: too large", false},
		{"openai rate limit", openAIAt, 429, `{"error":{"type":"rate_limit_error","message":"slow down"}}`, "rate_limit_error", "slow down", true},
		{"gateway string error", openAIAt, 401, `{"error":"Unauthorized"}`, "", "Unauthorized", false},
		{"not json", openAIAt, 502, `<html>bad gateway</html>`, "", "<html>bad gateway</html>", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeAPI{status: tt.status, reply: tt.reply}
			_, err := tt.provider(api.start(t)).Complete(context.Background(), req)
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want *APIError", err)
			}
			if apiErr.Status != tt.status || apiErr.Type != tt.wantType || apiErr.Message != tt.wantMsg || apiErr.Retryable() != tt.retryable {
				t.Errorf("err = %+v (retryable %v)", apiErr, apiErr.Retryable())
			}
			if strings.Contains(err.Error(), "sk-test") {
				t.Errorf("error leaks the API key: %v", err)
			}
		})
	}

	for name, reply := range map[string]string{
		"anthropic": `{"content":[],"usage":{}}`,
		"openai":    `{"choices":[]}`,
	} {
		api := &fakeAPI{status: 200, reply: reply}
		p := map[string]func(string) Provider{"anthropic": anthropicAt, "openai": openAIAt}[name](api.start(t))
		if _, err := p.Complete(context.Background(), req); !errors.Is(err, ErrEmpty) {
			t.Errorf("%s empty reply: err = %v, want ErrEmpty", name, err)
		}
	}
}

// echoKey is an httpDoer that fails with the request's credentials in the
// error, as a misbehaving proxy or custom client might.
type echoKey struct{ wrap error }

func (e echoKey) Do(r *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("proxy refused %s %s: %w", r.Header.Get("x-api-key"), r.Header.Get("Authorization"), e.wrap)
}

// hiddenKey is an httpDoer whose error message is clean but whose wrapped
// cause holds the credentials.
type hiddenKey struct{}

type opaque struct{ cause error }

func (opaque) Error() string   { return "request failed" }
func (o opaque) Unwrap() error { return o.cause }

func (hiddenKey) Do(r *http.Request) (*http.Response, error) {
	return nil, opaque{errors.Join(errors.New("key "+r.Header.Get("x-api-key")+r.Header.Get("Authorization")), context.Canceled)}
}

// chainContains reports whether any error in err's tree mentions s.
func chainContains(err error, s string) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), s) {
		return true
	}
	switch u := err.(type) { //nolint:errorlint // walking the tree by hand
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			if chainContains(e, s) {
				return true
			}
		}
	case interface{ Unwrap() error }:
		return chainContains(u.Unwrap(), s)
	}
	return false
}

func TestErrorsNeverContainTheAPIKey(t *testing.T) {
	const key = "sk-secret-123"
	echo := &fakeAPI{status: 401, reply: `{"error":{"type":"auth_` + key + `","message":"invalid key ` + key + `"}}`}
	echoURL := echo.start(t)
	plain := &fakeAPI{status: 500, reply: "bad key " + key}
	plainURL := plain.start(t)
	providers := map[string]Provider{
		"anthropic structured": &Anthropic{APIKey: key, Model: "m", BaseURL: echoURL},
		"openai structured":    &OpenAI{APIKey: key, Model: "m", BaseURL: echoURL},
		"anthropic fallback":   &Anthropic{APIKey: key, Model: "m", BaseURL: plainURL},
		"openai fallback":      &OpenAI{APIKey: key, Model: "m", BaseURL: plainURL},
		"anthropic transport":  &Anthropic{APIKey: key, Model: "m", Client: echoKey{errors.New("refused")}},
		"openai transport":     &OpenAI{APIKey: key, Model: "m", Client: echoKey{errors.New("refused")}},
	}
	for name, p := range providers {
		_, err := p.Complete(context.Background(), req)
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		var apiErr *APIError
		if errors.As(err, &apiErr) && (strings.Contains(apiErr.Type, key) || strings.Contains(apiErr.Message, key)) {
			t.Errorf("%s: APIError fields contain the key: %+v", name, apiErr)
		}
		if chainContains(err, key) {
			t.Errorf("%s: error tree contains the key: %v", name, err)
		}
		if !strings.Contains(err.Error(), "[REDACTED]") {
			t.Errorf("%s: %v, want a redaction marker", name, err)
		}
	}

	// A clean message does not make a key in a wrapped cause safe.
	for _, p := range []Provider{
		&Anthropic{APIKey: key, Model: "m", Client: hiddenKey{}},
		&OpenAI{APIKey: key, Model: "m", Client: hiddenKey{}},
	} {
		_, err := p.Complete(context.Background(), req)
		if err == nil || chainContains(err, key) || !errors.Is(err, context.Canceled) {
			t.Errorf("%s: hidden cause: %v", p.Name(), err)
		}
	}

	// Redaction keeps the deadline error that callers branch on.
	p := &OpenAI{APIKey: key, Model: "m", Client: echoKey{context.DeadlineExceeded}}
	if _, err := p.Complete(context.Background(), req); !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), key) {
		t.Errorf("deadline: %v", err)
	}
}

func TestOversizedResponse(t *testing.T) {
	big := `{"model":"m","choices":[{"message":{"content":"ok"}}]}` + strings.Repeat(" ", maxResponseBytes) + "garbage"
	for _, status := range []int{200, 500} {
		api := &fakeAPI{status: status, reply: big}
		_, err := openAIAt(api.start(t)).Complete(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), "larger than") {
			t.Errorf("status %d: err = %v, want a size error", status, err)
		}
	}
}

func anthropicAt(url string) Provider { return &Anthropic{APIKey: "sk-test", Model: "m", BaseURL: url} }
func openAIAt(url string) Provider    { return &OpenAI{APIKey: "sk-test", Model: "m", BaseURL: url} }

func TestRecordAndReplay(t *testing.T) {
	api := &fakeAPI{status: 200, reply: `{"model":"m-1","content":[{"type":"text","text":"recorded answer"}],"usage":{"input_tokens":5,"output_tokens":2}}`}
	live := anthropicAt(api.start(t))
	path := filepath.Join(t.TempDir(), "recordings.jsonl")
	rec := &Recorder{Provider: live, Path: path}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	other := req
	other.User = "What is DPoP?"
	if _, err := rec.Complete(context.Background(), other); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sk-test") {
		t.Fatal("recording contains the API key")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("recording mode %v, want 0600", info.Mode().Perm())
	}

	replay, err := LoadReplay(path, live.Name())
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC().Format(time.DateOnly)
	if replay.Oldest != today || replay.Newest != today {
		t.Errorf("recorded %s to %s, want %s", replay.Oldest, replay.Newest, today)
	}
	if replay.Len() != 2 || replay.Name() != live.Name() {
		t.Errorf("replay has %d recordings, name %q", replay.Len(), replay.Name())
	}
	resp, err := replay.Complete(context.Background(), req)
	if err != nil || resp.Text != "recorded answer" || resp.InputTokens != 5 {
		t.Errorf("replay = %+v, %v", resp, err)
	}
	changed := req
	changed.System += " "
	if _, err := replay.Complete(context.Background(), changed); !errors.Is(err, ErrNotRecorded) {
		t.Errorf("changed prompt: err = %v, want ErrNotRecorded", err)
	}

	otherModel, err := LoadReplay(path, "anthropic/other")
	if err != nil || otherModel.Len() != 0 {
		t.Errorf("other provider: %d recordings, %v", otherModel.Len(), err)
	}

	edited := strings.Replace(string(data), "recorded answer", "edited answer", 1)
	edited = strings.Replace(edited, `"user":"What is PKCE?"`, `"user":"What is PKCE??"`, 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReplay(path, live.Name()); err == nil || !strings.Contains(err.Error(), "edited by hand") {
		t.Errorf("hand-edited request: err = %v", err)
	}
}

// fixed is a Provider that always returns the same text.
type fixed string

func (fixed) Name() string { return "test/fixed" }
func (f fixed) Complete(context.Context, Request) (*Response, error) {
	return &Response{Text: string(f)}, nil
}

func TestRecordingLargerThanTheResponseCap(t *testing.T) {
	// JSON escapes '<' as \u003c, six bytes for one.
	path := filepath.Join(t.TempDir(), "rec.jsonl")
	rec := &Recorder{Provider: fixed(strings.Repeat("<", 800<<10)), Path: path}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Size() <= maxResponseBytes {
		t.Fatalf("recording is %d bytes; the test needs one over %d", info.Size(), maxResponseBytes)
	}
	replay, err := LoadReplay(path, "test/fixed")
	if err != nil || replay.Len() != 1 {
		t.Fatalf("LoadReplay: %d recordings, %v", replay.Len(), err)
	}
}

func TestRecorderRepairsOrRefusesTheFileTail(t *testing.T) {
	dir := t.TempDir()
	other := req
	other.User = "What is DPoP?"

	// A complete last record without a newline gets one before the append.
	path := filepath.Join(dir, "unterminated.jsonl")
	rec := &Recorder{Provider: fixed("first"), Path: path}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, bytes.TrimRight(data, "\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := rec.Complete(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	if replay, err := LoadReplay(path, "test/fixed"); err != nil || replay.Len() != 2 {
		t.Errorf("after repair: %v", err)
	}

	// A partial last record is refused, and the file is left as it was.
	path = filepath.Join(dir, "truncated.jsonl")
	partial := data[:len(data)/2]
	if err := os.WriteFile(path, partial, 0o600); err != nil {
		t.Fatal(err)
	}
	rec.Path = path
	if _, err := rec.Complete(context.Background(), other); err == nil || !strings.Contains(err.Error(), "incomplete record") {
		t.Errorf("truncated tail: err = %v", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, partial) {
		t.Error("the truncated file was changed")
	}
}

func TestRecorderMakesAnExistingFilePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rec.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil { // as a git checkout leaves it
		t.Fatal(err)
	}
	rec := &Recorder{Provider: fixed("answer"), Path: path}
	if _, err := rec.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
}
