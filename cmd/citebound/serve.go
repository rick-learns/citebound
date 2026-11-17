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
)

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	addr := fs.String("addr", "127.0.0.1:8080", "address to listen on")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	log := slog.New(slog.NewTextHandler(stderr, nil))
	fail := func(err error) int {
		log.Error("serve", "err", err)
		return 1
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
		Mode:    "extractive",
		RFCs:    rfcs,
		Pipeline: &answer.Pipeline{
			Index: ix, Lookup: verify.NewCorpusLookup(rfcs), Answerer: answer.Extractive{}, Policy: answer.DefaultPolicy,
		},
		Logger: log,
	})
	if err != nil {
		return fail(err)
	}

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
