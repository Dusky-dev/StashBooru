#!/usr/bin/env python3
"""Rebase every saved regular-file pathname in ANOTHER disposable SQLite copy.

Strict offline rehearsal. Requires verified whole-snapshot restoration and
independently verified image-only rebased SQLite. Refuses ZIP-backed files,
orphaned folders, ambiguous roots, missing media, and path inconsistencies.
Does not start StashBooru, read original media, or modify live databases.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sqlite3
import stat
import sys
from urllib.parse import quote

import image_storage_backup_set as backup_set
import image_storage_db_config_audit as db_audit
import image_storage_disposable_path_rebase as image_rebase
import image_storage_disposable_snapshot_restore as restored
import image_storage_historical_paths as historical
import image_storage_isolated_recovery as isolated
import media_library_backup as media_backup

MARKER = "scratch-all-file-path-rebase.json"
MAX_MARKER = 32 * 1024
MAX_FILES = 250_000
FLAGS = {
    "coherentSnapshotVerified": False,
    "fullLibraryRecovered": False,
    "configurationRebasedVerified": False,
    "restoredServerBootVerified": False,
    "nativeMediaConsumersVerified": False,
    "transactionalRollbackVerified": False,
    "canActivate": False,
    "originalEvictionEnabled": False,
}


def _readonly(path: Path) -> sqlite3.Connection:
    uri = "file:" + quote(str(path), safe="/") + "?mode=ro&immutable=1"
    connection = sqlite3.connect(uri, uri=True, timeout=10)
    connection.execute("PRAGMA query_only=ON")
    return connection


def _mapped_folders(original_db: Path, image_db: Path, manifest: dict,
                    restored_dir: Path) -> tuple[dict[int, str], int, int]:
    roots = historical._roots(manifest["roots"])
    index = {}
    for entry in manifest["entries"]:
        key = (entry["root"], entry["path"])
        if key in index:
            raise ValueError("duplicate media snapshot path")
        index[key] = entry
    folders = {}
    file_count, matched_entries = 0, set()
    with _readonly(original_db) as source, _readonly(image_db) as images:
        old = source.execute("SELECT id, path, zip_file_id FROM folders ORDER BY id").fetchall()
        current = images.execute("SELECT id, path, zip_file_id FROM folders ORDER BY id").fetchall()
        if not old or len(old) != len(current):
            raise ValueError("source and image-rebased folder schema disagree")
        original_folders = {row[0]: row for row in old}
        image_folders = {row[0]: row for row in current}
        if len(original_folders) != len(old) or set(original_folders) != set(image_folders):
            raise ValueError("saved SQLite folders are missing or duplicated")
        rows = source.execute(
            "SELECT id, parent_folder_id, basename, size, zip_file_id "
            "FROM files ORDER BY id LIMIT ?", (MAX_FILES + 1,)
        ).fetchall()
        if not 1 <= len(rows) <= MAX_FILES:
            raise ValueError("saved file catalogue outside bounded 1..250000 files")
        if rows != images.execute(
            "SELECT id, parent_folder_id, basename, size, zip_file_id "
            "FROM files ORDER BY id LIMIT ?", (MAX_FILES + 1,)
        ).fetchall():
            raise ValueError("image path rebasing modified file catalogue records")
        file_ids = set()
        for file_id, folder_id, name, size, zip_id in rows:
            if (type(file_id) is not int or file_id <= 0 or file_id in file_ids or
                    type(folder_id) is not int or folder_id not in original_folders or
                    type(size) is not int or size < 1 or zip_id is not None):
                raise ValueError("invalid, missing or ZIP-backed saved file")
            file_ids.add(file_id)
            _, old_dir, folder_zip = original_folders[folder_id]
            if folder_zip is not None:
                raise ValueError("ZIP-backed saved folder cannot be rebased safely")
            historic_file = historical._historical_path(old_dir, name)
            matches = []
            for label, root in roots.items():
                try:
                    relative = historic_file.relative_to(root).as_posix()
                except ValueError:
                    continue
                media_backup._relpath(relative)
                entry = index.get((label, relative))
                if entry is not None and entry["size"] == size:
                    matches.append((label, relative, entry))
            if len(matches) != 1:
                raise ValueError("saved file has missing or ambiguous backup path/size")
            label, relative, _entry = matches[0]
            if (label, relative) in matched_entries:
                raise ValueError("multiple saved file IDs share the same backing path")
            matched_entries.add((label, relative))
            target = restored_dir / "media" / label / relative
            details = target.lstat()
            if not stat.S_ISREG(details.st_mode) or details.st_size != size or target.resolve(strict=True) != target:
                raise ValueError("restored regular file missing or unsafe")
            new_folder = str(target.parent)
            if folder_id in folders and folders[folder_id] != new_folder:
                raise ValueError("one saved folder maps to inconsistent scratch paths")
            folders[folder_id] = new_folder
            # #286 must already have corrected primary image folders.
            image_path = image_folders[folder_id][1]
            if image_path not in (old_dir, new_folder):
                raise ValueError("image-only path rebase points outside expected scratch")
        # Orphaned folder entries could be used by other native code. Refuse
        # them rather than leaving any old production directory in the clone.
        if len(folders) != len(original_folders):
            raise ValueError("orphaned saved folder cannot be safely rebased")
        if not matched_entries:
            raise ValueError("no saved regular files mapped to recovered media")
        return folders, len(rows), len(matched_entries)


def _report(receipt: Path, recovered: Path, image_output: Path, output: Path,
            source_digest: str, image_digest: str, clone_digest: str,
            folders: int, files: int, media_snapshot: str) -> dict:
    return {
        "format": "stashbooru-disposable-all-file-path-rebase",
        "version": 1,
        "readOnlyAgainstOriginals": True,
        "disposableSQLiteCopyOnly": True,
        "allSavedRegularFileFoldersRebased": True,
        "savedFileRecordsVerified": files,
        "savedFolderRecordsRebased": folders,
        "mediaSnapshot": media_snapshot,
        "backupSetReceiptSHA256": db_audit.digest_regular(receipt, backup_set.RECEIPT_MAX)[1],
        "sourceRecoveredSQLiteSHA256": source_digest,
        "inputImageRebasedSQLiteSHA256": image_digest,
        "outputSQLiteSHA256": clone_digest,
        "recoveredMediaPath": str(recovered),
        "sourceImageRebasePath": str(image_output),
        "outputPath": str(output),
        **FLAGS,
    }


def _validate_inputs(receipt: Path, recovered: Path, image_output: Path):
    recovery = restored.audit(receipt, recovered)
    if recovery.get("status") != "complete_verified" or recovery.get("verified") is not True:
        raise ValueError("full media recovery not independently verified")
    previous = image_rebase.audit(receipt, recovered, image_output)
    if previous.get("status") != "complete_verified" or previous.get("verified") is not True:
        raise ValueError("prior primary-image SQLite rebase not verified")
    record = backup_set._load_record(receipt)
    manifest = media_backup._load_manifest(Path(record["media"]["store"]), record["media"]["snapshot"])
    source = recovered / "database" / "recovered.sqlite"
    image_db = image_output / "image-paths.sqlite"
    source_sha = db_audit.digest_regular(source, db_audit.MAX_DB)[1]
    image_sha = db_audit.digest_regular(image_db, db_audit.MAX_DB)[1]
    mapping, count, _ = _mapped_folders(source, image_db, manifest, recovered)
    return record, mapping, count, source_sha, image_sha


def _safe_output(output: Path, record: dict, recovered: Path, image_output: Path, fresh: bool):
    isolated.private_output(output, record)
    for other in (recovered, image_output):
        if backup_set._under(output, other) or backup_set._under(other, output):
            raise ValueError("scratch clone overlaps earlier verified recovery")
    if fresh and (output.exists() or output.is_symlink()):
        raise ValueError("new clone directory already exists")


def _check_rebased(path: Path, mapping: dict[int, str], expected_files: int):
    with _readonly(path) as conn:
        for folder_id, new_path in mapping.items():
            row = conn.execute("SELECT path FROM folders WHERE id=?", (folder_id,)).fetchall()
            if row != [(new_path,)]:
                raise ValueError("scratch all-file folder mapping did not persist")
        files = conn.execute("SELECT count(*) FROM files").fetchone()[0]
        if files != expected_files:
            raise ValueError("scratch file count changed while rebasing")
    if db_audit.sqlite_integrity(path)["integrityCheck"] is not True:
        raise ValueError("scratch full-file rebase failed SQLite integrity")


def _marker(output: Path, report: dict):
    raw = json.dumps(report, sort_keys=True, separators=(",", ":"), allow_nan=False).encode() + b"\n"
    if len(raw) > MAX_MARKER:
        raise ValueError("scratch proof marker exceeds size bound")
    fd = os.open(output / MARKER, os.O_CREAT | os.O_EXCL | os.O_WRONLY |
                 getattr(os, "O_NOFOLLOW", 0), 0o600)
    with os.fdopen(fd, "wb") as target:
        target.write(raw)
        target.flush()
        os.fsync(target.fileno())
    if os.name != "nt":
        fd = os.open(output, os.O_RDONLY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)


def rebase(receipt: Path, recovered: Path, image_output: Path, output: Path) -> dict:
    receipt, recovered, image_output, output = map(Path, (receipt, recovered, image_output, output))
    record, mapping, count, source_sha, image_sha = _validate_inputs(receipt, recovered, image_output)
    _safe_output(output, record, recovered, image_output, True)
    output.mkdir(mode=0o700)
    clone = output / "all-file-paths.sqlite"
    isolated.byte_copy(image_output / "image-paths.sqlite", clone, image_sha, db_audit.MAX_DB)
    with sqlite3.connect(clone, timeout=10) as connection:
        connection.execute("BEGIN IMMEDIATE")
        for folder_id, directory in sorted(mapping.items()):
            if connection.execute("UPDATE folders SET path=? WHERE id=?",
                                  (directory, folder_id)).rowcount != 1:
                raise ValueError("a saved file folder vanished during scratch transaction")
        connection.commit()
    _check_rebased(clone, mapping, count)
    clone_hash = db_audit.digest_regular(clone, db_audit.MAX_DB)[1]
    newer, same_mapping, same_count, same_source_sha, same_image_sha = _validate_inputs(
        receipt, recovered, image_output
    )
    if (newer != record or same_mapping != mapping or same_count != count or
            same_source_sha != source_sha or same_image_sha != image_sha):
        raise ValueError("previously verified backup evidence changed during rebase")
    result = _report(receipt, recovered, image_output, output,
                     source_sha, image_sha, clone_hash, len(mapping), count, record["media"]["snapshot"])
    _marker(output, result)
    return result


def audit(receipt: Path, recovered: Path, image_output: Path, output: Path) -> dict:
    receipt, recovered, image_output, output = map(Path, (receipt, recovered, image_output, output))
    record, mapping, count, source_sha, image_sha = _validate_inputs(receipt, recovered, image_output)
    _safe_output(output, record, recovered, image_output, False)
    if not output.exists():
        return {"status": "no_output", "verified": False, **FLAGS}
    if output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output:
        raise ValueError("unsafe full-file rebase scratch directory")
    marker = output / MARKER
    if not marker.exists():
        if marker.is_symlink():
            raise ValueError("unsafe incomplete marker")
        return {"status": "incomplete", "verified": False, **FLAGS}
    expected = {MARKER, "all-file-paths.sqlite"}
    if {p.name for p in output.iterdir()} != expected:
        raise ValueError("missing or extra files in scratch rebase")
    for entry in output.iterdir():
        if not stat.S_ISREG(entry.lstat().st_mode):
            raise ValueError("scratch rebase contains symlink or unsafe entry")
    clone = output / "all-file-paths.sqlite"
    clone_hash = db_audit.digest_regular(clone, db_audit.MAX_DB)[1]
    _check_rebased(clone, mapping, count)
    expected_report = _report(receipt, recovered, image_output, output,
                              source_sha, image_sha, clone_hash, len(mapping), count,
                              record["media"]["snapshot"])
    db_audit.regular(marker, MAX_MARKER)
    saved = json.loads(marker.read_bytes())
    if saved != expected_report:
        raise ValueError("scratch completion marker mismatches verified originals")
    return {"status": "complete_verified", "verified": True, **expected_report}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("rebase", "audit"):
        action = commands.add_parser(name)
        action.add_argument("--backup-set-receipt", type=Path, required=True)
        action.add_argument("--recovered-output", type=Path, required=True)
        action.add_argument("--image-rebase-output", type=Path, required=True)
        action.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        if args.command == "rebase":
            report = rebase(args.backup_set_receipt, args.recovered_output, args.image_rebase_output, args.output)
        else:
            report = audit(args.backup_set_receipt, args.recovered_output, args.image_rebase_output, args.output)
        print(json.dumps(report, sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, sqlite3.DatabaseError, media_backup.BackupError) as exc:
        print("disposable all-file path rebase refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
