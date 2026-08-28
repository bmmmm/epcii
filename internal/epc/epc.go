// Package epc builds and validates EPC069-12 ("GiroCode") QR payloads for
// SEPA credit transfer initiation, payload format version 002.
package epc

import (
	"fmt"
	"strings"
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
	Purpose string // optional 4-letter SEPA purpose code (AT-44)
	Ref     string // structured creditor reference (ISO 11649), <=35 chars
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

	if p.Ref != "" && p.Text != "" {
		return "", fmt.Errorf("structured reference and unstructured text are mutually exclusive")
	}
	if n := len([]rune(p.Ref)); n > 35 {
		return "", fmt.Errorf("structured reference is %d characters, limit is 35", n)
	}
	if n := len([]rune(p.Text)); n > 140 {
		return "", fmt.Errorf("remittance text is %d characters, limit is 140", n)
	}
	if n := len([]rune(p.Info)); n > 70 {
		return "", fmt.Errorf("beneficiary-to-originator info is %d characters, limit is 70", n)
	}

	fields := []string{
		"BCD",   // service tag
		"002",   // version
		"1",     // character set: UTF-8
		"SCT",   // identification: SEPA credit transfer
		bic,     // AT-C002 BIC, optional in version 002 (EEA)
		name,    // AT-E001 beneficiary name
		iban,    // AT-C001 beneficiary IBAN
		amount,  // AT-T002 amount, optional
		purpose, // AT-T007 purpose, optional
		p.Ref,   // AT-T009 structured remittance (exclusive with Text)
		p.Text,  // AT-T009 unstructured remittance
		p.Info,  // beneficiary-to-originator information
	}
	for i, f := range fields {
		if strings.ContainsAny(f, "\r\n") {
			return "", fmt.Errorf("field %d contains a line break", i+1)
		}
	}

	payload := strings.Join(trimTrailingEmpty(fields), "\n")
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
	for _, r := range bic {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return fmt.Errorf("BIC contains invalid character %q", r)
		}
	}
	return nil
}
