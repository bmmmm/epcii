package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestRunSVGOutput(t *testing.T) {
	code, out, errOut := runCLI(t,
		"--name", "ACME GmbH", "--iban", "DE02120300000000202051",
		"--amount", "580,00", "--text", "RE-1")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut)
	}
	if !strings.HasPrefix(out, "<svg ") || !strings.Contains(out, "</svg>") {
		t.Errorf("stdout is not an SVG document: %.80q", out)
	}
	if errOut != "" {
		t.Errorf("unexpected stderr: %q", errOut)
	}
}

func TestRunValidationError(t *testing.T) {
	code, out, errOut := runCLI(t, "--name", "X", "--iban", "DE00123")
	if code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if out != "" {
		t.Error("stdout must stay empty on validation errors")
	}
	if !strings.Contains(errOut, "IBAN") {
		t.Errorf("stderr should mention IBAN: %q", errOut)
	}
}

func TestRunMissingName(t *testing.T) {
	if code, _, _ := runCLI(t, "--iban", "DE02120300000000202051"); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

func TestRunPositionalArgRejected(t *testing.T) {
	// Classic mistake: --text hallo welt (missing quotes around the value).
	code, _, errOut := runCLI(t, "--name", "X", "--iban", "DE02120300000000202051",
		"--text", "hallo", "welt")
	if code != 2 || !strings.Contains(errOut, "welt") {
		t.Errorf("exit %d, stderr %q — want 2 and the stray arg named", code, errOut)
	}
	if !strings.Contains(errOut, "quotes") {
		t.Errorf("stderr should hint at missing quotes: %q", errOut)
	}
}

func TestRunValueLookingLikeFlag(t *testing.T) {
	// --text without a value swallows the next flag as its value.
	code, _, errOut := runCLI(t, "--name", "X", "--iban", "DE02120300000000202051",
		"--text", "--term")
	if code != 2 || !strings.Contains(errOut, "--term") {
		t.Errorf("exit %d, stderr %q — want 2 naming the swallowed flag", code, errOut)
	}

	// The documented escape hatch keeps literal dashy values possible.
	code, out, errOut := runCLI(t, "--name", "X", "--iban", "DE02120300000000202051",
		"--text=--term")
	if code != 0 {
		t.Errorf("--text=--term should be accepted, got exit %d, stderr %q", code, errOut)
	}
	if !strings.HasPrefix(out, "<svg ") {
		t.Error("SVG expected on stdout")
	}
}

func TestRunDetails(t *testing.T) {
	code, out, errOut := runCLI(t,
		"--name", "Erika Mustermann", "--iban", "DE02 1203 0000 0000 2020 51",
		"--amount", "1,37", "--text", "hallo welt", "--details")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut)
	}
	if !strings.HasPrefix(out, "<svg ") {
		t.Error("SVG still expected on stdout")
	}
	for _, want := range []string{
		"name:", "Erika Mustermann",
		"iban:", "DE02120300000000202051", // normalized, without spaces
		"amount:", "EUR1.37",
		"text:", "hallo welt",
		"QR version",
	} {
		if !strings.Contains(errOut, want) {
			t.Errorf("details output missing %q:\n%s", want, errOut)
		}
	}
}

func TestRunPNGAndTerm(t *testing.T) {
	pngPath := filepath.Join(t.TempDir(), "out.png")
	code, out, errOut := runCLI(t,
		"--name", "ACME GmbH", "--iban", "DE02120300000000202051",
		"--png", pngPath, "--term")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut)
	}
	data, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Error("written file is not a PNG")
	}
	if !strings.Contains(errOut, "\x1b[") {
		t.Error("terminal preview missing from stderr")
	}
	if !strings.HasPrefix(out, "<svg ") {
		t.Error("SVG still expected on stdout")
	}
}

// assertNoScratchFiles fails if the atomic PNG write left its temp file behind.
func assertNoScratchFiles(t *testing.T, dir string) {
	t.Helper()
	leftovers, err := filepath.Glob(filepath.Join(dir, ".epcii-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) > 0 {
		t.Errorf("scratch files left behind: %v", leftovers)
	}
}

func TestRunPNGWriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "out.png")
	code, _, errOut := runCLI(t,
		"--name", "ACME GmbH", "--iban", "DE02120300000000202051", "--png", pngPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut)
	}
	info, err := os.Stat(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		// os.CreateTemp opens 0600; without the chmod the result would be
		// unreadable for everyone but the caller.
		if perm := info.Mode().Perm(); perm != 0o644 {
			t.Errorf("mode %04o, want 0644", perm)
		}
	}
	assertNoScratchFiles(t, dir)
}

func TestRunPNGFailedWriteKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "out.png")
	existing := []byte("\x89PNG previous rendering — must survive a failed write\n")
	if err := os.WriteFile(pngPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	restore := encodePNG
	encodePNG = func(io.Writer, [][]bool, int) error {
		return errors.New("encoder blew up mid-write")
	}
	t.Cleanup(func() { encodePNG = restore })

	code, out, errOut := runCLI(t,
		"--name", "ACME GmbH", "--iban", "DE02120300000000202051", "--png", pngPath)
	if code != 1 || errOut == "" {
		t.Errorf("exit %d, stderr %q — want 1 with an error message", code, errOut)
	}
	if out != "" {
		t.Error("stdout must stay empty when the PNG write fails")
	}
	got, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, existing) {
		t.Errorf("existing file was clobbered: %q", got)
	}
	assertNoScratchFiles(t, dir)
}

func TestRunPNGUnwritablePath(t *testing.T) {
	code, _, errOut := runCLI(t,
		"--name", "X", "--iban", "DE02120300000000202051",
		"--png", filepath.Join(t.TempDir(), "missing-dir", "out.png"))
	if code != 1 || errOut == "" {
		t.Errorf("exit %d, stderr %q — want 1 with an error message", code, errOut)
	}
}

func TestRunVersionFlag(t *testing.T) {
	code, out, _ := runCLI(t, "--version")
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Errorf("exit %d, out %q", code, out)
	}
}
