# Contributing to epcii

Thanks for helping out. This document maps the codebase, states the
contracts you must not break, and explains what evidence each issue form
asks for and why.

## Module map

| Path | What lives there |
|---|---|
| `main.go` | CLI: flag parsing, input hardening, output plumbing. Testable via `run()` (`main_test.go`) |
| `internal/epc` | EPC069-12 payload builder and all validation: IBAN (SEPA country, length, mod-97), amount normalization, field limits, UTF-8 enforcement |
| `internal/qr` | QR encoder core — byte mode, ECC level M, versions 1–13, mask selection. Derived from [piglig/go-qr](https://github.com/piglig/go-qr) (MIT, see `NOTICE`) |
| `internal/render` | SVG / PNG / ANSI-terminal renderers over the module matrix |
| `internal/webapi` | The CLI pipeline as one pure function for the browser build; native tests |
| `cmd/epcii-wasm` | `js && wasm` entry point exposing `internal/webapi` as `globalThis.epcii` |
| `web/` | Static page (HTML/JS/CSS, no framework, no external resources); `web/dist/` is the gitignored build output |
| `scripts/build-web.sh` | Assembles `web/dist/` (page files, Go's `wasm_exec.js`, the wasm build) |
| `scripts/web-smoke.mjs` | Node gate: the wasm build's SVG/PNG must equal the CLI's byte for byte; also greps `web/` for storage APIs, address-bar writes, external resources and HTML string sinks, pins the CSP directives in `web/index.html` literally, and pins that `fileStem()` and `shareParams()` read the encoded result rather than the form, and that `render()` snapshots the fields they read |
| `scripts/gen_segno_fixtures.py` | One-shot generator for the segno golden fixtures in `internal/epc/testdata/` |
| `scripts/qrfixtures/` | Separate Go module: regenerates the upstream matrix fingerprints in `internal/qr/testdata/` from piglig/go-qr |

## Build and test

```sh
go build -o epcii .
go test ./...
go vet ./... && test -z "$(gofmt -l .)"
```

CI runs exactly these (`.github/workflows/ci.yml`) on Linux, macOS and
Windows, and on Linux additionally the race detector, `govulncheck`, and a
byte-for-byte regeneration of the upstream QR fingerprint fixture. `gofmt` and
that fixture diff are Linux-only — their answer cannot differ per OS.
`security.yml` adds gitleaks and a forbidden-file check.

## Cutting a release

Pushing a `v*` tag runs `release.yml`: it re-runs the tests, cross-compiles the
six release binaries with the version stamped via `-ldflags`, and publishes
them with a `SHA256SUMS` file as a GitHub release.

**Write the `CHANGELOG.md` section before you tag.** The release notes are the
section whose heading matches the tag without its `v` — `v0.3.0` needs
`## [0.3.0]`. A tag with no section, or one with a heading but no entries,
fails the job before anything is built, and you are left with a pushed tag and
no release; recovering means deleting the tag on both remotes and pushing it
again. Entries describe the effect on a user, with the input value that behaves
differently — not the commits.

## Public contracts — do not break

- **stdout carries only the SVG.** Previews (`--term`), details
  (`--details`), and errors go to stderr; downstream tooling pipes stdout.
- **Exit codes:** `0` success, `2` invalid input/usage, `1` I/O failure.
- **CLI flag names and semantics** — see the README table.
- **Payload format:** EPC069-12 version 002, LF separators, UTF-8
  (charset field `1`), amounts with exactly two decimals, trailing empty
  fields trimmed together with their separators.
- **Zero runtime dependencies.** `go.mod` may only grow test-only entries,
  each with a one-line justification.
- **The web version is the CLI.** Its SVG and PNG stay byte-identical to
  `epcii` / `epcii --png` (`scripts/web-smoke.mjs`), and the page stores and
  sends nothing: no cookies, no web storage, no service worker, no external
  resource, no automatic write to the address bar. The CSP meta tag is
  part of that contract and is pinned by the same gate. See README
  "Privacy".

`internal/` is not a public Go API — its shape may change freely.

## Test conventions

The encoder's correctness rests on four independent checks; keep all four
alive when touching the encode path:

1. **Round-trip decode** with the independent
   [gozxing](https://github.com/makiuchi-d/gozxing) ZXing port (test-only
   dependency), across every version 1–13 at exact byte capacity.
2. **Golden payload fixtures** cross-generated from segno's reference
   implementation. Regenerate with
   `uvx --from segno python scripts/gen_segno_fixtures.py` and keep the
   inputs in `internal/epc/golden_test.go` in sync with the script.
3. **SVG path reconstruction** — the emitted path is parsed back into a
   module matrix and compared 1:1 (quiet zone hardcoded to 4 per ISO 18004).
4. **Upstream matrix fingerprints** — `TestMatrixFingerprints` compares
   the chosen version and the full module matrix for every payload length
   0–331 against `internal/qr/testdata/matrix_fingerprints.txt`, generated
   from piglig/go-qr v1.1.0 by `scripts/qrfixtures` (its own Go module, so
   the upstream package never enters the main `go.mod`). Unlike the
   round-trip decoder it does not forgive ECC-repairable damage: mask
   choice, alignment geometry, the dark module and padding order all go
   red here.

House rule: **a new test must be able to go red.** If you add a gate,
demonstrate (e.g. via a temporary mutation) that it fails when the guarded
property breaks. Several tests here exist precisely because mutation
testing showed the previous suite stayed green.

Some validation tests contain real no-break/narrow-space Unicode characters
on purpose (copy-paste realism) — don't "clean them up".

### Touching `internal/qr`

The core is derived code, kept deliberately close to upstream piglig/go-qr.
Equivalence with upstream is pinned by the fingerprint fixture (check 4
above) for the parameters this core implements: byte mode, level M,
versions 1–13, automatic mask, no ECC boosting. If `TestMatrixFingerprints`
goes red you changed encode behaviour — restore equivalence, or justify the
intentional divergence per ISO/IEC 18004 in the PR. Regenerate the fixture
only from upstream (`go run -C scripts/qrfixtures . >
internal/qr/testdata/matrix_fingerprints.txt`), never from `EncodeM`, and
only when the upstream pin or the fixture payload rule changes. Keep the
MIT attribution headers and `NOTICE` intact.

### Payload rules

Payload questions are settled against the **official EPC069-12 guideline**
(v3.1, European Payments Council) — not blog posts or generator sites,
which contradict each other on trailing-field and amount-format rules.
Deliberate choices worth knowing: we always emit two decimals (spec allows
fewer; segno strips trailing zeros — both conform), and we emit LF (spec
allows CRLF too). IBANs outside the SEPA country table are rejected, not
merely mod-97 checked: the payload initiates a SEPA credit transfer, which
cannot reach them. Text fields are not NFC-normalized (that would pull
`golang.org/x/text` into the binary); decomposed input counts every
combining mark as a character, and the length error says so. No field may
carry an invisible character: control characters (Unicode `Cc`, which
includes ESC, NUL and TAB, not only CR/LF) and format characters (`Cf`:
bidi overrides, zero-width space and joiner, BOM) are rejected with the
field and codepoint named. An ESC would drive the terminal that displays
the `--details` view; a bidi override would make a beneficiary name read
differently from how it is stored. Visible whitespace such as the no-break
space stays legal.

## Issues and PRs

Use the issue forms — each asks for the evidence that actually routes its
bug class:

- **Scan failures** hinge on the `--details` output (payload bug vs.
  rendering/app problem) and on whether the PNG scans (symbol vs.
  embedding/print size).
- **Validation reports** hinge on whether your bank accepts the same value
  entered manually (validator too strict vs. input actually invalid).
- **Output reports** isolate one renderer once `--details` is correct and
  the PNG decodes.

PRs follow the template: claims a reviewer can check against the diff,
pasted verification output, tests that can go red, and the contracts
checklist above. Never include real IBANs or names in issues, PRs, or test
data — use test IBANs like `DE02120300000000202051`.

## License

Contributions are accepted under GPL-3.0-or-later (see `LICENSE`). The
bundled QR core keeps its MIT attribution (`NOTICE`).
