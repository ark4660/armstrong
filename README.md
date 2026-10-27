# Armstrong

A terminal launcher and a performance-focused Minecraft client for Fabric.

- **Launcher** (Go, terminal UI): Microsoft and offline accounts, instances,
  a Modrinth mod browser with compatibility checks, and automatic updates.
- **Client** (Fabric mod, `client-mod/`): a module menu (Right Shift), HUD
  modules and a terminal-style look, shipped with a tuned performance stack
  (Sodium, Lithium, FerriteCore, ImmediatelyFast, Entity Culling, Dynamic FPS)
  that the launcher downloads from Modrinth.

## Install

**Linux and macOS** (in a terminal):

```sh
curl -fsSL https://raw.githubusercontent.com/ark4660/armstrong/main/scripts/install.sh | sh
```

**Windows**: download and run
[ArmstrongSetup.exe](https://github.com/ark4660/armstrong/releases/latest/download/ArmstrongSetup.exe).
It's unsigned for now, so Windows may show "Windows protected your PC": click
**More info**, then **Run anyway**. Uninstall from Settings → Apps → Installed
apps. (Or, from PowerShell:
`irm https://raw.githubusercontent.com/ark4660/armstrong/main/scripts/install.ps1 | iex`.)

Then start Armstrong from your applications menu (Linux) or Start Menu
(Windows), or open a new terminal and run `armstrong`. No admin rights are needed:
it installs to `~/.local/bin` or `%LOCALAPPDATA%\Programs\Armstrong`, where it
can keep itself up to date. When a new release is out it shows
`update available` (press `U` on Home); the client updates on its own before
you play. From a terminal: `armstrong update`.

To uninstall: `curl -fsSL …/install.sh | sh -s -- --uninstall` on Linux and
macOS; on Windows, set `$env:ARMSTRONG_UNINSTALL = 1` before running the
PowerShell command above.

You can also download a launcher directly from the
[latest release](https://github.com/ark4660/armstrong/releases/latest).

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

## Benchmarking

`armstrong bench` measures frame times in a fresh world with a fixed seed and
fixed settings, so changes can be compared run for run. The client mod creates
the world, holds the camera on a scripted path and writes a JSON result per run
(average fps, 1% and 0.1% lows, p99, stutters, GC pauses, render-thread
allocation per frame, heap, time to the world).

```
armstrong bench -label before                       # static, fly and entities scenes on 26.3
armstrong bench -label zgc -preset zgc -runs 2      # another configuration
armstrong bench compare ~/.local/share/armstrong/bench/results/{before,zgc}
```

Scenes: `static`, `fly` (new chunks), `spin`, `entities` (144 villagers in view),
`menu`. Other flags: `-mc`, `-without sodium`, `-only lithium`, `-addons iris`,
`-no-modules`, `-jvm ARG`, `-env K=V`, `-opt key:value` (options.txt), `-wrapper`.
The game window opens for every run; keep it focused (Dynamic FPS is switched off,
and runs with an unfocused window are flagged). Only compare runs from the same
machine and Minecraft version.

Minecraft is a trademark of Mojang. Armstrong is not affiliated with Mojang or Microsoft.
