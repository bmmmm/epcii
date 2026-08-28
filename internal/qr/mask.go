// Derived from piglig/go-qr (https://github.com/piglig/go-qr),
// Copyright (c) 2023 piglig, MIT License — see NOTICE.
// Reduced for EPC QR needs: byte mode, error correction level M,
// versions 1-13, automatic mask selection.

package qr

import "math"

// Penalty weights for the four ISO/IEC 18004 masking evaluation rules.
const (
	penaltyN1 = 3
	penaltyN2 = 3
	penaltyN3 = 40
	penaltyN4 = 10
)

// maskInvert reports whether mask pattern mask flips the module at (x, y).
func maskInvert(mask, x, y int) bool {
	switch mask {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (x/3+y/2)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	case 7:
		return ((x+y)%2+x*y%3)%2 == 0
	}
	return false
}

// applyMask XORs mask pattern mask onto the non-function modules. XOR is
// self-inverse, so applying the same mask twice restores the previous state.
func (b *builder) applyMask(mask int) {
	for y := 0; y < b.size; y++ {
		for x := 0; x < b.size; x++ {
			b.modules[y][x] = b.modules[y][x] != (maskInvert(mask, x, y) && !b.isFunction[y][x])
		}
	}
}

// chooseBestMask scores all eight mask patterns (with their format bits drawn,
// as the spec requires) and returns the lowest-penalty one. The grid is left
// in its unmasked state; the caller applies the winning mask.
func (b *builder) chooseBestMask() int {
	best, minPenalty := 0, math.MaxInt32
	for mask := 0; mask < 8; mask++ {
		b.applyMask(mask)
		b.drawFormatBits(mask)
		if p := b.penaltyScore(); p < minPenalty {
			best, minPenalty = mask, p
		}
		b.applyMask(mask) // undo
	}
	return best
}

// penaltyScore evaluates the four ISO/IEC 18004 masking penalty rules on the
// current module grid: runs of same-colored modules (N1), 2x2 blocks (N2),
// finder-like 1:1:3:1:1 patterns (N3), and dark-module balance (N4).
func (b *builder) penaltyScore() int {
	res := 0
	size := b.size
	var runHistory [7]int
	dark := 0

	// Horizontal pass, fused with the 2x2-block rule and the dark tally.
	for y := 0; y < size; y++ {
		row := b.modules[y]
		var next []bool
		if y+1 < size {
			next = b.modules[y+1]
		}
		runColor, runX := false, 0
		runHistory = [7]int{}
		for x := 0; x < size; x++ {
			cell := row[x]
			if cell {
				dark++
			}
			if cell == runColor {
				runX++
				if runX == 5 {
					res += penaltyN1
				} else if runX > 5 {
					res++
				}
			} else {
				b.finderPenaltyAddHistory(runX, &runHistory)
				if !runColor {
					res += finderPenaltyCountPatterns(&runHistory) * penaltyN3
				}
				runColor = cell
				runX = 1
			}
			if next != nil && x+1 < size &&
				cell == row[x+1] && cell == next[x] && cell == next[x+1] {
				res += penaltyN2
			}
		}
		res += b.finderPenaltyTerminateAndCount(runColor, runX, &runHistory) * penaltyN3
	}

	// Vertical pass for the run and finder-pattern rules.
	for x := 0; x < size; x++ {
		runColor, runY := false, 0
		runHistory = [7]int{}
		for y := 0; y < size; y++ {
			cell := b.modules[y][x]
			if cell == runColor {
				runY++
				if runY == 5 {
					res += penaltyN1
				} else if runY > 5 {
					res++
				}
			} else {
				b.finderPenaltyAddHistory(runY, &runHistory)
				if !runColor {
					res += finderPenaltyCountPatterns(&runHistory) * penaltyN3
				}
				runColor = cell
				runY = 1
			}
		}
		res += b.finderPenaltyTerminateAndCount(runColor, runY, &runHistory) * penaltyN3
	}

	// Dark-module balance: 10 points per 5% deviation from 50%.
	total := size * size
	k := (abs(dark*20-total*10)+total-1)/total - 1
	return res + k*penaltyN4
}

// finderPenaltyCountPatterns checks whether the run history matches the
// 1:1:3:1:1 finder ratio with sufficient light borders.
func finderPenaltyCountPatterns(runHistory *[7]int) int {
	n := runHistory[1]
	core := n > 0 && runHistory[2] == n && runHistory[3] == n*3 && runHistory[4] == n && runHistory[5] == n

	res := 0
	if core && runHistory[0] >= n*4 && runHistory[6] >= n {
		res = 1
	}
	if core && runHistory[6] >= n*4 && runHistory[0] >= n {
		res++
	}
	return res
}

// finderPenaltyTerminateAndCount finalizes a row/column run history and
// returns the residual finder penalty.
func (b *builder) finderPenaltyTerminateAndCount(currentRunColor bool, currentRunLen int, runHistory *[7]int) int {
	if currentRunColor {
		b.finderPenaltyAddHistory(currentRunLen, runHistory)
		currentRunLen = 0
	}
	currentRunLen += b.size // light border counts as an implicit light run
	b.finderPenaltyAddHistory(currentRunLen, runHistory)
	return finderPenaltyCountPatterns(runHistory)
}

// finderPenaltyAddHistory shifts the run-length history and prepends the
// current run length.
func (b *builder) finderPenaltyAddHistory(currentRunLen int, runHistory *[7]int) {
	if runHistory[0] == 0 {
		currentRunLen += b.size
	}
	copy(runHistory[1:], runHistory[:6])
	runHistory[0] = currentRunLen
}
