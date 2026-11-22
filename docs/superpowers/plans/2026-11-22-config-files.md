# Config Files Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the daemon's config format with three files
(`keys.toml`, `global.toml`, `.catalog/config.toml`), lists that add up
across levels, rejected unknown keys, `.catalog/config.toml` as the
catalog marker, and a `catalog init` command.

**Architecture:** The `settings` package owns every file format and
every rule for combining them; the daemon, `cmd/catalog`, and `install`
call it. `trees` decides what is a catalog. `install` writes files for
`setup` and `init`. No component reads the old format.

**Tech Stack:** Go, BurntSushi/toml v1.6.0, Gremlins for mutation
testing (`make mutate PKG=...`).

**Spec:** `docs/superpowers/specs/2026-11-22-config-files-design.md`.
Read it before any task; this plan names the code, the spec says what it
must do.

## Global Constraints

- Work on branch `claude/config-files` in this worktree. Commit after
  each task; CLAUDE.md allows commits on a feature branch without
  approval. Run `make test` and `make markdown` before every commit.
- No error, status line, or journal entry may contain a value from
  `global.toml`, `keys.toml`, or a catalog's `config.toml`, or the text
  of an unknown key or of any name in `keys.toml`. Paths, line numbers,
  and the names of known keys are allowed.
- `keys.toml` and `global.toml` are read only through the private-file
  reader: regular file, opened with `O_NONBLOCK`, refused while any
  world bit (`0o007`) is set, with the existing `ErrNotRegularFile` and
  `ErrWorldAccessible` errors.
- Files `setup` creates are mode 0600 whatever the umask, created with
  `O_EXCL`, removed if writing fails, and never overwritten.
- Built-in lists: include `**/*.csv`, `**/*.md`, `**/*.pdf`, `**/*.txt`;
  exclude `**/CLAUDE.md`, `**/AGENTS.md`. `README.md` is no longer
  excluded.
- Key names, read by the daemon: `global.toml`: `watch`, `model`, `key`,
  `include`, `exclude`. `.catalog/config.toml`: `include`, `exclude`.
  `keys.toml`: any name, string values only. The key name used when
  `global.toml` sets no `key` is `default`.
- Run Gremlins over each package a task changes. A mutant no test kills
  is a missing test, except a default value the tests replace on
  purpose.
- Docs follow `writing-style.md`; examples use made-up names
  (`/Users/alex`, `~/src/acme/website`).

## Review Focus

- **A value or pasted key reaches an error.** A key pasted as a bare
  TOML key name in any of the three files, an `api_key` left in
  `global.toml`, and a `key = "..."` in `global.toml` that names a
  missing entry all produce errors with a line or fixed text, never the
  text. Tests in Tasks 1, 3, and 4 assert the secret string is absent
  from the error.
