## Claims

<!-- One bullet per user-visible behaviour change. Name symbols and files in
     backticks, reference issues as #N. Write each claim so a reviewer can
     check it against the diff instead of inferring intent. -->

-

## Verification

<!-- Paste real command output. "Should work" is not verification. For
     anything touching the encoder or payload builder, include the relevant
     `go test ./...` run; for CLI changes, the actual invocation + output. -->

```
```

## Tests

<!-- Name the specific test(s) covering this change, and confirm each fails
     without the change (revert your fix locally, or state why a red state
     is impossible). A gate that cannot go red proves nothing. -->

-

## Public contracts

Confirm none of these changed, or list the break under Claims:

- [ ] stdout carries only the SVG — previews, details, and errors go to stderr
- [ ] exit codes: `0` success, `2` invalid input/usage, `1` I/O failure
- [ ] CLI flag names and semantics are unchanged
- [ ] payload format: EPC069-12 v002, LF separators, UTF-8 (charset `1`),
      two-decimal amounts, trailing empty fields trimmed
- [ ] zero runtime dependencies (test-only dependencies are fine)

## Out of scope

<!-- What this PR deliberately does not touch, so the reviewer doesn't hunt
     for it. -->

-

## AI assistance

- [ ] I reviewed every line of this PR and can explain it
- [ ] I ran the verification commands myself
