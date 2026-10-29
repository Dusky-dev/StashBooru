#!/usr/bin/env python3
"""Create/verify a private receipt binding existing backup artifacts by SHA-256.

OFFLINE AND NON-DESTRUCTIVE. Does not capture coordinated snapshots, restore
databases, change native media serving, activate packing or delete originals.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import stat
import sys

import image_storage_db_config_audit as database_audit
import media_library_backup as media_backup

FORMAT = "stashbooru-storage-recovery-backup-set"
VERSION = 1
RECEIPT_MAX = 64 * 1024
DIGEST = re.compile(r"^[0-9a-f]{64}$")
BLOCKED = (
    "coherentSnapshotVerified",
    "configSemanticValidationVerified",
    "databaseRestoreVerified",
    "configRestoreVerified",
    "archiveBlobsIndividuallyVerified",
    "nativeMediaServingVerified",
    "transactionalRollbackVerified",
    "canActivate",
    "originalEvictionEnabled",
)


def _canonical_dir(path: Path) -> Path:
    if not path.is_absolute() or path.is_symlink() or not path.is_dir():
        raise ValueError("directory must be absolute, existing and non-symlink")
    if path != path.resolve(strict=True):
        raise ValueError("directory contains a symlink or noncanonical ancestor")
    return path


def _under(candidate: Path, parent: Path) -> bool:
    return candidate == parent or parent in candidate.parents


def _receipt_location(output: Path, store: Path, roots: list[Path]) -> None:
    if not output.is_absolute() or output.suffix.lower() != ".json":
        raise ValueError("receipt output must be an absolute .json path")
    parent = _canonical_dir(output.parent)
    if os.name != "nt" and parent.stat().st_mode & 0o077:
        raise ValueError("private receipt directory must have mode 0700")
    if output.exists() or output.is_symlink():
        raise ValueError("receipt must not replace an existing path")
    for protected in (store, *roots):
        if _under(parent, protected) or _under(protected, parent):
            raise ValueError("private receipt directory overlaps media or backup storage")


def _fingerprint_media(store: Path, snapshot: str) -> dict:
    if not media_backup.SNAPSHOT.fullmatch(snapshot):
        raise ValueError("invalid saved media snapshot ID")
    store = _canonical_dir(store)
    path = media_backup._snapshot_path(store, snapshot)
    before_bytes, before_hash = database_audit.digest_regular(path, RECEIPT_MAX)
    result = media_backup.verify(store, snapshot)
    if result.get("verified") is not True or result.get("snapshot") != snapshot:
        raise ValueError("original-media backup verification did not pass")
    after_bytes, after_hash = database_audit.digest_regular(path, RECEIPT_MAX)
    if (before_bytes, before_hash) != (after_bytes, after_hash):
        raise ValueError("media snapshot manifest changed during verification")
    return {
        "store": str(store),
        "snapshot": snapshot,
        "manifestBytes": before_bytes,
        "manifestSHA256": before_hash,
        "files": result["files"],
        "objects": result["objects"],
        "logicalBytes": result["logicalBytes"],
    }


def _build_record(database: Path, config: Path, store: Path, snapshot: str) -> dict:
    # Existing backup verifier checks complete SQLite integrity, byte hashes,
    # bounded ZIP metadata and separately saved configuration bytes.
    db_report = database_audit.inspect(database, config)
    if (db_report.get("sqliteIntegrityVerified") is not True or
            db_report.get("readOnlyBackupInspection") is not True or
            db_report.get("databaseConfigSnapshotPairVerified") is not False):
        raise ValueError("database/configuration inspection omitted required safety proof")
    media = _fingerprint_media(store, snapshot)
    if media["files"] < 1 or media["objects"] < 1:
        raise ValueError("media snapshot must contain at least one verified object")
    return {
        "format": FORMAT,
        "version": VERSION,
        "database": {
            "path": str(database),
            "kind": db_report["databaseKind"],
            "bytes": db_report["databaseBytes"],
            "sha256": db_report["databaseSHA256"],
            "sqliteIntegrityVerifiedAtInspection": True,
            "sqliteUserVersion": db_report["sqliteUserVersion"],
            "applicationTables": db_report["sqliteApplicationTables"],
        },
        "config": {
            "path": str(config),
            "bytes": db_report["configBytes"],
            "sha256": db_report["configSHA256"],
        },
        "media": media,
        "claims": {key: False for key in BLOCKED},
    }


def _load_record(path: Path) -> dict:
    database_audit.regular(path, RECEIPT_MAX)
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    with os.fdopen(os.open(path, flags), "rb") as source:
        raw = source.read(RECEIPT_MAX + 1)
        if len(raw) > RECEIPT_MAX:
            raise ValueError("private receipt too large")
    record = json.loads(raw)
    if not isinstance(record, dict) or set(record) != {"format", "version", "database", "config", "media", "claims"}:
        raise ValueError("invalid private backup-set receipt")
    if record["format"] != FORMAT or type(record["version"]) is not int or record["version"] != VERSION:
        raise ValueError("unsupported private receipt version")
    claims = record["claims"]
    if not isinstance(claims, dict) or set(claims) != set(BLOCKED) or any(claims[k] is not False for k in BLOCKED):
        raise ValueError("receipt incorrectly claims activation, rollback or coherent backup")
    checks = {
        "database": {"path", "kind", "bytes", "sha256", "sqliteIntegrityVerifiedAtInspection",
                     "sqliteUserVersion", "applicationTables"},
        "config": {"path", "bytes", "sha256"},
        "media": {"store", "snapshot", "manifestBytes", "manifestSHA256", "files",
                  "objects", "logicalBytes"},
    }
    for name, fields in checks.items():
        value = record[name]
        if not isinstance(value, dict) or set(value) != fields:
            raise ValueError("invalid backup-set member evidence: " + name)
        if not isinstance(value.get("sha256", value.get("manifestSHA256")), str) or not DIGEST.fullmatch(
                value.get("sha256", value.get("manifestSHA256"))):
            raise ValueError("invalid saved backup-set member digest")
    if record["database"]["sqliteIntegrityVerifiedAtInspection"] is not True:
        raise ValueError("receipt lacks SQLite integrity evidence")
    for p in (record["database"]["path"], record["config"]["path"], record["media"]["store"]):
        if not isinstance(p, str) or not Path(p).is_absolute():
            raise ValueError("receipt references an unsafe relative artifact")
    if not isinstance(record["media"]["snapshot"], str) or not media_backup.SNAPSHOT.fullmatch(record["media"]["snapshot"]):
        raise ValueError("invalid recorded snapshot identifier")
    for x in (record["database"]["bytes"], record["config"]["bytes"], record["media"]["manifestBytes"],
              record["media"]["files"], record["media"]["objects"], record["media"]["logicalBytes"]):
        if type(x) is not int or x < 1:
            raise ValueError("missing positive backup-set size/count evidence")
    return record


def verify_receipt(receipt: Path) -> dict:
    record = _load_record(receipt)
    db, cfg, media = record["database"], record["config"], record["media"]
    current = _build_record(Path(db["path"]), Path(cfg["path"]), Path(media["store"]), media["snapshot"])
    if current != record:
        raise ValueError("backup artifacts no longer match the saved backup-set receipt")
    return {
        "verified": True,
        "readOnly": True,
        "receipt": str(receipt),
        "databaseSHA256": db["sha256"],
        "configSHA256": cfg["sha256"],
        "mediaManifestSHA256": media["manifestSHA256"],
        "mediaObjectsVerified": media["objects"],
        "coherentSnapshotVerified": False,
        "databaseRestoreVerified": False,
        "configRestoreVerified": False,
        "nativeMediaServingVerified": False,
        "transactionalRollbackVerified": False,
        "canActivate": False,
        "originalEvictionEnabled": False,
    }


def create_receipt(database: Path, config: Path, store: Path, snapshot: str, output: Path) -> dict:
    database, config, store, output = map(Path, (database, config, store, output))
    # Verify the existing store and its declared historical roots before writing.
    _canonical_dir(store)
    manifest = media_backup._load_manifest(store, snapshot)
    roots = [_canonical_dir(Path(p)) for p in manifest["roots"].values()]
    _receipt_location(output, store, roots)
    record = _build_record(database, config, store, snapshot)
    data = json.dumps(record, sort_keys=True, separators=(",", ":"), allow_nan=False).encode() + b"\n"
    if len(data) > RECEIPT_MAX:
        raise ValueError("backup-set receipt exceeds size limit")
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
    fd = os.open(output, flags, 0o600)
    try:
        with os.fdopen(fd, "wb") as target:
            target.write(data)
            target.flush()
            os.fsync(target.fileno())
        if os.name != "nt":
            dir_fd = os.open(output.parent, os.O_RDONLY)
            try:
                os.fsync(dir_fd)
            finally:
                os.close(dir_fd)
    except BaseException:
        output.unlink(missing_ok=True)
        raise
    return verify_receipt(output)


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    create = sub.add_parser("create")
    create.add_argument("--database", required=True)
    create.add_argument("--config", required=True)
    create.add_argument("--store", required=True)
    create.add_argument("--snapshot", required=True)
    create.add_argument("--output", required=True)
    check = sub.add_parser("verify")
    check.add_argument("--receipt", required=True)
    args = parser.parse_args(argv)
    try:
        if args.action == "create":
            result = create_receipt(Path(args.database), Path(args.config), Path(args.store),
                                    args.snapshot, Path(args.output))
        else:
            result = verify_receipt(Path(args.receipt))
        print(json.dumps(result, sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, json.JSONDecodeError,
            media_backup.BackupError, Exception) as exc:
        print("backup-set receipt refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
