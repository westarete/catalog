# Apple's container tool

Apple's `container` is an open-source tool, written in Swift, that runs
Linux containers on a Mac
([README](https://github.com/apple/container)). It indexes nothing. We
looked at it because it's a recent Swift project from Apple with the
same outline as our daemon: a command-line tool that talks to a
background service for the logged-in user. We read its technical
overview, its package manifest, and its README, but not its source.

## Processes

The `container` command uses a client library to talk to
`container-apiserver`, which the overview calls a launch agent. The
agent starts when a person runs `container system start` and stops on
`container system stop`. It launches further helpers for images,
networking, and each running container. The overview lists XPC for
communication between processes and launchd for managing services
([technical overview](https://github.com/apple/container/blob/main/docs/technical-overview.md)).

The overview doesn't say how the agent is registered with launchd, or
what the helpers are named as services.

## How the package is divided

The whole project is one Swift package with many library targets and
eleven executables. Judging by their names, many targets come in pairs,
one for a service's client and one for its server, such as
`ContainerAPIClient` and `ContainerAPIService`; the manifest doesn't
describe their roles. The CLI executable depends on the argument parser,
a commands library, and `ContainerAPIClient`; it doesn't depend directly
on `ContainerAPIService`
([Package.swift](https://github.com/apple/container/blob/main/Package.swift)).

Its dependencies include Apple's `swift-argument-parser`, `swift-log`,
`swift-nio`, `swift-system`, `swift-collections`, and
`swift-configuration`, along with gRPC, an HTTP client, and TOML and
YAML parsers (same file).

## Installing it

People install `container` from a signed installer package, which asks
for an administrator password to put files under `/usr/local`. The
README doesn't mention Homebrew, and doesn't say why the developers
chose a package ([README](https://github.com/apple/container)).

## What the catalog could take from it

These are our own conclusions, not claims from the sources above.

- A CLI, a launch agent, and XPC between them is a shape Apple's own
  engineers chose for a Swift tool.
  [architecture.md](../design/architecture.md) proposes the same CLI and
  agent, and leaves open how their messages travel.
- Splitting client and server into separate targets lets the compiler
  keep the CLI from reaching into the daemon. We can do the same with
  one target per component.
- `container` starts its agent only when a person asks. Ours has to run
  without anyone asking, so we still need to learn how a per-user agent
  should be installed and kept running.
- Before we write the part that carries messages, we should read
  `container`'s source for how it registers the agent and checks who is
  calling.
