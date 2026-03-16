import request from "supertest";
import app from "../api/server";
import { getJob } from "../worker/worker";

// Prevent the worker from actually running jobs during API tests.
jest.mock("../worker/worker", () => {
  const store = new Map<string, import("../types").Job>();
  return {
    getJob: (id: string) => store.get(id),
    listJobs: () => [...store.values()],
    upsertJob: (job: import("../types").Job) => store.set(job.id, job),
    processJob: jest.fn().mockResolvedValue(undefined),
  };
});

// Prevent actual blob storage calls.
jest.mock("../storage/blob-storage", () => ({
  uploadBinary: jest.fn().mockResolvedValue("test-blob"),
  uploadGif: jest.fn().mockResolvedValue("https://example.com/test.gif"),
  uploadCast: jest.fn().mockResolvedValue("https://example.com/test.cast"),
  downloadBinary: jest.fn().mockResolvedValue(undefined),
}));

// Prevent actual Service Bus calls.
jest.mock("../queue/service-bus", () => ({
  enqueueJob: jest.fn().mockResolvedValue(undefined),
  startWorkerReceiver: jest.fn(),
  closeQueue: jest.fn(),
}));

describe("GET /health", () => {
  it("returns 200 with status ok", async () => {
    const res = await request(app).get("/health");
    expect(res.status).toBe(200);
    expect(res.body.status).toBe("ok");
    expect(res.body.timestamp).toBeDefined();
  });
});

describe("POST /jobs", () => {
  it("returns 202 with job details for a valid github source", async () => {
    const spec = {
      goal: "Show main menu navigation",
      source: { githubRepo: "https://github.com/gui-cs/Terminal.Gui", githubRef: "main" },
    };

    const res = await request(app)
      .post("/jobs")
      .set("Content-Type", "application/json")
      .send(spec);

    expect(res.status).toBe(202);
    expect(res.body.id).toBeDefined();
    expect(res.body.status).toBe("queued");
    expect(res.body.spec.goal).toBe("Show main menu navigation");
  });

  it("returns 400 when goal is missing", async () => {
    const res = await request(app)
      .post("/jobs")
      .set("Content-Type", "application/json")
      .send({ source: { githubRepo: "https://github.com/gui-cs/Terminal.Gui" } });

    expect(res.status).toBe(400);
    expect(res.body.error).toMatch(/goal/i);
  });

  it("returns 400 when source is missing and no binary uploaded", async () => {
    const res = await request(app)
      .post("/jobs")
      .set("Content-Type", "application/json")
      .send({ goal: "Show main menu navigation" });

    expect(res.status).toBe(400);
    expect(res.body.error).toMatch(/source/i);
  });
});

describe("GET /jobs", () => {
  it("returns an array", async () => {
    const res = await request(app).get("/jobs");
    expect(res.status).toBe(200);
    expect(Array.isArray(res.body)).toBe(true);
  });
});

describe("GET /jobs/:id", () => {
  it("returns 404 for an unknown job id", async () => {
    const res = await request(app).get("/jobs/nonexistent-id");
    expect(res.status).toBe(404);
    expect(res.body.error).toMatch(/not found/i);
  });

  it("returns the job if it exists", async () => {
    // First submit a job to put it in the store.
    const submitRes = await request(app)
      .post("/jobs")
      .set("Content-Type", "application/json")
      .send({
        goal: "Navigate file menu",
        source: { githubRepo: "https://github.com/gui-cs/Terminal.Gui" },
      });
    expect(submitRes.status).toBe(202);

    const jobId = submitRes.body.id;
    const res = await request(app).get(`/jobs/${jobId}`);
    expect(res.status).toBe(200);
    expect(res.body.id).toBe(jobId);
  });
});
