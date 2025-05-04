# The default "help" goal nicely prints all the available goals based on the funny looking ## comments.
# Source: https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
.DEFAULT_GOAL := help
.PHONY: help
help:  ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

GOLANGCI_LINT_VERSION := $(shell sed -n '/golangci-lint-action/,/version:/s/.*version: \(v[0-9.]*\).*/\1/p' .github/workflows/go.yml)

.PHONY: generate
generate: ## Generate easyjson
	go generate ./...

.PHONY: golangci-lint
golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: lint
lint: golangci-lint ## Run the linters
	golangci-lint run

.PHONY: test
test: ## Run the unit tests
	go test -cover ./...
