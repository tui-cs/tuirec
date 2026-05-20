# Recording Agent — TUIcast Keystroke Guide

This document teaches an AI agent how to compose TUIcast keystroke scripts for
recording any terminal application. Any AI system (Claude, Copilot, Codex, etc.)
can use this as context to drive `record-app.ps1` or `tuicast record` directly.

## Quick start

```powershell
./agent/record-app.ps1 `
    -Binary "./my-app" `
    -Name "search-replace" `
    -Title "my-app: find and replace" `
    -ShowCommand '$ my-app config.yaml' `
    -Keystrokes "wait:2000,Ctrl+H,hello,Tab,world,Alt+A,wait:1500,Esc"
```

---

## TUIcast keystroke syntax

Key tokens use **Terminal.Gui's `Key.ToString()` / `Key.TryParse()` format**.
A keystroke script is a **comma-separated** string. Each token is one of:

| Token type | Examples | Description |
|---|---|---|
| **Wait** | `wait:2000` | Pause N milliseconds before next key |
| **Named key** | `Enter`, `Esc`, `Tab`, `Space`, `Backspace`, `Delete` | Single special key press |
| **Arrow/nav** | `CursorUp`, `CursorDown`, `CursorLeft`, `CursorRight`, `Home`, `End`, `PageUp`, `PageDown` | Navigation keys |
| **Function key** | `F1`–`F12` | Function keys |
| **Modifier combo** | `Ctrl+S`, `Ctrl+Shift+Z`, `Alt+A`, `Shift+Tab`, `Ctrl+Alt+Shift+CursorUp` | Modifier + key |
| **Mouse click** | `click:10:5` | SGR mouse click at column:row (1-based) |
| **Literal text** | `hello world` | Typed character-by-character (spaces included) |

### Rules

- Uses Terminal.Gui key format: `Ctrl+C`, `Ctrl-C`, `A-Ctrl` are all valid.
- Older aliases like `ArrowUp`, `ArrowDown`, `Escape` are accepted (prefer
  `CursorUp`, `CursorDown`, `Esc`).
- **Unknown key-like tokens** (e.g. `Ctrl-Foo`) fail fast — they won't be
  silently typed as literal text.
- Literal text tokens are everything that doesn't match a known key name or
  `wait:N`.
- Commas inside literal text are **not supported** — split around them with
  separate tokens or escape with `\,`.
- `wait:N` is essential between actions that trigger UI transitions (dialog
  open, file load, menu animation). **Always wait after opening a dialog or
  menu.**

### Known gotchas

- ⚠️ **`cursor`, `page`, `arrow` as literal text** — the parser treats these as
  key-like prefixes (matching `CursorUp`, `PageDown`, etc.). If you need to
  type them literally, split across token boundaries: `cur,sor` types "cursor",
  `pa,ge` types "page". This is especially common when searching in Terminal.Gui
  apps where "cursor" is a frequent term.
- **`--agg-path` is required** unless `agg` is on your system PATH. When calling
  `tuicast record` directly, **always pass `--agg-path`**. Typical location:
  `~/tools/agg.exe` (Windows) or `~/tools/agg` (Unix). The `record-app.ps1`
  wrapper handles this automatically.
- **`--show-command` format** — TUIcast renders exactly what you provide. Include
  the `$ ` prompt prefix yourself if you want one: `--show-command '$ myapp foo'`.
  TUIcast does not add its own prompt decoration.
- **`--show-command` with alt-screen apps** — works correctly (pre-roll enters
  alt-screen automatically), but the synthetic prompt frame will be brief. Omit
  it if the app's own UI is the focus.
- **`--keystroke-delay` affects literal text too** — each character in a literal
  token gets the inter-key delay (default 200ms). For fast typing sequences,
  use `--keystroke-delay 50` or shorter.
- **If `record-app.ps1` is blocked** by execution policy or permissions, fall
  back to calling `tuicast.exe record` directly with equivalent flags.

---

## Composing keystroke scripts — best practices

1. **Always start with a wait** — `wait:1500` or `wait:2000` gives the app time
   to start and render its first frame.

2. **Use `--show-command` for polish** — adds a synthetic `$ my-app file.txt`
   prompt frame before the app launches. Makes the GIF look like a real terminal
   session.

3. **Use `--startup-delay`** when the app needs extra init time (large file,
   network) before you want output captured.

4. **Wait after UI transitions** — opening a dialog, switching tabs, or loading
   a file needs `wait:500` to `wait:1000` for the UI to settle before the next
   action.

5. **End with the app's quit key** — typically `Esc` or `Ctrl+C`. Ensure the
   process exits cleanly so the recording stops without hitting max-duration.

6. **Keep recordings short** — aim for 10–30 seconds of real time. Viewers lose
   interest after that. Use `--max-duration 60` as a safety net.

7. **Show, don't rush** — generous waits between meaningful actions let the
   viewer see what happened. `wait:1500` after a search highlights the match
   visually.

8. **Use `--verbosity high`** when debugging a keystroke script that isn't
   working as expected — it logs each key token and timing to stderr.

9. **Use `--kitty-keyboard` for Terminal.Gui apps** — this enables the Kitty
   keyboard protocol, which disambiguates Ctrl+M from Enter, Ctrl+I from
   Tab, etc. The app must support progressive enhancement (Terminal.Gui v2
   does). Without this flag, those key pairs produce identical bytes.

---

## Example keystroke scripts

### Open an app, type, and quit

```
wait:2000,Hello world!,wait:1500,Enter,More text here,wait:1500,Ctrl+C
```

### Navigate a file

```
wait:2000,PageDown,wait:1500,PageDown,wait:1500,Home,wait:1000,Esc
```

### Find and replace (Terminal.Gui app)

```
wait:2000,Ctrl+H,wait:500,hello,Tab,world,Alt+A,wait:1500,Esc,wait:500,Esc
```

### Menu-driven interaction

```
wait:2000,Alt+F,wait:400,O,wait:600,./myfile.txt,Enter,wait:2000,Esc
```

### Mouse click demo

```
wait:2000,click:15:3,wait:1000,click:40:10,wait:1500,Esc
```

---

## Invoking the recording

### Direct `tuicast record`

```bash
tuicast record \
    --binary ./my-app \
    --keystrokes "wait:2000,Hello,wait:1500,Esc" \
    --output demo.gif \
    --cast-output demo.cast \
    --show-command '$ my-app' \
    --startup-delay 500 \
    --kitty-keyboard \
    --agg-path ~/tools/agg \
    --cols 120 \
    --rows 36 \
    --max-duration 45
