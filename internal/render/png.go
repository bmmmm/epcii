package render

import (
	"image"
	"image/color"
	"image/png"
	"io"
)

// DefaultPNGScale is the pixels-per-module scale the CLI (--png) and the web
// version share, so both produce the same file: 69 modules + quiet zone
// => 616 px at most.
const DefaultPNGScale = 8

// PNG renders the module matrix as a PNG with the given pixels-per-module
// scale, including the quiet zone.
func PNG(w io.Writer, modules [][]bool, scale int) error {
	if scale < 1 {
		scale = 1
	}
	n := len(modules)
	total := (n + 2*QuietZone) * scale
	img := image.NewGray(image.Rect(0, 0, total, total))
	white := color.Gray{Y: 0xff}
	for y := 0; y < total; y++ {
		for x := 0; x < total; x++ {
			img.SetGray(x, y, white)
		}
	}
	for y, row := range modules {
		for x, dark := range row {
			if !dark {
				continue
			}
			px := (x + QuietZone) * scale
			py := (y + QuietZone) * scale
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.SetGray(px+dx, py+dy, color.Gray{})
				}
			}
		}
	}
	return png.Encode(w, img)
}
