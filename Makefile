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
KUBECONFORM_VERSION       ?= v0.8.0
KUSTOMIZE_VERSION         ?= v5.8.3
HELM_VERSION              ?= v4.3.1
K8S_SCHEMA_VERSION        ?= 1.37.0

BIN := $(CURDIR)/bin
export PATH := $(BIN):$(PATH)

PROFILE ?= local
AIBOX   := $(BIN)/aibox
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse HEAD 2>/dev/null || echo none)
LDFLAGS := -X github.com/hermeto/aibox/internal/version.Version=$(VERSION) -X github.com/hermeto/aibox/internal/version.Commit=$(COMMIT)

GOLANGCI_LINT := $(BIN)/golangci-lint
ACTIONLINT    := $(BIN)/actionlint
EC            := $(BIN)/editorconfig-checker
KUBECONFORM   := $(BIN)/kubeconform
KUSTOMIZE     := $(BIN)/kustomize
HELM          := $(BIN)/helm
SHELLCHECK    := $(BIN)/shellcheck
YAMLLINT      := uvx yamllint==$(YAMLLINT_VERSION)
MARKDOWNLINT  := npx --yes markdownlint-cli2@$(MARKDOWNLINT_CLI2_VERSION)

SHELL_SCRIPTS = $(shell find . -type f -name '*.sh' -not -path './.specify/*' -not -path './bin/*' -not -path './node_modules/*' -not -path '*/.terraform/*' | sort)

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
tools: $(GOLANGCI_LINT) $(ACTIONLINT) $(EC) $(SHELLCHECK) $(KUBECONFORM) $(KUSTOMIZE) $(HELM) ## Install pinned linters and renderers into ./bin

$(GOLANGCI_LINT):
	GOBIN=$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
$(ACTIONLINT):
	GOBIN=$(BIN) go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
$(EC):
	GOBIN=$(BIN) go install github.com/editorconfig-checker/editorconfig-checker/v3/cmd/editorconfig-checker@$(EDITORCONFIG_CHECKER_VERSION)
$(KUBECONFORM):
	GOBIN=$(BIN) go install github.com/yannh/kubeconform/cmd/kubeconform@$(KUBECONFORM_VERSION)
$(KUSTOMIZE):
	GOBIN=$(BIN) go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)
$(HELM):
	GOBIN=$(BIN) go install helm.sh/helm/v4/cmd/helm@$(HELM_VERSION)
$(SHELLCHECK):
	uv venv --quiet --allow-existing $(BIN)/.shellcheck
	uv pip install --quiet --python $(BIN)/.shellcheck shellcheck-py==$(SHELLCHECK_PY_VERSION)
	ln -sf .shellcheck/bin/shellcheck $(SHELLCHECK)

.PHONY: fmt
fmt: $(GOLANGCI_LINT) ## Format Go code
	$(GOLANGCI_LINT) fmt ./...

##@ Build

.PHONY: build
build: ## Build the aibox CLI into ./bin/aibox
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(AIBOX) ./cmd/aibox

##@ Platform (thin wrappers around the CLI; PROFILE=local|aws-dev|aws-bench)

.PHONY: up down models-sync smoke bench report cost doctor
up: build ## Bring up the platform: aibox up --profile $(PROFILE)
	$(AIBOX) up --profile $(PROFILE)
down: build ## Tear down the platform: aibox down --profile $(PROFILE)
	$(AIBOX) down --profile $(PROFILE)
models-sync: build ## Mirror catalog models: aibox models sync --profile $(PROFILE)
	$(AIBOX) models sync --profile $(PROFILE)
smoke: build ## Smoke-test the platform: aibox smoke --profile $(PROFILE)
	$(AIBOX) smoke --profile $(PROFILE)
bench: build ## Run an experiment: aibox bench run $(EXP) --profile $(PROFILE)
	$(AIBOX) bench run $(EXP) --profile $(PROFILE)
report: build ## Generate the benchmark report: aibox report
	$(AIBOX) report
cost: build ## Show the run's accumulated cost: aibox cost --profile $(PROFILE)
	$(AIBOX) cost --profile $(PROFILE)
doctor: build ## Check prerequisites: aibox doctor --profile $(PROFILE)
	$(AIBOX) doctor --profile $(PROFILE)

##@ Quality

.PHONY: generate
generate: ## Regenerate committed files derived from profiles (Karpenter node pools)
	for p in aws-dev aws-bench; do go run ./cmd/aibox render nodepools --profile $$p > platform/gpu/overlays/$$p/nodepools.yaml; done

.PHONY: check-image-digests
check-image-digests: $(KUSTOMIZE) $(HELM) ## Fail on platform images not pinned by digest (needs Docker and network)
	HELM=$(HELM) KUSTOMIZE=$(KUSTOMIZE) hack/check-image-digests.sh

.PHONY: check-model-revisions
check-model-revisions: ## Verify catalog revisions on Hugging Face; print the commit to pin for placeholders (needs network)
	go run ./hack/hfcheck

.PHONY: validate-manifests
validate-manifests: $(KUBECONFORM) $(KUSTOMIZE) $(HELM) ## Render platform/ (bootstrap chart + overlays) and validate with kubeconform
	HELM=$(HELM) KUSTOMIZE=$(KUSTOMIZE) hack/validate-manifests.sh "$(KUBECONFORM)" "$(K8S_SCHEMA_VERSION)"

.PHONY: lint
lint: lint-go lint-actions lint-pins lint-shell lint-yaml lint-markdown lint-editorconfig validate-manifests ## Run every linter

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

TF      ?= tofu
TF_DIRS := infra/terraform/aws infra/terraform/aws/bootstrap infra/terraform/aws/guardrails

.PHONY: terraform-check
terraform-check: ## Terraform/OpenTofu: fmt, validate and mocked tests (no credentials)
	$(TF) fmt -check -recursive infra/terraform
	for d in $(TF_DIRS); do \
		$(TF) -chdir=$$d init -backend=false -input=false && \
		$(TF) -chdir=$$d validate && \
		$(TF) -chdir=$$d test || exit 1; \
	done
	hack/check-tf-lock.sh $(TF_DIRS)

.PHONY: terraform-lint
terraform-lint: ## tflint with the AWS ruleset (needs network for the plugin)
	cd infra/terraform/aws && tflint --init && tflint --recursive

.PHONY: test
test: ## Run unit tests (race detector) and script tests
	go test -race -cover ./...
	hack/ci/dco-check_test.sh

.PHONY: release-check
release-check: ## Validate the GoReleaser configuration
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) check
