package main

import (
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/rick-learns/citebound/internal/corpus"
)

const corpusUsage = `Usage:
  citebound corpus verify [-dir corpus]
  citebound corpus stats [-dir corpus]
`

func runCorpus(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "verify" && args[0] != "stats") {
		fmt.Fprint(stderr, corpusUsage)
		return 2
	}
	fs := flag.NewFlagSet("corpus "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "corpus", "corpus directory containing manifest.json")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if args[0] == "stats" {
		return corpusStats(*dir, stdout, stderr)
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

func corpusStats(dir string, stdout, stderr io.Writer) int {
	rfcs, err := corpus.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "RFC\tSections\tParagraphs\tFigures\t")
	var sections, paragraphs, figures int
	for _, r := range rfcs {
		var p, f int
		for _, s := range r.Sections {
			p += len(s.Paragraphs)
			for _, para := range s.Paragraphs {
				if para.Figure {
					f++
				}
			}
		}
		fmt.Fprintf(tw, "%d\t%d\t%d\t%d\t\n", r.Number, len(r.Sections), p, f)
		sections, paragraphs, figures = sections+len(r.Sections), paragraphs+p, figures+f
	}
	fmt.Fprintf(tw, "Total\t%d\t%d\t%d\t\n", sections, paragraphs, figures)
	if err := tw.Flush(); err != nil {
		fmt.Fprintf(stderr, "citebound: %v\n", err)
		return 1
	}
	return 0
}
