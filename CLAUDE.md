# CLAUDE.md

## What this file is

This file governs agent behavior in this repository: communication
habits, process discipline, Markdown quality, and Git hygiene.

This file is instructions, not documentation. Use
[Anthropic's best practices for skills](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices)
for guidance on how to get good results using file.

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

Work still to do is tracked in two places: [PLAN.md](PLAN.md), following
the maintaining-plans skill, and
[GitHub issues](https://github.com/westarete/catalog/issues), where new
bugs and requests go. In the plan, do not check off an item until the
developer agrees it is done. Bundle the checkbox update in the same
commit that completes the work, and delete a section once every item in
it is checked.

**Don't let existing work decide a choice.** Judge a language, library,
design, or name by what the finished product needs. Code, branches, or
files that already exist are not an argument for themselves; the
developer will throw them away when convention or the product calls for
it. If reuse genuinely matters, give it as a separate, named trade-off,
not as the reason.

## Writing style

@writing-style.md

## Reread before you show

Before you show anyone new writing that other people will read, reread
it twice: once as a reader who doesn't know what you were thinking, and
once as the critic who will judge it. Fix every passage either one would
stumble on or reject.

## Check a claim before you make it

This applies to chat as much as to files. A design here once went into
two files with citations nobody had read; half of them didn't say what
the text claimed.

- A search result's summary is a lead, not a source, and so is a
  subagent's report. Before you cite a page, read it, or have it read
  and get back the exact sentence, and confirm it says what you're about
  to write.
- Never state a guess about how a product behaves (Cowork, Claude Code,
  macOS, Docker) as a fact or as a doubt. Look it up in its
  documentation first. If the documentation is silent, say "not
  documented" and stop there.
- Before you write that something can't work or must work, name the
  concrete scenario and walk through it once. If you can't name one,
  don't write the sentence.
- Keep what a source says apart from what you conclude from it, and
  label the second as reasoning. Say what you haven't tested.

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

Before planning or starting a bug fix or feature, read PLAN.md and the
open GitHub issues, so you don't duplicate or contradict work tracked in
either place. Use `gh issue list`; where `gh` isn't available, as in
Cowork, read the
[issues page](https://github.com/westarete/catalog/issues) instead.

## Config files

When creating config files (`.prettierrc.yaml`, `.markdownlint.yaml`,
`.markdownlint-cli2.yaml`, and similar), copy them from the
`knowledge-work` plugin (the markdown skill's `config/` directory)
rather than writing minimal stubs. The comments explain why each setting
exists and are part of the convention — don't strip them.

## Skills

Load these two at the start of every conversation, before doing any work
that might touch them:

- `knowledge-work:markdown` — load before editing any `.md` file.
- `knowledge-work:git-commit` — load before running `git commit` on the
  developer's behalf.

Both come from the `knowledge-work` plugin in the
[claude-plugins-internal](https://github.com/westarete/claude-plugins-internal)
marketplace.

## Running commands

Assume the **current working directory is this repository's root**.
Invoke tools relative to that root, the same way a developer would in a
terminal opened at the repo.

- Prefer `make markdown`, `make test`, `git status`, `git commit`, etc.
  — not absolute paths.
- Do not use `cd` in the command string when you can avoid it.

**Never run `sudo` or elevated-privilege commands.** If something truly
needs that, describe the issue and stop.

**Never search or `find` starting from `/`, `$HOME`, or other paths
outside this repo and its dependency caches.** A full-filesystem scan is
slow, and it can read files that have nothing to do with the task. To
look up a Go dependency's source, use `go doc <package>` scoped to the
module, or `go list -m -f '{{.Dir}}' <module>` to get its exact path,
rather than guessing a glob and searching from root.

## Markdown

After editing any `.md` file, run `make markdown`. It formats the whole
repo with Prettier, then lints the whole repo with markdownlint-cli2 —
always the full repo, never scoped to specific files. Write long
single-line paragraphs and let Prettier wrap. Do not hand-format, and do
not re-edit a file to undo what the formatter did.

`make markdown` runs `bin/format-markdown`, which runs both tools
through `npx` at the exact versions pinned in `.markdown-tool-versions`;
CI runs the same script with `--check`. Always use `make markdown`, in
Cowork too; never run `prettier` or `markdownlint-cli2` directly,
because a different version wraps text differently. The only thing to
install is Node.

## Git workflow

The hard rule: **never run `git commit` until the developer has approved
the exact message you proposed in chat.** "Commit when you're done" is
not pre-approval of a message.

Follow the release process in [README.md](README.md).

Tagging a release is a commit-level decision. Before running `git tag`,
propose the version number and the exact tag message in chat and wait
for approval, the same as a commit message. "Do the release" is not
approval of either. Do not push `main` or the tag until that approval is
given.

For the git-commit skill, the project's CHECK command is `make markdown`
and `make test` — there is no separate FORMAT command. Run whichever
applies to what changed, in Phase 2, before proposing a commit message,
every time.

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

### Research the idiomatic way before designing

Don't assume you know the most idiomatic way to use a tool, library,
language, or platform; that assumption has been wrong here. Before
proposing a new design, or a new way of using git, Claude Code, Swift,
macOS, or anything else, find out how the most skilled practitioners do
it: respected books, talks, and essays by experienced authors, and
well-crafted open-source projects. A tool's own documentation says what
the tool can do, not usually the most elegant way to use it. Report what
you found and from whom, then propose.

### API references and dependencies

Never write code or configuration against an external dependency from
memory. For any package, library, CLI tool, configuration file format,
or service — look up the current documentation before use. Check that
the version in use matches what the docs describe. A confident-sounding
answer based on stale knowledge is worse than admitting uncertainty.

Don't guess and don't pass the buck — check, report what you found, then
make the change.

### Clean code

Follow Robert Martin's definitions of clean code. Functions do one
thing. Names say what they mean. Side effects are explicit. Abstractions
match the problem domain, not the implementation. When something feels
awkward to name or test, that is usually a design problem, not a naming
problem.

### Testing catalog against its own repo

This repo is currently below the catalog skill's own size threshold for
adopting a catalog (see the skill's SETUP.md). If you run
`catalog bootstrap` or similar here to verify a change end to end, treat
`.catalog/` and `.catalog.md` as throwaway output and delete them
afterward rather than committing them. Don't add them to `.gitignore`
either — the goal is not to create them for a real reason, not to hide
them once created. If this repo ever grows enough to clear the threshold
for real, that's a deliberate decision to make afresh, not something
that should already be half-done because the files were quietly ignored.

### Changing the default model

Change `defaultModel` in `config.go` and the model, the three price keys
and `warn_request_tokens` in `config.example.toml` in one commit, then
release. `catalog init --print` is how the catalog skill learns the
recommended config, so a stale template means repos upgrade to the wrong
model or prices. Copy the prices from the
[pricing page](https://platform.claude.com/docs/en/about-claude/pricing),
and set `warn_request_tokens` to the model's long-request threshold or
drop it if there is none. A test keeps the template's model equal to
`defaultModel`, but nothing checks the prices.

### Work incrementally

Do not generate large amounts of code in one move. Propose a step,
confirm it makes sense, then write it. A slow developer who ships
correct code is better than a fast one who ships something to debug.
When the next step is unclear, say so rather than filling the space with
plausible-looking code.

## Principles (short)

- Update the doc that a reader would naturally open — usually the same
  commit as the idea, not a delayed pass.
- When recommending tools or practices, sanity-check assumptions; say
  when something is uncertain.
