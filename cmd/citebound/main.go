// Command citebound is an RFC assistant whose citations are checked by code.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rick-learns/citebound/internal/version"
)

const usage = `citebound: an RFC assistant that cannot fake a citation

Usage:
  citebound <command> [flags]

Commands:
  ask             Answer a question from the corpus: citebound ask <question>
  corpus verify   Check the pinned RFC files against their checksums
  corpus stats    Count the sections, paragraphs, and figures parsed from each RFC
  serve           Serve the HTTP API on 127.0.0.1:8080
  search          Search the corpus: citebound search <query>
  version         Print the build version
  help            Show this help
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches a subcommand and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "ask":
		return runAsk(args[1:], stdout, stderr)
	case "corpus":
		return runCorpus(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "search":
		return runSearch(args[1:], stdout, stderr)
	case "version", "--version":
		fmt.Fprintln(stdout, version.Version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "citebound: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
