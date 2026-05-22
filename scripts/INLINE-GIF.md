# Inline GIF: clet text (AppModel.Inline)

Produces `artifacts/inline-demo.gif` — demonstrates `--inline` mode recording
with `clet text`, a Terminal.Gui inline-mode input clet.

## Prerequisites

```sh
go build -o tuirec.exe ./cmd/tuirec      # Windows
go build -o tuirec    ./cmd/tuirec       # Unix
```

`agg` must be on PATH. `clet` must be installed:

```sh
dotnet tool install -g clet
```

## Run (Windows)

```powershell
.\tuirec.exe record `
    --binary clet `
    --args "text","--prompt","What is your name?" `
    --name inline-demo `
    --title "Inline Mode Demo — clet text" `
    --show-command "$ clet text --prompt `"What is your name?`"" `
    --inline `
    --keystrokes "wait:1000,`Jane Doe`,wait:500,Enter,wait:1000" `
    --cols 80 --rows 10 `
    --open
```

## Run (Unix)

```sh
./tuirec record \
    --binary clet \
    --args 'text,--prompt,What is your name?' \
    --name inline-demo \
    --title 'Inline Mode Demo — clet text' \
    --show-command '$ clet text --prompt "What is your name?"' \
    --inline \
    --keystrokes 'wait:1000,`Jane Doe`,wait:500,Enter,wait:1000' \
    --cols 80 --rows 10 \
    --open
```

## Script Breakdown

| Step | Token | What happens |
|------|-------|--------------|
| 1 | `wait:1000` | Let the inline prompt render |
| 2 | `` `Jane Doe` `` | Type the answer |
| 3 | `wait:500` | Pause to show typed text |
| 4 | `Enter` | Submit the input |
| 5 | `wait:1000` | Show the result output |

## What it demonstrates

- **Inline mode**: The app renders below the shell prompt, not fullscreen
- **Prompt → TUI → result flow**: The GIF shows the realistic terminal experience:
  1. `$ clet text ...` prompt typed at the top
  2. The inline TUI input appears below
  3. After Enter, the result appears below the input
- **Small terminal frame**: `--rows 10` keeps the GIF compact since only a
  few rows are needed for inline mode

## Flags explained

- `--inline` — skip alternate screen buffer; record in normal screen mode
- `--show-command` — synthetic prompt typed before the app starts
- `--cols 80 --rows 10` — smaller frame appropriate for inline apps

## Output

- `artifacts/inline-demo.gif` — the animated GIF
- `artifacts/inline-demo.cast` — the raw asciinema cast file
