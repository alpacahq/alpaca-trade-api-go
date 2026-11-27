# The default "help" goal nicely prints all the available goals based on the funny looking ## comments.
# Source: https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
.DEFAULT_GOAL := help
.PHONY: help
help:  ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -v '\[generated-sdk advanced\]' | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: generated-sdk-help
generated-sdk-help: ## Display advanced generated SDK maintenance goals
	@grep -E '^[a-zA-Z_-]+:.*?## \[generated-sdk advanced\] .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## \\[generated-sdk advanced\\] "}; {printf "\033[36m%-44s\033[0m %s\n", $$1, $$2}'

GOLANGCI_LINT_VERSION := $(shell sed -n '/golangci-lint-action/,/version:/s/.*version: \(v[0-9.]*\).*/\1/p' .github/workflows/go.yml)
GO_BIN_DIR := $(shell go env GOBIN)
ifeq ($(GO_BIN_DIR),)
GO_BIN_DIR := $(shell go env GOPATH)/bin
endif

.PHONY: generate
generate: ## Generate easyjson
	go generate ./...

.PHONY: golangci-lint
golangci-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: lint
lint: golangci-lint ## Run the linters
	"$(GO_BIN_DIR)/golangci-lint" run

.PHONY: generated-sdk-tooling-lint
generated-sdk-tooling-lint: golangci-lint ## Run the standalone generated SDK tooling linters
	cd $(GENERATED_SDK_ROOT)/tooling && "$(GO_BIN_DIR)/golangci-lint" run

.PHONY: generated-sdk-tooling-check
generated-sdk-tooling-check: generated-sdk-require-offline-go ## Test, vet, and verify tracked generated SDK inputs offline without Docker or JAR cache
	$(GENERATED_SDK_OFFLINE_TOOL) test -count=1 ./...
	$(GENERATED_SDK_OFFLINE_TOOL) vet ./...
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/generatortrust -mode check-tracked -repo-root $(CURDIR)
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/speccheck -mode pinned -repo-root $(CURDIR)

.PHONY: test
test: ## Run the unit tests
	go test -cover ./...

GENERATED_SDK_ROOT := tools/generated-sdk
GENERATED_SDK_GO_TOOLCHAIN := go1.27.3
GENERATED_SDK_TOOL := GOTOOLCHAIN=$(GENERATED_SDK_GO_TOOLCHAIN) GOWORK=off GOFLAGS="-mod=readonly" go -C $(GENERATED_SDK_ROOT)/tooling
GENERATED_SDK_GO_ROOT = $(shell GOTOOLCHAIN=$(GENERATED_SDK_GO_TOOLCHAIN) GOWORK=off GOPROXY=off go env GOROOT 2>/dev/null)
GENERATED_SDK_GO = $(GENERATED_SDK_GO_ROOT)/bin/go
GENERATED_SDK_OFFLINE_TOOL = GOTOOLCHAIN=local GOWORK=off GOFLAGS="-mod=readonly" GOPROXY=off GOSUMDB=off $(GENERATED_SDK_GO) -C $(GENERATED_SDK_ROOT)/tooling
GENERATED_SDK_HOST_ARCH = $(shell go env GOHOSTARCH)
GENERATED_SDK_PLATFORM ?= $(if $(filter amd64,$(GENERATED_SDK_HOST_ARCH)),linux/amd64,$(if $(filter arm64,$(GENERATED_SDK_HOST_ARCH)),linux/arm64/v8,unsupported/$(GENERATED_SDK_HOST_ARCH)))

.PHONY: generated-sdk-require-offline-go
generated-sdk-require-offline-go:
	@test -n "$(GENERATED_SDK_GO_ROOT)" && test -x "$(GENERATED_SDK_GO)" || { \
		echo "Pinned $(GENERATED_SDK_GO_TOOLCHAIN) is not cached locally. Run 'make generated-sdk-dependency-bootstrap' or install $(GENERATED_SDK_GO_TOOLCHAIN) first." >&2; \
		exit 1; \
	}

