package epc

import (
	"fmt"
	"strings"
)

// ValidateIBAN normalizes an IBAN (strips spaces, uppercases) and verifies
// its structure and ISO 13616 mod-97 check digits. It returns the normalized
// form suitable for the payload.
func ValidateIBAN(iban string) (string, error) {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(iban), " ", ""))
	if s == "" {
		return "", fmt.Errorf("IBAN is required")
	}
	if len(s) < 15 || len(s) > 34 {
		return "", fmt.Errorf("IBAN length %d is outside the valid range 15-34", len(s))
	}
	for i, r := range s {
		switch {
		case i < 2 && (r < 'A' || r > 'Z'):
			return "", fmt.Errorf("IBAN must start with a two-letter country code")
		case i >= 2 && i < 4 && (r < '0' || r > '9'):
			return "", fmt.Errorf("IBAN check digits (positions 3-4) must be numeric")
		case (r < 'A' || r > 'Z') && (r < '0' || r > '9'):
			return "", fmt.Errorf("IBAN contains invalid character %q", r)
		}
	}
	if mod97(s[4:]+s[:4]) != 1 {
		return "", fmt.Errorf("IBAN failed the mod-97 checksum — check for typos")
	}
	return s, nil
}

// mod97 computes the ISO 7064 mod-97-10 remainder over the rearranged IBAN,
// mapping letters A-Z to 10-35, processing incrementally to avoid big-integer
// arithmetic.
func mod97(s string) int {
	rem := 0
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			v := int(r-'A') + 10
			rem = (rem*100 + v) % 97
		} else {
			rem = (rem*10 + int(r-'0')) % 97
		}
	}
	return rem
}
