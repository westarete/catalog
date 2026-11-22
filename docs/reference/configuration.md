# Configuration

The daemon reads settings from two kinds of file: one `global.toml` that
applies to every catalog on the Mac, and an optional
`.catalog/config.toml` inside each catalog. This page lists where they
live, every key the daemon reads, its type and default, how a catalog's
settings override the global ones, and how soon edits take effect. For
why catalogs and profiles work this way, see
[where catalogs go](../explanation/catalogs.md) and
[profiles](../explanation/profiles.md); for the commands that read these
files, see [commands](commands.md).

Both files are in [TOML](https://toml.io/). The daemon ignores a key it
doesn't read, in either file, so a file with extra keys still works.

## The state folder

The daemon keeps its settings file and its own state in one folder:

```text
~/Library/Application Support/com.westarete.catalog
```

The program takes the home folder from `$HOME`. `catalog daemon` and
`catalog daemon --once` create the folder with mode 0700 and set that
mode if it already exists.

## global.toml

The file is `global.toml` in the state folder. The daemon never creates
it. [`catalog setup`](commands.md#catalog-setup) creates it with mode
0600 if it's missing, and a person can write it instead. The daemon
never changes its mode, but because the file holds the API key, the
daemon won't use it while it is world-readable or world-writable. Group
read and write are allowed, so you can share the file with a group on
purpose. If you wrote the file yourself, restrict it with `chmod 600`.
The daemon reads its API key only from this file, never from the
`ANTHROPIC_API_KEY` environment variable or any other environment
variable.

| Key                  | Type                                         | Default                                           | Meaning                                                                       |
| -------------------- | -------------------------------------------- | ------------------------------------------------- | ----------------------------------------------------------------------------- |
| `api_key`            | string                                       | none                                              | The key the daemon sends with each request to the model service.              |
| `model`              | string                                       | `claude-haiku-5-5`                                | The model that writes summaries. An empty string gives the default.           |
| `enumerate`          | array of strings                             | `["**/*.csv", "**/*.md", "**/*.pdf", "**/*.txt"]` | Globs for the files a catalog covers.                                         |
| `ignore`             | array of strings                             | `["README.md", "**/CLAUDE.md", "**/AGENTS.md"]`   | Globs for files to leave out, even if `enumerate` matches them.               |
| `[[watch]]` `folder` | array of tables, each with a string `folder` | none                                              | A folder to crawl for catalogs. Repeat the `[[watch]]` table for each folder. |

An example, with a made-up key:

```toml
api_key = "sk-example-not-a-real-key"
model = "claude-haiku-5-5"
enumerate = ["**/*.md", "**/*.pdf"]

[[watch]]
folder = "~/src"

[[watch]]
folder = "/Users/alex/notes"
```

### api_key

Without a key the daemon still crawls and writes each catalog's listing,
but it summarizes nothing, and `catalog status` says
`Not summarizing: no API key loaded from global.toml`. An empty
`api_key` counts as no key, and a missing one adds a
`Config error: <path>: no api_key` line to the status, where `<path>` is
the path of `global.toml`. `catalog daemon --once` refuses to run
without one.

When the key or the model changes, the daemon starts using the new one
and ends any pause that was waiting out a problem with the old one.

### watch

The daemon crawls each `folder` for `.catalog/` folders. With no
`[[watch]]` table, it crawls nothing.

- A `folder` that is `~`, or starts with `~/`, has that part replaced
  with the home folder. A path starting with `~` and a user name, such
  as `~other/x`, isn't expanded.
- A `folder` must be an absolute path after that. A relative one is
  skipped, and `catalog status` shows a `Config error` for it:
  `watch folder must be an absolute path or start with ~/; skipped`. An
  empty `folder` is skipped without an error.

### enumerate and ignore

A file belongs to a catalog if it matches an `enumerate` glob and no
`ignore` glob. These rules apply to the matching:

- Globs are matched against each file's path relative to the catalog's
  folder, the one that holds `.catalog/`, written with `/` separators.
- `*` and `?` match within one path segment, as in Go's `path.Match`,
  which also reads `[...]` as a character class. A `**` segment matches
  zero or more segments.
- Only regular files match. The daemon never follows a symbolic link to
  a file.
- Regardless of the globs, a catalog never includes anything under a
  `.git` folder, anything under a `.catalog` folder, or anything under a
  folder with a `.catalog/` of its own.

## .catalog/config.toml

The file is `.catalog/config.toml` in the folder of a catalog. It is
optional and can be empty. The daemon reads two keys from it:

| Key         | Type             | Default                | Meaning                                  |
| ----------- | ---------------- | ---------------------- | ---------------------------------------- |
| `enumerate` | array of strings | the global `enumerate` | Globs for the files this catalog covers. |
| `ignore`    | array of strings | the global `ignore`    | Globs for files this catalog leaves out. |

The globs mean the same as in `global.toml`, relative to this catalog's
folder.

### What a catalog's config can't hold

The daemon reads no key from a catalog's config other than `enumerate`
and `ignore`. A server, a credential, a model, and a list of folders to
watch have no effect there: the daemon doesn't read `api_key`, `model`,
or `watch` from a catalog's config, and it sends nothing to any service
the file names. A catalog's config can come from someone else, for
example in a clone, so settings that decide where file content goes stay
in `global.toml`, which only the person who runs the daemon writes. A
catalog's `enumerate` can cover more of its folder than the global list
does, so a config that arrives with a clone decides which of that
folder's files the daemon summarizes and sends to the person's provider.
The daemon sends requests only to the model service, with the key from
`global.toml`. [Profiles](../explanation/profiles.md) explains what that
service receives.

## How settings combine

Each layer overrides the one before it:

1. The defaults built into the daemon.
2. `global.toml`.
3. The catalog's `.catalog/config.toml`.

A list replaces the list before it; it never adds to it. If a catalog's
config sets `enumerate`, the global `enumerate` doesn't apply to that
catalog at all, and the same holds for `ignore`. A key that is present
replaces the earlier list even when its list is empty, and a key that is
absent leaves it alone. The two lists are independent: setting `ignore`
in a catalog's config leaves the global `enumerate` in force.

## When edits take effect

The daemon rereads `global.toml` and every catalog's
`.catalog/config.toml` on each crawl. It crawls when it starts and then
once a minute, so an edit takes effect by the next crawl, without a
restart. A crawl that comes due while the daemon is summarizing a file
starts when that file is done, so an edit can wait longer than a minute.
`catalog daemon --once` reads `global.toml` once, at the start, and the
catalog configs during its one crawl.

Changing `model` also changes what counts as a current summary, so the
daemon summarizes every file again under the new model.

## When a file doesn't load

A file that can't be read or doesn't parse doesn't stop the daemon. It
records the error in its journal and shows it in `catalog status` as a
`Config error` line, and it keeps the last settings that loaded:

- If `global.toml` fails, the daemon keeps the last `global.toml` that
  loaded. Before one has loaded, it has no settings, so it watches no
  folder and summarizes nothing.
- If `global.toml` is world-readable or world-writable, the daemon also
  drops the API key it loaded, and summarizes nothing until you run
  `chmod o-rw` on the file. The `Config error` line gives the file's
  mode.
- If a catalog's `.catalog/config.toml` fails, the daemon keeps the
  globs from the last config that loaded for that catalog. If none has
  loaded since the daemon started, it skips the catalog until the file
  loads.
- A missing `.catalog/config.toml` isn't an error. The catalog uses the
  global lists.

The daemon records a config error in the journal once, and records it
again only after a crawl in which the problem was gone.
