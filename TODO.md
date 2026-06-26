# Short-term development plan for Catalog

## 1. Rename `CATALOG.md` to `.catalog.md`

The dotfile convention signals "tooling artifact" rather than "document
you open." As the catalog grows into a multi-level structure, the
convention becomes more valuable. Migration cost is low.

- [ ] Update the hardcoded filename in `catalog.go`
- [ ] Update all references in the Go source, README.md, and FUTURE.md
- [ ] Update the catalog skill in `hq` (SKILL.md, SETUP.md,
      config.example.toml, pre-commit)

## 2. Prompt tweak for sibling weighting

The catalog is already hierarchically structured by directory, so the
model can weight same-directory entries without any code changes — just
a prompt hint.

- [ ] Update the inference prompt to draw the model's attention to
      entries in the same directory as the file being profiled

## 3. Size warning in `catalog check`

When the catalog grows past a threshold, `check` should warn so "the
index feels too big" becomes a measured signal rather than a gut
feeling.

- [ ] Decide on a threshold (roughly 150 entries as a starting point)
- [ ] Add the warning to `check` output (not a failure, just a warning)

## 4. README context when inferring profiles

When profiling a file, include the `README.md` from the same directory
if one exists. This helps with opaque files (CSVs, legacy archives) that
don't describe themselves well.

- [ ] When inferring a profile, check for a `README.md` sibling and
      include its content in the prompt context
- [ ] Skip it if the `README.md` is itself an enumerated document
      (already visible as a catalog neighbor)
