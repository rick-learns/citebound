package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/eval"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/verify"
)

func runEval(args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) > 0 && args[0] == "retrieval":
		return runEvalRetrieval(args[1:], stdout, stderr)
	case len(args) > 0 && args[0] == "answers":
		return runEvalAnswers(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "Usage: citebound eval retrieval|answers [flags]")
	return 2
}

func runEvalRetrieval(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval retrieval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	set := fs.String("set", "evals/retrieval.jsonl", "labeled questions, one JSON object per line")
	thresholds := fs.String("thresholds", "", "fail if scores fall below the thresholds in this file")
	out := fs.String("out", "", "also write the full report, with per-question results, as JSON to this file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		// Flags after a stray argument would be silently ignored.
		fmt.Fprintf(stderr, "citebound: unexpected argument %q\n", fs.Arg(0))
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
	if err := writeJSON(*out, rep); err != nil {
		return fail(err)
	}
	if *thresholds != "" {
		if err := rep.Check(want); err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "\nAbove the thresholds in %s.\n", *thresholds)
	}
	return 0
}

func runEvalAnswers(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval answers", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	answerable := fs.String("answerable", "evals/retrieval.jsonl", "answerable questions: the retrieval set")
	unanswerable := fs.String("unanswerable", "evals/unanswerable.jsonl", "questions the corpus cannot answer")
	redteam := fs.String("redteam", "evals/redteam.jsonl", "adversarial prompts")
	support := fs.String("support", "evals/support-labels.jsonl", "claim-support labels; every verified claim needs one. Empty skips claim support")
	out := fs.String("out", "", "also write the full report, with every claim, as JSON to this file")
	af := addAnswererFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		// Flags after a stray argument would be silently ignored.
		fmt.Fprintf(stderr, "citebound: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	var sets eval.AnswerSets
	var err error
	if sets.Answerable, err = eval.LoadRetrievalCases(*answerable); err != nil {
		return fail(fmt.Errorf("%s: %w", *answerable, err))
	}
	if sets.Unanswerable, err = eval.LoadUnanswerableCases(*unanswerable); err != nil {
		return fail(fmt.Errorf("%s: %w", *unanswerable, err))
	}
	if sets.RedTeam, err = eval.LoadRedTeamCases(*redteam); err != nil {
		return fail(fmt.Errorf("%s: %w", *redteam, err))
	}
	var labels eval.SupportLabels
	if *support != "" {
		if labels, err = eval.LoadSupportLabels(*support); err != nil {
			return fail(fmt.Errorf("%s: %w", *support, err))
		}
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
	p := &answer.Pipeline{Index: ix, Lookup: verify.NewCorpusLookup(rfcs), Answerer: a, Policy: answer.DefaultPolicy}
	name := a.mode
	if a.model != "" {
		name += " " + a.model
	}
	rep, err := eval.RunAnswers(ctx, p, name, sets, answer.SystemPrompt)
	if err != nil {
		return fail(err)
	}
	if labels != nil {
		if err := rep.ScoreSupport(labels); err != nil {
			return fail(fmt.Errorf("%s: %w", *support, err))
		}
	}
	rep.WriteSummary(stdout)
	if err := writeJSON(*out, rep); err != nil {
		return fail(err)
	}
	return 0
}

// writeJSON writes v, indented, to path. An empty path writes nothing.
func writeJSON(path string, v any) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
