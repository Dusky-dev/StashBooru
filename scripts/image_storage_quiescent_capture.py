#!/usr/bin/env python3
"""Capture a conservative, operator-quiesced SQLite/config/media backup set.

This is a stability-observed capture protocol, NOT an application-enforced
write barrier or an atomic multi-resource snapshot. Never enables eviction.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import stat
import sys
from urllib.parse import quote

import image_storage_backup_set as backup_set
import image_storage_db_config_audit as db_audit
import image_storage_library_coverage as coverage
import media_library_backup as media_backup

FORMAT = "stashbooru-operator-quiesced-capture"
VERSION = 1
MARKER = "capture-report.json"
FILES = ("database-before.sqlite", "database.sqlite",
         "config-before.yml", "config.yml", "backup-set.receipt.json", MARKER)
REPORT_MAX = 32 * 1024
FAIL_FALSE = {
    "coherentSnapshotVerified": False,
    "applicationWriteBarrierVerified": False,
    "fullLibraryRecovered": False,
    "nativeServingVerified": False,
    "transactionalRollbackVerified": False,
    "canActivate": False,
    "originalEvictionEnabled": False,
}


def _source_file(path: Path, max_size: int) -> None:
    db_audit.regular(path, max_size)


def _safe_inputs(database: Path, config: Path, store: Path,
                 roots: dict[str, Path], output: Path) -> None:
    if not output.is_absolute() or output != Path(os.path.normpath(output)):
        raise ValueError("scratch output must be absolute and canonical")
    parent = backup_set._canonical_dir(output.parent)
    if os.name != "nt" and parent.stat().st_mode & 0o077:
        raise ValueError("scratch parent must be private 0700")
    if output.exists() or output.is_symlink():
        raise ValueError("new capture output must not already exist")
    if not roots:
        raise ValueError("one or more labelled image roots required")
    backup_set._canonical_dir(store)
    _source_file(database, db_audit.MAX_DB)
    _source_file(config, db_audit.MAX_CONFIG)
    if config.suffix.lower() not in (".yml", ".yaml"):
        raise ValueError("source config must be YAML")
    if database == config:
        raise ValueError("database and config cannot be the same file")
    dirs = [store]
    for label, root in roots.items():
        if not media_backup.LABEL.fullmatch(label):
            raise ValueError("invalid media root label")
        dirs.append(backup_set._canonical_dir(root))
    for i, first in enumerate(dirs):
        if (backup_set._under(output, first) or backup_set._under(first, output) or
                backup_set._under(parent, first) or backup_set._under(first, parent)):
            raise ValueError("private capture output overlaps media/backup storage")
        for second in dirs[i + 1:]:
            if backup_set._under(first, second) or backup_set._under(second, first):
                raise ValueError("backup store and media roots must not overlap")
    for src in (database, config):
        if (backup_set._under(output, src) or backup_set._under(src, output) or
                any(backup_set._under(src, protected) for protected in dirs)):
            raise ValueError("live database/config cannot overlap media or backup storage")


def _sqlite_copy(source: Path, destination: Path) -> str:
    # SQLite's online backup API includes WAL-backed committed transactions.
    # Opening the source read-only avoids checkpointing/migrating the live DB.
    _source_file(source, db_audit.MAX_DB)
    fd = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL |
                 getattr(os, "O_NOFOLLOW", 0), 0o600)
    os.close(fd)
    uri = "file:" + quote(str(source), safe="/") + "?mode=ro"
    with sqlite3.connect(uri, uri=True, timeout=10) as incoming:
        with sqlite3.connect(destination) as outgoing:
            incoming.backup(outgoing, pages=256, sleep=0.05)
    _source_file(destination, db_audit.MAX_DB)
    with destination.open("rb") as copied:
        os.fsync(copied.fileno())
    if db_audit.sqlite_integrity(destination)["integrityCheck"] is not True:
        raise ValueError("online SQLite copy failed integrity verification")
    return db_audit.digest_regular(destination, db_audit.MAX_DB)[1]


def _config_copy(source: Path, dest: Path) -> str:
    _, expected = db_audit.digest_regular(source, db_audit.MAX_CONFIG)
    import image_storage_isolated_recovery as isolated
    isolated.byte_copy(source, dest, expected, db_audit.MAX_CONFIG)
    return expected


def _verify_current_media(store: Path, snapshot: str, roots: dict[str, Path]) -> dict:
    manifest = media_backup._load_manifest(store, snapshot)
    if manifest["roots"] != {key: str(value) for key, value in sorted(roots.items())}:
        raise ValueError("snapshot roots differ from operator-provided originals")
    mapped = {(entry["root"], entry["path"]): entry for entry in manifest["entries"]}
    if not 1 <= len(mapped) <= coverage.MAX_SNAPSHOT_MEMBERS:
        raise ValueError("unexpected empty or oversized media snapshot")
    seen = set()
    for label, root in sorted(roots.items()):
        for path, relative in media_backup._source_files(root):
            key = (label, relative)
            if key not in mapped or key in seen:
                raise ValueError("media set changed while backing up")
            seen.add(key)
            entry = mapped[key]
            before = media_backup._regular(path)
            actual_size, actual_hash = media_backup._read_hash(path)
            after = media_backup._regular(path)
            if (actual_size != entry["size"] or actual_hash != entry["sha256"] or
                    after.st_size != entry["size"] or before.st_size != after.st_size or
                    before.st_mtime_ns != after.st_mtime_ns or
                    before.st_ctime_ns != after.st_ctime_ns or
                    after.st_mtime_ns != entry["mtime_ns"] or
                    stat.S_IMODE(after.st_mode) != entry["mode"]):
                raise ValueError("source media changed during backup stability check")
    if seen != set(mapped):
        raise ValueError("some saved media paths disappeared or moved")
    result = media_backup.verify(store, snapshot)
    if result.get("verified") is not True or result.get("files") != len(seen):
        raise ValueError("verified original-media backup is incomplete")
    return result


def _coverage(database: Path, store: Path, snapshot: str) -> dict:
    manifest = media_backup._load_manifest(store, snapshot)
    roots, members = coverage.media_index(manifest)
    check = coverage.audit_database(database, roots, members, max_examples=10)
    if check["allSavedPrimaryImagePathsCovered"] is not True:
        raise ValueError("saved SQLite image paths do not all resolve to backed-up media")
    return check


def _report(output: Path, snapshot: str, before_db: str, db: str, cfg: str,
            receipt_hash: str, inspected: dict) -> dict:
    return {
        "format": FORMAT, "version": VERSION,
        "readOnlyAgainstLiveDatabase": True,
        "originalMediaUnmodifiedByCapture": True,
        "operatorQuiescenceAsserted": True,
        "sourceStabilityObserved": True,
        "sqliteOnlineBackupUsed": True,
        "databaseSnapshotsByteIdentical": True,
        "savedPrimaryImagePathsCovered": True,
        "mediaSnapshot": snapshot,
        "databaseBeforeSHA256": before_db,
        "databaseSHA256": db,
        "configSHA256": cfg,
        "backupSetReceiptSHA256": receipt_hash,
        "savedImageCount": inspected["totalSavedImages"],
        "coveredImageCount": inspected["coveredImageCount"],
        "outputPath": str(output),
        **FAIL_FALSE,
    }


def _write_marker(output: Path, report: dict) -> None:
    payload = json.dumps(report, sort_keys=True, separators=(",", ":"), allow_nan=False).encode() + b"\n"
    if len(payload) > REPORT_MAX:
        raise ValueError("completion record exceeds bound")
    descriptor = os.open(output / MARKER, os.O_WRONLY | os.O_EXCL | os.O_CREAT |
                         getattr(os, "O_NOFOLLOW", 0), 0o600)
    with os.fdopen(descriptor, "wb") as file:
        file.write(payload)
        file.flush()
        os.fsync(file.fileno())
    if os.name != "nt":
        descriptor = os.open(output, os.O_RDONLY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)


def capture(database: Path, config: Path, store: Path, roots: dict[str, Path],
            output: Path, acknowledged: bool = False) -> dict:
    if acknowledged is not True:
        raise ValueError("operator must acknowledge the application is quiesced")
    database, config, store, output = map(Path, (database, config, store, output))
    roots = {key: Path(value) for key, value in roots.items()}
    _safe_inputs(database, config, store, roots, output)
    output.mkdir(mode=0o700)
    first_db = _sqlite_copy(database, output / "database-before.sqlite")
    first_cfg = _config_copy(config, output / "config-before.yml")
    manifest_info = media_backup.backup(store, roots)
    snapshot = manifest_info["snapshot"]
    last_db = _sqlite_copy(database, output / "database.sqlite")
    last_cfg = _config_copy(config, output / "config.yml")
    if first_db != last_db or first_cfg != last_cfg:
        raise ValueError("database/config changed during capture window")
    # A second complete scan checks for additions/deletions and byte changes.
    _verify_current_media(store, snapshot, roots)
    inspected = _coverage(output / "database.sqlite", store, snapshot)
    receipt = output / "backup-set.receipt.json"
    backup_set.create_receipt(output / "database.sqlite", output / "config.yml",
                              store, snapshot, receipt)
    _, receipt_hash = db_audit.digest_regular(receipt, backup_set.RECEIPT_MAX)
    # Fail closed on changes observable after the combined backup-set receipt.
    _verify_current_media(store, snapshot, roots)
    _source_file(database, db_audit.MAX_DB)
    if _config_copy_digest(config) != last_cfg:
        raise ValueError("live config changed after media backup verification")
    # DB capture after media proves neither a native write barrier nor atomicity.
    return _finish_capture(database, output, snapshot, first_db, last_db,
                           last_cfg, receipt_hash, inspected)


def _config_copy_digest(source: Path) -> str:
    return db_audit.digest_regular(source, db_audit.MAX_CONFIG)[1]


def _finish_capture(database: Path, output: Path, snapshot: str,
                    before: str, after: str, config: str, receipt: str,
                    inspected: dict) -> dict:
    # One final online SQLite observation prevents publishing a receipt if
    # the catalogue changed after its second saved snapshot was captured.
    final = output / "database-final-check.sqlite"
    final_hash = _sqlite_copy(database, final)
    if final_hash != after:
        raise ValueError("live SQLite changed after saved backup generation")
    final.unlink()
    report = _report(output, snapshot, before, after, config, receipt, inspected)
    audit(output, require_marker=False)  # Independently validate saved artifacts.
    _write_marker(output, report)
    return report


def _safe_output(output: Path) -> None:
    if not output.is_absolute() or output != Path(os.path.normpath(output)):
        raise ValueError("invalid private capture location")
    folder = backup_set._canonical_dir(output)
    if os.name != "nt" and (folder.stat().st_mode & 0o077 or
                             folder.parent.stat().st_mode & 0o077):
        raise ValueError("capture output must remain private")
    for name in FILES:
        path = output / name
        if path.is_symlink():
            raise ValueError("scratch contains an unsafe symlink")


def audit(output: Path, require_marker: bool = True) -> dict:
    output = Path(output)
    _safe_output(output)
    marker = output / MARKER
    if not marker.exists():
        if require_marker:
            return {"status": "incomplete", "verified": False, **FAIL_FALSE}
    else:
        if not require_marker:
            raise ValueError("new capture should not yet have a completion marker")
    saved_files = {name for name in FILES if name != MARKER}
    for path in output.iterdir():
        if path.name not in (saved_files | {MARKER}) or not path.is_file() or path.is_symlink():
            raise ValueError("unexpected file in private capture output")
    if {path.name for path in output.iterdir()} != (saved_files | ({MARKER} if require_marker else set())):
        raise ValueError("capture output has missing or unexpected contents")
    receipt = output / "backup-set.receipt.json"
    backup_set.verify_receipt(receipt)
    record = backup_set._load_record(receipt)
    if record["database"]["path"] != str(output / "database.sqlite") or record["config"]["path"] != str(output / "config.yml"):
        raise ValueError("backup set references a different database/config")
    db_before = db_audit.digest_regular(output / "database-before.sqlite", db_audit.MAX_DB)[1]
    db_after = db_audit.digest_regular(output / "database.sqlite", db_audit.MAX_DB)[1]
    config_before = db_audit.digest_regular(output / "config-before.yml", db_audit.MAX_CONFIG)[1]
    config_after = db_audit.digest_regular(output / "config.yml", db_audit.MAX_CONFIG)[1]
    if db_before != db_after or config_before != config_after:
        raise ValueError("saved before/after database/config disagree")
    inspected = _coverage(output / "database.sqlite", Path(record["media"]["store"]),
                          record["media"]["snapshot"])
    receipt_hash = db_audit.digest_regular(receipt, backup_set.RECEIPT_MAX)[1]
    expected = _report(output, record["media"]["snapshot"], db_before, db_after,
                       config_after, receipt_hash, inspected)
    if not require_marker:
        return {"status": "ready_for_marker", "verified": True, **expected}
    db_audit.regular(marker, REPORT_MAX)
    report = json.loads(marker.read_bytes())
    if not isinstance(report, dict) or report != expected:
        raise ValueError("capture completion record differs from independently verified backups")
    return {"status": "complete_verified", "verified": True, **expected}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="action", required=True)
    create = commands.add_parser("capture")
    create.add_argument("--database", type=Path, required=True)
    create.add_argument("--config", type=Path, required=True)
    create.add_argument("--store", type=Path, required=True)
    create.add_argument("--root", action="append", required=True, metavar="LABEL=DIR")
    create.add_argument("--output", type=Path, required=True)
    create.add_argument("--acknowledge-quiesced", action="store_true")
    check = commands.add_parser("audit")
    check.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        if args.action == "capture":
            roots = {}
            for entry in args.root:
                if "=" not in entry:
                    raise ValueError("roots must be LABEL=DIR")
                label, folder = entry.split("=", 1)
                if not folder or label in roots:
                    raise ValueError("empty or duplicate root")
                roots[label] = Path(folder)
            result = capture(args.database, args.config, args.store, roots, args.output,
                             acknowledged=args.acknowledge_quiesced)
        else:
            result = audit(args.output)
        print(json.dumps(result, sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, sqlite3.DatabaseError,
            media_backup.BackupError) as exc:
        print("quiescent capture refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
