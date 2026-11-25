# Status version and times Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `catalog status` shows the running daemon's version, warns on
that line when it differs from the command's, and prints every time with
its date, seconds, and UTC offset.

**Architecture:** The daemon puts its build version in the
`control.Snapshot` it serves on the control socket. `control.Format`
takes the command's own version as a second argument, compares the two,
and prints three labeled lines in place of today's first line. One time
layout serves every time in the output.

**Tech Stack:** Go standard library (`time`, `encoding/json`), Gremlins
for mutation testing.

**Spec:**
[2026-11-25-status-version-design.md](../specs/2026-11-25-status-version-design.md)

## Global Constraints

- Time layout, local time: `2006-01-02 15:04:05-07:00`. UTC prints
  `+00:00`, never `Z`.
- Version line when the versions match: `Daemon version: <version>`.
- Version line when they differ:
  ``Daemon version: <daemon> ⚠️ Run `catalog setup` to restart it with <command>``.
  The sign is U+26A0 U+FE0F (`"⚠️"` in Go), with one space on each side.
  The backticks are literal characters in the output.
- A snapshot with no version prints `unknown` in place of the daemon's
  version, and always gets the warning.
- Before the first crawl ends: `Last crawled: not yet`, and nothing
  follows. With no start time: `Started: not yet`. The version line
  always shows.
- Every other line of status keeps its wording; only the times in the
  pause line and the Failing lines change layout.
- Run `make markdown` after editing any `.md` file, and `make test`
  before every commit.

## Review Focus

- **Privacy and security:** the snapshot gains only the version string.
  Check that no other daemon state was added to it, and that the
  socket's `0600` mode and its tests are unchanged.
- **A daemon from before this change** sends JSON with no `Version` key.
  Status should print `Daemon version: unknown ⚠️ Run …`, not an empty
  version or an error. Task 1 tests `Format` with an empty `Version`,
  and `TestServeAndAsk` covers the JSON round trip.
- **A zone with a half-hour offset**, such as India's `+05:30`, should
  print `+05:30`, and UTC should print `+00:00`. Task 1 tests both.
- **A daemon newer than the command** gets the same warning as an older
  one, since `catalog setup` fixes either. Task 1 tests it.
- **Two development builds** both report `dev` and get no warning. Task
  1 tests that equal versions, `dev` included, print no sign.
- **Times with fractions of a second** print whole seconds, never `.5`.
  Task 1's local-time test uses a time with nanoseconds.

---

### Task 1: Format shows the version line and full times

**Files:**

- Modify: `internal/control/control.go` (the `Snapshot` struct)
- Modify: `internal/control/format.go`
- Modify: `cmd/catalog/main.go` (`runStatus`)
- Test: `internal/control/control_test.go`

**Interfaces:**

- Produces: `Snapshot.Version string`, the daemon's build version, empty
  from a daemon that predates it. Doc comment says so.
- Produces: `func Format(s Snapshot, commandVersion string) string`.
  `commandVersion` is the version of the program printing the status.

- [ ] **Step 1: Update the existing Format tests and add the new ones**

