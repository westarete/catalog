# Notes on the command library

We'll build catalog's commands with
[Cobra](https://github.com/spf13/cobra). It's the only maintained Go
library we found that generates shell completion and documents
completing folder names alone, which `catalog completion <shell>` and
`catalog add` need. These notes go with the
[brainstorm on the command redesign](brainstorm-on-command-redesign.md),
so the spec can start from both.

## What Cobra gives us

- Nested commands, so `catalog service key set` is the normal case
  rather than something we build.
- A `completion` command for bash, zsh, fish, and PowerShell, which
  Cobra adds by default
  ([completion docs](https://github.com/spf13/cobra/blob/main/site/content/completions/_index.md)).
- Folder completion for `catalog add` and `catalog remove`: we write a
  `ValidArgsFunction` that returns `ShellCompDirectiveFilterDirs`.
- `catalog help` and `-h` or `--help` on every command. Cobra adds the
  `help` command to any command that has subcommands. We want it:
  `catalog help service` should work.
- An `unknown command` error with suggestions when someone mistypes a
  command.
- In-process tests through `SetArgs`, `SetIn`, `SetOut`, and `SetErr`,
  the same shape as today's `run(ctx, args, stdin, stdout, stderr)` in
  [main.go](../../cmd/catalog/main.go).

## Choices for the spec

- Version: Cobra adds `--version` and `-v` if we set the root command's
  `Version` field. Today `catalog` accepts `version`, `--version`, and
  `-v`. The brainstorm lists only `catalog version`, so the spec should
  say whether the flags stay.
- Usage after errors: Cobra prints the error, then the full usage text,
  whenever a command returns an error. `SilenceUsage` turns the usage
  text off.
- Error prefix: Cobra starts errors with `Error:`; today `catalog`
  starts them with `catalog:`. `SetErrPrefix` changes it.
- Streams: Cobra writes help and the version to stdout, and errors and
  usage to stderr.

## How to write the commands

Build each command in a function that returns a `*cobra.Command`, and
pass in what it needs. Cobra's
[user guide](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md)
declares commands as package-level variables and wires them together in
`func init()`; don't copy that. Peter Bourgon argues against both in
[A theory of modern Go](https://peter.bourgon.org/blog/2017/06/09/theory-of-modern-go.html),
and our [CLAUDE.md](../../CLAUDE.md) asks for side effects to be
explicit.

Don't call `cobra.CheckErr`, because it calls `os.Exit`. `main` should
keep choosing the exit code, as it does now. Cobra's only other
`os.Exit` is in a check that runs on Windows.

Import `github.com/spf13/cobra` only. The `cobra/doc` package generates
man pages and brings in two more modules.

## Privacy and security

- Cobra makes no network requests.
- It adds two modules we have to keep up to date: `pflag`, which parses
  flags, and `mousetrap`, which matters only on Windows.
- When someone presses Tab, their shell runs `catalog __complete` in the
  background with their permissions. Completion functions must print
  only folder and catalog names, never a key or a provider address.

The library doesn't change how often an agent finds the right files, how
many words a catalog adds, or how many model calls catalog makes.

## What we compared

|                        | Cobra                  | urfave/cli v3                                            | kong                                          | ff v4                   | `flag` and our own dispatch |
| ---------------------- | ---------------------- | -------------------------------------------------------- | --------------------------------------------- | ----------------------- | --------------------------- |
| Latest version         | v1.10.2                | v3.14.0, Oct 2026                                        | v1.17.0, Oct 2026                             | v4.0.0-beta.1, Aug 2025 | Go itself                   |
| Shell completion       | Built in, four shells  | Built in, four shells                                    | Only through kongplete, last changed Nov 2023 | None                    | We'd write it               |
| Folder-only completion | Yes                    | Not documented                                           | No                                            | No                      | We'd write it               |
| Commands it adds       | `help`, `completion`   | `help` (can be hidden), `completion` (hidden by default) | None                                          | None                    | None                        |
| Runtime dependencies   | pflag, mousetrap       | None                                                     | None                                          | go-toml, yaml           | None                        |
| Mistyped command       | Error with suggestions | Suggestions if `Suggest` is on                           | Error with suggestions                        | Runs the parent command | We'd write it               |

- [urfave/cli](https://github.com/urfave/cli) v3 is the closest
  alternative. Its documentation doesn't show folder-only completion. If
  an action returns its `cli.Exit` error, the library calls `os.Exit`
  itself, and a mistyped subcommand runs the parent command's action
  instead of failing.
- [kong](https://github.com/alecthomas/kong) has no completion of its
  own. [kongplete](https://github.com/WillAbides/kongplete) adds it, but
  its last change was in November 2023 and its `go.mod` names kong
  v0.8.1.
- [ff](https://github.com/peterbourgon/ff) v4 is still a pre-release, as
  its README says, and has no completion. Tailscale uses ff v3 and wrote
  its own completion by copying Cobra's shell scripts
  ([tempfork/spf13/cobra](https://github.com/tailscale/tailscale/tree/main/tempfork/spf13/cobra)).
- The standard library's `flag` with our own dispatch is what catalog
  does today, and what the `go` command and gopls do. We'd write
  completion for four shells ourselves.

## What experienced Go authors say

None of the experienced Go authors we read argues for Cobra's design,
though some praise its features.

- Peter Bourgon recommends package `flag` in his 2016 essay
  [Go best practices, six years in](https://peter.bourgon.org/go-best-practices-2016/),
  and wrote ff as a lighter alternative to Cobra and urfave/cli.
- Bjørn Erik Pedersen, who maintains Hugo, calls Cobra's features "hard
  to resist for bigger applications" and wrote
  [simplecobra](https://github.com/bep/simplecobra) because Cobra is
  complex to use beyond small programs.
- The `go` command's
  [Command type](https://github.com/golang/go/blob/master/src/cmd/go/internal/base/base.go)
  carries the comment "subcommands are in general best avoided," which
  bears on `catalog service key set`, a command three levels deep.
- Of the well-made Go tools with subcommands we checked, most list Cobra
  in their `go.mod`: [GitHub CLI](https://github.com/cli/cli),
  [kubectl](https://github.com/kubernetes/kubectl),
  [restic](https://github.com/restic/restic),
  [golangci-lint](https://github.com/golangci/golangci-lint),
  [Hugo](https://github.com/gohugoio/hugo) (through simplecobra), and
  [chezmoi](https://github.com/twpayne/chezmoi).

We haven't tried any of these libraries in catalog yet.
