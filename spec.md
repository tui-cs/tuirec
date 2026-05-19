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
| PTY driver deps | `pkg/pty` may import `github.com/creack/pty` (Unix) + `github.com/UserExistsError/conpty` (Windows) | Resolves the constitution R1 conflict; amended to R1 v1.1. Hand-rolling `forkpty`/ConPTY is out of scope |
| Windows ConPTY library | `github.com/UserExistsError/conpty` | Resolves CLAUDE.md open decision #1. The previously-referenced `iamacarpet/go-conpty` **does not exist** as a module. `UserExistsError/conpty` builds and passes on Windows — proven by the Phase 1 spike (PR #3) |
| Windows in v1 | **In scope — folded back into Phase 1** (not a deferred spike) | PR #3 is the spike evidence: ConPTY works. Cross-platform PTY is a Phase 1 deliverable again. The only Windows item still deferred is agg-on-Windows in CI for full-GIF integration |
| Module path | `github.com/gui-cs/TUIcast` (exact case) | Go import paths are case-sensitive; must match `go.mod`, README, and `.goreleaser.yaml` |
| agg distribution | Require as a prerequisite; **pin agg `v1.5.0`** | Resolves open decision "bundle vs require". CI installs it per-OS (see CI section). Bundling deferred post-v1. Upstream does not publish a native Windows ARM64 asset for `v1.5.0`; Windows ARM64 users should use the x64 Windows binary via OS emulation or build `agg` from source and pass `--agg-path`/`-agg-path` |
| Recording clock | Support a deterministic (scripted) clock in addition to wall-clock | Wall-clock timing makes GIFs non-reproducible and CI flaky; scripted timing enables golden-GIF regression |

---

## v1 Requirements

### Functional Requirements

1. **FR-1: Spawn any terminal app in a PTY**
   - Accept a path to any executable (+ optional args)
   - Configure terminal dimensions (cols × rows)
   - Set TERM, COLORTERM environment variables
   - Support Windows (ConPTY), macOS, and Linux

2. **FR-2: Inject a scripted keystroke sequence**
   - Tokens: named keys, mouse clicks, waits, literal text
   - Configurable inter-keystroke delay (default 200ms)
   - The grammar, precedence rules, escaping, and the complete named-key
     table are normative and specified in
     [§ Keystroke Script Grammar](#keystroke-script-grammar). Implement to
     that section exactly — it is the single biggest source of "plausible
     but wrong" behavior.

3. **FR-3: Record the PTY session as an asciinema v2 cast file**
   - Native implementation. Line 1 is a JSON header object:
     `{"version":2,"width":<cols>,"height":<rows>,"timestamp":<unix-secs>,"title":<title>,"env":{"TERM":"xterm-256color"}}`.
     Subsequent lines are `[<elapsed-seconds-float>, "o", <utf8-string>]`.
   - Elapsed time is relative to recording start, in seconds, as a float.
   - **UTF-8 boundary safety:** the PTY is read as raw bytes. A multi-byte
     rune split across two reads must be buffered and not flushed mid-rune,
     or the JSON `data` string corrupts. Unit-test with a split sequence.
   - **Streaming, not buffered:** write events as they arrive; do not retain
     all events in memory (the prototype's `recorder.ts` does — do not port
     that). Recordings may be long.
   - **Clock mode** (see Decisions): `--clock wall` (default) uses real
     elapsed time; `--clock scripted` advances the cast clock only by
     keystroke/`wait:` delays, making output byte-reproducible.
   - Output a `.cast` file when `--cast-output` is set.
   - **Exit criterion is a golden-file test**, not "playable by
     `asciinema play`" (that needs asciinema installed and is
     non-deterministic). With `--clock scripted` + the test fixture, the
     `.cast` is byte-stable; store it in `testdata/` and diff.

