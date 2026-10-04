#!/usr/bin/env python3
"""Repeatable P12 benchmark: repository artwork derivatives plus synthetic cases.

These are labelled acceptance fixtures, not a measured production-library corpus.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import sys
import tempfile

import numpy as np
from PIL import Image, ImageDraw, ImageOps, PngImagePlugin

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_delta_storage as storage

REPO = Path(__file__).resolve().parents[2]


def fixtures(root):
    root = Path(root)
    asset = REPO / "docs/readme_assets/demo_image.png"
    with Image.open(asset) as source:
        base = source.convert("RGBA").resize((640, 376), Image.Resampling.LANCZOS)
    paths = {}

    def save(name, image, **kwargs):
        path = root / name
        image.save(path, **kwargs)
        paths[name] = path
        return name

    save("base.png", base)
    for index in range(2):
        paths[f"copy{index}.png"] = root / f"copy{index}.png"
        paths[f"copy{index}.png"].write_bytes(paths["base.png"].read_bytes())
    for index in range(2):
        edit = base.copy()
        ImageDraw.Draw(edit).rectangle((64 + index * 12, 54, 90 + index * 12, 72), fill=(37, 127, 231, 201))
        save(f"local{index}.png", edit)
    save("border.png", ImageOps.expand(base, border=(12, 9, 12, 9), fill=(0, 0, 0, 0)))
    translated = Image.new("RGBA", base.size)
    translated.paste(base, (7, 4))
    save("translated.png", translated)
    save("half.png", base.resize((320, 188), Image.Resampling.LANCZOS))
    save("large.png", base.resize((800, 470), Image.Resampling.LANCZOS))
    for quality in (95, 90, 80):
        save(f"jpeg{quality}.jpg", base.convert("RGB"), quality=quality)
    # Separate JPEG/PNG containers decoding to the same RGB content.
    with Image.open(paths["jpeg90.jpg"]) as jpeg:
        save("jpeg90-decoded.png", jpeg.convert("RGB"))
    altered_alpha = np.array(base)
    altered_alpha[130:190, 210:300, 3] = 0
    save("alpha.png", Image.fromarray(altered_alpha))
    altered_alpha[140, 230, 0] ^= 1
    save("alpha-hidden-edit.png", Image.fromarray(altered_alpha))
    rng = np.random.default_rng(20261004)
    for index in range(2):
        save(f"random{index}.png", Image.fromarray(rng.integers(0, 256, (376, 640, 4), dtype=np.uint8)))
    high = rng.integers(0, 65536, (188, 320), dtype=np.uint16)
    save("gray16.png", Image.fromarray(high))
    high[20:28, 30:42] ^= 1
    save("gray16-edit.png", Image.fromarray(high))
    metadata = PngImagePlugin.PngInfo()
    metadata.add_text("Description", "Synthetic metadata retention acceptance fixture")
    exif = Image.Exif()
    exif[274] = 6
    save("metadata.png", base, pnginfo=metadata, exif=exif)
    cases = [
        ("exact-copies", ["base.png", "copy0.png", "copy1.png"], "repository screenshot + exact copies"),
        ("localized-edits", ["base.png", "local0.png", "local1.png"], "repository screenshot + inserted rectangles"),
        ("translation-borders", ["base.png", "border.png", "translated.png"], "repository screenshot + known geometry"),
        ("different-resolutions", ["base.png", "half.png", "large.png"], "repository screenshot + Lanczos resizes"),
        ("recompressed-jpegs", ["jpeg95.jpg", "jpeg90.jpg", "jpeg80.jpg"], "repository screenshot encoded at three JPEG qualities"),
        ("cross-format-same-pixels", ["jpeg90.jpg", "jpeg90-decoded.png"], "JPEG + lossless PNG of its decoded pixels"),
        ("unrelated-noise", ["random0.png", "random1.png"], "independent seeded random planes"),
        ("alpha-hidden-rgb", ["base.png", "alpha.png", "alpha-hidden-edit.png"], "alpha change + invisible one-level RGB edit"),
        ("high-bit-depth", ["gray16.png", "gray16-edit.png"], "seeded full-range uint16 grayscale + one-level edit"),
        ("metadata-orientation", ["base.png", "metadata.png"], "identical stored pixels + text/Exif orientation")]
    stacks = []
    for identity, names, label in cases:
        members = [{"id": f"fixture:{identity}:{index}", "path": str(paths[name])} for index, name in enumerate(names)]
        if identity == "translation-borders":
            members[2]["predictions"] = {members[0]["id"]: [{"width": 640, "height": 376, "left": 7, "top": 4}]}
        stacks.append({"id": identity, "representative": members[-1]["id"], "members": members})
    provenance = {"kind": "repository-asset derivatives and synthetic fixtures; not production-library calibration",
                  "asset": "docs/readme_assets/demo_image.png", "assetSHA256": hashlib.sha256(asset.read_bytes()).hexdigest(),
                  "randomSeed": 20261004, "cases": {identity: label for identity, _, label in cases}}
    return stacks, provenance


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output")
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix="p12-labelled-fixtures-") as root:
        groups, provenance = fixtures(root)
        result = storage.benchmark(groups)
        result["fixtureProvenance"] = provenance
        if args.output:
            storage.save_json(result, args.output)
    for row in result["results"]:
        if not row.get("verified"):
            print(json.dumps(row), file=sys.stderr)
            return 1
        print(f"{row['group']:26s} {row['fidelity']:11s} {row['originalFileBytes']:9d} -> {row['archiveBytes']:9d} bytes {row['layout']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
