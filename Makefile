APP_NAME := gosvc
VERSION ?= dev
RELEASE_PARALLEL ?= 3
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODULE := $(shell go list -m)
LDFLAGS := -s -w \
	-X '$(MODULE)/internal/buildinfo.Version=$(VERSION)' \
	-X '$(MODULE)/internal/buildinfo.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/buildinfo.BuildTime=$(BUILD_TIME)'

.PHONY: build test test-race vet fmt tidy verify run clean completions acceptance certify certify-real benchmark release-prepare release-check release-snapshot release-verify

build:
	mkdir -p bin
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/gosvc

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	go mod tidy

verify:
	test -z "$$(gofmt -l .)"
	go mod tidy
	@if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then git diff --exit-code -- go.mod go.sum; fi
	go vet ./...
	go test ./...
	go build ./...

acceptance:
	go run ./cmd/gosvc acceptance

certify:
	go run ./cmd/gosvc certify --mode static

certify-real:
	go run ./cmd/gosvc certify --mode real --require-real

benchmark:
	go test -run=^$$ -bench=. -benchmem ./internal/generator

completions:
	mkdir -p dist/completions
	go run ./cmd/gosvc completion bash > dist/completions/gosvc.bash
	go run ./cmd/gosvc completion zsh > dist/completions/_gosvc
	go run ./cmd/gosvc completion fish > dist/completions/gosvc.fish
	go run ./cmd/gosvc completion powershell > dist/completions/gosvc.ps1

release-prepare:
	@test -n "$(REPOSITORY)" || (echo "REPOSITORY=ailtonmacedo/gosvc is required" >&2; exit 2)
	go run ./cmd/gosvc release prepare --repository $(REPOSITORY) $(if $(DRY_RUN),--dry-run,)

release-check:
	go run ./cmd/gosvc release check --version $(VERSION) $(if $(REPOSITORY),--repository $(REPOSITORY),) $(if $(ALLOW_PLACEHOLDER),--allow-placeholder,)

release-snapshot:
	go run ./cmd/gosvc release snapshot --version $(VERSION) --output dist --parallel $(RELEASE_PARALLEL) $(if $(REPOSITORY),--repository $(REPOSITORY),) $(if $(ALLOW_PLACEHOLDER),--allow-placeholder,)

release-verify:
	go run ./cmd/gosvc release verify --dist dist

run:
	go run ./cmd/gosvc --help

clean:
	rm -rf bin dist coverage.out
