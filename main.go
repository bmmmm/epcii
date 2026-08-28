// epcii generates EPC QR codes ("GiroCode", EPC069-12) for SEPA credit
// transfers: a validated payload rendered as SVG on stdout, optionally as
// PNG file and terminal preview.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/bmmmm/epcii/internal/epc"
	"github.com/bmmmm/epcii/internal/qr"
	"github.com/bmmmm/epcii/internal/render"
)

var version = "dev"

const pngScale = 8 // pixels per module; 69 modules + quiet zone => 616 px max

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var (
		p           epc.Payment
		pngPath     string
		term        bool
		showVersion bool
	)
	fs := flag.NewFlagSet("epcii", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&p.Name, "name", "", "beneficiary name (required, max 70 chars)")
	fs.StringVar(&p.IBAN, "iban", "", "beneficiary IBAN (required)")
	fs.StringVar(&p.Amount, "amount", "", "amount in EUR, 0.01-999999999.99; German comma form accepted")
	fs.StringVar(&p.Text, "text", "", "unstructured remittance text (max 140 chars)")
	fs.StringVar(&p.Ref, "ref", "", "structured creditor reference (max 35 chars; RF references are ISO 11649 checked), excludes -text")
	fs.StringVar(&p.BIC, "bic", "", "BIC (optional within the EEA)")
	fs.StringVar(&p.Purpose, "purpose", "", "SEPA purpose code (max 4 chars, alphanumeric)")
	fs.StringVar(&p.Info, "info", "", "beneficiary-to-originator information (max 70 chars)")
	fs.StringVar(&pngPath, "png", "", "additionally write a PNG to this `file`")
	fs.BoolVar(&term, "term", false, "additionally print a preview to stderr")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(),
			"epcii — EPC QR code (GiroCode) generator; SVG on stdout\n\n"+
				"usage: epcii --name NAME --iban IBAN [options] > qr.svg\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if showVersion {
		fmt.Fprintln(stdout, versionString())
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "epcii: unexpected argument %q — all inputs are flags\n", fs.Arg(0))
		return 2
	}

	payload, err := p.Payload()
	if err != nil {
		fmt.Fprintln(stderr, "epcii:", err)
		return 2
	}
	code, err := qr.EncodeM([]byte(payload))
	if err != nil {
		fmt.Fprintln(stderr, "epcii:", err)
		return 2
	}
	matrix := code.Matrix()

	if pngPath != "" {
		if err := writePNG(pngPath, matrix); err != nil {
			fmt.Fprintln(stderr, "epcii:", err)
			return 1
		}
	}
	if term {
		fmt.Fprint(stderr, render.Terminal(matrix))
	}
	if _, err := stdout.Write(render.SVG(matrix)); err != nil {
		fmt.Fprintln(stderr, "epcii:", err)
		return 1
	}
	return 0
}

func writePNG(path string, matrix [][]bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := render.PNG(f, matrix, pngScale); err != nil {
		f.Close()
		return err
	}
	return f.Close()
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
