# TUIcast

**AI-Powered Terminal GIF Generation for Terminal.Gui Developers**

TUIcast lets Terminal.Gui developers generate polished, animated GIF recordings of their TUI apps by simply describing what they want to demonstrate. Provide a goal in plain English, point TUIcast at your app, and receive a ready-to-tweet GIF within minutes — no manual scripting or recording tool wrestling required.

---

## How It Works

```
Developer describes goal
        │
        ▼
 TUIcast API (Express)
        │  enqueues job
        ▼
 Azure Service Bus
        │  consumed by
        ▼
 Worker (Azure Container Instance)
   ├─ Spawns PTY with node-pty
   ├─ Records with asciinema
   ├─ Claude observes screen → sends keystrokes (loop)
   └─ Stops → renders GIF with agg
        │  uploads artifacts
        ▼
 Azure Blob Storage
        │  presigned URL
        ▼
 Developer downloads GIF
```

1. **Submit a job** – POST your app source (GitHub repo, Docker image, or uploaded binary) plus a plain-English goal to the API.
2. **PTY session** – A worker container spawns your app in a real PTY using [node-pty](https://github.com/microsoft/node-pty).
3. **asciinema recording** – Every byte of terminal output is captured in [asciinema v2 cast](https://docs.asciinema.org/manual/asciicast/v2/) format.
4. **Claude navigation** – [Claude](https://www.anthropic.com/claude) observes the terminal screen after each action and decides the next keystroke, iterating until the goal is complete or a timeout is reached. You can also supply a deterministic keystroke script to skip the AI loop.
5. **GIF rendering** – The cast file is rendered to an animated GIF by [agg](https://github.com/asciinema/agg) with configurable font, theme, and speed.
6. **Download** – The GIF is stored in Azure Blob Storage and a short-lived presigned URL is returned.

---

## Quick Start (Local / Development)

### Prerequisites

| Tool | Version |
|------|---------|
| Node.js | ≥ 20 |
| Docker + Compose | any recent version |
| `agg` | ≥ 1.5 — [install](https://github.com/asciinema/agg#installation) |
| Anthropic API key | [get one](https://console.anthropic.com) |

### 1. Clone & install

```bash
git clone https://github.com/gui-cs/TUIcast.git
cd TUIcast
npm install
```

### 2. Configure

```bash
cp .env.example .env
# Edit .env and set ANTHROPIC_API_KEY (everything else works out-of-the-box with defaults)
```

### 3. Start Azurite (local blob storage emulator)

```bash
docker compose up azurite -d
```

### 4. Start the API

```bash
npm run dev:api
# → API listening on http://localhost:3000
```

Open [http://localhost:3000](http://localhost:3000) in your browser to use the web UI.

### 5. (Optional) Start the background worker

When `USE_QUEUE=false` (the default in `.env.example`) the API processes jobs inline. Set `USE_QUEUE=true` and start the worker separately for production-like operation:

```bash
npm run dev:worker
```

---

## API Reference

### `POST /jobs`

Submit a new recording job.

**Content-Type:** `multipart/form-data` or `application/json`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `goal` | string | ✅ | Plain-English description of what the recording should demonstrate. |
| `source.githubRepo` | string | one of | GitHub repository URL (e.g. `https://github.com/gui-cs/Terminal.Gui`). |
| `source.githubRef` | string | | Branch, tag, or SHA. Default: `main`. |
| `source.dockerImage` | string | one of | Docker image in ACR (e.g. `myacr.azurecr.io/myapp:latest`). |
| `binary` | file | one of | Self-contained executable uploaded as multipart form data. |
| `gifConfig.cols` | number | | Terminal width in columns. Default: `120`. |
| `gifConfig.rows` | number | | Terminal height in rows. Default: `30`. |
| `gifConfig.theme` | string | | agg theme name (`monokai`, `dracula`, `solarized-dark`, `nord`, `github-dark`). Default: `monokai`. |
| `gifConfig.speed` | number | | GIF playback speed multiplier. Default: `1.0`. |
| `gifConfig.font` | string | | Font family recognised by agg. Default: `JetBrains Mono`. |
| `gifConfig.fontSize` | number | | Font size in pixels. Default: `14`. |
| `keystrokes` | string[] | | Deterministic keystroke script (one key per element). Skips the AI loop. |
| `maxDurationSeconds` | number | | Max PTY session length in seconds (≤ 60). Default: `60`. |

**Response:** `202 Accepted` — the created [`Job`](#job-object) object.

**Example (JSON):**
```bash
curl -X POST http://localhost:3000/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "goal": "Show how popover menus work",
    "source": { "githubRepo": "https://github.com/gui-cs/Terminal.Gui" },
    "gifConfig": { "theme": "dracula", "speed": 1.2 }
  }'
```

**Example (binary upload):**
```bash
curl -X POST http://localhost:3000/jobs \
  -F 'spec={"goal":"Navigate file open dialog","source":{"binaryPath":"__uploaded__"}}' \
  -F 'binary=@./MyApp'
```

---

### `GET /jobs`

List all jobs (most recent first).

**Response:** `200 OK` — array of [`Job`](#job-object) objects.

---

### `GET /jobs/:id`

Get the current state of a specific job.

**Response:** `200 OK` — [`Job`](#job-object) object.

---

### `GET /health`

Health check endpoint.

**Response:** `200 OK` — `{ "status": "ok", "timestamp": "<ISO-8601>" }`

---

### Job Object

```typescript
{
  id: string;           // UUID v4
  status: "queued" | "running" | "completed" | "failed";
  spec: JobSpec;        // Original submission
  createdAt: string;    // ISO-8601
  updatedAt: string;    // ISO-8601
  gifUrl?: string;      // Presigned download URL (completed jobs)
  castUrl?: string;     // Presigned asciinema cast URL (completed jobs)
  error?: string;       // Error message (failed jobs)
  actions?: ActionLog[]; // Per-action AI log
}
```

---

## Configuration

All configuration is via environment variables. See [`.env.example`](.env.example) for the full list.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | API server port |
| `ANTHROPIC_API_KEY` | — | **Required.** Anthropic API key |
| `CLAUDE_MODEL` | `claude-opus-4-5` | Claude model to use |
| `CLAUDE_MAX_TURNS` | `30` | Max AI navigation turns per job |
| `SERVICE_BUS_CONNECTION_STRING` | — | Azure Service Bus connection string |
| `SERVICE_BUS_QUEUE_NAME` | `tuicast-jobs` | Service Bus queue name |
| `USE_QUEUE` | `false` | `true` = enqueue jobs; `false` = process inline |
| `STORAGE_CONNECTION_STRING` | `UseDevelopmentStorage=true` | Azure Blob Storage connection string |
| `STORAGE_CONTAINER_GIFS` | `gifs` | Blob container for rendered GIFs |
| `STORAGE_CONTAINER_CASTS` | `casts` | Blob container for asciinema casts |
| `STORAGE_CONTAINER_BINARIES` | `binaries` | Blob container for uploaded binaries |
| `STORAGE_SAS_TTL_SECONDS` | `3600` | Presigned URL TTL in seconds |
| `AGG_PATH` | `agg` | Path to the `agg` binary |

---

## Production Deployment (Azure)

### 1. Provision infrastructure

```bash
# Service Bus
az deployment group create -g tuicast-rg -f infra/service-bus.bicep

# Storage
az deployment group create -g tuicast-rg -f infra/storage.bicep
```

### 2. Build and push container images

```bash
# API image
docker build -f Dockerfile.api -t myacr.azurecr.io/tuicast-api:latest .
docker push myacr.azurecr.io/tuicast-api:latest

# Worker image
docker build -f Dockerfile.worker -t myacr.azurecr.io/tuicast-worker:latest .
docker push myacr.azurecr.io/tuicast-worker:latest
```

### 3. Deploy the API

Deploy `Dockerfile.api` to Azure App Service, Azure Container Apps, or Azure Container Instances with the environment variables from `.env.example`.

### 4. Per-job worker containers

Each recording job can be executed in an isolated Azure Container Instance using the Bicep template in `infra/container-instance.bicep`. The template provisions a single-container group that reads one job from the Service Bus queue, processes it, and terminates.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          TUIcast System                             │
│                                                                     │
│  ┌──────────┐    POST /jobs    ┌──────────┐   enqueue   ┌────────┐ │
│  │ Web UI / │ ──────────────▶  │  API     │ ──────────▶ │Service │ │
│  │  CLI     │                  │(Express) │             │  Bus   │ │
│  └──────────┘                  └──────────┘             └───┬────┘ │
│                                     │                       │      │
│                                     │ inline (dev)          │      │
│                                     ▼                 consume│      │
│                               ┌──────────┐            ┌────▼────┐ │
│                               │  Worker  │◀───────────│  Worker │ │
│                               │ (inline) │            │  (ACI)  │ │
│                               └────┬─────┘            └────┬────┘ │
│                                    │                        │      │
│                               ┌────▼──────────────────────▼────┐  │
│                               │   node-pty PTY session          │  │
│                               │   + asciinema recorder          │  │
│                               │   + Claude AI driver            │  │
│                               │   + agg GIF renderer            │  │
│                               └─────────────────┬───────────────┘  │
│                                                 │                   │
│                               ┌─────────────────▼───────────────┐  │
│                               │   Azure Blob Storage             │  │
│                               │   (gifs / casts / binaries)      │  │
│                               └─────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## FAQ

**Q: What TUI frameworks does it support?**  
A: Terminal.Gui is the primary target. Any .NET console app that runs over a PTY will work. Other frameworks (Spectre.Console, etc.) are untested but likely functional.

**Q: How does the AI know what to do?**  
A: You provide a goal in plain English. Claude is given the goal, observes the terminal screen state after each action, and decides the next keystroke. It iterates until the goal appears complete or a timeout is hit. You can also provide a deterministic keystroke script if you want exact control.

**Q: What if the AI does something wrong or gets stuck?**  
A: You get a GIF either way. If the result isn't right, refine the goal description and re-run. Iteration is fast – typically under two minutes per run.

**Q: Can I control the look of the GIF?**  
A: Yes. Terminal size, font, color theme, and playback speed are all configurable. Defaults are tuned for legibility in social media embeds.

**Q: Does my app need to be publicly available?**  
A: No. You can push a Docker image to Azure Container Registry and reference it by name, or provide a self-contained binary via the upload API.

**Q: How is it hosted?**  
A: Each recording job runs in an isolated Azure Container Instance. Jobs are queued via Azure Service Bus. GIFs are stored in Azure Blob Storage and served via a short-lived presigned URL.

**Q: Is this secure?**  
A: Jobs run in isolated, ephemeral containers. Containers are destroyed after the job completes.

**Q: What are the current limitations?**  
A: Apps requiring external network calls, database connections, or complex setup beyond a single binary are not yet supported. Maximum recording duration is 60 seconds.

**Q: What does it cost?**  
A: Currently free for Terminal.Gui maintainers and contributors. Broader pricing TBD.

---

## Tech Stack

| Component | Technology | License |
|-----------|-----------|---------|
| PTY emulation | [node-pty](https://github.com/microsoft/node-pty) | MIT |
| Terminal rendering | [xterm.js](https://xtermjs.org/) (future) | MIT |
| Session recording | [asciinema](https://asciinema.org/) v2 format | AGPL-3.0* |
| GIF rendering | [agg](https://github.com/asciinema/agg) | MIT |
| AI navigation | [Anthropic Claude](https://www.anthropic.com/claude) | Commercial |
| Job queue | Azure Service Bus | Commercial |
| Artifact storage | Azure Blob Storage | Commercial |
| Container execution | Azure Container Instances | Commercial |

> *TUIcast writes asciinema v2 cast files natively without linking the asciinema recorder binary. Review your usage against the AGPL-3.0 license if you distribute a modified version.

---

## Development

```bash
npm run build        # Compile TypeScript → dist/
npm test             # Run Jest test suite
npm run dev:api      # Start API with hot-reload (ts-node-dev)
npm run dev:worker   # Start worker with hot-reload (ts-node-dev)
npm run lint         # ESLint
```

---

## License

MIT — see [LICENSE](LICENSE).
