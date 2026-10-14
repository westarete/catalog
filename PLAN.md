# Short-term development plan for Catalog

This file lists work that hasn't happened yet. Items are shared work for
the developer and the agent, and many are conversations to have rather
than tasks to finish. See the maintaining-plans skill for the
conventions.

## Prompt caching

- [ ] Cache the part of each profiling request that repeats, so a pass
      over many documents stops paying full price for it on every call.
      Today `inferProfile` sets no `cache_control`, so nothing is
      cached, which is why Anthropic warns about the cache rate during
      catalog runs. The repeated part is the instruction (about 600
      tokens) plus the neighbor profiles, which are every other
      document's profile and so grow to roughly the size of the whole
      `.catalog.md`. Two things keep the neighbors from being cacheable
      now: they come after the document in the request instead of before
      it, and they change on every call, because `neighbors` leaves out
      the document being profiled and `inferPass` writes each new
      profile before the next call. Decide whether the model still
      positions a document well when it sees a snapshot of the neighbors
      taken at the start of the pass and placed ahead of the document,
      then mark the end of that shared prefix for caching. Haiku 5.5
      caches any prefix of 512 tokens or more, reads cost a tenth of the
      input price, and writes cost 1.25 times it, per
      [Anthropic's prompt caching docs](https://platform.claude.com/docs/en/build-with-claude/prompt-caching).
      Confirm the change in the run's cost report, which already shows
      cached tokens, and check that an `update` touching one or two
      documents doesn't end up costing more because of the write
      premium.

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
