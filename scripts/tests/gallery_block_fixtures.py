#!/usr/bin/env python3
"""P13 measured gallery fixtures; no owner-library calibration is claimed."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys
import tempfile

import numpy as np
from PIL import Image, ImageDraw, PngImagePlugin

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import gallery_block_storage as storage
import image_delta_storage as base
from image_delta_fixtures import fixtures as image_fixtures


def fixtures(root):
    root = Path(root)
    groups, provenance = image_fixtures(root)
    rng = np.random.default_rng(20261004)
    # Seeded flat-page/control corpus, deliberately distinguished from real scans.
    page = Image.new("RGBA", (640, 384), "white")
    drawing = ImageDraw.Draw(page)
    for y in range(32, 352, 24):
        for x in range(32, 600, 56):
            drawing.rectangle((x, y, x + 35, y + 3), fill="black")
    paths = []
    for index in range(4):
        changed = page.copy()
        ImageDraw.Draw(changed).rectangle((64 + index * 64, 80, 96 + index * 64, 110), fill=(13 + index, 83, 123, 255))
        path = root / f"flat-page{index}.png"
        changed.save(path)
        paths.append(path)
    groups.append({"id": "flat-page-edits", "members": [{"id": f"page:{n}", "path": str(path)} for n, path in enumerate(paths)]})
    provenance["cases"]["flat-page-edits"] = "synthetic line-page drawings with distinct rectangles; not scanned pages"
    # Valid PNGs encoded without DEFLATE compression preserve common stream bytes.
    # Adding a small tEXt chunk shifts subsequent IDAT bytes, exposing CDC/fixed tradeoffs.
    noise = Image.fromarray(rng.integers(0, 256, (192, 256, 4), dtype=np.uint8))
    paths = []
    for index in range(3):
        info = PngImagePlugin.PngInfo()
        if index:
            info.add_text("Description", "stream prefix " + "x" * (index * 13))
        path = root / f"shifted-idat{index}.png"
        noise.save(path, compress_level=0, pnginfo=info)
        paths.append(path)
    groups.append({"id": "uncompressed-png-prefix-shift", "members": [{"id": f"shift:{n}", "path": str(path)} for n, path in enumerate(paths)]})
    provenance["cases"]["uncompressed-png-prefix-shift"] = "seeded noise in PNG DEFLATE level 0, with inserted tEXt before identical IDAT; atypical compressed-image control"
    # Multiple formats, dimensions, alpha and grayscale in the same ordered group.
    names = ["base.png", "alpha.png", "half.png", "gray16.png", "jpeg90.jpg"]
    groups.append({"id": "mixed-gallery", "members": [{"id": f"mixed:{n}", "path": str(root / name)} for n, name in enumerate(names)]})
    provenance["cases"]["mixed-gallery"] = "repository-artwork derivatives, seeded uint16, alpha and JPEG in one gallery"
    return groups, provenance


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output")
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix="p13-labelled-fixtures-") as root:
        groups, provenance = fixtures(root)
        report = storage.benchmark(groups)
        report["fixtureProvenance"] = provenance
        if args.output:
            base.save_json(report, args.output)
    for row in report["results"]:
        if not row.get("verified"):
            print(json.dumps(row), file=sys.stderr)
            return 1
        print(f"{row['gallery']:29s} {row['fidelity']:11s} {row['originalFileBytes']:9d} -> {row['archiveBytes']:9d} bytes {row['layout']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
