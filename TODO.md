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

## Someday

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
- [ ] Improve the "no config" error a brand-new repo hits on every
      command. Today (`config.go`'s `loadConfig`) it just says "copy
      config.example.toml from the catalog skill and edit it" — no path
      to that file, no mention that `catalog init` (SETUP.md's planned
      one-command setup) doesn't exist yet either. A person with no
      prior context has nowhere to go from this message. At minimum,
      name where the skill actually lives; ideally, this is the moment
      `catalog init` should exist to walk someone through setup instead
      of just refusing.
