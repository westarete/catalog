# First Catalog Daemon Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `catalog daemon`, `catalog daemon --once`, and
`catalog status`: a launchd agent that keeps `.catalog/catalog.md`
current in every `.catalog/` folder inside the folders a person watches.

**Architecture:** One Go module with a package per component from
architecture.md under `internal/`, and a thin `cmd/catalog`. One
goroutine owns all state and runs crawl → plan → read → profile → render
→ publish, one file at a time; a second goroutine answers status over a
Unix socket from snapshots. Tests swap in a fake file system
(`testing/fstest.MapFS` for reads, `t.TempDir()` for writes), a fixed
clock (a `time.Time` argument), and a fake model provider.

**Tech Stack:** Go 1.25; `github.com/BurntSushi/toml`;
`modernc.org/sqlite` (no cgo); `github.com/anthropics/anthropic-sdk-go`;
Gremlins for mutation testing; launchd.

**Spec:**
[docs/superpowers/specs/2026-11-15-daemon-mvp-design.md](../specs/2026-11-15-daemon-mvp-design.md)

## Global Constraints

- Module `westarete.com/catalog`, `go 1.25.0` in `go.mod`.
- Write code against a dependency only from its current documentation
  (`go doc`, the module source in `$(go env GOMODCACHE)`, or the
  project's docs), never from memory. Today's tool (commit `d9a8d1b`)
  compiled against `anthropic-sdk-go` v1.51.0, `modernc.org/sqlite`
  v1.56.0, and `BurntSushi/toml` v1.6.0; use `go get <module>@latest`
  and check the API still matches.
- The daemon writes nothing inside a watched folder except
  `.catalog/catalog.md` (via a temporary file in `.catalog/`). It never
  creates a `.catalog/` folder.
- All daemon state lives in
  `~/Library/Application Support/com.westarete.catalog/`: `global.toml`,
  `store.db`, `journal.jsonl`, `control.sock`.
- The API key comes only from `global.toml`. A catalog's
  `.catalog/config.toml` may set only `enumerate` and `ignore`; any
  other key there is ignored.
- Built-in defaults:
  `enumerate = ["**/*.csv", "**/*.md", "**/*.pdf", "**/*.txt"]`,
  `ignore = ["README.md", "**/CLAUDE.md", "**/AGENTS.md"]`,
  `model = "claude-haiku-5-5"`. A list in a later layer replaces the
  earlier list.
- Timing: crawl every 1 minute; a file must be unchanged for 3 minutes
  before it's summarized; retry backoff starts at 1 minute, doubles, and
  stays at 6 hours.
- Reader: text cut at 400,000 bytes; listing shows the first 10 lines,
  cut at 512 bytes; PDFs sent whole, no first lines.
- Before every commit, run `make markdown` and `make test`. Run
  `gremlins unleash` on each new package and add tests for every mutant
  that lives. Commit messages follow Tim Pope's style (imperative
  subject, ~50 characters, body explaining why); no attribution
  trailers.

## Review Focus

1. A file deleted between the crawl listing it and the loop reading it:
   the daemon drops it and carries on (test in Task 4).
2. A file that matches a glob but isn't valid UTF-8, such as a binary
   `.txt`: its listing shows no broken characters and the daemon doesn't
   crash (test in Task 7).
3. A file the daemon can't read because of its permissions: it's
   recorded as a failure for that file and every other file proceeds
   (test in Task 11).
4. A `.catalog/` folder the daemon can't write into: the error shows in
   status, the old `catalog.md` stays, and other catalogs still publish
   (test in Task 8 and Task 11).
5. A malformed `.catalog/config.toml` in one catalog: that catalog keeps
   its last good settings, status shows the error, and other catalogs
   are unaffected (test in Task 2).

---

### Task 1: Mutation testing tool and doc cleanup

**Files:**

- Modify: `Makefile`, `SETUP.md`, `README.md`
- Delete: `docs/design/mvp.md`

