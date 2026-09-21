# Security Policy

epcii generates payment QR codes. The security-relevant failure mode is a
generated code that initiates a transfer differing from what the user
requested.

## In scope

- **Payload integrity:** any input that makes the encoded payload differ
  from the validated input — e.g. field-separator injection (line breaks
  smuggled into a field shifting subsequent fields), charset confusion, or
  validation bypasses that let a malformed IBAN/amount into a scannable
  code.
- **Validation bypasses:** structurally invalid IBANs, amounts outside
  0.01–999999999.99, or oversized payloads that still produce a QR code.
- Crashes or hangs on crafted input (the tool may process untrusted
  invoice data in pipelines).
- **Data flow of the web version:** anything in
  <https://bmmmm.github.io/epcii/> that moves an entered name, IBAN or
  amount off the page other than through the share link, download or
  share sheet the user asked for — a request carrying field data, a
  write to storage or the address bar, a resource pulled from another
  origin. The contract those claims rest on is README “Privacy”.

## Out of scope

- Misuse of correctly generated codes (phishing with a code that encodes
  exactly what the operator asked for) — that is the operator's payload,
  faithfully encoded.
- Vulnerabilities in banking apps that scan the codes.
- Vulnerabilities in test-only dependencies (gozxing, segno) — they are
  not part of the shipped binary; still, feel free to mention them.

## Reporting

Please do **not** open a public issue for a suspected vulnerability.

- Preferred: GitHub private vulnerability reporting —
  <https://github.com/bmmmm/epcii/security/advisories/new>
- Alternatively: email `hi@brtsz.de`

You'll get an acknowledgement within a few days. Please include the exact
command, input, and what the encoded result was (the `--details` output
helps).
