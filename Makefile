GO_DIRS := ./cmd ./internal
BIN_DIR := ./bin
BIN_NAME := go-repo-orchestrator
MAIN_PKG := ./cmd/go-repo-orchestrator
BIN_PATH := $(BIN_DIR)/$(BIN_NAME)
GO_LIST_PATTERNS := ./cmd/... ./internal/... $(if $(wildcard ./pkg),./pkg/...)
GO_PACKAGES := $(shell go list $(GO_LIST_PATTERNS))
GOLANGCI_LINT := $(or $(shell command -v golangci-lint 2>/dev/null),$(shell go env GOPATH)/bin/golangci-lint)
GORELEASER := $(or $(shell command -v goreleaser 2>/dev/null),$(shell go env GOPATH)/bin/goreleaser)

.PHONY: test lint build check fmt-check vet

test:
	go test $(GO_PACKAGES)

lint:
	@test -x "$(GOLANGCI_LINT)" || { \
		echo "golangci-lint не найден. Установите его локально или запустите lint в CI."; \
		exit 1; \
	}
	"$(GOLANGCI_LINT)" run --timeout=5m $(GO_LIST_PATTERNS)

build:
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BIN_PATH) $(MAIN_PKG)

fmt-check:
	@unformatted="$$(gofmt -l $(GO_DIRS))"; \
	if [ -n "$$unformatted" ]; then \
		echo "Found unformatted files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	go vet $(GO_PACKAGES)

check: fmt-check test vet build lint

.PHONY: release-snapshot release-check goreleaser-install release-dry

goreleaser-install:
	@test -x "$(GORELEASER)" || go install github.com/goreleaser/goreleaser/v2@latest

release-check: goreleaser-install
	"$(GORELEASER)" check

release-snapshot: goreleaser-install
	"$(GORELEASER)" release --snapshot --clean --skip=publish

release-dry: goreleaser-install
	"$(GORELEASER)" release --clean --skip=publish

.PHONY: setup-hooks

setup-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/commit-msg
