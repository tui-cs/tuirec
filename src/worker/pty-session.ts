import * as pty from "node-pty";
import { EventEmitter } from "events";
import { GifConfig } from "../types";
import { keyToAnsi } from "../utils/keys";

export interface PtySessionOptions {
  /** Command to execute (absolute path or command available on PATH). */
  command: string;
  /** Arguments to pass to the command. */
  args?: string[];
  /** Working directory for the child process. */
  cwd?: string;
  /** Additional environment variables. */
  env?: NodeJS.ProcessEnv;
  gifConfig?: GifConfig;
}

/**
 * Manages a single PTY session for a TUIcast recording.
 *
 * Emits:
 *   - "data"  (chunk: string) – raw terminal output received from the PTY.
 *   - "exit"  (code: number)  – process exited.
 *   - "error" (err: Error)    – unrecoverable error.
 */
export class PtySession extends EventEmitter {
  private _pty: pty.IPty | null = null;
  private readonly _cols: number;
  private readonly _rows: number;

  constructor(private readonly options: PtySessionOptions) {
    super();
    this._cols = options.gifConfig?.cols ?? 120;
    this._rows = options.gifConfig?.rows ?? 30;
  }

  /** Start the PTY process. */
  start(): void {
    const env: NodeJS.ProcessEnv = {
      ...process.env,
      TERM: "xterm-256color",
      COLORTERM: "truecolor",
      COLUMNS: String(this._cols),
      LINES: String(this._rows),
      ...this.options.env,
    };

    this._pty = pty.spawn(this.options.command, this.options.args ?? [], {
      name: "xterm-256color",
      cols: this._cols,
      rows: this._rows,
      cwd: this.options.cwd ?? process.cwd(),
      env,
    });

    this._pty.onData((data: string) => {
      this.emit("data", data);
    });

    this._pty.onExit(({ exitCode }: { exitCode: number }) => {
      this.emit("exit", exitCode);
    });
  }

  /**
   * Send a string of characters to the PTY (as if typed by a user).
   * Special keys should be encoded as ANSI escape sequences by the caller.
   */
  write(data: string): void {
    if (!this._pty) throw new Error("PTY session has not been started.");
    this._pty.write(data);
  }

  /** Resize the PTY. */
  resize(cols: number, rows: number): void {
    this._pty?.resize(cols, rows);
  }

  /** Terminate the PTY process. */
  kill(signal: string = "SIGTERM"): void {
    this._pty?.kill(signal);
    this._pty = null;
  }

  get cols(): number {
    return this._cols;
  }
  get rows(): number {
    return this._rows;
  }
}

export { keyToAnsi } from "../utils/keys";
