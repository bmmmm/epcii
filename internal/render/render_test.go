package render

import (
	"bytes"
	"image/png"
	"strconv"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	zxqrcode "github.com/makiuchi-d/gozxing/qrcode"

	"github.com/bmmmm/epcii/internal/qr"
)

const testPayload = "BCD\n002\n1\nSCT\n\nMüller & Söhne GmbH\nDE02120300000000202051\nEUR580.00\n\n\nRE-4a7f Rechnung"

func encode(t *testing.T) *qr.Code {
	t.Helper()
	c, err := qr.EncodeM([]byte(testPayload))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPNGRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := PNG(&buf, encode(t).Matrix(), 8); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_CHARACTER_SET: "UTF-8",
	}
	res, err := zxqrcode.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		t.Fatalf("decode rendered PNG: %v", err)
	}
	if res.GetText() != testPayload {
		t.Errorf("PNG round-trip mismatch:\ngot:  %q\nwant: %q", res.GetText(), testPayload)
	}
}

func TestSVGStructure(t *testing.T) {
	code := encode(t)
	svg := string(SVG(code.Matrix()))

	total := strconv.Itoa(code.Size() + 2*QuietZone)
	wantViewBox := `viewBox="0 0 ` + total + " " + total + `"`
	for _, want := range []string{wantViewBox, `fill="#fff"`, `fill="#000"`, "</svg>"} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG missing %q", want)
		}
	}
	if strings.Count(svg, "<path") != 1 {
		t.Error("SVG should contain exactly one path element")
	}
}

func TestTerminalShape(t *testing.T) {
	code := encode(t)
	out := Terminal(code.Matrix())

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	wantLines := (code.Size() + 2*2 + 1) / 2 // two module rows per line, quiet zone 2
	if len(lines) != wantLines {
		t.Errorf("terminal preview has %d lines, want %d", len(lines), wantLines)
	}
	for i, l := range lines {
		if !strings.HasSuffix(l, "\x1b[0m") {
			t.Errorf("line %d does not reset ANSI colors", i)
		}
	}
}
