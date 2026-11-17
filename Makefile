# Every target CI runs is reachable from `make check`.
GO_LDFLAGS := -X github.com/rick-learns/citebound/internal/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: check go-check web-check build web-install clean

check: go-check web-check

go-check:
	go mod tidy -diff
	go vet ./...
	golangci-lint run ./...
	go test -race -count=1 ./...

web-install:
	cd web && npm ci

web-check:
	cd web && npm run lint && npm run typecheck && npm test

build:
	cd web && npm run build
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o bin/citebound ./cmd/citebound

clean:
	rm -rf bin web/dist
