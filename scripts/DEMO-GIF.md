# Demo GIF: Mouse Pointer Indicator

Recreates the `artifacts/mouse-pointer-demo.gif` showing the `--mouse-pointer` feature.

## Prerequisites

```sh
go build -o tuirec.exe ./cmd/tuirec      # Windows
go build -o tuirec    ./cmd/tuirec       # Unix
go build -o testapp.exe ./internal/testapp  # Windows
go build -o testapp    ./internal/testapp  # Unix
```

`agg` must be on PATH or auto-downloaded by tuirec.

## Run (Windows)

```powershell
.\tuirec.exe record `
    --binary .\testapp.exe `
    --name mouse-pointer-demo `
    --title "Mouse Pointer Indicator Demo" `
    --keystrokes "wait:1000,click:15:5,wait:500,click:30:8,wait:500,move:50:3,wait:300,move:60:10,wait:300,click:40:12,wait:500,drag:5:5:25:5,wait:500,scroll:down:20:10,wait:500,Ctrl+Q" `
    --mouse-pointer all `
    --cols 80 --rows 20 `
    --startup-delay 500 `
    --open
```

## Run (Unix)

```bash
./tuirec record \
    --binary ./testapp \
    --name mouse-pointer-demo \
    --title "Mouse Pointer Indicator Demo" \
    --keystrokes "wait:1000,click:15:5,wait:500,click:30:8,wait:500,move:50:3,wait:300,move:60:10,wait:300,click:40:12,wait:500,drag:5:5:25:5,wait:500,scroll:down:20:10,wait:500,Ctrl+Q" \
    --mouse-pointer all \
    --cols 80 --rows 20 \
    --startup-delay 500 \
    --open
```

## What it demonstrates

| Timestamp | Event | What you see |
|-----------|-------|--------------|
| ~1s | `click:15:5` | Yellow ● appears at column 15, row 5 |
| ~2s | `click:30:8` | ● moves to col 30, row 8 (old position cleared) |
| ~2.5s | `move:50:3` | ● moves to col 50, row 3 (hover — only visible with `--mouse-pointer all`) |
| ~3s | `move:60:10` | ● moves to col 60, row 10 |
| ~3.5s | `click:40:12` | ● moves to col 40, row 12 |
| ~4s | `drag:5:5:25:5` | ● appears at drag endpoint (col 25, row 5) |
| ~4.5s | `scroll:down:20:10` | ● appears at scroll position (col 20, row 10) |
| ~5s | `Ctrl+Q` | App exits |

## Flags explained

- `--mouse-pointer all` — show pointer on all mouse events (clicks + moves)
- `--mouse-pointer clicks` — (default) only clicks, drags, scrolls
- `--mouse-pointer none` — disable pointer indicator
- `--pointer-style "►"` — use a different character (default `●`)

## Output

- `artifacts/mouse-pointer-demo.gif` — the animated GIF
- `artifacts/mouse-pointer-demo.cast` — the raw asciinema cast file
