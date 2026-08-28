package epc

import (
	"fmt"
	"strings"
)

// NormalizeAmount parses a euro amount in English ("580.00", "1234.5") or
// German ("580,00", "1.234,56") notation and returns the canonical EPC form
// with a dot separator and exactly two decimals. Valid range is 0.01 to
// 999999999.99.
func NormalizeAmount(in string) (string, error) {
	s := strings.TrimSpace(in)
	s = strings.TrimPrefix(s, "EUR")
	s = strings.TrimSuffix(s, "€")
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("amount is empty")
	}

	// German notation: comma is the decimal separator, dots are thousands
	// separators. English notation: dot is decimal, commas are thousands.
	// A comma after the last dot means German; otherwise treat dots > 1 or a
	// trailing comma-group as ambiguous only when digits don't line up.
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")
	var intPart, fracPart string
	switch {
	case lastComma == -1 && lastDot == -1:
		intPart = s
	case lastComma > lastDot: // German: 1.234,56 or 580,5
		intPart = strings.ReplaceAll(s[:lastComma], ".", "")
		fracPart = s[lastComma+1:]
	default: // English: 1,234.56 or 580.5
		intPart = strings.ReplaceAll(s[:lastDot], ",", "")
		fracPart = s[lastDot+1:]
	}

	if intPart == "" {
		intPart = "0"
	}
	if len(fracPart) > 2 {
		return "", fmt.Errorf("amount %q has more than two decimal places", in)
	}
	for len(fracPart) < 2 {
		fracPart += "0"
	}
	for _, part := range []string{intPart, fracPart} {
		for _, r := range part {
			if r < '0' || r > '9' {
				return "", fmt.Errorf("amount %q is not a valid number", in)
			}
		}
	}

	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		intPart = "0"
	}
	if len(intPart) > 9 {
		return "", fmt.Errorf("amount %q exceeds the maximum of 999999999.99", in)
	}
	if intPart == "0" && fracPart == "00" {
		return "", fmt.Errorf("amount must be at least 0.01")
	}
	return intPart + "." + fracPart, nil
}
