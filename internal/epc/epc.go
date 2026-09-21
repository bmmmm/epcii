// Package epc builds and validates EPC069-12 ("GiroCode") QR payloads for
// SEPA credit transfer initiation, payload format version 002.
package epc

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxPayloadBytes is the EPC069-12 limit on the total payload size.
const MaxPayloadBytes = 331

// Payment holds the beneficiary and transfer data for one EPC QR payload.
// All strings are UTF-8; none may contain a line break, a control character
// or an invisible format character (see Payload).
type Payment struct {
	Name    string // beneficiary name, required, <=70 chars
	IBAN    string // beneficiary IBAN, required, mod-97 validated
	BIC     string // optional within the EEA, 8 or 11 chars
	Amount  string // normalized "580.00" form, optional; see NormalizeAmount
	Purpose string // optional SEPA purpose code (AT-T007), 1-4 alphanumeric
	Ref     string // structured creditor reference, <=35 chars; RF refs are normalized and ISO 11649 checked, others pass through verbatim
	Text    string // unstructured remittance info, <=140 chars
	Info    string // beneficiary-to-originator information, <=70 chars
}

// Payload assembles and validates the EPC069-12 version 002 payload.
func (p Payment) Payload() (string, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return "", fmt.Errorf("beneficiary name is required")
	}
	if err := tooLong("beneficiary name", name, 70); err != nil {
		return "", err
	}

	iban, err := ValidateIBAN(p.IBAN)
	if err != nil {
		return "", err
	}

	bic, err := asciiUpper("BIC", strings.ReplaceAll(p.BIC, " ", ""))
	if err != nil {
		return "", err
	}
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
	purpose, err := asciiUpper("purpose code", strings.TrimSpace(p.Purpose))
	if err != nil {
		return "", err
	}
	if len(purpose) > 4 {
		return "", fmt.Errorf("purpose code must be at most 4 characters, got %q", purpose)
	}
	for _, r := range purpose {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", fmt.Errorf("purpose code must be alphanumeric, got %q", purpose)
		}
	}

	ref := strings.TrimSpace(p.Ref)
	if ref != "" && p.Text != "" {
		return "", fmt.Errorf("structured reference and unstructured text are mutually exclusive")
	}
	// An ISO 11649 creditor reference ("RF" + two check digits) is normalized
	// — banks print it in groups of four — and then mod-97 verified like an
	// IBAN. Every other structured reference belongs to an issuer's own
	// scheme, where case and inner spacing may carry meaning, so it passes
	// through untouched.
	if norm := strings.ToUpper(strings.ReplaceAll(ref, " ", "")); isISO11649Claim(norm) {
		ref = norm
		if err := validateCreditorReference(ref); err != nil {
			return "", err
		}
	}
	if err := tooLong("structured reference", ref, 35); err != nil {
		return "", err
	}
	if err := tooLong("remittance text", p.Text, 140); err != nil {
		return "", err
	}
	if err := tooLong("beneficiary-to-originator info", p.Info, 70); err != nil {
		return "", err
	}

	fields := []struct {
		name  string
		value string
	}{
		{"service tag", "BCD"},
		{"version", "002"},
		{"character set", "1"}, // 1 = UTF-8
		{"identification", "SCT"},
		{"BIC", bic},                               // AT-C002, optional in version 002 (EEA)
		{"beneficiary name", name},                 // AT-E001
		{"IBAN", iban},                             // AT-C001
		{"amount", amount},                         // AT-T002, optional
		{"purpose code", purpose},                  // AT-T007, optional
		{"structured reference", ref},              // AT-T009 structured remittance (exclusive with text)
		{"remittance text", p.Text},                // AT-T009 unstructured remittance
		{"beneficiary-to-originator info", p.Info}, // the labels match tooLong's, so every error names a field the same way
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
		// Nothing invisible may enter a payment field. A control character
		// (ESC above all) drives the terminal that displays the --details
		// verification view; a format character — bidi override, zero-width
		// space, BOM — makes a beneficiary name read differently from how it
		// is stored. EPC069-12 does not contemplate either in these fields.
		if r, kind := invisibleRune(f.value); kind != "" {
			return "", fmt.Errorf("%s contains %s U+%04X", f.name, kind, r)
		}
		values[i] = f.value
	}

	payload := strings.Join(trimTrailingEmpty(values), "\n")
	if len(payload) > MaxPayloadBytes {
		hint := ""
		if hasCombiningMarks(payload) {
			hint = combiningHint
		}
		return "", fmt.Errorf("payload is %d bytes, EPC069-12 limit is %d%s", len(payload), MaxPayloadBytes, hint)
	}
	return payload, nil
}

