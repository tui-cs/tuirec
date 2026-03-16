#!/usr/bin/env node
/**
 * TUIcast standalone recorder CLI.
 *
 * Records a terminal session to an animated GIF without requiring any cloud
 * services (Azure, Anthropic). Designed for CI/CD pipelines and local testing.
 *
 * Usage:
 *   node dist/cli/record.js --binary <path> [options]
 *   (or via: npm run record -- --binary <path> [options])
 *
 * Options:
 *   --binary <path>         Path to the executable to record (required)
 *   --args <args>           Comma-separated arguments to pass to the binary
 *   --output <path>         Output GIF path (default: recording.gif)
 *   --cast-output <path>    Also save the asciinema cast file here
 *   --keystrokes <csv>      Comma-separated keystroke sequence, e.g.
 *                           "wait:3000,ArrowDown,ArrowDown,Enter,Ctrl+Q"
 *                           • Prefix any element with "wait:<ms>" to pause.
 *                           • Named keys: Enter, Tab, Escape, ArrowUp/Down/
 *                             Left/Right, F1-F10, Ctrl+C, Ctrl+Q, etc.
 *                           • Anything else is typed literally.
 *                           Default: "wait:3000,Ctrl+Q"
 *   --keystroke-delay <ms>  Pause between keystrokes in ms (default: 200)
 *   --cols <n>              Terminal columns (default: 120)
 *   --rows <n>              Terminal rows (default: 30)
 *   --theme <name>          agg color theme (default: monokai)
 *   --font <name>           Font family for agg; omit to use agg's built-in
 *                           bitmap font (no system font required)
 *   --font-size <n>         Font size in px (default: 14)
 *   --line-height <n>       Vertical line-height multiplier for agg (default: 1.0)
 *                           agg's built-in default is 1.4, which adds visible gaps
 *                           between rows. Use 1.0 for tight, gap-free rows.
 *   --speed <n>             GIF playback speed multiplier (default: 1.0)
 *   --max-duration <n>      Max recording seconds, capped at 60 (default: 60)
 *   --title <text>          Title embedded in the cast file
 */

import * as fs from "fs";
import * as os from "os";
import * as path from "path";

import { PtySession } from "../worker/pty-session";
import { AsciinemaRecorder } from "../worker/recorder";
import { renderGif } from "../worker/gif-renderer";
import { keyToAnsi } from "../utils/keys";
import { GifConfig } from "../types";

// ---------------------------------------------------------------------------
// Argument parsing
// ---------------------------------------------------------------------------

interface CliArgs {
  binary: string;
  binaryArgs: string[];
  output: string;
  castOutput?: string;
  keystrokes: string[];
  keystrokeDelay: number;
  cols: number;
  rows: number;
  theme: string;
  font: string;
  fontSize: number;
  lineHeight: number;
  speed: number;
  maxDuration: number;
  title: string;
}