```

### Via `record-app.ps1` (auto-resolves tools)

```powershell
./agent/record-app.ps1 `
    -Binary "./my-app" `
    -Name "demo" `
    -Title "my-app demo" `
    -ShowCommand '$ my-app' `
    -Keystrokes "wait:2000,Hello,wait:1500,Esc"
```

### Parameters

| Parameter | Required | Default | Description |
|---|---|---|---|
| `-Binary` | **Yes** | — | Path to the target executable |
| `-Keystrokes` | **Yes** | — | The TUIcast keystroke script |
| `-Name` | No | `demo` | Short ID for filenames (`<Name>.gif`) |
| `-Title` | No | `recording` | Title in cast metadata |
| `-ShowCommand` | No | — | Synthetic shell prompt pre-roll |
| `-StartupDelay` | No | 0 | Ms to wait after process start before output capture |
| `-InputDelay` | No | 0 | Ms pause before scripted keys begin |
| `-Output` | No | `artifacts/<Name>.gif` | GIF path |
| `-CastOutput` | No | `artifacts/<Name>.cast` | Cast path |
| `-Cols` | No | 120 | Terminal columns |
| `-Rows` | No | 36 | Terminal rows |
| `-MaxDuration` | No | 60 | Safety timeout (seconds) |
| `-DrainMs` | No | 1500 | Wait after last keystroke |
| `-Verbosity` | No | — | `quiet`, `normal`, or `high` |
| `-KittyKeyboard` | No | false | Enable Kitty keyboard protocol for modifier disambiguation |
| `-Args` | No | — | Arguments to pass to the binary |
| `-TuicastVersion` | No | `0.1.5` | Auto-download version |

---

## For AI agents — how to use this

When asked to "record <app> doing X", follow this process:

1. **Read this document** for keystroke syntax and best practices.
2. **Understand the target app's UI** — what keys does it respond to? What's its
   quit key? What dialogs does it have?
3. **Plan the interaction** — break the demo into steps (launch → navigate →
   perform action → show result → close).
4. **Compose the keystroke string** — use waits generously between transitions.
5. **Call `tuicast record`** (or `record-app.ps1`) with appropriate parameters.
6. **If execution fails due to permissions**, output the full command for the user
   to run manually — do not loop retrying.
7. **Report the output paths** back to the user.

You do NOT need to know the exact pixel layout — TUIcast drives the app through
its terminal input, just like a user would type. Focus on the logical key
sequence to accomplish the demo goal.

### Terminal.Gui app defaults

For any Terminal.Gui application (UICatalog, ted, etc.), always use:

```powershell
--kitty-keyboard --startup-delay 2000 --drain 2000 --cols 120 --rows 30
```

The binary is typically at:
```
<repo>\Examples\<AppName>\bin\Debug\net10.0\<AppName>.exe
```

Common Terminal.Gui keys: `Ctrl+A` (About), `Ctrl+Q` (Quit), `Alt+F` (File
menu), `F9` (Menu bar focus), `Esc` (close dialog/cancel).