4. **FR-4: Render the cast file to an animated GIF**
   - Invoke pinned `agg v1.5.0` (prerequisite; see Decisions + CI).
   - Flag mapping (apply only the flags whose CLI value is set):

     | CLI flag | agg flag | Notes |
     |----------|----------|-------|
     | `--theme` | `--theme <name>` | default `monokai` |
     | `--speed` | `--speed <f>` | default `1.0` |
     | `--font-size` | `--font-size <n>` | default `14` |
     | `--line-height` | `--line-height <f>` | **default `1.0`**, not agg's own `1.4`. PR #1: `1.4` adds a visible blank ~40% strip between rows in TUI recordings |
     | `--font` | `--font-family <name>` | **Omit entirely when unset.** PR #1: passing `--font-family` on a host without that font installed makes agg fail |

   - **Validation must be stronger than magic bytes** (a blank/all-black
     GIF passes that). Decode the GIF and assert: ≥ 2 frames, non-zero
     dimensions, and non-trivial inter-frame pixel variance (the screen
     actually changed). For the test fixture, golden-frame compare.

5. **FR-5: Max duration timeout**
   - Hard cap on recording duration (default 60s, configurable)
   - Graceful teardown: kill PTY process on timeout

6. **FR-6: Exit codes and error reporting**
   - Stable exit-code table so CI can assert specific failures:

     | Code | Meaning |
     |------|---------|
     | 0 | Success |
     | 1 | Generic/unexpected runtime error |
     | 2 | Usage error (bad flags / unparseable keystroke script) |
     | 3 | Prerequisite missing (`agg` not found, target binary not found/executable) |
     | 4 | Recording hit `--max-duration` and was force-terminated |
     | 5 | GIF produced but failed validation |

   - Every non-zero exit writes a single actionable line to stderr.
   - Only `main()` calls `os.Exit` (constitution R3).

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
| `github.com/UserExistsError/conpty` | Windows ConPTY | Windows (`iamacarpet/go-conpty` does not exist — do not use) |
| `cobra` or `pflag` | CLI argument parsing | |
| `os/exec` | Invoke agg | |
| (stdlib) | JSON, time, IO | asciinema recorder is ~50 lines |

**Canonical module path:** `github.com/gui-cs/TUIcast` (exact case — Go
import paths are case-sensitive). `go.mod` must declare exactly this.

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
internal/
  testapp/
    main.go           # Tiny deterministic TUI used as the test fixture
