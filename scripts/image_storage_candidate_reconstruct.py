#!/usr/bin/env python3
"""Recover an experimental P12/P13 archive into a NEW isolated directory only.

The trusted native job supplies a private evidence file. Neither this CLI nor
the API may restore into live media paths or modify the catalogue.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import sys

import image_storage_pack_proof as proof
import image_delta_storage as delta
import gallery_block_storage as tiles


def checked_file(path):
    path = Path(path)
    if not path.is_absolute() or path.is_symlink():
        raise ValueError("candidate path must be an absolute regular non-symlink file")
    info = os.lstat(path)
    if not stat.S_ISREG(info.st_mode):
        raise ValueError("candidate is not a regular file")
    return path


def reconstruct(evidence, destination):
    method = evidence.get("method")
    expected = evidence.get("sourceSHA256")
    archive = checked_file(evidence.get("archivePath"))
    destination = Path(destination)
    if method not in proof.METHODS or not isinstance(expected, dict):
        raise ValueError("invalid reconstruction method or source evidence")
    if len(expected) < 2 or len(expected) > 16:
        raise ValueError("candidate evidence requires 2–16 images")
    for identity, digest in expected.items():
        if (not isinstance(identity, str) or not identity.startswith("image:")
                or not isinstance(digest, str) or not delta.HASH.fullmatch(digest)):
            raise ValueError("invalid original image identity or SHA-256")
    if not destination.is_absolute() or destination.exists() or destination.is_symlink():
        raise ValueError("reconstruction output must be a new absolute directory")
    if not destination.parent.is_dir() or destination.parent.is_symlink():
        raise ValueError("reconstruction parent must exist and cannot be a symlink")

    # Prove every archived source byte before extracting. The archive reader's
    # native index and checksum verification is independent from this hash map.
    with proof.open_reader(method, archive) as reader:
        proof_result = reader.verify()
        if not proof_result["verified"] or proof_result["fidelity"] != "byte-exact":
            raise ValueError("candidate is not a verified byte-exact archive")
        if set(reader.entries) != set(expected):
            raise ValueError("candidate member IDs differ from retained-original evidence")
        for identity in expected:
            restored = reader.reconstruct(identity)
            if hashlib.sha256(restored).hexdigest() != expected[identity]:
                raise ValueError("candidate cannot reconstruct original image bytes")

    receipt = delta.extract(archive, destination) if method == "delta" else tiles.extract(archive, destination)
    try:
        if not receipt["complete"] or receipt["fidelity"] != "byte-exact":
            raise ValueError("reconstruction did not complete in byte-exact mode")
        members = receipt.get("members")
        if not isinstance(members, list) or len(members) != len(expected):
            raise ValueError("reconstructed image count differs from evidence")
        names = set()
        digests = {}
        for member in members:
            identity, name = member["id"], member["name"]
            if (identity not in expected or identity in digests
                    or not isinstance(name, str) or Path(name).name != name or name in names):
                raise ValueError("unexpected restored image identity or filename")
            names.add(name)
            exported = checked_file(destination / name)
            digest = hashlib.sha256(delta.read_regular(exported, delta.MAX_FILE)).hexdigest()
            if digest != expected[identity] or member["exportSHA256"] != digest:
                raise ValueError("disk reconstruction no longer matches original SHA-256")
            digests[identity] = digest
        if digests != expected:
            raise ValueError("missing reconstructed image evidence")
        if len(list(destination.iterdir())) != len(expected) + 1:
            raise ValueError("unexpected extra reconstruction output")
        return {
            "verified": True,
            "byteExact": True,
            "complete": True,
            "sourcesRetained": True,
            "originalEvictionEnabled": False,
            "nativeServingVerified": False,
            "archiveActivated": False,
            "databaseModified": False,
            "method": method,
            "group": evidence["group"],
            "mediaBackupSnapshot": evidence["mediaBackupSnapshot"],
            "restoredFiles": len(expected),
            "restoredSHA256": digests,
            "outputPath": str(destination),
        }
    except BaseException:
        shutil.rmtree(destination)
        raise


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--evidence", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args(argv)
    try:
        evidence_path = checked_file(args.evidence)
        evidence = json.loads(delta.read_regular(evidence_path, 128 * 1024))
        result = reconstruct(evidence, args.output)
        print(json.dumps(result, sort_keys=True, allow_nan=False))
        return 0
    except (ValueError, OSError, KeyError, TypeError, json.JSONDecodeError) as exc:
        print("candidate reconstruction refused: " + str(exc), file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("Candidate reconstruction interrupted; originals remain untouched.", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
