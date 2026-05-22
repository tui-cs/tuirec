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
$ks = 'wait:1000,`This is a test of the Emergency Broadcast System...`,wait:500,click:4:7,wait:3000'
.\tuirec.exe record `
    --binary clet `
    "--args=text" `
    --name inline-demo `
    --title "Inline Mode Demo — clet text" `
    "--show-command=$ clet text" `
    --inline `
    --kitty-keyboard `
    "--keystrokes=$ks" `
    --cols 80 --rows 10 `
    --keystroke-delay 20 `
    --drain 3000 `
    --open
```

## Run (Unix)

```sh
./tuirec record \
    --binary clet \
    '--args=text' \
    --name inline-demo \
    --title 'Inline Mode Demo — clet text' \
    '--show-command=$ clet text' \
    --inline \
    --kitty-keyboard \
    '--keystrokes=wait:1000,`This is a test of the Emergency Broadcast System...`,wait:500,click:4:7,wait:3000' \
    --cols 80 --rows 10 \
    --keystroke-delay 20 \
    --drain 3000 \
    --open
```

## Script Breakdown

| Step | Token | What happens |
|------|-------|--------------|
| 1 | `wait:1000` | Let the inline text editor render |
| 2 | `` `This is a test of the Emergency Broadcast System...` `` | Type the text |
| 3 | `wait:500` | Pause to show typed text |
| 4 | `click:4:7` | Click the OK button to accept |
| 5 | `wait:3000` | Show the result output on the command line |

## What it demonstrates

- **Inline mode**: The app renders below the shell prompt, not fullscreen
- **Prompt → TUI → result flow**: The GIF shows the realistic terminal experience:
  1. `$ clet text --title Name:` prompt typed at the top
  2. The inline text editor appears below with an OK button
  3. User types text and clicks OK
  4. After acceptance, the result text appears below on the command line
- **Small terminal frame**: `--rows 10` keeps the GIF compact since only a
  few rows are needed for inline mode

## Flags explained

- `--inline` — skip alternate screen buffer; record in normal screen mode
- `--show-command` — synthetic prompt typed before the app starts
- `--kitty-keyboard` — enable Kitty keyboard protocol (required by clet)
- `--cols 80 --rows 10` — smaller frame appropriate for inline apps
- `--drain 3000` — keep recording after keystrokes to capture exit output

## Output

- `artifacts/inline-demo.gif` — the animated GIF
- `artifacts/inline-demo.cast` — the raw asciinema cast file
