.PHONY: build install lint fmt sync-plugin-docs

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

install:
	go install ./cmd/twig

build:
	go build -ldflags "$(LDFLAGS)" -o out/twig ./cmd/twig

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

sync-plugin-docs:
	./scripts/sync-plugin-docs.sh
