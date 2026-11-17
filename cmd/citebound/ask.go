package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/verify"
)

func runAsk(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	af := addAnswererFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	question := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(question) == "" {
		fmt.Fprintln(stderr, "Usage: citebound ask [-dir corpus] [-model provider/model [-record | -replay file]] <question>")
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	a, err := af.build()
	if err != nil {
		return fail(err)
	}
	ctx := context.Background()
	rfcs, err := corpus.Load(*dir)
	if err != nil {
		return fail(err)
	}
	ix, err := index.Build(ctx, rfcs)
	if err != nil {
		return fail(err)
	}
	defer ix.Close()
	p := &answer.Pipeline{
		Index:    ix,
		Lookup:   verify.NewCorpusLookup(rfcs),
		Answerer: a,
		Policy:   answer.DefaultPolicy,
	}
	res, err := p.Ask(ctx, question, nil)
	if err != nil {
		return fail(err)
	}
	if res.Abstained {
		fmt.Fprintf(stdout, "The corpus does not support an answer to that question (%s).\n", res.Reason)
		if len(res.Claims) == 0 {
			return 0
		}
		fmt.Fprint(stdout, "\nWithheld claims, for transparency:\n\n")
	}
	printClaims(stdout, res.Claims)
	return 0
}

func printClaims(w io.Writer, results []verify.Result) {
	for _, r := range results {
		if r.Status == verify.Verified {
			fmt.Fprintf(w, "✓ %s\n  %s\n\n", r.Claim.Text, r.Claim.Ref)
		} else {
			fmt.Fprintf(w, "✗ %s\n  struck: %s\n\n", r.Claim.Text, r.Reason)
		}
	}
}
