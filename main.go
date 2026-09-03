// epcii generates EPC QR codes ("GiroCode", EPC069-12) for SEPA credit
// transfers: a validated payload rendered as SVG on stdout, optionally as
// PNG file and terminal preview.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

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
		details     bool
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
	fs.BoolVar(&details, "details", false, "additionally print the encoded payload fields to stderr")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(),
			"epcii — EPC QR code (GiroCode) generator; SVG on stdout\n\n"+
				"usage: epcii --name NAME --iban IBAN [options] > qr.svg\n\n")
		fs.PrintDefaults()
	}
	// A forgotten value makes the flag parser swallow the next flag as the
	// value ("--text --term" would encode the literal text "--term"). Catch
	// that on the raw arguments; the --flag=--literal form stays available.
	stringFlags := map[string]bool{
		"name": true, "iban": true, "amount": true, "text": true, "ref": true,
		"bic": true, "purpose": true, "info": true, "png": true,
	}
	for i := 0; i < len(args)-1; i++ {
		name := strings.TrimLeft(args[i], "-")
		if name == args[i] || strings.Contains(args[i], "=") {
			continue // not a flag token, or value attached via =
		}
		if stringFlags[name] && looksLikeFlag(args[i+1]) {
			fmt.Fprintf(stderr, "epcii: value of --%s is %q, which looks like a flag — "+
				"did the previous flag miss its value? Use --%s=%q if intentional\n",
				name, args[i+1], name, args[i+1])
			return 2
		}
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if showVersion {
		fmt.Fprintln(stdout, versionString())
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "epcii: unexpected argument %q — all inputs are flags; "+
			"multi-word values need quotes (e.g. --text \"invoice 42\")\n", fs.Arg(0))
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
	if details {
		printDetails(stderr, payload, code)
	}
	if _, err := stdout.Write(render.SVG(matrix)); err != nil {
		fmt.Fprintln(stderr, "epcii:", err)
		return 1
	}
	return 0
}

// payloadFieldNames labels the EPC069-12 payload lines in order; trailing
// optional lines may be omitted from the payload.
var payloadFieldNames = []string{
	"service tag", "version", "charset", "identification",
	"bic", "name", "iban", "amount", "purpose", "ref", "text", "info",
}

// printDetails writes the exact encoded payload, line by line with field
// labels, plus the QR geometry — so the user can verify what went into the
// symbol without scanning it.
func printDetails(w io.Writer, payload string, code *qr.Code) {
	fmt.Fprintf(w, "encoded GiroCode payload (%d bytes, QR version %d, %dx%d modules):\n",
		len(payload), code.Version(), code.Size(), code.Size())
	for i, line := range strings.Split(payload, "\n") {
		value := line
		if value == "" {
			value = "(empty)"
		}
		fmt.Fprintf(w, "  %-15s %s\n", payloadFieldNames[i]+":", value)
	}
}

// looksLikeFlag reports whether a value has the shape of a CLI flag
// (-t, --term); negative numbers like "-5" are not flagged so they reach
// the amount validation with a better message.
func looksLikeFlag(value string) bool {
	trimmed := strings.TrimLeft(value, "-")
	if trimmed == value || trimmed == "" {
		return false
	}
	r := rune(trimmed[0])
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// encodePNG is the PNG encoder used by writePNG; a test replaces it to
// exercise the failure path without a filesystem trick.
var encodePNG = render.PNG

// writePNG writes the PNG atomically: encode into a temp file in the target
// directory, then rename it over the target. os.Create would truncate an
// existing file up front, so a failed or partial encode would destroy it.
func writePNG(path string, matrix [][]bool) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".epcii-*.png")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := encodePNG(tmp, matrix, pngScale); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	// CreateTemp opens with 0600; a QR image is not a secret and should look
	// like any other generated file.
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// versionString reports the release version. An -ldflags override wins;
// otherwise the module version from the build info is used, which is the
// requested version for `go install module@version` and a VCS-stamped
// pseudo-version (e.g. v0.1.1-0.20260903003039-c6c6eb0bb191) for a plain
// `go build` inside a git checkout. Only builds without VCS information —
// `-buildvcs=false`, or a source archive without .git — report "dev".
func versionString() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
