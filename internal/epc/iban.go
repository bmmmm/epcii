package epc

import (
	"fmt"
	"strings"
	"unicode"
)

// sepaIBANLength maps SEPA-participant country codes to their official IBAN
// length. Sources: EPC409-09 v8.0 (EPC List of SEPA Scheme Countries,
// 24 Dec 2025) for membership, SWIFT IBAN Registry Release 102 (Jun 2026) for
// the lengths; verified 2026-09-04. AL/MD/ME/MK went live 5 Oct 2025, RS was
// admitted in May 2025 and went live 5 May 2026. Territories without an IBAN
// code of their own ride on their country's (Åland on FI, the French overseas
// departments on FR, Jersey/Guernsey/Isle of Man on GB); Gibraltar is the only
// one with its own code. The Faroe Islands (FO) and Greenland (GL) have IBAN
// codes but are not SEPA participants, hence absent. Countries not listed are
// rejected: an EPC QR code initiates a SEPA credit transfer, which cannot
// reach them.
var sepaIBANLength = map[string]int{
	"AD": 24, "AL": 28, "AT": 20, "BE": 16, "BG": 22, "CH": 21, "CY": 28,
	"CZ": 24, "DE": 22, "DK": 18, "EE": 20, "ES": 24, "FI": 18, "FR": 27,
	"GB": 22, "GI": 23, "GR": 27, "HR": 21, "HU": 28, "IE": 22, "IS": 26,
	"IT": 27, "LI": 21, "LT": 20, "LU": 20, "LV": 21, "MC": 27, "MD": 24,
	"ME": 22, "MK": 19, "MT": 31, "NL": 18, "NO": 15, "PL": 28, "PT": 25,
	"RO": 24, "RS": 22, "SE": 24, "SI": 19, "SK": 24, "SM": 27, "VA": 22,
}

// ValidateIBAN normalizes an IBAN (strips all whitespace, uppercases) and
// verifies its structure, that its country takes part in SEPA, the
// country-specific length, and the ISO 13616 mod-97 check digits. It returns
// the normalized form suitable for the payload. Every Unicode whitespace rune
// is dropped, since IBANs copied from PDFs and banking portals carry no-break
// and narrow spaces between the digit groups.
func ValidateIBAN(iban string) (string, error) {
	s := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, iban)
	if s == "" {
		return "", fmt.Errorf("IBAN is required")
	}
	// Non-ASCII is named before the case fold; see asciiUpper.
	s, err := asciiUpper("IBAN", s)
	if err != nil {
		return "", err
	}
	if len(s) < 4 {
		return "", fmt.Errorf("IBAN %q is too short to carry a country code and check digits", s)
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
	want, ok := sepaIBANLength[s[:2]]
	if !ok {
		return "", fmt.Errorf("IBAN country %s is not a SEPA participant; an EPC QR code initiates a SEPA credit transfer", s[:2])
	}
	if len(s) != want {
		return "", fmt.Errorf("a %s IBAN has %d characters, got %d", s[:2], want, len(s))
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
