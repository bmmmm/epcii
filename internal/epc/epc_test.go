package epc

import (
	"strings"
	"testing"
	"unicode"
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
		"NO9386011117947",                  // shortest SEPA length (15)
		"MT84MALT011000012345MTLCAST001S",  // longest SEPA length (31)
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
		"DE0212030000000020205":       "21 chars: the DE length rule (22) rejects it before mod-97 runs",
		"DE0":                         "too short to carry check digits",
		"D":                           "one letter: shorter than the country code the guard slices",
		"DE021203000000002020511":     "checksum fails",
		"XX0000000000000":             "unknown country, rejected before mod-97 runs",
		"D102120300000000202051":      "country code not letters",
		"DEAB120300000000202051":      "check digits not numeric",
		"DE02-1203-0000-0000-2020-51": "invalid characters",
		"DE311203000000002020":        "valid mod-97 but wrong length for DE (20 != 22)",
		"AT611904300234573201123":     "valid-looking but wrong length for AT (23 != 20)",
		// Both pass mod-97; only the SEPA membership rule can reject them.
		"SA0380000000608010167519": "valid registry example, SA is not a SEPA country",
		"TR4412030000000":          "valid mod-97, TR is not a SEPA country",
	}
	for in, why := range invalid {
		if _, err := ValidateIBAN(in); err == nil {
			t.Errorf("ValidateIBAN(%q) should fail (%s)", in, why)
		}
	}

	// The rejection must name the rule, not hide behind a checksum message.
	if _, err := ValidateIBAN("SA0380000000608010167519"); err == nil || !strings.Contains(err.Error(), "SEPA") {
		t.Errorf("non-SEPA IBAN: error must name SEPA membership, got %v", err)
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

func TestPayloadDecomposedTextHint(t *testing.T) {
	base := Payment{IBAN: "DE02120300000000202051"}

	// 40 graphemes that look like "ü" but arrive as u + U+0308 (NFD, the
	// macOS clipboard form): 80 runes, over the limit, and the error must say
	// why a 40-letter name is "too long".
	decomposed := base
	decomposed.Name = strings.Repeat("u\u0308", 40)
	_, err := decomposed.Payload()
	if err == nil {
		t.Fatal("decomposed 80-rune name must exceed the 70-character limit")
	}
	if !strings.Contains(err.Error(), "80 characters") || !strings.Contains(err.Error(), "combining marks") {
		t.Errorf("error must count the runes and name the cause, got %v", err)
	}

	// Precomposed text over the limit gets the plain message: no false hint.
	precomposed := base
	precomposed.Name = strings.Repeat("ü", 71)
	_, err = precomposed.Payload()
	if err == nil || strings.Contains(err.Error(), "combining") {
		t.Errorf("precomposed name must fail without the combining-mark hint, got %v", err)
	}

	// Within the limit, decomposed text is accepted as typed.
	short := base
	short.Name = strings.Repeat("u\u0308", 35)
	if _, err := short.Payload(); err != nil {
		t.Errorf("70-rune decomposed name must be accepted: %v", err)
	}

	// Every field within its character limit, yet the 331-byte budget blows
	// because each decomposed letter costs three bytes: the byte-limit error
	// must carry the same hint.
	bytes := base
	bytes.Name = strings.Repeat("u\u0308", 35)
	bytes.Text = strings.Repeat("u\u0308", 70)
	_, err = bytes.Payload()
	if err == nil {
		t.Fatal("decomposed name plus text must exceed 331 bytes")
	}
	if !strings.Contains(err.Error(), "bytes") || !strings.Contains(err.Error(), "combining marks") {
		t.Errorf("byte-limit error must name the cause, got %v", err)
	}
}

func TestNormalizeAmountAmbiguous(t *testing.T) {
	for _, in := range []string{"1.234", "1,234"} {
		_, err := NormalizeAmount(in)
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("NormalizeAmount(%q): want an ambiguity error, got %v", in, err)
		}
	}
	// A zero or leading-zero integer part, no integer part at all, or space
	// grouping leave only one reading: these must get the plain decimals
	// message, not advice about thousands grouping.
	for _, in := range []string{"0.125", ".123", "0580.123", "1 234,567"} {
		_, err := NormalizeAmount(in)
		if err == nil || strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("NormalizeAmount(%q): want the plain decimals error, got %v", in, err)
		}
	}
	// Two separators or an explicit decimal part resolve the ambiguity.
	for in, want := range map[string]string{"1.234,00": "1234.00", "1,234.00": "1234.00", "1.234.567": "1234567.00", "1,234,567": "1234567.00"} {
		got, err := NormalizeAmount(in)
		if err != nil || got != want {
			t.Errorf("NormalizeAmount(%q) = %q, %v; want %q", in, got, err, want)
		}
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
		"letters where check digits would be":      {"RFAA14", "RFAA14"},
		"issuer scheme starting with RF":           {"RFQ2026001", "RFQ2026001"},
		"bare RF marker is an issuer value":        {"RF", "RF"},
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
		"check digits with no body":      {"RF12", "at least 5"},
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

// TestPayloadRejectsInvisibleCharacters: only CR and LF used to be refused,
// so every other control character reached the payload and the --details
// view verbatim (ESC drives the terminal that shows it), and bidi overrides
// or zero-width runs could make a beneficiary name read differently from
// how it is stored. Each rejection names the field and the codepoint.
func TestPayloadRejectsInvisibleCharacters(t *testing.T) {
	const iban = "DE02120300000000202051"
	cases := map[string]struct {
		p    Payment
		want string // field and codepoint the error must name
	}{
		"ESC in name":            {Payment{Name: "Alice\x1b[31mEVIL", IBAN: iban}, "beneficiary name contains a control character U+001B"},
		"NUL in text":            {Payment{Name: "X", IBAN: iban, Text: "paid\x00"}, "remittance text contains a control character U+0000"},
		"TAB in name":            {Payment{Name: "ACME\tGmbH", IBAN: iban}, "U+0009"},
		"BEL in info":            {Payment{Name: "X", IBAN: iban, Info: "ring\a"}, "U+0007"},
		"DEL in text":            {Payment{Name: "X", IBAN: iban, Text: "x\x7fy"}, "U+007F"},
		"C1 control in name":     {Payment{Name: "A\u0085B", IBAN: iban}, "U+0085"},
		"RLO in name":            {Payment{Name: "ACME\u202e GmbH", IBAN: iban}, "beneficiary name contains an invisible format character U+202E"},
		"LRI in text":            {Payment{Name: "X", IBAN: iban, Text: "invoice\u2066 42"}, "U+2066"},
		"PDI in text":            {Payment{Name: "X", IBAN: iban, Text: "42\u2069"}, "U+2069"},
		"zero-width space":       {Payment{Name: "X", IBAN: iban, Text: "in\u200bvoice"}, "U+200B"},
		"BOM in info":            {Payment{Name: "X", IBAN: iban, Info: "\ufeffinfo"}, "U+FEFF"},
		"zero-width joiner":      {Payment{Name: "A\u200dB", IBAN: iban}, "U+200D"},
		"ESC in non-RF ref":      {Payment{Name: "X", IBAN: iban, Ref: "INV\x1b[2K"}, "structured reference contains a control character U+001B"},
		"escape in purpose code": {Payment{Name: "X", IBAN: iban, Purpose: "A\x1bB"}, ""}, // rejected by the alphanumeric rule; any error will do
	}
	for name, tc := range cases {
		payload, err := tc.p.Payload()
		if err == nil {
			t.Errorf("%s: accepted, payload %q", name, payload)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q must name the field and codepoint (%q)", name, err, tc.want)
		}
	}

	// Visible whitespace and decomposed text stay legal: the rule is about
	// characters that do not show, not about anything non-ASCII.
	good := map[string]Payment{
		"no-break space in name":   {Name: "ACME\u00a0GmbH", IBAN: iban},
		"combining mark in name":   {Name: "Mu\u0308ller", IBAN: iban},
		"narrow no-break in text":  {Name: "X", IBAN: iban, Text: "1\u202f234"},
		"tab inside the IBAN only": {Name: "X", IBAN: "DE02\t1203 0000 0000 2020 51"}, // whitespace is stripped before the check
	}
	for name, p := range good {
		if _, err := p.Payload(); err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}
	}
}

