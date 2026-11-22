# Configuration

The daemon reads three settings files: `keys.toml` and `global.toml` in
the state folder, which apply to every catalog on the Mac, and
`.catalog/config.toml` in each catalog, which also marks its folder as a
catalog. This page lists where each file lives, every key the daemon
reads, its type and default, how the `include` and `exclude` lists
combine, and how soon edits take effect. For why the settings are split
this way, see the
[architecture's Settings section](../explanation/architecture.md#settings);
for why catalogs go where they do, see
[where catalogs go](../explanation/catalogs.md); for the commands that
write and read these files, see [commands](commands.md).

All three files are in [TOML](https://toml.io/). The daemon rejects a
file that holds a key it doesn't read, as [Unknown keys](#unknown-keys)
describes.

## The state folder

The daemon keeps `keys.toml`, `global.toml`, and its own state in one
folder:

```text
~/Library/Application Support/com.westarete.catalog
```

The program takes the home folder from `$HOME`. `catalog daemon`,
`catalog daemon --once`, and `catalog setup` create the folder with mode
0700 and set that mode if it already exists.

## keys.toml

The file is `keys.toml` in the state folder. It holds API keys by name,
each a string at the top level of the file. Any name works:

```toml
default = "sk-example-not-a-real-key"
acme = "sk-example-another-made-up-key"
```

The daemon uses the key named by `key` in `global.toml`, or the one
named `default` when `global.toml` sets no `key`. It reads an API key
only from this file, never from `global.toml`, a catalog's config, the
`ANTHROPIC_API_KEY` environment variable, or any other environment
variable.

[`catalog setup`](commands.md#catalog-setup) creates the file with mode
0600 whenever it's missing. If the person gave a key, the file holds it
under the name `default`; if they skipped it, the file holds only this
comment:

```toml
# Your Anthropic API keys, one per line, by name. The daemon uses
# the one named default unless global.toml names another with key.
# default = "your-key"
```

A key added to that file by hand is then in a file only its owner can
read. A person can also write `keys.toml` without setup. If you do,
restrict it with `chmod 600`, because the daemon won't use it while it
is world-accessible, meaning any of its world read, write, or execute
bits is set. Group read and write are allowed, so you can share the file
with a group on purpose. The daemon never changes the file or its mode.

Every value must be a string. A value of another type makes the file
fail to load with
`parse keys.toml: line <n>: a key's value isn't a string`; text that
isn't TOML gives `parse keys.toml: line <n>: not valid TOML`. Neither
error names a key or quotes a value, since a person may have pasted a
key where its name belongs.

### Without a key

Without a key the daemon still crawls and writes each catalog's listing,
but it summarizes nothing, and `catalog status` says
`Not summarizing: no API key loaded from keys.toml`. It also shows one
of these `Config error` lines, where `<path>` is the path of
`keys.toml`:

- `Config error: <path>: not found; run catalog setup` when the file is
  missing.
- `Config error: <path>: no key under the name global.toml asks for`
  when the file has no key by that name, or an empty one.

The second message never names the key `global.toml` asks for, since a
person could paste anything into `key`. `catalog daemon --once` refuses
to run without a key.

When the key or the model changes, the daemon starts using the new one
and ends any pause that was waiting out a problem with the old one.

## global.toml

The file is `global.toml` in the state folder. It holds the folders to
watch, this machine's choice of model and key, and the `include` and
`exclude` lists for every catalog. It holds no secret.
[`catalog setup`](commands.md#catalog-setup) creates it with mode 0600
if it's missing, holding only a `watch` list, and a person can write it
instead. The daemon never changes it or its mode, but it won't use the
file while it is world-accessible, with the same rule as `keys.toml`,
because whoever can change the file decides which of the person's files
go to the model service.

| Key       | Type             | Default            | Meaning                                                                                       |
| --------- | ---------------- | ------------------ | --------------------------------------------------------------------------------------------- |
| `watch`   | array of strings | none               | The folders to crawl for catalogs.                                                            |
| `model`   | string           | `claude-haiku-5-5` | The model that writes summaries. An empty string gives the default.                           |
| `key`     | string           | `default`          | The name of the key in `keys.toml` that the daemon sends to the model service.                |
| `include` | array of strings | none               | Patterns for more files to catalog, added to the built-in list for every catalog.             |
| `exclude` | array of strings | none               | Patterns for files to leave out of every catalog, added to the built-in list. Exclusion wins. |

An example, with made-up folders:

```toml
watch = [
  "~/src/acme/website",
  "/Users/alex/notes",
]

model = "claude-haiku-5-5"
key = "default"
include = ["**/*.go"]
exclude = ["**/drafts/**"]
```

### watch

The daemon crawls each folder in `watch` for catalogs. With no `watch`
list, or an empty one, it crawls nothing.

- A folder that is `~`, or starts with `~/`, has that part replaced with
  the home folder. A path starting with `~` and a user name, such as
  `~other/x`, isn't expanded.
- A folder must be an absolute path after that. A relative one is
  skipped, and `catalog status` shows a `Config error` for it:
  `watch folder must be an absolute path or start with ~/; skipped`. An
  empty string is skipped without an error.

### key

`key` names the entry in `keys.toml` to use. An empty string, or no
`key` at all, means `default`. `catalog status` and the daemon's errors
never print the value of `key`, since a person could paste the API key
itself there by mistake.

## .catalog/config.toml

A folder inside a watched folder is a catalog when its `.catalog/`
folder holds a regular file named `config.toml`. The file can be empty
or hold only comments. A symbolic link or a folder named `config.toml`
doesn't count, and neither does a `.catalog/` folder the daemon can't
list. A `.catalog/` folder without a `config.toml` isn't a catalog: the
daemon stops updating any `catalog.md` left in it and doesn't delete it.
[`catalog init`](commands.md#catalog-init) writes a `config.toml` that
sets nothing and explains each setting in comments; a person can also
create the file.

The daemon reads two keys from it:

| Key       | Type             | Default | Meaning                                                                              |
| --------- | ---------------- | ------- | ------------------------------------------------------------------------------------ |
| `include` | array of strings | none    | Patterns for more files this catalog covers, added to the global and built-in lists. |
| `exclude` | array of strings | none    | Patterns for files this catalog leaves out, added to the global and built-in lists.  |

```toml
include = ["**/*.go"]
exclude = ["notes/private/**"]
```

The patterns are relative to this catalog's folder, the one that holds
`.catalog/`. A catalog's config can't set `watch`, `model`, or `key`,
and the daemon rejects one that holds any of them, or an API key pasted
as a key's name, as an [unknown key](#unknown-keys). It holds no secret,
so it can be shared, copied, or committed with the folder.

A catalog's `include` can cover more of its folder than the global lists
do, so a config that arrives with a clone decides which more of that
folder's files the daemon summarizes and sends to the person's model
service. It can't bring back a file the person excluded in
`global.toml`. [Profiles](../explanation/profiles.md) explains what the
model service receives.

## How lists combine

Three levels each contribute to `include` and `exclude`:

1. The built-in lists: include `**/*.csv`, `**/*.md`, `**/*.pdf`, and
   `**/*.txt`; exclude `**/CLAUDE.md` and `**/AGENTS.md`.
2. `global.toml`.
3. The catalog's `.catalog/config.toml`.

Each level adds to the levels before it; none replaces them. A file
belongs to a catalog if it matches any `include` pattern and no
`exclude` pattern, from any level. There are no `!` patterns, and the
order of patterns doesn't matter.

So a catalog can leave out one more file, or cover one more type of
file, with one line, and no setting can bring back a file that an
`exclude` at any level leaves out. If `global.toml` has
`exclude = ["secrets.txt"]` and a catalog's config has
`include = ["secrets.txt"]`, the catalog leaves `secrets.txt` out. No
setting can include `CLAUDE.md` or `AGENTS.md`.

`README.md` is in the built-in `include` list through `**/*.md`. To
leave a catalog's README out, add `exclude = ["README.md"]` to its
config, or `exclude = ["**/README.md"]` to `global.toml` for every
README everywhere. To leave out a type of file the built-in list
includes, exclude it the same way, for example `exclude = ["**/*.csv"]`.

These rules apply to the matching:

- Patterns are matched against each file's path relative to the
  catalog's folder, written with `/` separators.
- `*` and `?` match within one path segment, as in Go's `path.Match`,
  which also reads `[...]` as a character class. A `**` segment matches
  zero or more segments.
- Only regular files match. The daemon never follows a symbolic link to
  a file.
- Whatever the patterns say, a catalog never includes anything under a
  `.git` folder, anything under a `.catalog` folder, or anything under a
  folder that is a catalog of its own.

## Unknown keys

A key the daemon doesn't read makes `global.toml` or a catalog's
`config.toml` fail to load. Otherwise a misspelled `exclude` would leave
in the files it meant to leave out, and the daemon would send them to
the model service. `keys.toml` has no unknown keys, since any name there
is a key's name.

The error gives the line and never the key's text, since a person may
have pasted an API key where a key's name belongs. If a key the file can
hold is within two one-letter changes of the unknown one, the error
suggests it:

```text
parse config.toml: line 3: unknown key; did you mean exclude?
```

An API key left in `global.toml` under a name the file doesn't read
gives `parse global.toml: line <n>: unknown key`, with a suggestion only
if that name is close to one the file reads. An API key pasted as the
value of `key` is taken as a name, so the daemon reports that
`keys.toml` has no key under the name `global.toml` asks for, without
quoting it. A file that has an unknown key is handled like any other
file that doesn't load, as
[When a file doesn't load](#when-a-file-doesnt-load) describes.

## When edits take effect

The daemon rereads `global.toml`, `keys.toml`, and every catalog's
`.catalog/config.toml` on each crawl. It crawls when it starts and then
once a minute, so an edit takes effect by the next crawl, without a
restart. A crawl that comes due while the daemon is summarizing a file
starts when that file is done, so an edit can wait longer than a minute.
`catalog daemon --once` reads `global.toml` and `keys.toml` once, at the
start, and the catalog configs during its one crawl.

Changing `model` also changes what counts as a current summary, so the
daemon summarizes every file again under the new model.

## When a file doesn't load

A file that can't be read or doesn't parse doesn't stop the daemon. It
records the error in its journal and shows it in `catalog status` as a
`Config error` line, and it keeps the last settings that loaded:

- If `global.toml` fails, the daemon keeps the last `global.toml` that
  loaded, and doesn't read `keys.toml` until `global.toml` loads again.
  Before one has loaded, it has no settings, so it watches no folder and
  summarizes nothing.
- If `keys.toml` fails, the daemon keeps the key it loaded last, if any.
- If `global.toml` or `keys.toml` is world-accessible, the daemon also
  drops the API key it loaded, and summarizes nothing until you run
  `chmod o-rwx` on the file. It notices the mode when it rereads the
  file on its next crawl, as
  [When edits take effect](#when-edits-take-effect) describes, so it can
  go on summarizing with the old key until then. The `Config error` line
  gives the mode in the form `ls -l` shows, for example
  `world-accessible (mode -rw-r--r--); to fix it, run chmod o-rwx on it`.
- If a catalog's `.catalog/config.toml` fails, the daemon keeps that
  catalog's `include` and `exclude` from the last config that loaded,
  combined with the current global lists. If none has loaded since the
  daemon started, it skips the catalog until the file loads. Its
  `Config error` line names the catalog's folder.

The errors never quote a value from any of the three files. When a file
isn't valid TOML, the error gives the line, and the last key the parser
read if that is a key the file can hold, for example
`parse global.toml: line 4 (last key "exclude"): not valid TOML`, or
`a value has the wrong type` in place of `not valid TOML`.

The daemon records a config error in the journal once, and records it
again only after a crawl in which the problem was gone.
