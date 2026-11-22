# Developer tasks for tetherwise.
#
# golangci-lint v2.15.0 and govulncheck v1.10.0 match CI.
# Install notes: docs/00-prerequisites.md

BINARY := bin/tetherwise
PKG := ./cmd/tetherwise

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

GOVULNCHECK_VERSION ?= v1.10.0

.DEFAULT_GOAL := build

.PHONY: build test lint vuln fmt

build:
	mkdir -p $(dir $(BINARY))
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) $(PKG)

test:
	go test ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint v2.15.0 is not on PATH. See docs/00-prerequisites.md"; \
		exit 1; \
	}
	golangci-lint run ./...
	golangci-lint fmt --diff

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

fmt:
	gofmt -w .
	go mod tidy
