#!/usr/bin/env python3
"""Build the complete, version-matched remote worker source bundle.

No models, databases, credentials, or installed dependencies are packaged.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from zipfile import ZIP_DEFLATED, ZipFile, ZipInfo

SCRIPTS = Path(__file__).resolve().parent
MODULES = (
    "remote_worker_server.py",
    "visual_embedding_server.py",
    "visual_embedding_worker.py",
    "camie_tagger_worker.py",
    "media_conversion_worker.py",
    "media_upscale_worker.py",
    "video_overlap_worker.py",
    "image_restoration_worker.py",
    "remote_worker_check.py",
)
EXTRAS = {
    "remote-worker-README.md": "README.md",
    "remote-worker-requirements.txt": "requirements.txt",
}
BUNDLE_ROOT = "stashbooru-remote-worker"
ZIP_TIMESTAMP = (2026, 1, 1, 0, 0, 0)


def build_bundle(output: Path, scripts: Path = SCRIPTS) -> None:
    contents = {name: (scripts / name).read_bytes() for name in MODULES}
    contents.update({
        archive_name: (scripts / source).read_bytes()
        for source, archive_name in EXTRAS.items()
    })
    manifest = {
        "format": 1,
        "scripts": list(MODULES),
        "sha256": {
            name: hashlib.sha256(data).hexdigest()
            for name, data in sorted(contents.items())
        },
    }
    contents["manifest.json"] = (
        json.dumps(manifest, indent=2, sort_keys=True) + "\n"
    ).encode("utf-8")
    output.parent.mkdir(parents=True, exist_ok=True)
    with ZipFile(output, "w", compression=ZIP_DEFLATED, compresslevel=9) as archive:
        for name, data in sorted(contents.items()):
            info = ZipInfo(f"{BUNDLE_ROOT}/{name}", ZIP_TIMESTAMP)
            info.compress_type = ZIP_DEFLATED
            info.external_attr = 0o644 << 16
            archive.writestr(info, data)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=Path("dist/stash-remote-worker.zip"))
    args = parser.parse_args()
    build_bundle(args.output)
    print(f"Created {args.output}")


if __name__ == "__main__":
    main()
