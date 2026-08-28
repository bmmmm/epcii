// Package render turns a QR module matrix into SVG, PNG, and terminal output.
package render

import (
	"fmt"
	"strings"
)

// QuietZone is the mandatory light border around the symbol, in modules
// (ISO/IEC 18004; EPC069-12 keeps the default of 4).
const QuietZone = 4

// SVG renders the module matrix as a standalone SVG document. The viewBox is
// in module units including the quiet zone, so the image scales losslessly.
func SVG(modules [][]bool) []byte {
	n := len(modules)
	total := n + 2*QuietZone

	var path strings.Builder
	for y, row := range modules {
		for x := 0; x < n; {
			if !row[x] {
				x++
				continue
			}
			run := 0
			for x+run < n && row[x+run] {
				run++
			}
			fmt.Fprintf(&path, "M%d %dh%dv1h-%dz", x+QuietZone, y+QuietZone, run, run)
			x += run
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`+"\n", total, total)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/>`+"\n", total, total)
	fmt.Fprintf(&b, `<path d="%s" fill="#000"/>`+"\n", path.String())
	b.WriteString("</svg>\n")
	return []byte(b.String())
}
