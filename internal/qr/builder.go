// Derived from piglig/go-qr (https://github.com/piglig/go-qr),
// Copyright (c) 2023 piglig, MIT License — see NOTICE.
// Reduced for EPC QR needs: byte mode, error correction level M,
// versions 1-13, automatic mask selection.

package qr

// formatBitsM is the 2-bit format-information value for error correction
// level M (ISO/IEC 18004: L=1, M=0, Q=3, H=2).
const formatBitsM = 0

// builder is the mutable scaffold used to lay out a QR matrix. It owns the
// module grid plus the function-module map that drives codeword placement and
// masking; EncodeM drives it to completion and freezes it into a Code.
type builder struct {
	version    int
	size       int
	modules    [][]bool // dark/light state of every module
	isFunction [][]bool // true where a module belongs to a function pattern
}

func newBuilder(version int) *builder {
	size := version*4 + 17
	b := &builder{version: version, size: size}
	b.modules = make([][]bool, size)
	b.isFunction = make([][]bool, size)
	modBacking := make([]bool, size*size)
	fnBacking := make([]bool, size*size)
	for i := 0; i < size; i++ {
		b.modules[i] = modBacking[i*size : (i+1)*size]
		b.isFunction[i] = fnBacking[i*size : (i+1)*size]
	}
	return b
}

// setFunctionModule paints a module and marks it as part of a function pattern.
func (b *builder) setFunctionModule(x, y int, dark bool) {
	b.modules[y][x] = dark
	b.isFunction[y][x] = true
}

// drawFunctionPatterns draws all static patterns: timing, finders, alignment,
// and placeholders for format and version information.
func (b *builder) drawFunctionPatterns() {
	for i := 0; i < b.size; i++ {
		b.setFunctionModule(6, i, i%2 == 0)
		b.setFunctionModule(i, 6, i%2 == 0)
	}

	b.drawFinderPattern(3, 3)
	b.drawFinderPattern(b.size-4, 3)
	b.drawFinderPattern(3, b.size-4)

	alignPos := b.alignmentPatternPositions()
	numAlign := len(alignPos)
	for i := 0; i < numAlign; i++ {
		for j := 0; j < numAlign; j++ {
			// Skip the three corners occupied by finder patterns.
			if !(i == 0 && j == 0 || i == 0 && j == numAlign-1 || i == numAlign-1 && j == 0) {
				b.drawAlignmentPattern(alignPos[i], alignPos[j])
			}
		}
	}

	b.drawFormatBits(0)
	b.drawVersion()
}

// drawFinderPattern draws a finder pattern centered at (x, y), clipped at the
// symbol edges.
func (b *builder) drawFinderPattern(x, y int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			dist := max(abs(dx), abs(dy))
			xx, yy := x+dx, y+dy
			if 0 <= xx && xx < b.size && 0 <= yy && yy < b.size {
				b.setFunctionModule(xx, yy, dist != 2 && dist != 4)
			}
		}
	}
}

// drawAlignmentPattern draws an alignment pattern centered at (x, y).
func (b *builder) drawAlignmentPattern(x, y int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			b.setFunctionModule(x+dx, y+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

// alignmentPatternPositions returns the alignment pattern center coordinates.
// Empty for version 1. (The version-32 spacing irregularity is beyond the
// supported range and omitted.)
func (b *builder) alignmentPatternPositions() []int {
	if b.version == 1 {
		return nil
	}
	numAlign := b.version/7 + 2
	step := (b.version*4 + numAlign*2 + 1) / (numAlign*2 - 2) * 2

	res := make([]int, numAlign)
	res[0] = 6
	for i, pos := numAlign-1, b.size-7; i >= 1; i-- {
		res[i] = pos
		pos -= step
	}
	return res
}

// drawVersion encodes the version information blocks; only present for
// version >= 7.
func (b *builder) drawVersion() {
	if b.version < 7 {
		return
	}

	rem := b.version
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	bits := b.version<<12 | rem

	for i := 0; i < 18; i++ {
		bit := getBit(bits, i)
		a := b.size - 11 + i%3
		c := i / 3
		b.setFunctionModule(a, c, bit)
		b.setFunctionModule(c, a, bit)
	}
}

// drawFormatBits encodes ECC level M and the mask number into the two format
// information copies.
func (b *builder) drawFormatBits(mask int) {
	data := formatBitsM<<3 | mask
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	bits := (data<<10 | rem) ^ 0x5412

	for i := 0; i <= 5; i++ {
		b.setFunctionModule(8, i, getBit(bits, i))
	}
	b.setFunctionModule(8, 7, getBit(bits, 6))
	b.setFunctionModule(8, 8, getBit(bits, 7))
	b.setFunctionModule(7, 8, getBit(bits, 8))
	for i := 9; i < 15; i++ {
		b.setFunctionModule(14-i, 8, getBit(bits, i))
	}

	for i := 0; i < 8; i++ {
		b.setFunctionModule(b.size-1-i, 8, getBit(bits, i))
	}
	for i := 8; i < 15; i++ {
		b.setFunctionModule(8, b.size-15+i, getBit(bits, i))
	}
	b.setFunctionModule(8, b.size-8, true) // dark module
}

// addEccAndInterleave appends Reed-Solomon ECC bytes to the data codewords and
// interleaves the blocks according to the version's level-M layout.
func (b *builder) addEccAndInterleave(data []byte) []byte {
	numBlocks := numEccBlocksM[b.version]
	blockEccLen := eccCodewordsPerBlockM[b.version]
	rawCodewords := numRawDataModules(b.version) / 8
	numShortBlocks := numBlocks - rawCodewords%numBlocks
	shortBlockLen := rawCodewords / numBlocks

	blocks := make([][]byte, numBlocks)
	divisor := rsDivisor(blockEccLen)
	for i, k := 0, 0; i < numBlocks; i++ {
		extra := 0
		if i >= numShortBlocks {
			extra = 1
		}
		dat := data[k : k+shortBlockLen-blockEccLen+extra]
		k += len(dat)

		block := make([]byte, shortBlockLen+1)
		copy(block, dat)
		copy(block[len(block)-blockEccLen:], rsRemainder(dat, divisor))
		blocks[i] = block
	}

	res := make([]byte, rawCodewords)
	for i, k := 0, 0; i < len(blocks[0]); i++ {
		for j := 0; j < len(blocks); j++ {
			// Skip the padding slot of short blocks.
			if i != shortBlockLen-blockEccLen || j >= numShortBlocks {
				res[k] = blocks[j][i]
				k++
			}
		}
	}
	return res
}

// drawCodewords fills the non-function modules with the codeword bits
// following the QR zig-zag traversal.
func (b *builder) drawCodewords(data []byte) {
	i := 0
	for right := b.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < b.size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := vert
				if (right+1)&2 == 0 { // upward column
					y = b.size - 1 - vert
				}
				if !b.isFunction[y][x] && i < len(data)*8 {
					b.modules[y][x] = getBit(int(data[i>>3]), 7-(i&7))
					i++
				}
			}
		}
	}
}

// getBit returns the i-th bit (LSB-first) of x.
func getBit(x, i int) bool {
	return (x>>uint(i))&1 != 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