function parseArgs(argv: string[]): CliArgs {
  const get = (flag: string): string | undefined => {
    const i = argv.indexOf(flag);
    return i >= 0 ? argv[i + 1] : undefined;
  };

  const binary = get("--binary");
  if (!binary) {
    console.error("Error: --binary <path> is required.");
    process.exit(1);
  }

  const keystrokesRaw = get("--keystrokes");
  const keystrokes = keystrokesRaw
    ? keystrokesRaw.split(",").map((k) => k.trim()).filter(Boolean)
    : ["wait:3000", "Ctrl+Q"];

  return {
    binary,
    binaryArgs: (get("--args") ?? "").split(",").map((a) => a.trim()).filter(Boolean),
    output: get("--output") ?? "recording.gif",
    castOutput: get("--cast-output"),
    keystrokes,
    keystrokeDelay: parseInt(get("--keystroke-delay") ?? "200", 10),
    cols: parseInt(get("--cols") ?? "120", 10),
    rows: parseInt(get("--rows") ?? "30", 10),
    theme: get("--theme") ?? "monokai",
    font: get("--font") ?? "",
    fontSize: parseInt(get("--font-size") ?? "14", 10),
    lineHeight: parseFloat(get("--line-height") ?? "1.0"),
    speed: parseFloat(get("--speed") ?? "1.0"),
    maxDuration: (() => {
      const requested = parseInt(get("--max-duration") ?? "60", 10);
      const capped = Math.min(requested, 60);
      if (requested > 60) {
        console.warn(`[record] --max-duration ${requested}s exceeds the 60s cap; using 60s.`);
      }
      return capped;
    })(),
    title: get("--title") ?? "TUIcast Recording",
  };
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));

  if (!fs.existsSync(args.binary)) {
    console.error(`Error: binary not found: ${args.binary}`);
    process.exit(1);
  }

  // Ensure the output directory exists.
  const outputDir = path.dirname(path.resolve(args.output));
  fs.mkdirSync(outputDir, { recursive: true });

  // Temp directory for the cast file (if no explicit cast-output path given).
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "tuicast-"));
  const castPath = args.castOutput ?? path.join(tmpDir, "recording.cast");

  const gifConfig: GifConfig = {
    cols: args.cols,
    rows: args.rows,
    theme: args.theme,
    font: args.font || undefined,
    fontSize: args.fontSize,
    lineHeight: args.lineHeight,
    speed: args.speed,
  };

  console.log(`[record] binary      : ${args.binary}`);
  console.log(`[record] terminal    : ${args.cols}x${args.rows}`);
  console.log(`[record] keystrokes  : ${args.keystrokes.join(", ")}`);
  console.log(`[record] max duration: ${args.maxDuration}s`);
  console.log(`[record] cast file   : ${castPath}`);
  console.log(`[record] gif output  : ${args.output}`);

  // ── PTY session ──────────────────────────────────────────────────────────
  const session = new PtySession({
    command: args.binary,
    args: args.binaryArgs,
    gifConfig,
    env: {
      // Suppress .NET startup messages that could clutter the recording.
      DOTNET_NOLOGO: "1",
      DOTNET_CLI_TELEMETRY_OPTOUT: "1",
    },
  });

  // ── Recorder ─────────────────────────────────────────────────────────────
  const recorder = new AsciinemaRecorder({
    outputPath: castPath,
    cols: args.cols,
    rows: args.rows,
    title: args.title,
  });

  let sessionEnded = false;
  let exitCode = 0;

  recorder.start();
  session.start();
  console.log("[record] PTY session started.");

  session.on("data", (_chunk: string) => {
    recorder.write(_chunk);
  });

  session.on("exit", (code: number) => {
    sessionEnded = true;
    exitCode = code;
    console.log(`[record] Process exited (code ${code}).`);
  });

  // ── Keystroke playback ───────────────────────────────────────────────────
  const deadline = Date.now() + args.maxDuration * 1000;

  for (const key of args.keystrokes) {
    if (sessionEnded) {
      console.log("[record] Process already exited – stopping keystroke replay.");
      break;
    }
    if (Date.now() >= deadline) {
      console.log("[record] Max duration reached – stopping keystroke replay.");
      break;
    }

    const waitMatch = key.match(/^wait:(\d+)$/i);
    if (waitMatch) {
      const ms = Math.min(parseInt(waitMatch[1], 10), remainingMs(deadline));
      if (ms > 0) {
        console.log(`[record]   wait ${ms} ms`);
        await sleep(ms);
      }
      continue;
    }

    const sequence = keyToAnsi(key);
    console.log(`[record]   send "${key}"`);
    session.write(sequence);
    if (args.keystrokeDelay > 0) {
      await sleep(Math.min(args.keystrokeDelay, remainingMs(deadline)));
    }
  }

  // Give the app a moment to react after the final keystroke.
  if (!sessionEnded) {
    const settle = Math.min(500, remainingMs(deadline));
    if (settle > 0) await sleep(settle);
  }

  // ── Stop recording ───────────────────────────────────────────────────────
  await recorder.stop();
  session.kill();

  console.log(
    `[record] Recording stopped: ${recorder.eventCount} events, ${recorder.durationSeconds.toFixed(2)}s`
  );

  if (recorder.eventCount === 0) {
    console.error(
      "[record] Error: no terminal output was captured. " +
        "Check that the binary ran correctly and the PTY session received data."
    );
    cleanUp(tmpDir, args.castOutput);
    process.exit(1);
  }

  // ── Render GIF ───────────────────────────────────────────────────────────
  console.log("[record] Rendering GIF…");
  try {
    await renderGif({ castPath, outputPath: args.output, gifConfig });
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(`[record] GIF render failed: ${msg}`);
    cleanUp(tmpDir, args.castOutput);
    process.exit(1);
  }

  const stat = fs.statSync(args.output);
  console.log(`[record] ✅ GIF saved: ${args.output} (${(stat.size / 1024).toFixed(1)} KB)`);

  // ── Verify GIF magic bytes ───────────────────────────────────────────────
  const fd = fs.openSync(args.output, "r");
  const magic = Buffer.alloc(6);
  fs.readSync(fd, magic, 0, 6, 0);
  fs.closeSync(fd);
  const magicStr = magic.toString("ascii");
  if (magicStr !== "GIF89a" && magicStr !== "GIF87a") {
    console.error(`[record] Error: output file does not appear to be a valid GIF (magic: ${magicStr})`);
    cleanUp(tmpDir, args.castOutput);
    process.exit(1);
  }
  console.log(`[record] ✅ GIF magic bytes verified: ${magicStr}`);

  cleanUp(tmpDir, args.castOutput);

  // Surface the app's exit code so CI can detect crashes.
  if (exitCode !== 0) {
    console.warn(`[record] ⚠ App exited with code ${exitCode}.`);
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Returns the number of milliseconds remaining before the deadline (≥ 0). */
function remainingMs(deadline: number): number {
  return Math.max(0, deadline - Date.now());
}

function cleanUp(tmpDir: string, castOutput: string | undefined): void {
  // Only remove the temp dir if the cast was not explicitly saved there.
  if (!castOutput) {
    try {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    } catch {
      // Ignore cleanup errors.
    }
  }
}

main().catch((err: unknown) => {
  console.error("[record] Fatal:", err instanceof Error ? err.message : err);
  process.exit(1);
});
