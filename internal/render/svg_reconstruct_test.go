package render

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/bmmmm/epcii/internal/qr"
)

var (
	pathRe    = regexp.MustCompile(`<path d="([^"]*)" fill="#000"/>`)
	runRe     = regexp.MustCompile(`^M(\d+) (\d+)h(\d+)v1h-(\d+)z`)
	viewBoxRe = regexp.MustCompile(`viewBox="0 0 (\d+) (\d+)"`)
)

// TestSVGReconstruction parses the emitted path back into a module matrix and
// compares it against the source, for payloads spanning versions 1 to 13.
// The quiet-zone offset is hardcoded to 4 (ISO/IEC 18004 minimum for QR),
// so shrinking the QuietZone constant fails this test.
func TestSVGReconstruction(t *testing.T) {
	for _, n := range []int{1, 50, 150, 331} {
		payload := strings.Repeat("x", n)
		code, err := qr.EncodeM([]byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		svg := string(SVG(code.Matrix()))
		size := code.Size()

		vb := viewBoxRe.FindStringSubmatch(svg)
		if vb == nil {
			t.Fatalf("n=%d: no viewBox", n)
		}
		if want := strconv.Itoa(size + 8); vb[1] != want || vb[2] != want {
			t.Errorf("n=%d: viewBox %sx%s, want %s (size + 2*4 quiet zone)", n, vb[1], vb[2], want)
		}

		m := pathRe.FindStringSubmatch(svg)
		if m == nil {
			t.Fatalf("n=%d: no dark path found", n)
		}

		got := make([][]bool, size)
		for i := range got {
			got[i] = make([]bool, size)
		}
		d := m[1]
		for len(d) > 0 {
			run := runRe.FindStringSubmatch(d)
			if run == nil {
				t.Fatalf("n=%d: unparseable path remainder %q", n, d[:min(len(d), 40)])
			}
			x, _ := strconv.Atoi(run[1])
			y, _ := strconv.Atoi(run[2])
			w, _ := strconv.Atoi(run[3])
			if run[3] != run[4] {
				t.Fatalf("n=%d: run at (%d,%d) is not a rectangle: h%s vs h-%s", n, x, y, run[3], run[4])
			}
			for i := 0; i < w; i++ {
				mx, my := x+i-4, y-4
				if mx < 0 || my < 0 || mx >= size || my >= size {
					t.Fatalf("n=%d: dark module outside symbol at path (%d,%d)", n, x+i, y)
				}
				if got[my][mx] {
					t.Fatalf("n=%d: module (%d,%d) painted twice", n, mx, my)
				}
				got[my][mx] = true
			}
			d = d[len(run[0]):]
		}

		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if got[y][x] != code.Module(x, y) {
					t.Fatalf("n=%d: module (%d,%d) mismatch after SVG reconstruction", n, x, y)
				}
			}
		}
	}
}
