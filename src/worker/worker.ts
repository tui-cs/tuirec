import * as fs from "fs";
import * as path from "path";
import * as os from "os";
import { execSync } from "child_process";

import { JobMessage, Job, JobStatus, ActionLog } from "../types";
import { PtySession, keyToAnsi } from "./pty-session";
import { ClaudeDriver } from "./claude-driver";
import { AsciinemaRecorder } from "./recorder";
import { renderGif } from "./gif-renderer";
import { uploadGif, uploadCast, downloadBinary } from "../storage/blob-storage";
import { startWorkerReceiver, closeQueue } from "../queue/service-bus";

// ---------------------------------------------------------------------------
// In-process job store (replace with Redis / CosmosDB for multi-worker setups)
// ---------------------------------------------------------------------------
const jobStore = new Map<string, Job>();

export function getJob(id: string): Job | undefined {
  return jobStore.get(id);
}

export function listJobs(): Job[] {
  return [...jobStore.values()].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  );
}

export function upsertJob(job: Job): void {
  jobStore.set(job.id, job);
}

function updateJobStatus(
  id: string,
  status: JobStatus,
  extra: Partial<Job> = {}
): void {
  const job = jobStore.get(id);
  if (!job) return;
  jobStore.set(id, { ...job, status, updatedAt: new Date().toISOString(), ...extra });
}

// ---------------------------------------------------------------------------
// Job processor
// ---------------------------------------------------------------------------

/**
 * Process a single TUIcast job end-to-end:
 *   1. Download / locate the app binary.
 *   2. Start PTY session + asciinema recorder.
 *   3. Drive the session (AI or scripted).
 *   4. Stop recording, render GIF.
 *   5. Upload artifacts and update job state.
 */
export async function processJob(message: JobMessage): Promise<void> {
  const { jobId, spec, binaryBlobName } = message;

  console.log(`[worker] Starting job ${jobId}`);
  updateJobStatus(jobId, "running");

  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), `tuicast-${jobId}-`));
  const castPath = path.join(workDir, "recording.cast");
  const gifPath = path.join(workDir, "recording.gif");

  try {
    // 1. Resolve the binary to run.
    const binaryPath = await resolveBinary(spec.source, binaryBlobName, workDir);

    const gifConfig = spec.gifConfig ?? {};
    const cols = gifConfig.cols ?? 120;
    const rows = gifConfig.rows ?? 30;
    const maxDuration = Math.min(spec.maxDurationSeconds ?? 60, 60);

    // 2. Start PTY session.
    const session = new PtySession({
      command: binaryPath,
      gifConfig,
      cwd: workDir,
    });

    // 3. Start asciinema recorder.
    const recorder = new AsciinemaRecorder({
      outputPath: castPath,
      cols,
      rows,
      title: `TUIcast – ${spec.goal}`,
    });

    // Screen buffer (simple concatenation; a real implementation would use
    // an xterm.js headless parser to maintain the 2-D screen model).
    let screenBuffer = "";
    let sessionEnded = false;

    recorder.start();
    session.start();

    session.on("data", (chunk: string) => {
      recorder.write(chunk);
      // Keep a rolling window of the last 8 KB of output as the "screen".
      screenBuffer = (screenBuffer + chunk).slice(-8192);
    });

    session.on("exit", () => {
      sessionEnded = true;
    });

    // 4. Drive session.
    const maxDurationMs = maxDuration * 1000;
    const deadline = Date.now() + maxDurationMs;
    let actions: ActionLog[] = [];

    if (spec.keystrokes && spec.keystrokes.length > 0) {
      // Scripted playback mode.
      actions = await runScriptedPlayback(session, spec.keystrokes, deadline);
    } else {
      // AI vision-loop mode.
      const driver = new ClaudeDriver({
        goal: spec.goal,
        onKeystroke: (sequence: string) => {
          if (!sessionEnded) session.write(sequence);
        },
        readScreen: () => screenBuffer,
        wait: (ms: number) =>
          new Promise<void>((resolve) =>
            setTimeout(resolve, Math.min(ms, Math.max(0, deadline - Date.now())))
          ),
      });
      actions = await driver.run();
    }

    // Allow a brief settle period after the last action.
    if (!sessionEnded) {
      await sleep(500);
    }

    // 5. Stop recording and terminate PTY.
    await recorder.stop();
    session.kill();

    // 6. Render GIF.
    await renderGif({ castPath, outputPath: gifPath, gifConfig: spec.gifConfig });

    // 7. Upload artifacts.
    const gifData = fs.readFileSync(gifPath);
    const castData = fs.readFileSync(castPath);

    const [gifUrl, castUrl] = await Promise.all([
      uploadGif(jobId, gifData),
      uploadCast(jobId, castData),
    ]);

    updateJobStatus(jobId, "completed", { gifUrl, castUrl, actions });
    console.log(`[worker] Job ${jobId} completed. GIF: ${gifUrl}`);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    console.error(`[worker] Job ${jobId} failed:`, message);
    updateJobStatus(jobId, "failed", { error: message });
  } finally {
    // Clean up temporary files.
    try {
      fs.rmSync(workDir, { recursive: true, force: true });
    } catch {
      // Ignore cleanup errors.
    }
  }
}

