.PHONY: build install test lint fmt sync-plugin-docs \
	benchmark-setup benchmark-clean benchmark-add benchmark-remove benchmark-list benchmark-all

VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")-dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/twig

build:
	go build -ldflags "$(LDFLAGS)" -o out/twig ./cmd/twig

test:
	go test -tags=integration ./...

lint:
	golangci-lint run --build-tags=integration ./...

fmt:
	golangci-lint fmt ./...

sync-plugin-docs:
	./scripts/sync-plugin-docs.sh

# Benchmark targets
benchmark-setup:
	go run ./cmd/benchmark setup -files=1000 -worktrees=10 /tmp/twig-bench

benchmark-clean:
	go run ./cmd/benchmark run clean small

benchmark-add:
	go run ./cmd/benchmark run add small

benchmark-remove:
	go run ./cmd/benchmark run remove small

benchmark-list:
	go run ./cmd/benchmark run list small

benchmark-all:
	go run ./cmd/benchmark run all small
