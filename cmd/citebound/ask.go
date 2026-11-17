package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/verify"
)

// passages is how many search results the answerer reads.
const passages = 8

func runAsk(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	question := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(question) == "" {
		fmt.Fprintln(stderr, "Usage: citebound ask [-dir corpus] <question>")
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
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
	hits, err := ix.Search(ctx, question, passages)
	if err != nil && !errors.Is(err, index.ErrEmptyQuery) {
		return fail(err)
	}
	text, err := answer.Extractive{}.Answer(ctx, question, hits)
	if err != nil {
		return fail(err)
	}
	claims, abstained := verify.ParseAnswer(text)
	results := verify.VerifyAll(verify.NewCorpusLookup(rfcs), claims)
	verified := 0
	for _, r := range results {
		if r.Status == verify.Verified {
			verified++
			fmt.Fprintf(stdout, "✓ %s\n  %s\n\n", r.Claim.Text, r.Claim.Ref)
		} else {
			fmt.Fprintf(stdout, "✗ %s\n  struck: %s\n\n", r.Claim.Text, r.Reason)
		}
	}
	if abstained || verified == 0 {
		fmt.Fprintln(stdout, "The corpus does not support an answer to that question.")
	}
	return 0
}
