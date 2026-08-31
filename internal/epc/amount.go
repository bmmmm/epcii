package epc

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizeAmount parses a euro amount in English ("580.00", "1,234.56"),
// German ("580,00", "1.234,56"), or space-grouped ("1 234,56") notation and
// returns the canonical EPC form with a dot separator and exactly two
// decimals. All Unicode whitespace (incl. no-break and narrow spaces from
// copy-paste) is treated as a plain space. Valid range is 0.01 to
// 999999999.99.
func NormalizeAmount(in string) (string, error) {
	// Copy-pasted amounts carry NBSP/narrow-space thousands separators and
	// currency spacing; fold every whitespace rune to a plain space first.
	s := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, in)
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "EUR")
	s = strings.TrimSuffix(s, "€")
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("amount is empty")
	}

	// German notation: comma is the decimal separator, dots (or spaces) group
	// thousands. English notation: dot is decimal, commas (or spaces) group.
	// A comma after the last dot means German.
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")
	var intPart, fracPart string
	var err error
	switch {
	case lastComma == -1 && lastDot == -1:
		intPart, err = stripThousands(s, ' ') // bare "580" or "1 234"
	case lastComma > lastDot: // German: 1.234,56 or 580,5
		intPart, err = stripThousands(s[:lastComma], '.', ' ')
		fracPart = s[lastComma+1:]
	default: // English: 1,234.56 or 580.5
		intPart, err = stripThousands(s[:lastDot], ',', ' ')
		fracPart = s[lastDot+1:]
	}
	if err != nil {
		return "", fmt.Errorf("amount %q: %w", in, err)
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

// stripThousands removes thousands separators from the integer part after
// validating the grouping: at most one separator kind may be used, the first
// group has 1-3 digits and every following group exactly 3. Rejects garbage
// like "1.,2", "12.34,56", or "12 34,56".
func stripThousands(intRaw string, seps ...rune) (string, error) {
	var used rune
	for _, sep := range seps {
		if strings.ContainsRune(intRaw, sep) {
			if used != 0 {
				return "", fmt.Errorf("mixed thousands separators")
			}
			used = sep
		}
	}
	if used == 0 {
		return intRaw, nil
	}
	groups := strings.Split(intRaw, string(used))
	for i, g := range groups {
		if (i == 0 && (len(g) < 1 || len(g) > 3)) || (i > 0 && len(g) != 3) {
			return "", fmt.Errorf("malformed thousands grouping")
		}
	}
	return strings.Join(groups, ""), nil
}
