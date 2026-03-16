import { keyToAnsi } from "../utils/keys";

describe("keyToAnsi", () => {
  it("maps Enter to carriage return", () => {
    expect(keyToAnsi("Enter")).toBe("\r");
  });

  it("maps Tab to horizontal tab", () => {
    expect(keyToAnsi("Tab")).toBe("\t");
  });

  it("maps Escape to ESC byte", () => {
    expect(keyToAnsi("Escape")).toBe("\x1b");
  });

  it("maps ArrowUp to ANSI CSI sequence", () => {
    expect(keyToAnsi("ArrowUp")).toBe("\x1b[A");
  });

  it("maps ArrowDown to ANSI CSI sequence", () => {
    expect(keyToAnsi("ArrowDown")).toBe("\x1b[B");
  });

  it("maps Ctrl+C to ETX", () => {
    expect(keyToAnsi("Ctrl+C")).toBe("\x03");
  });

  it("returns unknown keys unchanged", () => {
    expect(keyToAnsi("a")).toBe("a");
    expect(keyToAnsi("hello")).toBe("hello");
  });
});
