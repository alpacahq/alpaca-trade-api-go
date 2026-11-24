# The default "help" goal nicely prints all the available goals based on the funny looking ## comments.
# Source: https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
.DEFAULT_GOAL := help
.PHONY: help
help:  ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

GOLANGCI_LINT_VERSION := $(shell sed -n '/golangci-lint-action/,/version:/s/.*version: \(v[0-9.]*\).*/\1/p' .github/workflows/go.yml)
GO_BIN_DIR := $(shell go env GOBIN)
EASYJSON_VERSION := v0.7.7
ifeq ($(GO_BIN_DIR),)
GO_BIN_DIR := $(shell go env GOPATH)/bin
endif

.PHONY: generate generate-easyjson
generate: generate-easyjson ## Generate all tracked generated files

generate-easyjson: ## Generate easyjson for SDK entity types
	@set -eu; \
	easyjson_tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$easyjson_tmp"' 0 1 2 15; \
	mkdir -p "$$easyjson_tmp/bin"; \
	cp go.mod "$$easyjson_tmp/easyjson.mod"; \
	cp go.sum "$$easyjson_tmp/easyjson.sum"; \
	GOWORK=off go mod edit -modfile="$$easyjson_tmp/easyjson.mod" \
		-require=github.com/mailru/easyjson@$(EASYJSON_VERSION); \
	GOBIN="$$easyjson_tmp/bin" GOWORK=off \
		go install github.com/mailru/easyjson/easyjson@$(EASYJSON_VERSION); \
	GOWORK=off PATH="$$easyjson_tmp/bin:$$PATH" \
		go generate -modfile="$$easyjson_tmp/easyjson.mod" -run='^easyjson ' ./...

.PHONY: golangci-lint
golangci-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: lint
lint: golangci-lint ## Run the linters
	"$(GO_BIN_DIR)/golangci-lint" run

.PHONY: test
test: ## Run the unit tests
	go test -cover ./...
