#!/usr/bin/env python3
"""Read-only restart audit for private P12/P13 missing-original scratch recovery.

Does NOT verify the archive or backup again, install recovered files, change
catalogue paths, enable serving, or authorize original-file eviction.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

CANDIDATE = re.compile(r"^stack-([1-9][0-9]*)-(delta|shared-tiles)-[0-9a-f]{24}$")
SCRATCH = re.compile(r"^stack-([1-9][0-9]*)-orphan-rehearsal-[0-9a-f]{24}$")
IMAGE_ID = re.compile(r"^image:[1-9][0-9]*$")
SHA256 = re.compile(r"^[0-9a-f]{64}$")
SAFE_NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,200}$")
MAX_JSON = 128 * 1024
MAX_IMAGE = 64 * 1024 * 1024
MAX_FILES = 16


def canonical_dir(path: Path) -> None:
    if not path.is_absolute() or path.is_symlink() or not path.is_dir():
        raise ValueError("expected an existing absolute non-symlink directory")
    if path.resolve(strict=True) != path:
        raise ValueError("directory contains a symlink or noncanonical ancestor")


def read_regular(path: Path, limit: int) -> bytes:
    if not path.is_absolute() or path.is_symlink():
        raise ValueError("unsafe regular-file path")
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or not 0 < before.st_size <= limit:
        raise ValueError("missing, empty or oversized regular file")
    data = bytearray()
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    with os.fdopen(os.open(path, flags), "rb") as handle:
        opened = os.fstat(handle.fileno())
        if (opened.st_dev, opened.st_ino) != (before.st_dev, before.st_ino):
            raise ValueError("file identity changed at open")
        while chunk := handle.read(1024 * 1024):
            data.extend(chunk)
            if len(data) > limit:
                raise ValueError("file exceeds read limit")
        after = os.fstat(handle.fileno())
    final = path.lstat()
    stamp = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns)
    if len(data) != before.st_size or stamp(before) != stamp(after) or stamp(before) != stamp(final):
        raise ValueError("file changed during inspection")
    return bytes(data)


def candidate_sources(receipt: Path) -> tuple[str, dict[str, str]]:
    candidate_id = receipt.name.removesuffix(".receipt.json")
    match = CANDIDATE.fullmatch(candidate_id)
    if not receipt.name.endswith(".receipt.json") or not match:
        raise ValueError("invalid saved candidate receipt name")
    record = json.loads(read_regular(receipt, MAX_JSON))
    if not isinstance(record, dict) or type(record.get("version")) is not int or record["version"] != 1:
        raise ValueError("unsupported saved candidate receipt")
    proof = record.get("report")
    if not isinstance(proof, dict) or proof.get("group") != "stack:" + match.group(1) or proof.get("method") != match.group(2):
        raise ValueError("saved candidate identity or method mismatch")
    if proof.get("fidelity") != "byte-exact" or proof.get("verified") is not True or proof.get("backupVerified") is not True:
        raise ValueError("candidate missing original byte-exact packing evidence")
    sources = proof.get("sourceSHA256")
    if not isinstance(sources, dict) or not 2 <= len(sources) <= MAX_FILES:
        raise ValueError("invalid candidate source digest map")
    if any(not isinstance(k, str) or not IMAGE_ID.fullmatch(k) or
           not isinstance(v, str) or not SHA256.fullmatch(v)
           for k, v in sources.items()):
        raise ValueError("invalid image identity or source SHA-256")
    return candidate_id, sources


def audit(receipt: Path, output: Path) -> dict:
    receipt, output = Path(receipt), Path(output)
    canonical_dir(receipt.parent)
    candidate_id, sources = candidate_sources(receipt)
    match = SCRATCH.fullmatch(output.name)
    if (not output.is_absolute() or output.parent != receipt.parent or not match or
            not candidate_id.startswith("stack-" + match.group(1) + "-")):
        raise ValueError("scratch output is not a server-named direct child for this stack")

    # All results retain unsafe-for-activation flags, including a clean audit.
    result = {
        "candidateID": candidate_id,
        "outputPath": str(output),
        "state": "no_output",
        "complete": False,
        "scratchBytesVerified": 0,
        "scratchFilesVerified": 0,
        "candidateArchiveReverified": False,
        "mediaBackupReverified": False,
        "historicalPathMappingVerified": False,
        "databaseRecoveryVerified": False,
        "nativeServingVerified": False,
        "crashSafeLiveRollbackVerified": False,
        "canActivate": False,
        "originalEvictionEnabled": False,
        "readOnlyInspection": True,
    }
    if not output.exists() and not output.is_symlink():
        return result
    canonical_dir(output)
    names = set()
    for entry in output.iterdir():
        if not SAFE_NAME.fullmatch(entry.name) or entry.name in names:
            raise ValueError("unsafe scratch filename")
        names.add(entry.name)
        if entry.is_symlink() or not entry.is_file():
            raise ValueError("scratch contains a symlink or non-file entry")
    if len(names) > len(sources) + 1:
        raise ValueError("scratch contains too many entries")
    marker = output / "extraction.json"
    if marker.name not in names:
        result["state"] = "incomplete"
        return result

    extraction = json.loads(read_regular(marker, MAX_JSON))
    if (not isinstance(extraction, dict) or extraction.get("complete") is not True or
            extraction.get("fidelity") != "byte-exact"):
        raise ValueError("invalid scratch completion marker")
    members = extraction.get("members")
    if not isinstance(members, list) or len(members) != len(sources):
        raise ValueError("scratch member count differs from candidate")
    seen_ids, seen_names = set(), set()
    verified_bytes = 0
    for member in members:
        if not isinstance(member, dict):
            raise ValueError("invalid scratch member entry")
        identity, name, digest = member.get("id"), member.get("name"), member.get("exportSHA256")
        if (not isinstance(identity, str) or identity not in sources or identity in seen_ids or
                not isinstance(name, str) or not SAFE_NAME.fullmatch(name) or
                name == "extraction.json" or name in seen_names or
                digest != sources[identity]):
            raise ValueError("scratch member identity, filename or receipt digest mismatch")
        seen_ids.add(identity)
        seen_names.add(name)
        if name not in names:
            raise ValueError("completed scratch is missing a recovered member")
        content = read_regular(output / name, MAX_IMAGE)
        if hashlib.sha256(content).hexdigest() != sources[identity]:
            raise ValueError("scratch member differs from original SHA-256 evidence")
        verified_bytes += len(content)
    if seen_ids != set(sources) or names != seen_names | {"extraction.json"}:
        raise ValueError("unrecognized or missing scratch files")
    result.update(state="complete_verified", complete=True,
                  scratchBytesVerified=verified_bytes, scratchFilesVerified=len(sources))
    return result


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--receipt", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args(argv)
    try:
        print(json.dumps(audit(Path(args.receipt), Path(args.output)), sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, json.JSONDecodeError) as error:
        print("scratch audit refused: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
