# Status shows the daemon's version and full times: design

`catalog status` will say which version of the daemon is running, warn
on that same line when it differs from the `catalog` command's version,
and show every time with its date, seconds, and UTC offset. Today it
shows no version, and its times, such as `Nov 23 15:02`, leave out the
year and the time zone, so a reader can't tell when anything happened.

## Why

Someone who updates catalog, or builds it from source, needs to know
whether the daemon that's running is the program they just installed.
The daemon keeps running the old program until something restarts it.
The command's own `catalog version` says only what's on disk.

The daemon's line today,
`Started Nov 23 15:02; last crawled Nov 25 08:22.`, packs two facts into
a sentence. A person can't skim it and a script can't pick a value out
of it.

## Goals

- Show the running daemon's version.
- When the daemon's version differs from the command's, say so on the
  version line and name the command that fixes it.
- Show every time in status in one unambiguous layout.
- Keep the output's shape the same from run to run: the same lines, in
  the same order, whether or not something is wrong.

## Non-goals

- Changing the Catalogs counts.
  [Issue #22](https://github.com/westarete/catalog/issues/22) covers
  them.
- Rewording the pause line or the Failing lines. Only their times
  change.
- Telling apart two development builds. Both report `dev`, so status
  sees them as the same version.

## Approaches

We considered two ways to show a mismatch:

1. **A warning on the daemon's version line, only when the versions
   differ.** The line keeps its place, and the fix sits beside the fact
   it fixes.
2. **Two lines, always: the command's version and the daemon's.** The
   shape never changes, but the reader has to compare the numbers and
   know the fix.

We chose the first. A warning on a line of its own would appear and
disappear, which is disorienting, and would sit apart from the version
it's about.

## What status shows

When the versions match:

```text
Daemon version: 0.7.3
Started: 2026-11-23 15:02:28-05:00
Last crawled: 2026-11-25 08:22:41-05:00
```

When they don't:

```text
Daemon version: 0.7.2 ⚠️ Run `catalog setup` to restart it with 0.7.3
Started: 2026-11-23 15:02:28-05:00
Last crawled: 2026-11-25 08:22:41-05:00
```

These three lines replace today's first line. Everything after them
stays as it is.

- **Version.** The daemon puts its build version, `main.version`, in the
  snapshot it serves on the control socket. `catalog status` compares it
  with its own `main.version`. A daemon from before this change sends no
  version, so the line reads `Daemon version: unknown`, followed by the
  same warning.
- **The fix.** `catalog setup` writes the launch agent's property list
  again and restarts the agent, which then runs the program that ran
  setup, as
  [set-up-for-development.md](../../how-to/set-up-for-development.md)
  describes. That works whichever version is older, so the warning
  doesn't need to say which one is.
- **The warning sign.** Status otherwise uses no emoji. A version
  mismatch is rare and needs action, so it gets one. Nothing follows it
  on the line, so how wide a terminal draws it doesn't matter.
- **Backticks.** The command in the warning goes in backticks, so the
  reader can see where it begins and ends.
- **Times.** Every time in status uses the Go layout
  `2006-01-02 15:04:05-07:00` in local time: the start, the last crawl,
  the time a pause ends, and each failing file's retry time. In UTC the
  offset prints as `+00:00`, not `Z`.
- **Before the first crawl ends.** The third line reads
  `Last crawled: not yet`, and nothing follows it, as today. If the
  daemon hasn't recorded its start time, the second line reads
  `Started: not yet`. The version line always shows.

## Sensitive data in this change

The snapshot gains one field, the daemon's version string, which is
public. It travels only over the control socket, which only the account
that runs the daemon can open (mode `0600` in a private state folder).
Status prints it to that person's terminal. Nothing new is stored,
logged, or sent to a service.

## Quality, size, and cost

Neither quality nor size changes, because nothing about catalogs
changes. Cost doesn't change: the daemon sends one more short string per
status request.

## Docs to change

- [commands.md](../../reference/commands.md): the status example and the
  description of each line.
- [architecture.md](../../explanation/architecture.md): the Control
  section's list of what the snapshot holds.
- [install-and-run.md](../../how-to/install-and-run.md): the Update the
  daemon section, which will tell the reader to check the version line
  after `brew upgrade`.

## Left to the builder

- How the command's version reaches `control.Format`, and how the daemon
  gets its version.
- The name of the snapshot field. Adding a field keeps the JSON readable
  by both an older and a newer `catalog status`.
