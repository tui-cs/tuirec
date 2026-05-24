# Investigation: VHS (charmbracelet) Renderer as agg Replacement

**Issue:** #56  
**Status:** Investigation complete — Path 1 (x/vt in tuirec) is viable  
**Date:** 2026-05-24

## Executive Summary

VHS itself **cannot** be used as a library (all `package main`, requires Chromium + ffmpeg). However, `charmbracelet/x/vt` — a pure-Go VT emulator — correctly handles emoji width and can serve as the foundation for a custom Go-native GIF renderer that replaces agg.

## Research Findings

### VHS Architecture (charmbracelet/vhs)

| Factor | Finding |
|--------|---------|
| Package structure | All `package main` — **not importable** |
| Rendering engine | Headless Chromium + xterm.js + canvas screenshots |
| GIF output | `ffmpeg` subprocess — no Go-native encoder |
| Runtime deps | ttyd + Chromium + ffmpeg (3 heavy binaries) |
| License | MIT ✅ |

**Verdict:** VHS is architecturally incompatible with tuirec's single-binary goal.

### charmbracelet/x/vt (VT Emulator Library)

| Factor | Finding |
|--------|---------|
| Importable | ✅ proper `package vt` |
| API | `NewEmulator(cols, rows)` → `Write([]byte)` → `CellAt(x, y)` |
| Wide char support | ✅ via `mattn/go-runewidth` + `rivo/uniseg` (grapheme clusters) |
| Cell model | `Cell{Content, Width, Style}` — full color/attr info |
| Unicode mode 2027 | ✅ supported via `WidthMethod()` toggle |
| License | MIT ✅ |

**Verdict:** Excellent VT emulator. Missing piece: no pixel/image rendering layer.

### Proof: x/vt Does NOT Have the #59 Bug

Three tests in `pkg/renderer/emoji_width_test.go` prove this:

1. **TestEmojiMixedASCII** — exact #59 scenario: 16 emoji (32 cols) + 88 ASCII dots at 120 cols, 3 rows auto-wrapped. **PASSES** — no tearing.
2. **TestAggDriftSimulation** — demonstrates agg's drift (16 cols/row) and proves x/vt is immune. **PASSES**.
3. **TestEmojiAutoWrap** — verifies wrap-before semantics for wide chars at terminal edge. **PASSES**.

## Path Analysis

### Path 1: Use x/vt in tuirec (RECOMMENDED)

Replace agg with a Go-native renderer:
1. `charmbracelet/x/vt` — VT emulator (parse cast → cell grid)
2. Custom rasterizer — cell grid → `image.RGBA` frames (needs font rendering)
3. `image/gif` — encode animated GIF (stdlib)

**Pros:**
- Single binary, zero external deps
- Fixes #59 (correct wcwidth via go-runewidth)
- Full control over rendering (themes, fonts, padding)
- Cross-platform without binary distribution headaches

**Cons:**
- Must implement font rasterization (~500-1000 LoC with `golang.org/x/image/font`)
- Need to embed a default monospace font (adds ~200KB to binary)
- Must handle all terminal attributes (bold, italic, underline, colors)

**Estimated work:** Medium — the VT emulator is free; the rendering layer is the effort.

### Path 2: Port tuirec to VHS (NOT RECOMMENDED)

Contribute tuirec's keystroke injection and cast recording to VHS upstream.

**Blockers:**
- VHS requires Chromium + ttyd + ffmpeg at runtime
- Fundamentally browser-based — tuirec is PTY-native
- VHS's tape format is different from tuirec's keystroke scripts
- VHS targets recording scripted demos; tuirec targets recording real apps
- Massive architectural mismatch for what amounts to adding 2 features

**Could work as:** a VHS plugin/mode proposal, but doesn't solve tuirec's goals.

## Recommended Next Steps

1. ✅ ~~Prove x/vt handles emoji correctly~~ (done)
2. Build `pkg/renderer` cast→cell-grid pipeline using x/vt
3. Add font rasterization layer (embed JetBrains Mono or similar)
4. Render #59's charmap-emoji scenario end-to-end
5. Compare output quality with agg (font, colors, file size)

## Dependencies Added

```
github.com/charmbracelet/x/vt (latest)
├── github.com/charmbracelet/ultraviolet (cell model)
├── github.com/charmbracelet/x/ansi (ANSI parser, width methods)
├── github.com/mattn/go-runewidth v0.0.23
├── github.com/rivo/uniseg v0.4.7
└── golang.org/x/sys (upgraded)
```
