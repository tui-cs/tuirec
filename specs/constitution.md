# TUIcast Constitution

**Version**: 1.1 | **Ratified**: 2026-05-19 | **Last Amended**: 2026-05-19

This constitution governs all contributions to `gui-cs/TUIcast`. It is the highest-authority document in the repository — PRs that violate it are rejected with a link to the specific rule.

---

## I. Purpose & Scope

TUIcast is a cross-platform CLI tool that records any terminal application session and produces an animated GIF. It spawns an app in a PTY, injects a scripted keystroke sequence, captures all terminal output as an asciinema v2 cast file, and renders it to GIF via `agg`.

**Primary use cases:**

1. Generate demo GIFs for project READMEs and documentation.
2. Automate visual regression captures in CI pipelines.
3. Produce tweet-ready recordings of TUI app features without manual screen recording.

TUIcast is **not** a terminal emulator, not a screen recorder with audio, and not a cloud service (v1).

## II. Non-Goals

These were considered and explicitly rejected for v1 — do not accidentally pursue them:

- AI/LLM-driven navigation (deferred to v2).
- A hosted web service, API, queue, or cloud storage backend.
- Video output formats (MP4, WebM). GIF only.
- A terminal emulator or VT parser (we record raw PTY output, not rendered frames).
- Backwards compatibility with the Node.js prototype.
- GUI or web UI of any kind.

## III. Tenets

### This Is Fun

While customers may take what we build seriously, we do this for fun, and we insist on using levity and humor — often cutting — throughout. The beatings will continue until morale improves.

We do this with a modicum of respect and a desire to not offend.

### Excellent Engineering

