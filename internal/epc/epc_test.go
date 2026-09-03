package epc

import (
	"strings"
	"testing"
)

func TestValidateIBAN(t *testing.T) {
	valid := []string{
		"DE02120300000000202051", // German test IBAN
		"DE89370400440532013000",
		"de89 3704 0044 0532 0130 00", // spaces + lowercase normalize
		"GB82WEST12345698765432",
		"FR1420041010050500013M02606",
		"DE89 3704 0044 0532 0130 00",      // no-break spaces (PDF copy-paste)
		"DE89 3704 0044 0532 0130 00",      // narrow no-break spaces
		"\tDE89 3704\t0044 0532 0130 00\n", // tabs and trailing newline
		// TR has no entry in sepaIBANLength, so these two synthetic IBANs sit
		// on the inner edges of the generic 15-34 range with nothing but that
		// range deciding them.
		"TR4412030000000",                    // 15 characters, the lower bound
		"TR28120300000000002020511203000020", // 34 characters, the upper bound
	}
	for _, in := range valid {
		got, err := ValidateIBAN(in)
		if err != nil {
			t.Errorf("ValidateIBAN(%q) unexpected error: %v", in, err)
			continue
		}
		if strings.Contains(got, " ") || got != strings.ToUpper(got) {
			t.Errorf("ValidateIBAN(%q) = %q, not normalized", in, got)
		}
	}

	invalid := map[string]string{
		"":                            "empty",
		"DE02120300000000202052":      "wrong check digit",
		"DE0212030000000020205":       "21 chars: inside the 15-34 range, so the DE length rule (22) is what rejects it",
		"DE021203000000002020511":     "checksum fails",
		"XX0000000000000":             "checksum fails",
		"D102120300000000202051":      "country code not letters",
		"DEAB120300000000202051":      "check digits not numeric",
		"DE02-1203-0000-0000-2020-51": "invalid characters",
		"DE311203000000002020":        "valid mod-97 but wrong length for DE (20 != 22)",
		"AT611904300234573201123":     "valid-looking but wrong length for AT (23 != 20)",
		// Both pass mod-97 and belong to a country outside sepaIBANLength, so
		// the generic 15-34 range check is the only rule that can reject them.
		"TR371203000000":                      "14 characters, one below the range",
		"TR941203000000000020205112030000202": "35 characters, one above the range",
	}
	for in, why := range invalid {
		if _, err := ValidateIBAN(in); err == nil {
			t.Errorf("ValidateIBAN(%q) should fail (%s)", in, why)
		}
	}
}

func TestNormalizeAmount(t *testing.T) {
	good := map[string]string{
		"580.00":       "580.00",
		"580,00":       "580.00",
		"580":          "580.00",
		"580.5":        "580.50",
		"0,5":          "0.50",
		",5":           "0.50",
		"1.234,56":     "1234.56",
		"1,234.56":     "1234.56",
		"EUR 580,00":   "580.00",
		"580,00 €":     "580.00",
		"0.01":         "0.01",
		"999999999.99": "999999999.99",
		"0580.00":      "580.00",
		"1,37":         "1.37",
		"1.37":         "1.37",
		"1 234,56":     "1234.56",
		"1 234 567.89": "1234567.89",
		"1 234,56":     "1234.56", // no-break space thousands separator
		" 1,37 €":      "1.37",    // NBSP-padded copy-paste
		"1 234,56":     "1234.56", // narrow no-break space (Apple/CH)
	}
	for in, want := range good {
		got, err := NormalizeAmount(in)
		if err != nil {
			t.Errorf("NormalizeAmount(%q) unexpected error: %v", in, err)
		} else if got != want {
			t.Errorf("NormalizeAmount(%q) = %q, want %q", in, got, want)
		}
	}

	bad := []string{"", "0.00", "0", "12.345", "abc", "12a", "1000000000.00", "1e3", "-5",
		"1.,2", "1,.2", "12.34,56", "1.2345,00", "1,,2"}
	for _, in := range bad {
		if got, err := NormalizeAmount(in); err == nil {
			t.Errorf("NormalizeAmount(%q) = %q, should fail", in, got)
		}
	}
}

