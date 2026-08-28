#!/usr/bin/env python3
"""Generate golden EPC payload fixtures from segno's reference implementation.

Run once (network needed for uvx):
    uvx --from segno python scripts/gen_segno_fixtures.py

Writes internal/epc/testdata/segno_<case>.golden with the raw payload bytes
of segno.helpers._make_epc_qr_data (encoding='utf-8' so the charset field is
1, matching epcii). Cases avoid trailing-zero amounts: segno strips them
(EUR580 for 580.00) while epcii keeps two decimals — both spec-conform (the
official EPC069-12 example uses EUR12.3), so amounts are chosen where the
two normalizations agree. The matching inputs live in the Go test
(internal/epc/golden_test.go); keep both in sync.
"""

from pathlib import Path

from segno.helpers import _make_epc_qr_data

CASES = {
    "basic": dict(
        name="ACME GmbH",
        iban="DE02120300000000202051",
        amount="12.34",
        text="Invoice RE-2026-001",
    ),
    "umlauts": dict(
        name="Müller & Söhne GmbH",
        iban="DE02120300000000202051",
        amount="580.55",
        text="Rechnung RE-4a7f — Überweisung",
    ),
    "bic_purpose": dict(
        name="François D'Alsace S.A.",
        iban="FR1420041010050500013M02606",
        bic="BNPAFRPP",
        amount="12.34",
        purpose="GDDS",
        text="Client:Marie Louise La Lune",
    ),
    "reference": dict(
        name="ACME GmbH",
        iban="DE02120300000000202051",
        amount="99.99",
        reference="RF18539007547034",
    ),
    "min_amount": dict(
        name="ACME GmbH",
        iban="DE02120300000000202051",
        amount="0.01",
        text="x",
    ),
    "max_amount": dict(
        name="ACME GmbH",
        iban="DE02120300000000202051",
        amount="999999999.99",
        text="max",
    ),
}


def main() -> None:
    outdir = Path(__file__).resolve().parent.parent / "internal" / "epc" / "testdata"
    outdir.mkdir(parents=True, exist_ok=True)
    for case, kwargs in CASES.items():
        data = _make_epc_qr_data(encoding="utf-8", **kwargs)
        path = outdir / f"segno_{case}.golden"
        path.write_bytes(data)
        print(f"{path.name}: {len(data)} bytes")


if __name__ == "__main__":
    main()
