import { createWriteStream, WriteStream } from "fs";
import { EventEmitter } from "events";

export interface CastHeader {
  version: 2;
  width: number;
  height: number;
  timestamp: number;
  title?: string;
  env?: Record<string, string>;
}

export interface CastEvent {
  time: number;
  type: "o" | "i";
  data: string;
}

/**
 * Records a terminal session in asciinema v2 cast format.
 *
 * Usage:
 *   const rec = new AsciinemaRecorder({ outputPath, cols, rows });
 *   rec.start();
 *   // ... later ...
 *   rec.write(chunk);   // called with each PTY data chunk
 *   rec.stop();
 *   await rec.flush();
 */
export class AsciinemaRecorder extends EventEmitter {
  private _stream: WriteStream | null = null;
  private _startTime: number = 0;
  private _events: CastEvent[] = [];
  private readonly outputPath: string;
  private readonly cols: number;
  private readonly rows: number;
  private readonly title: string;

  constructor(options: {
    outputPath: string;
    cols: number;
    rows: number;
    title?: string;
  }) {
    super();
    this.outputPath = options.outputPath;
    this.cols = options.cols;
    this.rows = options.rows;
    this.title = options.title ?? "TUIcast Recording";
  }

  /** Begin the recording session. */
  start(): void {
    this._startTime = Date.now();
    this._events = [];
    this._stream = createWriteStream(this.outputPath, { encoding: "utf8" });

    const header: CastHeader = {
      version: 2,
      width: this.cols,
      height: this.rows,
      timestamp: Math.floor(this._startTime / 1000),
      title: this.title,
      env: { TERM: "xterm-256color", SHELL: "/bin/bash" },
    };
    this._stream.write(JSON.stringify(header) + "\n");
  }

  /**
   * Record a chunk of output received from the PTY.
   * @param data – Raw terminal output (may contain ANSI sequences).
   */
  write(data: string): void {
    if (!this._stream) return;
    const elapsed = (Date.now() - this._startTime) / 1000;
    const event: CastEvent = { time: elapsed, type: "o", data };
    this._events.push(event);
    // asciinema v2 format: each event is a JSON array on its own line.
    this._stream.write(JSON.stringify([event.time, event.type, event.data]) + "\n");
  }

  /** Finish recording and close the output stream. */
  async stop(): Promise<void> {
    if (!this._stream) return;
    await new Promise<void>((resolve, reject) => {
      this._stream!.end((err: Error | null | undefined) => {
        if (err) reject(err);
        else resolve();
      });
    });
    this._stream = null;
  }

  /** Duration of the recording in seconds. */
  get durationSeconds(): number {
    if (this._events.length === 0) return 0;
    return this._events[this._events.length - 1].time;
  }

  /** Number of recorded events. */
  get eventCount(): number {
    return this._events.length;
  }
}