func TestPayloadFull(t *testing.T) {
	p := Payment{
		Name:   "François D'Alsace S.A.",
		IBAN:   "FR1420041010050500013M02606",
		BIC:    "BNPAFRPP",
		Amount: "12.30",
		Text:   "Client:Marie Louise La Lune",
	}
	got, err := p.Payload()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"BCD", "002", "1", "SCT",
		"BNPAFRPP",
		"François D'Alsace S.A.",
		"FR1420041010050500013M02606",
		"EUR12.30",
		"", "",
		"Client:Marie Louise La Lune",
	}, "\n")
	if got != want {
		t.Errorf("payload mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestPayloadMinimalTrimsTrailing(t *testing.T) {
	p := Payment{Name: "ACME GmbH", IBAN: "DE02120300000000202051"}
	got, err := p.Payload()
	if err != nil {
		t.Fatal(err)
	}
	want := "BCD\n002\n1\nSCT\n\nACME GmbH\nDE02120300000000202051"
	if got != want {
		t.Errorf("payload mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
	if strings.HasSuffix(got, "\n") {
		t.Error("payload must not end with a trailing newline")
	}
}

func TestPayloadUmlauts(t *testing.T) {
	p := Payment{
		Name:   "Müller & Söhne GmbH",
		IBAN:   "DE02120300000000202051",
		Amount: "580,00",
		Text:   "Rechnung RE-4a7f — Überweisung",
	}
	got, err := p.Payload()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Müller & Söhne GmbH") {
		t.Error("UTF-8 name must pass through unchanged")
	}
	if !strings.Contains(got, "EUR580.00") {
		t.Error("German comma amount must normalize to EUR580.00")
	}
}

func TestPayloadErrors(t *testing.T) {
	base := Payment{Name: "X", IBAN: "DE02120300000000202051"}

	cases := map[string]Payment{
		"missing name":     {IBAN: base.IBAN},
		"missing iban":     {Name: "X"},
		"name too long":    {Name: strings.Repeat("a", 71), IBAN: base.IBAN},
		"text too long":    {Name: "X", IBAN: base.IBAN, Text: strings.Repeat("a", 141)},
		"ref too long":     {Name: "X", IBAN: base.IBAN, Ref: strings.Repeat("a", 36)},
		"info too long":    {Name: "X", IBAN: base.IBAN, Info: strings.Repeat("a", 71)},
		"ref and text":     {Name: "X", IBAN: base.IBAN, Ref: "RF18539007547034", Text: "hi"},
		"bad bic":          {Name: "X", IBAN: base.IBAN, BIC: "TOOLONGBIC12"},
		"bad purpose":      {Name: "X", IBAN: base.IBAN, Purpose: "TOOLONG"},
		"newline in field": {Name: "X\nY", IBAN: base.IBAN},
		"bad amount":       {Name: "X", IBAN: base.IBAN, Amount: "abc"},
	}
	for name, p := range cases {
		if _, err := p.Payload(); err == nil {
			t.Errorf("%s: expected error, got none", name)
		}
	}
}

func TestPayloadValidationDetails(t *testing.T) {
	base := Payment{Name: "X", IBAN: "DE02120300000000202051"}

	bad := map[string]Payment{
		"purpose not alphanumeric": {Name: "X", IBAN: base.IBAN, Purpose: "a$b"},
		"purpose multibyte":        {Name: "X", IBAN: base.IBAN, Purpose: "ÄÖÜ"},
		"name not UTF-8":           {Name: "M\xfcller GmbH", IBAN: base.IBAN},
		"text not UTF-8":           {Name: "X", IBAN: base.IBAN, Text: "\xff\xfe"},
		"RF ref bad check digits":  {Name: "X", IBAN: base.IBAN, Ref: "RF19539007547034"},
		"RF ref bad characters":    {Name: "X", IBAN: base.IBAN, Ref: "RF18-53900754"},
		// A bare CR is a line break too: a reader that treats CR as EOL would
		// see a different payload than one that only splits on LF.
		"bare CR in name": {Name: "Erika\rMustermann", IBAN: base.IBAN},
		"bare CR in text": {Name: "X", IBAN: base.IBAN, Text: "Invoice\r2026"},
		"CRLF in info":    {Name: "X", IBAN: base.IBAN, Info: "line\r\nbreak"},
	}
	for name, p := range bad {
		if _, err := p.Payload(); err == nil {
			t.Errorf("%s: expected error, got none", name)
		}
	}

	good := map[string]Payment{
		"valid RF ref":  {Name: "X", IBAN: base.IBAN, Ref: "RF18539007547034"},
		"non-RF ref":    {Name: "X", IBAN: base.IBAN, Ref: "INV-2026-001"},
		"purpose alnum": {Name: "X", IBAN: base.IBAN, Purpose: "gdds"},
	}
	for name, p := range good {
		if _, err := p.Payload(); err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}
	}

	// Normalization: name is trimmed, BIC and purpose are uppercased.
	p := Payment{Name: "  ACME GmbH  ", IBAN: base.IBAN, BIC: "bnpafrpp", Purpose: "gdds"}
	payload, err := p.Payload()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(payload, "\n")
	if lines[4] != "BNPAFRPP" {
		t.Errorf("BIC not uppercased: %q", lines[4])
	}
	if lines[5] != "ACME GmbH" {
		t.Errorf("name not trimmed: %q", lines[5])
	}
	if lines[8] != "GDDS" {
		t.Errorf("purpose not uppercased: %q", lines[8])
	}
}

func TestPayloadByteLimit(t *testing.T) {
	// name 70 + text 140 + info 70 chars of 2-byte runes blows past 331 bytes
	p := Payment{
		Name: strings.Repeat("ü", 70),
		IBAN: "DE02120300000000202051",
		Text: strings.Repeat("ü", 140),
		Info: strings.Repeat("ü", 70),
	}
	if _, err := p.Payload(); err == nil {
		t.Error("expected payload byte-limit error")
	} else if !strings.Contains(err.Error(), "331") {
		t.Errorf("expected 331-byte-limit error, got: %v", err)
	}
}

// TestPayloadByteLimitBoundary pins the limit itself rather than a payload
// that overshoots it by hundreds of bytes: 331 bytes must pass and 332 must
// not. The fixed part is 67 bytes (BCD/002/1/SCT, an 11-char BIC, a 22-char
// IBAN, "EUR580.00", "GDDS" and the 11 LF separators), so name+text+info of
// 264 ASCII characters hit the limit exactly.
func TestPayloadByteLimitBoundary(t *testing.T) {
	build := func(infoLen int) Payment {
		return Payment{
			Name:    strings.Repeat("N", 70),
			IBAN:    "DE02120300000000202051",
			BIC:     "COBADEFFXXX",
			Amount:  "580.00",
			Purpose: "GDDS",
			Text:    strings.Repeat("T", 140),
			Info:    strings.Repeat("I", infoLen),
		}
	}

	got, err := build(54).Payload()
	if err != nil {
		t.Fatalf("a %d-byte payload must be accepted, got: %v", MaxPayloadBytes, err)
	}
	if len(got) != MaxPayloadBytes {
		t.Fatalf("payload is %d bytes, the boundary case needs exactly %d", len(got), MaxPayloadBytes)
	}

	if over, err := build(55).Payload(); err == nil {
		t.Errorf("a %d-byte payload must be rejected, got %d bytes of payload", MaxPayloadBytes+1, len(over))
	} else if !strings.Contains(err.Error(), "331") {
		t.Errorf("byte-limit error must name the 331-byte limit, got: %v", err)
	}
}

// TestPayloadReference covers the structured reference (AT-T009): ISO 11649
// "RF" references are normalized and checked, everything else is an issuer's
// own scheme and must survive untouched.
func TestPayloadReference(t *testing.T) {
	base := Payment{Name: "Erika Mustermann", IBAN: "DE02120300000000202051"}
	// Field order is BCD/version/charset/SCT/bic/name/iban/amount/purpose/ref.
	const refLine = 9

	accepted := map[string]struct{ in, want string }{
		"lowercase invoice number keeps its case":  {"inv-2026-abc", "inv-2026-abc"},
		"inner spaces of a non-RF ref survive":     {"Invoice 2026 001", "Invoice 2026 001"},
		"invoice number starting with rf survives": {"rfid-77", "rfid-77"},
		"surrounding whitespace is trimmed":        {"  INV-2026-001  ", "INV-2026-001"},
		"pasted RF ref is upper-cased and joined":  {"rf18 5390 0754 7034", "RF18539007547034"},
		"RF ref at the 25-character maximum":       {"RF39539007547034539007547", "RF39539007547034539007547"},
	}
	for name, tc := range accepted {
		p := base
		p.Ref = tc.in
		payload, err := p.Payload()
		if err != nil {
			t.Errorf("%s: Ref %q: unexpected error: %v", name, tc.in, err)
			continue
		}
		lines := strings.Split(payload, "\n")
		if len(lines) <= refLine {
			t.Errorf("%s: Ref %q: payload carries no reference line: %q", name, tc.in, payload)
			continue
		}
		if lines[refLine] != tc.want {
			t.Errorf("%s: Ref %q: reference line = %q, want %q", name, tc.in, lines[refLine], tc.want)
		}
	}

	// Each rejected case also pins which rule spoke, so a future rewrite
	// cannot collapse them into one blanket "invalid reference".
	rejected := map[string]struct{ in, wantMsg string }{
		"RF marker with no body":         {"RF", "at least 5"},
		"RF marker plus one character":   {"RF1", "at least 5"},
		"check digits are letters":       {"RFAA14", "check digits"},
		"26 characters, mod-97 still ok": {"RF635390075470345390075470", "limit is 25"},
		"29 characters, mod-97 still ok": {"RF255390075470345390075470345", "limit is 25"},
		"punctuation inside an RF ref":   {"RF18-53900754", "A-Z and 0-9"},
		"wrong check digits":             {"RF19539007547034", "mod-97"},
	}
	for name, tc := range rejected {
		p := base
		p.Ref = tc.in
		payload, err := p.Payload()
		if err == nil {
			t.Errorf("%s: Ref %q: expected an error, got payload %q", name, tc.in, payload)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantMsg) {
			t.Errorf("%s: Ref %q: error %q must name the failing rule (%q)", name, tc.in, err, tc.wantMsg)
		}
	}

	// A whitespace-only reference is no reference: it must not trip the
	// exclusivity rule, and it must leave the payload's ref field empty.
	blank := base
	blank.Ref = "   "
	blank.Text = "Invoice RE-2026-001"
	payload, err := blank.Payload()
	if err != nil {
		t.Fatalf("blank reference with text: unexpected error: %v", err)
	}
	want := "BCD\n002\n1\nSCT\n\nErika Mustermann\nDE02120300000000202051\n\n\n\nInvoice RE-2026-001"
	if payload != want {
		t.Errorf("blank reference with text:\ngot:  %q\nwant: %q", payload, want)
	}

	// The exclusivity rule itself still fires for a real reference.
	both := base
	both.Ref = "RF18539007547034"
	both.Text = "Invoice RE-2026-001"
	if _, err := both.Payload(); err == nil {
		t.Error("reference and text together must be rejected")
	} else if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected a mutual-exclusion error, got: %v", err)
	}
}

