#!/usr/bin/env python3
"""Create a SEPARATE SQLite copy rebasing saved image paths to recovered scratch.

Never writes the verified recovered SQLite, any original-media root, or the
active StashBooru database. This is not a runnable-server proof.
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
import image_storage_disposable_snapshot_restore as restored
import image_storage_historical_paths as historical
import image_storage_isolated_recovery as isolated
import media_library_backup as media_backup

MARKER = "scratch-image-path-rebase.json"
MAX_MARKER = 24 * 1024
FALSE_FLAGS = {
    "coherentSnapshotVerified": False,
    "fullLibraryRecovered": False,
    "otherMediaPathsRebasedVerified": False,
    "configurationRebasedVerified": False,
    "restoredServerBootVerified": False,
    "nativeServingVerified": False,
    "transactionalRollbackVerified": False,
    "canActivate": False,
    "originalEvictionEnabled": False,
}

SQL = (
    'SELECT a.image_id, f.parent_folder_id, d.path, f.basename, f.size '
    'FROM images_files AS a '
    'JOIN files AS f ON f.id = a.file_id '
    'JOIN folders AS d ON d.id = f.parent_folder_id '
    'WHERE a."primary" = 1 '
    'ORDER BY a.image_id'
)


def _plan(database: Path, old_roots: dict, snapshot: dict,
          recovered_output: Path, expected_images: int) -> dict[int, str]:
    if type(expected_images) is not int or not 1 <= expected_images <= 250000:
        raise ValueError("invalid image count for bounded rebase")
    manifest = {(e["root"], e["path"]): e for e in snapshot["entries"]}
    roots = historical._roots(old_roots)
    uri = "file:" + quote(str(database), safe="/") + "?mode=ro&immutable=1"
    source = sqlite3.connect(uri, uri=True, timeout=5)
    mapping = {}
    found = set()
    try:
        source.execute("PRAGMA query_only=ON")
        for image_id, folder_id, parent, name, size in source.execute(SQL):
            if type(image_id) is not int or image_id <= 0 or image_id in found:
                raise ValueError("duplicate or invalid saved image ID")
            found.add(image_id)
            if type(folder_id) is not int or folder_id <= 0 or type(size) is not int or size <= 0:
                raise ValueError("saved primary image has invalid parent or size")
            old_file = historical._historical_path(parent, name)
            choices = []
            for label, old_root in roots.items():
                try:
                    relative = old_file.relative_to(old_root)
                except ValueError:
                    continue
                rel = relative.as_posix()
                media_backup._relpath(rel)
                member = manifest.get((label, rel))
                if member is not None and member["size"] == size:
                    choices.append((label, relative, member))
            if len(choices) != 1:
                raise ValueError("image primary file cannot map uniquely into saved media")
            label, relative, member = choices[0]
            dest = recovered_output / "media" / label / relative
            if dest.is_symlink() or not dest.is_file() or dest.stat().st_size != member["size"]:
                raise ValueError("restored media member absent or unsafe")
            new_folder = str(dest.parent)
            if folder_id in mapping and mapping[folder_id] != new_folder:
                raise ValueError("saved folder has inconsistent recovered path")
            mapping[folder_id] = new_folder
        if len(found) != expected_images or not mapping:
            raise ValueError("saved image catalogue and mapped image identities differ")
    finally:
        source.close()
    return mapping


def _check_rebased(db: Path, mapping: dict[int, str], expected_images: int) -> None:
    uri = "file:" + quote(str(db), safe="/") + "?mode=ro&immutable=1"
    conn = sqlite3.connect(uri, uri=True, timeout=5)
    try:
        conn.execute("PRAGMA query_only=ON")
        for folder_id, pathname in mapping.items():
            rows = conn.execute("SELECT path FROM folders WHERE id=?", (folder_id,)).fetchall()
            if rows != [(pathname,)]:
                raise ValueError("rebased image folder differs from expected scratch path")
        count = conn.execute('SELECT count(*) FROM images_files WHERE "primary"=1').fetchone()[0]
        if count != expected_images:
            raise ValueError("rebased database changed saved image associations")
    finally:
        conn.close()
    if db_audit.sqlite_integrity(db)["integrityCheck"] is not True:
        raise ValueError("rebased SQLite integrity check failed")


def _provenance(receipt: Path, recovered_output: Path):
    report = restored.audit(receipt, recovered_output)
    if report.get("verified") is not True or report.get("status") != "complete_verified":
        raise ValueError("the disposable full-media recovery has not passed independent audit")
    record = backup_set._load_record(receipt)
    snapshot = media_backup._load_manifest(Path(record["media"]["store"]), record["media"]["snapshot"])
    recovered_db = recovered_output / "database" / "recovered.sqlite"
    _, source_hash = db_audit.digest_regular(recovered_db, db_audit.MAX_DB)
    original_mapping = _plan(recovered_db, snapshot["roots"], snapshot, recovered_output,
                             report["savedImageCount"])
    return report, record, original_mapping, source_hash


def _report(receipt: Path, recovered_output: Path, output: Path,
            report: dict, original_sha: str, rewritten_sha: str, folder_count: int):
    receipt_sha = db_audit.digest_regular(receipt, backup_set.RECEIPT_MAX)[1]
    return {
        "format": "stashbooru-disposable-scratch-image-path-rebase",
        "version": 1,
        "isolatedScratchOnly": True,
        "sourceSavedSQLitePreserved": True,
        "imagePrimaryFoldersRebased": True,
        "savedImageCount": report["savedImageCount"],
        "rebasedFolderCount": folder_count,
        "recoveredOriginalDatabaseSHA256": original_sha,
        "rebasedDatabaseSHA256": rewritten_sha,
        "backupSetReceiptSHA256": receipt_sha,
        "mediaSnapshot": report["mediaSnapshot"],
        "sourceRecoveryPath": str(recovered_output),
        "outputPath": str(output),
        **FALSE_FLAGS,
    }


def _safe_output(output: Path, record: dict, recovered_output: Path, new: bool) -> None:
    isolated.private_output(output, record)
    if (backup_set._under(output, recovered_output) or
            backup_set._under(recovered_output, output)):
        raise ValueError("rebased database may not overwrite saved recovery")
    if new and (output.exists() or output.is_symlink()):
        raise ValueError("rebased SQLite output must be in a new private directory")


def rebase(receipt: Path, recovered_output: Path, output: Path) -> dict:
    receipt, recovered_output, output = map(Path, (receipt, recovered_output, output))
    report, record, mapping, old_hash = _provenance(receipt, recovered_output)
    _safe_output(output, record, recovered_output, True)
    output.mkdir(mode=0o700)
    rewritten = output / "image-paths.sqlite"
    isolated.byte_copy(recovered_output / "database" / "recovered.sqlite", rewritten,
                       old_hash, db_audit.MAX_DB)
    with sqlite3.connect(rewritten, timeout=5) as conn:
        # SQLite's online backup preserves WAL journal_mode in the saved DB.
        # A WAL write in this disposable clone would be invisible to the
        # immutable read-only verifier, which deliberately does not use WAL.
        # Switch only the new scratch copy to a single-file journal *before*
        # any path mutation so the verifier sees committed base-file bytes.
        if conn.execute("PRAGMA journal_mode=DELETE").fetchone() != ("delete",):
            raise ValueError("disposable image SQLite clone did not leave WAL mode")
        conn.execute("BEGIN IMMEDIATE")
        for folder_id, new_path in sorted(mapping.items()):
            update = conn.execute("UPDATE folders SET path=? WHERE id=?", (new_path, folder_id))
            if update.rowcount != 1:
                raise ValueError("failed to rebase a saved image folder")
        conn.commit()
    _check_rebased(rewritten, mapping, report["savedImageCount"])
    _, new_hash = db_audit.digest_regular(rewritten, db_audit.MAX_DB)
    after, _, same_mapping, same_old_hash = _provenance(receipt, recovered_output)
    if after != report or same_mapping != mapping or same_old_hash != old_hash:
        raise ValueError("source recovery changed during path rebase")
    result = _report(receipt, recovered_output, output, report, old_hash, new_hash, len(mapping))
    encoded = json.dumps(result, sort_keys=True, separators=(",", ":")).encode() + b"\n"
    if len(encoded) > MAX_MARKER:
        raise ValueError("scratch rebase completion record too long")
    fd = os.open(output / MARKER, os.O_CREAT | os.O_EXCL | os.O_WRONLY |
                 getattr(os, "O_NOFOLLOW", 0), 0o600)
    with os.fdopen(fd, "wb") as target:
        target.write(encoded)
        target.flush()
        os.fsync(target.fileno())
    if os.name != "nt":
        dir_fd = os.open(output, os.O_RDONLY)
        try:
            os.fsync(dir_fd)
        finally:
            os.close(dir_fd)
    return result


def audit(receipt: Path, recovered_output: Path, output: Path) -> dict:
    receipt, recovered_output, output = map(Path, (receipt, recovered_output, output))
    report, record, mapping, old_hash = _provenance(receipt, recovered_output)
    _safe_output(output, record, recovered_output, False)
    if not output.exists():
        return {"status": "no_output", "verified": False, **FALSE_FLAGS}
    if output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output:
        raise ValueError("unsafe scratch image path rebase output")
    marker = output / MARKER
    if not marker.exists():
        if marker.is_symlink():
            raise ValueError("unsafe scratch marker")
        return {"status": "incomplete", "verified": False, **FALSE_FLAGS}
    expected_files = {MARKER, "image-paths.sqlite"}
    for path in output.iterdir():
        if (path.name not in expected_files or path.is_symlink() or
                not stat.S_ISREG(path.lstat().st_mode)):
            raise ValueError("unexpected scratch rebase file")
    if {p.name for p in output.iterdir()} != expected_files:
        raise ValueError("missing rebased database or proof")
    rewritten = output / "image-paths.sqlite"
    _, rewritten_hash = db_audit.digest_regular(rewritten, db_audit.MAX_DB)
    _check_rebased(rewritten, mapping, report["savedImageCount"])
    expected = _report(receipt, recovered_output, output, report,
                       old_hash, rewritten_hash, len(mapping))
    db_audit.regular(marker, MAX_MARKER)
    saved = json.loads(marker.read_bytes())
    if saved != expected:
        raise ValueError("rebased completion record does not match current artifacts")
    _provenance(receipt, recovered_output)
    return {"status": "complete_verified", "verified": True, **expected}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("rebase", "audit"):
        c = commands.add_parser(name)
        c.add_argument("--backup-set-receipt", required=True, type=Path)
        c.add_argument("--recovered-output", required=True, type=Path)
        c.add_argument("--output", required=True, type=Path)
    args = parser.parse_args(argv)
    try:
        if args.command == "rebase":
            result = rebase(args.backup_set_receipt, args.recovered_output, args.output)
        else:
            result = audit(args.backup_set_receipt, args.recovered_output, args.output)
        print(json.dumps(result, sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, sqlite3.DatabaseError,
            media_backup.BackupError) as exc:
        print("disposable SQLite image path rebase refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
