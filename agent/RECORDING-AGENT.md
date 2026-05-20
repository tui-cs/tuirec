# Recording Agent — TUIcast Keystroke Guide

This document teaches an AI agent how to compose TUIcast keystroke scripts for
recording any terminal application. Any AI system (Claude, Copilot, Codex, etc.)
can use this as context to drive `tuicast record` directly.

## Quick start

```bash
# From the TUIcast repo root (after `go build ./cmd/tuicast`):
./tuicast record \
    --binary "./my-app" \
    --name "search-replace" \
    --title "my-app: find and replace" \
    --show-command '$ my-app config.yaml' \
    --keystrokes "wait:2000,Ctrl+H,hello,Tab,world,Alt+A,wait:1500,Esc"
```

> **Note:** `tuicast record` auto-downloads `agg` if not found on PATH or in the
> cache (`~/.cache/tuicast/agg-v1.5.0/`). No separate setup needed.
>
> **From source:** If `tuicast` is not on PATH, build it first with
> `go build -o tuicast.exe ./cmd/tuicast` (Windows) or
> `go build -o tuicast ./cmd/tuicast` (Unix), then invoke via `./tuicast`.

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

- ⚠️ **`cursor`, `page`, `arrow` as literal text** — the parser recognizes
  these as prefixes of key names (`CursorUp`, `PageDown`, `ArrowLeft`). A bare
  token that equals "cursor", "page", or "arrow" exactly is treated as literal
  (it's too short to be a key name), but **Terminal.Gui's `Key.TryParse` might
  accept it in a future version**. Defensive best practice: always split across
  token boundaries: `cur,sor` types "cursor", `pa,ge` types "page".
  **Note:** splitting inserts one `--keystroke-delay` pause at the split point.
  Use `--keystroke-delay 50` to minimize the visible gap when splitting words.
- **When is a token literal?** Any alphanumeric token that doesn't match a known
  key name (`Enter`, `Esc`, `F1`, `CursorUp`, etc.) and doesn't match the
  `wait:N` or `click:col:row` patterns is typed as literal text. Short tokens
  like `de`, `ab`, `foo` are always safe. Only worry about tokens that *start
  with* `Cursor`, `Page`, `Arrow` followed by more characters — those resolve
  as key names.
- **`--agg-path` is required** unless `agg` is on your system PATH. **Always
  pass `--agg-path ~/tools/agg.exe`** (Windows) or `--agg-path ~/tools/agg`
  (Unix) when calling `tuicast record` directly.
- **`--show-command` format** — TUIcast renders exactly what you provide. Include
  the `$ ` prompt prefix yourself if you want one: `--show-command '$ myapp foo'`.
  TUIcast does not add its own prompt decoration. **Windows/PowerShell note:**
  use single quotes to prevent `$` interpolation:
  `--show-command '$ myapp'` — double quotes would require backtick-escaping
  `` --show-command "`$ myapp" ``.
- **`--show-command` with alt-screen apps** — works correctly (pre-roll enters
  alt-screen automatically), but the synthetic prompt frame will be brief. Omit
  it if the app's own UI is the focus.
- **`--keystroke-delay` affects literal text too** — each character in a literal
  token gets the inter-key delay (default 200ms). Characters *within* a single
  token are paced by `--keystroke-delay`; the same delay also applies at token
  boundaries. **Rule of thumb:** budget `n × 200ms` per literal word at default
  delay (e.g. "cursor" = 6 chars × 200ms = 1.2s). For typing-heavy scripts, use
  `--keystroke-delay 50` or shorter to keep total recording time reasonable.
- **If `record-app.ps1` is blocked** by execution policy or permissions, fall
  back to calling `tuicast.exe record` directly with equivalent flags. If
  PowerShell's `&` call operator is also blocked, wrap in `cmd /c`:
  `cmd /c "tuicast.exe record --binary ... 2>err.txt"`
- **First frame may be blank** — `--startup-delay` records the alt-screen
  transition as the initial frame. The actual UI appears after the delay. This
  is normal; the blank frame is brief in the GIF.
- **Verifying recording content** — after recording, check the `.cast` file for
  expected output strings (e.g. `grep "About" demo.cast`) to confirm the app
  reached the intended state without needing to view the GIF.

---

## Composing keystroke scripts — best practices

1. **Always start with a wait** — `wait:1500` or `wait:2000` gives the app time
   to start and render its first frame.

2. **Use `--show-command` for polish** — adds a synthetic `$ my-app file.txt`
   prompt frame before the app launches. Makes the GIF look like a real terminal
   session.

3. **Use `--startup-delay`** when the app needs extra init time (large file,
   network) before you want output captured. **Note:** `--startup-delay` delays
   *output capture* only — keystroke playback starts independently after the
   script's first `wait:` token. You do NOT need both `--startup-delay 2000` and
   `wait:2000` at the start of your script; use one or the other. Use
   `--startup-delay` to suppress early noise; use `wait:` to pace visible actions.

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

10. **Use `--drain 2000` for TUI apps** — after the last keystroke, keep
    recording for 2 seconds so the final UI state is visible in the GIF.
    Without drain, the recording may end before the last action renders.

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

### Search with literal text that matches key prefixes

```
wait:2000,Ctrl+F,wait:500,cur,sor,Enter,wait:1500,Esc
```

Note: `cursor` is split as `cur,sor` as a defensive measure — the parser
currently treats bare `cursor` as literal, but `Key.TryParse` could accept it
in the future. Same applies to `page` → `pa,ge`.

### Mouse click demo

```
wait:2000,click:15:3,wait:1000,click:40:10,wait:1500,Esc
```

---

## Invoking the recording

### Direct `tuicast record` (recommended)

```bash
tuicast record \
    --binary ./my-app \
    --name "demo" \
    --keystrokes "wait:2000,Hello,wait:1500,Esc" \
    --show-command '$ my-app' \
    --startup-delay 500 \
    --kitty-keyboard \
    --cols 120 \
    --rows 36 \
    --max-duration 45
```

The `--name` flag sets `--output artifacts/<name>.gif` and `--cast-output
artifacts/<name>.cast` automatically. You can override either with explicit
flags. `tuicast` will auto-download `agg` if it's not on PATH.

### Via `record-app.ps1` (deprecated)

> **Deprecated:** Use `tuicast record --name <name>` instead. The script is
> retained for backward compatibility and will be removed in a future release.

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
| `--binary` | **Yes** | — | Path to the target executable |
| `--keystrokes` | **Yes** | — | The TUIcast keystroke script |
| `--name` | No | — | Short ID for filenames (`artifacts/<name>.gif`) |
| `--title` | No | — | Title in cast metadata |
| `--show-command` | No | — | Synthetic shell prompt pre-roll |
| `--startup-delay` | No | 0 | Ms to wait after process start before output capture |
| `--input-delay` | No | 0 | Ms pause before scripted keys begin |
| `--output` | No | `recording.gif` | GIF path (overrides `--name`) |
| `--cast-output` | No | — | Cast path (overrides `--name`) |
| `--cols` | No | 120 | Terminal columns |
| `--rows` | No | 30 | Terminal rows |
| `--max-duration` | No | 60 | Safety timeout (seconds) |
| `--drain` | No | 500 | Wait after last keystroke (ms) |
| `--verbosity` | No | `normal` | `quiet`, `normal`, or `high` |
| `--kitty-keyboard` | No | false | Enable Kitty keyboard protocol for modifier disambiguation |
| `--args` | No | — | Arguments to pass to the binary |
| `--agg-path` | No | auto | Path to agg (auto-downloaded if not found) |
| `--open` | No | false | Open the GIF in the default viewer after recording |
| `--copy` | No | false | Copy the GIF file path to the system clipboard |

---

## For AI agents — how to use this

When asked to "record <app> doing X", follow this process:

1. **Read this document** for keystroke syntax and best practices.
2. **Understand the target app's UI** — what keys does it respond to? What's its
   quit key? What dialogs does it have? **Examine the app's source code** if
   available — look at View composition, tab order, key bindings, and control
   types (e.g. DateEditor, ColorPicker) to determine what keystrokes each control
   accepts.
3. **Plan the interaction** — break the demo into steps (launch → navigate →
   perform action → show result → close).
4. **Compose the keystroke string** — use waits generously between transitions.
5. **Call `tuicast record --name <name> --open --copy`** with appropriate
   parameters. `--open` launches the GIF in the default viewer so the user sees
   the result immediately; `--copy` puts the GIF path on the clipboard. Always
   include both. The binary auto-downloads agg and creates the artifacts/
   directory as needed.
6. **If execution fails due to permissions**, output the full command for the user
   to run manually — do not loop retrying.
7. **Report the output paths** back to the user.

You do NOT need to know the exact pixel layout — TUIcast drives the app through
its terminal input, just like a user would type. Focus on the logical key
sequence to accomplish the demo goal.

### Terminal.Gui app defaults

For any Terminal.Gui application (UICatalog, ted, etc.), always use:

```powershell
--kitty-keyboard --startup-delay 2000 --drain 2000
```

The default `--cols 120 --rows 30` is appropriate for most demos. Increase rows
for apps with tall content (e.g. `--rows 40` for log viewers) or cols for wide
tables.

The binary is typically at:
```
<repo>\Examples\<AppName>\bin\Debug\net10.0\<AppName>.exe
```

Common Terminal.Gui keys: `Ctrl+A` (About), `Ctrl+Q` (Quit), `Alt+F` (File
menu), `F9` (Menu bar focus), `Esc` (close dialog/cancel).

### Common text-editing keys

- `Home` — move cursor to start of field (use before typing to overwrite)
- `End` — move cursor to end of field
- `Ctrl+A` — select all (in text fields; note: also opens About in UICatalog)
- `Delete` / `Backspace` — delete character
- `Tab` / `Shift+Tab` — move between controls

### Terminal.Gui control keystroke recipes

**DateEditor / DatePicker** — formatted fields auto-skip separators. Type digits
only (not slashes). For Sept 10, 1966 in MM/dd/yyyy format: `Home,09101966`.

**ColorPicker** — Tab between H/S/V sliders, use `CursorUp`/`CursorDown` to
adjust values.

**FileDialog** — type the path directly into the text field, then `Enter`.

**ListView / TableView** — `CursorUp`/`CursorDown` to navigate, `Enter` to
select.

### Notes on cast output noise

Terminal.Gui apps may emit ConfigurationManager warnings or other stderr output
on exit. This is normal and appears in the `.cast` file after the app closes.
It doesn't affect the GIF (the recording stops at process exit).
