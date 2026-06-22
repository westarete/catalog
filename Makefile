.PHONY: markdown build test lint

# Format and lint all Markdown in one pass. Always runs on the whole repo.
markdown:
	prettier --write "**/*.md"
	markdownlint-cli2 "**/*.md"

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run ./...
