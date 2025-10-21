.PHONY: tidy test run lint build

MODULE := github.com/vaheed/kubeOP/kubeop-operator

lint:
	set -euo pipefail
	go vet ./...

build:
	set -euo pipefail
	go build ./cmd/manager

run:
	set -euo pipefail
	go run ./cmd/manager

test:
	set -euo pipefail
	go test ./...

tidy:
	set -euo pipefail
	go mod tidy
