// epcii generates EPC QR codes ("GiroCode", EPC069-12) for SEPA credit
// transfers: a validated payload rendered as SVG on stdout, optionally as
// PNG file and terminal preview.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/bmmmm/epcii/internal/epc"
	"github.com/bmmmm/epcii/internal/qr"
	"github.com/bmmmm/epcii/internal/render"
)

var version = "dev"

const pngScale = 8 // pixels per module; 69 modules + quiet zone => 616 px max

func main() {
	var (
		p           epc.Payment
		pngPath     string
		term        bool
		showVersion bool
	)
	flag.StringVar(&p.Name, "name", "", "beneficiary name (required, max 70 chars)")
	flag.StringVar(&p.IBAN, "iban", "", "beneficiary IBAN (required)")
	flag.StringVar(&p.Amount, "amount", "", "amount in EUR, 0.01-999999999.99; German comma form accepted")
	flag.StringVar(&p.Text, "text", "", "unstructured remittance text (max 140 chars)")
	flag.StringVar(&p.Ref, "ref", "", "structured creditor reference (ISO 11649), excludes -text")
	flag.StringVar(&p.BIC, "bic", "", "BIC (optional within the EEA)")
	flag.StringVar(&p.Purpose, "purpose", "", "SEPA purpose code (max 4 chars)")
	flag.StringVar(&p.Info, "info", "", "beneficiary-to-originator information (max 70 chars)")
	flag.StringVar(&pngPath, "png", "", "additionally write a PNG to this `file`")
	flag.BoolVar(&term, "term", false, "additionally print a preview to stderr")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"epcii — EPC QR code (GiroCode) generator; SVG on stdout\n\n"+
				"usage: epcii --name NAME --iban IBAN [options] > qr.svg\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		fmt.Println(versionString())
		return
	}
	if flag.NArg() > 0 {
		fail(fmt.Errorf("unexpected argument %q — all inputs are flags", flag.Arg(0)))
	}

	payload, err := p.Payload()
	if err != nil {
		fail(err)
	}
	code, err := qr.EncodeM([]byte(payload))
	if err != nil {
		fail(err)
	}
	matrix := code.Matrix()

	if pngPath != "" {
		f, err := os.Create(pngPath)
		if err != nil {
			fail(err)
		}
		if err := render.PNG(f, matrix, pngScale); err != nil {
			f.Close()
			fail(err)
		}
		if err := f.Close(); err != nil {
			fail(err)
		}
	}
	if term {
		fmt.Fprint(os.Stderr, render.Terminal(matrix))
	}
	if _, err := os.Stdout.Write(render.SVG(matrix)); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "epcii:", err)
	os.Exit(2)
}

// versionString reports the release version: the -ldflags override when set,
// otherwise the module version recorded by `go install module@version`.
func versionString() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
