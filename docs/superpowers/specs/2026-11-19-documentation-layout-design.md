# Documentation layout: design

We organize catalog's documentation with
[Diátaxis](https://diataxis.fr/), which sorts documentation by what the
reader needs: tutorials, how-to guides, reference, and explanation. This
design moves the documentation we have into that shape, so that each doc
serves one need and the gaps show. It also says how we tell people and
agents that we work this way.

## Why

Two documents carry most of what we know about catalog:
`docs/design/daemon.md` and `docs/design/architecture.md`. They were
written before the daemon existed, as a proposal and an outline, and
we've been patching current facts into them since. That causes four
problems.

- **They describe a proposal, not the product.** daemon.md calls itself
  "a brief" and says "We offer this as one example of how to approach
  the problem, not as a plan." architecture.md says "Nothing here is
  built." The daemon is built, so a reader can't tell which parts
  describe catalog today.
- **Their names no longer fit.** The whole repository is the daemon now,
  so a document called "daemon" says nothing about what's in it.
- **They overlap.** What the catalog is for and the contract with
  agents, where agents start and what a tree needs, cloud drives, and
  MCP each appear half in one file and half in the other.
- **They mix kinds of content.** Each holds design, research findings,
  history, and open questions together. Setup steps, configuration keys,
  and file locations sit in the README, and file modes sit in the first
  daemon's spec, where a person looking them up won't find them.

Together the two documents run to about 1,100 lines. An agent that needs
one topic loads all of it, which is the problem catalog exists to solve
for its users.

## Goals

- Each doc serves one Diátaxis need, and its folder says which.
- The explanation docs describe catalog as it's built, in the present
  tense, and say plainly what isn't built yet.
- Reference docs describe the product exactly as the code behaves.
- An index at `docs/README.md` orients a reader to each section and its
  docs, and notes at the end that we organize them with Diátaxis.
- CLAUDE.md tells agents to place each new doc by its type and to use
  Diátaxis to look for what's missing.
- Privacy and security sit inside each topic they bear on, where a
  person making that decision will read them.

## Non-goals

- Writing new tutorials or how-to guides. The setup guides for new users
  and for developers, with help getting initial traction, come next, as
  a separate piece of work that uses this layout to find what's missing.
  This design only moves the setup steps that already exist.
- Changing the content of the research notes, or of the dated specs and
  plans in `docs/superpowers/`.
- Changing any code.

## Anti-goals

This work has failed, even if every file lands where this design says,
if:

- a folder or section exists with nothing real in it (Diátaxis: "It
  certainly does not mean that you should create empty structures for
  tutorials/howto guides/reference/explanation with nothing in them.
  Don't do that.")
- something we knew is lost in the move, other than the history this
  design drops on purpose
- an explanation or reference doc says something the code doesn't do
- privacy and security become a separate section bolted onto each doc
  instead of part of the topic
- a doc states its own scope, or the current state, in a way the next
  agent would read as a limit

## Approaches

We considered three layouts and chose the second.

1. **Topic folders, with Diátaxis as a lens.** `docs/design/` holds the
   explanation files, the release guide stays in `docs/release/`, and
   the index says which type each doc is. It moves the fewest files, but
   a reader can't tell from the layout what kind of doc they're opening,
   and the gaps stay hidden.
2. **Diátaxis folders, each created only when it has real content
   (chosen).** This reorganization creates `docs/explanation/`,
   `docs/reference/`, and `docs/how-to/`, each with content moved from
   where it lives now. It doesn't create `docs/tutorials/`, because we
   have no tutorial yet. The folder names show what kind of doc each is,
   which makes gaps easy to see, and it follows Diátaxis's advice to let
   structure grow from real content.
3. **All four sections at once, with placeholders.** Diátaxis rules this
   out, in the sentence quoted under "Anti-goals."

Effect on what we weigh: agents find the right type of doc and load
smaller files, which helps quality and size; the cost is one
reorganization; and privacy and security improve because each concern
sits where someone deciding it will read it, including a reference doc
that says who can read each file the daemon writes.

## The layout

```text
README.md                     what catalog is, its status, links into docs/
docs/
  README.md                   the index: what's here and where to start
  explanation/
    overview.md
    catalogs.md
    profiles.md
    architecture.md
  reference/
    commands.md
    configuration.md
    files.md
    catalog-format.md
  how-to/
    install-and-run.md
    move-from-the-earlier-tool.md
    set-up-for-development.md
    release.md
    apple-code-signing.md
  research/                   unchanged, plus mcp.md
  best-practices/             unchanged
  superpowers/                dated specs and plans, unchanged
```

`docs/research/` and `docs/best-practices/` hold our sources and the
people we learn from. `docs/superpowers/` holds dated working records:
each spec and plan describes one decision and how we carried it out, and
the explanation and reference docs hold the current picture. None of
these is one of Diátaxis's four types, and the index says so, so that a
reader or agent doesn't take a dated spec as a description of catalog
today.

## Where each piece goes

### README.md

The README keeps what catalog is (its mission, including that it never
shows anyone more than they could already see), what state it's in, and
a short "How it works" (each entry is a profile). It links to
`docs/README.md` and to the install guide. Its long sections move:

| Section now                               | Moves to                                                                  |
| ----------------------------------------- | ------------------------------------------------------------------------- |
| Running the daemon                        | `how-to/install-and-run.md`                                               |
| Which folders to watch                    | `how-to/install-and-run.md`, and the reasons to `explanation/catalogs.md` |
| Moving a repository from the earlier tool | `how-to/move-from-the-earlier-tool.md`                                    |
| Development                               | a short pointer to `how-to/set-up-for-development.md`                     |
| Releases, Making a release                | `how-to/release.md`                                                       |
| macOS Gatekeeper and code signing         | a line in `how-to/release.md` linking `how-to/apple-code-signing.md`      |

`SETUP.md` moves to `how-to/set-up-for-development.md`, and
`docs/release/apple-code-signing.md` moves to
`how-to/apple-code-signing.md`.

### docs/design/daemon.md

| Section now                           | Moves to                                                                                                                                                                   |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| What the catalog is for               | `explanation/overview.md`                                                                                                                                                  |
| Privacy and security                  | `explanation/overview.md`                                                                                                                                                  |
| Who it is for                         | `explanation/overview.md`                                                                                                                                                  |
| What hurts today                      | `explanation/overview.md`, as the problem catalog solves; the account of how we got there is dropped                                                                       |
| The experiment                        | split by topic: no state in Git to `overview.md`; one daemon per Mac and nested catalogs to `catalogs.md`; the model to `profiles.md`; watching files to `architecture.md` |
| No state in Git by default            | `explanation/overview.md`                                                                                                                                                  |
| Daemon state                          | `explanation/architecture.md`, under the Store                                                                                                                             |
| A side challenge: cloud drives        | `explanation/catalogs.md`                                                                                                                                                  |
| Our new struggle is wall-clock time   | `explanation/profiles.md`                                                                                                                                                  |
| Time vs quality vs cost               | `explanation/profiles.md`, including choosing a model and what a provider keeps                                                                                            |
| Profiles deepen over time             | `explanation/profiles.md`                                                                                                                                                  |
| More than documents                   | `explanation/profiles.md`                                                                                                                                                  |
| Where agents start                    | `explanation/catalogs.md`, including subfolders shared more narrowly                                                                                                       |
| Statistics for tuning                 | `explanation/architecture.md`, under the Journal                                                                                                                           |
| OS support                            | `explanation/architecture.md`, under platforms                                                                                                                             |
| Delivering the catalog without a file | `explanation/catalogs.md`                                                                                                                                                  |
| User interface                        | `explanation/architecture.md`, under clients                                                                                                                               |
| Open questions                        | GitHub issues, or the open questions of the doc each one shapes (see below)                                                                                                |
| Background                            | dropped; Git keeps the history                                                                                                                                             |

### docs/design/architecture.md

| Section now                                                  | Moves to                                                                                                              |
| ------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------- |
| The contract with agents                                     | `explanation/overview.md`, with the file's exact shape in `reference/catalog-format.md`                               |
| What a tree needs                                            | `explanation/catalogs.md`; the steps to put it in place in `how-to/install-and-run.md`                                |
| How work moves, Components, Rules between components         | `explanation/architecture.md`                                                                                         |
| Platforms and language                                       | `explanation/architecture.md`, with one sentence on what `internal/` means in Go                                      |
| Processes and macOS                                          | `explanation/architecture.md`                                                                                         |
| Delivering the catalog over MCP: What the documentation says | `research/mcp.md`                                                                                                     |
| Delivering the catalog over MCP: What we make of it          | `explanation/catalogs.md`                                                                                             |
| Left for implementation                                      | `explanation/architecture.md`, updated to what was decided in the build; items still open become open questions there |
| Open questions                                               | GitHub issues, or the open questions of the doc each one shapes                                                       |

### Reference docs

Each reference doc describes the product as the code and the first
daemon's spec define it. Diátaxis says reference material "is led by the
product it describes" and "should be austere," and that "One hardly
reads reference material; one consults it."

- `reference/commands.md`: `catalog daemon`, `catalog daemon --once`,
  and `catalog status`, with their arguments, output, and exit codes,
  from `cmd/catalog/main.go`.
- `reference/configuration.md`: every key in `global.toml` and in a
  catalog's `.catalog/config.toml`, with defaults, how a catalog's
  settings override the global ones, and what a catalog's config may
  never hold.
- `reference/files.md`: every file and socket the daemon reads or
  writes, where it lives, its mode, who can read it, and what it
  contains. This holds the facts now in the first daemon's spec under
  "Who can read what the daemon keeps."
- `reference/catalog-format.md`: the shape of `.catalog/catalog.md` that
  agents read, from the Renderer.

### Privacy and security

`explanation/overview.md` keeps what applies to all of catalog: the rule
that it never shows anyone more than they could already see, whose
information it protects, the questions to ask at every step, and the
goals we keep working toward. Everything tied to one topic moves into
that topic: what a provider keeps and trains on into `profiles.md`,
subfolders shared more narrowly and synced or committed catalogs into
`catalogs.md`, each component's copies and who may read them into
`architecture.md`, and each file's location and mode into
`reference/files.md`. Links to `daemon.md#privacy-and-security` point to
the matching section of `overview.md`. (Our call.)

### What's built and what isn't

The builder decides what's built by reading the code and the first
daemon's spec. Design that isn't built yet, such as MCP delivery, a menu
bar item, deeper profiles, or Linux, stays in the explanation doc it
belongs to, described as direction we intend and marked plainly as not
built. It isn't dropped, and it isn't turned into open questions unless
it really is undecided. (Our call: the explanation docs are where the
reasons for that direction live.)

### Open questions

Each open question either shapes one doc today, and stays in that doc's
open questions, or is work to do, and becomes a GitHub issue. A doc gets
an "Open questions" section only if at least one question stays in it.

The builder sorts the questions first, before rewriting the docs: it
reads the open issues so as not to duplicate one (several, such as
profiling source code or sizing entries, may already be open), and shows
the developer the list of issues to file and the questions that stay.
Nothing is posted without the developer's approval, since the repository
is public. The docs are written with the questions in place, and the
approved issues are filed at the end. (Our call on the order.)

### Links

The builder updates every link to a moved file or section, in the
README, CLAUDE.md, the research notes, and the dated specs and plans, so
that every link still resolves. Where an old section's content now lives
in several places, the link points to the place that holds its main
point. In the dated specs and plans, only the links change. (Our call: a
broken link in a record helps nobody, and changing only the link leaves
the record's content as it was.) `make markdown` doesn't follow links
between files, so the builder checks every relative link and anchor with
a throwaway script before each commit and doesn't add it to the
repository. (Our call.)

Text in GitHub issues stays as it is, including any that mention the
earlier tool's `.catalog.md`.

### What else stays

The README's "How it works" stays as it is. `docs/design/`,
`docs/release/`, and `SETUP.md` are deleted once their content has
moved.

## Declaring Diátaxis

- `docs/README.md` introduces the documentation itself. It opens by
  telling a reader what's here and where to go for what they need, such
  as getting started, looking something up, or understanding why catalog
  works the way it does, and then lists each section and its docs.
  Diátaxis comes last, in a sentence or two saying that's how we
  organize the docs and naming the types we don't have yet, such as
  tutorials, without creating folders for them.
- CLAUDE.md gains a short rule: place each new doc by its Diátaxis type,
  and when you add or change a feature, check which of the four types it
  still needs. It links to
  [craftspeople.md](../../best-practices/craftspeople.md), which already
  names Daniele Procida and Diátaxis.
- writing-style.md stays where it is; CLAUDE.md imports it.

## Constraints

- Follow [writing-style.md](../../../writing-style.md) and run
  `make markdown` after every edit.
- Check every statement in an explanation or reference doc against the
  code or the first daemon's spec before writing it. Reference docs
  describe what the code does, taking each key, command, path, and mode
  from the code itself (`cmd/catalog`, `internal/settings`, and wherever
  files and sockets are created). Where the code and the spec disagree,
  the builder documents the code and tells the developer about the
  difference. If the difference is a privacy or security fault, such as
  a file readable by more people than the spec says, the builder stops
  and reports it to the developer instead of documenting the fault as
  intended behavior.
- Keep examples made up, and keep client work and the company's internal
  information out.
- Name a model provider only where the text describes a version that
  uses it.

## Sensitive data in this change

The change moves and rewrites documentation. It reads none of the
documents catalog serves and sends nothing anywhere. Its risks are that
new text carries client work or internal information, which the builder
avoids by making up every example and each reviewer checks for; and that
`reference/files.md` gets a file's mode or location wrong, which would
mislead someone checking who can read their data, so the builder takes
each mode and path from the code.

## Left to the builder

- the wording of each new doc, and the order of sections inside it
- the order of the work, as long as each commit leaves every link
  working
- whether `reference/catalog-format.md` quotes a rendered example or
  describes the shape in prose
