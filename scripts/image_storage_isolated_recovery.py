#!/usr/bin/env python3
"""Rehearse recovery of one media member plus DB and config into private scratch.

Reads a verified backup-set receipt. Never opens live database paths, installs
a restored file, changes media routing, or authorizes original-file eviction.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import sys
import zipfile

import image_storage_backup_set as backup_set
import image_storage_db_config_audit as db_audit
import media_library_backup as media_backup

MARKER = "isolated-recovery-proof.json"
MAX_MARKER = 16 * 1024
BLOCKERS = {
    "readOnlyAgainstLibrary": True,
    "isolatedScratchOnly": True,
    "coherentSnapshotVerified": False,
    "completeMediaLibraryRecovered": False,
    "databaseRestoreIntegrated": False,
    "configurationRestoreIntegrated": False,
    "nativeServingVerified": False,
    "transactionalRollbackVerified": False,
    "canActivate": False,
    "originalEvictionEnabled": False,
}


def private_output(output: Path, record: dict) -> None:
    if not output.is_absolute() or output != Path(os.path.normpath(output)):
        raise ValueError("scratch output must be absolute and normalized")
    parent = backup_set._canonical_dir(output.parent)
    if os.name != "nt" and parent.stat().st_mode & 0o077:
        raise ValueError("scratch parent must have private 0700 permissions")
    protected = [Path(record["database"]["path"]), Path(record["config"]["path"]),
                 Path(record["media"]["store"])]
    protected += [Path(v) for v in media_backup._load_manifest(
        Path(record["media"]["store"]), record["media"]["snapshot"])["roots"].values()]
    for item in protected:
        if item.exists() and item.resolve(strict=True) != item:
            raise ValueError("protected source includes symlinks")
        if backup_set._under(output, item) or backup_set._under(item, output):
            raise ValueError("scratch output overlaps a protected original or backup")
        if item.is_dir() and (backup_set._under(parent, item) or backup_set._under(item, parent)):
            raise ValueError("scratch parent overlaps a protected media/backup directory")


def source_media(record: dict, member: str) -> dict:
    if not isinstance(member, str) or ":" not in member:
        raise ValueError("choose one saved root:path media member")
    label, rel = member.split(":", 1)
    if not media_backup.LABEL.fullmatch(label):
        raise ValueError("invalid media root label")
    media_backup._relpath(rel)
    manifest = media_backup._load_manifest(Path(record["media"]["store"]), record["media"]["snapshot"])
    selection = [entry for entry in manifest["entries"] if entry["root"] == label and entry["path"] == rel]
    if len(selection) != 1:
        raise ValueError("chosen source does not exist exactly once in the saved snapshot")
    return selection[0]


def byte_copy(source: Path, destination: Path, expected: str, limit: int) -> int:
    if not destination.parent.is_dir() or destination.parent.is_symlink():
        raise ValueError("scratch destination parent is not a private directory")
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    copied = 0
    digest = hashlib.sha256()
    with os.fdopen(os.open(source, flags), "rb") as original, destination.open("xb") as output:
        os.chmod(destination, 0o600)
        while chunk := original.read(1024 * 1024):
            copied += len(chunk)
            if copied > limit:
                raise ValueError("scratch reconstruction exceeded size limit")
            output.write(chunk)
            digest.update(chunk)
        output.flush()
        os.fsync(output.fileno())
    if digest.hexdigest() != expected:
        raise ValueError("saved backup changed while writing private scratch")
    return copied


def _files(output: Path, relpath: str) -> Path:
    parts = media_backup._relpath(relpath).parts
    path = output
    for part in parts:
        path = path / part
        if path.is_symlink():
            raise ValueError("scratch path contains a symlink")
    if not path.is_relative_to(output):
        raise ValueError("scratch path escaped output")
    return path


def verify_files(output: Path, record: dict, media_entry: dict, meta: dict) -> None:
    recovered = output / "database" / "recovered.sqlite"
    config = output / "config" / "config.yml"
    media_path = _files(output / "media" / media_entry["root"], media_entry["path"])
    for path, expected, cap in (
        (recovered, meta["recoveredDatabaseSHA256"], db_audit.MAX_DB),
        (config, record["config"]["sha256"], db_audit.MAX_CONFIG),
        (media_path, media_entry["sha256"], db_audit.MAX_DB),
    ):
        if not path.is_file() or path.is_symlink():
            raise ValueError("missing or symlinked scratch recovery file")
        size, digest = db_audit.digest_regular(path, cap)
        if digest != expected or (path == media_path and size != media_entry["size"]):
            raise ValueError("reconstructed scratch file failed SHA-256/size comparison")
    if not db_audit.sqlite_integrity(recovered)["integrityCheck"]:
        raise ValueError("reconstructed SQLite integrity check failed")
    allowed = {recovered, config, media_path, output / MARKER}
    for path in output.rglob("*"):
        if path.is_symlink():
            raise ValueError("scratch recovery contains a symlink")
        if path.is_file() and path not in allowed:
            raise ValueError("scratch recovery contains an unexpected file")


def _frozen_report(output: Path, record: dict, media_entry: dict, database_hash: str) -> dict:
    return {
        "format": "stashbooru-isolated-storage-recovery-proof",
        "version": 1,
        "sourceDatabaseSHA256": record["database"]["sha256"],
        "sourceConfigSHA256": record["config"]["sha256"],
        "mediaSnapshot": record["media"]["snapshot"],
        "mediaMember": media_entry["root"] + ":" + media_entry["path"],
        "mediaMemberSHA256": media_entry["sha256"],
        "mediaMemberBytes": media_entry["size"],
        "recoveredDatabaseSHA256": database_hash,
        "recoveredFiles": 3,
        "outputPath": str(output),
        **BLOCKERS,
    }


def _check_report(record: dict, report: dict, output: Path, member: str) -> dict:
    if not isinstance(report, dict) or report.get("mediaMember") != member:
        raise ValueError("scratch completion proof does not match the chosen member")
    media_entry = source_media(record, member)
    expected = _frozen_report(output, record, media_entry, report.get("recoveredDatabaseSHA256"))
    if report != expected or not isinstance(report["recoveredDatabaseSHA256"], str) or not backup_set.DIGEST.fullmatch(
            report["recoveredDatabaseSHA256"]):
        raise ValueError("scratch proof is incomplete, incompatible, or has unsafe capability claims")
    verify_files(output, record, media_entry, report)
    return report


def rehearse(receipt: Path, output: Path, member: str) -> dict:
    backup_set.verify_receipt(receipt)  # Never trust an unverified saved identity.
    record = backup_set._load_record(receipt)
    private_output(output, record)
    media_entry = source_media(record, member)
    if output.exists() or output.is_symlink():
        raise ValueError("scratch directory already exists; never overwrite previous recovery")
    output.mkdir(mode=0o700)  # Failure leaves an incomplete, clearly unmarked scratch dir.
    (output / "media").mkdir(mode=0o700)
    (output / "database").mkdir(mode=0o700)
    (output / "config").mkdir(mode=0o700)
    saved_database = Path(record["database"]["path"])
    recovered = output / "database" / "recovered.sqlite"
    if record["database"]["kind"] == "sqlite-with-blobs-zip":
        db_audit.sqlite_from_zip(saved_database, recovered)
        os.chmod(recovered, 0o600)
    else:
        byte_copy(saved_database, recovered, record["database"]["sha256"], db_audit.MAX_DB)
    _, recovered_digest = db_audit.digest_regular(recovered, db_audit.MAX_DB)
    if not db_audit.sqlite_integrity(recovered)["integrityCheck"]:
        raise ValueError("private restored SQLite is corrupt")
    byte_copy(Path(record["config"]["path"]), output / "config" / "config.yml",
              record["config"]["sha256"], db_audit.MAX_CONFIG)
    copied = media_backup.restore(Path(record["media"]["store"]), record["media"]["snapshot"],
                                  output / "media", member)
    if copied.get("restored") != 1:
        raise ValueError("private media recovery did not restore exactly one member")
    report = _frozen_report(output, record, media_entry, recovered_digest)
    verify_files(output, record, media_entry, report)
    backup_set.verify_receipt(receipt)  # Fail if backups changed during scratch write.
    marker = output / MARKER
    payload = json.dumps(report, sort_keys=True, separators=(",", ":")).encode() + b"\n"
    if len(payload) > MAX_MARKER:
        raise ValueError("scratch completion report too large")
    fd = os.open(marker, os.O_CREAT | os.O_EXCL | os.O_WRONLY |
                 getattr(os, "O_NOFOLLOW", 0), 0o600)
    with os.fdopen(fd, "wb") as f:
        f.write(payload)
        f.flush()
        os.fsync(f.fileno())
    if os.name != "nt":
        directory_fd = os.open(output, os.O_RDONLY)
        try:
            os.fsync(directory_fd)
        finally:
            os.close(directory_fd)
    return _check_report(record, report, output, member)


def audit(receipt: Path, output: Path) -> dict:
    backup_set.verify_receipt(receipt)
    record = backup_set._load_record(receipt)
    private_output(output, record)
    if not output.exists():
        return {"status": "no_output", "verified": False, "canActivate": False,
                "originalEvictionEnabled": False}
    if output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output:
        raise ValueError("unsafe recovery output directory")
    marker = output / MARKER
    if not marker.exists():
        if marker.is_symlink():
            raise ValueError("symlinked incomplete recovery marker")
        return {"status": "incomplete", "verified": False, "canActivate": False,
                "originalEvictionEnabled": False}
    db_audit.regular(marker, MAX_MARKER)
    report = json.loads(marker.read_text(encoding="utf-8"))
    if not isinstance(report, dict):
        raise ValueError("invalid scratch completion JSON")
    member = report.get("mediaMember")
    proof = _check_report(record, report, output, member)
    backup_set.verify_receipt(receipt)
    return {"status": "complete_verified", "verified": True, **proof}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_subparsers(dest="action", required=True)
    run = group.add_parser("rehearse")
    run.add_argument("--receipt", required=True)
    run.add_argument("--output", required=True)
    run.add_argument("--member", required=True)
    check = group.add_parser("audit")
    check.add_argument("--receipt", required=True)
    check.add_argument("--output", required=True)
    args = parser.parse_args(argv)
    try:
        if args.action == "rehearse":
            result = rehearse(Path(args.receipt), Path(args.output), args.member)
        else:
            result = audit(Path(args.receipt), Path(args.output))
        print(json.dumps(result, sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, sqlite3.DatabaseError, zipfile.BadZipFile,
            media_backup.BackupError) as exc:
        print("isolated scratch recovery refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
