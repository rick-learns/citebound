# Every target CI runs is reachable from `make check`.
GO_LDFLAGS := -X github.com/rick-learns/citebound/internal/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: check go-check web-check eval commits build web-install clean demo test

# Commit subjects are checked against this ref.
BASE ?= origin/main

check: go-check web-check eval demo build commits

go-check:
	go mod tidy -diff
	go vet ./...
	golangci-lint run ./...
	go test -race -count=1 ./...
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
eval:
	go run ./cmd/citebound eval retrieval -thresholds evals/thresholds.json

web-install:
	cd web && npm ci

web-check:
	cd web && npm run lint && npm run typecheck && npm test && npm run build

commits:
	scripts/check-commits.sh $(BASE)

# The binary, with the web UI embedded (docs/decisions/0009-embed-the-web-ui.md).
build: web/node_modules
	cd web && npm run build
	go test -tags embedui ./web
	go build -tags embedui -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/citebound ./cmd/citebound

clean:
	rm -rf bin web/dist
