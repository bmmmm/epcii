// Command qrfixtures regenerates internal/qr/testdata/matrix_fingerprints.txt
// from the upstream encoder piglig/go-qr, the reference that internal/qr was
// extracted from. It lives in its own Go module so that the upstream package
// never enters the main module's dependency graph.
//
// The fixture pins, for every payload length 0..331, the QR version and a
// SHA-256 fingerprint of the module matrix that upstream produces with the
// exact parameters internal/qr implements: byte mode, error correction level
// M, versions 1-13, automatic mask selection, no ECC boosting. The test in
// internal/qr/fingerprint_test.go compares EncodeM against it.
//
// Usage (from the repository root):
//
//	go run -C scripts/qrfixtures . > internal/qr/testdata/matrix_fingerprints.txt
//
// Regenerating is only legitimate when the upstream pin in go.mod changes or
// the fixture payload rule changes — never to make a red test green.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	goqr "github.com/piglig/go-qr"
)

const (
	maxPayloadBytes = 331 // EPC069-12 limit; fits QR version 13-M exactly
	minVersion      = 1
	maxVersion      = 13
)

// fixturePayload mirrors fixturePayload in internal/qr/fingerprint_test.go:
// the EPC header lines followed by filler up to n bytes. Both copies must
// stay identical, otherwise every fingerprint mismatches at once.
func fixturePayload(n int) []byte {
	const header = "BCD\n002\n1\nSCT\n"
	if n <= len(header) {
		return []byte(header[:n])
	}
	b := make([]byte, n)
	copy(b, header)
	for i := len(header); i < n; i++ {
		b[i] = 'A'
	}
	return b
}

func main() {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	fmt.Fprintln(w, "# QR matrix fingerprints from piglig/go-qr (see scripts/qrfixtures).")
	fmt.Fprintln(w, "# Columns: payload length, QR version, SHA-256 of the row-major module")
	fmt.Fprintln(w, "# bitmap (one byte per module, 1 = dark). Byte mode, ECC level M,")
	fmt.Fprintln(w, "# versions 1-13, automatic mask, boostEcl=false.")
	for n := 0; n <= maxPayloadBytes; n++ {
		seg, err := goqr.MakeBytes(fixturePayload(n))
		if err != nil {
			fmt.Fprintf(os.Stderr, "length %d: %v\n", n, err)
			os.Exit(1)
		}
		code, err := goqr.EncodeSegments([]*goqr.QrSegment{seg}, goqr.Medium, minVersion, maxVersion, -1, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "length %d: %v\n", n, err)
			os.Exit(1)
		}
		size := code.Size()
		h := sha256.New()
		row := make([]byte, size)
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				row[x] = 0
				if code.Module(x, y) {
					row[x] = 1
				}
			}
			h.Write(row)
		}
		fmt.Fprintf(w, "%d %d %s\n", n, (size-17)/4, hex.EncodeToString(h.Sum(nil)))
	}
}
