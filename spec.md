# TUIcast v1 — Tightened Plan

## Summary

Rewrite TUIcast as a **cross-platform Go CLI** that records any terminal app and produces an animated GIF. v1 is scripted-only (no AI). The hosted service is a future milestone.

---

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Cross-platform, single binary, fast builds, good PTY support, learning goal |
| Scope | CLI tool only | Hosted service deferred; no Azure/API/queue in v1 |
| Input mode | Scripted keystrokes only | AI navigation deferred to v2 |
| Target apps | Any terminal app | Not limited to Terminal.Gui or .NET |
| Platforms | Windows (ConPTY), macOS, Linux | Cross-platform from day one |
| GIF renderer | agg (external binary) | Proven in prototype; MIT; static binary |
| Recording format | asciinema v2 cast (native) | Simple JSON-lines; no AGPL dependency |

---

## v1 Requirements

### Functional Requirements

1. **FR-1: Spawn any terminal app in a PTY**
   - Accept a path to any executable (+ optional args)
   - Configure terminal dimensions (cols × rows)
   - Set TERM, COLORTERM environment variables
   - Support Windows (ConPTY), macOS, and Linux

2. **FR-2: Inject a scripted keystroke sequence**
   - Named keys: Enter, Tab, Escape, Arrow keys, F1–F12, Ctrl+letter, Alt+letter
   - Mouse clicks: `click:col:row`
   - Wait/delay: `wait:<ms>`
   - Literal text strings
   - Configurable inter-keystroke delay (default 200ms)

3. **FR-3: Record the PTY session as an asciinema v2 cast file**
   - Native implementation (JSON header + event lines)
   - Capture all PTY output with accurate timestamps
   - Output `.cast` file

4. **FR-4: Render the cast file to an animated GIF**
   - Invoke `agg` binary (user must have it installed or we bundle it)
   - Configurable: theme, speed, font, fontSize, line-height
   - Validate output (GIF magic bytes, minimum size)

5. **FR-5: Max duration timeout**
   - Hard cap on recording duration (default 60s, configurable)
   - Graceful teardown: kill PTY process on timeout

6. **FR-6: Exit codes and error reporting**
   - Exit 0 on success
   - Non-zero + stderr message on failure
   - Validate agg is available before starting

### Non-Functional Requirements

1. **NFR-1: Single static binary** — `go build` produces one file, no runtime dependencies
2. **NFR-2: Cross-platform** — CI tests on ubuntu, macos, windows
3. **NFR-3: Fast** — CLI overhead < 100ms; recording adds negligible latency
4. **NFR-4: No network required** — v1 works entirely offline
5. **NFR-5: agg is the only external dependency** — clearly documented
6. **NFR-6: Easy install** — users can install via Homebrew, Scoop, `go install`, or download a binary from GitHub Releases
7. **NFR-7: Automated releases** — pushing a `v*` tag builds cross-platform binaries and publishes everywhere

---

## CLI Interface

```
tuicast record \
  --binary <path>           # Path to executable (required)
  --args <arg1,arg2,...>    # Arguments to pass to the binary
  --output <path>           # Output GIF path (default: recording.gif)
  --cast-output <path>      # Also save the raw cast file
  --keystrokes <csv>        # Keystroke sequence (default: "wait:3000,Ctrl+C")
  --keystroke-delay <ms>    # Default pause between keystrokes (default: 200)
  --cols <n>                # Terminal columns (default: 120)
  --rows <n>                # Terminal rows (default: 30)
  --theme <name>            # agg color theme (default: monokai)
  --font <name>             # Font family (omit for agg built-in bitmap)
  --font-size <n>           # Font size in px (default: 14)
  --line-height <f>         # Vertical spacing multiplier (default: 1.0)
  --speed <f>               # GIF playback speed multiplier (default: 1.0)
  --max-duration <s>        # Max recording seconds (default: 60)
  --title <text>            # Title embedded in cast file
  --agg-path <path>         # Path to agg binary (default: find in PATH)
```

---

## Architecture

