# Demo GIF: Mouse Pointer Indicator (UICatalog)

Recreates `artifacts/uicatalog-demo.gif` — demonstrates the `--mouse-pointer` feature
using Terminal.Gui's UICatalog sample application.

## Prerequisites

```sh
go build -o tuirec.exe ./cmd/tuirec      # Windows
go build -o tuirec    ./cmd/tuirec       # Unix
```

`agg` must be on PATH. UICatalog must be pre-built (the path below assumes a local
Terminal.Gui checkout; adjust as needed).

## Run (Windows)

```powershell
.\tuirec.exe record `
    --binary "C:\Users\Tig\source\repos\Terminal.Gui\Examples\UICatalog\bin\Debug\net10.0\UICatalog.exe" `
    --name uicatalog-demo `
    --title "Mouse Pointer Indicator Demo — UICatalog" `
    --keystrokes "wait:2000,click:40:1,wait:500,click:40:4,wait:1000,drag:60:8:80:8,drag:80:8:80:18,drag:80:18:40:18,drag:40:18:40:8,drag:40:8:60:8,wait:1000,hover:55:17,wait:500,hover:60:17,wait:500,hover:65:17,wait:2000,click:84:19,wait:500,Ctrl+Q" `
    --mouse-pointer all `
    --cols 120 --rows 30 `
    --open
```

## Script Breakdown

| Step | Token | What happens |
|------|-------|--------------|
| 1 | `click:40:1` | Click the "Help" menu in the menu bar |
| 2 | `click:40:4` | Click "About..." in the Help dropdown |
| 3 | `drag:60:8:80:8` | Start dragging dialog top border to the right |
| 4 | `drag:80:8:80:18` | Continue drag downward |
| 5 | `drag:80:18:40:18` | Continue drag to the left |
| 6 | `drag:40:18:40:8` | Continue drag upward |
| 7 | `drag:40:8:60:8` | Return dialog to original position |
| 8 | `hover:55:17` → `hover:65:17` | Hover over the GitHub link (shows tooltip) |
| 9 | `click:84:19` | Click the "OK" button to dismiss the dialog |
| 10 | `Ctrl+Q` | Quit UICatalog |

## What it demonstrates

- **Menu interaction**: Clicking menu items with the mouse pointer indicator visible
- **Dialog dragging**: The About dialog is dragged in a rectangular loop, showing
  the pointer tracking the drag motion with interpolated intermediate events
- **Hover/tooltip**: Hovering over a hyperlink triggers the tooltip to appear
- **Button click**: Clicking the OK button dismisses the dialog
- **Pointer visibility**: The yellow ● pointer appears at every mouse event position

## Flags explained

- `--mouse-pointer all` — show pointer on all mouse events (clicks + moves + drags)
- `--mouse-pointer clicks` — (default) only clicks, drags, scrolls
- `--mouse-pointer none` — disable pointer indicator
- `--pointer-style "►"` — use a different character (default `●`)

## Output

- `artifacts/uicatalog-demo.gif` — the animated GIF
- `artifacts/uicatalog-demo.cast` — the raw asciinema cast file
