# Developer setup

This is for a developer who wants to work on `catalog` itself — the Go
source and the release infrastructure. Use the `catalog` Claude skill if
you want to add catalog capabilities to a content repo.

## Prerequisites

Install these once with Homebrew:

```sh
brew install go prettier markdownlint-cli2 golangci-lint
```

Minimum versions: Go 1.24, Prettier 3, markdownlint-cli2 0.17. The
versions above will satisfy those.

You also need an Anthropic API key to run the commands that infer
profiles (`bootstrap`, `update`, `force`). `check` and all tests run
without one.

## Clone and verify

```sh
git clone https://github.com/westarete/catalog.git
cd catalog
make test
make markdown
```

Both should exit clean.

## API key

The profile-writing commands read `ANTHROPIC_API_KEY` from the
environment. Export it in your shell profile, or prefix commands inline:

```sh
ANTHROPIC_API_KEY=sk-... catalog bootstrap
```

## Day-to-day

- `make test` — run the Go test suite
- `make lint` — run golangci-lint (includes `go vet`, staticcheck,
  errcheck, and more)
- `make build` — build the binary
- `make markdown` — format and lint all Markdown (always the whole repo)

Run `make markdown` after editing any `.md` file, and before committing.
Run `make test` and `make lint` after editing any `.go` file.
