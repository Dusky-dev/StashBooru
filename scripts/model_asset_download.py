"""Opt-in, atomic Hugging Face model downloads for StashBooru workers."""
from __future__ import annotations
import hashlib
import os
from pathlib import Path
import re
import tempfile
from urllib.request import Request, urlopen

CHUNK = 4 * 1024 * 1024
TRUSTED = {
    "Camais03/camie-tagger-v2": {"camie-tagger-v2.onnx", "camie-tagger-v2-metadata.json"},
    "alpindale/PixAI-v1.0-INT8": {"model.safetensors", "config.json"},
}

def fetch_model_file(repo: str, filename: str, destination: Path) -> None:
    if repo not in TRUSTED or filename not in TRUSTED[repo]:
        raise ValueError("model source is not approved")
    destination.parent.mkdir(parents=True, exist_ok=True)
    url = f"https://huggingface.co/{repo}/resolve/main/{filename}?download=true"
    fd, temporary = tempfile.mkstemp(prefix=destination.name + ".", suffix=".part", dir=destination.parent)
    os.close(fd)
    part = Path(temporary)
    try:
        req = Request(url, headers={"User-Agent": "StashBooru-model-installer/1"})
        with urlopen(req, timeout=120) as stream, part.open("wb") as target:
            etag = (stream.headers.get("X-Linked-ETag") or stream.headers.get("ETag") or "").strip('"')
            if not re.fullmatch(r"[0-9a-f]{64}", etag):
                raise RuntimeError(f"missing SHA-256 checksum from Hugging Face for {filename}")
            digest = hashlib.sha256()
            total = 0
            expected = stream.headers.get("Content-Length")
            while True:
                data = stream.read(CHUNK)
                if not data:
                    break
                target.write(data)
                digest.update(data)
                total += len(data)
            if expected and expected.isdecimal() and total != int(expected):
                raise RuntimeError(f"incomplete {filename} download: {total} of {expected} bytes")
            if not total or digest.hexdigest() != etag:
                raise RuntimeError(f"SHA-256 verification failed for {filename}")
        os.replace(part, destination)
    finally:
        part.unlink(missing_ok=True)