In `control_test.go`, set `Version: "0.7.3"` in `sample()`, so
`TestServeAndAsk` checks it survives the socket. Then change the Format
tests to these expectations (all under `useLocal(t, time.UTC)` unless
noted; `started` is the package's existing 2026-11-15 09:00 UTC):

- `TestFormat`: `Format(sample(), "0.7.3")` begins with these lines, and
  the rest of the expected output is today's with only the times
  changed:

  ```text
  Daemon version: 0.7.3
  Started: 2026-11-15 09:00:00+00:00
  Last crawled: 2026-11-15 09:41:00+00:00
  Config error: /state/global.toml: parse global.toml: bad
  Summarizing paused until 2026-11-15 09:45:00+00:00: summarize /notes/a.md: 529 Overloaded
  ```

  and the Failing line ends `(retry 2026-11-15 10:00:00+00:00)`.

- `TestFormatBeforeFirstCrawl`, with command version `0.7.3` and
  `Version: "0.7.3"`: the `started` case gives
  `"Daemon version: 0.7.3\nStarted: 2026-11-15 09:00:00+00:00\nLast crawled: not yet\n"`;
  the zero-start case gives
  `"Daemon version: 0.7.3\nStarted: not yet\nLast crawled: not yet\n"`.
- `TestFormatNoCatalogs`: the expected first three lines change to the
  labeled form; `No catalogs found.` stays.
- `TestFormatUsesLocalTime`: under `time.FixedZone("EST", -5*60*60)`, a
  start of `time.Date(2026, 11, 15, 14, 0, 0, 500_000_000, time.UTC)`
  prints the line `Started: 2026-11-15 09:00:00-05:00`.
- New `TestFormatHalfHourOffset`: under
  `time.FixedZone("IST", 5*60*60+30*60)`, a start of 09:00 UTC prints
  `Started: 2026-11-15 14:30:00+05:30`.
- New `TestFormatVersionMismatch`, table-driven, checking the first line
  only:

  | Snapshot `Version` | command version | first line                                                                  |
  | ------------------ | --------------- | --------------------------------------------------------------------------- |
  | `0.7.2`            | `0.7.3`         | ``Daemon version: 0.7.2 ⚠️ Run `catalog setup` to restart it with 0.7.3``   |
  | `0.7.4`            | `0.7.3`         | ``Daemon version: 0.7.4 ⚠️ Run `catalog setup` to restart it with 0.7.3``   |
  | empty              | `0.7.3`         | ``Daemon version: unknown ⚠️ Run `catalog setup` to restart it with 0.7.3`` |
  | `0.7.3`            | `0.7.3`         | `Daemon version: 0.7.3`                                                     |
  | `dev`              | `dev`           | `Daemon version: dev`                                                       |

  Write the sign in the expected strings as `⚠️`.

- `TestFormatFolderProblemAndStop` only needs the new second argument.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/control/` Expected: compile failure,
`too many arguments in call to Format` and `unknown field Version`.

- [ ] **Step 3: Add `Snapshot.Version` and change `Format`**

In `format.go`, set `timeLayout` to `"2006-01-02 15:04:05-07:00"` and
replace `startLine` with a function that writes the three lines. Give
the version line its own function,
`versionLine(daemon, command string) string`, holding the `unknown` and
warning rules. Keep `pauseLine`, `writeCatalogs`, and `writeFailing` as
they are; they call `when`, so their times change with the layout. In
`main.go`, `runStatus` calls `control.Format(s, version)`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/control/ ./cmd/catalog/` Expected: PASS.

- [ ] **Step 5: Mutation-test the package**

Run: `make mutate PKG=./internal/control` Expected: no mutant in
`format.go` LIVED. If one lives, add the test that kills it.

- [ ] **Step 6: Commit**

Run `make test`, then commit `internal/control/` and
`cmd/catalog/main.go` through `knowledge-work:git-commit`.

### Task 2: The daemon reports its version, and the docs describe it

**Files:**

- Modify: `internal/daemon/daemon.go` (the `Daemon` struct)
- Modify: `internal/daemon/snapshot.go` (`status`)
- Modify: `cmd/catalog/main.go` (`runLoop`)
- Test: `internal/daemon/snapshot_test.go`, `cmd/catalog/main_test.go`
- Modify: `docs/reference/commands.md`,
  `docs/explanation/architecture.md`, `docs/how-to/install-and-run.md`

**Interfaces:**

- Consumes: `control.Snapshot.Version`,
  `control.Format(s, commandVersion)` from Task 1.
- Produces: `Daemon.Version string`, exported beside the other fields
  `runLoop` sets. Doc comment: the program's build version, which status
  reports.

- [ ] **Step 1: Write the failing tests**

- New `TestSnapshotCarriesVersion` in `snapshot_test.go`: with
  `d.Version = "0.7.3"` and one step, `d.Snapshot().Version == "0.7.3"`.
- In `main_test.go`, `TestStatusOfRunningDaemon` sets `version` to
  `"1.2.3"` the way the version test at line 1107 does, and asserts that
  `stdout` contains the line `Daemon version: 1.2.3` and doesn't contain
  `⚠`.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/daemon/ -run TestSnapshotCarriesVersion` and
`go test ./cmd/catalog/ -run TestStatusOfRunningDaemon` Expected: the
first fails to compile on `d.Version`; the second fails because the line
reads `Daemon version: unknown ⚠️ …`.

- [ ] **Step 3: Implement**

Add `Daemon.Version`, copy it into the snapshot in `status()`, and set
`d.Version = version` in `runLoop` beside `d.GlobalPath`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/daemon/ ./cmd/catalog/` Expected: PASS.

- [ ] **Step 5: Update the docs**

- `commands.md`, Status output: replace the example's first line with
  the three labeled lines, with times in the new layout throughout the
  example. Rewrite item 1 of the numbered list to describe the three
  lines, the `not yet` cases, the `unknown` version, and the warning and
  what `catalog setup` does about it.
- `architecture.md`, the Control section: the snapshot holds the
  daemon's version as well.
- `install-and-run.md`, Update the daemon: after `brew upgrade catalog`,
  tell the reader to check that `Daemon version:` shows the new version
  with no warning.

Load `knowledge-work:markdown` first, follow `writing-style.md`, and run
`make markdown`.

- [ ] **Step 6: Commit**

Run `make test`, then commit through `knowledge-work:git-commit`.

### Task 3: Finish the branch

- [ ] **Step 1:** Run the whole-branch review and Claude Code's
      `/security-review`, and fix what they find.
- [ ] **Step 2:** File any undone steps or deferred findings as GitHub
      issues, then delete this plan in the same branch, and commit.
