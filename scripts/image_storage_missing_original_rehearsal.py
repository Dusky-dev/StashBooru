#!/usr/bin/env python3
"""Rehearse recovering originals that may be missing, but only in private scratch.

Consumes one immutable Go candidate receipt and a verified media backup snapshot.
It NEVER opens original source files, discovers live catalogue paths, or writes
outside a fresh sibling scratch directory. This does not restore StashBooru.
"""
from __future__ import annotations

import argparse
from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

import image_delta_storage as delta
import image_storage_candidate_reconstruct as reconstruct
import image_storage_pack_proof as packing
import media_library_backup as backup

IDENTIFIER = re.compile(r"^stack-[1-9][0-9]*-(delta|shared-tiles)-[0-9a-f]{24}$")
MAX_RECEIPT = 128 * 1024
MAX_ARCHIVE = 384 * 1024 * 1024

TRUE_FLAGS = ("verified", "backupVerified", "sourcesRetained",
              "nativePackingCandidateOnly")
FALSE_FLAGS = ("nativeServing", "nativeServingVerified",
               "databaseBackupVerified", "originalEvictionEnabled",
               "archiveActivated")


def _hash_regular(path: Path, max_bytes: int) -> tuple[int, str]:
    """Reject symlink opening and ensure file identity, size and time are stable."""
    if not path.is_absolute() or path.is_symlink():
        raise ValueError("expected an absolute non-symlink regular file")
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or not 0 < before.st_size <= max_bytes:
        raise ValueError("invalid bounded regular file")
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    digest = hashlib.sha256()
    with os.fdopen(os.open(path, flags), "rb") as inp:
        opened = os.fstat(inp.fileno())
        if not stat.S_ISREG(opened.st_mode) or opened.st_ino != before.st_ino or opened.st_dev != before.st_dev:
            raise ValueError("file identity changed while opening")
        total = 0
        while data := inp.read(1024 * 1024):
            total += len(data)
            if total > max_bytes:
                raise ValueError("file grew beyond limit")
            digest.update(data)
        after_open = os.fstat(inp.fileno())
    after_path = path.lstat()
    stamp = lambda x: (x.st_dev, x.st_ino, x.st_size, x.st_mtime_ns, x.st_ctime_ns)
    if total != before.st_size or stamp(before) != stamp(after_open) or stamp(before) != stamp(after_path):
        raise ValueError("file changed during verification")
    return total, digest.hexdigest()


def _strict_root(path: Path) -> Path:
    if not path.is_absolute() or not path.is_dir() or path.is_symlink():
        raise ValueError("private root must exist and not be a symlink")
    resolved = path.resolve(strict=True)
    if resolved != path:
        raise ValueError("private root has a symlink or noncanonical ancestor")
    return path


def _overlap(a: Path, b: Path) -> bool:
    return a == b or a.is_relative_to(b) or b.is_relative_to(a)


def _read_receipt(receipt: Path):
    if not receipt.name.endswith(".receipt.json"):
        raise ValueError("not a versioned candidate receipt")
    candidate_id = receipt.name[:-len(".receipt.json")]
    if not IDENTIFIER.fullmatch(candidate_id):
        raise ValueError("invalid server-generated candidate identifier")
    size, _ = _hash_regular(receipt, MAX_RECEIPT)
    if size == 0:
        raise ValueError("empty saved receipt")
    record = json.loads(delta.read_regular(receipt, MAX_RECEIPT))
    if not isinstance(record, dict) or type(record.get("version")) is not int or record["version"] != 1:
        raise ValueError("invalid persistent receipt version")
    expected_archive = record.get("archiveSHA256")
    proof = record.get("report")
    if not isinstance(expected_archive, str) or not delta.HASH.fullmatch(expected_archive) or not isinstance(proof, dict):
        raise ValueError("invalid saved archive proof")
    for flag in TRUE_FLAGS:
        if proof.get(flag) is not True:
            raise ValueError("candidate missing required safety evidence: " + flag)
    for flag in FALSE_FLAGS:
        if proof.get(flag) is not False:
            raise ValueError("candidate has unsupported state: " + flag)
    method, group, snapshot = proof.get("method"), proof.get("group"), proof.get("mediaBackupSnapshot")
    if method not in packing.METHODS or not isinstance(group, str) or not re.fullmatch(r"stack:[1-9][0-9]*", group):
        raise ValueError("candidate identity is invalid")
    if not candidate_id.startswith(group.replace(":", "-") + "-" + method + "-"):
        raise ValueError("saved candidate ID disagrees with proof")
    if not isinstance(snapshot, str) or not backup.SNAPSHOT.fullmatch(snapshot):
        raise ValueError("candidate snapshot is invalid")
    expected = proof.get("sourceSHA256")
    if not isinstance(expected, dict) or not 2 <= len(expected) <= 16:
        raise ValueError("invalid archived source SHA-256 evidence")
    if type(proof.get("sourceFiles")) is not int or proof["sourceFiles"] != len(expected):
        raise ValueError("source file count disagrees with saved receipt")
    for key, digest in expected.items():
        if not isinstance(key, str) or not re.fullmatch(r"image:[1-9][0-9]*", key) or not isinstance(digest, str) or not delta.HASH.fullmatch(digest):
            raise ValueError("invalid saved original image identity or SHA-256")
    if proof.get("fidelity") != "byte-exact":
        raise ValueError("saved archive is not byte exact")
    archive = receipt.parent / (candidate_id + ".pack")
    if proof.get("archivePath") != str(archive):
        raise ValueError("saved archive path differs from private receipt directory")
    length, actual = _hash_regular(archive, MAX_ARCHIVE)
    if actual != expected_archive or type(proof.get("archiveBytes")) is not int or proof["archiveBytes"] != length:
        raise ValueError("candidate archive hash or size differs from immutable saved receipt")
    return archive, proof


