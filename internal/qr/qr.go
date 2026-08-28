// Derived from piglig/go-qr (https://github.com/piglig/go-qr),
// Copyright (c) 2023 piglig, MIT License — see NOTICE.
// Reduced for EPC QR needs: byte mode, error correction level M,
// versions 1-13, automatic mask selection.

// Package qr encodes binary data into a QR Code Model 2 module matrix.
// The scope is exactly what EPC069-12 requires: byte mode, error correction
// level M, versions 1-13 (331 payload bytes fit version 13-M exactly), and
// automatic mask selection per ISO/IEC 18004 penalty scoring. No ECI header
// is emitted; the payload's own character-set field declares the encoding.
package qr

import "fmt"

// MinVersion and MaxVersion delimit the supported QR version range.
// EPC069-12 caps symbols at version 13.
const (
	MinVersion = 1
	MaxVersion = 13
)

// Per-version tables for error correction level M, index 0 unused.
// Sliced from the ISO/IEC 18004 tables; other ECC levels are out of scope.
var (
	eccCodewordsPerBlockM = [MaxVersion + 1]int{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22}
	numEccBlocksM         = [MaxVersion + 1]int{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9}
)

// Code is an immutable, fully rendered QR Code symbol.
type Code struct {
	version int
	size    int
	mask    int
	modules [][]bool
}

// Size returns the side length of the symbol in modules.
func (c *Code) Size() int { return c.size }

// Module reports whether the module at (x, y) is dark.
func (c *Code) Module(x, y int) bool {
	return 0 <= x && x < c.size && 0 <= y && y < c.size && c.modules[y][x]
}

// Version returns the QR version (1-13) of the symbol.
func (c *Code) Version() int { return c.version }

// Matrix returns the module rows; the caller must not modify them.
func (c *Code) Matrix() [][]bool { return c.modules }

// EncodeM encodes data as a byte-mode QR symbol at error correction level M,
// choosing the smallest version that fits and the lowest-penalty mask.
func EncodeM(data []byte) (*Code, error) {
	version := 0
	for v := MinVersion; v <= MaxVersion; v++ {
		if 4+charCountBits(v)+8*len(data) <= dataCapacityBits(v) {
			version = v
			break
		}
	}
	if version == 0 {
		return nil, fmt.Errorf("qr: %d bytes exceed the version %d-M capacity of %d bytes",
			len(data), MaxVersion, dataCapacityBits(MaxVersion)/8-2)
	}

	// Assemble the data bit stream: mode indicator, character count, payload,
	// terminator, byte alignment, then alternating pad bytes (ISO 18004).
	capacity := dataCapacityBits(version)
	bb := &bitBuffer{}
	bb.appendBits(0x4, 4) // byte mode
	bb.appendBits(len(data), charCountBits(version))
	for _, x := range data {
		bb.appendBits(int(x), 8)
	}
	bb.appendBits(0, min(4, capacity-bb.n))
	bb.appendBits(0, (8-bb.n%8)%8)
	for pad := 0xEC; bb.n < capacity; pad ^= 0xEC ^ 0x11 {
		bb.appendBits(pad, 8)
	}

	b := newBuilder(version)
	b.drawFunctionPatterns()
	b.drawCodewords(b.addEccAndInterleave(bb.bytes()))
	mask := b.chooseBestMask()
	b.applyMask(mask)
	b.drawFormatBits(mask)

	return &Code{version: version, size: b.size, mask: mask, modules: b.modules}, nil
}

// charCountBits returns the byte-mode character count field width for a
// version: 8 bits for versions 1-9, 16 for 10 and up.
func charCountBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

// dataCapacityBits returns the number of data bits (excluding ECC) available
// at the given version for level M.
func dataCapacityBits(version int) int {
	dataCodewords := numRawDataModules(version)/8 -
		eccCodewordsPerBlockM[version]*numEccBlocksM[version]
	return dataCodewords * 8
}

// numRawDataModules returns the number of modules available for data + ECC
// codewords at the given version.
func numRawDataModules(version int) int {
	size := version*4 + 17
	res := size * size
	res -= 8 * 8 * 3       // three finder patterns incl. separators
	res -= 15*2 + 1        // format info + dark module
	res -= (size - 16) * 2 // timing patterns
	if version >= 2 {
		numAlign := version/7 + 2
		res -= (numAlign - 1) * (numAlign - 1) * 25 // alignment patterns
		res -= (numAlign - 2) * 2 * 20              // timing overlap
		if version >= 7 {
			res -= 6 * 3 * 2 // version info
		}
	}
	return res
}
