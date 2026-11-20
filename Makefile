.PHONY: markdown build test lint mutate

# Format and lint all Markdown in one pass. Always runs on the whole repo.
# The script runs Prettier and markdownlint-cli2 through npx at the versions
# pinned in .markdown-tool-versions, the same ones CI uses.
markdown:
	bin/format-markdown

build:
	go build ./...

test: lint
	go test ./...

lint:
	golangci-lint run ./...

# Mutation-test one package, for example: make mutate PKG=./internal/planner
mutate:
	$(if $(PKG),,$(error PKG is required, e.g. make mutate PKG=./internal/planner))
	$(shell go env GOPATH)/bin/gremlins unleash --timeout-coefficient 50 $(PKG)
