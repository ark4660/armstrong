# Armstrong

A terminal launcher and a performance-focused Minecraft client for Fabric.

- **Launcher** (Go, terminal UI): Microsoft and offline accounts, instances,
  a Modrinth mod browser with compatibility checks, and automatic updates.
- **Client** (Fabric mod, `client-mod/`): a module menu (Right Shift), HUD
  modules and a terminal-style look, shipped with a tuned performance stack
  (Sodium, Lithium, FerriteCore, ImmediatelyFast, Entity Culling, Dynamic FPS)
  that the launcher downloads from Modrinth.

## Install

Download the launcher for your system from the
[latest release](https://github.com/ark4660/armstrong/releases/latest) and run it:

| System | File |
|---|---|
| Windows | `armstrong-windows-amd64.exe` |
| macOS (Apple Silicon / Intel) | `armstrong-darwin-arm64` / `armstrong-darwin-amd64` |
| Linux | `armstrong-linux-amd64` / `armstrong-linux-arm64` |

The launcher keeps itself and the client up to date: when a new release is out
it shows `update available` (press `U` on Home), and the client updates on its
own before you play. From a terminal: `armstrong update`.

## Releasing

```sh
git tag v0.2.0
git push origin v0.2.0
```

The `release` workflow tests everything, builds the client mod and launchers
for every platform, and publishes a GitHub release with a signed `release.json`.
Launchers only accept releases signed with the release key (the public half is
in `internal/update/key.go`; the private half is the `ARMSTRONG_SIGNING_KEY`
repository secret and never goes in the repository).

## Building

- Launcher: Go 1.27+ — `go build ./cmd/armstrong`
- Client mod: `armstrong client build` (uses the Java 25 runtime the launcher installs)

Minecraft is a trademark of Mojang. Armstrong is not affiliated with Mojang or Microsoft.
