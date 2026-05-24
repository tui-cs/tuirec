// Package fonts embeds the CaskaydiaCove Nerd Font for use by the renderer.
package fonts

import _ "embed"

// CaskaydiaCoveRegular is the embedded CaskaydiaCove Nerd Font (Regular).
// It provides full Unicode coverage including line-drawing, box-drawing,
// and emoji glyphs.
//
//go:embed CaskaydiaCoveNerdFont-Regular.ttf
var CaskaydiaCoveRegular []byte
