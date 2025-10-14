SHELL := /bin/bash
APP := kubeop
PKG := ./cmd/api
WATCHER := kubeop-watcher
WATCHER_PKG := ./cmd/kubeop-watcher
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build run test tidy

build:
        CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X kubeop/internal/version.Version=$(VERSION) -X kubeop/internal/version.Commit=$(COMMIT) -X kubeop/internal/version.Date=$(DATE)" -o bin/$(APP) $(PKG)
        CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X kubeop/internal/version.Version=$(VERSION) -X kubeop/internal/version.Commit=$(COMMIT) -X kubeop/internal/version.Date=$(DATE)" -o bin/$(WATCHER) $(WATCHER_PKG)

.PHONY: build-watcher
build-watcher:
        CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X kubeop/internal/version.Version=$(VERSION) -X kubeop/internal/version.Commit=$(COMMIT) -X kubeop/internal/version.Date=$(DATE)" -o bin/$(WATCHER) $(WATCHER_PKG)

run:
	go run $(PKG)

test:
	go test ./...

tidy:
	go mod tidy

