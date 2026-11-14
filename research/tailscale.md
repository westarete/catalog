# Tailscale's client

Tailscale is a VPN whose client runs a background daemon on several
operating systems. It indexes nothing. We looked at it for one question:
how its developers ship the same daemon on more than one operating
system, with a native interface on the Mac. We read its repository's
README and one page of its documentation, but not its source.

## One codebase, small wrappers

The README says the repository "includes the `tailscaled` daemon and the
`tailscale` CLI tool", and that "the macOS, iOS, and Windows clients use
the code in this repository but additionally include small GUI
wrappers." Those wrappers aren't open source
([tailscale/tailscale](https://github.com/tailscale/tailscale)). GitHub
counts about 96% of the repository's code as Go
([languages](https://api.github.com/repos/tailscale/tailscale/languages)).

## Three ways to run it on a Mac

Tailscale's documentation describes three variants for macOS: a
standalone app, a Mac App Store app, and the open-source `tailscale` and
`tailscaled` with no GUI
([Three ways to run Tailscale on macOS](https://tailscale.com/docs/concepts/macos-variants)).

The same page explains why there are two apps. Apple requires App Store
apps to run in the macOS App Sandbox, so that variant uses a Network
Extension. The standalone variant uses a system extension, isn't subject
to the App Store's limits, and doesn't require an Apple ID.

The page doesn't say what language the apps are written in, or how the
daemon inside them relates to the open-source `tailscaled`.

## What the catalog could take from it

These are our own conclusions, not claims from the sources above.

- A daemon written once in Go, with a small native wrapper on the Mac,
  is how Tailscale's developers build their Mac client.
  [ARCHITECTURE.md](../ARCHITECTURE.md) lists that as one of our
  language options.
- Tailscale maintains a separate Mac variant for the App Store because
  of the sandbox. If we go to the App Store, we should expect the
  sandbox to shape our daemon too.
