package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		for e := err; e != nil; e = errors.Unwrap(e) {
			if strings.Contains(e.Error(), key) {
				t.Errorf("%s: error chain contains the key: %v", name, e)
			}
		}
		if !strings.Contains(err.Error(), "[REDACTED]") {
			t.Errorf("%s: %v, want a redaction marker", name, err)
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
