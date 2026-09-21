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
];
for (const name of ['index.html', 'app.js', 'style.css']) {
  const src = readFileSync(join(root, 'web', name), 'utf8');
  const hits = forbidden.filter(([re]) => re.test(src)).map(([, what]) => what);
  if (hits.length) fail(`web/${name} violates the zero-storage contract: ${hits.join(', ')}`);
  else console.log(`ok   web/${name} names no storage, address-bar write or external resource`);
}

process.exit(failures ? 1 : 0);