- [ ] **Step 1:** Install Gremlins with
      `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`
      (check the [install page](https://gremlins.dev/latest/install/)
      for a newer version first). Run `gremlins --version`; expect a
      version string.
- [ ] **Step 2:** Add a `mutate` target to `Makefile` that runs
      `gremlins unleash` for one package given as `PKG`, e.g.
      `make mutate PKG=./internal/planner`. Add Gremlins to `SETUP.md`'s
      install list with the command from Step 1.
- [ ] **Step 3:** Delete `docs/design/mvp.md`. In `README.md`, replace
      the sentence that links mvp.md with one that links the spec.
- [ ] **Step 4:** Run `make markdown`; expect "Markdown OK".
      (`make test` fails until Task 2 adds Go code; that's expected for
      this commit only.)
- [ ] **Step 5:** Commit.

### Task 2: Model and Settings

**Files:**

- Create: `internal/model/model.go`, `internal/settings/settings.go`,
  `internal/settings/settings_test.go`

**Interfaces:**

- Produces, in `internal/model`:
  - `type Hash string` (lowercase hex SHA-256)
  - `type Recipe string`
  - `type FileState struct { Rel string; Size int64; ModTime time.Time; Hash Hash; ChangedAt time.Time }`
    — `Rel` is the path relative to the catalog's folder, with `/`
    separators.
  - `type Catalog struct { Dir string; Enumerate, Ignore []string }` —
    `Dir` is the absolute folder that holds `.catalog/`.
- Produces, in `internal/settings`:
  - `type Global struct { APIKey, Model string; Enumerate, Ignore []string; Watch []string }`
  - `func ParseGlobal(data []byte) (Global, error)` — applies built-in
    defaults for unset fields; expands a leading `~/` in `Watch` against
    `os.UserHomeDir()`.
  - `func ApplyCatalog(g Global, data []byte) (enumerate, ignore []string, err error)`
    — layers a `.catalog/config.toml` over `g`; reads only `enumerate`
    and `ignore`.
  - `const GlobalFile = "global.toml"`;
    `func StateDir() (string, error)` returning
    `~/Library/Application Support/com.westarete.catalog`.

TOML shape for `global.toml`: top-level `api_key`, `model`, `enumerate`,
`ignore`, and an array of tables `[[watch]]` with key `folder`.

- [ ] **Step 1: Write the failing tests** in `settings_test.go`:
  - `TestParseGlobalDefaults`: empty input →
    `Model == "claude-haiku-5-5"`, `Enumerate` equals the four default
    globs, `Ignore` equals the three default globs.
  - `TestParseGlobalWatch`: two `[[watch]]` entries → `Watch` holds
    both, with `~/` expanded.
  - `TestApplyCatalogReplacesLists`: catalog config
    `enumerate = ["**/*.md"]` → enumerate is exactly `["**/*.md"]`,
    ignore equals the global ignore.
  - `TestApplyCatalogIgnoresOtherKeys`: catalog config with `api_key`,
    `model`, `profile`, and price keys from today's tool → no error, and
    only enumerate/ignore can change.
  - `TestApplyCatalogEmpty`: empty or comment-only file → global lists
    unchanged.
  - `TestApplyCatalogMalformed`: unparsable TOML → non-nil error (Review
    Focus 5; the daemon's use of last good settings is tested in Task
    11).
- [ ] **Step 2:** Run `go test ./internal/settings/`; expect failures
      (undefined names).
- [ ] **Step 3:** Implement with `BurntSushi/toml` (`toml.Decode` into
      structs with pointer-to-slice fields so "unset" differs from
      "empty").
- [ ] **Step 4:** Run `make test`; expect PASS. Run
      `make mutate PKG=./internal/settings`; add tests for any live
      mutant.
- [ ] **Step 5:** Commit.

### Task 3: Trees

**Files:**

- Create: `internal/trees/glob.go`, `internal/trees/trees.go`,
  `internal/trees/trees_test.go`

**Interfaces:**

- Produces:
  - `func Match(pattern, rel string) bool` — `**` spans segments, other
    segments use `path.Match`. Port `matchGlob` from today's `glob.go`
    (`git show d9a8d1b:glob.go`).
  - `func FindCatalogs(watched fs.FS) ([]string, error)` — with
    `watched` rooted at a watched folder (`os.DirFS`), every directory
    (`.` included) that contains a `.catalog` directory, as sorted slash
    paths; never descends into `.git` or `.catalog`.
  - `func Members(catalog fs.FS, enumerate, ignore, nested []string) ([]string, error)`
    — with `catalog` rooted at a catalog's folder, sorted slash paths of
    regular files that match an `enumerate` glob and no `ignore` glob,
    excluding anything under a directory in `nested` (relative to the
    catalog's folder), under `.git/`, or under `.catalog/`.

- [ ] **Step 1: Write the failing tests** with `fstest.MapFS`:
  - `TestMatch`: table from today's `glob_test.go`
    (`git show d9a8d1b:glob_test.go`).
  - `TestFindCatalogs`: `a/.catalog/`, `a/b/.catalog/`,
    `a/.git/x/.catalog/`, `c/` → `["a", "a/b"]`.
  - `TestMembersExcludesNested`: `a/doc.md`, `a/b/doc.md` with nested
    `a/b` → only `doc.md`.
  - `TestMembersSkipsCatalogAndGit`: `.catalog/catalog.md`, `.git/x.md`,
    `README.md`, `notes.md` → only `notes.md`.
  - `TestMembersWorktreeFolders`: `.claude/worktrees/w/doc.md` with no
    `.catalog/` in it → included as `.claude/worktrees/w/doc.md`.
- [ ] **Step 2:** Run `go test ./internal/trees/`; expect failures.
- [ ] **Step 3:** Implement with `fs.WalkDir`.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/trees`; expect PASS and no live
      mutants.
- [ ] **Step 5:** Commit.

### Task 4: Watcher

**Files:**

- Create: `internal/watcher/watcher.go`,
  `internal/watcher/watcher_test.go`

**Interfaces:**

- Consumes: `model.FileState`, `trees.Members` output.
- Produces:
  `func Scan(catalog fs.FS, rels []string, prev map[string]model.FileState, now time.Time) []model.FileState`
  — with `catalog` rooted at the catalog's folder, for each `rel`, stat
  it; if size and mtime match `prev`, return `prev` unchanged; otherwise
  read it, hash it with SHA-256, and set `ChangedAt = now`. A file that
  can't be stat'ed or read is left out of the result.

- [ ] **Step 1: Write the failing tests:**
  - `TestScanNewFile`: no prev → one state with the SHA-256 hex of the
    content and `ChangedAt == now`.
  - `TestScanUnchanged`: prev with the same size and mtime → returned
    state equals prev exactly (same `ChangedAt`).
  - `TestScanChanged`: different size → new hash, `ChangedAt == now`.
  - `TestScanVanished` (Review Focus 1): `rels` names a file that isn't
    in the FS → result omits it, no panic.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/watcher`.
- [ ] **Step 5:** Commit.

### Task 5: Store

**Files:**

- Create: `internal/store/store.go`, `internal/store/store_test.go`

**Interfaces:**

- Consumes: `model.FileState`, `model.Hash`, `model.Recipe`.
- Produces:
  - `func Open(path string) (*Store, error)` — opens SQLite with WAL
    journal mode and a busy timeout (set through `modernc.org/sqlite`'s
    DSN `_pragma` parameters; confirm the syntax in its docs), creates
    tables if missing.
  - `func (s *Store) Files(dir string) (map[string]model.FileState, error)`
    and
    `func (s *Store) SetFiles(dir string, files []model.FileState) error`
    (replaces the set for `dir`).
  - `func (s *Store) Summary(h model.Hash, r model.Recipe) (string, bool, error)`,
    `func (s *Store) AnySummary(h model.Hash) (text string, recipe model.Recipe, ok bool, err error)`,
    `func (s *Store) PutSummary(h model.Hash, r model.Recipe, text string) error`.
  - `type Failure struct { Count int; LastError string; RetryAt time.Time }`;
    `func (s *Store) Failure(dir, rel string) (Failure, bool, error)`,
    `RecordFailure(dir, rel string, f Failure) error`,
    `ClearFailure(dir, rel string) error`.
  - `func (s *Store) Close() error`.

- [ ] **Step 1: Write the failing tests** against
      `filepath.Join(t.TempDir(), "store.db")`:
  - `TestFilesRoundTrip`: set two files for a dir, read them back equal;
    set one, read back one.
  - `TestSummaryByHashAndRecipe`: put under recipe A; `Summary(h, A)`
    finds it, `Summary(h, B)` doesn't, `AnySummary(h)` returns it with
    recipe A.
  - `TestFailureLifecycle`: record, read back equal, clear, read back
    not found.
  - `TestReopen`: data survives `Close` and `Open` of the same path.
  - `TestWALMode`: `PRAGMA journal_mode` returns `wal`.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement with `database/sql` and
      `_ "modernc.org/sqlite"`. Schema is yours; keep summaries keyed by
      `(hash, recipe)` and files and failures keyed by `(dir, rel)`.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/store`.
- [ ] **Step 5:** Commit.

### Task 6: Planner

**Files:**

- Create: `internal/planner/planner.go`,
  `internal/planner/planner_test.go`

**Interfaces:**

- Produces:
  - `type Candidate struct { Dir, Rel string; Hash model.Hash; ChangedAt time.Time; HasCurrent, HasOlder bool; RetryAt time.Time }`
  - `func Plan(cs []Candidate, now time.Time) []Candidate` — keeps
    candidates with `!HasCurrent`,
    `now.Sub(ChangedAt) >= 3*time.Minute`, and `!now.Before(RetryAt)`;
    orders those with `!HasOlder` first, each group by `ChangedAt`
    ascending, ties by `Dir` then `Rel`.
  - `func Backoff(failures int) time.Duration` — 1 minute for the first
    failure, doubling, capped at 6 hours.

- [ ] **Step 1: Write the failing tests:**
  - `TestPlanQuietPeriod`: changed 2m59s ago → excluded; 3m ago →
    included.
  - `TestPlanSkipsCurrent`: `HasCurrent` → excluded.
  - `TestPlanRespectsRetry`: `RetryAt` one second in the future →
    excluded; equal to now → included.
  - `TestPlanOrder`: a file with `HasOlder` changed long ago comes after
    a file without a summary changed recently; within a group, older
    `ChangedAt` first.
  - `TestBackoff`: 1→1m, 2→2m, 3→4m, 9→256m, 10→6h, 50→6h.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/planner`.
- [ ] **Step 5:** Commit.

### Task 7: Reader

**Files:**

- Create: `internal/reader/reader.go`, `internal/reader/reader_test.go`

**Interfaces:**

- Produces:
  - `type Content struct { PDF bool; Text string; Data []byte; Truncated bool }`
  - `func Read(rel string, data []byte) Content` — `.pdf`
    (case-insensitive) → `PDF: true, Data: data`; otherwise text cut at
    400,000 bytes at a line boundary with today's truncation note (port
    `truncateDocument` from `git show d9a8d1b:generate.go`).
  - `type Listing struct { Type string; Size int64; FirstLines string }`
  - `func List(rel string, data []byte) Listing` — `Type` from the
    extension (e.g. `"Markdown"`, `"PDF"`, `"text"`, `"CSV"`);
    `FirstLines` is the first 10 lines cut at 512 bytes for non-PDFs,
    empty for PDFs.
  - `const Version = "1"` (part of the recipe).

- [ ] **Step 1: Write the failing tests:**
  - `TestReadTruncates`: 500,000 bytes of 100-byte lines → `Truncated`,
    text ≤ 400,000 bytes plus the note, ends at a line boundary.
  - `TestReadPDF`: `x.PDF` → `PDF` true, `Data` equal to input.
  - `TestListFirstLines`: 20 lines → exactly the first 10; a 2,000-byte
    first line → 512 bytes.
  - `TestListInvalidUTF8` (Review Focus 2): bytes `0xff 0xfe` plus text
    → `FirstLines` is valid UTF-8 (`utf8.ValidString`), no panic.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement; replace invalid UTF-8 with
      `strings.ToValidUTF8(s, "�")`.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/reader`.
- [ ] **Step 5:** Commit.

### Task 8: Renderer and Publisher

**Files:**

- Create: `internal/render/render.go`, `internal/render/render_test.go`,
  `internal/publish/publish.go`, `internal/publish/publish_test.go`

**Interfaces:**

- Consumes: `reader.Listing`.
- Produces:
  - `type Entry struct { Rel string; Summary string; Listing reader.Listing }`
    (empty `Summary` means listing only)
  - `func render.Render(entries []Entry) string` — header, then entries
    grouped under `## <dir>/` (`(root)` for top level) and `### <rel>`,
    sorted as today's `byDirectoryThenPath`
    (`git show d9a8d1b:catalog.go`). A listing-only entry's body starts
    with `Not summarized yet.` then the type, size, and first lines in a
    fenced `text` block.
  - `func publish.Publish(catalogDir, text string) (changed bool, err error)`
    — writes `.catalog/catalog.md` via a temp file in `.catalog/` and
    `os.Rename`; returns `changed=false` and writes nothing if the
    existing file has the same bytes.

The header: a `# Catalog` title, a paragraph telling the agent to read
the entries and open a file only when its task matches, one sentence
saying paths are relative to the folder that holds `.catalog/`, and one
saying an entry marked "Not summarized yet." shows only the file's
start. It must not mention `catalog update` or `catalog status`.

- [ ] **Step 1: Write the failing tests:**
  - `TestRenderGroups`: entries `b/x.md`, `a.md`, `b/y.md` →
    `## (root)/` before `## b/`, each `### <rel>` line once, in order.
  - `TestRenderListingOnly`: entry without a summary → its body starts
    with `Not summarized yet.` and contains the first lines.
  - `TestHeaderNoCommands`: output contains no `catalog update` and no
    `catalog status`.
  - `TestPublishWritesAndSkips`: first call writes the file and returns
    true; same text again returns false and leaves the mtime unchanged.
  - `TestPublishUnwritable` (Review Focus 4): `.catalog/` set to mode
    `0o555` holding an existing `catalog.md` → error returned, existing
    file's bytes unchanged.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `make test`, `make mutate PKG=./internal/render`,
      `make mutate PKG=./internal/publish`.
- [ ] **Step 5:** Commit.

### Task 9: Models and Profiler

**Files:**

- Create: `internal/models/models.go`, `internal/models/anthropic.go`,
  `internal/models/models_test.go`, `internal/profiler/profiler.go`,
  `internal/profiler/prompt.go`, `internal/profiler/profiler_test.go`

**Interfaces:**

- Consumes: `reader.Content`.
- Produces, in `internal/models`:
  - `type Request struct { System, Text string; PDF []byte; MaxTokens int64 }`
  - `type Response struct { Text string; InputTokens, OutputTokens int64 }`
  - `type Provider interface { Complete(ctx context.Context, req Request) (Response, error) }`
  - `type Kind int` with `FileProblem` and `ServiceProblem`;
    `func Classify(err error) Kind` — `*anthropic.Error` with status 400
    or 413 → `FileProblem`; any other status, a network error, or
    anything else → `ServiceProblem`. An empty model reply is a
    `FileProblem`.
  - `func NewAnthropic(apiKey, model string) Provider` —
    `client.Messages.New` with the system prompt, a text block, and for
    a PDF a base64 `NewDocumentBlock` (see today's `buildDocumentBlocks`
    and `inferProfile`, `git show d9a8d1b:generate.go`).
- Produces, in `internal/profiler`:
  - `const PromptVersion = "1"`; `profileInstruction` copied byte for
    byte from today's `generate.go`; `const maxTokens = 4096`.
  - `type Neighbor struct { Rel, Summary string }`
  - `func Profile(ctx context.Context, p models.Provider, rel string, c reader.Content, neighbors []Neighbor) (text string, usage models.Response, err error)`
    — builds the request as today's
    `buildProfilePrompt`/`buildNeighborPrompt` do, with neighbors sorted
    by `Rel` and `rel` itself left out.
  - `func Recipe(model string) model.Recipe` →
    `"reader=" + reader.Version + ";prompt=" + PromptVersion + ";model=" + model`.

- [ ] **Step 1: Write the failing tests** with a fake `Provider` that
      records the request:
  - `TestProfileTextRequest`: text content → request `Text` contains
    `<document path="notes.md">` and the neighbor lines, not the file's
    own summary; `System` equals `profileInstruction`.
  - `TestProfilePDFRequest`: PDF content → `PDF` set, `Text` holds only
    the neighbor section.
  - `TestProfileEmptyReply`: provider returns `""` → error, and
    `Classify(err) == FileProblem`.
  - `TestClassify`: `&anthropic.Error{StatusCode: 400}` → File; 413 →
    File; 401, 429, 529 → Service; `errors.New("dial tcp")` → Service.
  - `TestRecipe`:
    `Recipe("claude-haiku-5-5") == "reader=1;prompt=1;model=claude-haiku-5-5"`.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement. Confirm `anthropic.Error`,
      `NewDocumentBlock`, and `Base64PDFSourceParam` against the SDK
      version you install.
- [ ] **Step 4:** Run `make test`,
      `make mutate PKG=./internal/profiler`,
      `make mutate PKG=./internal/models` (the Anthropic call itself has
      no unit test).
- [ ] **Step 5:** Commit.

### Task 10: Journal

**Files:**

- Create: `internal/journal/journal.go`,
  `internal/journal/journal_test.go`

**Interfaces:**

- Produces:
  `type Event struct { Time time.Time; Kind, Dir, Rel, Recipe, Error string; InputTokens, OutputTokens int64; Millis int64 }`
  with JSON tags in snake_case and `omitempty`;
  `func Append(path string, e Event) error` — opens with
  `O_APPEND|O_CREATE|O_WRONLY`, writes one JSON line. Kinds: `summary`,
  `failure`, `pause`, `resume`, `config_error`, `publish_error`.

- [ ] **Step 1: Write the failing test** `TestAppendLines`: append two
      events → file has two lines, each valid JSON that decodes back to
      the event.
- [ ] **Step 2:** Run; expect failure.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/journal`.
- [ ] **Step 5:** Commit.

### Task 11: Daemon step and `catalog daemon --once`

**Files:**

- Create: `internal/daemon/daemon.go`, `internal/daemon/daemon_test.go`,
  `cmd/catalog/main.go`
- Modify: `.golangci.yml` (depguard), `.goreleaser.yaml` if its `main`
  path no longer matches

**Interfaces:**

- Consumes: everything above.
- Produces:
  - `type Daemon struct { Global settings.Global; Store *store.Store; Provider models.Provider; JournalPath string; FS func(dir string) fs.FS }`
    plus last-good catalog settings and status fields it keeps
    internally.
  - `func (d *Daemon) Crawl(now time.Time) error` — for each watched
    folder: `FindCatalogs`, apply each catalog's `.catalog/config.toml`
    (on error keep that catalog's last good lists, journal
    `config_error`, record it for status), `Members`, `Scan`,
    `SetFiles`, then render and publish each catalog that changed.
  - `func (d *Daemon) Work(ctx context.Context, now time.Time, ignoreQuiet bool) (did bool, err error)`
    — build candidates from the Store, `Plan`, take the first; read,
    profile, store the summary, clear the failure, publish the catalog,
    journal `summary`. On a `FileProblem` record a failure with
    `RetryAt = now + Backoff(count)` and journal it. On a
    `ServiceProblem` return the error without charging the file.
    `ignoreQuiet` treats every `ChangedAt` as old enough.
  - `cmd/catalog`: `catalog daemon --once <folder>` loads `global.toml`,
    treats `<folder>` as the only watched folder, then runs `Crawl` and
    `Work(ignoreQuiet=true)` until `did == false`, and exits non-zero on
    a service problem.

Rendering a catalog uses, per member,
`Store.Summary(hash, current recipe)`, falling back to `AnySummary`,
falling back to a listing. A file's neighbors for `Profile` are the
other members of the same catalog that have a summary, found the same
way. `Daemon.FS` returns `os.DirFS(dir)` in production and an `os.DirFS`
over a temp dir in tests.

- [ ] **Step 1: Write the failing tests**, with a fake provider and
      catalogs built in `t.TempDir()`:
  - `TestOnceProducesCatalog`: two Markdown files under a folder with
    `.catalog/` → after crawl and work, `.catalog/catalog.md` holds both
    summaries.
  - `TestListingBeforeSummary`: after `Crawl` only → entries say
    `Not summarized yet.`
  - `TestQuietPeriod`: `Work(now, false)` within 3 minutes of the change
    does nothing; at 3 minutes it summarizes.
  - `TestNestedCatalog`: `a/.catalog/` and `a/b/.catalog/` → `a`'s
    catalog lists none of `b`'s files.
  - `TestFileFailureDoesNotBlock`: provider fails one file with a 400 →
    that file gets a failure with `RetryAt = now + 1m`, the other file
    is summarized.
  - `TestUnreadableFile` (Review Focus 3): one file at mode `0o000` →
    it's left out or failed, the other file is summarized, no panic.
  - `TestServiceFailureChargesNoFile`: provider returns 529 → `Work`
    returns an error, no failure is recorded, and the listings are still
    published.
  - `TestBadCatalogConfig` (Review Focus 5): a valid `config.toml`,
    crawl, then overwrite it with garbage and crawl → the catalog still
    uses the earlier globs, a `config_error` line is in the journal, and
    a second catalog is unaffected.
  - `TestUnwritableCatalog` (Review Focus 4): one `.catalog/` at mode
    `0o555` → a `publish_error` journal line, and the other catalog
    publishes.
  - `TestNothingElseWritten`: after a run, every path under the watched
    folder that wasn't there before is `.catalog/catalog.md`.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement. Add depguard rules to `.golangci.yml`
      (check its current config syntax) so that `internal/*` packages
      import only `internal/model`, except `internal/publish` →
      `internal/render` and `internal/render`/`internal/profiler` →
      `internal/reader`, `internal/profiler` → `internal/models`, and
      `internal/daemon` → all; `cmd/catalog` → `internal/daemon`,
      `internal/settings`, `internal/store`, `internal/models`,
      `internal/control`.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/daemon`.
- [ ] **Step 5:** Commit.

### Task 12: Manual run on a real repository

**Files:** none

- [ ] **Step 1:** Clone a repository that uses today's tool into a
      scratch folder. Delete `.catalog/catalog.db` and the root
      `.catalog.md`; keep `.catalog/config.toml`.
- [ ] **Step 2:** Write a `global.toml` with a real API key, run
      `go run ./cmd/catalog daemon --once <clone>`, and expect exit 0.
- [ ] **Step 3:** Read `.catalog/catalog.md` and compare it with the old
      `.catalog.md` from Git history: the same documents, every entry
      summarized, and each summary saying when to open the file. Check
      `journal.jsonl` has one `summary` line per file with token counts.
- [ ] **Step 4:** Report what you found to the maintainers. Fix bugs in
      a follow-up commit with a test.

### Task 13: The crawl loop

**Files:**

- Modify: `internal/daemon/daemon.go`, `internal/daemon/daemon_test.go`,
  `cmd/catalog/main.go`

**Interfaces:**

- Produces:
  `func (d *Daemon) Run(ctx context.Context, clock func() time.Time, tick <-chan time.Time) error`
  — on each tick: reread `global.toml` (keep the last good on a parse
  error and journal it), `Crawl`, then call `Work` until it does
  nothing, `ctx` ends, or a service problem occurs. After a service
  problem, skip `Work` until `pausedUntil = now + Backoff(consecutive)`,
  journaling `pause` and later `resume`; keep crawling and publishing
  listings meanwhile. `catalog daemon` runs `Run` with a one-minute
  `time.Ticker` and stops on SIGTERM or SIGINT.

- [ ] **Step 1: Write the failing tests**, driving `tick` by hand and a
      fake clock:
  - `TestRunPausesOnServiceProblem`: provider fails with 529 → after the
    tick, no further `Complete` calls until the clock passes 1 minute,
    then one call; journal has `pause` then `resume`.
  - `TestRunPicksUpNewCatalog`: create a `.catalog/` between ticks → the
    next tick publishes its catalog.
  - `TestRunRereadsGlobal`: change `enumerate` in `global.toml` between
    ticks → the next tick's catalog follows the new globs.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/daemon`.
- [ ] **Step 5:** Commit.

### Task 14: Status over the socket

**Files:**

- Create: `internal/control/control.go`,
  `internal/control/control_test.go`
- Modify: `internal/daemon/daemon.go`, `cmd/catalog/main.go`

**Interfaces:**

- Produces:
  - `type Snapshot struct { Started, LastCrawl time.Time; ConfigErrors []string; PausedUntil time.Time; PauseReason string; Catalogs []CatalogStatus; Failing []FailingFile }`,
    `type CatalogStatus struct { Dir string; Files, Summarized, Waiting int }`,
    `type FailingFile struct { Dir, Rel, Error string; RetryAt time.Time }`
    — JSON-encoded.
  - `func Serve(ctx context.Context, socketPath string, current func() Snapshot) error`
    — listens on a Unix socket (removing a stale socket file first) and
    writes the current snapshot as one JSON object to each connection.
  - `func Ask(socketPath string) (Snapshot, error)`.
  - `func Format(s Snapshot) string` — the text `catalog status` prints.
  - `Daemon` publishes a fresh `Snapshot` through an
    `atomic.Pointer[control.Snapshot]` after each crawl and each `Work`
    call.
  - `catalog status` prints `Format(Ask(...))`, or "The catalog daemon
    isn't running." when the socket is missing or refuses the
    connection.

- [ ] **Step 1: Write the failing tests:**
  - `TestServeAndAsk`: serve a fixed snapshot on a socket in a short
    temp dir (`os.MkdirTemp("", "cat")`; macOS limits socket paths to
    about 104 bytes) → `Ask` returns an equal snapshot.
  - `TestAskNoDaemon`: missing socket → an error the command maps to
    "isn't running".
  - `TestFormat`: a snapshot with one paused reason, one catalog, one
    failing file → output names the catalog folder, its counts, the
    failing file and its error, and the pause reason.
  - `TestDaemonSnapshot`: after a `Work` that records a failure, the
    daemon's snapshot lists that file.
- [ ] **Step 2:** Run; expect failures.
- [ ] **Step 3:** Implement with `net.Listen("unix", ...)` and
      `encoding/json`.
- [ ] **Step 4:** Run `make test` and
      `make mutate PKG=./internal/control`.
- [ ] **Step 5:** Commit.

### Task 15: launchd agent and README

**Files:**

- Create: `launchd/com.westarete.catalog.plist`
- Modify: `README.md`

- [ ] **Step 1:** Write the property list from `man launchd.plist`:
      `Label` `com.westarete.catalog`; `ProgramArguments` the installed
      binary's absolute path and `daemon`; `RunAtLoad` true; `KeepAlive`
      true; `StandardOutPath` and `StandardErrorPath` in
      `~/Library/Logs/` (launchd doesn't expand `~`, so the README tells
      people to replace it with their home path). Validate with
      `plutil -lint launchd/com.westarete.catalog.plist`; expect `OK`.
- [ ] **Step 2:** Add a README section on running the daemon: build and
      install the binary, write `global.toml` (an example with
      `[[watch]]`), copy the property list to `~/Library/LaunchAgents/`
      and load it with `launchctl` (check `man launchctl` for the
      current subcommand), and check `catalog status`. State that people
      should watch only folders that no cloud drive syncs and that macOS
      doesn't protect, such as `~/src`, not Desktop, Documents,
      Downloads, or iCloud Drive. Add the steps for moving a repository
      from today's tool, taken from the spec's "Moving from today's
      tool."
- [ ] **Step 3:** Load the agent with `~/src` (or another unprotected
      folder holding a `.catalog/`) in the watch list and wait two
      minutes. Expect `catalog status` to show the catalog with files
      counted and no read errors. If macOS refuses access, stop and
      report it to the maintainers; the spec has no fallback for that.
- [ ] **Step 4:** Run `make markdown` and `make test`.
- [ ] **Step 5:** Commit.