```

### Test Fixture (`internal/testapp`)

Every phase after PTY needs something to record. v1 must **not** depend on
an external app (PR #1 used UICatalog — a .NET 10 + Terminal.Gui
`v2_develop` clone, ~90s, unusable as a Go self-test). `internal/testapp`
is a ~30-line pure-Go TUI compiled by the test harness:

- On start, clears the screen and prints a known banner + a cursor block.
- Reads stdin: arrow keys move the block; printable keys echo at the cursor.
- Exits cleanly on `Ctrl+Q` (`\x11`) — exercises the exact bug PR #1 hit.
- No deps, deterministic output → enables golden `.cast` and golden-frame
  GIF assertions, and a self-contained integration test on every OS.

---

## Keystroke Script Grammar

The `--keystrokes` value is a comma-separated token list. **Implement this
section verbatim.**

### Grammar (EBNF)

```
script   = token { "," token } ;
token    = wait | click | namedkey | literal ;
wait     = "wait:" digit { digit } ;        (* milliseconds, integer *)
click    = "click:" int ":" int ;           (* 1-based col ":" row *)
namedkey = (* an exact, case-sensitive entry in the Named-Key table *) ;
literal  = (* anything else: typed verbatim, rune by rune *) ;
```

### Resolution precedence (per token, first match wins)

1. Matches `wait:<digits>` → delay that many ms (no extra keystroke-delay after).
2. Matches `click:<int>:<int>` → SGR mouse click (see table).
3. Matches a Terminal.Gui-compatible key token → its sequence.
4. Key-like unknown tokens are errors instead of literals. This includes
   malformed `wait:`/`click:` tokens, unknown modifier combinations,
   unknown cursor/page names, and unsupported function keys.
5. Otherwise → literal; type each rune with `--keystroke-delay` between runes.

### Separators & escaping

- List separator is `,`. Literal comma = `\,`; literal backslash = `\\`.
- `click` uses `:` sub-separators — PR #1 chose `:` specifically to avoid
  the `,` list-separator conflict. Keep that.
- Key tokens are compatible with Terminal.Gui's `Key.ToString()` /
  `Key.TryParse()` persisted format. `+` is the canonical separator, and
  Terminal.Gui-compatible alternate separators/orderings such as `Ctrl-C` and
  `A-Ctrl` are accepted. Modifier and key names are case-insensitive.
- Named keys and clicks are followed by one `--keystroke-delay`.
- Default `--keystrokes` is `wait:3000,Ctrl+C`. Many TUIs ignore `Ctrl+C`;
  document that the default commonly relies on `--max-duration` teardown
  (exit 4) and recommend an explicit quit key (e.g. `Ctrl+Q`) for a clean
  exit. (Changing the default value itself is an open decision — surface,
  don't silently change.)

### Key tokens (Terminal.Gui-compatible — constitution R6)

Seeded from Terminal.Gui `Key.ToString()` / `Key.TryParse()` and the proven
prototype `keys.ts`; **F11/F12 and `Alt+<char>` were missing there and are
added here** — every row needs a unit test.

| Key | Sequence | Key | Sequence |
|-----|----------|-----|----------|
| `Enter` / `Return` | `\r` | `Home` | `\x1b[H` |
| `Tab` | `\t` | `End` | `\x1b[F` |
| `Esc` / `Escape` | `\x1b` | `PageUp` | `\x1b[5~` |
| `Backspace` | `\x7f` | `PageDown` | `\x1b[6~` |
| `Delete` | `\x1b[3~` | `F1` | `\x1bOP` |
| `CursorUp` / `ArrowUp` | `\x1b[A` | `F2` | `\x1bOQ` |
| `CursorDown` / `ArrowDown` | `\x1b[B` | `F3` | `\x1bOR` |
| `CursorRight` / `ArrowRight` | `\x1b[C` | `F4` | `\x1bOS` |
| `CursorLeft` / `ArrowLeft` | `\x1b[D` | `F5` | `\x1b[15~` |
| `F6` | `\x1b[17~` | `F7` | `\x1b[18~` |
| `F8` | `\x1b[19~` | `F9` | `\x1b[20~` |
| `F10` | `\x1b[21~` | `F11` | `\x1b[23~` |
| `F12` | `\x1b[24~` | `Ctrl+A`…`Ctrl+Z` / `Ctrl-A`…`Ctrl-Z` / `A-Ctrl`…`Z-Ctrl` | `\x01`…`\x1a` |
| `Alt+<char>` / `Alt-<char>` | `\x1b` + `<char>` | `Shift+Tab` | `\x1b[Z` |

Modified cursor/navigation/function keys use standard xterm modifier sequences
where available, e.g. `Ctrl+Alt+Shift+CursorUp` → `\x1b[1;8A` and
`Ctrl+Alt+Shift+Delete` → `\x1b[3;8~`. Modified Enter/Tab/Esc fall back to
CSI-u sequences when there is no legacy escape sequence.

Mouse: `click:col:row` → SGR press+release, 1-based:
`\x1b[<0;col;rowM` immediately followed by `\x1b[<0;col;rowm`.

## Concurrency & Teardown

The orchestrator (`pkg/record`) owns lifecycle. This is the part most
likely to hang or race; implement it as specified:

- A single `context.Context` carrying the `--max-duration` deadline is the
  **only** teardown trigger. Everything selects on `ctx.Done()`.
- Goroutines: (a) copy PTY → recorder; (b) run the keystroke player → PTY.
  The recorder is written from exactly one goroutine (no data race).
- **Sole owner of PTY/process close is the orchestrator**, on the first of:
  child exits, player finishes, ctx deadline, or fatal read error.
- **Drain window:** after the last keystroke, keep reading the PTY for a
  short grace period (default ~500ms, ≥ `--keystroke-delay`) so the final
  UI frame lands in the cast — without it the GIF cuts off before the
  result is visible. This directly affects whether output "works great".
- Read-after-exit must be normalized to `io.EOF` per platform: on Unix a
  PTY read returning `EIO` (`input/output error`) after the child exits is
  **normal EOF, not a failure**; on Windows ConPTY the equivalent
  closed-pipe / broken-pipe error after exit is likewise EOF. `pkg/pty`
  owns this normalization so callers see a single clean stream end —
  otherwise every successful run reports a spurious failure.
- On ctx deadline, kill the child (process group) and exit 4.
- Verify with the race detector: `go test -race ./pkg/record`.

## Implementation Phases

Every exit gate is a **runnable command**, not prose — that is what makes
the build self-verifying and autonomous. Every phase that adds user-visible
behavior must also include a **demo gate**: a command a user can run locally
to see the current capability, preferably producing an artifact they can open
(`.cast`, `.gif`, or CLI output). If a phase is infrastructure-only, mark the
demo gate as **N/A** and say why. The Windows ConPTY spike already ran (PR #3)
and resolved **go**, so Phase 1 is cross-platform: there is no separate spike
row and no Unix-only fallback.

| Phase | Scope | Verifiable Test Gate | User-runnable Demo Gate |
|-------|-------|----------------------|-------------------------|
| 0 | Scaffold: `go.mod` (canonical path), package skeletons, `internal/testapp`, CI wired with pinned `agg` | `go build ./...`, `go vet ./...`, `golangci-lint run ./...` green on the CI matrix | N/A — scaffold/CI only. README must still show local build/run commands. |
| 1 | **Cross-platform** PTY session: Unix `creack/pty` + Windows `UserExistsError/conpty` | `go test ./pkg/pty` (untagged) green on ubuntu + macOS + windows: spawn `internal/testapp`, send `Ctrl+Q`, assert clean exit; platform read-after-exit (`EIO` on Unix, ConPTY close on Windows) normalized to EOF | `go run ./internal/testapp` lets a user see the deterministic fixture and quit with `Ctrl+Q`. |
| 2 | asciinema v2 recorder (streaming, UTF-8-safe, scripted clock) | `go test ./pkg/recorder`: golden `.cast` byte-match + split-rune test | Add/keep a demo command that writes `demo.cast` from fixture output, then document how to inspect/play it if `asciinema` is installed. |
| 3 | Keystroke player + complete key map | `go test ./pkg/keystroke`: a row for **every** Named-Key entry (R6) + grammar/escaping tests | N/A acceptable if folded into the next recording demo; otherwise provide a small script-preview demo that shows parsed actions for a sample `--keystrokes` value. |
| 4 | GIF renderer | `go test -tags integration ./pkg/gif`: render fixture cast, decode GIF, assert >= 2 frames + pixel variance + golden frame | `go run ./examples/render-gif -output ./demo.gif`, then open `demo.gif`. The demo auto-detects `./tools/agg.exe`/`./tools/agg` before PATH. |
| 5 | `pkg/record` orchestration + teardown | `go test -race ./pkg/record`: deadline, drain window, single-owner close, no data race | `go run ./examples/record-pipeline -output ./pipeline-demo.gif -cast-output ./pipeline-demo.cast`, then open `pipeline-demo.gif`. The demo auto-detects `./tools/agg.exe`/`./tools/agg` before PATH. |
| 6 | CLI wiring (cobra), all flags + exit codes | `go test ./cmd/tuicast`: flag parsing + exit-code table; `--help` snapshot | `go run ./cmd/tuicast record --binary go --args run,./internal/testapp --keystrokes "wait:1000,Ctrl+Q" --output ./cli-demo.gif --cast-output ./cli-demo.cast`, then open `cli-demo.gif`. |
| 7 | End-to-end GIF integration | `go test -tags integration ./...` green on ubuntu + macOS: real `tuicast record` command + `internal/testapp` -> validated GIF. Windows PTY is already covered by Phase 1; Windows full-GIF integration follows in Phase 8. | Same CLI command as Phase 6, documented as the canonical README quickstart using `internal/testapp`. |
| 8 | Windows full-GIF integration | `go test -tags integration ./...` green on ubuntu + macOS + windows: CI installs pinned `agg v1.5.0` on Windows and runs the real CLI `internal/testapp` -> validated GIF path there too. | Same canonical CLI demo on Windows, using repo-local `tools\agg.exe` or `agg` on PATH. |

---

## Distribution

### Install Methods

| Method | Command | Platform |
|--------|---------|----------|
| Go install | `go install github.com/gui-cs/TUIcast/cmd/tuicast@latest` | Any (requires Go) |
| Binary download | GitHub Releases page | Any |
| Homebrew | Planned after tap repo + token setup | macOS, Linux |
| Scoop | Planned after bucket repo + token setup | Windows |

### Release Process

1. Tag a commit: `git tag v0.1.0 && git push --tags`
2. GoReleaser (via `.github/workflows/release.yml`) builds for linux/darwin/windows × amd64/arm64
3. Creates GitHub Release with tarballs, zips, and checksums
4. Homebrew tap and Scoop bucket publishing are enabled after the target repos
   and tokens exist.

### CI

- `.github/workflows/ci.yml` — already present. Pins **Go 1.22**, runs
  build + unit tests + `go vet` on ubuntu/macOS/windows, `golangci-lint`
  on ubuntu, and an integration job that installs **pinned `agg v1.5.0`**
  per-OS (Linux `x86_64-unknown-linux-musl`, macOS `aarch64-apple-darwin`,
  Windows `x86_64-pc-windows-msvc.exe`) then runs
  `go test -tags integration ./...`. The Windows integration job exercises
  both ConPTY and the full cast→GIF path.
- `golangci-lint-action` is pinned to a fixed `golangci-lint` version and
  `.golangci.yml` documents the enabled linters.
- `.github/workflows/release.yml` — GoReleaser on `v*` tag push.

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

## Lessons Carried Over From the Prototype (PR #1)

The Node.js prototype proved the pipeline but discovered concrete gotchas.
Encode these so they are **not** rediscovered:

- `Ctrl+Q` (`\x11`) was missing from the key map → sessions hung until
  timeout. The full table above includes it; the testapp exits on it.
- The prototype key map also lacked **F11/F12** and **`Alt+<char>`** —
  added to the normative table above.
- `agg`'s default `--line-height 1.4` adds a visible blank strip between
  rows in TUI recordings; default to `1.0`.
- Passing `--font-family` to `agg` on a host without that font fails;
  omit the flag entirely when `--font` is unset.
- Mouse uses SGR press+release (`\x1b[<0;col;rowM` / `…m`), 1-based.
- The prototype reads decoded JS strings so it dodged UTF-8 boundary
  bugs; Go reads raw PTY **bytes** — split runes must be buffered (FR-3).
- The prototype recorder buffered all events in memory — do not port that;
  stream to disk.
- `github.com/iamacarpet/go-conpty` (named in early drafts) **does not
  exist** as a Go module. The working Windows ConPTY library is
  `github.com/UserExistsError/conpty` (Phase 1 spike, PR #3).
- If anyone records an external Terminal.Gui app instead of the testapp:
  it has no `main` branch (use `v2_develop`), targets `net10.0`, and
  UICatalog lives at `Examples/UICatalog/UICatalog.csproj`. v1 should
  rely on `internal/testapp` and not take this dependency.

## Key Risks

| Risk | Mitigation |
|------|-----------|
| Windows ConPTY quirks | **Resolved:** spike (PR #3) proved `github.com/UserExistsError/conpty` builds + passes on Windows; folded into Phase 1. Full Windows cast→GIF integration is covered by the CI integration job. |
| agg not available on all platforms | Pinned `agg v1.5.0`, installed per-OS in CI; required-vs-skip behavior defined; documented prerequisite for users |
| Key map incompleteness | Full normative table in-spec (incl. the F11/F12/`Alt` gaps the prototype had); R6 unit test per row; the `Ctrl+Q` bug is the testapp's exit path |
| Weak success signal | GIF validation decodes frames + asserts pixel variance + golden compare, not just magic bytes |
| Non-reproducible recordings | `--clock scripted` makes `.cast`/GIF byte-stable for golden regression |
| Windows ARM64 agg availability | Use upstream x64 Windows `agg` under Windows emulation for demos; validated on Windows ARM64. Native Windows ARM64 `agg` is a tracked follow-up unless upstream ships an asset |
| Concurrency hangs/races | Single-context teardown, single PTY-close owner, drain window, `EIO`-as-EOF, `-race` gate |
| Learning Go | Project is well-scoped; PTY handling is the only complex part |

---

## Success Criteria

v1 is done when:
1. `tuicast record --binary /path/to/app --keystrokes "..." --output demo.gif`
   produces a GIF that **decodes to ≥ 2 frames with non-trivial inter-frame
   pixel variance** (not just valid magic bytes)
2. Every phase exit gate passes: `go test ./...`, `go test -race ./pkg/record`,
   and `go test -tags integration ./...` green
3. PTY recording and the full cast→GIF pipeline work on Linux, macOS,
   **and Windows** (ConPTY)
4. Single binary, no runtime deps beyond pinned `agg v1.5.0`
5. README with install instructions and usage examples
6. CI green on the matrix (untagged tests and `integration` job on Linux,
   macOS, and Windows)
