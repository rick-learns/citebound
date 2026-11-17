// Package server serves the Citebound HTTP API: corpus metadata, sections for
// the citation panel, and answers streamed as server-sent events.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/corpus"
)

// Limits on a question.
const (
	MaxQuestionRunes = 1000
	maxBodyBytes     = 16 << 10
)

// Config is what the server needs.
type Config struct {
	Version  string
	Mode     string // "extractive", "model" (live), or "replay" (recorded answers)
	Model    string // the model name, unless Mode is "extractive"
	RFCs     []*corpus.RFC
	Pipeline *answer.Pipeline
	// AskTimeout bounds one answer, end to end. Zero means 60 seconds.
	AskTimeout time.Duration
	Logger     *slog.Logger

	// Guards for a public deployment (decision 0010). Zero disables each;
	// New rejects negative, non-finite, and slower than daily rates.
	//
	// AskRate is how many asks a minute each client may start, in bursts of
	// up to AskBurst (at least 1).
	AskRate  float64
	AskBurst int
	// MaxInFlight caps the asks being answered at once, across all clients.
	MaxInFlight int
	// ClientIPHeader names a header holding the client's address, set by a
	// trusted reverse proxy. Without one, the connection's address is used.
	// Only set it behind a proxy that overwrites the header: otherwise
	// clients choose their own rate-limit key.
	ClientIPHeader string
	// DailyTokens caps the model tokens answers use each UTC day, across all
	// clients (decision 0011). Once it is reached, questions are answered by
	// the extractive answerer until the day ends. Answers already running
	// finish, so a day can overshoot by what they use. Ignored in extractive
	// mode.
	DailyTokens int
}

// Server is the HTTP API.
type Server struct {
	cfg   Config
	rfcs  map[int]*corpus.RFC
	meta  []byte
	mux   *http.ServeMux
	clock func() time.Time
	// Nil when their guard is disabled.
	limits   *limiter
	inflight chan struct{}
	budget   *budget
	// With a budget, charged is the pipeline charging its answers to it, and fallback
	// answers once it is used up: the same pipeline, extractively.
	charged, fallback *answer.Pipeline
}

// New returns a server for cfg.
func New(cfg Config) (*Server, error) {
	if cfg.Pipeline == nil {
		return nil, errors.New("server: no pipeline")
	}
	if err := cfg.validateGuards(); err != nil {
		return nil, err
	}
	if cfg.AskTimeout == 0 {
		cfg.AskTimeout = 60 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	s := &Server{cfg: cfg, rfcs: map[int]*corpus.RFC{}, mux: http.NewServeMux(), clock: time.Now}
	if cfg.AskRate > 0 {
		s.limits = newLimiter(cfg.AskRate, max(1, cfg.AskBurst), func() time.Time { return s.clock() })
	}
	if cfg.MaxInFlight > 0 {
		s.inflight = make(chan struct{}, cfg.MaxInFlight)
	}
	if cfg.DailyTokens > 0 && cfg.Mode != "extractive" {
		s.budget = &budget{limit: cfg.DailyTokens, now: func() time.Time { return s.clock() }, usedUp: func() {
			cfg.Logger.Warn("daily token budget used up; answering extractively until midnight UTC", "limit", cfg.DailyTokens)
		}}
		m, fb := *cfg.Pipeline, *cfg.Pipeline
		m.Answerer = budgeted{cfg.Pipeline.Answerer, s.budget}
		fb.Answerer = answer.Extractive{}
		s.charged, s.fallback = &m, &fb
	}
	m := metaResponse{Version: cfg.Version, Mode: cfg.Mode, Model: cfg.Model, RFCs: []rfcMeta{}}
	for _, r := range cfg.RFCs {
		s.rfcs[r.Number] = r
		m.RFCs = append(m.RFCs, rfcMeta{Number: r.Number, Title: r.Title})
	}
	var err error
	if s.meta, err = json.Marshal(m); err != nil {
		return nil, err
	}
	for _, route := range []struct {
		method, path string
		h            http.HandlerFunc
	}{
		{http.MethodGet, "/api/meta", s.handleMeta},
		{http.MethodGet, "/api/section", s.handleSection},
		{http.MethodPost, "/api/ask", s.handleAsk},
	} {
		s.mux.HandleFunc(route.method+" "+route.path, route.h)
		// Without this, the /api/ catch-all below would answer 404.
		allow := route.method
		if allow == http.MethodGet {
			allow += ", " + http.MethodHead // GET patterns also match HEAD
		}
		s.mux.HandleFunc(route.path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Allow", allow)
			writeError(w, http.StatusMethodNotAllowed, "bad_request", "use "+route.method)
		})
	}
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})
	return s, nil
}

