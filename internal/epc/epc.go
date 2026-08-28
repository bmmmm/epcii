// Package epc builds and validates EPC069-12 ("GiroCode") QR payloads for
// SEPA credit transfer initiation, payload format version 002.
package epc

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxPayloadBytes is the EPC069-12 limit on the total payload size.
const MaxPayloadBytes = 331

// Payment holds the beneficiary and transfer data for one EPC QR payload.
// All strings are UTF-8; none may contain line breaks.
type Payment struct {
	Name    string // beneficiary name, required, <=70 chars
	IBAN    string // beneficiary IBAN, required, mod-97 validated
	BIC     string // optional within the EEA, 8 or 11 chars
	Amount  string // normalized "580.00" form, optional; see NormalizeAmount
	Purpose string // optional SEPA purpose code (AT-T007), 1-4 alphanumeric
	Ref     string // structured creditor reference, <=35 chars; RF refs are ISO 11649 checked
	Text    string // unstructured remittance info, <=140 chars
	Info    string // beneficiary-to-originator information, <=70 chars
}

// Payload assembles and validates the EPC069-12 version 002 payload.
func (p Payment) Payload() (string, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return "", fmt.Errorf("beneficiary name is required")
	}
	if n := len([]rune(name)); n > 70 {
		return "", fmt.Errorf("beneficiary name is %d characters, limit is 70", n)
	}

	iban, err := ValidateIBAN(p.IBAN)
	if err != nil {
		return "", err
	}

	bic := strings.ToUpper(strings.ReplaceAll(p.BIC, " ", ""))
	if bic != "" {
		if err := validateBIC(bic); err != nil {
			return "", err
		}
	}

	amount := ""
	if p.Amount != "" {
		amount, err = NormalizeAmount(p.Amount)
		if err != nil {
			return "", err
		}
		amount = "EUR" + amount
	}

	// EPC069-12 section 2.2: purpose (AT-T007) is 1..4 alphanumeric.
	purpose := strings.ToUpper(strings.TrimSpace(p.Purpose))
	if len(purpose) > 4 {
		return "", fmt.Errorf("purpose code must be at most 4 characters, got %q", purpose)
	}
	for _, r := range purpose {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", fmt.Errorf("purpose code must be alphanumeric, got %q", purpose)
		}
	}

	if p.Ref != "" && p.Text != "" {
		return "", fmt.Errorf("structured reference and unstructured text are mutually exclusive")
	}
	ref := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(p.Ref), " ", ""))
	if n := len([]rune(ref)); n > 35 {
		return "", fmt.Errorf("structured reference is %d characters, limit is 35", n)
	}
	// An ISO 11649 creditor reference ("RF" + check digits) is mod-97
	// verifiable like an IBAN; other structured references pass as-is.
	if strings.HasPrefix(ref, "RF") && (len(ref) < 5 || !isUpperAlnum(ref) || mod97(ref[4:]+ref[:4]) != 1) {
		return "", fmt.Errorf("creditor reference %q failed the ISO 11649 mod-97 check", ref)
	}
	if n := len([]rune(p.Text)); n > 140 {
		return "", fmt.Errorf("remittance text is %d characters, limit is 140", n)
	}
	if n := len([]rune(p.Info)); n > 70 {
		return "", fmt.Errorf("beneficiary-to-originator info is %d characters, limit is 70", n)
	}

	fields := []struct {
		name  string
		value string
	}{
		{"service tag", "BCD"},
		{"version", "002"},
		{"character set", "1"}, // 1 = UTF-8
		{"identification", "SCT"},
		{"bic", bic},         // AT-C002, optional in version 002 (EEA)
		{"name", name},       // AT-E001 beneficiary name
		{"iban", iban},       // AT-C001 beneficiary IBAN
		{"amount", amount},   // AT-T002, optional
		{"purpose", purpose}, // AT-T007, optional
		{"ref", ref},         // AT-T009 structured remittance (exclusive with text)
		{"text", p.Text},     // AT-T009 unstructured remittance
		{"info", p.Info},     // beneficiary-to-originator information
	}
	values := make([]string, len(fields))
	for i, f := range fields {
		if strings.ContainsAny(f.value, "\r\n") {
			return "", fmt.Errorf("%s contains a line break", f.name)
		}
		// The payload declares character set 1 (UTF-8); reject bytes that are
		// not valid UTF-8 (e.g. Latin-1 argv from a mis-configured locale).
		if !utf8.ValidString(f.value) {
			return "", fmt.Errorf("%s is not valid UTF-8", f.name)
		}
		values[i] = f.value
	}

	payload := strings.Join(trimTrailingEmpty(values), "\n")
	if len(payload) > MaxPayloadBytes {
		return "", fmt.Errorf("payload is %d bytes, EPC069-12 limit is %d", len(payload), MaxPayloadBytes)
	}
	return payload, nil
}

// trimTrailingEmpty drops empty trailing elements: EPC069-12 marks the tail
// elements optional, and reference encoders (e.g. segno) omit empty trailing
// lines entirely rather than emitting bare separators.
func trimTrailingEmpty(fields []string) []string {
	end := len(fields)
	for end > 0 && fields[end-1] == "" {
		end--
	}
	return fields[:end]
}

func validateBIC(bic string) error {
	if len(bic) != 8 && len(bic) != 11 {
		return fmt.Errorf("BIC must be 8 or 11 characters, got %d", len(bic))
	}
	if !isUpperAlnum(bic) {
		return fmt.Errorf("BIC %q contains invalid characters", bic)
	}
	return nil
}

func isUpperAlnum(s string) bool {
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
