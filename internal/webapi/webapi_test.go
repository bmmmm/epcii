package webapi

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bmmmm/epcii/internal/qr"
	"github.com/bmmmm/epcii/internal/render"
)

const testIBAN = "DE02120300000000202051"

func TestGenerateMatchesCLIPipeline(t *testing.T) {
	in := Input{Name: "Test Persona", IBAN: testIBAN, Amount: "12,5", Text: "invoice 42"}
	out := Generate(in)
	if out.Error != "" {
		t.Fatalf("unexpected error: %q", out.Error)
	}
	want := "BCD\n002\n1\nSCT\n\nTest Persona\n" + testIBAN + "\nEUR12.50\n\n\ninvoice 42"
	if out.Payload != want {
		t.Errorf("payload:\n got %q\nwant %q", out.Payload, want)
	}
	code, err := qr.EncodeM([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != code.Version() || out.Size != code.Size() {
		t.Errorf("geometry: got v%d %dx%d, want v%d %dx%d",
			out.Version, out.Size, out.Size, code.Version(), code.Size(), code.Size())
	}
	if out.SVG != string(render.SVG(code.Matrix())) {
		t.Error("SVG differs from render.SVG")
	}
	var png bytes.Buffer
	if err := render.PNG(&png, code.Matrix(), render.DefaultPNGScale); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.PNG, png.Bytes()) {
		t.Error("PNG differs from render.PNG at scale 8")
	}
}

func TestGenerateErrors(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want string // substring of the error, case-insensitive
	}{
		{"missing iban", Input{Name: "Test Persona"}, "iban"},
		{"ref and text", Input{Name: "Test Persona", IBAN: testIBAN, Ref: "RF18539007547034", Text: "x"}, "ref"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Generate(tc.in)
			if out.Error == "" {
				t.Fatal("expected an error")
			}
			if !strings.Contains(strings.ToLower(out.Error), tc.want) {
				t.Errorf("error %q does not mention %q", out.Error, tc.want)
			}
			if strings.HasPrefix(out.Error, "epcii:") {
				t.Errorf("error carries the CLI prefix: %q", out.Error)
			}
			if out.SVG != "" || out.PNG != nil || out.Payload != "" {
				t.Errorf("error output must be otherwise empty: %+v", out)
			}
		})
	}
}
