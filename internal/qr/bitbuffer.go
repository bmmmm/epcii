// Derived from piglig/go-qr (https://github.com/piglig/go-qr),
// Copyright (c) 2023 piglig, MIT License — see NOTICE.
// Reduced for EPC QR needs: byte mode, error correction level M,
// versions 1-13, automatic mask selection.

package qr

// bitBuffer is an append-only sequence of bits, packed eight to a byte,
// most-significant bit first.
type bitBuffer struct {
	data []byte // packed bits; bit i lives at data[i/8], MSB-first
	n    int    // number of valid bits
}

// appendBits appends the low length bits of val, most-significant bit first.
// Callers guarantee 0 <= length <= 16 and val < 1<<length.
func (b *bitBuffer) appendBits(val, length int) {
	for i := length - 1; i >= 0; i-- {
		if b.n>>3 >= len(b.data) {
			b.data = append(b.data, 0)
		}
		if (val>>uint(i))&1 != 0 {
			b.data[b.n>>3] |= 0x80 >> uint(b.n&7)
		}
		b.n++
	}
}

// bytes returns the buffer content as whole bytes; the caller ensures the bit
// count is byte-aligned.
func (b *bitBuffer) bytes() []byte {
	return b.data[:b.n/8]
}
