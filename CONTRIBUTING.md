# Contributing to epcii

Thanks for helping out. This document maps the codebase, states the
contracts you must not break, and explains what evidence each issue form
asks for and why.

## Module map

| Path | What lives there |
|---|---|
| `main.go` | CLI: flag parsing, input hardening, output plumbing. Testable via `run()` (`main_test.go`) |
| `internal/epc` | EPC069-12 payload builder and all validation: IBAN (mod-97 + SEPA lengths), amount normalization, field limits, UTF-8 enforcement |
| `internal/qr` | QR encoder core — byte mode, ECC level M, versions 1–13, mask selection. Derived from [piglig/go-qr](https://github.com/piglig/go-qr) (MIT, see `NOTICE`) |
| `internal/render` | SVG / PNG / ANSI-terminal renderers over the module matrix |
| `scripts/gen_segno_fixtures.py` | One-shot generator for the segno golden fixtures in `internal/epc/testdata/` |
| `scripts/qrfixtures/` | Separate Go module: regenerates the upstream matrix fingerprints in `internal/qr/testdata/` from piglig/go-qr |

## Build and test

```sh
go build -o epcii .
go test ./...
go vet ./... && test -z "$(gofmt -l .)"
```

CI runs exactly these (`.github/workflows/ci.yml`), plus gitleaks and a
forbidden-file check (`security.yml`).

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
allows CRLF too).

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
