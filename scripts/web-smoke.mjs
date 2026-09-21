#!/usr/bin/env node
// Gate: the web build must produce exactly what the CLI produces.
//
// Loads web/dist/epcii.wasm through the same wasm_exec.js the browser uses,
// calls globalThis.epcii.generate with a fixed input, and compares the SVG
// and PNG byte for byte with `epcii` run on the same input. A second case
// checks that an invalid IBAN surfaces as an error, not an exception.
//
// Usage: node scripts/web-smoke.mjs [path/to/epcii-binary]   (default ./epcii)
// Requires a prior `go build -o epcii .` and `scripts/build-web.sh`.
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const dist = join(root, 'web', 'dist');
const cli = resolve(process.argv[2] || join(root, 'epcii'));

const input = {
  name: 'Test Persona',
  iban: 'DE02120300000000202051',
  amount: '12,5',
  text: 'invoice 42',
};

// --- reference from the CLI
const tmp = mkdtempSync(join(tmpdir(), 'epcii-smoke-'));
const pngPath = join(tmp, 'ref.png');
const refSVG = execFileSync(cli, [
  '--name', input.name, '--iban', input.iban, '--amount', input.amount,
  '--text', input.text, '--png', pngPath,
]);
const refPNG = readFileSync(pngPath);
rmSync(tmp, { recursive: true, force: true });

// --- the wasm build, loaded like the browser does (plus node's globals)
globalThis.fs = require('node:fs');
globalThis.path = require('node:path');
require(join(dist, 'wasm_exec.js'));
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(readFileSync(join(dist, 'epcii.wasm')), go.importObject);
go.run(instance); // never resolves: main blocks in select{}
for (let i = 0; i < 100 && !globalThis.epcii; i++) await new Promise((r) => setTimeout(r, 10));
if (!globalThis.epcii) fail('globalThis.epcii was not registered by the wasm module');

// --- compare
let failures = 0;
function fail(msg) {
  console.error('FAIL: ' + msg);
  failures++;
}
function firstDiff(a, b) {
  const n = Math.min(a.length, b.length);
  for (let i = 0; i < n; i++) if (a[i] !== b[i]) return i;
  return a.length === b.length ? -1 : n;
}
function expectEqual(label, got, want) {
  const at = firstDiff(got, want);
  if (at === -1) console.log(`ok   ${label} (${got.length} bytes identical)`);
  else fail(`${label} differs at byte ${at} (web ${got.length} bytes, cli ${want.length} bytes)`);
}

const res = globalThis.epcii.generate(input);
if (res.error) fail(`unexpected error: ${res.error}`);
expectEqual('svg', Buffer.from(res.svg, 'utf8'), refSVG);
expectEqual('png', Buffer.from(res.png), refPNG);
if (typeof res.version !== 'number' || typeof res.size !== 'number') fail('version/size not numbers');

const bad = globalThis.epcii.generate({ ...input, iban: 'DE00' });
if (!bad.error) fail('invalid IBAN did not produce an error');
else if (bad.svg !== '' || bad.png.length !== 0) fail('error result still carries svg/png');
else console.log(`ok   invalid IBAN → "${bad.error}"`);

if (typeof globalThis.epcii.version !== 'string' || !globalThis.epcii.version) fail('version string missing');
else console.log(`ok   version ${globalThis.epcii.version}`);

