# epcii

Generate EPC QR codes ("GiroCode") for SEPA credit transfers — a single
static binary with zero runtime dependencies, including its own QR encoder
core.

An EPC QR code (EPC069-12) encodes recipient, IBAN, amount, and remittance
text so a banking app can pre-fill a SEPA transfer from a single scan.

## Usage

```sh
epcii --name "ACME GmbH" --iban DE02120300000000202051 \
      --amount 580.00 --text "Invoice RE-2026-001" > qr.svg
```

Output is an SVG on stdout. Options:

| Flag | Description |
|---|---|
| `--name` | Beneficiary name (required, ≤70 chars) |
| `--iban` | Beneficiary IBAN (required, mod-97 checked) |
| `--amount` | Amount in EUR, `0.01`–`999999999.99`; German comma form (`580,00`) accepted |
| `--text` | Unstructured remittance text (≤140 chars) |
| `--ref` | Structured creditor reference (ISO 11649), mutually exclusive with `--text` |
| `--bic` | BIC (optional within the EEA) |
| `--purpose` | 4-letter SEPA purpose code |
| `--png <file>` | Additionally write a PNG |
| `--term` | Print a terminal preview |

## Design

- EPC069-12 version 002 payload, UTF-8, LF separators, ≤331 bytes,
  error correction level M — validated before encoding.
- QR encoder core in `internal/qr`, derived from
  [piglig/go-qr](https://github.com/piglig/go-qr) (a Go port of
  [Nayuki's QR Code generator](https://www.nayuki.io/page/qr-code-generator-library),
  MIT), reduced to byte mode, level M, versions 1–13. See `NOTICE`.

## License

GPL-3.0-or-later — see [LICENSE](LICENSE). Bundled QR encoder code is
MIT-licensed (compatible); attribution in [NOTICE](NOTICE).
