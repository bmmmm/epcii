// Derived from piglig/go-qr (https://github.com/piglig/go-qr),
// Copyright (c) 2023 piglig, MIT License — see NOTICE.
// Reduced for EPC QR needs: byte mode, error correction level M,
// versions 1-13, automatic mask selection.

package qr

// Reed-Solomon error correction over GF(2^8) with the QR Code field
// polynomial 0x11D — encoder side only (generator polynomial + remainder).

var (
	gfExp [512]byte // gfExp[i] = generator^i (doubled length to avoid mod on multiply)
	gfLog [256]byte // gfLog[x] = i such that generator^i = x
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x = gfMultiplyBitwise(x, 0x02)
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
}

// gfMultiplyBitwise multiplies two field elements via the Russian-peasant
// method. It does not depend on the log tables, so it bootstraps them.
func gfMultiplyBitwise(x, y int) int {
	z := 0
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11D)
		z ^= ((y >> i) & 1) * x
	}
	return z
}

// gfMul multiplies two field elements via the log tables.
func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// rsDivisor derives the Reed-Solomon generator polynomial of the given degree
// (the QR spec uses degrees up to 30).
func rsDivisor(degree int) []byte {
	res := make([]byte, degree)
	res[degree-1] = 1

	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := 0; j < len(res); j++ {
			res[j] = gfMul(res[j], root)
			if j+1 < len(res) {
				res[j] ^= res[j+1]
			}
		}
		root = gfMul(root, 0x02)
	}
	return res
}

// rsRemainder computes the Reed-Solomon ECC remainder of data for a generator
// polynomial (divisor). The returned slice has len(divisor) bytes.
func rsRemainder(data, divisor []byte) []byte {
	res := make([]byte, len(divisor))
	for _, b := range data {
		factor := b ^ res[0]
		copy(res, res[1:])
		res[len(res)-1] = 0
		for i := 0; i < len(res); i++ {
			res[i] ^= gfMul(divisor[i], factor)
		}
	}
	return res
}
