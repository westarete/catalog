# Set up for development

This is for anyone working on `catalog` itself. You need Go,
golangci-lint, and Node, which runs the pinned Markdown tools through
`npx`. With Homebrew:

```sh
brew install go golangci-lint node
```

Install Gremlins, the mutation testing tool, with:

```sh
go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
```

Then `make mutate PKG=./some/package` runs it on one package.

CI uses the Go and golangci-lint versions pinned in
[.github/workflows/check.yml](../../.github/workflows/check.yml).

Then clone the repository and check that it builds clean:

```sh
git clone https://github.com/westarete/catalog.git
cd catalog
make test
make markdown
```

## Install the daemon from source

Until the first Homebrew release, this is how you run the daemon. Build
it into the folder on your `PATH` where you keep programs you install
yourself, putting that folder in place of `<folder>`:

```sh
go build -o <folder>/catalog ./cmd/catalog
```

Check that `command -v catalog` prints the path you just built to, then
run `catalog setup`, which asks for your API key and the folders to
watch and loads the launch agent.
[Install and run the daemon](install-and-run.md#give-the-daemon-its-key-and-folders)
shows the session.

After you pull a newer version, build over the program in place and run
`catalog setup` again. If `global.toml` exists, setup asks nothing; it
writes the property list again and restarts the agent, so the daemon
runs your new build.

A Mac has one launch agent with the label `com.westarete.catalog`, so a
source build and a Homebrew install share it. The agent runs whichever
program ran `catalog setup` last, or the cask's installer, which runs
`catalog setup --agent-only`. If setup replaces a property list that
named a different program, it says which program the agent ran before
and which it runs now. If both are installed, `command -v catalog` shows
which one a bare `catalog` runs.
