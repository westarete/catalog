# Installing the daemon with Homebrew: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After `brew install` and `catalog setup`, the daemon runs
under launchd with the person's key and folders; each `brew upgrade`
moves it to the new build, `brew uninstall` removes its agent, and
`catalog status` says where its files are.

**Architecture:** A new package, `internal/install`, writes the launch
agent's property list and loads it through `launchctl`, asks for and
writes `global.toml`, and describes the install for status.
`cmd/catalog` gains `catalog setup [--agent-only]` and prints the
install lines at the top of `catalog status`. The GoReleaser cask runs
`catalog setup --agent-only` as an `installer script:` and removes the
agent with `uninstall launchctl:`.

**Tech Stack:** Go 1.26, `golang.org/x/term` v0.48.0,
`github.com/BurntSushi/toml` (already a dependency), `/bin/launchctl`,
GoReleaser v2.20.3, Homebrew.

**Spec:**
[docs/superpowers/specs/2026-11-19-homebrew-install-design.md](../specs/2026-11-19-homebrew-install-design.md)

## Global Constraints

- Label: `com.westarete.catalog`, in the property list and in the cask's
  `uninstall launchctl:`.
- Property list: `~/Library/LaunchAgents/com.westarete.catalog.plist`,
  mode 0600, keys `Label`, `ProgramArguments` (`[<program>, "daemon"]`),
  `RunAtLoad` true, `KeepAlive` true, `Umask` 63, `StandardOutPath` and
  `StandardErrorPath` both `~/Library/Logs/catalog.log`, all paths
  absolute.
- State folder 0700; `global.toml` 0600, created only when missing and
  never changed by setup afterwards. Set modes explicitly; never rely on
  the umask.
- The API key is never an argument, never printed, never logged, and
  never written anywhere but `global.toml`.
- Exact copy, from the spec. The two prompts end with one space and no
  newline:

  ```text
  Paste your Anthropic API key and press Return, or press Return to skip this step.
  API key:
  The daemon will watch the folders you list. Don't list a folder that a cloud drive syncs, or Desktop, Documents, or Downloads.
  Enter one folder per line, then press Return on an empty line to finish. To watch only ~/src, press Return now.
  Folder:
  Your settings are in <path>, and only you can read that file.
  The catalog daemon is running. To see what it's doing, run `catalog status`.
  ```

  - Status: `Launch agent: <path>` or
    `Launch agent: not installed; run catalog setup`;
    `Settings: <path> (API key set)`, `(no API key)`,
    `(not found; run catalog setup)`, or `(can't be read: <error>)`;
    `Log: <path>`; the home folder shown as `~`.

- Copy the spec leaves to us (our call, keep it exactly):
  `Your settings are already in <path>. To change them, edit that file.`,
  `<folder> doesn't exist yet. The daemon will start watching it once you create it.`,
  `The daemon was running <old>, and now it runs <new>.`, and
  `Setup can start the daemon only on a Mac.`
- No code, config, or doc mentions the earlier command-line tool.
- Run `make test` and `make markdown` before each commit. Gremlins
  (`make mutate PKG=...`) leaves no live mutant in new code.
- Never run the real `/bin/launchctl` from a test.

## Review Focus

- **Paths with spaces or XML characters.** The state folder has a space,
  and a home folder or program path could hold `&` or `<`. The property
  list must still be valid XML that names the right paths (Task 1 tests
  a path with `&` and a space).
- **An agent that's already loaded.** Rerunning setup, an upgrade, and a
  source build replacing Homebrew's all find the agent loaded. Setup
  must unload it before loading it, and must work when a property list
  exists but the agent isn't loaded (Task 1).
- **A `global.toml` that exists but is broken or unreadable.** Setup
  must leave its bytes untouched, and status must report the problem
  without quoting any value (Tasks 2 and 3).
- **Input that ends early or has stray spaces.** Ctrl-D at a prompt, a
  closed standard input, or spaces around the key and folders. Treat end
  of input as an empty answer and trim answers, and `--agent-only` must
  never read standard input (Tasks 2 and 4).
- **A permissive umask.** With umask 000, the state folder,
  `global.toml`, and the property list must still come out 0700, 0600,
  and 0600 (Tasks 1 and 2).
- **Privacy and security, for every task reviewer:** check that the API
  key reaches only `global.toml` (not argv, output, logs, the property
  list, or test logs), that every file and folder this change creates
  has the mode above whatever the umask, and that nothing new is sent
  over a network.

---

### Task 1: The launch agent

**Files:**

