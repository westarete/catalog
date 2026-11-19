# Documentation Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move catalog's documentation into explanation, reference, and
how-to docs that each serve one reader's need, as the design describes.

**Architecture:** New docs are written first, beside the old ones, so no
link breaks along the way. Then the how-to task moves the README's setup
sections and `SETUP.md`, and the last task retires `docs/design/`,
repoints every link, trims the README, and adds the index and the
CLAUDE.md rule. Each task ends in one commit.

**Tech Stack:** Markdown, formatted and checked by `make markdown`; Go
source read for reference docs.

**Spec:**
[docs/superpowers/specs/2026-11-19-documentation-layout-design.md](../specs/2026-11-19-documentation-layout-design.md)

## Global Constraints

- Read the spec in full before any task. Its tables under "Where each
  piece goes" say where every section of the README,
  `docs/design/daemon.md`, and `docs/design/architecture.md` lands; its
  sections "Privacy and security," "What's built and what isn't," "Open
  questions," "Links," and "What else stays" settle the cases those
  tables don't.
- Load `knowledge-work:markdown` before editing any `.md` file, and run
  `make markdown` after every edit. Write each paragraph as one long
  line.
- Follow `writing-style.md`. Open every doc with complete sentences that
  tell the reader what they need.
- Explanation docs describe catalog as built, in the present tense, and
  mark design that isn't built as intended direction. Check each
  statement against the code or
  `docs/superpowers/specs/2026-11-15-daemon-mvp-design.md`.
- Reference docs describe only what the code does, taking each key,
  command, path, and mode from the code. Where code and spec disagree,
  document the code and report the difference. If the difference is a
  privacy or security fault, stop and report it instead of documenting
  it.
