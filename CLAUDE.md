# CLAUDE.md

This file provides guidance to AI coding agents (Claude Code, GitHub Copilot, OpenAI Codex, Grok, etc.) when working with code in this repository.

## Project Status

Pre-alpha. Rewriting from a Node.js/TypeScript prototype to Go. The prototype (on branch `copilot/add-tui-gif-generation-feature`) proved the pipeline works: PTY → asciinema v2 cast → agg GIF. The Go rewrite starts fresh on `main`. See [spec.md](spec.md) for the v1 plan, requirements, and architecture.

## What This Project Does

tuirec is a cross-platform CLI that records any terminal application and produces an animated GIF. You give it a binary and a keystroke script; it spawns the app in a PTY, injects the keystrokes, records all terminal output as an asciinema v2 cast file, then invokes `agg` to render an animated GIF.

## Branch Workflow

Active development happens on **`main`** during pre-alpha. Feature branches are short-lived and merge via PR.

- Work on feature branches. PRs merge to `main`.
- Direct pushes to `main` are allowed for trivial fixes during pre-alpha.

## Build and Test

Requires Go 1.22+ and `agg` (for integration tests that produce GIFs).

```sh
go build ./...
go test ./...
```

Run a single test:

```sh
go test ./pkg/keystroke -run TestKeyMap
```

### Linting

```sh
golangci-lint run ./...
```

CI runs `golangci-lint` and `go vet`. Code must pass both before merge.

## CI/CD

- **`.github/workflows/ci.yml`** — runs on every push/PR to `main`. Builds, tests (unit + integration), vets, and lints across Linux, macOS, Windows.
- **`.github/workflows/release.yml`** — triggered by `v*` tag push. Uses GoReleaser to cross-compile and publish to GitHub Releases, Homebrew tap, and Scoop bucket.
- **`.goreleaser.yaml`** — GoReleaser config. Builds `CGO_ENABLED=0` static binaries for linux/darwin/windows × amd64/arm64.

### Releasing

```sh
git tag v0.1.0
git push --tags
```

That's it. GoReleaser handles the rest.

## Architecture

Single binary CLI. Module structure:

```
cmd/
  tuirec/
    main.go             # Entry point, CLI parsing (cobra)
pkg/
  pty/
    session.go          # Cross-platform PTY interface
    session_unix.go     # Unix (creack/pty)
    session_windows.go  # Windows (ConPTY)
  recorder/
    cast.go             # asciinema v2 cast writer
  keystroke/
    player.go           # Parse + execute keystroke script
    keymap.go           # Named key → escape sequence mapping
  gif/
    renderer.go         # Invoke agg, validate output
  record/
    pipeline.go         # Orchestrate: PTY + recorder + player + renderer
```

The boundary between packages matters:
- `pkg/pty` — only PTY lifecycle (spawn, read, write, resize, close)
- `pkg/recorder` — only cast file I/O (no PTY knowledge)
- `pkg/keystroke` — only script parsing and key mapping (no PTY knowledge)
- `pkg/gif` — only agg invocation (no PTY or recorder knowledge)
- `pkg/record` — orchestration; the only package that imports all others

## Coding Standards

Standard Go. Follow:

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- `gofmt` / `goimports` formatting (non-negotiable)
- `golangci-lint` default rules

### Key Rules

1. **No global mutable state.** Pass dependencies explicitly. No `init()` functions that set package-level vars.
2. **Errors are values.** Return `error`; don't panic. Wrap errors with `fmt.Errorf("context: %w", err)` for stack context.
3. **Interfaces are small.** Define interfaces at the consumer, not the producer. One or two methods max.
4. **Test files live next to code.** `session_test.go` beside `session.go`. Use table-driven tests.
5. **No magic comments or build tags** unless genuinely needed for platform-specific code (`//go:build unix`, `//go:build windows`).
6. **Comments explain why, not what.** Don't comment obvious code. Do comment non-obvious design decisions.
7. **Package names are short, lowercase, singular.** `pty`, `recorder`, `keystroke`, `gif`, `record`.
8. **Exported names have doc comments.** Every exported function, type, and const gets a `// Name does X.` comment.
9. **Keep functions short.** If a function exceeds ~50 lines, extract helpers.
10. **No dead code.** Don't commit commented-out code or unused functions.

### Platform-Specific Code

Use build-tag-separated files:

```
session_unix.go      //go:build unix
session_windows.go   //go:build windows
```

The `session.go` file defines the interface; platform files implement it. Tests that need a real PTY use `//go:build !windows` or run on all platforms with appropriate skips.

### Dependencies

Minimize external dependencies. Current expected deps:

| Package | Purpose |
|---------|---------|
| `github.com/creack/pty` | Unix PTY |
| `github.com/UserExistsError/conpty` | Windows ConPTY |
| `github.com/spf13/cobra` | CLI framework |
| (stdlib) | Everything else |

Add a dependency only when the stdlib alternative is significantly worse. Document the reason in the PR.

## Testing

### Unit Tests

Every package has `_test.go` files. Use table-driven tests:

```go
func TestKeyMap(t *testing.T) {
    tests := []struct {
        name string
        input string
        want string
    }{
        {"enter", "Enter", "\r"},
        {"escape", "Escape", "\x1b"},
        {"ctrl+c", "Ctrl+C", "\x03"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Resolve(tt.input)
            if got != tt.want {
                t.Errorf("Resolve(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

### Integration Tests

Spawning a real PTY/ConPTY or an in-repo helper process is **not** by
itself integration — if the test is fast and self-contained it stays an
untagged unit test (see constitution R5). The `integration` tag is
reserved for tests that require `agg` or exercise the full cast→GIF
pipeline, plus any test driving an external target app:

```go
//go:build integration

package record_test
```

Run them explicitly: `go test -tags integration ./...`

Integration tests are allowed to be slow and to require `agg` on PATH. Unit tests must be fast and self-contained (but may spawn an in-repo PTY child).

### CI Matrix

Tests run on: `ubuntu-latest`, `macos-latest`, `windows-latest`.

## Non-Goals (v1)

Don't accidentally pursue these:

- AI/Claude navigation (v2 feature)
- Hosted service / API / queue / cloud storage (future)
- GitHub Actions reusable action
- Web UI
- Docker images
- Binary upload / GitHub repo source resolution
- xterm.js / headless VT parsing
- Video output (MP4, WebM) — GIF only

## Open Decisions

See `spec.md`  "Key Risks" for known unknowns. Key open questions:

1. ~~Windows ConPTY library choice~~ — **Resolved:** `github.com/UserExistsError/conpty` (the earlier-referenced `iamacarpet/go-conpty` does not exist). Proven by the Phase 1 spike (PR #3). Windows is folded back into Phase 1.
2. ~~Whether to bundle `agg` or require it~~ — **Resolved:** require it; pin `agg v1.5.0` (see `spec.md` Decisions). Bundling deferred post-v1.
3. Mouse event encoding format (currently `click:col:row` from prototype)

If a task touches one of these, surface the decision rather than picking unilaterally.

## Constitution

See [specs/constitution.md](specs/constitution.md) for the project's governing rules and tenets. That document supersedes this one on any conflict.
