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

## Usage

```sh
tuicast record \
  --binary ./myapp \
  --keystrokes "wait:2000,Tab,Enter,wait:1000,Ctrl+C" \
  --output demo.gif
```

## Status

🚧 **Pre-alpha** — rewriting from Node.js prototype to Go. See [spec.md](spec.md) for the v1 plan.