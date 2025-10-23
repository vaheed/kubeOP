SHELL := /bin/bash
APP := kubeop
PKG := ./cmd/api
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_SOURCES := $(shell git ls-files --cached --others --exclude-standard '*.go')

.PHONY: build run test tidy fmt

fmt:
	@if [ -z "$(GO_SOURCES)" ]; then \
	echo "No Go sources detected; skipping gofmt"; \
	else \
	echo "Running gofmt on tracked Go sources"; \
	gofmt -w $(GO_SOURCES); \
	fi

build:
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X kubeop/internal/version.rawVersion=$(VERSION) -X kubeop/internal/version.rawCommit=$(COMMIT) -X kubeop/internal/version.rawDate=$(DATE)" -o bin/$(APP) $(PKG)

run:
	go run $(PKG)

test:
	go test ./...

tidy:
	go mod tidy

