# Every target CI runs is reachable from `make check`.
GO_LDFLAGS := -X github.com/rick-learns/citebound/internal/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: check go-check web-check eval scorecard commits build web-install clean demo test

# Commit subjects are checked against this ref.
BASE ?= origin/main

check: go-check web-check eval demo build commits

go-check:
	go mod tidy -diff
	go vet ./...
	golangci-lint run ./...
# The CLI tests rebuild the index per case, which is slow under -race (#65).
	go test -race -count=1 -timeout 20m ./...
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

# A tour of Citebound with recorded answers: no API key, no network.
demo:
	scripts/demo.sh

# Go and web tests, with no API key.
test: web/node_modules
	go test ./...
	cd web && npm test

web/node_modules: web/package-lock.json
	cd web && npm ci
	touch web/node_modules

# Scores are deterministic, so the thresholds are the committed baseline.
# The scorecard check fails if docs/scorecard.md is stale or a verified claim
# has no support label.
eval:
	go run ./cmd/citebound eval retrieval -thresholds evals/thresholds.json
	go run ./cmd/citebound eval scorecard -check docs/scorecard.md

scorecard:
	go run ./cmd/citebound eval scorecard -out docs/scorecard.md

web-install:
	cd web && npm ci

web-check:
	cd web && npm run lint && npm run typecheck && npm test && npm run build

commits:
	scripts/check-commits.sh $(BASE)

# The binary, with the web UI embedded (docs/decisions/0009-embed-the-web-ui.md).
build: web/node_modules
	cd web && npm run build
	go vet -tags embedui ./...
	go test -tags embedui ./web
	go build -tags embedui -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/citebound ./cmd/citebound

clean:
	rm -rf bin web/dist
