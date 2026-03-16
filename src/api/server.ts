import express, { Request, Response, NextFunction } from "express";
import cors from "cors";
import rateLimit from "express-rate-limit";
import path from "path";

import healthRouter from "./routes/health";
import jobsRouter from "./routes/jobs";

const app = express();
const PORT = parseInt(process.env.PORT ?? "3000", 10);

// ---------------------------------------------------------------------------
// Rate limiting
// ---------------------------------------------------------------------------

/** General API rate limiter: 100 requests per minute per IP. */
const apiLimiter = rateLimit({
  windowMs: 60 * 1000,
  max: 100,
  standardHeaders: true,
  legacyHeaders: false,
  message: { error: "Too many requests – please try again later." },
});

/**
 * Job submission rate limiter: 10 job submissions per minute per IP.
 * Recording jobs are compute-intensive; this prevents abuse.
 */
const jobSubmitLimiter = rateLimit({
  windowMs: 60 * 1000,
  max: 10,
  standardHeaders: true,
  legacyHeaders: false,
  message: { error: "Too many job submissions – please try again later." },
});

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------
app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Apply general rate limiting to all API routes.
app.use("/health", apiLimiter);
app.use("/jobs", apiLimiter);

// Serve static frontend (no rate limit needed for static files in practice,
// but we apply a permissive limit to satisfy security scanners).
app.use(
  rateLimit({ windowMs: 60 * 1000, max: 300, standardHeaders: true, legacyHeaders: false }),
  express.static(path.join(__dirname, "../../public"))
);

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------
app.use("/health", healthRouter);
// Apply tighter limit specifically to job submission.
app.post("/jobs", jobSubmitLimiter);
app.use("/jobs", jobsRouter);

// Catch-all: serve index.html for any unmatched route (SPA support).
const staticPageLimiter = rateLimit({
  windowMs: 60 * 1000,
  max: 300,
  standardHeaders: true,
  legacyHeaders: false,
});
app.get("*", staticPageLimiter, (_req: Request, res: Response) => {
  res.sendFile(path.join(__dirname, "../../public", "index.html"));
});

// ---------------------------------------------------------------------------
// Global error handler
// ---------------------------------------------------------------------------
// eslint-disable-next-line @typescript-eslint/no-unused-vars
app.use((err: Error, _req: Request, res: Response, _next: NextFunction) => {
  console.error("[api] Unhandled error:", err);
  res.status(500).json({ error: err.message ?? "Internal server error." });
});

// ---------------------------------------------------------------------------
// Start
// ---------------------------------------------------------------------------
if (require.main === module) {
  app.listen(PORT, () => {
    console.log(`[api] TUIcast API listening on http://0.0.0.0:${PORT}`);
  });
}

export default app;
