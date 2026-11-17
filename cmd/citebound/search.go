package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
)

func runSearch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	k := fs.Int("k", 5, "number of results")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	query := strings.Join(fs.Args(), " ")
	if query == "" || *k < 1 {
		fmt.Fprintln(stderr, "Usage: citebound search [-dir corpus] [-k 5] <query>")
		return 2
	}
	ctx := context.Background()
	rfcs, err := corpus.Load(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	ix, err := index.Build(ctx, rfcs)
	if err != nil {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	defer ix.Close()
	hits, err := ix.Search(ctx, query, *k)
	if err != nil {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	for i, h := range hits {
		fmt.Fprintf(stdout, "%d. %s  %s (score %.2f)\n   %s\n\n", i+1, h.Ref, h.SectionTitle, h.Score, excerpt(h.Text, 200))
	}
	return 0
}

// excerpt shortens s to about n bytes on a word boundary.
func excerpt(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	if i := strings.LastIndexByte(s[:n], ' '); i > 0 {
		n = i
	}
	return s[:n] + " …"
}