def rehearse(receipt: Path, store: Path, output: Path) -> dict:
    receipt, store, output = Path(receipt), Path(store), Path(output)
    if not receipt.is_absolute() or not store.is_absolute() or not output.is_absolute():
        raise ValueError("all paths must be absolute")
    private_root = _strict_root(receipt.parent)
    store = _strict_root(store)
    if output.parent != private_root or output.exists() or output.is_symlink():
        raise ValueError("rehearsal output must be a new direct child of the private candidate directory")
    if _overlap(private_root, store):
        raise ValueError("candidate root overlaps immutable media backup store")
    archive, proof = _read_receipt(receipt)
    snapshot = proof["mediaBackupSnapshot"]
    manifest = backup._load_manifest(store, snapshot)
    for root in manifest["roots"].values():
        original_root = Path(root)
        if not original_root.is_absolute() or original_root.is_symlink() or _overlap(private_root, original_root.resolve(strict=False)):
            raise ValueError("candidate scratch output overlaps a configured original-media root")
    verified = backup.verify(store, snapshot)
    if verified.get("verified") is not True or verified.get("snapshot") != snapshot:
        raise ValueError("media backup snapshot verification failed")
    # Without the current database, only content digests can be correlated:
    # this is NOT proof of each image ID's historical backup member path.
    available = Counter(entry["sha256"] for entry in manifest["entries"])
    selected = Counter(proof["sourceSHA256"].values())
    if any(available[digest] < count for digest, count in selected.items()):
        raise ValueError("verified media backup lacks required original-image content digests")
    # Existing reconstruct() proves all archive entries before writing, and
    # copies each into a new scratch directory, never into catalogue paths.
    recovered = reconstruct.reconstruct(proof, output)
    return {
        "verified": True,
        "readOnlyAgainstLibrary": True,
        "originalSourcesNotRead": True,
        "requiresOriginalSources": False,
        "archiveSHA256Verified": True,
        "mediaBackupObjectsVerified": True,
        "backupContentDigestsCovered": True,
        "sourceToBackupPathMappingVerified": False,
        "databaseRecoveryVerified": False,
        "nativeMediaServingVerified": False,
        "crashSafeLiveRollbackVerified": False,
        "canActivate": False,
        "originalEvictionEnabled": False,
        "candidateID": receipt.name[:-len(".receipt.json")],
        "snapshot": snapshot,
        "outputPath": recovered["outputPath"],
        "restoredFiles": recovered["restoredFiles"],
        "restoredSHA256": recovered["restoredSHA256"],
        "disclaimer": "Restored private copies only; no live file, database or configuration was recovered.",
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--receipt", required=True)
    parser.add_argument("--store", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args(argv)
    try:
        report = rehearse(Path(args.receipt), Path(args.store), Path(args.output))
        print(json.dumps(report, allow_nan=False, sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, json.JSONDecodeError, backup.BackupError) as exc:
        print("missing-source rehearsal refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
