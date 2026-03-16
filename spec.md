# TUIcast: Technical Implementation Spec & Plan

*Version 0.1 | March 2026 | Tig Kindel | DRAFT*

-----

## 1. Overview

TUIcast accepts a Terminal.Gui application binary and a natural language goal, runs the app in a headless PTY environment, injects keystrokes and mouse events (scripted or AI-driven), records the session, and delivers an animated GIF. Primary use case: tweet-ready demo GIFs for Terminal.Gui features without manual screen recording.

-----

## 2. Architecture

### 2.1 Component Overview

|Component                |Role                            |Notes                                    |
|-------------------------|--------------------------------|-----------------------------------------|
|node-pty                 |PTY host / stdin writer         |MIT; Node.js bindings to OS PTY          |
|xterm.js 5.x             |Terminal emulator / VT processor|MIT; headless-capable; Kitty kbd protocol|
|asciinema                |Session recorder                |AGPL v3 – license review required        |
|agg                      |asciinema -> GIF renderer       |MIT; configurable font, theme, speed     |
|Claude API               |AI vision + keystroke generation|Anthropic; pay-per-token                 |
|Azure Container Apps     |Worker autoscale host           |Scales to zero; Service Bus trigger      |
|Azure Container Instances|Isolated job execution (alt)    |Per-second billing; simpler than ACA     |
|Azure Service Bus        |Job queue                       |Standard tier sufficient                 |
|Azure Blob Storage       |GIF output store                |                                         |
|Azure Container Registry |Docker image store              |                                         |

### 2.2 System Layers

**API Layer (Azure Container Apps)**

- Thin HTTP service; accepts job submissions and serves status/results
- Enqueues jobs to Azure Service Bus
- Returns `job_id` immediately; client polls for completion

**Worker Layer (Azure Container Apps, Service Bus trigger)**

- Pulls jobs from Service Bus queue
- Manages timeout, retry, and error reporting
- Uploads GIF to Azure Blob Storage on completion

**Recording Container (Docker, Ubuntu 24.04)**

- Contains: .NET runtime, node-pty, xterm.js headless, asciinema, agg
- Spawned per job; ephemeral; no network egress
- Runs the target app in a PTY, injects inputs, records output

### 2.3 Data Flow

1. Client POSTs `{ app_ref, goal, options }` to `/jobs`
1. API enqueues job to Service Bus; returns `{ job_id }`
1. Worker dequeues message; pulls app image from ACR if needed
1. Worker spawns recording container with job params as env vars
1. Container: launches app in PTY; waits for readiness signal
1. Container: executes keystroke plan (scripted) or AI vision loop
1. Container: asciinema records full PTY session
1. Container: agg converts recording to GIF
1. Worker: uploads GIF to Blob Storage; updates job status
1. Client polls `GET /jobs/:id`; receives `gif_url` on completion

### 2.4 Input Modes

#### Scripted Mode (Phase 3)

Claude API is called once upfront with the goal and app documentation. It returns a keystroke JSON array. The worker executes this deterministically.

Keystroke schema:

```json
{
  "actions": [
    { "type": "key",   "value": "Tab",    "delay_ms": 500 },
    { "type": "key",   "value": "Enter",  "delay_ms": 300 },
    { "type": "mouse", "x": 10, "y": 5,  "button": "left", "delay_ms": 200 },
    { "type": "wait",  "pattern": ".*Menu.*", "timeout_ms": 2000 }
  ]
}
```

The `wait` action blocks until PTY output matches a regex or times out. This handles Terminal.Gui render timing without hardcoded sleeps.

#### AI Vision Loop (Phase 4)

After each action, the current PTY buffer is captured as text and optionally rendered to a PNG via xterm.js canvas. This is sent to Claude with the goal and action history. Claude returns the next action. The loop continues until the goal is achieved or max iterations is reached.

Slower than scripted mode (one Claude API round-trip per action) but handles dynamic app state, timing variability, and novel UI patterns without manual scripting.

-----

## 3. API Contract

### 3.1 Endpoints

|Endpoint              |Purpose             |Request                     |Response                           |
|----------------------|--------------------|----------------------------|-----------------------------------|
|`POST /jobs`          |Submit recording job|`{ app_ref, goal, options }`|`{ job_id }`                       |
|`GET /jobs/:id`       |Poll job status     |—                           |`{ status, gif_url, error }`       |
|`GET /jobs/:id/log`   |Fetch action log    |—                           |Array of `{ action, screen_state }`|
|`POST /jobs/:id/retry`|Re-run with new goal|`{ goal }`                  |`{ job_id }`                       |

### 3.2 Job Request Schema

```json
{
  "app_ref": "tuicast.azurecr.io/demo-app:latest",
  "goal": "Show how popover menus open and navigate",
  "options": {
    "width":  220,
    "height": 50,
    "font":   "JetBrainsMono",
    "theme":  "monokai",
    "speed":  1.0,
    "mode":   "scripted"
  }
}
```

### 3.3 Job Status Response

```json
{
  "job_id":     "abc123",
  "status":     "complete",
  "gif_url":    "https://tuicast.blob.core.windows.net/gifs/abc123.gif",
  "expires_at": "2026-04-16T00:00:00Z",
  "error":      null
}
```

Status values: `queued | running | complete | failed`

-----

## 4. Infrastructure

### 4.1 Azure Resources

|Resource                    |Purpose                                             |
|----------------------------|----------------------------------------------------|
|Azure Container Registry    |Stores recording image and user app images          |
|Azure Container Apps        |Hosts API service and worker service                |
|Azure Service Bus (Standard)|Jobs queue; 1hr TTL; dead-letter queue for failures |
|Azure Blob Storage          |GIF output; 30-day lifecycle policy; SAS URLs       |
|Azure Key Vault             |API keys and connection strings via managed identity|

