# catalog

Once you have 20 or more files in a repository, agents do a pretty poor
job of deciding which files to load into context. They'll randomly load
ones files they don't need, and they won't load the ones that they do
need.

`catalog` is a command-line tool that profiles the documents in your
repo provides agents with a catalog so they can load precisely what they
need. This means better answers, faster responses, and more context
window left for the actual work.

If you want to add catalog capabilities to your repo, install the
`knowledge-work` plugin from West Arete's
[claude-plugins-internal](https://github.com/westarete/claude-plugins-internal)
marketplace. It includes the
[catalog skill](https://github.com/westarete/claude-plugins-internal/tree/main/plugins/knowledge-work/skills/catalog),
which sets your repo up. It will prompt you to install this utility and
set up the necessary configuration.

If you want to do development on this repo, See [SETUP.md](SETUP.md) for
developer installation and first-run instructions.

## How it works

Each entry in `.catalog.md` is a **profile**: a description of the
conditions under which a document is relevant — the same idea as a
skill's description field, which tells the harness when to load the
skill body. A profile says "open this file when the task involves X,"
not "this file is about X." That distinction is what makes routing work:
a wrong summary is ugly; a wrong profile sends the agent to the wrong
file.

The tool infers profiles by reading each document with the surrounding
catalog as context, so it can distinguish near-neighbors from each other
rather than describing them in isolation.

Documents aren't limited to Markdown. Plain text and CSV work today with
no extra setup — just point a glob at them in `.catalog/config.toml`.
PDFs are also supported: catalog sends them to the model as native PDF
documents rather than raw text. A text document longer than 400,000
bytes is cut at a line boundary before it goes to the model, with a note
saying so; a profile only needs the start of a file to say what it is.

## Commands

Run from the repo root. Profiles live in a committed SQLite database at
`.catalog/catalog.db` — the sole source of truth; `.catalog.md` is
always a generated artifact rendered from it. Three commands write
profiles and need `ANTHROPIC_API_KEY`. `init`, `status` and `diff` need
no key. `status` and `diff` read only the database and the filesystem
and have no dependency on Git.

- `catalog init` — create `.catalog/config.toml` from the template built
  into the binary, then edit its `enumerate` and `ignore` globs for the
  repo. It won't overwrite an existing config. `catalog init --print`
  writes the template to stdout instead and changes nothing, so you can
  compare it with a config from an older version when upgrading. No key
  needed.
- `catalog bootstrap` — rebuild the database from scratch: infer
  profiles for every enumerated document in two passes (the second pass
  sharpens each profile using the full catalog as context), then write
  `.catalog.md`. Ignores any existing database or `.catalog.md`
  entirely. Use this when setting up a new repo or after major
  reorganization.
- `catalog update` — re-infer profiles for documents that are new or
  have changed content, drop rows for documents that are deleted or no
  longer enumerated, and always rewrite `.catalog.md`. Run this after
  editing or removing a document, the same way you'd run a formatter.
- `catalog force [file ...]` — re-infer the named documents (or all
  documents) unconditionally. Use it to redo a few entries you're
  unhappy with, or to rebuild everything after a prompt change.
- `catalog status` — report which documents are new, modified, or
  deleted relative to the database, and whether `.catalog.md` on disk
  matches a fresh render of it. Exits non-zero if anything is out of
  sync. No model call needed.
- `catalog diff` — show a unified diff between `.catalog.md` on disk and
  a fresh render of the database: exactly what `update` would change. No
  model call needed.

### What a run costs

Every command that calls the model ends with a line like
`24 model calls, 388,131 tokens in, 12,560 out, 1m8s`. Add a `[pricing]`
table to `.catalog/config.toml` and the line ends with an estimate, such
as `~$0.045`:

```toml
model = "claude-haiku-5-5"

[pricing]
input_per_mtok = 0.10 # dollars per million input tokens
output_per_mtok = 0.50
cache_read_per_mtok = 0.01 # optional; defaults to the input price
```

The Models API doesn't report prices, so catalog can't look them up.
Copy them from
[Anthropic's pricing page](https://platform.claude.com/docs/en/about-claude/pricing)
when you pick a model and again when prices change. The estimate uses
standard prices and counts only the tokens the API reports; it ignores
any batch discount.

Some models charge more for a request whose prompt is long. Haiku 5.5,
for example, charges five times as much once a prompt passes 100,000
tokens. catalog doesn't model that: the estimate always uses the base
prices you give it. Two optional settings help you stay out of the
expensive range and tell you when you didn't:

```toml
warn_request_tokens = 100000 # note in the report when a request is this big
truncate_request_tokens = 80000 # cut text documents to fit this budget
```

- `warn_request_tokens` adds a line after the cost line saying how many
  requests went over, and that the real cost may be higher than the
  estimate. It works without `[pricing]`.
- `truncate_request_tokens` cuts a text document at a line boundary
  before it goes to the model, the same way the 400,000-byte limit does,
  and the run notes each cut. Tokens are converted at 4 bytes each,
  which is rough, and the budget covers the document only, not the
  neighbor profiles or the instructions, so leave room below the
  threshold you care about. PDFs are sent whole and are never cut.
  Unset, the 400,000-byte limit applies.

### Why bootstrap needs two passes

A profile's job is to distinguish a document from the things it might be
confused with — usually its neighbors. On the first pass, those
neighbors have no profiles yet, so every entry is written without that
context. `bootstrap` runs a second pass once all entries exist,
re-inferring each profile with the full catalog in view. That second
pass is what "bootstrap" refers to: the catalog uses its own first-pass
output to improve itself.

`update` and `force` skip the second pass because the entries they leave
untouched already hold real profiles, which serve as neighbor context
for the entries being rewritten.

## Repository layout

```text
catalog/
  *.go                  Go source (at the repo root)
  .goreleaser.yaml      release config
```

## Installing in a repo

The skill walks through the decision and setup. Install the
`knowledge-work` plugin in Claude Code, which includes it:

```text
/plugin marketplace add westarete/claude-plugins-internal
/plugin install knowledge-work
```

The short version: run `catalog init` to create `.catalog/config.toml`,
run `catalog bootstrap` to generate the first `.catalog.md`, wire
`@.catalog.md` into `CLAUDE.md`, and register the pre-commit hook.

## Releases

Releases are tagged from `main` and published via GoReleaser. The
primary install surface is the `westarete/homebrew-tap` Homebrew tap;
GitHub Releases carries the raw binaries as a secondary surface for CI
and non-Homebrew environments.

```sh
brew tap westarete/tap
brew trust westarete/tap
brew install catalog
```

To upgrade later:

```sh
brew upgrade catalog
```

### Making a release

Only tag a release when something changes for the user — new features,
bug fixes, behavior changes, or documentation that affects how they use
the tool. Workflow changes, CI fixes, and internal docs do not warrant a
release.

This project uses [semantic versioning](https://semver.org). Patch
releases fix bugs without changing behavior. Minor releases add
functionality. Major releases make breaking changes.

Before picking a version number, fetch tags from upstream so you have
the full picture:

```sh
git fetch --tags
git tag --sort=-v:refname
```

Increment the appropriate part of the latest tag, create an annotated
tag with a one-line summary of what changed, and push it:

```sh
git tag -a vX.Y.Z -m "Short summary of what changed"
git push origin main
git push origin vX.Y.Z
```

Pushing the tag triggers the GoReleaser workflow, which builds binaries,
creates the GitHub release, and commits the updated cask directly to the
tap repo's `main`.

## macOS Gatekeeper and code signing

Released binaries are code-signed and notarized. They pass Gatekeeper on
macOS 15+ without any security prompt. See [APPLE.md](APPLE.md) for how
the signing and notarization pipeline works.

## Why Go

The Anthropic SDK requires a recent version of Python, and managing
Python versions on macOS is fragile — the system Python is too old, and
juggling pyenv, virtualenvs, or Homebrew Python versions adds friction
for every user who wants to use the tool. Go sidesteps all of that: a
single static binary with no runtime dependencies, distributed via
Homebrew with no version management required on the user's end.

## Development workflow

Near-term work in progress is tracked in [PLAN.md](PLAN.md).

Before committing: `make test` after touching any `.go` file (it lints
with `golangci-lint`, then runs the Go test suite — one command, no
separate lint step to remember), and `make markdown` after touching any
`.md` file (formats and lints the whole repo).

The typical cycle when working on the binary:

1. **Build locally** into `~/.local/bin` to shadow the Homebrew version:

   ```sh
   go build -o ~/.local/bin/catalog
   ```

2. **Verify** you're running the local build:

   ```sh
   which catalog   # should show ~/.local/bin/catalog
   ```

3. **Test in real repos** — run `catalog update`, `catalog status`, etc.
   against actual content repositories to confirm the change works as
   expected in practice, not just in unit tests.

4. **Release** — when satisfied, commit, tag, and push. GoReleaser
   publishes the signed binary to Homebrew.

5. **Remove the local build** to fall back to the released Homebrew
   version:

   ```sh
   rm ~/.local/bin/catalog
   brew upgrade catalog
   ```
