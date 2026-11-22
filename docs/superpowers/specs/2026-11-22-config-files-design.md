# The config files: design

The daemon reads three files: `keys.toml` holds the person's API keys,
`global.toml` holds the folders to watch and the defaults for every
catalog, and each catalog's `.catalog/config.toml` holds that catalog's
own settings under the same names as those defaults. Keys never sit in a
catalog's folder, so a catalog's config can go wherever the folder goes,
shared or copied. Two lists, `include` and `exclude`, replace today's
`enumerate` and `ignore`, and both add up across the levels instead of
replacing each other. A folder becomes a catalog when it has a
`.catalog/config.toml`, not just an empty `.catalog/` folder. A new
`catalog init` command puts a catalog in place, and in a Git repository
it also keeps `catalog.md` out of commits.

## Why, and what success looks like

The first build's config format came from the earlier tool and was never
designed on its own terms. A `global.toml` that `catalog setup` writes
today, with made-up folders, shows it:

```toml
model = "claude-haiku-5-5"

[[watch]]
folder = "~/src/acme/website"

[[watch]]
folder = "~/src/notes"
```

Each watched folder is a table that holds nothing but its path. Nobody
wrote down why ([the daemon spec](2026-11-15-daemon-mvp-design.md) only
shows it in an example), and no planned setting belongs to a watched
folder. Other problems showed up on the same look:

