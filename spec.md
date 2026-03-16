# TUIcast: Technical Specification

*Version 0.2 | March 2026 | DRAFT*

---

## 1. Overview

TUIcast accepts a Terminal.Gui application binary and a plain-English goal, runs the app in a headless PTY, injects keystrokes (scripted or AI-driven), records the session, and delivers an animated GIF. Primary use case: demo GIFs for Terminal.Gui features without manual screen recording or scripting.

---

## 2. Architecture

### 2.1 Component Map

| Component | Role | Notes |
|---|---|---|
| node-pty | PTY host / stdin writer | MIT; Node.js bindings to OS PTY |
| asciinema v2 (native) | Session recording format | Written natively — no binary or AGPL library linked (see §6) |
| agg | asciinema cast → animated GIF | MIT; configurable font, theme, speed |
| Claude API | AI screen observation + keystroke generation | Anthropic; pay-per-token |
| Azure Service Bus | Job queue | Standard tier; dead-letter after 5 attempts |
| Azure Blob Storage | GIF / cast artifact store | Presigned SAS URLs; 30-day lifecycle |
| Azure Container Instances | Per-job isolated worker | Per-second billing; ephemeral; simpler than ACA |

> **xterm.js is not used.** The original spec listed it for headless terminal rendering. In practice, the Claude driver reads raw PTY output directly (see §3.3). xterm.js may be revisited if richer screen-state parsing is needed.

### 2.2 System Layers

**API Layer** — Thin Express HTTP service. Accepts job submissions; enqueues to Service Bus (or processes inline in dev mode). Returns `job_id` immediately; client polls for completion.

**Worker Layer** — Pulls jobs from Service Bus. Manages timeout, retry, and error reporting. Uploads GIF and cast to Blob Storage on completion. Runs inline (`USE_QUEUE=false`) for local development.

**Recording Pipeline** — Contained within the worker process (or ACI container):
1. Resolve the app binary (blob download / git clone + dotnet publish / Docker image).
2. Spawn the app in a PTY via `node-pty`.
3. Capture all PTY output into an asciinema v2 cast file (native implementation; no external binary).
4. Drive keystrokes via scripted script or Claude AI vision loop.
5. Invoke `agg` to convert the cast to an animated GIF.
6. Upload both artifacts to Blob Storage; update job status.

### 2.3 Data Flow

```
Client POSTs { source, goal, options }  →  POST /jobs
API enqueues job to Service Bus          →  { job_id }
Worker dequeues message
  ├─ Resolves binary (blob / clone / docker)
  ├─ Starts PTY session (node-pty)
  ├─ Starts asciinema recorder (native Node.js)
  ├─ Drives session (scripted or AI loop)
  ├─ Stops recorder; terminates PTY
  ├─ Renders GIF (agg)
  └─ Uploads GIF + cast → Blob Storage
Client polls GET /jobs/:id  →  { status, gif_url }
```

---

## 3. Core Pipeline

### 3.1 PTY Session (`src/worker/pty-session.ts`)

Wraps `node-pty`. Spawns the app with:

```
TERM=xterm-256color
COLORTERM=truecolor
COLUMNS=<cols>
LINES=<rows>
```

Exposes `write(data)` for keystroke injection and emits `data` / `exit` events. Terminal dimensions default to **120 × 30**; configurable via `GifConfig`.

### 3.2 asciinema Recorder (`src/worker/recorder.ts`)

