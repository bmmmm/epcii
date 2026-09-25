# epcii

Generate EPC QR codes ("GiroCode") for SEPA credit transfers — a single
static binary with zero runtime dependencies, including its own QR encoder
core. The same code runs in the browser at
<https://bmmmm.github.io/epcii/> (see [Web](#web)): no server, no storage.

An EPC QR code ([EPC069-12](https://www.europeanpaymentscouncil.eu/document-library/guidance-documents/quick-response-code-guidelines-enable-data-capture-initiation))
encodes recipient, IBAN, amount, and remittance text so a banking app can
pre-fill a SEPA transfer from a single scan.

## Install

No install: open <https://bmmmm.github.io/epcii/>.

For the command line, requires Go 1.26 or newer:

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

## Web

The same generator runs in the browser at
<https://bmmmm.github.io/epcii/>: the Go code is compiled to WebAssembly
(`cmd/epcii-wasm`), so validation, encoder and renderers are the ones above,
and the SVG and PNG you download are byte-identical to the CLI's output —
`scripts/web-smoke.mjs` proves that in CI on every change.

The form shows name, IBAN, amount and remittance text; reference, BIC,
purpose code and the note to the payer are folded under "More fields", since
IBAN-only is the SEPA norm (a BIC is only needed for accounts outside the
EEA). Every field of the flag table above is available and travels in share
links. The page is in English and German; the switch keeps nothing.

### Privacy

Name and IBAN are processed by this page and nowhere else. Three things hold
that up, and each of them is gated rather than promised:

- **No server, no storage.** GitHub Pages serves a handful of static files;
  there is no backend, no cookie, no local storage, no service worker, no
  analytics, and nothing loaded from another origin. Once the page is up,
  the only URL its script requests is `epcii.wasm` (twice, if the browser
  forces the non-streaming fallback), and no field you type is ever part of
  a request. GitHub therefore learns what any web host learns from serving a
  page (address, time, user agent) and nothing about the payment. The
  links in the footer point at github.com and bezahlbrudi.jetzt, but they
  are links: nothing is fetched from there unless you click, and
  `referrer: no-referrer` means a click carries nothing with it.
- **Content Security Policy.** Pages cannot send HTTP headers, so the policy
  is a `<meta>` tag: `default-src 'none'`, scripts and styles only from the
  page's own origin, `connect-src 'self'`, `form-action 'none'`,
  `base-uri 'none'`, no inline script. No subresource, no XHR or `fetch`,
  and no form post can reach another origin. What a meta CSP cannot do is
  also worth stating: `frame-ancestors`, `sandbox` and `report-uri` are
  ignored in it; `form-action 'none'` stops a form post, but no CSP without
  `navigate-to` restrains a *scripted* top-level navigation, and none of
  them covers WebRTC — code running on this page could still carry data
  away that way. The policy narrows what a bug can do; it is not a substitute
  for the page being small enough to read.
- **Gated, not asserted.** `scripts/web-smoke.mjs` greps the page sources
  for storage APIs, address-bar writes, external resources and HTML string
  sinks, and pins the CSP directives above, read the way a browser reads
  them: deleting the `<meta>` tag, commenting it out, widening
  `connect-src` or prepending a permissive duplicate all fail the run.
  `.github/workflows/pages.yml` runs that gate in the job the deploy depends
  on. The two claims above therefore break the build instead of quietly
  drifting out of date. The grep is a tripwire, not a proof: it catches the
  APIs it names and URLs written as literals, so it would not by itself
  notice a `sendBeacon`, a `fetch` built from a variable, or an assignment
  to `window.location` without a property. Those are caught by the
  diff being small and reviewed.

Nothing leaves the machine until you ask for it. When you do, it really does
leave, and the page cannot follow it:

- **Share links stay in the fragment — but a link is a link.** "Copy link"
  and "Share" build a URL of the form
  `…/epcii/#v=1&name=…&iban=…&amount=…`; browsers do not send the
  `#fragment` in requests, so payment data reaches neither GitHub's logs nor
  a `Referer`. The address bar is never written automatically: a link exists
  only when you ask for one, and opening one fills the form and renders.
  Beyond that it is an ordinary URL. Whoever opens it has it in their
  browser history, and a browser that syncs history or feeds the address bar
  to a search provider treats it like any other address.
- **Copying and sharing are the real exit.** "Copy link" puts that URL on
  the clipboard, which a synchronised clipboard (Apple Universal Clipboard,
  Windows cloud clipboard) carries to that vendor. Whatever you then paste
  it into — mail, a messenger — sees name and IBAN in clear text inside the
  link: the fragment hides them from a web server, not from the channel you
  send them through. "Share" hands the same URL — and, where the platform
  accepts a file, the SVG — to the operating system's share sheet and to the
  app you pick there. Share such a link as you would share the payment data
  itself.
- **Downloads carry the IBAN in the file name.** The buttons save
  `epc-<IBAN>.svg` / `.png`, named after the payload that was actually
  encoded. That makes a folder of codes sortable, but it also means the IBAN
  is legible in a directory listing, a cloud folder or a backup before
  anyone scans anything.
- **What the page cannot control.** A browser extension can read form fields
  on any page; a CSP does not stop it. `autocomplete="off"` is set on the
  form, but browsers may offer autofill regardless.

Build it locally with `scripts/build-web.sh` (output in `web/dist/`, served
by any static file server) and check it with `node scripts/web-smoke.mjs`
after `go build -o epcii .`.

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
scripts/build-web.sh && node scripts/web-smoke.mjs   # web build + CLI-identity gate
```

The browser build lives in `cmd/epcii-wasm` (a `js && wasm` entry point
over `internal/webapi`, the CLI pipeline as one function) and `web/`
(static page, no framework). `scripts/build-web.sh` assembles `web/dist/`;
`pages.yml` deploys it to GitHub Pages on every push to `main`, after the
same smoke gate CI runs on pull requests.

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
module so upstream never enters `go.mod`). The web build adds a fifth:
`scripts/web-smoke.mjs` runs the wasm through Go's `wasm_exec.js` and
compares its SVG and PNG with the CLI byte for byte, then greps the page
sources for storage APIs, address-bar writes, external resources and HTML
string sinks, and pins that the download name comes from the encoded
payload rather than the form.

## Alternatives

♥ Friends with [BezahlBrudi](https://bezahlbrudi.jetzt/) by XenGi
([source](https://git.xengi.de/xengi/bezahlbrudi)) — same idea, its own
flavour: GiroCodes in the browser, with no payment service in between. It
remembers your details on your device and speaks more languages. If that
suits you better, go for it.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) — module map, public contracts, and
test conventions ([AGENTS.md](AGENTS.md) has the condensed version for
coding agents). Bug reports go through the issue forms; suspected
vulnerabilities go through [SECURITY.md](SECURITY.md), never a public
issue.

## License

GPL-3.0-or-later — see [LICENSE](LICENSE). Bundled QR encoder code is
MIT-licensed (compatible); attribution in [NOTICE](NOTICE).
