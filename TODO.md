# Short-term development plan for Catalog

This file tracks near-term work in progress. A few conventions:

- Work is grouped into phases. Each phase is a section below.
- Check off items in the same commit that completes the work, so the
  history and the checklist stay in sync.
- Do not check an item off until a human has tested and verified it. Ask
  them to do so, and then only check it off after they've confirmed that
  it's satisfactory.
- This is not a historical record. We will clear completed phases out
  periodically so the file stays useful as a view of what's actually in
  progress.

## 1. Rename `CATALOG.md` to `.catalog.md`

The dotfile convention signals "tooling artifact" rather than "document
you open." As the catalog grows into a multi-level structure, the
convention becomes more valuable. Migration cost is low. For why the
catalog stays a root-level dotfile rather than moving inside
`.catalog/`, see [FUTURE.md](FUTURE.md).

- [x] Update the hardcoded filename in `catalog.go`
- [x] Update all references in the Go source, README.md, and FUTURE.md
- [x] Update the catalog skill in `hq` (SKILL.md, SETUP.md,
      config.example.toml, pre-commit)

## 2. SQLite profile store

Migrate profiles out of the flat `.catalog.md` into a committed SQLite
database at `.catalog/catalog.db`. The database is the sole source of
truth; `.catalog.md` is always a generated artifact rendered from it.
Design is recorded in [FUTURE.md](FUTURE.md)'s "The profile store"
section — read that before starting.

Each step below should land with its own test before moving to the next
one, rather than building the whole feature and testing at the end. Each
step is one commit, in the order listed — the order is a dependency
order, not just a reading order, so don't reorder or squash across
steps.

- [ ] Add the `modernc.org/sqlite` dependency (pure Go, no cgo — matches
      `.goreleaser.yaml`'s `CGO_ENABLED=0`) together with the schema
      (`path`, `content_hash`, `profile`) and a function to
      open/initialize `.catalog/catalog.db` — `go mod tidy` drops an
      imported-but-unused dependency, so the `go get` and its first real
      caller have to land in the same commit. Test: opening a fresh path
      creates the table; opening an existing populated one leaves its
      rows alone; opening a corrupt or non-SQLite file at that path
      returns a clear error rather than silently treating it as missing
      and rebuilding over it.
- [ ] Add a content-hash function (SHA-256) for document text. Test:
      identical content hashes identically, different content hashes
      differently.
- [ ] Add a store read (fetch all rows), a store write (upsert one row
      by path), and a store delete (remove one row by path — pulled in
      early since `update`'s "drop rows for deleted docs" job needs it
      and it belongs with the other store operations). Test all three
      against a real temp-file database — no mocking needed, since the
      driver is pure Go.
- [ ] Add a pure classification function: given the enumerated docs'
      current hashes and the store's rows, return which are new,
      modified, or deleted. Test with table-driven cases, no I/O and no
      real store.
- [ ] Adapt `render` to build `.catalog.md` from store rows instead of
      the parsed `*catalog` struct. Carry over the existing render
      tests, adjusted for the new input shape.
- [ ] Wire `bootstrap`, `update`, and `force` against the store. Landed
      as one commit rather than three — they share `runGenerate`,
      `inferPass`, `neighbors`, `hashDocs`, and `requirePopulated`,
      which all had to change shape together; splitting them wouldn't
      have produced independently buildable commits. `bootstrap` always
      fully rebuilds, ignoring any existing `.catalog.md` entirely.
      `update` re-infers only new/modified docs, drops rows for deleted
      docs, and always rewrites `.catalog.md`. `force` re-infers named
      docs (or all, with no args) unconditionally. Integration tests
      exercise the store/classify/render pipeline directly (real
      temp-file database, no API key) rather than the commands end to
      end, since `runGenerate` needs a real key before it reaches the
      network — see CLAUDE.md and FUTURE.md on keeping the model-calling
      path out of automated tests.
- [ ] Add `catalog status`: run the classification function and report
      new/modified/deleted in `git status`-style language (oriented
      around the user's files, not the catalog's bookkeeping), plus a
      separate check for whether `.catalog.md` on disk matches a fresh
      render of the store. Non-zero exit if anything is out of sync. No
      model call. Test each category independently.
- [ ] Add `catalog diff`: render the store, unified-diff it against
      `.catalog.md` on disk. Pick a Go unified-diff library and test
      against known before/after string pairs.
- [ ] Delete `git.go` and `git_test.go`; remove the git calls from
      `check.go` (renamed `status.go`) and `generate.go`. `catalog` no
      longer depends on Git or on running inside a repository. Must come
      after `force` is rewired (the previous step) — `force` is the last
      caller of the git-based staleness functions, so deleting them any
      earlier breaks the build.
- [ ] Update README.md's Commands section to describe `status`, `diff`,
      and hash-based staleness in place of the current Git-based
      description.
- [ ] Update the `hq` catalog skill. This lives in a separate repo
      (`claude-plugins-internal`,
      `plugins/knowledge-work/skills/catalog/`), so it's its own PR, not
      part of this repo's commits. Specific changes needed, found by
      reading the current files:
  - `pre-commit` — calls `catalog check` (line 18); change to
    `catalog status`.
  - `SKILL.md`'s whole "Staleness is a Git query" section describes the
    Git-based mechanism directly (reference commit, working-tree diff,
    subtree exemption) — rewrite around hash-based staleness with no Git
    dependency. Say plainly that `catalog` doesn't need to run inside a
    Git repository at all now.
  - `SKILL.md`'s "Commands" section describes `check` reading Git and
    reports orphans/stale/missing in that vocabulary — rewrite for
    `status` (new/modified/deleted) and mention `diff`.
  - `SKILL.md`'s "CI" section says `fetch-depth: 0` is required because
    `check` needs Git history — no longer true; a shallow or even
    single-commit checkout is fine since staleness never reads history.
    This is also the fix for the pre-commit-hook catch-22 with
    never-committed files: staleness no longer depends on commit state
    at all, so that failure mode is gone, not worked around.
  - `SETUP.md` step 4 ("Register the git hook") and step 5 ("Add CI")
    both reference `catalog check` — update to `catalog status`. Step
    5's `fetch-depth: 0` note is the same stale claim as above.
  - `config.example.toml` doesn't reference Git or the old filename —
    check it still needs no change once read again at update time.

## Minor

- [ ] Size warning in `catalog status` when entry count crosses ~150
- [ ] `catalog edit <path>` — let a person set the profile text for a
      document directly, overriding the model's inference. The override
      holds only against the content hash it was set for: once that
      document's content changes, the override no longer applies and
      `update`/`force`/`bootstrap` regenerate the profile from the new
      content, same as any other stale entry.
- [ ] `catalog resolve` — after a merge conflict on
      `.catalog/catalog.db`, ask which side to keep (ours/theirs), check
      out that side, then run `update`. Saves remembering
      `git checkout --theirs/--ours` syntax mid-merge. The manual
      two-step path (check out a side, then run `catalog update`) always
      works, so this is a convenience, not a blocker.
