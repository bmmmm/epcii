// Package webapi is the browser-facing entry point: one pure function that
// runs the same pipeline as the CLI (validate → encode → render) and returns
// every artefact the page shows. It has no syscall/js dependency, so it is
// built and tested natively; cmd/epcii-wasm is the thin JS bridge on top.
package webapi

import (
	"bytes"

	"github.com/bmmmm/epcii/internal/epc"
	"github.com/bmmmm/epcii/internal/qr"
	"github.com/bmmmm/epcii/internal/render"
)

// pngScale is pixels per module for the PNG download. Keep equal to
// pngScale in main.go so the web PNG matches `epcii --png` byte for byte.
const pngScale = 8

// Input mirrors the CLI flags (see the flag table in main.go).
type Input struct {
	Name, IBAN, BIC, Amount, Purpose, Ref, Text, Info string
}

// Output is everything the page renders. On a validation or encoding error
// only Error is set; the other fields stay at their zero value.
type Output struct {
	Payload string // exact encoded EPC069-12 payload (LF-separated)
	Version int    // QR symbol version
	Size    int    // modules per side, without quiet zone
	SVG     string // identical to the CLI's stdout
	PNG     []byte // identical to the CLI's --png file
	Error   string // validation/encoding error text, without the "epcii:" prefix
}

// Generate runs the CLI pipeline (main.go run()) on in.
func Generate(in Input) Output {
	p := epc.Payment{
		Name: in.Name, IBAN: in.IBAN, BIC: in.BIC, Amount: in.Amount,
		Purpose: in.Purpose, Ref: in.Ref, Text: in.Text, Info: in.Info,
	}
	payload, err := p.Payload()
	if err != nil {
		return Output{Error: err.Error()}
	}
	code, err := qr.EncodeM([]byte(payload))
	if err != nil {
		return Output{Error: err.Error()}
	}
	matrix := code.Matrix()
	var png bytes.Buffer
	if err := render.PNG(&png, matrix, pngScale); err != nil {
		return Output{Error: err.Error()}
	}
	return Output{
		Payload: payload,
		Version: code.Version(),
		Size:    code.Size(),
		SVG:     string(render.SVG(matrix)),
		PNG:     png.Bytes(),
	}
}
