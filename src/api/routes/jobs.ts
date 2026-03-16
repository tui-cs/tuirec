import { Router, Request, Response, NextFunction } from "express";
import multer from "multer";
import { v4 as uuidv4 } from "uuid";

import { Job, JobSpec, JobMessage } from "../../types";
import { enqueueJob } from "../../queue/service-bus";
import { uploadBinary } from "../../storage/blob-storage";
import {
  getJob,
  listJobs,
  upsertJob,
  processJob,
} from "../../worker/worker";

const jobsRouter = Router();

// Multer: store uploaded binary in memory (max 256 MB).
const upload = multer({
  storage: multer.memoryStorage(),
  limits: { fileSize: 256 * 1024 * 1024 },
});

// ---------------------------------------------------------------------------
// POST /jobs – submit a new recording job
// ---------------------------------------------------------------------------
jobsRouter.post(
  "/",
  upload.single("binary"),
  async (req: Request, res: Response, next: NextFunction): Promise<void> => {
    try {
      let spec: JobSpec;
      try {
        spec = parseSpec(req.body);
      } catch (err: unknown) {
        res.status(400).json({
          error: err instanceof Error ? err.message : "Invalid request body.",
        });
        return;
      }

      const id = uuidv4();
      const now = new Date().toISOString();
      const job: Job = {
        id,
        status: "queued",
        spec,
        createdAt: now,
        updatedAt: now,
      };
      upsertJob(job);

      // Handle binary upload if provided.
      let binaryBlobName: string | undefined;
      if (req.file) {
        binaryBlobName = `${id}-${req.file.originalname}`;
        await uploadBinary(binaryBlobName, req.file.buffer);
      }

      const message: JobMessage = { jobId: id, spec, binaryBlobName };

      const useQueue =
        process.env.SERVICE_BUS_CONNECTION_STRING &&
        process.env.USE_QUEUE !== "false";

      if (useQueue) {
        await enqueueJob(message);
      } else {
        // Process synchronously in the same process (dev / single-process mode).
        setImmediate(() => processJob(message));
      }

      res.status(202).json(job);
    } catch (err) {
      next(err);
    }
  }
);

// ---------------------------------------------------------------------------
// GET /jobs – list all jobs
// ---------------------------------------------------------------------------
jobsRouter.get("/", (_req: Request, res: Response) => {
  res.json(listJobs());
});

// ---------------------------------------------------------------------------
// GET /jobs/:id – get job status
// ---------------------------------------------------------------------------
jobsRouter.get("/:id", (req: Request, res: Response) => {
  const job = getJob(String(req.params.id));
  if (!job) {
    res.status(404).json({ error: "Job not found." });
    return;
  }
  res.json(job);
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function parseSpec(body: Record<string, unknown>): JobSpec {
  const raw = typeof body.spec === "string" ? JSON.parse(body.spec) : body;

  if (!raw.goal || typeof raw.goal !== "string") {
    throw new Error("'goal' is required and must be a string.");
  }

  if (!raw.source || typeof raw.source !== "object") {
    throw new Error("'source' is required.");
  }

  const source = raw.source as Record<string, unknown>;
  const hasSource =
    source.githubRepo || source.dockerImage || source.binaryPath;
  if (!hasSource && !body.binary) {
    throw new Error(
      "'source' must specify githubRepo, dockerImage, or binaryPath " +
        "(or upload a binary via multipart form)."
    );
  }

  // Coerce binaryPath to indicate an upload when a file is attached.
  if (body.binary && !source.binaryPath) {
    source.binaryPath = "__uploaded__";
  }

  const maxDuration = raw.maxDurationSeconds
    ? Math.min(Number(raw.maxDurationSeconds), 60)
    : 60;

  return {
    goal: raw.goal as string,
    source: {
      githubRepo: source.githubRepo as string | undefined,
      githubRef: source.githubRef as string | undefined,
      dockerImage: source.dockerImage as string | undefined,
      binaryPath: source.binaryPath as string | undefined,
    },
    gifConfig: raw.gifConfig as JobSpec["gifConfig"],
    maxDurationSeconds: maxDuration,
    keystrokes: Array.isArray(raw.keystrokes)
      ? (raw.keystrokes as string[])
      : undefined,
  };
}

export default jobsRouter;