.PHONY: generated-sdk-dependency-bootstrap
generated-sdk-dependency-bootstrap: ## Download root, tooling, and fixture dependencies for offline validation
	@set -eu; tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
		GOTOOLCHAIN=go1.26.10 go version >/dev/null; \
		GOTOOLCHAIN=$(GENERATED_SDK_GO_TOOLCHAIN) go version >/dev/null; \
		cp go.mod go.sum "$$tmp"/; \
		(cd "$$tmp" && GOTOOLCHAIN=go1.26.10 GOWORK=off go mod download all); \
		cp $(GENERATED_SDK_ROOT)/go.mod "$$tmp"/tooling.mod; \
		GOTOOLCHAIN=$(GENERATED_SDK_GO_TOOLCHAIN) GOWORK=off go mod download -modfile="$$tmp/tooling.mod" all; \
		cp $(GENERATED_SDK_ROOT)/generator/fixtures/template-fixture.mod "$$tmp"/fixture.mod; \
		cp $(GENERATED_SDK_ROOT)/generator/fixtures/template-fixture.sum "$$tmp"/fixture.sum; \
		GOTOOLCHAIN=$(GENERATED_SDK_GO_TOOLCHAIN) GOWORK=off go mod download -modfile="$$tmp/fixture.mod" all

.PHONY: generated-sdk-generator-bootstrap
generated-sdk-generator-bootstrap: ## Download and verify the pinned OpenAPI Generator JAR
	$(GENERATED_SDK_TOOL) run ./cmd/generatortrust -mode bootstrap -repo-root $(CURDIR)

.PHONY: generated-sdk-generator-image-bootstrap
generated-sdk-generator-image-bootstrap: ## Pull and verify the pinned generator image for GENERATED_SDK_PLATFORM
	$(GENERATED_SDK_TOOL) run ./cmd/generatortrust -mode image-bootstrap -platform "$(GENERATED_SDK_PLATFORM)" -repo-root $(CURDIR)

.PHONY: generated-sdk-generator-trust-check
generated-sdk-generator-trust-check: generated-sdk-require-offline-go ## Verify pinned generator inputs and trust policy offline
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/generatortrust -mode check -repo-root $(CURDIR)

.PHONY: generated-sdk-generator-digest-proposal
generated-sdk-generator-digest-proposal: generated-sdk-require-offline-go ## [generated-sdk advanced] Print proposed generator artifact digests without modifying tracked files
	@$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/generatortrust -mode digest-proposal -repo-root $(CURDIR)

.PHONY: generated-sdk-template-check
generated-sdk-template-check: generated-sdk-require-offline-go ## Test and verify the reviewed generator template policy offline
	$(GENERATED_SDK_OFFLINE_TOOL) test -count=1 ./...
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/templatecheck -platform "$(GENERATED_SDK_PLATFORM)" -repo-root $(CURDIR)

.PHONY: generated-sdk-spec-check
generated-sdk-spec-check: generated-sdk-require-offline-go ## Verify pinned specifications and feature policy offline
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/speccheck -mode pinned -repo-root $(CURDIR)

.PHONY: generated-sdk-spec-drift-check
generated-sdk-spec-drift-check: ## Compare both approved public URL forms with pinned specifications
	$(GENERATED_SDK_TOOL) run ./cmd/speccheck -mode latest -repo-root $(CURDIR)

.PHONY: generated-sdk-spec-allowlist-proposal
generated-sdk-spec-allowlist-proposal: generated-sdk-require-offline-go ## [generated-sdk advanced] Print proposed pinned-spec feature policy without modifying tracked files
	@$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/speccheck -mode allowlist-proposal -repo-root $(CURDIR)

.PHONY: generated-sdk-generation-candidate-check
generated-sdk-generation-candidate-check: generated-sdk-require-offline-go ## Generate twice and compile an isolated candidate without installing source
	bash $(GENERATED_SDK_ROOT)/generator/run-pinned.sh "$(GENERATED_SDK_PLATFORM)"

.PHONY: generated-sdk-spec-update-dry-run
generated-sdk-spec-update-dry-run: ## [generated-sdk advanced] Acquire, classify, and validate a proposed specification update without tracked writes
	@test -n "$(GENERATED_SDK_SPEC_PROPOSAL_DIR)" || (echo "GENERATED_SDK_SPEC_PROPOSAL_DIR is required" >&2; exit 1)
	@test -n "$(GENERATED_SDK_RETRIEVAL_DATE)" || (echo "GENERATED_SDK_RETRIEVAL_DATE is required (YYYY-MM-DD)" >&2; exit 1)
	$(GENERATED_SDK_TOOL) run ./cmd/specupdate -mode dry-run -repo-root $(CURDIR) \
		-platform "$(GENERATED_SDK_PLATFORM)" -retrieval-date "$(GENERATED_SDK_RETRIEVAL_DATE)" \
		-output-dir "$(GENERATED_SDK_SPEC_PROPOSAL_DIR)"

