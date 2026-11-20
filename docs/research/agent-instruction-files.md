# How agents find their instruction files

An agent reads the catalog only if an instruction file it already loads,
such as `CLAUDE.md` or `AGENTS.md`, imports `.catalog.md`. None of these
agent tools knows what a catalog is. So to decide where a catalog has to
sit, and what has to point at it, we need to know which instruction
files each tool loads, and when. We read each tool's documentation on
November 15, 2026. The tools change often, so check the pages again
before relying on what follows.

## Claude Code

Claude Code's
[memory documentation](https://code.claude.com/docs/en/memory) describes
three ways it finds instruction files.

- At launch, it loads `CLAUDE.md` "from your current working directory
  and every directory above it." The page doesn't say where that walk
  stops.
- It also finds `CLAUDE.md` files in subfolders, but it loads each one
  only once Claude "reads, writes, or edits" a file in that subfolder.
- Each person can keep a `~/.claude/CLAUDE.md` with preferences for all
  their projects.

Claude Code resolves an `@path` import "relative to the file containing
the import, not the working directory." Imports can nest up to "a
maximum depth of four hops." By default, Claude Code reads `AGENTS.md`
"only when you have no `CLAUDE.md` in your working directory or above
it."

## Gemini CLI

According to its
[GEMINI.md documentation](https://geminicli.com/docs/cli/gemini-md),
Google replaced Gemini CLI with Antigravity CLI on June 18, 2026, for
unpaid and Google One users.

- Gemini CLI searches "your configured workspace directories and their
  parent directories." The page doesn't say how far up it goes. It also
  loads a global `~/.gemini/GEMINI.md`.
- It loads files in subfolders just in time: "When a tool accesses a
  file or directory," it scans that directory and its ancestors up to a
  trusted root.
- A person can rename the file it looks for, for example to `AGENTS.md`,
  with the `context.fileName` setting.
- It supports `@file.md` imports. The page doesn't say what a relative
  path is relative to.

## Codex

OpenAI's
[AGENTS.md guide](https://learn.chatgpt.com/docs/agent-configuration/agents-md)
treats the project and the folder Codex starts in as two separate
things.

- Codex starts "at the project root (typically the Git root)" and walks
  down to the folder it started in, reading at most one file per folder.
  If it finds no project root, it checks only the starting folder.
- It reads nothing below the starting folder. The page doesn't say
  whether it reads anything above the project root.
- It also reads an `AGENTS.md` in the Codex home directory, which is
  `~/.codex` by default.
- It stops adding files once they reach `project_doc_max_bytes`, 32 KiB
  by default. The page documents no import syntax.

## Claude Cowork

We found nothing in Anthropic's documentation that says whether Cowork
reads a `CLAUDE.md` or `AGENTS.md` in the folder a person selects, in
the folders above it, or in the folders below it. A person can give
Cowork instructions in three other places, according to
[Get started with Cowork](https://support.claude.com/en/articles/13345190-get-started-with-claude-cowork)
and the
[Cowork projects guide](https://claude.com/docs/cowork/guide/projects).

- Global instructions, which the person sets under Settings and which
  apply to every session. Cowork keeps them in a `CLAUDE.md` inside the
  app's own data folder, not in the person's folder
  ([data storage](https://claude.com/docs/cowork/3p/data-storage)).
- Folder instructions, which "add project-specific context to Cowork
  when you select a local folder." The article says "Claude can also
  update these on its own," but it doesn't say where Cowork stores them.
- Project instructions, which apply to every session in a Cowork
  project.

The help article says Claude "can only read and write files in folders
you've connected." The pages don't say whether Cowork can see the
folders above a connected folder.

## What we make of it

This is our reasoning, and we haven't tested any of it.

- Each tool works with two scopes: a broad one, such as the project
  root, the workspace, or the connected folder, and the folder where the
  agent starts. The tools read in different directions from the starting
  folder. Claude Code reads everything above it at launch and the
  folders below it as the agent reaches them. Codex reads from the
  project root down to the starting folder. Cowork's documentation
  doesn't say.
- If a `CLAUDE.md` high in a folder tree imports a catalog, Claude Code
  loads that catalog in every session that starts below it. A large
  catalog there costs context in all of those sessions.
- A subfolder could have its own `CLAUDE.md` that imports its own
  catalog. Claude Code would then load that catalog only when the agent
  works in the subfolder, and not before.
- Each of these tools also reads an instruction file that belongs to the
  person rather than a folder: `~/.claude/CLAUDE.md`, Cowork's global
  instructions, `~/.gemini/GEMINI.md`, and `~/.codex/AGENTS.md`.
