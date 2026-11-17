package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/rick-learns/citebound/internal/corpus"
)

const corpusUsage = `Usage:
  citebound corpus verify [-dir corpus]
`

func runCorpus(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "verify" {
		fmt.Fprint(stderr, corpusUsage)
		return 2
	}
	fs := flag.NewFlagSet("corpus verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	m, err := corpus.LoadManifest(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	if err := m.Verify(*dir); err != nil {
		fmt.Fprintf(stderr, "citebound: corpus does not match its manifest:\n%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%d RFCs verified\n", len(m.Documents))
	return 0
}