- Create: `internal/install/agent.go`,
  `internal/install/agent.plist.tmpl` (embedded with `//go:embed`)
- Test: `internal/install/agent_test.go`

**Interfaces:**

- Produces:
  - `const Label = "com.westarete.catalog"`
  - `func PlistPath(home string) string` returns
    `<home>/Library/LaunchAgents/com.westarete.catalog.plist`
  - `func LogPath(home string) string` returns
    `<home>/Library/Logs/catalog.log`
  - `type Launchctl func(args ...string) error`, which runs
    `/bin/launchctl` with `args` and returns an error for a nonzero
    exit, including the command's combined output in the message
  - `type Agent struct { Home string; UID int; Launchctl Launchctl }`
  - `func (a Agent) Install(program string) (previous string, err error)`:
    writes the property list naming `program`, unloads the agent if
    `launchctl print gui/<uid>/com.westarete.catalog` succeeds, then
    runs `launchctl bootstrap gui/<uid> <plist path>`; `previous` is the
    program the old property list named, or `""` if there was none or it
    was the same
  - `func RunLaunchctl(args ...string) error`, the real `Launchctl`

- [ ] **Step 1: Write the failing tests**

```go
func TestInstallWritesPlist(t *testing.T)            // parse the file with encoding/xml: Label, ProgramArguments == [program, "daemon"], RunAtLoad and KeepAlive true, Umask 63, both log paths == LogPath(home)
func TestInstallEscapesPaths(t *testing.T)            // home "a&b c", program ".../a&b c/catalog": file parses as XML and names the paths exactly
func TestInstallModesIgnoreUmask(t *testing.T)        // syscall.Umask(0) for the test: plist 0600, and ~/Library/Logs is created
func TestInstallLoadsWhenNotLoaded(t *testing.T)      // fake: print fails → calls == [print gui/501/com.westarete.catalog], [bootstrap gui/501 <plist>]
func TestInstallReloadsWhenLoaded(t *testing.T)       // fake: print succeeds → print, bootout gui/501/com.westarete.catalog, bootstrap gui/501 <plist>
func TestInstallReportsPreviousProgram(t *testing.T)  // existing plist names /old/catalog → previous == "/old/catalog"; same program → ""; no plist → ""
func TestInstallFailsWhenBootstrapFails(t *testing.T) // fake bootstrap error → Install returns it
```

The fake `Launchctl` records each call's arguments and fails `print` or
`bootstrap` on request.

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/install/` Expected: FAIL to compile; `Agent` is
undefined.

- [ ] **Step 3: Implement `agent.go` and the template**

Render the template with `text/template`, escaping each value with
`xml.EscapeText`. Write the property list to a temporary file in the
same folder, set it to 0600, and rename it into place. Create
`~/Library/LaunchAgents` (0755, as macOS makes it) and `~/Library/Logs`
if missing. Read the old property list's first `ProgramArguments` string
with `encoding/xml` before overwriting it. Don't call
`launchctl enable`: `launchctl(1)` says it targets the system, user, and
user-login domains only.

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/install/` Expected: PASS

- [ ] **Step 5: Mutation-test, then commit**

Run: `make mutate PKG=./internal/install` Expected: no `LIVED` mutant in
`agent.go`.

```bash
git add internal/install
git commit -m "Write and load the daemon's launch agent"
```

### Task 2: Asking for the settings

**Files:**

- Create: `internal/install/settings.go`
- Test: `internal/install/settings_test.go`

**Interfaces:**

- Produces:
  - `func ReadFolders(in *bufio.Reader, out io.Writer) []string`: prints
    the `Folder:` prompt, with its trailing space, before each line it
    reads, trims each line, stops at the first empty line or the end of
    input, and returns `["~/src"]` if no folder was given
  - `func WriteGlobal(path, apiKey string, folders []string) error`:
    creates `path` with `O_EXCL` and mode 0600, sets 0600 explicitly,
    and writes `api_key` (left out when empty) and one `[[watch]]` table
    per folder, encoded with `toml.NewEncoder`; fails with an error
    wrapping `fs.ErrExist` if the file exists
  - `type Prompt struct { In *bufio.Reader; Out io.Writer; ReadKey func() (string, error); Home string }`
  - `func AskSettings(p Prompt, globalPath string) error`: if
    `globalPath` exists, prints
    `Your settings are already in <path>. To change them, edit that file.`
    and returns; otherwise prints the key prompt, calls `ReadKey`,
    prints the two folder lines, calls `ReadFolders`, prints
    `<folder> doesn't exist yet. The daemon will start watching it once you create it.`
    for each folder whose expanded path is missing, writes
    `global.toml`, and prints
    `Your settings are in <path>, and only you can read that file.`,
    followed by `To add your API key later, set api_key in that file.`
    when the key was empty; paths printed with the home folder as `~`
  - `func Tilde(home, path string) string`: replaces a leading `home`
    with `~`

