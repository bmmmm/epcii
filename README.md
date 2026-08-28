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
| `--iban` | Beneficiary IBAN (required; mod-97 and SEPA length checked) |
| `--amount` | Amount in EUR, `0.01`–`999999999.99`; German comma form (`580,00`) accepted |
| `--text` | Unstructured remittance text (≤140 chars) |
| `--ref` | Structured creditor reference (≤35 chars; `RF…` refs are ISO 11649 checked), mutually exclusive with `--text` |
| `--bic` | BIC (optional within the EEA) |
| `--purpose` | SEPA purpose code (≤4 chars, alphanumeric) |
| `--info` | Beneficiary-to-originator information (≤70 chars) |
| `--png <file>` | Additionally write a PNG |
| `--term` | Print a terminal preview to stderr |
| `--version` | Print the version and exit |

## Design

- EPC069-12 version 002 payload, UTF-8, LF separators, ≤331 bytes,
  error correction level M — validated before encoding.
- QR encoder core in `internal/qr`, derived from
  [piglig/go-qr](https://github.com/piglig/go-qr) (MIT), reduced to byte
  mode, level M, versions 1–13 — proven byte-identical to upstream for
  every payload length and round-trip verified with an independent
  decoder. See `NOTICE`.

## License

GPL-3.0-or-later — see [LICENSE](LICENSE). Bundled QR encoder code is
MIT-licensed (compatible); attribution in [NOTICE](NOTICE).