// TestValidateBIC pins the ISO 9362 structure: four letters for the
// institution, two for the country, then alphanumerics.
func TestValidateBIC(t *testing.T) {
	valid := []string{
		"COBADEFF",    // 8-character BIC
		"COBADEFFXXX", // 11-character BIC with the XXX head-office branch
		"MARKDEF1100", // digit in the location code, digits in the branch
		"BNPAFRPP",
	}
	for _, bic := range valid {
		if err := validateBIC(bic); err != nil {
			t.Errorf("validateBIC(%q) unexpected error: %v", bic, err)
		}
	}

	invalid := map[string]string{
		"00000000":     "all digits: no institution or country code",
		"1234DE2X":     "institution code must be letters",
		"COBA2EFF":     "country code must be letters",
		"COBADEF":      "7 characters",
		"TOOLONGBIC12": "12 characters",
		"COBA-EFF":     "not alphanumeric",
	}
	for bic, why := range invalid {
		if err := validateBIC(bic); err == nil {
			t.Errorf("validateBIC(%q) should fail (%s)", bic, why)
		}
	}

	// The structural rule must be reachable through Payload, not just here.
	p := Payment{Name: "Erika Mustermann", IBAN: "DE02120300000000202051", BIC: "00000000"}
	if _, err := p.Payload(); err == nil {
		t.Error("Payload accepted the placeholder BIC 00000000")
	}
}
