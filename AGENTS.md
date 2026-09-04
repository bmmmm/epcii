# AGENTS.md — working on epcii

Condensed rules for coding agents. Details and rationale: `CONTRIBUTING.md`.

## Commands

```sh
go build -o epcii .                    # build
go test ./...                          # full suite (~3 s)
go vet ./... && test -z "$(gofmt -l .)"  # must both pass before commit
```

## Map

- `main.go` — CLI; logic lives in `run()`, tested in `main_test.go`
- `internal/epc` — payload builder + ALL validation
- `internal/qr` — QR encoder (byte mode, level M, versions 1–13)
- `internal/render` — SVG / PNG / terminal renderers

## Traps

- `internal/qr` is derived from piglig/go-qr (MIT). Do not refactor it
  casually: its correctness proof is byte-identical equivalence with
  upstream, pinned by `TestMatrixFingerprints` against
  `internal/qr/testdata/matrix_fingerprints.txt` for every payload length
  0–331. Keep attribution headers and `NOTICE`. Never regenerate that
  fixture to turn the test green: it comes only from upstream via
  `go run -C scripts/qrfixtures .` (a separate module, so piglig stays out
  of `go.mod`), and only when the upstream pin or the payload rule changes
  — say which in the PR.
- stdout must carry ONLY the SVG. Anything else (preview, details, errors)
  goes to stderr. Exit codes: 0 ok, 2 bad input/usage, 1 I/O.
- Payload questions are settled by the official EPC069-12 v3.1 PDF, not by
  generator websites. We deliberately emit two-decimal amounts and LF.
- Some tests in `internal/epc` contain literal NBSP/narrow-space characters
  — that is intentional; do not normalize them.
- `go.mod` must stay free of runtime dependencies; gozxing is test-only.
- The segno golden fixtures pair with `scripts/gen_segno_fixtures.py`; if
  you change the cases in `golden_test.go`, regenerate the fixtures and
  keep both in sync.
- Tagging `vX.Y.Z` requires a matching `## [X.Y.Z]` section with entries in
  `CHANGELOG.md`: `release.yml` builds its notes from it and fails the job
  without it. See CONTRIBUTING, "Cutting a release".

## Definition of done

1. `gofmt` clean, `go vet` clean, `go test ./...` green (run them — don't
   assume).
2. A new or changed test has been shown able to fail (temporary mutation or
   reverted fix) — a gate that cannot go red does not count as coverage.
3. README flag table still matches `main.go` if flags changed; CONTRIBUTING
   contracts untouched or the break is called out explicitly.
4. No real IBANs/names anywhere; test IBAN is `DE02120300000000202051`.