// Handle registers h for pattern, for example to serve the web UI at "/" (see
// UI).
func (s *Server) Handle(pattern string, h http.Handler) { s.mux.Handle(pattern, h) }

// ServeHTTP implements http.Handler, adding security headers to every
// response.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	s.mux.ServeHTTP(w, r)
}

type rfcMeta struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

type metaResponse struct {
	Version string    `json:"version"`
	Mode    string    `json:"mode"`
	Model   string    `json:"model"`
	RFCs    []rfcMeta `json:"rfcs"`
}

func (s *Server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(s.meta)
}

type paragraphJSON struct {
	Ref       string `json:"ref"`
	Paragraph int    `json:"paragraph"`
	Text      string `json:"text"`
	Figure    bool   `json:"figure"`
}

type sectionResponse struct {
	RFC          int             `json:"rfc"`
	RFCTitle     string          `json:"rfcTitle"`
	Section      string          `json:"section"`
	SectionTitle string          `json:"sectionTitle"`
	Paragraphs   []paragraphJSON `json:"paragraphs"`
}

func (s *Server) handleSection(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.URL.Query().Get("rfc"))
	rfc := s.rfcs[n]
	if err != nil || rfc == nil {
		writeError(w, http.StatusNotFound, "not_found", "no such RFC in the corpus")
		return
	}
	sec := rfc.Section(r.URL.Query().Get("section"))
	if sec == nil {
		writeError(w, http.StatusNotFound, "not_found", "no such section in RFC "+strconv.Itoa(n))
		return
	}
	resp := sectionResponse{RFC: rfc.Number, RFCTitle: rfc.Title, Section: sec.ID, SectionTitle: sec.Title, Paragraphs: []paragraphJSON{}}
	for _, p := range sec.Paragraphs {
		resp.Paragraphs = append(resp.Paragraphs, paragraphJSON{Ref: p.Ref.String(), Paragraph: p.Ref.Paragraph, Text: p.Text, Figure: p.Figure})
	}
	writeJSON(w, http.StatusOK, resp)
}

type askRequest struct {
	Question string `json:"question"`
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "bad_request", "send the question as application/json")
		return
	}
	// Read the raw body first: encoding/json replaces invalid UTF-8 with
	// U+FFFD, so it must be rejected before decoding.
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil || !utf8.Valid(raw) {
		writeError(w, http.StatusBadRequest, "bad_request", `send {"question": "..."} as UTF-8 JSON of at most 16 KiB`)
		return
	}
	var req askRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "bad_request", `send {"question": "..."}`)
		return
	}
	q := strings.TrimSpace(req.Question)
	switch {
	case q == "":
		writeError(w, http.StatusBadRequest, "bad_request", "the question is empty")
		return
	case utf8.RuneCountInString(q) > MaxQuestionRunes:
		writeError(w, http.StatusBadRequest, "bad_request", "the question must be at most "+strconv.Itoa(MaxQuestionRunes)+" characters")
		return
	}
	// Guards run after validation, so a malformed request costs no token.
	if s.limits != nil {
		if ok, wait := s.limits.allow(clientKey(r, s.cfg.ClientIPHeader)); !ok {
			secs := retryAfter(wait)
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many questions; try again in "+strconv.Itoa(secs)+" s")
			return
		}
	}
	if s.inflight != nil {
		select {
		case s.inflight <- struct{}{}:
			defer func() { <-s.inflight }()
		default:
			w.Header().Set("Retry-After", "5")
			writeError(w, http.StatusServiceUnavailable, "unavailable", "the server is answering too many questions; try again shortly")
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.AskTimeout)
	defer cancel()
	// The server's WriteTimeout suits JSON endpoints, not a stream that may
	// run for the whole ask timeout. Allow that, plus time to send the error.
	stream, err := newEventStream(w, s.clock().Add(s.cfg.AskTimeout+5*time.Second))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "streaming is not supported")
		return
	}
	s.answer(ctx, stream, q)
}