- **Odd characters in what setup writes.** A folder or key containing
  `"`, `\`, a space, or non-ASCII text must read back unchanged. Task 2
  and Task 3 round-trip such values through the parser.
- **Mode under a permissive umask.** `keys.toml` written by setup is
  0600 with umask 000, in both the key and the comment-only case. Task 2
  asserts it.
- **A catalog's config can't undo a person's exclusion.** A catalog
  `include = ["secrets.txt"]` with a global `exclude = ["secrets.txt"]`
  leaves the file out. Task 3 asserts it.
- **Privacy and security, for every task reviewer:** check that no new
  error or status text quotes a value or a key's text, that the key is
  read only from `keys.toml` through the private-file reader, that every
  file setup creates is 0600 under any umask, and that nothing new is
  sent over a network. Copy this line into each task reviewer's brief.

---

### Task 1: Read keys.toml

**Files:**

- Create: `internal/settings/keys.go`, `internal/settings/keys_test.go`
- Modify: `internal/settings/settings.go` (extract the private-file
  reader from `ReadGlobalFile`)

**Interfaces:**

- Produces: `const KeysFile = "keys.toml"`;
  `const DefaultKeyName = "default"`; `type Keys map[string]string`;
  `func ReadKeysFile(path string) (Keys, error)`;
  `func ParseKeys(data []byte) (Keys, error)`;
  `func readPrivateFile(path string) ([]byte, error)`, which
  `ReadGlobalFile` now uses.

- [ ] **Step 1: Write the failing tests** in `keys_test.go`:
  - `TestParseKeysReadsNamedKeys`: `default = "sk-a"\nacme = "sk-b"`
    gives `Keys{"default": "sk-a", "acme": "sk-b"}`.
  - `TestParseKeysCommentOnly`: a file of comments gives an empty map
    and no error.
  - `TestParseKeysErrorsQuoteNothing`: for `sk-secret-1`,
    `sk-secret-2 = 3`, and `default = sk-secret-3`, the error is
    non-nil, contains `line` and a number, and contains no `sk-secret`.
  - `TestReadKeysFileRefusesWorldAccessible` and
    `TestReadKeysFileRefusesNonRegular`: as the existing
    `ReadGlobalFile` tests, with `errors.Is` on the existing errors.
- [ ] **Step 2: Run** `go test ./internal/settings/`. Expected: build
      fails, `ParseKeys` undefined.
- [ ] **Step 3: Implement.** Move the open, stat, regular-file, and mode
      checks from `ReadGlobalFile` into `readPrivateFile`. `ParseKeys`
      decodes into `map[string]any`; a non-string value is an error
      naming the line only. Parse errors give
      `parse keys.toml: line N: not valid TOML` with no key, using
      `toml.ParseError`'s position.
- [ ] **Step 4: Run** `go test ./internal/settings/`. Expected: PASS.
      Then `make mutate PKG=./internal/settings`.
- [ ] **Step 5: Commit** "Read named API keys from keys.toml".

### Task 2: Move the API key to keys.toml

**Files:**

- Modify: `internal/settings/settings.go` (`Global` loses `APIKey`,
  gains `Key`; `api_key` is no longer read),
  `internal/daemon/global.go`, `internal/daemon/daemon.go`,
  `internal/daemon/snapshot.go`, `cmd/catalog/main.go`,
  `cmd/catalog/setup.go`, `internal/install/settings.go`,
  `internal/install/describe.go`, and the tests of each.

**Interfaces:**

- Consumes: Task 1.
- Produces:
  - `settings.Global.Key string` (empty means `DefaultKeyName`);
    `func (g Global) KeyName() string`.
  - `daemon.Daemon.KeysPath string`; the daemon keeps the loaded key in
    an unexported field and rebuilds the Provider when the key or
    `Model` changes.
  - `install.WriteKeys(path, apiKey string) error`;
    `install.WriteGlobal(path string, folders []string) error`, which no
    longer takes a key;
    `install.AskSettings(p Prompt, globalPath, keysPath string) error`.
  - `install.Keys{Path string; Found, HasKey bool; Err error}`;
    `install.Describe(home string, s Settings, k Keys, onMac bool) string`.
    `install.Settings` drops `HasKey`.

- [ ] **Step 1: Write the failing tests.**
  - Daemon (`run_test.go`, `snapshot_test.go`): update the
    `useGlobal`/`writeGlobal` helpers to write the key to a `keys.toml`
    beside `global.toml`. New tests:
    - `TestRunUsesKeyNamedInGlobal`: `key = "acme"` with `keys.toml`
      holding `acme = "key-2"` builds a Provider with `key-2`.
    - `TestRunMissingKeyNameIsConfigError`: `key = "sk-pasted"` with no
      such entry gives a config error on `keys.toml` that contains no
      `sk-pasted`, and no Provider.
    - `TestRunStopsWhileKeysIsWorldAccessible`: as the existing global
      test, for `keys.toml`.
  - Install (`settings_test.go`):
    - `TestWriteKeysModeUnderUmask` (umask 000, mode 0600, with a key
      and without).
    - `TestWriteKeysRoundTrip`, with a key containing `"`, `\`, and `é`,
      read back through `settings.ParseKeys`.
    - `TestAskSettingsAsksOnlyForMissingFiles`, for the four
      combinations of each file existing or not, asserting the prompts
      shown and the files written.
    - `TestAskSettingsSkippedKeyWritesCommentOnlyKeys`.
  - Describe (`describe_test.go`): a `Keys:` line with `(key set)`,
    `(no key)`, `(not found; run catalog setup)`, and
    `(can't be read: ...)`; the `Settings:` line says `(valid)` when it
    loads.
  - Command (`main_test.go`): `--once` refuses with no `keys.toml` and
    with a world-accessible one; status shows the `Keys:` line.
