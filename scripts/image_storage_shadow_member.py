#!/usr/bin/env python3
"""Reconstruct one P12/P13 member as bytes for a disconnected shadow endpoint.

The caller owns archive, member identity, source path and SHA-256. No source or
archive is modified. No files are created and no native media route is replaced.
"""
from __future__ import annotations

import argparse
import hashlib
import os
from pathlib import Path
import stat
import sys

import image_delta_storage as delta
import image_storage_pack_proof as proof


def regular_bytes(path):
    location = Path(path)
    if not location.is_absolute() or location.is_symlink():
        raise ValueError("shadow source must be an absolute, non-symlink file")
    meta = os.lstat(location)
    if not stat.S_ISREG(meta.st_mode) or meta.st_size <= 0 or meta.st_size > delta.MAX_FILE:
        raise ValueError("shadow source must be a bounded regular file")
    return delta.read_regular(location, delta.MAX_FILE)


def shadow_member(archive, method, member, digest, source):
    if method not in proof.METHODS:
        raise ValueError("only byte-exact P12/P13 candidates are allowed")
    if not isinstance(member, str) or not member.startswith("image:"):
        raise ValueError("invalid image identity")
    if not isinstance(digest, str) or not delta.HASH.fullmatch(digest):
        raise ValueError("invalid original SHA-256")
    source_bytes = regular_bytes(source)
    if hashlib.sha256(source_bytes).hexdigest() != digest:
        raise ValueError("current native original differs from verified candidate")
    archive = Path(archive)
    if not archive.is_absolute() or archive.is_symlink():
        raise ValueError("shadow archive must be an absolute non-symlink path")
    with proof.open_reader(method, archive) as reader:
        fidelity = reader.manifest["fidelity"] if method == "delta" else reader.index["fidelity"]
        if fidelity != "byte-exact" or member not in reader.entries:
            raise ValueError("member is not available as byte-exact data")
        reconstructed = reader.reconstruct(member)
    if len(reconstructed) != len(source_bytes):
        raise ValueError("reconstruction has unexpected length")
    if hashlib.sha256(reconstructed).hexdigest() != digest or reconstructed != source_bytes:
        raise ValueError("shadow bytes do not match the current original")
    return reconstructed


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive", required=True)
    parser.add_argument("--method", choices=proof.METHODS, required=True)
    parser.add_argument("--member", required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--original", required=True)
    args = parser.parse_args(argv)
    try:
        data = shadow_member(args.archive, args.method, args.member, args.sha256, args.original)
        sys.stdout.buffer.write(data)
        sys.stdout.buffer.flush()
        return 0
    except (ValueError, OSError, KeyError, TypeError) as error:
        print("shadow serving refused: " + str(error), file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("shadow serving interrupted; no live files changed", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
