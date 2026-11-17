# 0002. Go backend, TypeScript UI, one SQLite file

- Status: Accepted
- Date: 2026-11-17

## Context

Citebound must be easy for a reviewer to run: one command, no API key, no
services to install. It streams answers to a browser, runs evaluation suites
in CI, and ships a small fixed corpus.

## Decision

- **Backend:** Go. One static binary holds the CLI, the HTTP server, the
  retriever, the verifier, and the eval harness.
- **Storage:** one SQLite file with FTS5, through the pure-Go
  `modernc.org/sqlite` driver, so the build needs no C toolchain.
- **UI:** TypeScript with Vite and no framework. The built assets are embedded
  in the binary with `go:embed`.

## Consequences

- `go build` produces everything; releases are single binaries.
- The pure-Go driver is slower than the C one. At this corpus size (tens of
  thousands of paragraphs) that does not matter; we will measure if it does.
- Two toolchains (Go and Node) are needed to develop, but only Go to run.

## Alternatives considered

- **TypeScript everywhere.** One language, but a Node runtime to ship and a
  weaker story for a single self-contained binary.
- **Python backend.** Strong ML ecosystem, but packaging and startup time work
  against "run it with one command".
- **A vector database service.** Unnecessary at this scale and one more thing
  to run. Retrieval starts with BM25; any embedding work gets its own record.
- **A UI framework (React, Svelte).** The UI is one page with a streaming
  answer and a side panel. Plain TypeScript keeps the bundle small and the
  code readable.
