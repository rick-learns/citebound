# Every target CI runs is reachable from `make check`.
GO_LDFLAGS := -X github.com/rick-learns/citebound/internal/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: check go-check web-check eval commits build web-install clean

# Commit subjects are checked against this ref.
BASE ?= origin/main

check: go-check web-check eval commits

go-check:
	go mod tidy -diff
	go vet ./...
	golangci-lint run ./...
	go test -race -count=1 ./...
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

# Scores are deterministic, so the thresholds are the committed baseline.
eval:
	go run ./cmd/citebound eval retrieval -thresholds evals/thresholds.json

web-install:
	cd web && npm ci

web-check:
	cd web && npm run lint && npm run typecheck && npm test && npm run build

commits:
	scripts/check-commits.sh $(BASE)

build:
	cd web && npm run build
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/citebound ./cmd/citebound

clean:
	rm -rf bin web/dist
