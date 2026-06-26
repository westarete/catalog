# CLAUDE.md

## What this file is

This file governs agent behavior in this repository: communication
habits, process discipline, Markdown quality, and Git hygiene.

## Do not use the agent memory system

Do not write to or rely on a per-machine agent memory store (for
example, a `memory/` directory or a `MEMORY.md` index outside this
repo). Memory written on one machine does not exist on another, so a
habit saved there silently fails to apply elsewhere and creates
machine-specific behavior the developer cannot see or review.

Durable lessons belong **in this repository** instead — in this file, or
in the relevant skill — where they are version-controlled, reviewable in
a diff, and present on every machine that checks out the repo. If you
catch yourself wanting to "remember" something for next time, propose
the edit to this file or a skill in the same reply.

## Working with the developer

They care about clear writing, clean history, and habits that survive
the next conversation. Prefer small, verifiable steps; call out
trade-offs when a choice matters; avoid expanding scope beyond what they
asked.

Near-term work in progress is tracked in [TODO.md](TODO.md). Do not
check off items — that's the developer's job after verifying the work.
Bundle the checkbox update in the same commit that completes the work.

When pointing at web pages or repo files, use **clickable Markdown
links** (e.g. `[README](README.md)`), not bare URLs in backticks or
bold, so nothing requires copy-paste.

**Speak plainly. Avoid throwaway metaphors from unrelated domains.**
Phrases like "land it," "load-bearing," "in flight," "lift," "moving the
needle," "north star," "heavy lift," or other shallow aviation,
construction, or sports analogies are annoying when a direct verb will
do. Say "commit it," "essential," "in progress," "effort," "changes the
result," "goal," "hard," etc. This applies to chat as much as to docs.

## Writing style

This is a small team, not a corporation. Match that voice everywhere you
write — README sections, CLAUDE.md updates, replies in chat. The test
isn't whether the writing is correct; it's whether a colleague would
want to read it.

Some specific habits:

- **Don't write for agents when humans will read it.** Phrases like
  "authoritative source," "deeper procedural guidance," "in full before
  acting on its topic," or "pointers, not replacements" read like a
  system prompt. Cut them.
- **Don't reach for "canonical."** It disambiguates one version from
  another; use it only when you're actually doing that. If the
  surrounding sentence already explains what makes the document the one
  we care about, leave the word out.
- **Drop redundant qualifiers.** If the next clause does the work, the
  adjective doesn't.
- **Say what something is for, not its policy status.** "For more
  guidance" beats "for the full policy."
- **Skip forward references the reader will hit on their own.**
- **Don't bold link text just because it's a link.** The link styling is
  already enough.
- **Use the vocabulary the doc has already defined.**

## No willpower promises

If you catch yourself wanting to say "I'll keep that in mind going
forward" or "I'll be more careful next time" — stop. That promise won't
survive a new conversation, and the developer knows it.

The fix is to capture the lesson here, in this file (or in the relevant
skill), so the next session starts with it loaded. If the edit is small,
propose it in the same reply where you'd otherwise apologize. If it's
larger, ask whether it's worth writing down.

## Conversation startup

At the start of every conversation, read README.md and this file before
doing substantive work.

## Config files

When creating config files (`.prettierrc.yaml`, `.markdownlint.yaml`,
`.markdownlint-cli2.yaml`, and similar), copy them from `hq` rather than
writing minimal stubs. The comments explain why each setting exists and
are part of the convention — don't strip them.

## Skills

Load these two at the start of every conversation, before doing any work
that might touch them:

- [markdown](skill/SKILL.md) — read before editing any `.md` file. Until
  that file exists, load the harness skill `markdown` instead.
- [git-commit](skill/git-commit/SKILL.md) — read before running
  `git commit` on the developer's behalf. Until that file exists, load
  the harness skill `git-commit` instead.

## Running commands

Assume the **current working directory is this repository's root**.
Invoke tools relative to that root, the same way a developer would in a
terminal opened at the repo.

- Prefer `make markdown`, `make test`, `make lint`, `git status`,
  `git commit`, etc. — not absolute paths.
- Do not use `cd` in the command string when you can avoid it.

**Never run `sudo` or elevated-privilege commands.** If something truly
needs that, describe the issue and stop.

## Markdown

After editing any `.md` file, run `make markdown`. It formats the whole
repo with Prettier, then lints the whole repo with markdownlint-cli2 —
always the full repo, never scoped to specific files. Write long
single-line paragraphs and let Prettier wrap. Do not hand-format, and do
not re-edit a file to undo what the formatter did. Install the tools
once with `brew install prettier markdownlint-cli2`.

## Git workflow

The hard rule: **never run `git commit` until the developer has approved
the exact message you proposed in chat.** "Commit when you're done" is
not pre-approval of a message. Commit messages follow Tim Pope's
[A note about Git commit messages](https://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html).

Follow the release process in [README.md](README.md).

For the git-commit skill, the project's CHECK command is `make markdown`
and there is no separate FORMAT command — `make markdown` both formats
and checks. Run it in Phase 2 before proposing a commit message, every
time.

### When process fails

If a step in the skill is skipped (e.g. commit without `make markdown`,
or without message approval), treat it as a **process** problem: fix the
instructions or tooling in-repo, not "try harder next time."
Willpower-based promises are not fixes.

## Code

This is a code-centric repo. Hold it to the same standard as the
writing.

### Tests

Write tests at every layer: unit, integration, and system. Aim for
maximum sensible coverage — not 100% for its own sake, but enough that a
regression is caught before it reaches the developer. When adding a
feature or fixing a bug, tests come with it, not after.

### API references and dependencies

Never write code against an API or library from memory. Look up the
current documentation before using any external package, standard
library function, or CLI flag. Check that the dependency version in use
matches what the docs describe. A confident-sounding answer based on
stale knowledge is worse than admitting uncertainty.

This applies to GitHub Actions versions too. Never suggest upgrading or
pinning an action to a version without first looking up the current
release on GitHub. Don't guess and don't pass the buck — check the
releases page, report what you found, then make the change.

### Clean code

Follow Robert Martin's definitions of clean code. Functions do one
thing. Names say what they mean. Side effects are explicit. Abstractions
match the problem domain, not the implementation. When something feels
awkward to name or test, that is usually a design problem, not a naming
problem.

### Work incrementally

Do not generate large amounts of code in one move. Propose a step,
confirm it makes sense, then write it. A slow developer who ships
correct code is better than a fast one who ships something to debug.
When the next step is unclear, say so rather than filling the space with
plausible-looking code.

## Principles (short)

- Prefer clarity over decoration-heavy Markdown (excessive bold, etc.).
- Update the doc that a reader would naturally open — usually the same
  commit as the idea, not a delayed pass.
- When recommending tools or practices, sanity-check assumptions; say
  when something is uncertain.