// --- zero-storage contract (CONTRIBUTING "The web version is the CLI"):
// the page sources must not name a storage API, write the address bar, or
// pull anything from another origin. A grep gate, but one that can go red.
const forbidden = [
  [/localStorage|sessionStorage|indexedDB|document\.cookie|serviceWorker|caches\./, 'storage API'],
  [/history\.(pushState|replaceState|go|back|forward)|location\.(hash|href|search|assign|replace)\s*[=(]/, 'address bar write'],
  [/<(script|link|img|iframe)[^>]+(src|href)=["']https?:/i, 'external resource tag'],
  [/@import|url\(\s*["']?https?:/i, 'external stylesheet resource'],
  [/\bimport\s*\(|\bfetch\(\s*["']https?:/, 'dynamic import / cross-origin fetch'],
  // The SVG is inserted as a parsed XML node, never as an HTML string: the
  // "render.SVG emits only geometry" invariant belongs to another module,
  // and a string sink here would turn any future change there into script.
  [/\binnerHTML\b|\bouterHTML\s*=|insertAdjacentHTML|document\.write\s*\(/, 'HTML string sink'],
];
for (const name of ['index.html', 'app.js', 'style.css']) {
  const src = readFileSync(join(root, 'web', name), 'utf8');
  const hits = forbidden.filter(([re]) => re.test(src)).map(([, what]) => what);
  if (hits.length) fail(`web/${name} violates the zero-storage contract: ${hits.join(', ')}`);
  else console.log(`ok   web/${name} names no storage, address-bar write, external resource or HTML string sink`);
}

// The CSP is the other half of the privacy contract (README "Privacy"), and
// the greps above cannot see it: removing the meta tag leaves every one of
// them green. GitHub Pages cannot send headers, so the policy has no second
// home -- pin the directives the page's claims rest on.
//
// Parsed the way a browser reads it, or the gate lies in both directions:
// comments are stripped (a commented-out policy is a missing one), the first
// mention of a directive wins (a browser ignores later duplicates, so a
// permissive one prepended must not hide a pinned one below), names are
// case-insensitive, and values are compared as a set so reordering them is
// not a false alarm.
{
  const html = readFileSync(join(root, 'web', 'index.html'), 'utf8').replace(/<!--[\s\S]*?-->/g, '');
  const tag = html.match(/<meta\s+http-equiv="Content-Security-Policy"[^>]*?content="([^"]*)"/i);
  if (!tag) {
    fail('web/index.html: no Content-Security-Policy meta tag with a content attribute');
  } else {
    const norm = (v) => v.split(/\s+/).filter(Boolean).sort().join(' ');
    const got = new Map();
    for (const d of tag[1].split(';').map((s) => s.trim()).filter(Boolean)) {
      const [name, ...vals] = d.split(/\s+/);
      const key = name.toLowerCase();
      if (!got.has(key)) got.set(key, norm(vals.join(' ')));
    }
    const required = [
      ['default-src', "'none'"],
      ['connect-src', "'self'"],
      ['form-action', "'none'"],
      ['base-uri', "'none'"],
      ['script-src', "'self' 'wasm-unsafe-eval'"],
      ['style-src', "'self'"],
    ];
    const bad = required
      .filter(([k, v]) => got.get(k) !== norm(v))
      .map(([k, v]) => `${k} must be "${v}", got "${got.get(k) ?? '(absent)'}"`);
    if (bad.length) fail('web/index.html CSP: ' + bad.join('; '));
    else console.log('ok   web/index.html CSP pins ' + required.map(([k]) => k).join(', '));
  }
}

// Everything a share carries must describe one and the same payment. The
// file name, the link and the attached image all come from the last encoded
// result; reading the form instead would let them disagree, because
// rendering is debounced (150 ms) and the form can already be ahead. A link
// saying IBAN B next to an image encoding IBAN A is the failure this pins.
//
// The body is cut by matching braces, not by the next line starting with
// "}", which a nested block would end early, hiding whatever follows it
// inside the function. Braces in strings and comments are still counted,
// so a template literal can still cut the slice short -- the snapshot
// check below does not rely on the slice and catches what it would miss.
function bodyOf(src, fn) {
  const start = src.indexOf(`function ${fn}(`);
  if (start === -1) return null;
  let depth = 0;
  for (let i = src.indexOf('{', start); i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}' && --depth === 0) return src.slice(start, i + 1);
  }
  return null;
}

{
  const src = readFileSync(join(root, 'web', 'app.js'), 'utf8');
  for (const fn of ['fileStem', 'shareParams']) {
    const body = bodyOf(src, fn);
    if (!body) {
      fail(`web/app.js: ${fn}() not found -- its gate has nothing to check`);
    } else if (/\breadForm\b|\$\(|\.value\b|document\./.test(body)) {
      fail(`web/app.js: ${fn}() reads the form instead of the encoded result`);
    } else {
      console.log(`ok   web/app.js ${fn}() derives its value from the encoded result`);
    }
  }

  // Both of those only hold because render() snapshots the fields onto the
  // result before publishing it. Without that line shareParams() has nothing
  // to read, so gate the assignment itself, and its order.
  const render = bodyOf(src, 'render');
  const snapshot = render ? render.indexOf('res.fields = fields;') : -1;
  const publish = render ? render.indexOf('last = res;') : -1;
  const share = bodyOf(src, 'shareParams');
  if (!render) fail('web/app.js: render() not found -- the snapshot gate has nothing to check');
  else if (snapshot === -1) fail('web/app.js: render() does not snapshot the encoded fields onto the result');
  else if (publish === -1 || snapshot > publish) fail('web/app.js: render() publishes the result before snapshotting its fields');
  else if (share && !share.includes('last.fields')) fail('web/app.js: shareParams() does not read the snapshot');
  else console.log('ok   web/app.js render() snapshots the encoded fields before publishing the result');
}

process.exit(failures ? 1 : 0);
