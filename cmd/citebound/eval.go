package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/eval"
	"github.com/rick-learns/citebound/internal/index"
)

func runEval(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "retrieval" {
		fmt.Fprintln(stderr, "Usage: citebound eval retrieval [flags]")
		return 2
	}
	fs := flag.NewFlagSet("eval retrieval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	set := fs.String("set", "evals/retrieval.jsonl", "labeled questions, one JSON object per line")
	thresholds := fs.String("thresholds", "", "fail if scores fall below the thresholds in this file")
	out := fs.String("out", "", "also write the full report, with per-question results, as JSON to this file")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	fail := func(err error) int {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	var want eval.Thresholds
	if *thresholds != "" {
		var err error
		if want, err = eval.LoadThresholds(*thresholds); err != nil {
			return fail(err)
		}
	}
	cases, err := eval.LoadRetrievalCases(*set)
	if err != nil {
		return fail(fmt.Errorf("%s: %w", *set, err))
	}
	rfcs, err := corpus.Load(*dir)
	if err != nil {
		return fail(err)
	}
	if err := eval.CheckGold(cases, rfcs); err != nil {
		return fail(err)
	}
	ctx := context.Background()
	ix, err := index.Build(ctx, rfcs)
	if err != nil {
		return fail(err)
	}
	defer ix.Close()
	rep, err := eval.RunRetrieval(ctx, ix, cases)
	if err != nil {
		return fail(err)
	}
	rep.WriteSummary(stdout)
	if *out != "" {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(*out, append(data, '\n'), 0o600); err != nil {
			return fail(err)
		}
	}
	if *thresholds != "" {
		if err := rep.Check(want); err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "\nAbove the thresholds in %s.\n", *thresholds)
	}
	return 0
}
