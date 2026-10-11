# Short-term development plan for Catalog

This file lists work that hasn't happened yet. Items are shared work for
the developer and the agent, and many are conversations to have rather
than tasks to finish. See the maintaining-plans skill for the
conventions.

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
- [x] Improve the "no config" error a brand-new repo hits on every
      command. Today (`config.go`'s `loadConfig`) it just says "copy
      config.example.toml from the catalog skill and edit it" — no path
      to that file, no mention that `catalog init` (SETUP.md's planned
      one-command setup) doesn't exist yet either. A person with no
      prior context has nowhere to go from this message. At minimum,
      name where the skill actually lives; ideally, this is the moment
      `catalog init` should exist to walk someone through setup instead
      of just refusing.