### 4.2 Recording Container

Base image: `ubuntu:24.04`. Added layers:

- `dotnet-runtime-8.0`
- `nodejs 20.x` + `node-pty`
- `asciinema` (pip; AGPL – see risks)
- `agg` (GitHub releases binary)
- JetBrainsMono Nerd Font
- `recorder.js` – orchestration entrypoint

`recorder.js` receives job params as env vars, runs the app in a PTY, injects inputs, drives asciinema, and invokes agg.

### 4.3 Environment Variables

|Variable                |Purpose                                   |
|------------------------|------------------------------------------|
|`ANTHROPIC_API_KEY`     |Claude API credentials                    |
|`ACR_IMAGE`             |e.g. `tuicast.azurecr.io/recorder:latest` |
|`SERVICE_BUS_CONNECTION`|Azure Service Bus connection string       |
|`BLOB_ACCOUNT_URL`      |Azure Blob Storage endpoint               |
|`BLOB_CONTAINER`        |Output GIF container name                 |
|`GIF_FONT`              |Font name for agg (default: JetBrainsMono)|
|`GIF_THEME`             |agg theme (default: monokai)              |
|`MAX_JOB_SECONDS`       |Recording timeout (default: 60)           |

### 4.4 Networking

Recording containers have no public egress. They communicate only with Service Bus (completion events) and Blob Storage (GIF upload). The app under test must be self-contained.

-----

## 5. Implementation Plan

|Phase|Duration|Scope                                                           |Exit Criteria                        |
|-----|--------|----------------------------------------------------------------|-------------------------------------|
|1    |2 weeks |PTY + record + GIF pipeline; scripted input only; local Docker  |Runnable end-to-end; no AI; no Azure |
|2    |1 week  |Azure deployment: ACA + Service Bus + Blob + HTTP API           |Cloud-hosted; POST job -> GIF URL    |
|3    |2 weeks |Claude API integration; prompt -> keystroke JSON                |AI-driven demos from natural language|
|4    |2 weeks |AI vision feedback loop; screen capture -> Claude -> next action|Adaptive navigation                  |
|5    |1 week  |Polish: GIF config, retry, logging, error handling              |Production-ready                     |

### Phase 1: Local Pipeline

Prove the core pipeline works end-to-end before touching Azure.

- Set up Docker image with all dependencies
- Write `recorder.js`: spawn app in PTY, inject hardcoded keystroke sequence, record with asciinema, convert with agg
- Test against Terminal.Gui demo app
- Validate GIF quality, font rendering, timing
- Establish baseline keystroke schema and wait-pattern matching

### Phase 2: Azure Deployment

- Provision ACR, ACA, Service Bus, Blob Storage via Bicep
- Simple Express API: `POST /jobs` enqueues; `GET /jobs/:id` reads status from Blob metadata
- Worker Container App: pulls from Service Bus, runs recording container, uploads result
- Wire managed identity to Key Vault
- Smoke test end-to-end

### Phase 3: Scripted AI Mode

- Write Claude prompt: goal + app docs + keystroke schema + JSON-only instruction
- Parse and validate output; retry on schema violation
- Execute keystroke plan with wait-pattern support
- Test against a suite of Terminal.Gui demo scenarios
- Instrument success rate; define acceptable threshold

### Phase 4: AI Vision Loop

- After each action, capture PTY text buffer; optionally render xterm.js canvas to PNG
- Send screen state + action history + goal to Claude; receive next action
- Implement loop controller: max iterations, goal-achieved detection, timeout
- Benchmark vision loop vs scripted mode on fixed scenarios
- Default to scripted for known demos; vision for novel interactions

### Phase 5: Polish

- GIF configuration exposed in API (font, theme, speed, terminal size)
- Retry endpoint with new goal
- Action log endpoint for debugging AI decisions
- Dead-letter queue monitoring and alerting
- Blob lifecycle policy (30-day auto-delete)
- README and usage examples for Terminal.Gui contributors

-----

## 6. Risks

|Risk                      |Likelihood|Impact|Mitigation                                                            |
|--------------------------|----------|------|----------------------------------------------------------------------|
|AI navigation unreliable  |High      |Medium|Ship scripted mode first; AI is additive. Conservative delays + retry.|
|asciinema AGPL license    |Low       |High  |Evaluate before Phase 2. ttyd or custom PTY recorder as fallback.     |
|ACI cold start latency    |Medium    |Low   |Pre-warmed Container App worker for interactive use; ACI for batch.   |
|Terminal.Gui render timing|Medium    |Medium|Wait-pattern matching on PTY output before injecting next input.      |
|GIF size / quality        |Low       |Low   |agg defaults are good; terminal size and fps are tunable.             |

-----

## 7. Open Questions

1. **License:** Does asciinema's AGPL require open-sourcing TUIcast? Resolve before Phase 2.
1. **App delivery:** ACR image ref works for maintainers. Binary upload endpoint deferred until broader user base.
1. **Multi-user isolation:** Current design is single-tenant. Job isolation and billing attribution not yet designed.
1. **GIF hosting duration:** 30 days assumed. Should be configurable.
1. **Windows support:** Terminal.Gui runs on Windows; recording container is Linux-only. Windows path (conpty) is out of scope.

-----

## 8. External Dependencies

|Dependency  |License  |Notes                                            |
|------------|---------|-------------------------------------------------|
|node-pty    |MIT      |Node 18+; binds to OS PTY                        |
|xterm.js 5.x|MIT      |Kitty keyboard protocol; headless via node-canvas|
|asciinema   |AGPL v3  |License review required                          |
|agg         |MIT      |Standalone binary; requires font install         |
|Claude API  |Anthropic|Pay-per-token; add retry with exponential backoff|
