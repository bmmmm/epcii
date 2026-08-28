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
		"DE0212030000000020205":       "too short for DE but passes length range; checksum must fail",
		"DE021203000000002020511":     "checksum fails",
		"XX0000000000000":             "checksum fails",
		"D102120300000000202051":      "country code not letters",
		"DEAB120300000000202051":      "check digits not numeric",
		"DE02-1203-0000-0000-2020-51": "invalid characters",
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
	}
	for in, want := range good {
		got, err := NormalizeAmount(in)
		if err != nil {
			t.Errorf("NormalizeAmount(%q) unexpected error: %v", in, err)
		} else if got != want {
			t.Errorf("NormalizeAmount(%q) = %q, want %q", in, got, want)
		}
	}

	bad := []string{"", "0.00", "0", "12.345", "abc", "12a", "1000000000.00", "1e3", "-5"}
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