Developers — AI agents and humans — working on this project strive to raise the bar as Principal Engineers. Principal Engineers are measured by how they live the [Amazon PE Community Tenets](https://www.amazon.jobs/content/en/teams/principal-engineering/tenets):

1. **Exemplary practitioner** — set the standard through your own work.
2. **Technically fearless** — tackle the hardest, most ambiguous problems.
3. **Lead with empathy** — foster inclusion; be mindful of your impact.
4. **Balanced and pragmatic** — neither dogmatic nor reckless.
5. **Illuminate and clarify** — bring clarity to complexity; drive crisp decisions.
6. **Flexible in approach** — adapt style and methods to the problem at hand.
7. **Respect what came before** — appreciate existing systems; learn from the past.
8. **Learn, educate, and advocate** — pursue continuous learning and teach others.
9. **Have resounding impact** — results are the minimum; lasting impact is the bar.

### Simple and Correct

TUIcast is a pipeline tool. Each stage (PTY → recorder → keystroke player → GIF renderer) does one thing well. Complexity belongs in the orchestration, not in the components.

- Prefer the standard library over external dependencies.
- Prefer explicit over clever.
- Prefer working cross-platform over working perfectly on one OS.

### Delightful Developer Experience

TUIcast serves three customers, listed in the order in which tradeoffs are made:

1. **CLI users** running `tuicast record` to produce a GIF.
2. **CI pipeline authors** integrating TUIcast into automated workflows.
3. **Contributors** (human or AI) extending or maintaining the tool.

The CLI must have excellent error messages, sensible defaults, and predictable behavior.

### Performance Matters

Recording adds negligible overhead to the target app. The bottleneck is `agg` (GIF rendering), which is out of our control. Our code must never be the slow part.

### Go Is the Right Tool

We chose Go for single-binary distribution, cross-platform support, fast compilation, and approachability. We are good citizens of the Go ecosystem: follow `gofmt`, use modules, keep dependencies minimal, write idiomatic code.

## IV. Architectural Rules

Every PR must comply. Reviewers (human or agent) must reject violations and cite the rule number.

### R1 — Package boundaries are load-bearing

The five core packages (`pty`, `recorder`, `keystroke`, `gif`, `record`) have strict import rules:

- `pkg/pty` imports only stdlib + the platform PTY driver libraries (`github.com/creack/pty` on Unix, the chosen ConPTY library on Windows). No other external dependencies.
- `pkg/recorder` imports only stdlib.
- `pkg/keystroke` imports only stdlib.
- `pkg/gif` imports only stdlib + `os/exec`.
- `pkg/record` is the sole orchestrator; it may import all `pkg/*` packages.
- `cmd/tuicast` imports `pkg/record` and `cobra`. Nothing else from `pkg/`.

A package may take an external dependency **only** when (a) this rule explicitly grants it and (b) the dependency is listed in `spec.md`'s dependency table with a one-line justification. The bar stays high — prefer the standard library; the PTY/ConPTY drivers and the CLI framework are the only anticipated exceptions. R1's intent is to keep leaf packages thin and decoupled, **not** to forbid the irreducible platform primitives that `pkg/pty` is built on. Reimplementing `forkpty`/ConPTY by hand to satisfy a literal "stdlib only" reading is explicitly *not* required and not desired.

No circular imports. No "utils" package. Shared types live in `pkg/record` (the orchestrator) or are duplicated (prefer duplication over coupling).

### R2 — No global mutable state

No package-level `var` that gets mutated. No `init()` functions that set state. All configuration flows through function parameters or struct fields. This enables parallel testing and prevents action-at-a-distance bugs.

### R3 — Errors are values, not panics

Functions return `error`. Only `main()` may call `os.Exit()`. Never `panic` in library code except for genuinely unrecoverable programmer errors (unreachable code after exhaustive switch). Wrap errors with context: `fmt.Errorf("spawn pty: %w", err)`.

### R4 — Platform code is build-tag separated

Platform-specific implementations live in `_unix.go` / `_windows.go` / `_darwin.go` files with the appropriate `//go:build` tag. The shared file defines the interface or common types. Tests that need a real PTY use `t.Skip` on unsupported platforms rather than build tags where possible.

### R5 — Tests are fast and parallelizable

Unit tests (`go test ./...`) must complete in < 5 seconds total. Tests that spawn real processes or invoke `agg` are integration tests (tagged `//go:build integration`). Unit tests must not depend on external binaries.

### R6 — The key map is complete and tested

Every named key in the keystroke specification (see `spec.md` FR-2) has a unit test asserting its escape sequence. No key may be added without a corresponding test. The prototype's `Ctrl+Q` bug must not recur.

### R7 — CLI flags have sensible defaults

A user should be able to run `tuicast record --binary ./myapp` and get a reasonable GIF without specifying any other flags. Defaults are documented in `--help` output and in `spec.md`.

### R8 — External binary invocation is validated upfront

Before starting a recording session, validate that:
- The target binary exists and is executable.
- `agg` is available (unless `--cast-output` is used without `--output`).

Fail fast with a clear error message. Don't record for 60 seconds then fail at the GIF step.

## V. Testing

### Tiers

| Tier | Tag | What it tests | Speed |
|------|-----|---------------|-------|
| Unit | (none) | Pure logic: key map, cast writer, script parser | < 5s total |
| Integration | `integration` | Real PTY + real processes + agg | < 60s total |

### CI Matrix

All tests run on: `ubuntu-latest`, `macos-latest`, `windows-latest`.

Integration tests may skip on Windows until ConPTY support lands (Phase 7).

### Conventions

- Table-driven tests with `t.Run` subtests.
- `t.Parallel()` on every test that doesn't need serial execution.
- Test helpers return errors, not `t.Fatal` — let the caller decide.
- Golden file tests for cast output (store expected `.cast` in `testdata/`).

## VI. Coding Standards

Standard Go. See [CLAUDE.md](../CLAUDE.md) for the full guide. Key points:

- `gofmt` / `goimports` — non-negotiable.
- `golangci-lint` default rules — CI enforced.
- Short functions (< 50 lines).
- Exported names have doc comments.
- No dead code.
- Minimal dependencies.

## VII. Naming Convention for Specs & Features

Every feature, work item, and spec directory must have a plain English name that a user would instantly recognize.

- **No letter+number codes.** Use descriptive English: `keystroke-player`, `pty-session`, `gif-renderer`.
- **Lowercase kebab-case** for directory/file names: `specs/keystroke-format.md`.
- **Title-case** in document headings.
- **Cross-references use the English name**, never an ID.

## VIII. Governance

This constitution supersedes all other documents. Amendments require:

1. A written proposal in a PR touching this file.
2. Review and approval from the project maintainer.
3. A migration plan for any existing code that would be out of compliance.