- A catalog's `ignore` replaces the global list instead of adding to it,
  so leaving out one more file means copying the whole global list into
  a tracked file.
  [Issue #17](https://github.com/westarete/catalog/issues/17) hit
  exactly that.
- `enumerate` is a word nobody else uses for this. TypeScript and Ruff
  say `include` and `exclude`.
- The API key sits in `global.toml`, beside settings that people may
  want to share.
- The daemon silently ignores keys it doesn't know, so a misspelled
  `exclude` does nothing and the files it meant to leave out go to the
  model service.
- Our advice for keeping `catalog.md` out of Git, a line in Git's global
  ignore file, misses nested catalogs. In a test repository, the pattern
  `.catalog/catalog.md` matched only the top-level catalog;
  `**/.catalog/catalog.md` matched both.
- An empty `.catalog/` folder is enough to make a catalog. A person who
  comes across one can't tell what it does or that it matters, and
  deleting it silently stops the catalog.

Catalog is before 1.0 and has one user, so this is the cheapest time to
change the format. The open issues already point at settings that will
need a home soon: whether to skip files Git ignores
([#17](https://github.com/westarete/catalog/issues/17)), secret patterns
([#10](https://github.com/westarete/catalog/issues/10)), the header text
([#2](https://github.com/westarete/catalog/issues/2)), entry size and a
prompt for code ([#3](https://github.com/westarete/catalog/issues/3),
[#4](https://github.com/westarete/catalog/issues/4)), and, from the
conversation that led to this design, a model and key per catalog, a
custom prompt, and a glossary for the prompt.

It works when:

- People who share a folder share its catalog's `config.toml`, while
  each keeps their own API key on their own machine.
- A catalog can leave out one more file, or cover one more file type,
  with a one-line config.
- A file the person excluded in `global.toml` can't come back through a
  catalog's config.
- A misspelled key shows up in `catalog status` instead of being
  ignored.
- The settings the issues anticipate fit into these files without
  another change to their shape.
- `catalog init` puts a catalog in place in one step. In a Git
  repository, it also keeps `catalog.md` out of commits with no setup
  outside the repository.

## Goals

- One vocabulary: a setting has the same name in `global.toml` and in a
  catalog's config.
- Secrets in a file of their own, away from catalog folders and from the
  places agents work.
- Lists that combine the way people expect, with no order-dependent
  rules.
- Files a person can read and edit by hand without the reference open.
- Room for the anticipated settings without another format change.

## Non-goals

- Building the anticipated settings. A model or key per catalog, a
  custom prompt, a glossary, the Git-ignore switch, and secret patterns
  all fit the shape this design sets, but each is its own change.
- Settings for a subfolder inside a catalog, or for one watched folder.
  Customization happens at the level of a catalog.
- Migration code, or reading the old format. The only existing files are
  on one machine, and they'll be edited by hand when this ships.
- Asking a person to approve a catalog's config that came from someone
  else before it takes effect. That may come later, together with the
  settings that would make it matter.
- Moving `catalog.md` out of `.catalog/`.

## Anti-goals

- A key in a catalog's folder, or anywhere else the daemon watches, even
  in a file Git ignores.
- A catalog's config that can undo what the person excluded on their own
  machine.
- An error message, a status line, or a journal entry that quotes a
  value from these files, or a key name that might be pasted text.
- A config file the daemon writes over without being asked.

## Constraints

- Catalog is experimental and before 1.0, so a release may change the
  format ([Release catalog](../../how-to/release.md)).
- The state folder stays
  `~/Library/Application Support/com.westarete.catalog`, mode 0700
  ([files reference](../../reference/files.md)).
- A file with a secret is refused while it's world-accessible, as
  `global.toml` is today
  ([configuration reference](../../reference/configuration.md#globaltoml)).
- The daemon rereads every config file on each crawl and keeps the last
  settings that loaded when a file fails, as today.
- The TOML library is BurntSushi/toml v1.6.0. Its `MetaData.Undecoded`
  reports unknown keys by name, not by line, so finding a line for one
  is our code's job.

## Approaches

The format follows from the conversation; what we compared was how to
get from today's files to it.

1. **Read only the new format, and edit the existing files by hand.**
   The daemon knows nothing about the old names. Chosen: the only old
   files are on one machine, and editing six of them is less work than
   any code that reads them.
2. **Read both formats for a while.** Kind to many users, but there
   aren't many users, and it means two readers to write, test, and later
   remove.
3. **A command that rewrites old files.** Code that runs once, on one
   machine, and must keep hand-written comments, which TOML encoders
   don't.

Three earlier choices were also weighed and settled:

- **What makes a folder a catalog.** Options were an empty `.catalog/`
  folder (today's rule), the catalog's `config.toml`, a list of catalogs
  in `global.toml`, and a registry that `catalog init` writes to.
  Chosen: `.catalog/config.toml`. A marker should say what it marks: a
  file named for the catalog's settings, holding a commented template
  from `catalog init`, tells a person who opens the folder what it is
  and what they can change there, where an empty folder tells them
  nothing. It's also the usual way a project's root is marked, as a
  `go.mod` file marks a Go module (`go help modules`: "a tree of source
  files, with a go.mod file in its root"). A list in `global.toml` or a
  registry would mean editing settings on the Mac to add a catalog,
  which Claude Cowork and the catalog skill can't do through a folder,
  and a registry would point at the old path after a folder moves. In a
  Git repository, a file also survives where an empty folder wouldn't,
  since Git doesn't keep empty folders.

- **Where per-folder settings live.** Options were settings on each
  `[[watch]]` entry, path-keyed sections in `global.toml` (as in
  EditorConfig or ssh's `Host` blocks), and the catalog's own
  `config.toml`. Chosen: the catalog's own `config.toml`, so
  customization sits with the catalog it describes and can be shared
  with it. With nothing left to attach to a watched folder, `watch` is a
  plain list.
- **How lists combine.** Options were replacing (today's rule, and
  TypeScript's `extends`, which
  [overwrites `include` and `exclude`](https://www.typescriptlang.org/tsconfig/#extends)),
  adding, and both under different names (Ruff's `exclude` and
  `extend-exclude`;
  [Ruff's settings](https://docs.astral.sh/ruff/settings/) tell people
  they'll usually want the adding one). Chosen: both lists add, and
  exclusion wins over inclusion with no negation. A catalog can then
  leave out one more file, or cover one more file type, without
  restating anything; nothing a catalog's config says can undo what the
  person excluded on their own machine; and the order of lines never
  matters. Git's ignore files are prior art on both sides: layered files
  that add up, which other tools copy, and `!` rules, which
  [can't re-include a file under an excluded folder](https://git-scm.com/docs/gitignore)
  and depend on the order of lines, which people struggle with.

## How it works

### keys.toml

`keys.toml` sits in the state folder next to `global.toml`. It holds
named API keys as top-level strings:

```toml
default = "sk-example-not-a-real-key"
acme = "sk-example-another-made-up-key"
```

- `catalog setup` creates it with mode 0600 whenever it's missing, even
  when the person skips the key, as described under `catalog setup`
  below. A person can also write it by hand.
- The daemon refuses it while it's world-accessible, with the same rule
  and message as `global.toml` today, and drops any key it loaded from
  it.
- The daemon uses the key named by `key` in `global.toml`, or `default`
  when `key` isn't set.
- No other file holds a key, and no other file is read for one. A
  `config.toml` or `global.toml` that has `api_key` gets the unknown-key
  error below, which never quotes the value.
- Errors about this file never name a key in it, since a name might be
  pasted text. A parse error gives the line only.

Named keys are there for later: a catalog's config will choose a key by
name, so a shared config can say `key = "acme"` and each person defines
`acme` in their own `keys.toml`. This design builds only the global
`key`.

### global.toml

```toml
watch = [
  "~/src/acme/website",
  "~/src/notes",
]

model = "claude-haiku-5-5"
key = "default"
include = ["**/*.go"]
exclude = ["**/drafts/**"]
```

- `watch` is a list of folders, each absolute or starting with `~/`,
  with today's rules for expanding `~` and refusing relative paths.
- `model` and `key` are this machine's choices. A catalog's config can't
  set them in this design.
- `include` and `exclude` are defaults for every catalog, combined as
  described below.
- The file holds no secret, but the daemon still refuses it while it's
  world-accessible. Whoever can change it decides which of the person's
  files go to the model service.

### .catalog/config.toml

A folder inside a watched folder is a catalog when it holds
`.catalog/config.toml` as a regular file. The file can be empty or hold
only comments. A symbolic link named `config.toml` doesn't count, the
same rule the daemon applies to every other file. A `.catalog/` folder
without one isn't a catalog: the daemon stops updating any `catalog.md`
left in it, and doesn't delete it. The walk that finds catalogs still
skips `.git` and never looks inside a `.catalog/` folder.

A catalog's config uses the same names as the defaults in `global.toml`.
In this design that's `include` and `exclude`:

```toml
include = ["**/*.go"]
exclude = ["notes/private/**"]
```

It can travel with the folder, shared or copied, since it holds no
secret and can only narrow or widen which files in its own folder the
daemon covers. Later settings join it under the names they'll have in
`global.toml`.

### How lists combine

Three levels each contribute to `include` and `exclude`: the daemon's
built-in defaults, `global.toml`, and the catalog's config. At each
level a list adds to the levels before it; it never replaces them. A
file belongs to a catalog if it matches any `include` pattern and no
`exclude` pattern, at any level. There are no `!` rules, and the order
of patterns doesn't matter.

Built-in defaults: include `**/*.csv`, `**/*.md`, `**/*.pdf`,
`**/*.txt`; exclude `**/CLAUDE.md` and `**/AGENTS.md`. Agents read those
two files at the start of a session, so a catalog entry for them would
only cost context, and because exclusion wins, no level can bring them
back. `README.md` is no longer a built-in exclude, as it is today: a
catalog's README is often the first thing an agent needs, and agents
don't read it unless something sends them there. A person who doesn't
want it cataloged excludes it with one line. To leave out a file type
the defaults include, a person excludes it the same way, for example
`exclude = ["**/*.csv"]`.

The rules that don't depend on config stay: nothing under `.git` or
`.catalog`, nothing under a folder with a `.catalog/` of its own, only
regular files, no symbolic links followed.

### Unknown keys

A key the daemon doesn't read is an error in that file, handled like a
parse error today: status shows a `Config error` line, the journal
records it, and the daemon keeps the last settings that loaded from that
file. Before any have loaded, a catalog is skipped. Rejecting the file,
not just warning, keeps a misspelled `exclude` from sending the files it
was meant to leave out.

The message gives the line, never the key's text, and suggests the
nearest known key when one is close:

```text
Config error: /Users/alex/src/notes/.catalog/config.toml: line 3: unknown key; did you mean exclude?
```

Since the library reports unknown keys by name only, the daemon finds
the line in the file's text itself.

### catalog init

`catalog init [folder]` puts a catalog in place in `folder`, or the
current folder when none is given:

- It creates `.catalog/` if it's missing.
- It writes `.catalog/config.toml`, which makes the folder a catalog,
  with every key commented out and a line saying what each does, so a
  person can see the settings without the reference.
- If the folder is inside a Git repository, it writes
  `.catalog/.gitignore` containing `/catalog.md` and `/catalog-*.tmp`,
  so `catalog.md` stays out of commits in this catalog, and in any
  nested one that has its own, without a global ignore line. The second
  line covers the temporary file the publisher writes the full catalog
  text to before renaming it into place (`tempPattern` in
  `internal/publish/publish.go`); one left behind by a crash or an
  uninstall would otherwise be committed by a `git add -A`. Outside a
  repository, `catalog init` writes no `.gitignore`.
- It never changes a file that exists, and says which ones it left
  alone.
- It prints what an agent's instruction file needs to load the catalog,
  `@.catalog/catalog.md`, and whether the folder is inside a watched
  folder.

These files are meant to travel with the folder, so `catalog init`
doesn't restrict their modes; `catalog.md` stays 0600 because the daemon
writes it.

### catalog setup and catalog status

`catalog setup` writes the key to `keys.toml` as `default` and the
folders to `global.toml` as a `watch` list. It treats each file on its
own, with today's rule for a file that exists: it asks for a key only if
`keys.toml` is missing and for folders only if `global.toml` is missing,
and changes neither file if it exists. If the person skips the key,
setup still creates `keys.toml` with mode 0600, holding only a comment
that says how to add one. A key added later by hand then goes into a
file that already has the right mode; a file the person creates
themselves would get their umask, usually 0644, and the daemon would
refuse it. If `keys.toml` exists without the key, running setup again
doesn't ask for one and says to add it to that file. A person who
already has a `global.toml` and no `keys.toml` is asked only for the
key.

`catalog status` adds a line for `keys.toml`, in the form of the
existing install lines:

```text
Keys: /Users/alex/Library/Application Support/com.westarete.catalog/keys.toml (key set)
```

The text in parentheses is fixed and never names a key: `(key set)` when
`keys.toml` has the key that `global.toml` asks for,
`(not found; run catalog setup)`, `(can't be read: <error>)`, or
`(no key)` for a file without the key `global.toml` asks for. The value
of `key` in `global.toml` is never printed, since a person could paste
anything there.

The `Settings:` line keeps its path and reports whether the file loads,
but no longer reports a key.

## Sensitive data in this change

| Data                                            | Where a copy ends up                                                                                                          | Who can read it there                                                                                                           |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| API keys                                        | `keys.toml` in the state folder, mode 0600, refused while world-accessible; the daemon's memory                               | The person's account. No catalog folder, journal, log, status line, or error message gets a key or a key's name from this file. |
| Watched folders, model, key name, defaults      | `global.toml` in the state folder, refused while world-accessible; status and the journal as paths and line numbers in errors | The person's account                                                                                                            |
| A catalog's `include` and `exclude`             | `.catalog/config.toml`, which travels with the folder: shared, copied, or committed                                           | Everyone the folder is shared with, as today. It names patterns, not file contents.                                             |
| Unknown keys                                    | Status and the journal, as a line number and at most the name of a known key                                                  | The person's account. The unknown key's text is never copied, since it might be pasted text.                                    |
| `.catalog/.gitignore`, only in a Git repository | The repository                                                                                                                | Everyone with the repository. It names only `catalog.md` and the publisher's temporary files.                                   |

What changes for privacy:

- The key moves out of `global.toml`, so that file can be shared, and
  nothing about the key changes in the request to the model service.
- Exclusion wins at every level, so a config that came from someone else
  can't bring back a file the person excluded on their machine. It can
  still add `include` patterns and widen what the daemon sends from that
  catalog's folder, as it can today.
- A misspelled key no longer fails open.
- In a Git repository, `catalog init`'s `.gitignore` keeps `catalog.md`,
  which can name and describe private files, out of commits in every
  catalog, nested ones included.

## Quality, size, and cost

None directly: the daemon covers and summarizes the same files once the
person's six files are converted. Adding lists make it easier to leave a
file out, which can reduce what a catalog costs, and nothing else
changes the prompt or the model calls.

## Docs to change

- [Configuration](../../reference/configuration.md): rewrite for the
  three files, the shared names, how lists combine, and unknown keys.
  Replace "What a catalog's config can't hold" with what a catalog's
  config can hold now.
- [Commands](../../reference/commands.md): add `catalog init`; change
  `catalog setup` and the status install lines.
- [Files](../../reference/files.md): add `keys.toml` and
  `.catalog/.gitignore`.
- [Install and run the daemon](../../how-to/install-and-run.md): setup's
  output, putting a catalog in place with `catalog init`, and drop the
  global ignore advice.
- [Where catalogs go](../../explanation/catalogs.md): what a tree needs,
  now a `config.toml` instead of an empty folder, and the same ignore
  advice.
- The README, if it names `enumerate`, `ignore`, or `[[watch]]`.

The explanation docs need a short passage on why keys live apart and why
lists add up; the reference shouldn't carry that reasoning.

## How we'll know it works

- Tests cover each rule above, including: a catalog that excludes one
  file without restating the global list; a global exclude a catalog's
  `include` can't undo; an unknown key reported by line with no text
  from the file; `api_key` in `global.toml` reported as unknown; a
  world-accessible `keys.toml` refused; `catalog init` writing nothing
  over existing files; an empty `.catalog/` and a `.catalog/` whose
  `config.toml` is a symbolic link, neither of them a catalog.
- Mutation testing over the new code, as CLAUDE.md asks.
- After release, the person's `global.toml` and five `config.toml` files
  converted by hand, with `catalog status` showing no config errors and
  the same file counts as before.

## Left to the builder

- How to find the line of an unknown key in the file's text, and the
  rule for "close enough" when suggesting a known key.
- The wording of the commented template `catalog init` writes.
- Whether `catalog init` creates a missing folder or refuses.
- How `catalog init` tells that a folder is inside a Git repository,
  without requiring the `git` program, since a folder doesn't have to be
  a repository.
