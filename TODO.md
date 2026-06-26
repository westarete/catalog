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
- [ ] Update the catalog skill in `hq` (SKILL.md, SETUP.md,
      config.example.toml, pre-commit)

## 2. SQLite profile store

Migrate profiles out of the flat `.catalog.md` into a committed SQLite
database. The Markdown file becomes a generated projection of the store.

- [ ] Design the schema
- [ ] Implement `readCatalog` and `writeCatalog` against SQLite
- [ ] Regenerate `.catalog.md` from the store on every write
- [ ] Update `catalog check` to verify store and Markdown are consistent

## Minor

- [ ] Size warning in `catalog check` when entry count crosses ~150
