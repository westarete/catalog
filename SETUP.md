# Developer setup

This is for anyone working on `catalog` itself. You need Go,
golangci-lint, and Node, which runs the pinned Markdown tools through
`npx`. With Homebrew:

```sh
brew install go golangci-lint node
```

Mutation testing uses Gremlins, which Go installs. Run
`make mutate PKG=./some/package` to check how well a package's tests
catch faults. Install it with:

```sh
go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
```

CI uses the Go and golangci-lint versions pinned in
[.github/workflows/check.yml](.github/workflows/check.yml).

Then clone the repository and check that it builds clean:

```sh
git clone https://github.com/westarete/catalog.git
cd catalog
make test
make markdown
```
