#!/usr/bin/env python3
"""Bind separately verified offline backup and scratch recovery evidence.

This is a restart-auditable, isolated rollback rehearsal CHECKPOINT, not an
actual live rollback, application boot proof, or permission to remove originals.
Only a new private output directory is ever written.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sqlite3
import stat
import sys

import image_storage_backup_set as backup_set
import image_storage_db_config_audit as db_audit
import image_storage_disposable_all_file_rebase as all_file
import image_storage_disposable_snapshot_restore as snapshot_restore
import image_storage_isolated_recovery as isolated
import media_library_backup as media_backup

MARKER = "offline-rollback-checkpoint.json"
MAX_MARKER = 16384
FALSE_CLAIMS = {
    "coherentSnapshotVerified": False,
    "restoredApplicationBootVerified": False,
    "fullLibraryRecovered": False,
    "transactionalRollbackVerified": False,
    "crashSafeLiveRollbackVerified": False,
    "canActivate": False,
    "originalEvictionEnabled": False,
}


def _safe_output(receipt: Path, recovered: Path, images: Path, all_paths: Path,
                 output: Path, fresh: bool) -> None:
    record = backup_set._load_record(receipt)
    isolated.private_output(output, record)
    for source in (receipt, recovered, images, all_paths):
        if backup_set._under(output, source) or backup_set._under(source, output):
            raise ValueError("checkpoint overlaps a saved input or earlier scratch output")
    if fresh and (output.exists() or output.is_symlink()):
        raise ValueError("checkpoint must be created at a new output path")


def _evidence(receipt: Path, recovered: Path, images: Path, all_paths: Path,
              output: Path) -> dict:
    original = backup_set.verify_receipt(receipt)
    saved = snapshot_restore.audit(receipt, recovered)
    rebased = all_file.audit(receipt, recovered, images, all_paths)
    if (original.get("verified") is not True or saved.get("verified") is not True or
            saved.get("status") != "complete_verified" or
            rebased.get("verified") is not True or
            rebased.get("status") != "complete_verified" or
            rebased.get("allSavedRegularFileFoldersRebased") is not True or
            any(rebased.get(name) is not False for name in all_file.FLAGS) or
            any(saved.get(name) is not False for name in snapshot_restore.BLOCKED)):
        raise ValueError("offline recovery evidence is incomplete or unsafe")
    db_path = all_paths / "all-file-paths.sqlite"
    _, db_hash = db_audit.digest_regular(db_path, db_audit.MAX_DB)
    return {
        "format": "stashbooru-offline-rollback-checkpoint",
        "version": 1,
        "isolatedScratchOnly": True,
        "readOnlyAgainstOriginals": True,
        "savedBackupSetReverified": True,
        "fullSavedMediaRecoveryReverified": True,
        "allSavedRegularFilePathsReverified": True,
        "backupSetReceiptSHA256": db_audit.digest_regular(receipt, backup_set.RECEIPT_MAX)[1],
        "databaseBackupSHA256": original["databaseSHA256"],
        "configBackupSHA256": original["configSHA256"],
        "mediaManifestSHA256": original["mediaManifestSHA256"],
        "allFileRebasedSQLiteSHA256": db_hash,
        "savedMediaFiles": saved["mediaFilesRestored"],
        "savedFileRecords": rebased["savedFileRecordsVerified"],
        "mediaSnapshot": rebased["mediaSnapshot"],
        "receiptPath": str(receipt),
        "recoveredPath": str(recovered),
        "imageRebasePath": str(images),
        "allFileRebasePath": str(all_paths),
        "outputPath": str(output),
        **FALSE_CLAIMS,
    }


def _inputs(receipt, recovered, images, all_paths, output):
    values = tuple(map(Path, (receipt, recovered, images, all_paths, output)))
    if any(not path.is_absolute() or path != Path(os.path.normpath(path)) for path in values):
        raise ValueError("checkpoint inputs must be canonical absolute paths")
    return values


def prepare(receipt, recovered, images, all_paths, output) -> dict:
    receipt, recovered, images, all_paths, output = _inputs(
        receipt, recovered, images, all_paths, output)
    _safe_output(receipt, recovered, images, all_paths, output, fresh=True)
    before = _evidence(receipt, recovered, images, all_paths, output)
    # The fresh private directory is a write-ahead interruption boundary:
    # absent final marker means incomplete. Never overwrite or auto-resume.
    output.mkdir(mode=0o700)
    if os.name != "nt":
        folder = os.open(output.parent, os.O_RDONLY)
        try:
            os.fsync(folder)
        finally:
            os.close(folder)
    after = _evidence(receipt, recovered, images, all_paths, output)
    if before != after:
        raise ValueError("saved backup/scratch evidence changed during checkpoint")
    raw = json.dumps(after, sort_keys=True, separators=(",", ":"), allow_nan=False).encode() + b"\n"
    if len(raw) > MAX_MARKER:
        raise ValueError("checkpoint evidence too large")
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
    fd = os.open(output / MARKER, flags, 0o600)
    with os.fdopen(fd, "wb") as target:
        target.write(raw)
        target.flush()
        os.fsync(target.fileno())
    if os.name != "nt":
        folder = os.open(output, os.O_RDONLY)
        try:
            os.fsync(folder)
        finally:
            os.close(folder)
    return {"status": "checkpoint_created", "verified": True, **after}


def audit(receipt, recovered, images, all_paths, output) -> dict:
    receipt, recovered, images, all_paths, output = _inputs(
        receipt, recovered, images, all_paths, output)
    _safe_output(receipt, recovered, images, all_paths, output, fresh=False)
    if not output.exists() and not output.is_symlink():
        return {"status": "no_output", "verified": False, **FALSE_CLAIMS}
    if output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output:
        raise ValueError("unsafe rollback checkpoint directory")
    if os.name != "nt" and output.stat().st_mode & 0o077:
        raise ValueError("rollback checkpoint directory is not private")
    paths = list(output.iterdir())
    marker = output / MARKER
    if not marker.exists() and not marker.is_symlink():
        if paths:
            raise ValueError("unexpected content in interrupted rollback checkpoint")
        return {"status": "incomplete", "verified": False, **FALSE_CLAIMS}
    if len(paths) != 1 or paths[0] != marker:
        raise ValueError("unknown file or unexpected directory in checkpoint")
    info = marker.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_size > MAX_MARKER or info.st_size < 1:
        raise ValueError("unsafe, malformed or oversized checkpoint marker")
    if os.name != "nt" and info.st_mode & 0o077:
        raise ValueError("checkpoint marker is not private")
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    with os.fdopen(os.open(marker, flags), "rb") as stream:
        raw = stream.read(MAX_MARKER + 1)
    if len(raw) > MAX_MARKER:
        raise ValueError("checkpoint marker grew during read")
    saved = json.loads(raw)
    expected = _evidence(receipt, recovered, images, all_paths, output)
    if type(saved) is not dict or saved != expected:
        raise ValueError("rollback checkpoint does not match fresh saved evidence")
    return {"status": "complete_verified", "verified": True, **expected}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="action", required=True)
    for action in ("prepare", "audit"):
        p = commands.add_parser(action)
        p.add_argument("--backup-set-receipt", type=Path, required=True)
        p.add_argument("--recovered-output", type=Path, required=True)
        p.add_argument("--image-rebase-output", type=Path, required=True)
        p.add_argument("--all-file-rebase-output", type=Path, required=True)
        p.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        run = prepare if args.action == "prepare" else audit
        print(json.dumps(run(args.backup_set_receipt, args.recovered_output,
                             args.image_rebase_output, args.all_file_rebase_output,
                             args.output), sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, sqlite3.DatabaseError,
            json.JSONDecodeError, media_backup.BackupError) as exc:
        print("offline rollback checkpoint refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
