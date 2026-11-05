# AIBox Makefile — single entry point for local development and CI.
# Run `make help` to list targets. No target requires cloud credentials.

SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := help

# ---- Pinned tool versions (single source of truth for local, pre-commit and CI) ----
GOLANGCI_LINT_VERSION     ?= v2.15.0
ACTIONLINT_VERSION        ?= v1.7.12
EDITORCONFIG_CHECKER_VERSION ?= v3.11.3
SHELLCHECK_PY_VERSION     ?= 0.11.0.1
YAMLLINT_VERSION          ?= 1.38.0
MARKDOWNLINT_CLI2_VERSION ?= 0.24.0
PRE_COMMIT_VERSION        ?= 4.7.0
GORELEASER_VERSION        ?= v2.20.0

BIN := $(CURDIR)/bin
export PATH := $(BIN):$(PATH)

GOLANGCI_LINT := $(BIN)/golangci-lint
ACTIONLINT    := $(BIN)/actionlint
EC            := $(BIN)/editorconfig-checker
SHELLCHECK    := $(BIN)/shellcheck
YAMLLINT      := uvx yamllint==$(YAMLLINT_VERSION)
MARKDOWNLINT  := npx --yes markdownlint-cli2@$(MARKDOWNLINT_CLI2_VERSION)

SHELL_SCRIPTS = $(shell find . -name '*.sh' -not -path './.specify/*' -not -path './bin/*' -not -path './node_modules/*' | sort)

##@ General

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z0-9_.-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

.PHONY: dev-setup
dev-setup: ## Check prerequisites, install pinned tools and pre-commit hooks
	@hack/dev-setup.sh
	@$(MAKE) --no-print-directory tools
	@if command -v pre-commit >/dev/null 2>&1; then pre-commit install --install-hooks; \
		else uvx pre-commit==$(PRE_COMMIT_VERSION) install --install-hooks; fi

.PHONY: tools
tools: $(GOLANGCI_LINT) $(ACTIONLINT) $(EC) $(SHELLCHECK) ## Install pinned linters into ./bin

$(GOLANGCI_LINT):
	GOBIN=$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
$(ACTIONLINT):
	GOBIN=$(BIN) go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
$(EC):
	GOBIN=$(BIN) go install github.com/editorconfig-checker/editorconfig-checker/v3/cmd/editorconfig-checker@$(EDITORCONFIG_CHECKER_VERSION)
$(SHELLCHECK):
	uv venv --quiet --allow-existing $(BIN)/.shellcheck
	uv pip install --quiet --python $(BIN)/.shellcheck shellcheck-py==$(SHELLCHECK_PY_VERSION)
	ln -sf .shellcheck/bin/shellcheck $(SHELLCHECK)

.PHONY: fmt
fmt: $(GOLANGCI_LINT) ## Format Go code
	$(GOLANGCI_LINT) fmt ./...

##@ Quality

.PHONY: lint
lint: lint-go lint-actions lint-pins lint-shell lint-yaml lint-markdown lint-editorconfig ## Run every linter

.PHONY: lint-go
lint-go: $(GOLANGCI_LINT) ## Lint Go code
	$(GOLANGCI_LINT) run ./...

.PHONY: lint-actions
lint-actions: $(ACTIONLINT) $(SHELLCHECK) ## Lint GitHub Actions workflows (with shellcheck)
	$(ACTIONLINT) -shellcheck=$(SHELLCHECK)

.PHONY: lint-pins
lint-pins: ## Fail if any GitHub Action is not pinned by full commit SHA
	hack/ci/check-action-pins.sh

.PHONY: lint-shell
lint-shell: $(SHELLCHECK) ## Lint shell scripts
	$(SHELLCHECK) $(SHELL_SCRIPTS)

.PHONY: lint-yaml
lint-yaml: ## Lint YAML files
	$(YAMLLINT) --strict .

.PHONY: lint-markdown
lint-markdown: ## Lint Markdown files
	$(MARKDOWNLINT)

.PHONY: lint-editorconfig
lint-editorconfig: $(EC) ## Check EditorConfig compliance
	$(EC)

.PHONY: test
test: ## Run unit tests (race detector) and script tests
	go test -race -cover ./...
	hack/ci/dco-check_test.sh

.PHONY: release-check
release-check: ## Validate the GoReleaser configuration
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) check
