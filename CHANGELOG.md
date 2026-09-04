# Changelog

Notable changes to epcii. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). While the major
version is 0, behaviour may change in a minor release — the entries below say
what to check.

## [Unreleased]

## [0.2.0] - 2026-09-04

### Changed

- `--ref` only normalizes a reference that presents itself as ISO 11649, that
  is `RF` followed by two digits. Every other reference belongs to an issuer's
  own scheme and now reaches the payload verbatim after trimming surrounding
  whitespace: `inv-2026-abc` stays lower case instead of becoming
  `INV-2026-ABC`, and `rfid-77` is no longer rejected as a broken creditor
  reference.
- `--iban` rejects an IBAN whose country is not a SEPA scheme participant. An
  EPC QR code initiates a SEPA credit transfer, so such an IBAN cannot be a
  valid target; previously it passed on the mod-97 check alone. The country
  table also replaces the generic 15-34 character length check.
- `--amount` refuses a lone `1.234` as ambiguous and offers both unambiguous
  spellings, instead of reporting "more than two decimal places" — a message a
  German reader cannot act on. `0.125`, `.123` and space-grouped input keep the
  plain decimals message.
- `--amount` accepts whole amounts with two or more separator groups as
  thousands grouping: `1.234.567` and `1,234,567` now parse instead of being
  refused.
- `--png` writes atomically. The encoder renders into a scratch file next to
  the target and renames it over the target, so a failure mid-write no longer
  leaves a truncated PNG behind or destroys the previous rendering. The result
  carries the permissions `os.Create` would have given it (0666 minus umask);
  an existing file keeps its own mode and a symlink is still written through.
- A field-limit or 331-byte error names combining marks as the cause when the
  input carries them. Text pasted from macOS often arrives decomposed, where a
  40-letter name is counted as 138 characters with no hint why.
- `--bic` enforces the ISO 9362 shape: four letters for the institution, two
  for the country, then alphanumerics for location and optional branch.

### Added

- Prebuilt binaries for Linux, macOS and Windows on amd64 and arm64, published
  with a `SHA256SUMS` file whenever a `v*` tag is pushed. They are static
  (`CGO_ENABLED=0`), reproducible in path (`-trimpath`) and stamped, so
  `--version` reports the tag.
- `TestMatrixFingerprints` pins the QR matrix — version and every module —
  against fingerprints generated from upstream piglig/go-qr for every payload
  length 0 to 331. The round-trip decoder forgives ECC-repairable damage and
  stayed green with mask selection disabled or the dark module missing; this
  gate does not.

### Fixed

- The ISO 11649 check enforces the standard's 25-character limit, and each
  failure names the rule that actually fired instead of blaming mod-97.
  `RF255390075470345390075470345` carries a valid mod-97 remainder and was
  accepted before; it is now refused as too long.
- Only `RF` followed by two digits is treated as an ISO 11649 claim. A
  reference like `RFAA14`, `RFQ2026001` or a bare `RF` is no longer forced
  through that validation and passes through as an issuer's own value — it is
  accepted, but epcii no longer represents it as a checked creditor reference.
  `RF12`, a claim without a body, stays rejected.
- `--ref "   "` combined with `--text` no longer trips the mutual-exclusion
  check: it is trimmed to empty first.
- `--bic 00000000` is rejected instead of being accepted as a placeholder.

## [0.1.0] - 2026-08-31

### Added

- Initial release: EPC069-12 payload builder, own QR encoder core, and SVG,
  PNG and terminal output.
