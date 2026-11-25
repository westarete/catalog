# Notes for the command redesign issue

These notes collect what we've decided so far about catalog's commands
and files, so the issue can be written from them. They aren't the issue.

## Terms

- A catalog is a folder tree the person has added with `catalog add`.
  Commands find it by walking up from the current folder, as Git finds a
  repository.
- A catalog covers its folder tree except any subfolder that holds a
  `.catalog/` folder of its own, whether or not anyone has added that
  subfolder. The issue should state this rule and the one above in one
  place, since together they decide what a catalog covers.
- The service is the background process on this Mac. It replaces the
  word "daemon" for people; "model service" becomes "model provider" so
  the two don't collide.

## Commands

```text
catalog add [folder]
catalog remove [folder]
catalog status

catalog service run | start | stop | restart | status
catalog service key set [name]

catalog version
catalog completion <shell>
```

- `catalog add` creates `.catalog/`, writes the catalog's ID there, and
  adds a `[[catalog]]` entry to `service.toml`. A folder that arrives
  with a clone does nothing until the person adds it.
- Before it changes anything, `catalog add` prints the folder, the
  provider, the key name, and how many files the catalog covers. Adding
  with a key or provider other than the defaults needs confirmation at a
  terminal, so an agent can't choose whose key pays.
- `catalog add` uses the `[defaults]` and takes flags such as
  `--provider` and `--model`, so agents and scripts can add a catalog
  with the defaults without a prompt.
- `catalog add` on a catalog that's already added says so and says how
  to change its settings.
- In a Git repository, `catalog add` writes `.catalog/.gitignore`, which
  leaves out `catalog.md` and the ID file.
- `catalog remove` says that it left the folder's files and its
  `.catalog/` folder alone. Leaving `.catalog/` in place keeps an
  enclosing catalog from taking over the removed catalog's files.
- Every command directly under `catalog` acts on one catalog: the one
  you're in, or the folder you name.
- `catalog service key set` takes the key's name; without one, it uses
  the key named in `[defaults]`.
- The help text marks `catalog service run` as the command launchd runs,
  so nobody takes it for `start`.
- `setup`, `init`, the top-level daemon `status`, and the `watch` list
  go away.

## A catalog's identity

- `catalog add` writes a random ID into the folder, in `.catalog/id`,
  and the same ID into the catalog's entry in `service.toml`. The
  service works on a catalog only when the folder at the entry's path
  holds the matching ID.
- Only `catalog add` creates `.catalog/`; the service never does, as
  today's daemon creates no catalog itself.
- This is the shape of Git's worktrees, where the worktree's `.git` file
  points to its entry and the entry's `gitdir` file points back
  (`man git-worktree`, DETAILS).
- A different folder at the same path, whether a fresh clone or another
  project, holds no matching ID, so the service stops and
  `catalog status` says the folder hasn't been added.
- A catalog moved to another disk or restored from a backup keeps its
  ID. Running `catalog add` at the new path finds the ID and offers to
  move the entry there, keeping the settings and summaries, as
  `git worktree repair` does.
- A copy made in the Finder brings the ID along. Nothing happens until
  someone adds the copy; then `catalog add` sees that the ID belongs to
  a catalog still in place and writes a new one.
- If the ID file is missing, `catalog status` says so and names
  `catalog add`.
- The ID isn't a secret. Git ignores it because each person's
  `catalog add` writes a different one, which would conflict on every
  merge.
- The service reads the ID file as it reads `config.toml`: it refuses a
  symbolic link or anything but a regular file.

## Files

- `global.toml` becomes `service.toml`. The `watch` list and the global
  `include` leave it. It holds the person-wide `exclude`, a `[defaults]`
  table that `catalog add` copies from, a table for each provider that
  needs an address, and a `[[catalog]]` entry for each catalog the
  person added.
- The program owns `service.toml`. `catalog add` and `catalog remove`
  read it, change it, and write it whole, so it holds no comments;
  explanations live in help and the docs. It stays a text file so people
  can read it.
- Each write starts the file with this header:

  ```toml
  # Comments you add to this file will be deleted.
  # catalog rewrites this file whenever you run `catalog add` or `catalog remove`.
  # To change these settings, run `catalog service edit`.
  ```

- `catalog service edit` copies `service.toml` into the state folder,
  opens `$VISUAL` or `$EDITOR` on the copy, and checks it when the
  editor exits. It offers to edit again or discard, as `vipw` does, and
  never saves a file that fails the check; `visudo`'s `Q` does, which is
  the part to avoid. Once the check passes, it writes the file in its
  own format, so comments disappear at once. Keeping the copy in the
  state folder keeps the editor's swap and backup files there too.
- go-toml v2 has an `unstable/edit` package that keeps a file's comments
  and layout when a program edits it, but as of 2026-11-25 it isn't in a
  release. Revisit it only if people need to keep comments in the file.
- `keys.toml` stays apart from `service.toml`, so the settings file
  holds no credentials. It still holds catalog paths and key names,
  which can identify clients, so the docs mustn't tell people it's safe
  to paste in public.
- `control.sock` becomes `service.sock`.
- `journal.jsonl` goes away. The service writes each event as a JSON
  line to standard output, and launchd sends standard output and
  standard error to `~/Library/Logs/catalog.log`. Standard error carries
  only what we don't control, such as a Go panic.

## Provider, model, and key for each catalog

- Each catalog's entry in `service.toml` names its provider, model, and
  key, such as Anthropic with Haiku for one catalog and a local model
  through Ollama for another. The `[defaults]` table holds what
  `catalog add` copies into a new entry.
- `.catalog/config.toml` still can't set any of them, because a folder
  that arrives with a clone mustn't decide where its files go or whose
  key pays.
