# Developer setup

This is for a developer who wants to work on `catalog` itself — the Go
source and the release infrastructure. Use the `catalog` Claude skill if
you want to add catalog capabilities to a content repo.

## Prerequisites

Install these once with Homebrew:

```sh
brew install go prettier markdownlint-cli2 golangci-lint
```

Minimum versions: Go 1.25, Prettier 3, markdownlint-cli2 0.17. The
versions above will satisfy those.

You also need an Anthropic API key to run the commands that infer
profiles (`bootstrap`, `update`, `force`). `status`, `diff`, and all
tests run without one.

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

See [README.md](README.md)'s development workflow for the day-to-day
commands once you're set up.
