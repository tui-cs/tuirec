import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { AsciinemaRecorder } from "../worker/recorder";

describe("AsciinemaRecorder", () => {
  let tmpDir: string;
  let castPath: string;

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "tuicast-test-"));
    castPath = path.join(tmpDir, "test.cast");
  });

  afterEach(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  });

  it("writes a valid asciinema v2 header on start", async () => {
    const recorder = new AsciinemaRecorder({
      outputPath: castPath,
      cols: 80,
      rows: 24,
      title: "Test Recording",
    });

    recorder.start();
    await recorder.stop();

    const content = fs.readFileSync(castPath, "utf8");
    const lines = content.trim().split("\n");
    expect(lines.length).toBeGreaterThanOrEqual(1);

    const header = JSON.parse(lines[0]);
    expect(header.version).toBe(2);
    expect(header.width).toBe(80);
    expect(header.height).toBe(24);
    expect(header.title).toBe("Test Recording");
  });

  it("records output events with elapsed time", async () => {
    const recorder = new AsciinemaRecorder({
      outputPath: castPath,
      cols: 80,
      rows: 24,
    });

    recorder.start();
    recorder.write("hello");
    recorder.write(" world");
    await recorder.stop();

    const content = fs.readFileSync(castPath, "utf8");
    const lines = content.trim().split("\n");
    // Header + 2 events
    expect(lines.length).toBe(3);

    const event1 = JSON.parse(lines[1]);
    expect(event1[1]).toBe("o");
    expect(event1[2]).toBe("hello");

    const event2 = JSON.parse(lines[2]);
    expect(event2[1]).toBe("o");
    expect(event2[2]).toBe(" world");
  });

  it("reports correct event count and duration", async () => {
    const recorder = new AsciinemaRecorder({
      outputPath: castPath,
      cols: 80,
      rows: 24,
    });

    recorder.start();
    expect(recorder.eventCount).toBe(0);
    expect(recorder.durationSeconds).toBe(0);

    recorder.write("a");
    recorder.write("b");
    expect(recorder.eventCount).toBe(2);

    await recorder.stop();
    expect(recorder.durationSeconds).toBeGreaterThanOrEqual(0);
  });
});
