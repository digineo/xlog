MAKEFLAGS += --no-print-directory

# The repository hosts three Go modules: the root xlog module and the
# optional xlog/slogor and xlog/graylog backends. Most targets iterate
# over all of them.
MODULES = . slogor graylog

## help (prints target names with trailing "## comment")

.PHONY: help
help: ## print a short help message
	@grep -hE '^[a-zA-Z_-]+:[^:]*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

## testing

.PHONY: test
test: ## runs tests for all modules
	@for mod in $(MODULES); do \
		echo "==> testing $$mod"; \
		( cd $$mod && go test -race ./... ) || exit 1; \
	done

.PHONY: coverage.out
coverage.out: ## runs tests and writes a coverage profile (root module)
	go test -race -covermode=atomic -coverprofile=$@ ./...

coverage.html: coverage.out ## renders the coverage profile as HTML
	go tool cover -html $< -o $@

## development

.PHONY: lint
lint: ## runs golangci-lint on all modules
	@for mod in $(MODULES); do \
		echo "==> linting $$mod"; \
		( cd $$mod && golangci-lint run ) || exit 1; \
	done

.PHONY: update
update: ## update dependencies in all modules
	@for mod in $(MODULES); do \
		echo "==> updating $$mod"; \
		( cd $$mod && go get -u -t . ) || exit 1; \
	done

.PHONY: tidy
tidy: ## runs go mod tidy on all modules
	@for mod in $(MODULES); do \
		echo "==> tidying $$mod"; \
		( cd $$mod && go mod tidy ) || exit 1; \
	done

.PHONY: clean
clean: ## cleanup build fragments
	rm -f coverage.*
