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
  "Ctrl+A": "\x01",
  "Ctrl+B": "\x02",
  "Ctrl+C": "\x03",
  "Ctrl+D": "\x04",
  "Ctrl+E": "\x05",
  "Ctrl+F": "\x06",
  "Ctrl+G": "\x07",
  "Ctrl+H": "\x08",
  "Ctrl+I": "\x09",
  "Ctrl+J": "\x0a",
  "Ctrl+K": "\x0b",
  "Ctrl+L": "\x0c",
  "Ctrl+M": "\x0d",
  "Ctrl+N": "\x0e",
  "Ctrl+O": "\x0f",
  "Ctrl+P": "\x10",
  "Ctrl+Q": "\x11",
  "Ctrl+R": "\x12",
  "Ctrl+S": "\x13",
  "Ctrl+T": "\x14",
  "Ctrl+U": "\x15",
  "Ctrl+V": "\x16",
  "Ctrl+W": "\x17",
  "Ctrl+X": "\x18",
  "Ctrl+Y": "\x19",
  "Ctrl+Z": "\x1a",
};

/**
 * Convert a human-readable key name to the corresponding ANSI escape sequence.
 * Unknown keys are returned unchanged.
 */
export function keyToAnsi(key: string): string {
  return KEY_ANSI_MAP[key] ?? key;
}

/**
 * Generate an SGR (XTerm extended) mouse click sequence for the given
 * 1-based column and row.  Sends a left-button press immediately followed
 * by a release so the app sees a complete click event.
 *
 * Terminal.Gui v2 enables SGR mouse reporting (\x1b[?1006h) on start-up, so
 * these sequences are understood without any prior setup from our side.
 *
 * Format:
 *   press   \x1b[<0;{col};{row}M
 *   release \x1b[<0;{col};{row}m
 */
export function mouseClickSequence(col: number, row: number): string {
  return `\x1b[<0;${col};${row}M\x1b[<0;${col};${row}m`;
}