// combiningHint explains a character or byte overrun caused by decomposed
// Unicode: text pasted from macOS surfaces often carries "u" plus a combining
// diaeresis instead of "ü", and every mark counts — against the field limits
// and against the 331-byte budget alike. NFC normalization would need
// golang.org/x/text at runtime, which the zero-dependency contract rules out,
// so the errors name the cause instead.
const combiningHint = "; the text contains combining marks (decomposed Unicode from copy-paste), each of which counts — retype the accented letters"

func hasCombiningMarks(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.M, r) {
			return true
		}
	}
	return false
}

// invisibleRune returns the first control (Cc) or format (Cf) rune in s with
// a description for the error message, or kind "" when there is none.
// Whitespace other than CR/LF/TAB is not invisible in this sense: a no-break
// space shows as a space and IBAN normalization strips it anyway.
func invisibleRune(s string) (rune, string) {
	for _, r := range s {
		switch {
		case unicode.IsControl(r):
			return r, "a control character"
		case unicode.Is(unicode.Cf, r):
			return r, "an invisible format character"
		}
	}
	return 0, ""
}

// asciiUpper upper-cases an identifier that is ASCII by definition — BIC,
// purpose code, IBAN. Anything outside ASCII is refused by name before any
// folding: strings.ToUpper turns U+017F (long s) into S and U+0131 (dotless
// i) into I, so a pasted homoglyph would otherwise be validated — and, for
// the checksum-free BIC, encoded — as a value the user never typed.
func asciiUpper(field, s string) (string, error) {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return "", fmt.Errorf("%s contains a non-ASCII character U+%04X (%q); only A-Z and 0-9 are valid", field, r, r)
		}
	}
	return strings.ToUpper(s), nil
}

// tooLong reports a character-limit violation for a text field.
func tooLong(field, s string, limit int) error {
	n := utf8.RuneCountInString(s)
	if n <= limit {
		return nil
	}
	hint := ""
	if hasCombiningMarks(s) {
		hint = combiningHint
	}
	return fmt.Errorf("%s is %d characters, limit is %d%s", field, n, limit, hint)
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

// isISO11649Claim reports whether a normalized reference presents itself as
// an ISO 11649 creditor reference: the "RF" marker followed by its two check
// digits. Issuer schemes that merely start with those letters ("rfid-77",
// "RFQ2026001") are not claims and pass through untouched; a claim is held
// to the full standard.
func isISO11649Claim(ref string) bool {
	return len(ref) >= 4 && ref[0] == 'R' && ref[1] == 'F' && isASCIIDigit(ref[2]) && isASCIIDigit(ref[3])
}

// validateCreditorReference checks a normalized ISO 11649 creditor reference
// ("RF" + two check digits, as isISO11649Claim guarantees): 1..21 further
// alphanumerics, 25 characters at most, verified with the mod-97 scheme used
// for IBANs.
func validateCreditorReference(ref string) error {
	if len(ref) < 5 {
		return fmt.Errorf("creditor reference %q is %d characters, ISO 11649 requires at least 5", ref, len(ref))
	}
	if !isUpperAlnum(ref) {
		return fmt.Errorf("creditor reference %q may only contain A-Z and 0-9", ref)
	}
	if len(ref) > 25 {
		return fmt.Errorf("creditor reference %q is %d characters, ISO 11649 limit is 25", ref, len(ref))
	}
	if mod97(ref[4:]+ref[:4]) != 1 {
		return fmt.Errorf("creditor reference %q failed the ISO 11649 mod-97 check", ref)
	}
	return nil
}

// validateBIC checks an ISO 9362 business identifier code: four letters for
// the institution, two letters for the country, two alphanumerics for the
// location and an optional three-character branch code.
func validateBIC(bic string) error {
	if len(bic) != 8 && len(bic) != 11 {
		return fmt.Errorf("BIC must be 8 or 11 characters, got %d", len(bic))
	}
	if !isUpperAlnum(bic) {
		return fmt.Errorf("BIC %q contains invalid characters", bic)
	}
	for i := 0; i < 6; i++ {
		if bic[i] < 'A' || bic[i] > 'Z' {
			return fmt.Errorf("BIC %q: institution and country code (first six characters) must be letters", bic)
		}
	}
	return nil
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

func isUpperAlnum(s string) bool {
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
