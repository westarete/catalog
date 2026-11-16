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

Track work still to do in Superpowers plans under
`docs/superpowers/plans/` and in
[GitHub issues](https://github.com/westarete/catalog/issues), where new
bugs and requests go.

**Don't let existing work decide a choice.** Judge a language, library,
design, or name by what the finished product needs. Code, branches, or
files that already exist are not an argument for themselves; the
developer will throw them away when convention or the product calls for
it. If reuse genuinely matters, give it as a separate, named trade-off,
not as the reason.

**Design for the project's audience, not for whoever is testing it.**
This is West Arete's public, open-source project, meant to be broadly
useful and to represent the company. An early user's answers are
evidence about the product, not its scope: never turn "it's only me for
now" into a design, spec, or plan built around one person. Name a person
in a doc only where the text is about that person.

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

Before planning or starting a bug fix or feature, read the open GitHub
issues, so you don't duplicate or contradict work tracked there. Use
`gh issue list`; where `gh` isn't available, as in Cowork, read the
[issues page](https://github.com/westarete/catalog/issues) instead.

## Config files

When creating config files (`.prettierrc.yaml`, `.markdownlint.yaml`,
`.markdownlint-cli2.yaml`, and similar), copy them from the
`knowledge-work` plugin (the markdown skill's `config/` directory)
rather than writing minimal stubs. The comments explain why each setting
exists and are part of the convention — don't strip them.

## Skills

Load `knowledge-work:markdown` before editing any `.md` file. It comes
from the `knowledge-work` plugin in the
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

## Building with Superpowers

Build code with the Superpowers plugin's workflow: brainstorming, a spec
in `docs/superpowers/specs/`, a plan in `docs/superpowers/plans/`, then
subagent-driven development. Where this file and a Superpowers skill
disagree about process, follow the skill.

## Balance quality, size, and cost

These compete in this project. Quality is how often an agent finds the
right files. Size is how many words a catalog adds to every request and
every agent's context. Cost is model calls and money, and time is a good
proxy for it. When you propose a change that improves one of them, say
what it does to the other two.

## Security in every design and review

The daemon reads private documents and stores what it learns about them.
The first build left its journal and store readable by every account on
the Mac ([issue #6](https://github.com/westarete/catalog/issues/6)),
because no spec asked who could read them and no reviewer was asked to
check.

- In every spec, add a section on the sensitive data the change reads,
  stores, or sends: each place a copy ends up (files, logs, sockets,
  requests to a service) and who can read it there.
- In every plan's Review Focus, include at least one security line, and
  copy it into each task reviewer's brief.
- Before finishing a branch, run Claude Code's `/security-review` as
  well as the whole-branch review.
- Keep `gosec` enabled in `.golangci.yml`. When it flags a line on
  purpose, suppress that line with `//nolint:gosec` and a comment saying
  why; don't turn off a rule for the whole repository without a comment
  in the config saying why.

## Own everything the program does

Security, quality, and robustness are your responsibility, including
what a library, the OS, or a tool does on the program's behalf. A file
the program creates, a socket it opens, or a request it sends has the
mode, scope, and limits we choose, or failed to choose. Never explain a
problem away with another package's defaults.

When the developer reports a problem, fix all of it before you move on.
If any part stays unfixed, say so in the first lines of your reply, with
the reason. Never leave part of a reported problem only in an issue, a
commit message, or a list of follow-ups. When the developer reported the
journal and store readable by every account, an agent fixed only the
journal, which the linter flagged, filed the store in an issue, and
reported "the journal" as if that were all of it.

## Git workflow

Commit freely on a feature branch or worktree. Never commit directly to
`main`, merge into it, push it, or push a tag without the developer's
approval in chat.

Run `make markdown` and `make test` before every commit.

Follow the release process in [README.md](README.md).

Tagging a release is a commit-level decision. Before running `git tag`,
propose the version number and the exact tag message in chat and wait
for approval. "Do the release" is not approval of either.

## Code

This is a code-centric repo. Hold it to the same standard as the
writing.

### Tests

Judge a test by whether it catches a fault, not by how it was written.
Run a mutation testing tool over new code from the first package onward,
and treat a mutant that no test kills as a missing or weak test. Choose
the tool during design;
[research/coding-agent-tools.md](docs/research/coding-agent-tools.md)
covers Gremlins as the candidate for Go.

### Research the idiomatic way before designing

Don't assume you know the most idiomatic way to use a tool, library,
language, or platform; that assumption has been wrong here. Before
proposing a new design, or a new way of using git, Claude Code, Swift,
macOS, or anything else, find out how the most skilled practitioners do
it: respected books, talks, and essays by experienced authors, and
well-crafted open-source projects. For anything about processes, files,
or how programs coordinate, start from the Unix classics in
[docs/best-practices/craftspeople.md](docs/best-practices/craftspeople.md),
not from newer protocols such as MCP. A tool's own documentation says
what the tool can do, not usually the most elegant way to use it. Report
what you found and from whom, then propose.

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

## Principles (short)

- Update the doc that a reader would naturally open — usually the same
  commit as the idea, not a delayed pass.
- When recommending tools or practices, sanity-check assumptions; say
  when something is uncertain.
