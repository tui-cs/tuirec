# TUIcast

**Cross-platform CLI that records any terminal app and produces an animated GIF.**

Give it a binary and a keystroke script → get a polished GIF. No manual screen recording, no browser-based tools, no runtime dependencies beyond `agg`.

## Install

```sh
# macOS / Linux
brew install gui-cs/tap/tuicast

# Windows
scoop bucket add gui-cs https://github.com/gui-cs/scoop-bucket
scoop install tuicast

# Go (requires Go 1.22+)
go install github.com/gui-cs/TUIcast/cmd/tuicast@latest
```

Or download a binary from [GitHub Releases](https://github.com/gui-cs/TUIcast/releases).

**Prerequisite:** [agg](https://github.com/asciinema/agg) must be on your PATH (for GIF rendering).

## Build and Run Locally on Windows

The CLI shell, cross-platform PTY, asciinema recorder, keystroke player, GIF renderer, and recording pipeline packages are in place. The user-facing `record` command wiring is the next phase, so `tuicast record` still reports that it is planned.

From the repo root:

```powershell
go build -o .\tuicast.exe .\cmd\tuicast
.\tuicast.exe --version
.\tuicast.exe --help
```

Run the Windows ConPTY tests:

```powershell
go test .\...
```

If `agg` is installed on your PATH, run the GIF renderer integration test:

```powershell
go test -tags integration .\pkg\gif
```

To create and open a visible demo GIF from the bundled cast fixture:

```powershell
go run .\examples\render-gif -output .\demo.gif
Invoke-Item .\demo.gif
```

## Usage

Planned v1 CLI usage:

```sh
tuicast record \
  --binary ./myapp \
  --keystrokes "wait:2000,Tab,Enter,wait:1000,Ctrl+C" \
  --output demo.gif
```

## Status

🚧 **Pre-alpha** — rewriting from Node.js prototype to Go. See [spec.md](spec.md) for the v1 plan.