// FuzzPayload pins the shape invariant behind the field gate: whatever the
// inputs, an accepted payload has at most twelve LF-separated lines and
// carries no control or format character other than the separators. The
// seeds are the hostile inputs from the 2026-09-21 audit.
func FuzzPayload(f *testing.F) {
	const iban = "DE02120300000000202051"
	f.Add("ACME GmbH", iban, "", "12,50", "", "", "invoice 42", "")
	f.Add("Alice\x1b[31mEVIL", iban, "", "", "", "", "", "")
	f.Add("X", iban, "", "", "", "", "paid\x1b[2A\x1b[2Kiban: DE00SPOOFED", "")
	f.Add("ACME\u202e GmbH", iban, "", "", "", "", "invoice\u2066 42\u2069\u200b", "\ufeffinfo")
	f.Add("X", iban, "BNPAFRPP", "0.01", "GDDS", "RF18539007547034", "", "")
	f.Fuzz(func(t *testing.T, name, iban, bic, amount, purpose, ref, text, info string) {
		p := Payment{Name: name, IBAN: iban, BIC: bic, Amount: amount, Purpose: purpose, Ref: ref, Text: text, Info: info}
		payload, err := p.Payload()
		if err != nil {
			return
		}
		if n := strings.Count(payload, "\n"); n > 11 {
			t.Fatalf("accepted payload has %d lines, EPC069-12 has 12 fields:\n%q", n+1, payload)
		}
		for _, r := range payload {
			if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
				t.Fatalf("accepted payload carries U+%04X:\n%q", r, payload)
			}
		}
	})
}
