import { spawn } from "child_process";
import { GifConfig } from "../types";

export interface RenderOptions {
  castPath: string;
  outputPath: string;
  gifConfig?: GifConfig;
}

/**
 * Render an asciinema cast file to an animated GIF using `agg`.
 *
 * `agg` must be available on the system PATH (or at AGG_PATH env var).
 * See: https://github.com/asciinema/agg
 *
 * @param options – Cast file path, output GIF path, and rendering options.
 */
export async function renderGif(options: RenderOptions): Promise<void> {
  const { castPath, outputPath, gifConfig } = options;

  const aggPath = process.env.AGG_PATH ?? "agg";
  // Use an explicit font only when one is configured; fall back to agg's
  // built-in bitmap font so the renderer works without any system fonts.
  const font = gifConfig?.font ?? "";
  const fontSize = gifConfig?.fontSize ?? 14;
  const theme = gifConfig?.theme ?? "monokai";
  const speed = gifConfig?.speed ?? 1.0;

  const args = [
    castPath,
    outputPath,
    "--font-size",
    String(fontSize),
    "--theme",
    theme,
    "--speed",
    String(speed),
  ];

  // Only pass --font-family when a font name is explicitly provided;
  // otherwise let agg use its built-in fallback font.
  if (font) {
    args.splice(2, 0, "--font-family", font);
  }

  await runCommand(aggPath, args);
}

function runCommand(command: string, args: string[]): Promise<void> {
  return new Promise((resolve, reject) => {
    const proc = spawn(command, args, { stdio: ["ignore", "pipe", "pipe"] });

    let stderr = "";
    proc.stderr?.on("data", (chunk: Buffer) => {
      stderr += chunk.toString();
    });

    proc.on("close", (code: number | null) => {
      if (code === 0) {
        resolve();
      } else {
        reject(
          new Error(
            `agg exited with code ${code}. stderr: ${stderr.trim()}`
          )
        );
      }
    });

    proc.on("error", (err: Error) => {
      reject(new Error(`Failed to spawn agg: ${err.message}`));
    });
  });
}