- `catalog add` writes the choice into the entry, so changing a default
  later never moves an existing catalog to another provider.
- If a catalog's entry doesn't load, the service summarizes nothing in
  that catalog. It never falls back to the default provider.
- `catalog status` shows the catalog's provider, model, and key name,
  never the key.
- A provider's address in `service.toml` must be on this Mac, or reached
  over HTTPS. The service checks this when it loads the file, and
  `catalog service edit` checks it too. Issue #24 raises the same
  requirement.

## Include and exclude patterns

- `.catalog/config.toml` is optional and committed with the folder, like
  `.gitignore`. It holds the catalog's `include` and `exclude` patterns.
  It no longer marks the catalog; the ID and the entry in `service.toml`
  do.
- Since a person must run `catalog add` before anything happens, a
  cloned config no longer acts without consent. A later `git pull` can
  still widen what's sent; status and spending reports make that
  visible.
- `service.toml` keeps a person's `exclude` list, which applies to every
  catalog and which no catalog can override. It loses its `include`
  list. Git's ignore files have the same shape: `.gitignore` in the
  repository, and `core.excludesFile` for the person, with no global
  include.
- Built-in defaults should be good enough that most people never write a
  pattern.
- Whatever the patterns say, the crawl always leaves out the state
  folder and the log, so a broad `include` in a folder like `~` can't
  send `keys.toml` to a provider.

## Privacy and security

These requirements came from reviewing the design as an attacker would.
They're in order of how likely and how harmful the problem each one
prevents.

1. Every file the service or a command writes into a catalog's folder,
   such as `catalog.md`, `.gitignore`, the ID, and later `status.json`,
   is written to a temporary file and renamed into place, and never
   through a symbolic link. Otherwise a hostile repository could get the
   service to overwrite one of the person's files.
2. A catalog whose folder was replaced stops until the person adds it
   again (see the ID section).
3. An agent can't add a catalog with a non-default key or provider
   without a person confirming at a terminal (see the commands section).
4. The docs don't call `service.toml` safe to paste in public.
5. The crawl always leaves out the state folder and the log.
6. Provider addresses are on this Mac or reached over HTTPS.

## First run

- The Homebrew cask runs `catalog service start` at install, where it
  runs `catalog setup --agent-only` today. Its caveat text names
  `catalog service key set` and then `catalog add`.
- `catalog status` and `catalog service status` say plainly when there's
  no key yet.

## For developers

- Rename the code's daemon, journal, control, and global to service,
  log, and so on in the same change as the commands, so people and
  agents don't meet two vocabularies.

## Documentation to write

- Include and exclude patterns tune relevance and cost; they don't keep
  anything from the model. Don't catalog a folder that holds anything
  that mustn't reach the catalog's provider.
- Whether a folder may be processed by AI at all is for whoever owns the
  information to decide, through contracts and policy. A local provider
  doesn't settle it, since a local model is still AI. Say so in the
  guide for adding a catalog and in the overview's privacy and security
  section.
- The person's settings protect against what arrives in a folder: a
  clone, a teammate's commit, an agent in Cowork. They don't protect
  against an agent that runs as the person with permission to edit those
  settings or run `catalog`.
- Update the commands, configuration, and files references; the install
  how-to; the explanation of where catalogs go, which removing watch
  folders changes most; and the Journal section of the architecture doc.
  Add a how-to for adding a catalog.

## Before planning

Someone has to run this experiment, because `man launchd.plist` and
commit 9b1394f disagree about the log's mode. The man page says launchd
creates `StandardErrorPath` with the `Umask` key's umask; the commit
says launchd created it 0644 regardless.

1. Write a throwaway property list in `tmp/`, label
   `com.example.umask-test`, that runs
   `/bin/sh -c 'echo test; echo test >&2'` with `Umask` `077` and both
   `StandardOutPath` and `StandardErrorPath` in `tmp/`.
2. Load it with `launchctl bootstrap gui/$(id -u)`, check the log's
   mode, then `launchctl bootout` it and delete it.
3. Repeat with a 0600 log already in place, to confirm launchd keeps an
   existing file's mode.

The plan depends on the answer: if a log created ahead of time keeps
0600, `catalog service start` should create it before loading the agent.

## Open questions

- Does `catalog remove` delete the catalog's summaries at once, or keep
  them for a grace period, as `git gc` keeps unreachable objects for two
  weeks?
- On upgrade, does the service reject an old `watch` list with an error
  naming `catalog add`, or add every catalog the list finds today, once?
  Adding them automatically would also add the cloned repositories the
  redesign means to stop, so the error, or a list the person confirms,
  is safer.
- Does `daemon --once` survive as `service run --once`?
- What happens to an entry whose folder is gone and was never moved with
  `catalog add`? `git worktree prune` removes such entries after a grace
  period.
- What does `catalog status` call a folder that has a `.catalog/` folder
  but hasn't been added on this Mac, such as a fresh clone?
- Does `catalog add` write a commented `config.toml` into every folder
  it adds, which teaches the patterns but puts a file in the person's
  repository, or only when asked?
- May a catalog's entry hold its own `exclude` list, so a person can
  leave something out of a team's repository without committing it?
- How does a Windows service capture standard output and standard error?
- Cowork can't add a catalog, just as it can't `git push`. We accept
  that; a Mac app would add catalogs later.

## Later, not in the first step

- `.catalog/status.json`: the catalog's state as a whole, rewritten
  whole, with the time it was written, and ignored by Git.
- Reports of spending and disk use for each catalog.
- A spending limit for each catalog.
- `config path` and `config check`.
- A command that catalogs a folder now.
- A terminal interface, perhaps with Bubble Tea, built on the same
  commands, as a draft of the Mac app's settings screens. Check how it
  works with screen readers.
