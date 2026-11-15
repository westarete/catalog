# A first daemon good enough to replace today's catalog: draft

This is a proposal for the smallest daemon we could build on a developer's Mac that follows [architecture.md](architecture.md) and is good enough for them to stop running today's `catalog` in their repositories. Nothing here
is decided.

The goal is to end the two problems [daemon.md](daemon.md) describes
under "What hurts today": a catalog that falls behind because nobody ran
`update`, and catalog files that conflict in Git. We'd accept a catalog
of somewhat lower quality than today's to get there, and improve it
later.

## What it does

The daemon keeps `.catalog.md` current in each tree a person lists, keeps
all of its state outside those trees, and profiles files with a hosted
model. Each component in architecture.md is present, but most are the
simplest version that keeps the component's interface, so a better
version can replace it later without changing its neighbors. Where the
settings and the Store's files live, and what format they take, are
still open; the locations in the table are placeholders.

| Component | First version                                                                                                                                                                                                                                                                                               |
| --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Settings  | A file for this machine in `~/Library/Application Support/com.westarete.catalog/` that lists the folders the daemon may look in, and an API key. Each tree's `.catalog.toml` holds the same `enumerate` and `ignore` globs as today's `.catalog/config.toml`.                                               |
| Trees     | Walks the authorized folders, finds each `.catalog.toml`, and applies the nesting rule and the globs. Git worktrees need nothing extra, because each one has its own copy of `.catalog.toml`.                                                                                                               |
| Watcher   | No FSEvents yet. It crawls every minute or two and compares each file's modification time and size with the last crawl. Apple advises crawling when events are dropped ([research/spotlight.md](../research/spotlight.md)), so we'd build that path first and add FSEvents behind the same interface later. |
| Store     | One SQLite file in the Application Support folder, with profiles keyed by content hash and recipe.                                                                                                                                                                                                          |
| Planner   | A pure function that returns the files needing a profile, oldest first, once each has been unchanged for a few minutes. That delay is a first answer to the question in daemon.md about not profiling a file on every auto-save.                                                                            |
| Reader    | Text cut at a size limit, and PDFs sent to the model whole.                                                                                                                                                                                                                                                 |
| Profiler  | A prompt for each depth, with neighbor profiles as context, in one pass. It offers two of the three depths in daemon.md: a listing of path, type, size, and first lines, available at once, and a summary once the model has read the file. The level that weighs a file against its peers waits.           |
| Models    | One provider, Anthropic, starting with Haiku, behind the interface.                                                                                                                                                                                                                                         |
| Renderer  | Writes entries in the shape agents already know from today's `.catalog.md`, and marks entries that are only listings so far.                                                                                                                                                                                |
| Publisher | Writes `.catalog.md` to a temporary file and renames it into place, and only when the text has changed. Trees must leave `.catalog.md` out of what it indexes.                                                                                                                                              |
| Journal   | An append-only file with one JSON line per event: each profiling with its tokens and time, and each failure. It costs little, and it means the record exists from the start, as architecture.md asks.                                                                                                       |
| Daemon    | A `catalog daemon` loop that a `launchd` agent runs, from a property list written by hand.                                                                                                                                                                                                                  |

Developers often keep their repositories in `~/src`, which isn't among the protected
folders Howard Oakley lists
([Oakley, 2026](https://eclecticlight.co/2026/04/07/privacy-protected-folders/),
cited in architecture.md). We expect a bare launch agent to read it
without an app bundle, but we haven't tested that.

## What waits

- Control and its clients. There's no socket and no `catalog status`
  that asks the daemon. A person would read the Journal or the launchd log
  instead. This is the one place the first version falls short of
  architecture.md, and it stays a gap rather than a break only if no
  client reads the Store directly in the meantime.
- FSEvents, other model providers, the peers depth, setup commands, MCP,
  and Linux.

## Moving a repository to the daemon

1. Move `.catalog/config.toml` to `.catalog.toml`.
2. Stop tracking `.catalog/catalog.db` and `.catalog.md` with
   `git rm --cached`, and add `.catalog.md` to Git's global ignore file.
3. Leave the line in `CLAUDE.md` that loads `.catalog.md` as it is.
4. Run a one-time `catalog import` that copies the old profiles into the
   Store.

The import is what makes the move cheap. Today's database stores each
document's content hash beside its profile
([store.go](https://github.com/westarete/catalog/blob/d9a8d1b3a9dd6826224bda2068a5422b37f8bce9/store.go)),
and the new Store is keyed by content hash, so the daemon can reuse
every existing profile without calling the model again. On the first
day, catalog quality matches what the repository has now.

## What we give up

- Today's `bootstrap` profiles every document twice, so the second pass
  sees good neighbor profiles
  ([generate.go](https://github.com/westarete/catalog/blob/d9a8d1b3a9dd6826224bda2068a5422b37f8bce9/generate.go)).
  The daemon profiles once, with whatever neighbor profiles exist at the
  time. A batch of new files would place itself among its neighbors a
  little worse until those files are profiled again.
- Cloud agents and fresh clones see no catalog, because `.catalog.md` is
  no longer on GitHub. daemon.md already accepts this under "What the
  catalog is for."
- A new file appears as a listing within a couple of minutes and gets a
  summary a few minutes after that, instead of when someone runs
  `update`.

## The order we'd build it in

Each step is usable before the next one starts.

1. `catalog daemon --once <tree>`: the Store, Planner, Reader, Profiler,
   Renderer, and Publisher over one tree, tested with a fake model and
   file system.
2. `catalog import`, then a manual run against one of our repositories.
3. The crawl loop, and finding trees under the authorized folders.
4. The launchd property list and the Journal, then moving the rest of our repositories.

## Language

The first version is all Go, because it has none of the Mac's native
parts: no menu bar item, no app bundle, and no Apple framework readers.
Swift arrives with the first of those.
