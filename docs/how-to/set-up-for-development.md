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