.PHONY: generated-sdk-spec-update-finalize
generated-sdk-spec-update-finalize: generated-sdk-require-offline-go ## [generated-sdk advanced] Seal a reviewed specification proposal after complete offline validation
	@test -n "$(GENERATED_SDK_SPEC_PROPOSAL_DIR)" || (echo "GENERATED_SDK_SPEC_PROPOSAL_DIR is required" >&2; exit 1)
	@test -n "$(GENERATED_SDK_SPEC_FINAL_DIR)" || (echo "GENERATED_SDK_SPEC_FINAL_DIR is required" >&2; exit 1)
	@test -n "$(GENERATED_SDK_SPEC_REVIEWED_DIR)" || (echo "GENERATED_SDK_SPEC_REVIEWED_DIR is required" >&2; exit 1)
	@test -n "$(GENERATED_SDK_SPEC_PLAN_DIGEST)" || (echo "GENERATED_SDK_SPEC_PLAN_DIGEST is required" >&2; exit 1)
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/specupdate -mode finalize -repo-root $(CURDIR) \
		-platform "$(GENERATED_SDK_PLATFORM)" -proposal-dir "$(GENERATED_SDK_SPEC_PROPOSAL_DIR)" \
		-reviewed-dir "$(GENERATED_SDK_SPEC_REVIEWED_DIR)" -output-dir "$(GENERATED_SDK_SPEC_FINAL_DIR)" \
		-accept-plan "$(GENERATED_SDK_SPEC_PLAN_DIGEST)"

.PHONY: generated-sdk-spec-update-adopt-additive
generated-sdk-spec-update-adopt-additive: generated-sdk-require-offline-go ## [generated-sdk advanced] Adopt a finalized additive specification proposal and validate it offline
	@test -n "$(GENERATED_SDK_SPEC_FINAL_DIR)" || (echo "GENERATED_SDK_SPEC_FINAL_DIR is required" >&2; exit 1)
	@test -n "$(GENERATED_SDK_SPEC_PLAN_DIGEST)" || (echo "GENERATED_SDK_SPEC_PLAN_DIGEST is required" >&2; exit 1)
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/specupdate -mode adopt-additive -repo-root $(CURDIR) \
		-platform "$(GENERATED_SDK_PLATFORM)" -proposal-dir "$(GENERATED_SDK_SPEC_FINAL_DIR)" \
		-accept-plan "$(GENERATED_SDK_SPEC_PLAN_DIGEST)"

.PHONY: generated-sdk-spec-update-adopt-breaking
generated-sdk-spec-update-adopt-breaking: generated-sdk-require-offline-go ## [generated-sdk advanced] Adopt an explicitly acknowledged breaking specification proposal and validate it offline
	@test -n "$(GENERATED_SDK_SPEC_FINAL_DIR)" || (echo "GENERATED_SDK_SPEC_FINAL_DIR is required" >&2; exit 1)
	@test -n "$(GENERATED_SDK_SPEC_PLAN_DIGEST)" || (echo "GENERATED_SDK_SPEC_PLAN_DIGEST is required" >&2; exit 1)
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/specupdate -mode adopt-breaking -repo-root $(CURDIR) \
		-platform "$(GENERATED_SDK_PLATFORM)" -proposal-dir "$(GENERATED_SDK_SPEC_FINAL_DIR)" \
		-accept-plan "$(GENERATED_SDK_SPEC_PLAN_DIGEST)"

.PHONY: generated-sdk-spec-update-rollback
generated-sdk-spec-update-rollback: generated-sdk-require-offline-go ## [generated-sdk advanced] Restore generator inputs after an interrupted specification adoption
	$(GENERATED_SDK_OFFLINE_TOOL) run ./cmd/specupdate -mode rollback -repo-root $(CURDIR)
