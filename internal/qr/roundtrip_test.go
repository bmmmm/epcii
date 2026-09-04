// This test is original epcii code, not derived from piglig/go-qr.

package qr

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	zxqrcode "github.com/makiuchi-d/gozxing/qrcode"
)

// codeImage rasterizes a Code at 4 px per module with a 4-module quiet zone,
// as an input for the independent gozxing decoder.
func codeImage(t *testing.T, c *Code) image.Image {
	t.Helper()
	const scale, quiet = 4, 4
	total := (c.Size() + 2*quiet) * scale
	img := image.NewGray(image.Rect(0, 0, total, total))
	for y := 0; y < total; y++ {
		for x := 0; x < total; x++ {
			mx := x/scale - quiet
			my := y/scale - quiet
			if c.Module(mx, my) {
				img.SetGray(x, y, color.Gray{})
			} else {
				img.SetGray(x, y, color.Gray{Y: 0xff})
			}
		}
	}
	return img
}

// decode runs the independent gozxing (ZXing port) decoder on the image and
// returns the text, interpreted as UTF-8 to match the EPC charset field.
func decode(t *testing.T, img image.Image) string {
	t.Helper()
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatalf("bitmap: %v", err)
	}
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_CHARACTER_SET: "UTF-8",
	}
	res, err := zxqrcode.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return res.GetText()
}

func TestRoundTrip(t *testing.T) {
	payloads := map[string]string{
		"minimal EPC":      "BCD\n002\n1\nSCT\n\nACME GmbH\nDE02120300000000202051",
		"umlauts":          "BCD\n002\n1\nSCT\n\nMüller & Söhne GmbH\nDE02120300000000202051\nEUR580.00\n\n\nRechnung RE-4a7f — Überweisung",
		"official example": "BCD\n002\n1\nSCT\nBNPAFRPP\nFrançois D'Alsace S.A.\nFR1420041010050500013M02606\nEUR12.3\n\n\nClient:Marie Louise La Lune",
		"short":            "x",
	}
	for name, payload := range payloads {
		c, err := EncodeM([]byte(payload))
		if err != nil {
			t.Errorf("%s: encode: %v", name, err)
			continue
		}
		if got := decode(t, codeImage(t, c)); got != payload {
			t.Errorf("%s: round-trip mismatch:\ngot:  %q\nwant: %q", name, got, payload)
		}
	}
}

func TestRoundTrip331BytesIsVersion13(t *testing.T) {
	// Build a payload of exactly 331 bytes — the EPC maximum, which must fit
	// version 13 exactly.
	base := "BCD\n002\n1\nSCT\n\nACME GmbH\nDE02120300000000202051\nEUR580.00\n\n\n"
	payload := base + strings.Repeat("x", 331-len(base))
	if len(payload) != 331 {
		t.Fatalf("test payload is %d bytes, want 331", len(payload))
	}

	c, err := EncodeM([]byte(payload))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if c.Version() != 13 {
		t.Errorf("331-byte payload encoded as version %d, want 13", c.Version())
	}
	if got := decode(t, codeImage(t, c)); got != payload {
		t.Error("331-byte round-trip mismatch")
	}

	if _, err := EncodeM(make([]byte, 332)); err == nil {
		t.Error("332 bytes must not fit version 13-M")
	}
}

// byteCapacityM is the largest byte-mode payload each version holds at level
// M: the ISO/IEC 18004 data codeword count for that version, less the 4-bit
// mode indicator and the character count field. Spelled out as literals on
// purpose — deriving the bound from dataCapacityBits would test the encoder
// against itself, and a wrong ECC table entry would move the bound with it.
var byteCapacityM = [MaxVersion + 1]int{
	-1, 14, 26, 42, 62, 84, 106, 122, 152, 180, 213, 251, 287, 331,
}

// TestRoundTripAllVersions exercises every version 1-13 at its exact byte
// capacity, guarding the per-version ECC tables: a wrong table entry makes
// the symbol undecodable or shifts version selection.
func TestRoundTripAllVersions(t *testing.T) {
	for v := MinVersion; v <= MaxVersion; v++ {
		maxBytes := byteCapacityM[v]
		if got := (dataCapacityBits(v) - 4 - charCountBits(v)) / 8; got != maxBytes {
			t.Errorf("version %d-M: encoder offers %d payload bytes, ISO/IEC 18004 says %d", v, got, maxBytes)
			continue
		}
		payload := strings.Repeat("a", maxBytes)
		c, err := EncodeM([]byte(payload))
		if err != nil {
			t.Fatalf("version %d (%d bytes): %v", v, maxBytes, err)
		}
		if c.Version() != v {
			t.Errorf("payload of %d bytes selected version %d, want %d", maxBytes, c.Version(), v)
			continue
		}
		if got := decode(t, codeImage(t, c)); got != payload {
			t.Errorf("version %d: round-trip mismatch", v)
		}
	}
}

func TestVersionSelection(t *testing.T) {
	// Data codeword counts per version at level M, from ISO/IEC 18004.
	wantDataCodewords := map[int]int{1: 16, 2: 28, 3: 44, 13: 334}
	for v, want := range wantDataCodewords {
		if got := dataCapacityBits(v) / 8; got != want {
			t.Errorf("version %d-M: %d data codewords, want %d", v, got, want)
		}
	}
	// 331 bytes + 2 header bytes + terminator fit version 13 (334 codewords)
	// and nothing smaller.
	c, err := EncodeM(make([]byte, 331))
	if err != nil {
		t.Fatal(err)
	}
	if c.Version() != 13 {
		t.Errorf("version %d, want 13", c.Version())
	}
}