// ---------------------------------------------------------------------------
// Binary resolution
// ---------------------------------------------------------------------------

async function resolveBinary(
  source: JobMessage["spec"]["source"],
  binaryBlobName: string | undefined,
  workDir: string
): Promise<string> {
  if (source.binaryPath && binaryBlobName) {
    // Binary was uploaded via the API and stored in blob storage.
    const localBin = path.join(workDir, "app");
    await downloadBinary(binaryBlobName, localBin);
    fs.chmodSync(localBin, 0o755);
    return localBin;
  }

  if (source.githubRepo) {
    // Clone the repo and build (assumes dotnet publish).
    const repoDir = path.join(workDir, "repo");
    const ref = source.githubRef ?? "main";
    execSync(
      `git clone --depth 1 --branch ${ref} ${source.githubRepo} ${repoDir}`,
      { stdio: "inherit" }
    );
    execSync("dotnet publish -c Release -o ./publish", {
      cwd: repoDir,
      stdio: "inherit",
    });
    const publishDir = path.join(repoDir, "publish");
    // Find the first executable.
    const exes = fs
      .readdirSync(publishDir)
      .filter((f) => !f.endsWith(".dll") && !f.endsWith(".pdb"));
    if (exes.length === 0) {
      throw new Error("dotnet publish did not produce an executable.");
    }
    const exePath = path.join(publishDir, exes[0]);
    fs.chmodSync(exePath, 0o755);
    return exePath;
  }

  if (source.dockerImage) {
    // Docker image mode: we assume the worker itself IS the container image.
    // The command is extracted from the image's CMD/ENTRYPOINT via docker inspect.
    const result = execSync(
      `docker inspect --format='{{json .Config.Cmd}}' ${source.dockerImage}`,
      { encoding: "utf8" }
    ).trim();
    const cmd: string[] = JSON.parse(result);
    return cmd[0];
  }

  throw new Error("AppSource must specify binaryPath, githubRepo, or dockerImage.");
}

// ---------------------------------------------------------------------------
// Scripted playback
// ---------------------------------------------------------------------------

async function runScriptedPlayback(
  session: PtySession,
  keystrokes: string[],
  deadline: number
): Promise<ActionLog[]> {
  const actions: ActionLog[] = [];
  for (const key of keystrokes) {
    if (Date.now() >= deadline) {
      actions.push({
        timestamp: new Date().toISOString(),
        type: "timeout",
        description: "Deadline reached during scripted playback.",
      });
      break;
    }
    const sequence = keyToAnsi(key);
    session.write(sequence);
    actions.push({
      timestamp: new Date().toISOString(),
      type: "keystroke",
      description: `Scripted: ${key}`,
      raw: sequence,
    });
    await sleep(150);
  }
  return actions;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// ---------------------------------------------------------------------------
// Entry point (standalone worker process)
// ---------------------------------------------------------------------------

if (require.main === module) {
  console.log("[worker] TUIcast worker starting…");

  const stop = startWorkerReceiver(async (msg: JobMessage) => {
    await processJob(msg);
  });

  process.on("SIGTERM", async () => {
    console.log("[worker] Shutting down…");
    await stop();
    await closeQueue();
    process.exit(0);
  });

  process.on("SIGINT", async () => {
    console.log("[worker] Interrupted – shutting down…");
    await stop();
    await closeQueue();
    process.exit(0);
  });
}
