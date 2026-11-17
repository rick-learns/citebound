// Package server serves the Citebound HTTP API: corpus metadata and sections
// for the citation panel.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/rick-learns/citebound/internal/corpus"
)

// Config is what the server needs.
type Config struct {
	Version string
	Mode    string // "extractive" or "model"
	Model   string // the model name, if Mode is "model"
	RFCs    []*corpus.RFC
	Logger  *slog.Logger
}

// Server is the HTTP API.
type Server struct {
	cfg  Config
	rfcs map[int]*corpus.RFC
	meta []byte
	mux  *http.ServeMux
}

// New returns a server for cfg.
func New(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	s := &Server{cfg: cfg, rfcs: map[int]*corpus.RFC{}, mux: http.NewServeMux()}
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
	} {
		s.mux.HandleFunc(route.method+" "+route.path, route.h)
		// Without this, the /api/ catch-all below would answer 404.
		s.mux.HandleFunc(route.path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Allow", route.method)
			writeError(w, http.StatusMethodNotAllowed, "bad_request", "use "+route.method)
		})
	}
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})
	return s, nil
}

// Handle registers h for pattern, for example to serve the web UI at "/".
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

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, struct {
		Error errorBody `json:"error"`
	}{errorBody{Code: code, Message: message}})
}