- [ ] **Step 1: Write the failing tests**

```go
func TestReadFolders(t *testing.T)                   // "\n" → [~/src]; "  ~/src  \n~/notes/Project Plans\n\n" → [~/src, ~/notes/Project Plans] and "Folder: " printed 3 times; "~/src" then EOF → [~/src]
func TestWriteGlobalRoundTrips(t *testing.T)         // settings.ParseGlobal reads back APIKey "sk-test" and Watch of both folders (expanded)
func TestWriteGlobalLeavesOutEmptyKey(t *testing.T)  // file has no "api_key"
func TestWriteGlobalQuotesOddCharacters(t *testing.T) // key with `"` and `\`, folder with a space: round-trips exactly
func TestWriteGlobalModeIgnoresUmask(t *testing.T)   // syscall.Umask(0): 0600
func TestWriteGlobalRefusesExisting(t *testing.T)    // errors.Is(err, fs.ErrExist); bytes unchanged
func TestAskSettingsWritesNewFile(t *testing.T)      // ReadKey returns " sk-test ", In "~/src\n\n": output is the spec's copy in order, file holds key "sk-test"
func TestAskSettingsEndOfInput(t *testing.T)         // ReadKey returns "", io.EOF; In empty: file has no api_key and watches ~/src; output includes "To add your API key later, set api_key in that file."
func TestAskSettingsWarnsMissingFolder(t *testing.T) // "~/nope": prints "~/nope doesn't exist yet. The daemon will start watching it once you create it." and still writes it
func TestAskSettingsKeepsExistingFile(t *testing.T)  // existing malformed file: prints "Your settings are already in ...", never calls ReadKey, bytes unchanged
func TestAskSettingsNeverPrintsKey(t *testing.T)     // output does not contain "sk-test"
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/install/` Expected: FAIL to compile;
`AskSettings` is undefined.

- [ ] **Step 3: Implement `settings.go`**

Trim the key. Treat `io.EOF` from `ReadKey` or `In` as an empty answer.
Create the file's folder with mode 0700 if it's missing, then set 0700
explicitly, with the same `//nolint:gosec` comment `cmd/catalog/main.go`
uses for the state folder.

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/install/` Expected: PASS

- [ ] **Step 5: Mutation-test, then commit**

Run: `make mutate PKG=./internal/install` Expected: no `LIVED` mutant in
`settings.go`.

```bash
git add internal/install
git commit -m "Ask for the API key and folders and write global.toml"
```

### Task 3: Describing the install

**Files:**

- Create: `internal/install/describe.go`
- Test: `internal/install/describe_test.go`

**Interfaces:**

- Consumes: `PlistPath`, `LogPath`, `Tilde` (Tasks 1 and 2);
  `settings.ParseGlobal`, `settings.GlobalFile`
- Produces: `func Describe(home, stateDir string, onMac bool) string`,
  the install lines, each ending in `\n`: `Launch agent:` (left out when
  `onMac` is false), `Settings:`, `Log:`

- [ ] **Step 1: Write the failing tests**

```go
func TestDescribeInstalled(t *testing.T)       // plist and global.toml with a key exist: exactly the spec's three lines, paths under ~
func TestDescribeNothingInstalled(t *testing.T) // "Launch agent: not installed; run catalog setup", "Settings: ~/... (not found; run catalog setup)"
func TestDescribeNoAPIKey(t *testing.T)        // global.toml with only [[watch]]: "(no API key)"
func TestDescribeBrokenSettings(t *testing.T)  // api_key = "sk-secret" then garbage: "(can't be read: ...)" and the line does not contain "sk-secret"
func TestDescribeNotOnMac(t *testing.T)        // onMac false: no "Launch agent" line
```

- [ ] **Step 2: Run them and see them fail**

Run: `go test ./internal/install/` Expected: FAIL to compile; `Describe`
is undefined.

- [ ] **Step 3: Implement `describe.go`**

Read `global.toml` with `os.ReadFile` and parse it with
`settings.ParseGlobal`, whose errors already quote no value. Report a
read or parse error as `(can't be read: <error>)`.

- [ ] **Step 4: Run the tests and see them pass**

Run: `go test ./internal/install/` Expected: PASS

- [ ] **Step 5: Mutation-test, then commit**

Run: `make mutate PKG=./internal/install` Expected: no `LIVED` mutant in
`describe.go`.

```bash
git add internal/install
git commit -m "Describe the daemon's install for status"
```

### Task 4: `catalog setup` and the new status lines

**Files:**

- Modify: `cmd/catalog/main.go`, `go.mod`, `go.sum`
- Test: `cmd/catalog/main_test.go`

**Interfaces:**

- Consumes: `install.Agent`, `install.RunLaunchctl`,
  `install.AskSettings`, `install.Prompt`, `install.Describe`,
  `install.Tilde`
- Produces:
  - `run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int`
  - package variables tests replace:
    `var launchctl install.Launchctl = install.RunLaunchctl` and
    `var onMac = runtime.GOOS == "darwin"`
  - usage gains `catalog setup [--agent-only]`

- [ ] **Step 1: Add the dependency and read its docs**

Run: `go get golang.org/x/term@v0.48.0`, then
`go doc golang.org/x/term.ReadPassword` and
`go doc golang.org/x/term.IsTerminal`. Expected:
`func ReadPassword(fd int) ([]byte, error)` and
`func IsTerminal(fd int) bool`.

- [ ] **Step 2: Write the failing tests**

```go
func TestSetupWritesSettingsAndLoadsAgent(t *testing.T) // stdin "sk-test\n~/src\n\n", fake launchctl: global.toml 0600 with the key, plist 0600 naming the test binary, stdout ends with "The catalog daemon is running." line, code 0
func TestSetupAgentOnlyReadsNothing(t *testing.T)       // stdin that fails on Read: code 0, no global.toml created, plist written
func TestSetupWithExistingSettings(t *testing.T)        // existing global.toml unchanged; "Your settings are already in" printed
func TestSetupSaysWhenProgramChanges(t *testing.T)      // plist naming /old/catalog: prints "The daemon was running /old/catalog, and now it runs <program>."
func TestSetupNotOnMac(t *testing.T)                    // onMac false: global.toml written, "Setup can start the daemon only on a Mac." printed, launchctl never called, code 0
func TestSetupLaunchctlFails(t *testing.T)              // fake bootstrap error: code 1, stderr "catalog: ..."
func TestSetupStateFolderPrivate(t *testing.T)          // syscall.Umask(0): state folder 0700
func TestStatusWhenDaemonIsNotRunning(t *testing.T)     // update: stdout is the install lines, a blank line, then "The catalog daemon isn't running.\n"; code 1
func TestStatusOfRunningDaemon(t *testing.T)            // update: the install lines and a blank line come before the status
func TestUsageErrors(t *testing.T)                      // add: "setup extra", "setup --bogus"
```

Update every existing `run(...)` call in the tests for the new `stdin`
parameter.

- [ ] **Step 3: Run them and see them fail**

Run: `go test ./cmd/catalog/` Expected: FAIL; `setup` is a usage error.

- [ ] **Step 4: Implement `runSetup` in `cmd/catalog/main.go`**

Order: create the state folder with `privateStateDir`; unless
`--agent-only`, call `install.AskSettings`; then on a Mac install the
agent with
`install.Agent{Home, os.Getuid(), launchctl}.Install(program)` and print
the program change, if any, and the line that begins
`The catalog daemon is running.`; off a Mac print
`Setup can start the daemon only on a Mac.` instead. `program` is
`os.Executable()` passed through `filepath.EvalSymlinks`. `ReadKey` uses
`term.ReadPassword` and then prints a newline when `stdin` is an
`*os.File` for which `term.IsTerminal` is true; otherwise it reads a
line from the same `bufio.Reader` as the folders. `main` passes
`os.Stdin`. In `runStatus`, print
`install.Describe(home, stateDir, onMac)` and a blank line before what
it prints today, with the exit codes unchanged.

- [ ] **Step 5: Run the tests and see them pass**

Run: `make test` Expected: PASS, lint clean.

- [ ] **Step 6: Mutation-test, then commit**

Run: `make mutate PKG=./cmd/catalog` Expected: no `LIVED` mutant in the
new code.

```bash
git add cmd/catalog go.mod go.sum
git commit -m "Add catalog setup and show where the daemon's files are"
```

### Task 5: The cask

**Files:**

- Modify: `.goreleaser.yaml` (`homebrew_casks`)

**Interfaces:**

- Consumes: the `catalog setup --agent-only` command (Task 4) and
  `Label` (Task 1)

- [ ] **Step 1: Read GoReleaser's docs for the fields used**

Read
[homebrew_casks](https://goreleaser.com/customization/publish/homebrew_casks/)
for `description`, `caveats`, `uninstall`, `zap`, and `custom_block`,
and the [Cask Cookbook](https://docs.brew.sh/Cask-Cookbook) for
`installer script:`, `uninstall launchctl:`, `zap delete:`, and
`on_macos`.

- [ ] **Step 2: Change the cask's config**

- `description`: a line under 80 characters with no final period that
  says the program keeps a `catalog.md` current in each cataloged
  folder.
- `custom_block`: an `on_macos` block holding
  `installer script: { executable: "catalog", args: ["setup", "--agent-only"] }`.
- `uninstall`: `launchctl: ["com.westarete.catalog"]`.
- `zap`: `delete:` the state folder and `~/Library/Logs/catalog.log`,
  replacing today's `zap`.
- `caveats`: run `catalog setup` to add an API key and the folders to
  watch, then `catalog status` to see how the daemon is doing; a plain
  `brew uninstall` keeps the settings and summaries, and
  `brew uninstall --zap` deletes them.

- [ ] **Step 3: Check the config and the generated cask**

Run: `go run github.com/goreleaser/goreleaser/v2@v2.20.3 check`
Expected: no errors.

Run:
`go run github.com/goreleaser/goreleaser/v2@v2.20.3 release --snapshot --clean --skip=notarize`,
then `brew style --cask dist/homebrew/Casks/catalog.rb` Expected: the
cask holds the `installer`, `uninstall`, `zap`, and `caveats` above. If
`brew style` flags only stanza order caused by `custom_block` placing
lines first, record it in the task report; a third-party tap doesn't
have to pass `brew style`.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yaml
git commit -m "Load and remove the launch agent from the cask"
```

### Task 6: Docs

**Files:**

- Modify: `docs/how-to/install-and-run.md`,
  `docs/how-to/set-up-for-development.md`, `docs/reference/commands.md`,
  `docs/reference/files.md`, `docs/explanation/architecture.md`,
  `README.md`
- Delete: `launchd/com.westarete.catalog.plist`

- [ ] **Step 1: Load the markdown skill**

Load `knowledge-work:markdown`, and follow `writing-style.md`.

- [ ] **Step 2: Rewrite the docs as the spec's "Docs to change" lists**

- Install guide: `brew tap westarete/tap`,
  `brew trust --cask westarete/tap/catalog`, `brew install catalog`,
  `catalog setup`, putting a catalog in place, and uninstalling, with
  what a plain uninstall and `--zap` each leave or delete.
- Developer guide: build with
  `go build -o <folder>/catalog ./cmd/catalog` and run `catalog setup`,
  and say that a source build and a Homebrew install share one agent.
- Commands: a `catalog setup` section with its prompts, `--agent-only`,
  output, and exit codes; the status output section gains the install
  lines.
- Files: the property list row (mode 0600, created by `catalog setup`),
  and `global.toml` created by `catalog setup` as 0600 or by a person.
- Architecture, "Processes and macOS": the property list comes from
  `catalog setup` and the cask.
- README: until the release, the daemon is built but not released, and
  the developer guide says how to try it.

- [ ] **Step 3: Check them**

Run: `make markdown` and
`grep -rn "launchd/com.westarete" docs README.md` Expected:
`Markdown OK`, and no matches.

- [ ] **Step 4: Commit**

```bash
git add docs README.md launchd
git commit -m "Document installing with Homebrew and catalog setup"
```

### Task 7: Check it on a Mac, and review

The developer runs Step 1, because it needs a published release.

- [ ] **Step 1: Check the agent after the v0.6.0 release and the next
      one**

Only the developer installs from `westarete/tap`, so check the real
releases instead of a test cask. After the v0.6.0 release, run
`brew update`, install the daemon as the install guide says
(`brew tap westarete/tap`, `brew trust --cask westarete/tap/catalog`,
`brew install catalog`), then:

```sh
launchctl print gui/$(id -u)/com.westarete.catalog | grep -E '^\s*(state|program) ='
```

Expected: `state = running` and a `program` under
`Caskroom/catalog/0.6.0/`. After the next release, run `brew update`,
`brew upgrade catalog`, and the same command. Expected: a `program`
under that release's folder. If the agent isn't running after either,
fix the cask and release again.

- [ ] **Step 2: Review the branch**

Run `/security-review` and the whole-branch review, and fix what they
find.

- [ ] **Step 3: Finish the plan**

File the plan's undone steps and deferred findings as issues, delete
this plan, and commit.
