#!/usr/bin/env python3
"""Read-only proof that selected image sources match a verified media snapshot.

This is a prerequisite *check*, not permission to replace source files.
A passing snapshot does not cover StashBooru's DB/configuration, native media
consumers, subsequent source edits, or interrupted live image operations.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys

import image_delta_storage as delta
import media_library_backup as backup

MAX_IMAGES = 16


def _stamp(info):
    return (info.st_dev, info.st_ino, info.st_size,
            info.st_mtime_ns, info.st_ctime_ns)


def _roots(manifest):
    roots = {}
    for label, location in manifest["roots"].items():
        path = Path(location)
        if not path.is_absolute() or not path.is_dir() or path.is_symlink():
            raise backup.BackupError("snapshot contains a missing or unsafe media root")
        if path.resolve(strict=True) != path:
            raise backup.BackupError("snapshot media root has a symlink or noncanonical path")
        roots[label] = path
    if not roots:
        raise backup.BackupError("snapshot has no media roots")
    return roots


def _source_key(path, roots):
    source = Path(path)
    if not source.is_absolute() or source.is_symlink():
        raise backup.BackupError("selected image source must be an absolute regular non-symlink path")
    # Reject symlinked ancestor directories, even when they point into the
    # original root. Otherwise the manifest path is ambiguous.
    real = source.resolve(strict=True)
    if source != real:
        raise backup.BackupError("selected image source uses a symlink or noncanonical path")
    backup._regular(source)
    matches = []
    for label, root in roots.items():
        if source.is_relative_to(root):
            relative = source.relative_to(root).as_posix()
            backup._relpath(relative)
            matches.append((label, relative))
    if len(matches) != 1:
        raise backup.BackupError("selected source is not covered by exactly one snapshot root")
    return matches[0], source


def preflight(group: dict, store: Path, snapshot: str) -> dict:
    """Verify immutable backup objects and their current source counterparts."""
    if not isinstance(group, dict):
        raise backup.BackupError("invalid image group")
    members = group.get("members")
    if not isinstance(members, list) or not 2 <= len(members) <= MAX_IMAGES:
        raise backup.BackupError("select a group of 2–16 image files")
    ids = [item.get("id") for item in members if isinstance(item, dict)]
    if len(ids) != len(members) or any(
        not isinstance(identity, str) or not identity for identity in ids
    ) or len(ids) != len(set(ids)):
        raise backup.BackupError("image member IDs must be distinct nonempty strings")
    if group.get("representative") not in ids:
        raise backup.BackupError("representative must be a selected member")
    store = Path(store)
    if not store.is_absolute() or store.is_symlink() or store.resolve() != store:
        raise backup.BackupError("backup store must be an absolute non-symlink directory")
    # This checks every stored snapshot object, not just the selected files.
    verified = backup.verify(store, snapshot)
    if not verified.get("verified") or verified.get("snapshot") != snapshot:
        raise backup.BackupError("selected media snapshot was not verified")
    manifest = backup._load_manifest(store, snapshot)
    roots = _roots(manifest)
    entries = {(entry["root"], entry["path"]): entry for entry in manifest["entries"]}
    digests = {}
    selected = set()
    for item in members:
        path = item.get("path")
        if not isinstance(path, str):
            raise backup.BackupError("selected image source path is missing")
        key, source = _source_key(path, roots)
        if key in selected:
            raise backup.BackupError("the same source was selected more than once")
        selected.add(key)
        entry = entries.get(key)
        if entry is None:
            raise backup.BackupError("selected image was not included in the snapshot")
        before = backup._regular(source)
        size, digest = backup._read_hash(source)
        after = backup._regular(source)
        if _stamp(before) != _stamp(after):
            raise backup.BackupError("source image changed while checking backup coverage")
        if size != entry["size"] or digest != entry["sha256"]:
            raise backup.BackupError("selected image differs from its verified backup")
        digests[item["id"]] = digest
    return {
        "experimental": True,
        "readOnly": True,
        "verified": True,
        "mediaBackupVerified": True,
        "snapshot": snapshot,
        "group": group["id"],
        "selectedImagesCovered": len(digests),
        "snapshotFilesVerified": verified["files"],
        "snapshotObjectsVerified": verified["objects"],
        "imageSHA256": digests,
        "sourcesRetained": True,
        "nativePackingEnabled": False,
        "nativeServingVerified": False,
        "databaseBackupVerified": False,
        "originalEvictionEnabled": False,
        "disclaimer": (
            "Point-in-time media-only backup check. No packing, catalogue activation, "
            "database backup, live serving, rollback or original deletion is authorized."
        ),
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stacks", required=True, help="explicit stacks JSON (offline operator only)")
    parser.add_argument("--group", required=True)
    parser.add_argument("--store", required=True, help="existing original-media backup store")
    parser.add_argument("--snapshot", required=True)
    args = parser.parse_args(argv)
    try:
        groups = delta.read_stacks(args.stacks)
        group = next((item for item in groups if item["id"] == args.group), None)
        if group is None:
            raise backup.BackupError("unknown image group")
        report = preflight(group, Path(args.store), args.snapshot)
        print(json.dumps(report, sort_keys=True, allow_nan=False))
        return 0
    except (backup.BackupError, ValueError, OSError, KeyError, TypeError) as error:
        print("image backup preflight: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
