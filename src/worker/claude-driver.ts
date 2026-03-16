import Anthropic from "@anthropic-ai/sdk";
import { ActionLog } from "../types";
import { keyToAnsi } from "../utils/keys";

const MAX_TURNS = parseInt(process.env.CLAUDE_MAX_TURNS ?? "30", 10);
const MODEL = process.env.CLAUDE_MODEL ?? "claude-opus-4-5";

export interface ClaudeDriverOptions {
  goal: string;
  /**
   * Callback invoked when Claude decides to send a keystroke.
   * The driver passes the raw ANSI sequence.
   */
  onKeystroke: (sequence: string, description: string) => void;
  /**
   * Callback invoked after each action so the driver can observe the updated
   * terminal screen.  Must return the current screen contents as plain text.
   */
  readScreen: () => string;
  /** Called to signal a short pause between actions (ms). */
  wait?: (ms: number) => Promise<void>;
}

/**
 * Drives a PTY session toward a stated goal using Claude as the reasoning
 * engine.
 *
 * The driver operates in a loop:
 *  1. Read the current terminal screen.
 *  2. Send it to Claude with the goal and the action history.
 *  3. Execute the action Claude returns.
 *  4. Repeat until Claude reports the goal is reached or the turn limit is hit.
 */
export class ClaudeDriver {
  private readonly client: Anthropic;
  private readonly actions: ActionLog[] = [];

  constructor(private readonly options: ClaudeDriverOptions) {
    const apiKey = process.env.ANTHROPIC_API_KEY;
    if (!apiKey) {
      throw new Error("ANTHROPIC_API_KEY environment variable is not set.");
    }
    this.client = new Anthropic({ apiKey });
  }

  /** Run the AI navigation loop. Resolves when done (goal reached or timeout). */
  async run(): Promise<ActionLog[]> {
    const { goal, onKeystroke, readScreen, wait } = this.options;
    const pause = wait ?? defaultWait;

    const systemPrompt = buildSystemPrompt();

    // Conversation history for Claude.
    const messages: Anthropic.MessageParam[] = [];

    for (let turn = 0; turn < MAX_TURNS; turn++) {
      const screen = readScreen();

      const userContent =
        turn === 0
          ? `Goal: ${goal}\n\nCurrent terminal screen:\n\`\`\`\n${screen}\n\`\`\``
          : `Current terminal screen after your last action:\n\`\`\`\n${screen}\n\`\`\`\n\nContinue working toward the goal. If the goal is fully achieved, respond with ACTION: DONE.`;

      messages.push({ role: "user", content: userContent });

      const response = await this.client.messages.create({
        model: MODEL,
        max_tokens: 1024,
        system: systemPrompt,
        messages,
      });

      const assistantText = extractText(response);
      messages.push({ role: "assistant", content: assistantText });

      const action = parseAction(assistantText);

      if (action.type === "done") {
        this.logAction("goal_reached", "Claude reported goal is achieved.");
        break;
      }

      if (action.type === "key") {
        this.logAction("keystroke", action.description, action.raw);
        onKeystroke(action.raw, action.description);
        await pause(action.delayMs ?? 200);
      } else if (action.type === "wait") {
        const ms = action.delayMs ?? 1000;
        this.logAction("wait", `Waiting ${ms} ms`);
        await pause(ms);
      }
    }

    return this.actions;
  }

  private logAction(
    type: ActionLog["type"],
    description: string,
    raw?: string
  ): void {
    this.actions.push({
      timestamp: new Date().toISOString(),
      type,
      description,
      raw,
    });
  }

  get actionLog(): ActionLog[] {
    return [...this.actions];
  }
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

function buildSystemPrompt(): string {
  return `You are TUIcast, an AI that navigates Terminal.Gui TUI applications running in a PTY terminal to demonstrate specific features.

You receive the current terminal screen as plain text and must decide the next action to take in order to achieve the stated goal.

Respond with EXACTLY ONE of the following action formats:

ACTION: KEY <key_name>
  Send a single keystroke. Key names: Enter, Tab, Escape, Backspace, ArrowUp, ArrowDown, ArrowLeft, ArrowRight, F1-F10, Ctrl+C, or any printable character/string.

ACTION: TYPE <text>
  Type a string of characters (equivalent to sending each character in sequence).

ACTION: WAIT <ms>
  Pause for the specified number of milliseconds (e.g. WAIT 500).

ACTION: DONE
  The goal has been fully achieved. Stop recording.

Rules:
- Always respond with exactly one action on its own line.
- You may include a brief REASONING: line before the ACTION line to explain your thinking.
- Do not include anything else after the ACTION line.
- If the app appears stuck or unresponsive after several attempts, try pressing Escape or Tab to recover.
- Terminal.Gui apps are keyboard-driven; mouse is not available.`;
}

interface ParsedAction {
  type: "key" | "wait" | "done";
  raw: string;
  description: string;
  delayMs?: number;
}

function parseAction(text: string): ParsedAction {
  const lines = text.split("\n");

  for (const line of lines) {
    const trimmed = line.trim();

    if (/^ACTION:\s*DONE/i.test(trimmed)) {
      return { type: "done", raw: "", description: "Goal achieved" };
    }

    const keyMatch = trimmed.match(/^ACTION:\s*KEY\s+(.+)$/i);
    if (keyMatch) {
      const keyName = keyMatch[1].trim();
      const raw = keyToAnsi(keyName);
      return { type: "key", raw, description: `KEY ${keyName}` };
    }

    const typeMatch = trimmed.match(/^ACTION:\s*TYPE\s+(.+)$/i);
    if (typeMatch) {
      const typed = typeMatch[1].trim();
      return { type: "key", raw: typed, description: `TYPE "${typed}"` };
    }

    const waitMatch = trimmed.match(/^ACTION:\s*WAIT\s+(\d+)/i);
    if (waitMatch) {
      const ms = parseInt(waitMatch[1], 10);
      return { type: "wait", raw: "", description: `WAIT ${ms}ms`, delayMs: ms };
    }
  }

  // Default: send Enter if Claude responded unclearly.
  return { type: "key", raw: "\r", description: "Enter (default fallback)" };
}

function extractText(response: Anthropic.Message): string {
  return response.content
    .filter((b) => b.type === "text")
    .map((b) => (b as Anthropic.TextBlock).text)
    .join("");
}

function defaultWait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