- Carry over what the old docs say, including sources and quotations
  exactly as they are; drop only the history the spec drops
  ("Background," and the account of how we got here in "What hurts
  today").
- Privacy and security go inside the topics they bear on, as the spec's
  "Privacy and security" section assigns them; no doc gets a separate
  privacy section bolted on.
- Create no folder or section without real content in it. No
  `docs/tutorials/`.
- Keep examples made up, keep client work and the company's internal
  information out, and name a model provider only where the text
  describes a version that uses it.
- Before every commit, run `make markdown` and `make test`, and check
  that every relative link and `#anchor` in every tracked `.md` file
  resolves (GitHub's heading slugs). Use a throwaway script kept outside
  the repository. Commit messages follow Tim Pope's style with no
  attribution trailers.

## Review Focus

Copy these into every task reviewer's brief.

- **Lost content:** every section the spec's tables assign to this task
  appears in the new doc, with its sources and quotations intact.
- **Accuracy:** no explanation or reference statement says something the
  code doesn't do; anything not built is marked as not built.
- **Privacy and security:** no protection the docs state is weakened or
  dropped (file modes, the API key out of logs, a catalog's
  `config.toml` never naming a server or holding a credential, the rule
  that nobody sees more than they already could), and
  `reference/files.md` gives each file's real mode and readers.
- **Fences and bolt-ons:** no sentence reads as a limit on where privacy
  and security belong, and they aren't split off into a separate
  section.
- **Links:** every relative link and anchor resolves after the commit.
- **Client and internal information:** none in the new text or the
  commit message.

---

### Task 1: Sort the open questions

**Files:**

- Create: `open-questions.md` in the plan's workspace (not in the
  repository)

**Interfaces:**

- Produces: `open-questions.md`, listing each open question from
  daemon.md's "Open questions," architecture.md's "Open questions" and
  "Left for implementation," as either "stays in `<doc>`" or "issue,"
  with the matching open issue number when one exists, and a draft title
  and body for each new issue. Tasks 2 to 5 use it to decide which
  questions each explanation doc keeps; the controller shows the issue
  list to the developer at the end.

- [ ] **Step 1:** Read `gh issue list --state open` and each issue that
      might match a question.
- [ ] **Step 2:** For each item in "Left for implementation," check the
      code: if the build decided it, record what was decided for Task 5
      instead of treating it as a question.
- [ ] **Step 3:** Write `open-questions.md`. File nothing.

### Task 2: explanation/overview.md

**Files:**

- Create: `docs/explanation/overview.md`

**Interfaces:**

- Consumes: `open-questions.md` from Task 1.
- Produces: `docs/explanation/overview.md` with a
  `## Privacy and security` heading, which Task 9 points the old
  `daemon.md#privacy-and-security` links to.

- [ ] **Step 1:** Write the overview from the rows the spec assigns it:
      what the catalog is for, privacy and security (the rule, whose
      information it protects, the three questions, the goals), who it
      is for, the problem catalog solves, no state in Git, and the
      contract with agents.
- [ ] **Step 2:** Run `make markdown` and the link check.
- [ ] **Step 3:** Commit.

### Task 3: explanation/catalogs.md and research/mcp.md

**Files:**

- Create: `docs/explanation/catalogs.md`, `docs/research/mcp.md`

**Interfaces:**

- Consumes: `open-questions.md` from Task 1.

- [ ] **Step 1:** Write `research/mcp.md` from architecture.md's "What
      the documentation says," with its sources and quotations as they
      are, in the shape of the other research notes.
- [ ] **Step 2:** Write `catalogs.md` from the rows the spec assigns it:
      where agents start (with subfolders shared more narrowly, and
      committed or synced catalogs), what a tree needs, nesting, one
      daemon per Mac, cloud drives, the reasons to watch only unsynced
      folders, and delivering the catalog without a file (with "What we
      make of it," linking `research/mcp.md`).
- [ ] **Step 3:** Run `make markdown` and the link check.
- [ ] **Step 4:** Commit.

### Task 4: explanation/profiles.md

**Files:**

- Create: `docs/explanation/profiles.md`

**Interfaces:**

- Consumes: `open-questions.md` from Task 1.

- [ ] **Step 1:** Write `profiles.md` from the rows the spec assigns it:
      what a profile is, wall-clock time, time against quality and cost
      (with choosing a model and what a provider keeps and trains on),
      how profiles deepen, the second pass and pointers between files as
      the first daemon's spec describes them, and more than documents.
- [ ] **Step 2:** Run `make markdown` and the link check.
- [ ] **Step 3:** Commit.

### Task 5: explanation/architecture.md

**Files:**

- Create: `docs/explanation/architecture.md`

**Interfaces:**

- Consumes: `open-questions.md` from Task 1, including what the build
  decided for "Left for implementation."

- [ ] **Step 1:** Write `architecture.md` from the rows the spec assigns
      it: how work moves, each component (with what it copies and who
      may read it), rules between components, daemon state, the Journal
      and statistics, clients and user interface, platforms and language
      (with one sentence saying `internal/` is Go's convention for
      packages only this module can import, citing
      `go help importpath`), and processes on macOS. Describe the one
      loop, the crawl every minute, and SQLite as built, and FSEvents
      and the other unbuilt parts as direction.
- [ ] **Step 2:** Run `make markdown` and the link check.
- [ ] **Step 3:** Commit.

### Task 6: reference/commands.md and reference/configuration.md

**Files:**

- Create: `docs/reference/commands.md`,
  `docs/reference/configuration.md`

- [ ] **Step 1:** From `cmd/catalog/main.go`, write `commands.md`: each
      command and flag, what it prints, and its exit codes.
- [ ] **Step 2:** From `internal/settings` and wherever settings are
      read, write `configuration.md`: where `global.toml` lives, every
      key in it and in `.catalog/config.toml` with its type and default,
      how a catalog's settings override the global ones, how soon edits
      take effect, and what a catalog's config may never hold.
- [ ] **Step 3:** Run `make markdown` and the link check.
- [ ] **Step 4:** Commit.

### Task 7: reference/files.md and reference/catalog-format.md

**Files:**

- Create: `docs/reference/files.md`, `docs/reference/catalog-format.md`

- [ ] **Step 1:** From the code that creates each file and socket
      (`internal/store`, `internal/journal`, `internal/publish`,
      `internal/control`, `internal/daemon`, and
      `launchd/com.westarete.catalog.plist`), write `files.md`: each
      file the daemon reads or writes, where it lives, its mode, who can
      read it, and what it holds. If a mode in the code allows more
      readers than the first daemon's spec says, stop and report it.
- [ ] **Step 2:** From `internal/render`, write `catalog-format.md`: the
      shape of `.catalog/catalog.md` an agent reads, including the
      header and how a listing-only entry looks.
- [ ] **Step 3:** Run `make markdown` and the link check.
- [ ] **Step 4:** Commit.

### Task 8: how-to guides

**Files:**

- Create: `docs/how-to/install-and-run.md`,
  `docs/how-to/move-from-the-earlier-tool.md`, `docs/how-to/release.md`
- Move: `SETUP.md` to `docs/how-to/set-up-for-development.md`;
  `docs/release/apple-code-signing.md` to
  `docs/how-to/apple-code-signing.md` (with `git mv`)
- Modify: `README.md`, and every file that links to a moved file or
  section

- [ ] **Step 1:** Move the README's "Running the daemon" and "Which
      folders to watch" into `install-and-run.md`, adding the steps that
      put a catalog in place from architecture.md's "What a tree needs."
      Link `explanation/catalogs.md` for why to watch only unsynced
      folders.
- [ ] **Step 2:** Move the README's "Moving a repository from the
      earlier tool" into `move-from-the-earlier-tool.md`, and
      "Releases," "Making a release," and the code-signing paragraph
      into `release.md`, which links `apple-code-signing.md`.
- [ ] **Step 3:** `git mv` the two existing guides, and fix the relative
      links inside them.
- [ ] **Step 4:** Replace each moved README section with a one-line
      pointer, and repoint links to the moved files and sections,
      including CLAUDE.md's "Follow the release process in README.md."
- [ ] **Step 5:** Run `make markdown` and the link check.
- [ ] **Step 6:** Commit.

### Task 9: Retire docs/design, index, README, and CLAUDE.md

**Files:**

- Create: `docs/README.md`
- Delete: `docs/design/daemon.md`, `docs/design/architecture.md`
- Modify: `README.md`, `CLAUDE.md`, and every file that links into
  `docs/design/`

**Interfaces:**

- Consumes: the files from Tasks 2 to 8.

- [ ] **Step 1:** Check that every row of the spec's two `docs/design/`
      tables has landed in its new home. Report any that hasn't instead
      of deleting.
- [ ] **Step 2:** Repoint every link into `docs/design/`, in the README,
      CLAUDE.md, the research notes, and the dated specs and plans, to
      the place that holds the section's main point. In dated specs and
      plans, change only links.
- [ ] **Step 3:** Delete the two design docs with `git rm`.
- [ ] **Step 4:** Rewrite the top of `README.md`: what catalog is,
      including that it never shows anyone more than they could already
      see, its status, "How it works" as it is, and links to
      `docs/README.md` and `docs/how-to/install-and-run.md`.
- [ ] **Step 5:** Write `docs/README.md` as the spec's "Declaring
      Diátaxis" describes: what's here and where to start, each section
      and its docs, and at the end a sentence or two that we organize
      the docs with Diátaxis (linking it and
      `docs/best-practices/craftspeople.md`), naming tutorials as a type
      we don't have yet. Describe `research/`, `best-practices/`, and
      `superpowers/`, saying that dated specs and plans are records and
      the explanation and reference docs are the current picture.
- [ ] **Step 6:** Load `knowledge-work:maintaining-agent-instructions`
      and add the CLAUDE.md rule: place each new doc by its Diátaxis
      type, and when adding or changing a feature, check which of the
      four types it still needs. Link `docs/README.md`.
- [ ] **Step 7:** Run `make markdown`, `make test`, and the link check.
- [ ] **Step 8:** Commit.

### After the final review

The controller shows the developer the issue list from
`open-questions.md` and files only the issues they approve, with
`gh issue create`.
