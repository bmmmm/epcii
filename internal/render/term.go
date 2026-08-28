package render

import (
	"strings"
)

// Terminal renders the module matrix as a terminal preview using Unicode
// half blocks (two module rows per text line). Colors are set explicitly via
// ANSI codes so the polarity is correct on both light and dark terminals.
// The two-module quiet zone keeps the preview compact; it is a preview, not
// a print artifact — the SVG/PNG outputs carry the full four-module zone.
func Terminal(modules [][]bool) string {
	const quiet = 2
	n := len(modules)
	total := n + 2*quiet

	at := func(x, y int) bool {
		x -= quiet
		y -= quiet
		if x < 0 || y < 0 || x >= n || y >= n {
			return false
		}
		return modules[y][x]
	}

	// ▀ paints the top half in the foreground color and the bottom half in
	// the background color: foreground = top module, background = bottom.
	color := func(dark bool, codes [2]string) string {
		if dark {
			return codes[0]
		}
		return codes[1]
	}
	fg := [2]string{"30", "37"} // black / white foreground
	bg := [2]string{"40", "47"} // black / white background

	var b strings.Builder
	for y := 0; y < total; y += 2 {
		for x := 0; x < total; x++ {
			b.WriteString("\x1b[" + color(at(x, y), fg) + ";" + color(at(x, y+1), bg) + "m▀")
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String()
}
