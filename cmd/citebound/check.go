package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/verify"
)

// stdin is read by "check -". Tests replace it.
var stdin io.Reader = os.Stdin

// runCheck verifies an answer written by anyone: a model, a person, or a
// test. It exits 1 if any claim is struck.
func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Usage: citebound check [-dir corpus] <answer-file | ->")
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	var data []byte
	var err error
	if name := fs.Arg(0); name == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(name) //nolint:gosec // the operator names the file to check
	}
	if err != nil {
		return fail(err)
	}
	rfcs, err := corpus.Load(*dir)
	if err != nil {
		return fail(err)
	}
	claims, abstained := verify.ParseAnswer(string(data))
	if len(claims) == 0 {
		if abstained {
			fmt.Fprintln(stdout, "The answer abstains.")
			return 0
		}
		return fail(fmt.Errorf("no claims in %s", fs.Arg(0)))
	}
	results := verify.VerifyAll(verify.NewCorpusLookup(rfcs), claims)
	printClaims(stdout, results)
	verified := 0
	for _, r := range results {
		if r.Status == verify.Verified {
			verified++
		}
	}
	fmt.Fprintf(stdout, "%d of %d claims verified.\n", verified, len(results))
	if verified < len(results) {
		return 1
	}
	return 0
}
