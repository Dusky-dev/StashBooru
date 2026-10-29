#!/usr/bin/env python3
"""Restore ALL members of one verified saved media snapshot into private scratch.

This is a disposable offline recovery rehearsal. It cannot reactivate a
StashBooru instance, rebase catalogue paths, overwrite originals, or evict.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import shutil
import sqlite3
import stat
import sys
import zipfile

import image_storage_backup_set as backup_set
import image_storage_db_config_audit as db_audit
import image_storage_isolated_recovery as isolated
import image_storage_library_coverage as coverage
import media_library_backup as media_backup

MARKER = "full-snapshot-recovery.json"
VERSION = 1
MAX_MARKER = 32 * 1024
DEFAULT_MAX_FILES = 128
DEFAULT_MAX_BYTES = 512 * 1024 * 1024
MAX_FILES = 1_000_000
MAX_BYTES = 2 * 1024 * 1024 * 1024 * 1024
BLOCKED = {
    "coherentSnapshotVerified": False,
    "fullLibraryRecovered": False,
    "restoredApplicationBootVerified": False,
    "recoveredCataloguePathRebasingVerified": False,
    "ZIPDatabaseBlobsRestored": False,
    "nativeServingVerified": False,
    "transactionalRollbackVerified": False,
    "canActivate": False,
    "originalEvictionEnabled": False,
}


def _bounded_manifest(record: dict, max_files: int, max_bytes: int) -> tuple[dict, int]:
    if (type(max_files) is not int or not 1 <= max_files <= MAX_FILES or
            type(max_bytes) is not int or not 1 <= max_bytes <= MAX_BYTES):
        raise ValueError("invalid per-rehearsal file or byte limit")
    manifest = media_backup._load_manifest(Path(record["media"]["store"]), record["media"]["snapshot"])
    entries = manifest["entries"]
    if not 1 <= len(entries) <= max_files:
        raise ValueError("media snapshot exceeds selected file bound")
    total = 0
    for entry in entries:
        total += entry["size"]
        if total > max_bytes:
            raise ValueError("media snapshot exceeds selected byte bound")
    if total != record["media"]["logicalBytes"] or len(entries) != record["media"]["files"]:
        raise ValueError("media count or size differs from saved backup-set receipt")
    return manifest, total


def _preflight(receipt: Path, max_files: int, max_bytes: int) -> tuple[dict, dict, dict, int, str]:
    backup_set.verify_receipt(receipt)
    record = backup_set._load_record(receipt)
    manifest, total = _bounded_manifest(record, max_files, max_bytes)
    covered = coverage.inspect(receipt)
    if not covered["allSavedPrimaryImagePathsCovered"] or covered["savedMediaSnapshot"] != record["media"]["snapshot"]:
        raise ValueError("saved image catalogue has incomplete historical media coverage")
    _, receipt_sha = db_audit.digest_regular(receipt, backup_set.RECEIPT_MAX)
    return record, manifest, covered, total, receipt_sha


def _copy_database(record: dict, output: Path) -> str:
    source = Path(record["database"]["path"])
    dest = output / "database" / "recovered.sqlite"
    if record["database"]["kind"] == "sqlite-with-blobs-zip":
        db_audit.sqlite_from_zip(source, dest)
        os.chmod(dest, 0o600)
    else:
        isolated.byte_copy(source, dest, record["database"]["sha256"], db_audit.MAX_DB)
    if db_audit.sqlite_integrity(dest)["integrityCheck"] is not True:
        raise ValueError("scratch SQLite recovery failed integrity check")
    return db_audit.digest_regular(dest, db_audit.MAX_DB)[1]


def _files(output: Path, manifest: dict) -> dict[Path, tuple[str, int]]:
    files = {}
    for entry in manifest["entries"]:
        destination = isolated._files(output / "media" / entry["root"], entry["path"])
        if destination in files:
            raise ValueError("duplicate recovery file destination")
        files[destination] = entry["sha256"], entry["size"]
    return files


def _check_scratch(output: Path, manifest: dict, database_digest: str,
                   config_digest: str, marked: bool) -> None:
    media_files = _files(output, manifest)
    expected = {
        output / "database" / "recovered.sqlite": (database_digest, None),
        output / "config" / "config.yml": (config_digest, None),
        **media_files,
    }
    expected_names = set(expected)
    if marked:
        expected_names.add(output / MARKER)
    allowed_folders = {output, output / "database", output / "config", output / "media"}
    for location in expected:
        allowed_folders.update(path for path in location.parents if path.is_relative_to(output))
    actual_names = set()
    for location in output.rglob("*"):
        details = location.lstat()
        if stat.S_ISDIR(details.st_mode):
            if location not in allowed_folders:
                raise ValueError("unexpected scratch subdirectory")
        elif stat.S_ISREG(details.st_mode):
            if location not in expected_names:
                raise ValueError("unexpected scratch file")
            actual_names.add(location)
        else:
            raise ValueError("symlink or unexpected scratch file type")
    if actual_names != expected_names:
        raise ValueError("scratch recovery is missing expected media/database/config files")
    for path, (digest, size) in expected.items():
        if path in media_files:
            count, actual = media_backup._read_hash(path)
        else:
            limit = db_audit.MAX_DB if path.name == "recovered.sqlite" else db_audit.MAX_CONFIG
            count, actual = db_audit.digest_regular(path, limit)
        if actual != digest or (size is not None and count != size):
            raise ValueError("recovered media or database failed SHA-256/size verification")
    if db_audit.sqlite_integrity(output / "database" / "recovered.sqlite")["integrityCheck"] is not True:
        raise ValueError("recovered SQLite failed independent integrity verification")


def _report(output: Path, record: dict, covered: dict, total: int, receipt_hash: str,
            restored_hash: str) -> dict:
    return {
        "format": "stashbooru-disposable-snapshot-recovery",
        "version": VERSION,
        "isolatedScratchOnly": True,
        "liveOriginalsNotRead": True,
        "savedBackupSetVerified": True,
        "allSavedMediaSnapshotMembersRestored": True,
        "allSavedPrimaryImagePathsCovered": True,
        "mediaSnapshot": record["media"]["snapshot"],
        "mediaFilesRestored": record["media"]["files"],
        "mediaLogicalBytesRestored": total,
        "savedImageCount": covered["totalSavedImages"],
        "restoredFileCount": record["media"]["files"] + 2,
        "databaseSHA256": restored_hash,
        "configSHA256": record["config"]["sha256"],
        "backupSetReceiptSHA256": receipt_hash,
        "outputPath": str(output),
        **BLOCKED,
    }


def _private_output(output: Path, record: dict) -> None:
    isolated.private_output(output, record)
    if output.exists() or output.is_symlink():
        raise ValueError("scratch output must be new; recovery cannot overwrite")


def _write_marker(output: Path, value: dict) -> None:
    payload = json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode() + b"\n"
    if len(payload) > MAX_MARKER:
        raise ValueError("scratch marker exceeds safe size limit")
    fd = os.open(output / MARKER, os.O_CREAT | os.O_EXCL | os.O_WRONLY |
                 getattr(os, "O_NOFOLLOW", 0), 0o600)
    with os.fdopen(fd, "wb") as target:
        target.write(payload)
        target.flush()
        os.fsync(target.fileno())
    if os.name != "nt":
        folder_fd = os.open(output, os.O_RDONLY)
        try:
            os.fsync(folder_fd)
        finally:
            os.close(folder_fd)


def rehearse(receipt: Path, output: Path, max_files: int = DEFAULT_MAX_FILES,
             max_bytes: int = DEFAULT_MAX_BYTES) -> dict:
    receipt, output = Path(receipt), Path(output)
    record, manifest, covered, total, receipt_hash = _preflight(receipt, max_files, max_bytes)
    _private_output(output, record)
    # Fail before materialization if this filesystem cannot accommodate
    # the snapshot plus SQLite and private safety headroom.
    required = total + db_audit.MAX_DB + db_audit.MAX_CONFIG + 32 * 1024 * 1024
    if shutil.disk_usage(output.parent).free < required:
        raise ValueError("scratch filesystem lacks the required recovery headroom")
    output.mkdir(mode=0o700)
    for name in ("database", "config", "media"):
        (output / name).mkdir(mode=0o700)
    recovered_sha = _copy_database(record, output)
    isolated.byte_copy(Path(record["config"]["path"]), output / "config" / "config.yml",
                       record["config"]["sha256"], db_audit.MAX_CONFIG)
    restored = media_backup.restore(Path(record["media"]["store"]), record["media"]["snapshot"],
                                    output / "media", None)
    if restored.get("restored") != record["media"]["files"]:
        raise ValueError("media recovery did not include every saved snapshot file")
    _check_scratch(output, manifest, recovered_sha, record["config"]["sha256"], False)
    second_record, second_manifest, second_coverage, second_total, second_sha = _preflight(
        receipt, max_files, max_bytes
    )
    if (second_record != record or second_manifest != manifest or
            second_coverage != covered or second_total != total or second_sha != receipt_hash):
        raise ValueError("saved backup evidence changed during disposable restoration")
    result = _report(output, record, covered, total, receipt_hash, recovered_sha)
    _write_marker(output, result)
    return result


def audit(receipt: Path, output: Path, max_files: int = MAX_FILES,
          max_bytes: int = MAX_BYTES) -> dict:
    receipt, output = Path(receipt), Path(output)
    record, manifest, covered, total, receipt_sha = _preflight(receipt, max_files, max_bytes)
    isolated.private_output(output, record)
    if not output.exists():
        return {"status": "no_output", "verified": False, **BLOCKED}
    if output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output:
        raise ValueError("scratch output has unsafe path")
    marker = output / MARKER
    if not marker.exists():
        if marker.is_symlink():
            raise ValueError("unsafe scratch marker")
        return {"status": "incomplete", "verified": False, **BLOCKED}
    db_audit.regular(marker, MAX_MARKER)
    saved = json.loads(marker.read_bytes())
    if (not isinstance(saved, dict) or not isinstance(saved.get("databaseSHA256"), str) or
            not backup_set.DIGEST.fullmatch(saved["databaseSHA256"])):
        raise ValueError("invalid scratch completion marker")
    expected = _report(output, record, covered, total, receipt_sha, saved["databaseSHA256"])
    if saved != expected:
        raise ValueError("scratch marker differs from independently verified saved backup")
    _check_scratch(output, manifest, saved["databaseSHA256"], record["config"]["sha256"], True)
    current, same_manifest, same_coverage, same_total, same_sha = _preflight(receipt, max_files, max_bytes)
    if (current != record or same_manifest != manifest or same_coverage != covered or
            same_total != total or same_sha != receipt_sha):
        raise ValueError("backup changed during scratch audit")
    return {"status": "complete_verified", "verified": True, **expected}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("rehearse", "audit"):
        cmd = commands.add_parser(name)
        cmd.add_argument("--backup-set-receipt", required=True, type=Path)
        cmd.add_argument("--output", required=True, type=Path)
        cmd.add_argument("--max-files", type=int, default=DEFAULT_MAX_FILES if name == "rehearse" else MAX_FILES)
        cmd.add_argument("--max-bytes", type=int, default=DEFAULT_MAX_BYTES if name == "rehearse" else MAX_BYTES)
    args = parser.parse_args(argv)
    try:
        if args.command == "rehearse":
            result = rehearse(args.backup_set_receipt, args.output, args.max_files, args.max_bytes)
        else:
            result = audit(args.backup_set_receipt, args.output, args.max_files, args.max_bytes)
        print(json.dumps(result, sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, json.JSONDecodeError,
            sqlite3.DatabaseError, zipfile.BadZipFile, media_backup.BackupError) as exc:
        print("disposable complete-media restore refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
