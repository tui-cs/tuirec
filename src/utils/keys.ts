/**
 * Maps a human-readable key name to the corresponding ANSI escape sequence
 * that a PTY expects. Unknown keys are returned unchanged (treated as literal
 * character(s) to type).
 */
export const KEY_ANSI_MAP: Readonly<Record<string, string>> = {
  Enter: "\r",
  Return: "\r",
  Tab: "\t",
  Escape: "\x1b",
  Backspace: "\x7f",
  Delete: "\x1b[3~",
  ArrowUp: "\x1b[A",
  ArrowDown: "\x1b[B",
  ArrowRight: "\x1b[C",
  ArrowLeft: "\x1b[D",
  Home: "\x1b[H",
  End: "\x1b[F",
  PageUp: "\x1b[5~",
  PageDown: "\x1b[6~",
  F1: "\x1bOP",
  F2: "\x1bOQ",
  F3: "\x1bOR",
  F4: "\x1bOS",
  F5: "\x1b[15~",
  F6: "\x1b[17~",
  F7: "\x1b[18~",
  F8: "\x1b[19~",
  F9: "\x1b[20~",
  F10: "\x1b[21~",
  "Ctrl+C": "\x03",
  "Ctrl+D": "\x04",
  "Ctrl+Z": "\x1a",
  "Ctrl+L": "\x0c",
  "Ctrl+A": "\x01",
  "Ctrl+E": "\x05",
};

/**
 * Convert a human-readable key name to the corresponding ANSI escape sequence.
 * Unknown keys are returned unchanged.
 */
export function keyToAnsi(key: string): string {
  return KEY_ANSI_MAP[key] ?? key;
}
