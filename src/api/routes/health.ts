import { Router, Request, Response } from "express";

/** GET /health */
const healthRouter = Router();

healthRouter.get("/", (_req: Request, res: Response) => {
  res.json({ status: "ok", timestamp: new Date().toISOString() });
});

export default healthRouter;
