# Inline GIF: clet text (AppModel.Inline)

Produces `artifacts/inline-demo.gif` — demonstrates inline-mode recording
with `clet text`, a Terminal.Gui inline-mode input clet. The recording
launches a real shell so the app appears inline below the prompt, not
fullscreen.

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
$ks = 'wait:1000,`clet text`,wait:200,Enter,wait:2000,`This is a test of the Emergency Broadcast System...`,wait:500,click:4:9,wait:3000'
.\tuirec.exe record `
    --binary cmd `
    "--args=/d,/k,prompt $$ " `
    --name inline-demo `
    --title "Inline Mode Demo — clet text" `
    "--keystrokes=$ks" `
    --cols 80 --rows 12 `
    --keystroke-delay 20 `
    --drain 2000 `
    --open
```

## Run (Unix)

```sh
./tuirec record \
    --binary bash \
    --args '--norc' \
    --name inline-demo \
    --title 'Inline Mode Demo — clet text' \
    '--keystrokes=wait:1000,`clet text`,wait:200,Enter,wait:2000,`This is a test of the Emergency Broadcast System...`,wait:500,click:4:9,wait:3000' \
    --cols 80 --rows 12 \
    --keystroke-delay 20 \
    --drain 2000 \
    --open
```

## Script Breakdown

| Step | Token | What happens |
|------|-------|--------------|
| 1 | `wait:1000` | Let the shell prompt render |
| 2 | `` `clet text` `` | Type the command |
| 3 | `Enter` | Execute it — clet renders inline below the prompt |
| 4 | `wait:2000` | Let the inline text editor render |
| 5 | `` `This is a test of the Emergency Broadcast System...` `` | Type the answer |
| 6 | `wait:500` | Pause to show typed text |
| 7 | `click:4:9` | Click the OK button to accept |
| 8 | `wait:3000` | Show the result output on the command line |

## What it demonstrates

- **Inline mode via real shell**: The app is launched inside a shell, so it
  renders inline below the prompt — not fullscreen
- **Prompt → TUI → result flow**: The GIF shows the realistic terminal experience:
  1. `$ clet text` typed at the shell prompt
  2. The inline text editor appears below
  3. User types text and clicks OK
  4. Result text appears on the command line, prompt returns

## Output

- `artifacts/inline-demo.gif` — the animated GIF
- `artifacts/inline-demo.cast` — the raw asciinema cast file
