#!/usr/bin/env python3
"""Offline, opt-in P12/P13 archive publication proof; never touches originals.

This tool deliberately has NO media activation, eviction, database, or native
serving path. A candidate is not published until the persisted archive is
decoded and every byte-exact member matches the captured original SHA-256.
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
import tempfile

import gallery_block_storage as blocks
import image_delta_storage as delta


METHODS = ("delta", "shared-tiles")


def source_snapshot(group):
    members = group.get("members")
    if not isinstance(members, list) or not 2 <= len(members) <= 16:
        raise ValueError("select an image-only group of 2–16 members")
    signatures = {}
    for member in members:
        if not isinstance(member, dict) or not isinstance(member.get("id"), str):
            raise ValueError("invalid member identity")
        identity, path = member["id"], member.get("path")
        if identity in signatures or not isinstance(path, str) or not Path(path).is_absolute():
            raise ValueError("duplicate member ID or non-absolute source path")
        before = os.lstat(path)
        if not stat.S_ISREG(before.st_mode):
            raise ValueError("source is not a regular, non-symlink file")
        data = delta.read_regular(path, delta.MAX_FILE)
        after = os.lstat(path)
        stamp = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns)
        if stamp(before) != stamp(after):
            raise ValueError("source changed during fingerprinting")
        signatures[identity] = {
            "path": str(Path(path).resolve(strict=True)),
            "sha256": hashlib.sha256(data).hexdigest(),
            "size": len(data),
            "stamp": stamp(after),
        }
    return signatures


def require_unchanged(group, expected):
    current = source_snapshot(group)
    if current != expected:
        raise ValueError("source images changed; refuse archive publication")


def validate_destination(output, source_dirs=()):
    output = Path(output)
    if not output.is_absolute() or output.name in ("", ".", ".."):
        raise ValueError("archive output must be an absolute new file path")
    if output.exists() or output.is_symlink():
        raise ValueError("refusing to replace an existing archive or symlink")
    parent = output.parent
    if not parent.is_dir() or parent.is_symlink():
        raise ValueError("archive parent must be a real existing directory")
    destination_dir = parent.resolve(strict=True)
    if any(destination_dir == Path(p).parent.resolve(strict=True) for p in source_dirs):
        raise ValueError("archive output must be in a separate directory from source images")
    return output


def open_reader(method, archive):
    if method == "delta":
        return delta.PackReader(archive)
    if method == "shared-tiles":
        return blocks.GalleryReader(archive)
    raise ValueError("invalid P12/P13 method")


def verify_candidate(method, archive, expected=None):
    with open_reader(method, archive) as reader:
        result = reader.verify()
        if not result["verified"] or result["fidelity"] != "byte-exact":
            raise ValueError("candidate is not verified byte-exact")
        identities = set(reader.entries)
        if expected is not None and identities != set(expected):
            raise ValueError("candidate member IDs do not match the selected sources")
        digests = {}
        for identity in identities:
            reconstructed = reader.reconstruct(identity)
            digest = hashlib.sha256(reconstructed).hexdigest()
            if expected is not None and (
                len(reconstructed) != expected[identity]["size"]
                or digest != expected[identity]["sha256"]
            ):
                raise ValueError("reconstructed original bytes do not match SHA-256")
            digests[identity] = digest
    return digests


def plan(group, method, min_bytes=4096, min_percent=5.0):
    if method == "delta":
        manifest, blobs, report = delta.plan_group(
            group, "byte-exact", min_bytes, min_percent
        )
        return (lambda output: delta.write_pack(manifest, blobs, output)), report
    if method == "shared-tiles":
        index, members, blobs, report = blocks.plan_gallery(
            group, "byte-exact", min_bytes, min_percent
        )
        return (lambda output: blocks.write_pack(index, members, blobs, output)), report
    raise ValueError("invalid P12/P13 method")


def pack_proof(group, method, output, min_bytes=4096, min_percent=5.0,
               checkpoint=lambda phase, path: None):
    """A published pack is always independently verified against source bytes.

    If interrupted after staging, the staged candidate remains for explicit
    offline 'resume'. A killed process cannot change or evict originals.
    """
    if method not in METHODS:
        raise ValueError("invalid P12/P13 method")
    if group.get("representative") not in {
        member.get("id") for member in group.get("members", [])
    }:
        raise ValueError("representative must be a member")
    expected = source_snapshot(group)
    output = validate_destination(output, [item["path"] for item in expected.values()])
    writer, estimate = plan(group, method, min_bytes, min_percent)
    require_unchanged(group, expected)
    stage_dir = Path(tempfile.mkdtemp(prefix=".stash-pack-proof-", dir=output.parent))
    candidate = stage_dir / "candidate.pack"
    published = False
    try:
        written = writer(candidate)
        checkpoint("staged", candidate)
        digests = verify_candidate(method, candidate, expected)
        require_unchanged(group, expected)
        checkpoint("verified", candidate)
        require_unchanged(group, expected)
        # This publishes only a new archive, atomically and without overwrite.
        delta.publish(candidate, output)
        published = True
        return {
            "experimental": True,
            "sourcesRetained": True,
            "nativeServing": False,
            "backupVerified": False,
            "originalEvictionEnabled": False,
            "fidelity": "byte-exact",
            "method": method,
            "verified": bool(written["verified"]),
            "sourceFiles": len(expected),
            "sourceBytes": sum(item["size"] for item in expected.values()),
            "archiveBytes": output.stat().st_size,
            "potentialSavedBytes": max(
                0, sum(item["size"] for item in expected.values()) - output.stat().st_size
            ),
            "retainedOriginalsPlusArchiveBytes": (
                sum(item["size"] for item in expected.values()) + output.stat().st_size
            ),
            "sourceSHA256": digests,
            "plan": {"layout": estimate["layout"], "eligible": (
                estimate.get("eligibleDelta") if method == "delta"
                else estimate.get("sharingSelected")
            )},
        }
    finally:
        # Preserve an interrupted, verified candidate for explicit recovery;
        # an incomplete staging directory never replaces a live file.
        if published or not candidate.exists():
            shutil.rmtree(stage_dir)


def resume_pack(method, staged, output):
    """Recover a persistent candidate from an interrupted run without originals."""
    staged = Path(staged)
    if not staged.is_absolute() or staged.is_symlink():
        raise ValueError("staged candidate must be an absolute non-symlink file")
    if not staged.is_file():
        raise ValueError("staged candidate is not a regular file")
    output = validate_destination(output)
    digests = verify_candidate(method, staged)
    delta.publish(staged, output)
    return {
        "experimental": True,
        "recovered": True,
        "verified": True,
        "sourcesRetained": True,
        "originalEvictionEnabled": False,
        "fidelity": "byte-exact",
        "method": method,
        "archiveBytes": output.stat().st_size,
        "memberSHA256": digests,
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    pack = sub.add_parser("pack", help="pack one explicit image group to a separate destination")
    pack.add_argument("--stacks", required=True)
    pack.add_argument("--group", required=True)
    pack.add_argument("--method", required=True, choices=METHODS)
    pack.add_argument("--output", required=True)
    pack.add_argument("--min-bytes", type=int, default=4096)
    pack.add_argument("--min-percent", type=float, default=5.0)
    recovery = sub.add_parser("resume", help="verify and publish an interrupted staged candidate")
    recovery.add_argument("--staged", required=True)
    recovery.add_argument("--method", required=True, choices=METHODS)
    recovery.add_argument("--output", required=True)
    args = parser.parse_args(argv)
    try:
        delta.require_dependencies()
        if args.command == "pack":
            groups = delta.read_stacks(args.stacks)
            selected = next((g for g in groups if g["id"] == args.group), None)
            if selected is None:
                raise ValueError("unknown Visual Stack")
            result = pack_proof(selected, args.method, args.output,
                                args.min_bytes, args.min_percent)
        else:
            result = resume_pack(args.method, args.staged, args.output)
        print(json.dumps(result, sort_keys=True, allow_nan=False))
        return 0
    except KeyboardInterrupt:
        print("Interrupted: original media are untouched; inspect staging for recovery.", file=sys.stderr)
        return 130
    except (ValueError, OSError, KeyError, TypeError) as error:
        print(f"non-destructive pack proof: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