- [ ] **Step 2: Run** `go test ./...`. Expected: failures and build
      errors in the packages above.
- [ ] **Step 3: Implement.**
  - `settings`: remove `api_key` from `globalFile`, add `key`, and add
    `key` to the known keys in `globalParseError`.
  - `daemon`: `reloadGlobal` reads `GlobalPath` and then `KeysPath`, and
    reports keys errors on the `keys.toml` path. Replace `errNoAPIKey`
    with `errors.New("no key under the name global.toml asks for")`. On
    `ErrWorldAccessible` from either file, drop the key. `noProvider`
    becomes `"no API key loaded from keys.toml"`.
  - `cmd/catalog`: replace `loadGlobalFile` with
    `loadSettings(stateDir string) (settings.Global, string, error)`,
    which returns the key. `runLoop` sets `KeysPath`. `settingsState`
    also reports keys, using `g.KeyName()`, or `DefaultKeyName` when
    `global.toml` doesn't load.
  - `install`: extract
    `createPrivateFile(path string, write func(io.Writer) error) error`
    from `WriteGlobal` and use it for both files.
    - `WriteKeys` writes `default = "<key>"`, or, for an empty key, this
      comment:

      ```toml
      # Your Anthropic API keys, one per line, by name. The daemon uses
      # the one named default unless global.toml names another with key.
      # default = "your-key"
      ```

    - Quote the key with a new `tomlString(s string) string`, which
      writes a TOML basic string: escape `\`, `"`, and control
      characters; keep everything else.
    - `AskSettings` messages:
      - `Your settings are already in %s. To change them, edit that file.`
      - `Your API keys are already in %s. To change them, edit that file.`
      - `Your API key is in %s, and only you can read that file.`
      - `To add your API key later, set default in %s.`
- [ ] **Step 4: Run** `make test`. Expected: PASS. Then `make mutate`
      for `./internal/settings`, `./internal/install`, and
      `./internal/daemon`.
- [ ] **Step 5: Commit** "Keep API keys in keys.toml, apart from
      global.toml".

### Task 3: Watch list, include and exclude, and lists that add up

**Files:**

- Modify: `internal/settings/settings.go`, `internal/daemon/crawl.go`,
  `internal/daemon/daemon.go` (`globs` fields become
  `include, exclude`), `internal/trees/trees.go` (`Members` parameters
  renamed), `internal/install/settings.go` (`WriteGlobal`), and their
  tests.

**Interfaces:**

- Consumes: Task 2's `tomlString`.
- Produces:
  - `settings.Global{Model, Key string; Include, Exclude, Watch []string}`,
    where `Include` and `Exclude` hold only what `global.toml` says.
  - `type Catalog struct{ Include, Exclude []string }`;
    `func ParseCatalog(data []byte) (Catalog, error)`.
  - `var BuiltinInclude, BuiltinExclude []string`.
  - `func Lists(g Global, c Catalog) (include, exclude []string)`, which
    returns new slices holding built-in, then global, then catalog
    patterns.
  - `ApplyCatalog` is removed. `ParseCatalog`'s errors take the same
    place-only form as `global.toml`'s, such as
    `parse config.toml: line 2: not valid TOML`, since today's
    `parse catalog config: %w` passes on the decoder's message, which
    can quote a value.

