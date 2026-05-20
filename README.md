# TUIcast

**Cross-platform CLI that records any terminal app and produces an animated GIF.**

Give it a binary and a keystroke script → get a polished GIF. No manual screen recording, no browser-based tools.

## Install

```sh
# Go (requires Go 1.22+)
go install github.com/gui-cs/TUIcast/cmd/tuicast@latest
```

Or download a binary from [GitHub Releases](https://github.com/gui-cs/TUIcast/releases). Release archives include a pinned `agg v1.5.0` binary next to `tuicast`, and the CLI auto-detects that sibling binary before falling back to `PATH`.
Homebrew and Scoop manifests are planned after the first release automation pass.

**Prerequisite for source builds:** [agg](https://github.com/asciinema/agg) `v1.5.0` renders casts to GIFs. `go install` and local `go build` produce only `tuicast`; install `agg` on your PATH or pass `--agg-path` unless you are using a release archive.

## Build and Run Locally on Windows

The CLI shell, cross-platform PTY, asciinema recorder, keystroke player, GIF renderer, recording pipeline, and `record` command are in place.

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

If `agg` is installed on your PATH or next to `tuicast`, run the GIF renderer and CLI end-to-end integration tests:

```powershell
go test -tags integration .\...
```

To install the pinned `agg` binary locally for demos on Windows:

```powershell
New-Item -ItemType Directory -Force .\tools | Out-Null
Invoke-WebRequest `
  https://github.com/asciinema/agg/releases/download/v1.5.0/agg-x86_64-pc-windows-msvc.exe `
  -OutFile .\tools\agg.exe
.\tools\agg.exe --version
```

On Windows ARM64, upstream `agg v1.5.0` does not publish a native ARM64 Windows binary. The Windows ARM64 TUIcast release archive includes the x64 Windows `agg` binary for Windows x64 emulation (validated on Windows ARM64). You can also build `agg` from source and pass that binary with `--agg-path`. The demo commands automatically prefer `.\tools\agg.exe` when it exists.

To create and open a visible demo GIF from the bundled cast fixture:

```powershell
go run .\examples\render-gif -output .\demo.gif
Invoke-Item .\demo.gif
```

To exercise the full package pipeline against the bundled test TUI and open the result:

```powershell
go run .\examples\record-pipeline -output .\pipeline-demo.gif -cast-output .\pipeline-demo.cast
Invoke-Item .\pipeline-demo.gif
```

To run the real CLI against the bundled test TUI and open the result:

```powershell
go run .\cmd\tuicast record `
  --binary go `
  --args run,.\internal\testapp `
  --keystrokes "wait:1000,ArrowRight,ArrowDown,Hi,wait:500,Ctrl+Q" `
  --output .\cli-demo.gif `
  --cast-output .\cli-demo.cast
Invoke-Item .\cli-demo.gif
```

## Usage

v1 CLI usage:

```sh
tuicast record \
  --binary ./myapp \
  --keystrokes "wait:2000,Tab,Enter,wait:1000,Ctrl+C" \
  --output demo.gif
```

Key tokens use Terminal.Gui's persisted `Key.ToString()` / `Key.TryParse()`
format. `Ctrl+C`, `Ctrl-C`, `A-Ctrl`, `Shift+Tab`, `Ctrl+Alt+Shift+CursorUp`,
`Esc`, `Enter`, `Delete`, and `F1` all work. Older aliases such as `ArrowUp`
and `Escape` are also accepted. Unknown key-like tokens such as `Ctrl-Foo`
fail fast instead of being typed as literal text.

## Status

🚧 **Pre-alpha** — rewriting from Node.js prototype to Go. See [spec.md](spec.md) for the v1 plan.