Writes the [asciinema v2 cast format](https://docs.asciinema.org/manual/asciicast/v2/) natively. No external binary or library required. Each PTY `data` event is appended as a JSON event line:

```
[<elapsed_seconds>, "o", "<raw_output>"]
```

The header contains width, height, timestamp, and title.

### 3.3 Claude AI Driver (`src/worker/claude-driver.ts`)

Operates in a turn-based loop (max 30 turns by default; `CLAUDE_MAX_TURNS` env var):

1. Read the current "screen" — the last 8 KB of raw PTY output.
2. Send it to Claude with the goal and full action history.
3. Parse the response for a single structured action (`ACTION: KEY ...`, `ACTION: TYPE ...`, `ACTION: WAIT ...`, `ACTION: DONE`).
4. Execute the action and repeat.

**Known limitation:** The screen buffer contains raw ANSI escape sequences. Claude handles these reasonably well but a proper headless VT parser (e.g. xterm.js headless) would produce cleaner screen text. This is a planned improvement.

When a `keystrokes` array is provided in the job spec the AI loop is skipped entirely and the scripted sequence is replayed deterministically.

### 3.4 Key Map (`src/utils/keys.ts`)

Maps human-readable key names to ANSI escape sequences. Shared by `PtySession`, `ClaudeDriver`, the standalone CLI, and the worker scripted-playback path.

**Known gap:** `Ctrl+Q` (`\x11`) is absent from the map. Any keystroke not found in the map is passed to the PTY as-is (literal characters). The POC workflow uses `Ctrl+Q` as the quit key but relies on the `--max-duration` timeout for clean teardown because the literal string `"Ctrl+Q"` does not quit UICatalog. Fix: add `"Ctrl+Q": "\x11"` (and any other missing control codes) to the map.

### 3.5 GIF Renderer (`src/worker/gif-renderer.ts`)

Invokes `agg` (static binary; resolved via `AGG_PATH` env var or `PATH`). The `--font-family` flag is **omitted when no font is configured**, allowing `agg` to fall back to its built-in bitmap font. This is essential in CI environments where no system fonts are installed.

Configurable via `GifConfig`: `cols`, `rows`, `font`, `fontSize`, `theme`, `speed`.

---

## 4. Input Modes

### 4.1 Scripted Mode

Provide a `keystrokes` array (job spec) or a `--keystrokes` CSV string (CLI). Each element is either:

- `wait:<ms>` — pause for the given number of milliseconds.
- A named key (`Enter`, `Tab`, `Escape`, `ArrowUp`, `ArrowDown`, `F1`–`F10`, `Ctrl+C`, etc.).
- A literal string to type.

The CLI format uses comma separation:

```
"wait:3000,ArrowDown,wait:500,ArrowDown,wait:1000,Ctrl+Q"
```

The `wait:` prefix is handled before key lookup, so any delay can be interleaved between keystrokes.

### 4.2 AI Vision Loop

When no keystroke script is provided, the Claude driver iterates until it reports `ACTION: DONE` or the turn limit is reached. Claude is given:

- The goal (plain English).
- The current terminal screen (raw PTY text, last 8 KB).
- The history of actions taken so far.

Claude responds with a single action per turn. The default inter-action pause is 200 ms; `ACTION: WAIT <ms>` overrides this.

---

## 5. API Contract

### 5.1 Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/jobs` | POST | Submit a recording job |
| `/jobs` | GET | List all jobs (most recent first) |
| `/jobs/:id` | GET | Poll job status |
| `/health` | GET | Health check |

Rate limits: 100 req/min (general), 10 req/min (job submission), per IP.

### 5.2 Job Request Schema

```json
{
  "goal": "Show how the File Open dialog navigates folders",
  "source": {
    "githubRepo": "https://github.com/gui-cs/Terminal.Gui",
    "githubRef": "v2_develop"
  },
  "gifConfig": {
    "cols": 132,
    "rows": 40,
    "theme": "monokai",
    "speed": 1.0
  },
  "maxDurationSeconds": 30,
  "keystrokes": ["wait:3000", "ArrowDown", "ArrowDown", "Ctrl+Q"]
}
```

App sources are mutually exclusive: `source.githubRepo`, `source.dockerImage`, or binary upload (`multipart/form-data`).

### 5.3 Job Status Response

```json
{
  "id": "abc123",
  "status": "completed",
  "spec": { "..." : "..." },
  "createdAt": "2026-03-16T00:00:00Z",
  "updatedAt": "2026-03-16T00:01:30Z",
  "gifUrl": "https://tuicast.blob.core.windows.net/gifs/abc123.gif?sv=...",
  "castUrl": "https://tuicast.blob.core.windows.net/casts/abc123.cast?sv=...",
  "error": null,
  "actions": [...]
}
```

Status values: `queued | running | completed | failed`

---

## 6. Standalone Recorder CLI

`src/cli/record.ts` — CI-friendly recorder that requires no Azure or Anthropic credentials.

```bash
# Record via npm script (TypeScript, no build step)
npm run record -- \
  --binary      /path/to/UICatalog \
  --keystrokes  "wait:3000,ArrowDown,wait:500,ArrowDown,wait:1000,Ctrl+Q" \
  --output      demo.gif \
  --cast-output demo.cast \
  --cols        132 \
  --rows        40 \
  --theme       monokai \
  --max-duration 20

# Options
#   --binary <path>         Path to executable (required)
#   --args <csv>            Arguments to pass to the binary
#   --output <path>         Output GIF path (default: recording.gif)
#   --cast-output <path>    Also save the raw cast file
#   --keystrokes <csv>      Keystroke sequence (default: "wait:3000,Ctrl+Q")
#   --keystroke-delay <ms>  Pause between keystrokes (default: 200)
#   --cols <n>              Terminal columns (default: 120)
#   --rows <n>              Terminal rows (default: 30)
#   --theme <name>          agg color theme (default: monokai)
#   --font <name>           Font family; omit to use agg's built-in bitmap font
#   --font-size <n>         Font size in px (default: 14)
#   --speed <n>             GIF playback speed multiplier (default: 1.0)
#   --max-duration <n>      Max recording seconds; capped at 60 (default: 60)
#   --title <text>          Title embedded in the cast file
```

The CLI verifies the output GIF's magic bytes (`GIF89a`) and exits non-zero on failure, making it suitable for CI assertions.

---

## 7. POC GitHub Actions Workflow

`.github/workflows/poc-uicatalog.yml` — fully self-contained; no Azure or Anthropic credentials required.

```yaml
on:
  workflow_dispatch:
  push:
    branches: [copilot/add-tui-gif-generation-feature]
```

Steps:
1. Checkout TUIcast.
2. Install .NET 10 (`actions/setup-dotnet@v4`, version `10.0.x`).
3. Clone Terminal.Gui (`--branch v2_develop`) and publish UICatalog.
4. Install Node.js 20 and native build tools (`python3 make g++` for node-pty).
5. `npm ci`.
6. Download `agg` v1.5.0 static binary from GitHub releases.
7. `npm run record` with a deterministic keystroke script.
8. Assert GIF existence, size ≥ 1 KB, and valid `GIF89a` magic bytes.
9. Upload GIF and cast as workflow artifacts.

---

## 8. Infrastructure

### 8.1 Azure Resources

| Resource | Purpose |
|---|---|
| Azure Container Instances | Per-job isolated worker containers |
| Azure Service Bus (Standard) | Job queue; dead-letter after 5 attempts; 1-hr TTL |
| Azure Blob Storage | GIF + cast output; 30-day lifecycle; SAS presigned URLs |
| Azure Container Registry | Stores API and worker Docker images |

Bicep templates: `infra/service-bus.bicep`, `infra/storage.bicep`, `infra/container-instance.bicep`.

### 8.2 Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `ANTHROPIC_API_KEY` | — | Claude API credentials |
| `CLAUDE_MODEL` | `claude-opus-4-5` | Claude model |
| `CLAUDE_MAX_TURNS` | `30` | Max AI navigation turns per job |
| `SERVICE_BUS_CONNECTION_STRING` | — | Azure Service Bus connection string |
| `SERVICE_BUS_QUEUE_NAME` | `tuicast-jobs` | Queue name |
| `USE_QUEUE` | `false` | `true` = enqueue; `false` = process inline |
| `STORAGE_CONNECTION_STRING` | `UseDevelopmentStorage=true` | Blob Storage connection string |
| `STORAGE_CONTAINER_GIFS` | `gifs` | Blob container for GIFs |
| `STORAGE_CONTAINER_CASTS` | `casts` | Blob container for cast files |
| `STORAGE_CONTAINER_BINARIES` | `binaries` | Blob container for uploaded binaries |
| `STORAGE_SAS_TTL_SECONDS` | `3600` | Presigned URL TTL |
| `AGG_PATH` | `agg` | Path to the `agg` binary |
| `PORT` | `3000` | API server port |

### 8.3 Local Development

```bash
# Start Azurite (blob storage emulator) and run the API inline
docker compose up azurite -d
npm run dev:api        # API + inline worker on http://localhost:3000
```

---

## 9. Implementation Phases

| Phase | Status | Scope |
|---|---|---|
| 1 | ✅ Done | PTY + native asciinema recorder + agg GIF pipeline; scripted input; no cloud |
| 2 | ✅ Done | Express API; Azure Service Bus + Blob Storage; inline + queue worker modes |
| 3 | ✅ Done | Claude AI driver; text-based screen observation; scripted fallback |
| 4 | ✅ Done | Standalone CLI (`src/cli/record.ts`); POC GitHub Actions workflow |
| 5 | 🔲 Next | Fix key map gaps (`Ctrl+Q` et al.); headless VT parser for cleaner screen text |
| 6 | 🔲 Next | Azure Container Instance deployment; managed identity; Key Vault integration |
| 7 | 🔲 Next | Multi-user isolation; billing attribution; retry endpoint; action-log endpoint |

---

## 10. Known Issues & Lessons Learned

The following issues were discovered during POC implementation and should be resolved before a wider rollout.

### 10.1 `Ctrl+Q` (and other control codes) missing from the key map

`src/utils/keys.ts` maps `Ctrl+C`, `Ctrl+D`, `Ctrl+Z`, `Ctrl+L`, `Ctrl+A`, `Ctrl+E` but **not** `Ctrl+Q` (`\x11`). Unknown keys fall through to the PTY as literal strings, so `"Ctrl+Q"` sent to the PTY is five characters, not the quit signal. In the POC this is masked by `--max-duration` forcing the session to end. Fix: add all missing Ctrl+letter codes to the map.

### 10.2 Terminal.Gui has no `main` branch

The repository uses `v2_develop` as its primary integration branch and targets `net10.0` (not `net8.0`). Any workflow or tooling that clones Terminal.Gui must use `--branch v2_develop`. The UICatalog project file is at `Examples/UICatalog/UICatalog.csproj`.

### 10.3 node-pty requires native build tools

`node-pty` is a native Node.js module. CI runners need `python3 make g++` (or `build-essential`) before `npm ci`. Without these the install fails with a node-gyp error.

### 10.4 `agg` requires no font flag in headless CI

Passing `--font-family <name>` to `agg` on a runner without the named font installed causes `agg` to error. When no font is configured, `gif-renderer.ts` intentionally omits the flag so `agg` falls back to its built-in bitmap font. When a font is desired in production, the font must be installed in the container image.

### 10.5 Screen buffer is raw PTY output, not rendered text

The Claude driver reads the last 8 KB of raw PTY output, which includes ANSI escape sequences. Claude can interpret these but a proper headless VT processor would provide cleaner screen text and accurate cursor position. Using xterm.js in headless mode (Node.js, no DOM) is the intended upgrade path.

### 10.6 asciinema AGPL — resolved

The original risk of linking the asciinema recorder binary (AGPL v3) is avoided: `src/worker/recorder.ts` implements the asciinema v2 cast format natively in Node.js. No AGPL code is linked or distributed.

---

## 11. Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| AI navigation unreliable for novel UIs | High | Medium | Ship scripted mode first; AI is additive. Conservative delays + retry. |
| Raw PTY screen text degrades AI accuracy | Medium | Medium | Upgrade to headless VT parser (xterm.js) in Phase 5. |
| Key map gaps cause silent keystroke failures | Medium | Medium | Audit and fill key map; add unit tests for all named keys. |
| ACI cold-start latency (5–15 s) | Medium | Low | Acceptable for batch use; pre-warmed worker for interactive path. |
| Terminal.Gui render timing variability | Medium | Medium | Use `wait:<ms>` in keystroke scripts; increase settle time before stop. |
| GIF size too large for social media | Low | Low | agg defaults produce acceptable sizes; terminal dimensions are tunable. |

---

## 12. External Dependencies

| Dependency | License | Notes |
|---|---|---|
| node-pty | MIT | Node 18+; requires native build tools at install time |
| agg | MIT | Static binary; no system font required with built-in fallback |
| @anthropic-ai/sdk | Commercial | Claude API; pay-per-token; add retry with exponential backoff |
| @azure/service-bus | MIT | Azure SDK for Service Bus |
| @azure/storage-blob | MIT | Azure SDK for Blob Storage |
| express | MIT | HTTP server |
| express-rate-limit | MIT | Per-route rate limiting |
| multer | MIT | Multipart binary upload |

---

## 13. Open Questions

1. **Key map completeness** — Which Ctrl+letter codes does Terminal.Gui expect? Should `Alt+*` combos be added?
2. **Screen text quality** — When does raw PTY output become insufficient for Claude? At what point is a headless VT parser justified?
3. **Multi-user isolation** — Current design is effectively single-tenant. What isolation guarantees are needed for broader access?
4. **GIF hosting duration** — 30-day lifecycle assumed. Should it be configurable per-job?
5. **Windows support** — Terminal.Gui runs on Windows (ConPTY). The recording pipeline is Linux-only. Is a Windows recording path needed?
6. **App delivery for third parties** — GitHub repo clone + `dotnet publish` works for Terminal.Gui examples. Broader binary upload or Docker image path needs documentation and validation.
