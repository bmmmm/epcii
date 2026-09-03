# epcii

Generate EPC QR codes ("GiroCode") for SEPA credit transfers — a single
static binary with zero runtime dependencies, including its own QR encoder
core.

An EPC QR code ([EPC069-12](https://www.europeanpaymentscouncil.eu/document-library/guidance-documents/quick-response-code-guidelines-enable-data-capture-initiation))
encodes recipient, IBAN, amount, and remittance text so a banking app can
pre-fill a SEPA transfer from a single scan.

## Install

Requires Go 1.26 or newer:

```sh
go install github.com/bmmmm/epcii@latest
```

Or download a prebuilt binary: every
[GitHub release](https://github.com/bmmmm/epcii/releases) carries static
binaries for Linux, macOS, and Windows (amd64 and arm64) plus a
`SHA256SUMS` file to verify them against.

Or build from a checkout:

```sh
git clone https://github.com/bmmmm/epcii && cd epcii
go build -o epcii .
```

The result is a single static binary — copy or symlink it anywhere on your
`PATH`.

## Usage

```sh
epcii --name "ACME GmbH" --iban DE02120300000000202051 \
      --amount 580.00 --text "Invoice RE-2026-001" > qr.svg
```

Output is an SVG on stdout. Options:

| Flag | Description |
|---|---|
| `--name` | Beneficiary name (required, ≤70 chars) |
| `--iban` | Beneficiary IBAN (required; must belong to a SEPA country, length and mod-97 checked) |
| `--amount` | Amount in EUR, `0.01`–`999999999.99`; German comma form (`580,00`) and thousands grouping (`1.234,56`, `1,234.56`, `1.234.567`) accepted; a lone `1.234` is refused as ambiguous |
| `--text` | Unstructured remittance text (≤140 chars) |
| `--ref` | Structured creditor reference (≤35 chars), mutually exclusive with `--text`. `RF` followed by two digits marks an ISO 11649 creditor reference: it is normalized (upper-cased, spaces removed) and checked (≤25 chars, mod-97). Every other reference is passed through after trimming surrounding whitespace |
| `--bic` | BIC (optional within the EEA) |
| `--purpose` | SEPA purpose code (≤4 chars, alphanumeric) |
| `--info` | Beneficiary-to-originator information (≤70 chars) |
| `--png <file>` | Additionally write a PNG |
| `--term` | Print a terminal preview to stderr |
| `--details` | Print the encoded payload fields to stderr for verification |
| `--version` | Print the version and exit |

Inputs are normalized defensively: IBANs may contain any whitespace
(no-break and narrow spaces from PDF copy-paste included), amounts accept
comma or dot decimals and space/dot/comma thousands grouping, and invalid
input fails with a named-field error before anything is encoded.

For print, keep the symbol at least ~40 mm wide (EPC recommendation); the
SVG scales losslessly and already includes the 4-module quiet zone.

## Design

- EPC069-12 version 002 payload, UTF-8, LF separators, ≤331 bytes,
  error correction level M — validated before encoding, against the rules
  of the official EPC069-12 v3.1 guideline.
- QR encoder core in `internal/qr`, derived from
  [piglig/go-qr](https://github.com/piglig/go-qr) (MIT), reduced to byte
  mode, level M, versions 1–13 — pinned byte-identical to upstream (level
  M, no ECC boosting) for every payload length by an in-tree fingerprint
  fixture, and round-trip verified with an independent decoder. See
  `NOTICE`.
- Zero runtime dependencies: the only `go.mod` entry,
  [gozxing](https://github.com/makiuchi-d/gozxing), is a test-only decoder
  used for round-trip verification and is not compiled into the binary.

## Development

```sh
go build -o epcii .   # build
go test ./...         # unit, golden, and round-trip tests
go vet ./... && gofmt -l .
```

`--version` reports whatever the build info carries: the module version for
`go install`, a VCS pseudo-version for a plain `go build` in a checkout.
Release builds stamp it explicitly:

```sh
go build -trimpath -ldflags "-s -w -X main.version=v1.2.3" -o epcii .
```

The encoder is cross-checked four ways: round-trip decoding with the
independent gozxing ZXing port, golden payload fixtures generated from
segno's reference implementation (regenerate via
`uvx --from segno python scripts/gen_segno_fixtures.py`), an SVG path
reconstruction test that rebuilds the module matrix from the emitted path,
and matrix fingerprints for every payload length generated from the
upstream piglig/go-qr encoder (`go run -C scripts/qrfixtures .`, a separate
module so upstream never enters `go.mod`).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) — module map, public contracts, and
test conventions ([AGENTS.md](AGENTS.md) has the condensed version for
coding agents). Bug reports go through the issue forms; suspected
vulnerabilities go through [SECURITY.md](SECURITY.md), never a public
issue.

## License

GPL-3.0-or-later — see [LICENSE](LICENSE). Bundled QR encoder code is
MIT-licensed (compatible); attribution in [NOTICE](NOTICE).
