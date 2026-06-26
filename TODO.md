# Short-term development plan for Catalog

## 1. Rename `CATALOG.md` to `.catalog.md`

The dotfile convention signals "tooling artifact" rather than "document
you open." As the catalog grows into a multi-level structure, the
convention becomes more valuable. Migration cost is low.

- [ ] Update the hardcoded filename in `catalog.go`
- [ ] Update all references in the Go source, README.md, and FUTURE.md
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
