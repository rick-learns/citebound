package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	case len(args) > 0 && args[0] == "scorecard":
		return runEvalScorecard(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "Usage: citebound eval retrieval|answers|scorecard [flags]")
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

// answerSetFlags name the question sets and labels an answer eval reads.
type answerSetFlags struct {
	answerable, unanswerable, redteam, support *string
}

func addAnswerSetFlags(fs *flag.FlagSet) *answerSetFlags {
	return &answerSetFlags{
		answerable:   fs.String("answerable", "evals/retrieval.jsonl", "answerable questions: the retrieval set"),
		unanswerable: fs.String("unanswerable", "evals/unanswerable.jsonl", "questions the corpus cannot answer"),
		redteam:      fs.String("redteam", "evals/redteam.jsonl", "adversarial prompts"),
		support:      fs.String("support", "evals/support-labels.jsonl", "claim-support labels; every verified claim needs one. Empty skips claim support"),
	}
}

// load reads the sets, and the labels unless -support is empty.
func (f *answerSetFlags) load() (eval.AnswerSets, eval.SupportLabels, error) {
	var sets eval.AnswerSets
	var err error
	if sets.Answerable, err = eval.LoadRetrievalCases(*f.answerable); err != nil {
		return sets, nil, fmt.Errorf("%s: %w", *f.answerable, err)
	}
	if sets.Unanswerable, err = eval.LoadUnanswerableCases(*f.unanswerable); err != nil {
		return sets, nil, fmt.Errorf("%s: %w", *f.unanswerable, err)
	}
	if sets.RedTeam, err = eval.LoadRedTeamCases(*f.redteam); err != nil {
		return sets, nil, fmt.Errorf("%s: %w", *f.redteam, err)
	}
	if *f.support == "" {
		return sets, nil, nil
	}
	labels, err := eval.LoadSupportLabels(*f.support)
	if err != nil {
		return sets, nil, fmt.Errorf("%s: %w", *f.support, err)
	}
	return sets, labels, nil
}

// scoreAnswers asks a every question in sets and, if labels is not nil,
// scores the claims' support.
func scoreAnswers(ctx context.Context, ix *index.Index, rfcs []*corpus.RFC, a answerer, name string, sets eval.AnswerSets, labels eval.SupportLabels, supportPath string) (*eval.AnswerReport, error) {
	p := &answer.Pipeline{Index: ix, Lookup: verify.NewCorpusLookup(rfcs), Answerer: a, Policy: answer.DefaultPolicy}
	rep, err := eval.RunAnswers(ctx, p, name, sets, answer.SystemPrompt)
	if err != nil {
		return nil, err
	}
	if labels != nil {
		if err := rep.ScoreSupport(labels); err != nil {
			return nil, fmt.Errorf("%s: %w", supportPath, err)
		}
	}
	return rep, nil
}

func runEvalAnswers(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval answers", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	sf := addAnswerSetFlags(fs)
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
	sets, labels, err := sf.load()
	if err != nil {
		return fail(err)
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
	name := a.mode
	if a.model != "" {
		name += " " + a.model
	}
	rep, err := scoreAnswers(ctx, ix, rfcs, a, name, sets, labels, *sf.support)
	if err != nil {
		return fail(err)
	}
	rep.WriteSummary(stdout)
	if err := writeJSON(*out, rep); err != nil {
		return fail(err)
	}
	return 0
}

// scorecardConfig is evals/scorecard.json: the answerers the scorecard
// scores, in order.
type scorecardConfig struct {
	Answerers []scorecardAnswerer `json:"answerers"`
}

type scorecardAnswerer struct {
	Name  string `json:"name"`
	Model string `json:"model"`
	// Replay is the recordings file, relative to the config file. A model
	// answerer needs one: the scorecard never calls a model.
	Replay string `json:"replay,omitempty"`
}

func loadScorecardConfig(path string) (scorecardConfig, error) {
	var c scorecardConfig
	data, err := os.ReadFile(path) //nolint:gosec // the path is an operator-chosen config
	if err != nil {
		return c, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return c, fmt.Errorf("%s: more than one JSON value", path)
	}
	if len(c.Answerers) == 0 {
		return c, fmt.Errorf("%s: no answerers", path)
	}
	for i, a := range c.Answerers {
		switch {
		case a.Name == "" || a.Model == "":
			return c, fmt.Errorf("%s: answerer %d: name and model are required", path, i+1)
		case a.Model != "extractive" && a.Replay == "":
			return c, fmt.Errorf("%s: %s: a model answerer needs a replay file", path, a.Name)
		case a.Replay != "":
			c.Answerers[i].Replay = filepath.Join(filepath.Dir(path), a.Replay)
		}
	}
	return c, nil
}

// build makes the answerer, replaying its recordings.
func (a scorecardAnswerer) build() (answerer, error) {
	model, record, replay := a.Model, "", a.Replay
	return (&answererFlags{model: &model, record: &record, replay: &replay}).build()
}

func runEvalScorecard(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval scorecard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	sf := addAnswerSetFlags(fs)
	config := fs.String("config", "evals/scorecard.json", "the answerers to score")
	out := fs.String("out", "", "write the scorecard to this file instead of stdout")
	check := fs.String("check", "", "fail if this file differs from the scorecard, instead of writing it")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "citebound: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	if *out != "" && *check != "" {
		return fail(errors.New("use -out or -check, not both"))
	}
	if *sf.support == "" {
		return fail(errors.New("the scorecard needs -support labels"))
	}
	cfg, err := loadScorecardConfig(*config)
	if err != nil {
		return fail(err)
	}
	var have []byte
	if *check != "" {
		if have, err = os.ReadFile(*check); err != nil {
			return fail(err)
		}
	}
	sets, labels, err := sf.load()
	if err != nil {
		return fail(err)
	}
	ctx := context.Background()
	rfcs, err := corpus.Load(*dir)
	if err != nil {
		return fail(err)
	}
	if err := eval.CheckGold(sets.Answerable, rfcs); err != nil {
		return fail(err)
	}
	ix, err := index.Build(ctx, rfcs)
	if err != nil {
		return fail(err)
	}
	defer ix.Close()
	card := &eval.Scorecard{}
	if card.Retrieval, err = eval.RunRetrieval(ctx, ix, sets.Answerable); err != nil {
		return fail(err)
	}
	for _, c := range cfg.Answerers {
		a, err := c.build()
		if err != nil {
			return fail(fmt.Errorf("%s: %w", c.Name, err))
		}
		rep, err := scoreAnswers(ctx, ix, rfcs, a, c.Name, sets, labels, *sf.support)
		if err != nil {
			return fail(fmt.Errorf("%s: %w", c.Name, err))
		}
		card.Answers = append(card.Answers, rep)
	}
	var buf bytes.Buffer
	card.WriteMarkdown(&buf)
	switch {
	case *check != "":
		if !bytes.Equal(have, buf.Bytes()) {
			return fail(fmt.Errorf("%s is out of date: run make scorecard", *check))
		}
		fmt.Fprintf(stdout, "%s is up to date.\n", *check)
	case *out != "":
		if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil { //nolint:gosec // a public document
			return fail(err)
		}
	default:
		_, _ = stdout.Write(buf.Bytes())
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