- [ ] **Step 1: Write the failing tests.**
  - `settings_test.go`:
    - `TestParseGlobalWatchList`: `watch = ["~/a", "/b"]` expands `~`,
      as today.
    - `TestListsAddUp`: a global `exclude = ["drafts/**"]` and a catalog
      `exclude = ["notes.md"]` give both, plus the built-ins.
    - `TestCatalogIncludeCannotUndoGlobalExclude`, checked through
      `trees.Members` on an `fstest.MapFS`: catalog
      `include = ["secrets.txt"]`, global `exclude = ["secrets.txt"]`,
      and the file is not a member.
    - `TestBuiltinsNoLongerExcludeREADME`.
    - `TestParseCatalogErrorQuotesNothing`: `include = [sk-secret-4`
      gives an error containing `line` and no `sk-secret`.
      `TestListsDoesNotAliasInputs`, which replaces
      `TestCrawlDoesNotMutateGlobalLists`'s purpose.
  - `install/settings_test.go`: `WriteGlobal` writes `watch` one folder
    per line, and a folder with `"`, `\`, a space, and `é` reads back
    unchanged through `settings.ParseGlobal`.
- [ ] **Step 2: Run** `go test ./...`. Expected: failures.
- [ ] **Step 3: Implement.**
  - `globalFile.Watch` becomes `[]string`, and `enumerate`/`ignore`
    become `include`/`exclude`.
  - `catalogGlobs` calls `ParseCatalog` and then `Lists`, keeping the
    last good lists per catalog as today.
  - `WriteGlobal(path string, folders []string)` writes the file itself,
    with `tomlString`:

    ```toml
    watch = [
      "~/src",
    ]
    ```

  - Update every test fixture that writes `[[watch]]`, `enumerate`,
    `ignore`, or `api_key`.
- [ ] **Step 4: Run** `make test`. Expected: PASS. Then `make mutate`
      for `./internal/settings`, `./internal/trees`, and
      `./internal/install`.
- [ ] **Step 5: Commit** "Make watch a list and let include and exclude
      add up".

### Task 4: Reject unknown keys

**Files:**

- Create: `internal/settings/unknown.go`,
  `internal/settings/unknown_test.go`
- Modify: `internal/settings/settings.go` (`ParseGlobal` and
  `ParseCatalog` call the check)

**Interfaces:**

- Produces:
  `func unknownKeyError(file string, data []byte, md toml.MetaData, known []string) error`,
  which returns nil when `md.Undecoded()` is empty.

- [ ] **Step 1: Write the failing tests:**
  - `TestUnknownKeyGivesLineAndSuggestion`: `exlude = []` on line 3 of a
    catalog config gives exactly
    `parse config.toml: line 3: unknown key; did you mean exclude?`.
  - `TestUnknownKeyNoSuggestionWhenFar`: `colour = 1` gives
    `line 1: unknown key` with no suggestion.
  - `TestUnknownKeyQuotesNothing`: `sk-pasted-secret = "x"` in
    `global.toml`, and `api_key = "sk-secret"` in `global.toml`, each
    give an error containing neither `sk-`.
  - `TestUnknownKeyInTable`: `[watch]\nfolder = "x"` reports the line of
    `[watch]`.
  - `TestUnknownKeyRejectsWholeFile`: the parse returns the error and no
    settings.
- [ ] **Step 2: Run** `go test ./internal/settings/`. Expected: FAIL.
- [ ] **Step 3: Implement.**
  - For the first undecoded key, find its line: the first line whose
    text before `=`, trimmed of whitespace and quotes, equals the key's
    last part, or a line `[key]` or `[[key]]` for a table.
  - Suggest the known key with the smallest Levenshtein distance, if
    that distance is 2 or less.
  - Report only the first unknown key.
- [ ] **Step 4: Run** `make test`, then
      `make mutate PKG=./internal/settings`. Expected: PASS.
- [ ] **Step 5: Commit** "Reject keys the daemon doesn't read".

### Task 5: Make .catalog/config.toml the marker

**Files:**

- Modify: `internal/trees/trees.go` (`FindCatalogs`),
  `internal/trees/trees_test.go`, and daemon and command test fixtures
  that make a catalog with an empty `.catalog/`.

**Interfaces:**

- Produces: `FindCatalogs(watched fs.FS) ([]string, error)` keeps its
  signature. A folder is found only if its `.catalog/` holds a regular
  file `config.toml`.

- [ ] **Step 1: Write the failing tests** in `trees_test.go`, on an
      `fstest.MapFS`, plus a symbolic link case using a temp folder with
      `os.DirFS`:
  - `TestFindCatalogsNeedsConfig`: an empty `.catalog/` is not found.
  - `TestFindCatalogsIgnoresLinkedConfig`: `config.toml` as a symbolic
    link is not found.
  - `TestFindCatalogsNested`: two configs, one nested, both found.
- [ ] **Step 2: Run** `go test ./internal/trees/`. Expected: FAIL.
- [ ] **Step 3: Implement.** On a `.catalog` directory, list it with
      `fs.ReadDir` and require an entry named `config.toml` whose
      `Type().IsRegular()`. Keep skipping `.git` and the inside of
      `.catalog`. Update `makeCatalog` and similar helpers to write an
      empty `config.toml`.
- [ ] **Step 4: Run** `make test`, then
      `make mutate PKG=./internal/trees`. Expected: PASS.
- [ ] **Step 5: Commit** "Make a catalog's config.toml what marks it".

### Task 6: catalog init

**Files:**

- Create: `internal/install/catalog.go`,
  `internal/install/catalog_test.go`
- Modify: `cmd/catalog/main.go` (usage and dispatch),
  `cmd/catalog/setup.go` or a new `cmd/catalog/init.go`, and
  `cmd/catalog/main_test.go`

**Interfaces:**

- Produces:
  - `func InitCatalog(folder string, out io.Writer) error`.
  - `func InGitRepository(folder string) bool`, which looks for a `.git`
    entry, a folder or a file, in `folder` or any parent, without
    running `git`.
  - The usage gains `catalog init [folder]`.

- [ ] **Step 1: Write the failing tests:**
  - `TestInitCreatesConfig`: a commented template that
    `settings.ParseCatalog` accepts.
  - `TestInitWritesGitignoreOnlyInRepository`: with a `.git` folder, and
    with a `.git` file as in a worktree, `.gitignore` is exactly
    `/catalog.md\n/catalog-*.tmp\n`; without `.git`, there's no
    `.gitignore`.
  - `TestInitLeavesExistingFiles`: existing `config.toml` and
    `.gitignore` bytes are unchanged, and the output names each file
    left alone.
  - `TestInitSaysWhetherWatched`, in `main_test.go`: the output says
    whether the folder is inside a folder `global.toml` watches, and
    says nothing about it when `global.toml` doesn't load.
  - `TestInitMissingFolder`: refuses with an error that names the
    folder; it creates no folder of its own.
- [ ] **Step 2: Run** `go test ./internal/install/ ./cmd/catalog/`.
      Expected: FAIL.
- [ ] **Step 3: Implement.**
  - Write the files with `O_EXCL` and the default mode, since they're
    meant to travel with the folder.
  - The template lists `include` and `exclude`, commented out, each with
    one line on what it does and that it adds to the global list.
  - The output ends with the line an instruction file needs:
    `@.catalog/catalog.md`.
- [ ] **Step 4: Run** `make test`, then
      `make mutate PKG=./internal/install`. Expected: PASS.
- [ ] **Step 5: Commit** "Add catalog init".

### Task 7: Docs

**Files:**

- Modify:
  - `docs/reference/configuration.md` (rewrite)
  - `docs/reference/commands.md` (`catalog init`; setup's prompts and
    messages; the `Keys:` and `Settings:` lines)
  - `docs/reference/files.md` (`keys.toml`, `.catalog/.gitignore`)
  - `docs/how-to/install-and-run.md` (setup output; putting a catalog in
    place with `catalog init`; drop the global ignore advice)
  - `docs/explanation/catalogs.md` (what a tree needs)
  - `docs/explanation/architecture.md` (Settings: one short passage on
    why keys live apart and why lists add up)
  - `docs/how-to/move-from-the-earlier-tool.md` (key names only)

- [ ] **Step 1: Load `knowledge-work:markdown`**, and follow
      `writing-style.md`.
- [ ] **Step 2: Rewrite the docs** listed above to match the spec's "How
      it works", using made-up examples.
- [ ] **Step 3: Check** that
      `grep -rn "enumerate\|\[\[watch\]\]\|api_key" README.md docs --include='*.md' | grep -v superpowers`
      prints nothing, then run `make markdown`.
- [ ] **Step 4: Commit** "Document keys.toml, the new settings, and
      catalog init".

### Task 8: Review and finish

- [ ] **Step 1:** Run `/security-review` over the branch and a
      whole-branch code review, and fix what they find.
- [ ] **Step 2:** File any undone steps and deferred findings as issues,
      delete this plan, and commit.

Converting the developer's own `global.toml` and five catalog configs
happens after the release, with the developer, and isn't part of this
branch.
