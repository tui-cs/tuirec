/** Status values for a TUIcast recording job. */
export type JobStatus =
  | "queued"
  | "running"
  | "completed"
  | "failed";

/** Source of the Terminal.Gui application to record. */
export interface AppSource {
  /** Full URL of a GitHub repository (e.g. https://github.com/org/repo). */
  githubRepo?: string;
  /** Git ref (branch, tag, or SHA) for the GitHub repo. Defaults to main. */
  githubRef?: string;
  /**
   * Name of a Docker image that has been pushed to the configured Azure
   * Container Registry.  Mutually exclusive with githubRepo / binaryPath.
   */
  dockerImage?: string;
  /**
   * Path (within the job container) to a self-contained executable.
   * Used when the binary is uploaded directly via the API.
   */
  binaryPath?: string;
}

/** Visual / playback settings for the rendered GIF. */
export interface GifConfig {
  /** Terminal width in columns. Default: 120. */
  cols?: number;
  /** Terminal height in rows. Default: 30. */
  rows?: number;
  /** Font family recognised by agg. Default: "JetBrains Mono". */
  font?: string;
  /** Font size in pixels. Default: 14. */
  fontSize?: number;
  /**
   * Named theme for agg (e.g. "monokai", "dracula", "solarized-dark").
   * Default: "monokai".
   */
  theme?: string;
  /** Playback speed multiplier for the rendered GIF. Default: 1.0. */
  speed?: number;
  /**
   * Vertical line-height multiplier passed to agg.
   * 1.0 = no extra gap between rows (tight); agg default is 1.4.
   * Use 1.0 for Terminal.Gui apps whose cells already have built-in padding.
   * Default: 1.0.
   */
  lineHeight?: number;
}

/** Full specification for a TUIcast recording job. */
export interface JobSpec {
  /** Plain-English description of what the recording should demonstrate. */
  goal: string;
  /** Source of the Terminal.Gui application. */
  source: AppSource;
  /** Optional GIF rendering configuration. */
  gifConfig?: GifConfig;
  /**
   * Maximum duration of the PTY session in seconds. Capped at 60.
   * Default: 60.
   */
  maxDurationSeconds?: number;
  /**
   * Optional deterministic keystroke script.  Each element is either a
   * string of characters to type or a special key name (e.g. "Enter",
   * "Tab", "Escape", "ArrowUp").
   * When provided the AI vision loop is skipped in favour of this script.
   */
  keystrokes?: string[];
}

/** Persisted state of a TUIcast recording job. */
export interface Job {
  /** Unique identifier (UUID v4). */
  id: string;
  /** Current processing status. */
  status: JobStatus;
  /** Original specification provided by the submitter. */
  spec: JobSpec;
  /** ISO-8601 creation timestamp. */
  createdAt: string;
  /** ISO-8601 timestamp of the last status update. */
  updatedAt: string;
  /** Presigned URL from which the finished GIF can be downloaded. */
  gifUrl?: string;
  /**
   * Presigned URL for the raw asciinema cast file (for debugging /
   * re-rendering).
   */
  castUrl?: string;
  /** Human-readable error message if the job failed. */
  error?: string;
  /** Structured log of every action taken by the AI driver. */
  actions?: ActionLog[];
}

/** A single action taken by the AI driver during a recording session. */
export interface ActionLog {
  /** ISO-8601 timestamp. */
  timestamp: string;
  /** Type of action performed. */
  type: "keystroke" | "mouse" | "wait" | "goal_reached" | "timeout";
  /** Human-readable description of the action. */
  description: string;
  /** Raw value sent to the PTY (e.g. ANSI sequence). */
  raw?: string;
}

/** Message envelope placed on the Azure Service Bus queue. */
export interface JobMessage {
  jobId: string;
  spec: JobSpec;
  /** Blob name of the uploaded binary (if any). */
  binaryBlobName?: string;
}
