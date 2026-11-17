# 0009. Embed the built web UI behind a build tag

- Status: Accepted
- Date: 2026-11-17

## Context

`citebound serve` should serve the web UI as well as the API, from one
binary, so the demo is one command and one process. The UI is built by Vite
into `web/dist`, which is not committed.

`go:embed` fails to compile if the files it names are missing. A fresh clone,
`go test ./...`, `go install`, and the Go CI job have no `web/dist`, and
should not need Node.js to build the Go code.

## Decision

- The `web` Go package embeds `web/dist` only when built with the `embedui`
  tag (`web/embed.go`). Without it (`web/noembed.go`), `web.Files()` returns
  nil.
- `make build` builds the UI, then the binary with `-tags embedui`. It first
  runs `go vet -tags embedui ./...` and `go test -tags embedui ./web`, which
  checks that every asset `index.html` references was embedded.
- `server.UI` serves the files at `/`. `/api/` keeps its own routes and
  errors, and every response keeps the server's security headers.
  - It serves only GET and HEAD, and never lists a directory.
  - Files under `assets/` are named by a hash of their content, so they are
    cached as immutable. Everything else, including `index.html`, is
    revalidated.
  - With no UI embedded, every path is a plain-text 404 that says to run
    `make build`, or the Vite dev server, which proxies `/api` to the
    server.
- CI builds the embedded binary in the Go job, after the tests that run
  without it.

## Consequences

- `go build ./...` and `go test ./...` keep working without Node.js. A binary
  built that way has no UI, and says so.
- Two builds exist, so `embed.go` is only compiled and vetted by
  `make build`, not by the default lint.
- The UI and the API ship as one versioned binary, so they can't drift apart
  in a deployment.
- `index.html` must be revalidated on every load, or a cached page could ask
  for asset names a new binary no longer has.

## Alternatives considered

- **Commit a placeholder `web/dist`.** Vite empties `dist` on every build, so
  each build would delete a tracked file and leave the tree dirty.
- **Commit the built UI.** That puts generated code in review diffs, and it
  goes stale unless CI checks that it matches a fresh build.
- **Serve `web/dist` from disk with a flag.** That works, but the binary is
  no longer self-contained, and a deployment has to ship and locate the
  directory.
- **Mount at `GET /`.** Go's mux rejects it, because it conflicts with the
  API's `/api/` catch-all, so `server.UI` checks the method itself.