```
┌─────────────────────────────────────────────┐
│              tuicast CLI (Go)                │
│                                             │
│  ┌─────────┐   ┌──────────┐   ┌─────────┐ │
│  │  PTY    │──▶│ Recorder │──▶│   agg   │ │
│  │ Session │   │ (cast)   │   │ (GIF)   │ │
│  └─────────┘   └──────────┘   └─────────┘ │
│       ▲                                     │
│       │                                     │
│  ┌─────────┐                                │
│  │Keystroke│                                │
│  │ Player  │                                │
│  └─────────┘                                │
└─────────────────────────────────────────────┘
```

### Key Go packages/modules:

| Package | Purpose | Notes |
|---------|---------|-------|
| `creack/pty` | Unix PTY spawn | Linux + macOS |
| `UserExistsError/conpty` | Windows ConPTY | Windows |
| `cobra` or `pflag` | CLI argument parsing | |
| `os/exec` | Invoke agg | |
| (stdlib) | JSON, time, IO | asciinema recorder is ~50 lines |

### Module structure:

```
cmd/
  tuicast/
    main.go           # Entry point, CLI parsing
pkg/
  pty/
    session.go        # Cross-platform PTY spawn/read/write
    session_unix.go   # Unix-specific (creack/pty)
    session_windows.go # Windows ConPTY
  recorder/
    cast.go           # asciinema v2 writer
  keystroke/
    player.go         # Parse + execute keystroke script
    keymap.go         # Named key → escape sequence mapping
  gif/
    renderer.go       # Invoke agg, validate output
  record/
    pipeline.go       # Orchestrate: PTY + recorder + player + renderer
```

---

## Implementation Phases

| Phase | Scope | Exit Criteria |
|-------|-------|---------------|
| 1 | Project scaffold + cross-platform PTY session | Spawn an app, read output, write input, clean exit on Linux, macOS, and Windows |
| 2 | asciinema v2 recorder | Produce valid .cast file playable by `asciinema play` |
| 3 | Keystroke player + key map | Parse CSV script, inject keys with delays |
| 4 | GIF renderer (agg invocation) | Produce valid GIF from cast; validate magic bytes |
| 5 | CLI wiring (cobra) | Full `tuicast record` command with all flags |
| 6 | Integration tests | End-to-end: record a simple app → valid GIF on all 3 OS |

---

---

## Distribution

### Install Methods

| Method | Command | Platform |
|--------|---------|----------|
| Homebrew | `brew install gui-cs/tap/tuicast` | macOS, Linux |
| Scoop | `scoop bucket add gui-cs https://github.com/gui-cs/scoop-bucket && scoop install tuicast` | Windows |
| Go install | `go install github.com/gui-cs/TUIcast/cmd/tuicast@latest` | Any (requires Go) |
| Binary download | GitHub Releases page | Any |

### Release Process

1. Tag a commit: `git tag v0.1.0 && git push --tags`
2. GoReleaser (via `.github/workflows/release.yml`) builds for linux/darwin/windows × amd64/arm64
3. Creates GitHub Release with tarballs, zips, and checksums
4. Updates Homebrew tap formula (`gui-cs/homebrew-tap`)
5. Updates Scoop manifest (`gui-cs/scoop-bucket`)

### CI

- `.github/workflows/ci.yml` — build + unit tests + lint on all 3 OS on every push/PR
- `.github/workflows/release.yml` — GoReleaser on `v*` tag push

---

## Out of Scope (v1)

- AI/Claude navigation (v2)
- Hosted service / API / queue / cloud storage (v2+)
- GitHub Actions reusable action (nice-to-have, post-v1)
- Web UI
- Multi-user, billing, auth
- Docker images
- Binary upload / GitHub repo source resolution
- xterm.js / headless VT parsing

---

## Key Risks

| Risk | Mitigation |
|------|-----------|
| Windows ConPTY quirks | `UserExistsError/conpty` wraps the stable Windows ConPTY API |
| agg not available on all platforms | Document as prerequisite; provide install instructions per OS |
| Key map incompleteness | Comprehensive map from day 1; unit test every named key |
| Learning Go | Project is well-scoped; PTY handling is the only complex part |

---

## Success Criteria

v1 is done when:
1. `tuicast record --binary /path/to/app --keystrokes "..." --output demo.gif` produces a valid animated GIF
2. Works on Linux, macOS, and Windows
3. Single binary, no runtime deps beyond agg
4. README with install instructions and usage examples
5. CI passing on all 3 platforms
