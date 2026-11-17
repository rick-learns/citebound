package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/server"
	"github.com/rick-learns/citebound/internal/verify"
	"github.com/rick-learns/citebound/internal/version"
	"github.com/rick-learns/citebound/web"
)

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	addr := fs.String("addr", "127.0.0.1:8080", "address to listen on")
	// Guards for a public deployment (decision 0010); 0 disables each.
	askRate := fs.Float64("ask-rate", 10, "questions a minute each client may ask")
	askBurst := fs.Int("ask-burst", 5, "questions a client may ask at once before -ask-rate applies")
	maxInFlight := fs.Int("max-inflight", 8, "questions answered at once, across all clients")
	dailyTokens := fs.Int("daily-tokens", 1_000_000, "model tokens answers may use each UTC day before falling back to extractive answers")
	ipHeader := fs.String("client-ip-header", "", "header holding the client's address, set by a trusted reverse proxy (for example Fly-Client-IP)")
	af := addAnswererFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	log := slog.New(slog.NewTextHandler(stderr, nil))
	fail := func(err error) int {
		log.Error("serve", "err", err)
		return 1
	}

	a, err := af.build()
	if err != nil {
		return fail(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	rfcs, err := corpus.Load(*dir)
	if err != nil {
		return fail(err)
	}
	ix, err := index.Build(ctx, rfcs)
	if err != nil {
		return fail(err)
	}
	defer ix.Close()
	api, err := server.New(server.Config{
		Version: version.Version,
		Mode:    a.mode,
		Model:   a.model,
		RFCs:    rfcs,
		Pipeline: &answer.Pipeline{
			Index: ix, Lookup: verify.NewCorpusLookup(rfcs), Answerer: a.Answerer, Policy: answer.DefaultPolicy,
		},
		Logger:         log,
		AskRate:        *askRate,
		AskBurst:       *askBurst,
		MaxInFlight:    *maxInFlight,
		ClientIPHeader: *ipHeader,
		DailyTokens:    *dailyTokens,
	})
	if err != nil {
		return fail(err)
	}
	api.Handle("/", server.UI(web.Files()))

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fail(err)
	}
	srv := &http.Server{
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       60 * time.Second,
		// The ask handler extends its own write deadline to cover a stream.
		WriteTimeout: 30 * time.Second,
	}
	fmt.Fprintf(stdout, "Citebound %s listening on http://%s\n", version.Version, ln.Addr())
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return fail(err)
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fail(err)
	}
	return 0
}
