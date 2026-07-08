MAKEFLAGS += --no-print-directory

## help (prints target names with trailing "## comment")

.PHONY: help
help: ## print a short help message
	@grep -hE '^[a-zA-Z_-]+:[^:]*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

## testing

.PHONY: coverage.out
coverage.out: ## runs tests and writes a coverage profile
	go test -race -covermode=atomic -coverprofile=$@ ./...

coverage.html: coverage.out ## renders the coverage profile as HTML
	go tool cover -html $< -o $@

.PHONY: test
test: ## runs tests
	go test -race ./...

## development

.PHONY: lint
lint: ## runs golangci-lint on source files
	golangci-lint run

.PHONY: clean
clean: ## cleanup build fragments
	rm -f coverage.*
