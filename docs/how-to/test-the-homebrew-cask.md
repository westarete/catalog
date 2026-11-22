# Test the Homebrew cask on a Mac

These steps build a notarized snapshot, install it as a cask from a tap
on your Mac, then upgrade and uninstall it, so you can watch the launch
agent before a release puts the cask in `westarete/tap`. Run them when
you change the cask in [`.goreleaser.yaml`](../../.goreleaser.yaml) or
what `catalog setup` does.

The snapshot has to be notarized. Homebrew quarantines every file a cask
downloads
([`cask/download.rb`](https://github.com/Homebrew/brew/blob/main/Library/Homebrew/cask/download.rb)
calls `Quarantine.cask!`), and Homebrew 5.0 deprecated `--no-quarantine`
because "Homebrew does not wish to easily provide circumvention to macOS
security features"
([Homebrew 5.0.0](https://brew.sh/2025/11/12/homebrew-5.0.0/)). As
[Apple code signing](apple-code-signing.md) explains, on macOS 15 and
later Gatekeeper stops a program that is signed but not notarized.
Notarizing the snapshot also tests the program the way a release ships
it.

You need:

- the five signing and notarization values in the table in
  [Apple code signing, Step 6](apple-code-signing.md#step-6--store-secrets-in-github)
- a Homebrew that has `brew trust`, as Homebrew 7 does
- a clone of this repository whose path has no spaces, because these
  steps put that path in a URL without encoding it

Run every command from the repository's root, in one Terminal window,
because later steps use shell variables that earlier ones set.

## Set aside a daemon that already runs

The cask's launch agent has the same label as any catalog daemon already
running in your account, so it would replace that daemon, and
`brew uninstall --zap` would delete its settings and summaries. If you
already run catalog in this account,
`launchctl print gui/$(id -u)/com.westarete.catalog` prints its agent.
Note the program it runs, so you can start it again at the end:

```sh
/usr/libexec/PlistBuddy -c "Print :ProgramArguments:0" ~/Library/LaunchAgents/com.westarete.catalog.plist
```

Then stop it, and move its state folder and log aside:

```sh
launchctl bootout gui/$(id -u)/com.westarete.catalog
mv ~/Library/Application\ Support/com.westarete.catalog ~/Library/Application\ Support/com.westarete.catalog.aside
mv ~/Library/Logs/catalog.log ~/Library/Logs/catalog.log.aside
```

`mv` keeps the folder's mode, 0700, so only you can read the summaries
while they're set aside.

## Build a notarized snapshot

Set the five values in this shell. Read each one with `read -rs`, so it
stays off the screen and out of your shell history, and paste it at the
empty prompt:

```sh
read -rs MACOS_CERTIFICATE && export MACOS_CERTIFICATE
```

Do the same for `MACOS_CERTIFICATE_PASSWORD`, `MACOS_NOTARY_ISSUER_ID`,
`MACOS_NOTARY_KEY`, and `MACOS_NOTARY_KEY_ID`.

The GoReleaser config turns notarization off for snapshots, so build
from a copy that turns it on:

```sh
config=$(mktemp)
sed 's/enabled: "{{ not .IsSnapshot }}"/enabled: true/' .goreleaser.yaml > "$config"
go run github.com/goreleaser/goreleaser/v2@v2.20.3 release --snapshot --clean --config "$config"
```

GoReleaser writes the archives and the cask,
`dist/homebrew/Casks/catalog.rb`, to `dist/`. Check that the Mac program
is signed with the Developer ID:

```sh
codesign -dv dist/catalog_darwin_*/catalog 2>&1 | grep Authority
```

The first line should read
`Authority=Developer ID Application: West Arete Computing, Inc. (HH6PJ9ECM)`.
If GoReleaser's log says it skipped signing and notarizing, one of the
five values is missing.

## Put the cask in a local tap

The generated cask points at the release on GitHub. Make a tap that has
no Git repository, and copy the cask into it with its URLs pointing at
the archives in `dist/`. It keeps the version GoReleaser gave the
snapshot, which is the latest tag followed by `-SNAPSHOT-` and the
commit, such as `0.5.3-SNAPSHOT-4ecd62e`. Homebrew's
[own cask tests](https://github.com/Homebrew/brew/blob/main/Library/Homebrew/test/cask/cask_spec.rb)
use `file://` URLs like these.

```sh
brew tap-new --no-git local/catalog-check
tap=$(brew --repository local/catalog-check)
mkdir -p "$tap/Casks"
sed "s|https://github.com/westarete/catalog/releases/download/v[^/]*/|file://$PWD/dist/|" \
    dist/homebrew/Casks/catalog.rb > "$tap/Casks/catalog.rb"
brew trust --tap local/catalog-check
```

Homebrew loads a cask from a tap outside Homebrew's own only if you
trust the tap, which `brew trust` records. `westarete/tap` also has a
cask named `catalog`, so these steps always name the test cask in full,
as `local/catalog-check/catalog`.

## Install it

```sh
brew install --cask local/catalog-check/catalog
```

Homebrew should print the cask's caveats and ask you nothing. Then check
that the agent runs the program in the cask's folder, and that only you
can read its property list:

```sh
launchctl print gui/$(id -u)/com.westarete.catalog | grep -E '^\s*(state|program) ='
stat -f %Lp ~/Library/LaunchAgents/com.westarete.catalog.plist
```

The first command should print `state = running` and a `program` under
`Caskroom/catalog/`, in a folder named for the snapshot's version. The
second should print `600`.

## Run setup

Your `PATH` may find another `catalog` before Homebrew's, so name the
cask's program by its path:

```sh
cmd=$(brew --prefix)/bin/catalog
"$cmd" setup
```

At the `API key:` prompt, type a few letters. None should appear. Press
Ctrl-C. Setup should print `catalog: setup interrupted`, and when you
type at the shell prompt again, the letters should appear.
`ls ~/Library/Application\ Support/com.westarete.catalog` should show no
`global.toml`.

Run `"$cmd" setup` again. Press Return to skip the key, enter
`~/catalog-check` as the only folder, and press Return on the empty
`Folder:` line. Setup should say that `~/catalog-check` doesn't exist
yet, that your settings are in `global.toml`, and that the daemon is
running. Then:

```sh
stat -f %Lp ~/Library/Application\ Support/com.westarete.catalog/global.toml
"$cmd" status
```

The first should print `600`. The status should begin with the launch
agent's path, `Settings:` with `(no API key)`, and the log's path, then
show the daemon running.

## Upgrade it

Homebrew upgrades a cask whenever the tap's version differs from the
installed one
([`cask/cask.rb`](https://github.com/Homebrew/brew/blob/main/Library/Homebrew/cask/cask.rb),
`outdated_version`), so add `-2` to the version:

```sh
sed -i '' 's/^  version "\(.*\)"/  version "\1-2"/' "$tap/Casks/catalog.rb"
brew upgrade --cask local/catalog-check/catalog
launchctl print gui/$(id -u)/com.westarete.catalog | grep -E '^\s*(state|program) ='
"$cmd" status
```

The upgrade should ask for no password. `launchctl print` should show
`state = running` and a `program` in a folder whose name ends in `-2`,
and the status should answer. If no agent is loaded after the upgrade,
the cask's design doesn't work; note what Homebrew printed before you go
on.

## Log out and back in

Log out of macOS and log back in. Run the `launchctl print` command
again. It should show the agent running the program in the `-2` folder.

## Uninstall it

```sh
brew uninstall --cask local/catalog-check/catalog
launchctl print gui/$(id -u)/com.westarete.catalog
ls ~/Library/LaunchAgents/com.westarete.catalog.plist ~/Library/Application\ Support/com.westarete.catalog
```

`launchctl print` should fail, because no agent is loaded. `ls` should
say the property list doesn't exist and should still list the state
folder, which holds `global.toml`, as the caveats say.

Install it again, then uninstall it with `--zap`:

```sh
brew install --cask local/catalog-check/catalog
brew uninstall --zap --cask local/catalog-check/catalog
ls ~/Library/Application\ Support/com.westarete.catalog ~/Library/Logs/catalog.log
```

`ls` should say that neither exists.

## Clean up

Remove the tap, the build, and the signing values:

```sh
brew untrust --tap local/catalog-check
brew untap local/catalog-check
rm -rf dist "$config"
unset MACOS_CERTIFICATE MACOS_CERTIFICATE_PASSWORD MACOS_NOTARY_ISSUER_ID MACOS_NOTARY_KEY MACOS_NOTARY_KEY_ID
```

If you set a daemon aside, move its state folder and log back, then run
`setup --agent-only` with the program you noted at the start, which
writes its launch agent and starts it. If that program was built before
`catalog setup` existed, rebuild it first.

```sh
mv ~/Library/Application\ Support/com.westarete.catalog.aside ~/Library/Application\ Support/com.westarete.catalog
mv ~/Library/Logs/catalog.log.aside ~/Library/Logs/catalog.log
<program you noted> setup --agent-only
```

Write down what you saw in the pull request or issue for the change,
especially any step whose result differed from this guide.
