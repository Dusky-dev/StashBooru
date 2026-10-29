#!/usr/bin/env python3
"""Isolated recovery of all 2-16 historically mapped P12/P13 candidate images.

Does not touch live paths, activate serving, alter the catalogue or delete originals.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sqlite3
import stat
import sys
import zipfile

import image_storage_backup_set as backup_set
import image_storage_db_config_audit as db_audit
import image_storage_historical_paths as historical
import image_storage_isolated_recovery as isolated
import media_library_backup as media_backup

MARKER = "candidate-media-recovery.json"
MAX_MEMBER = 64 * 1024 * 1024
MAX_TOTAL = 256 * 1024 * 1024
MAX_MARKER = 24 * 1024
FALSE_FLAGS = (
    "databaseMediaSnapshotCoherent", "fullLibraryRecovered",
    "currentCataloguePathsVerified", "nativeServingVerified",
    "transactionalRollbackVerified", "canActivate", "originalEvictionEnabled",
)


def selected_members(proof: dict) -> list[dict]:
    if proof.get("historicalPathMappingVerified") is not True or proof.get("readOnly") is not True:
        raise ValueError("historical mapping was not independently verified")
    members = proof.get("members")
    if not isinstance(members, list) or not 2 <= len(members) <= 16:
        raise ValueError("expected between 2 and 16 verified candidate members")
    ids, names, total = set(), set(), 0
    for entry in members:
        if not isinstance(entry, dict):
            raise ValueError("invalid historical media mapping")
        image_id, name, digest, size = (
            entry.get("imageID"), entry.get("backupMember"), entry.get("sha256"), entry.get("bytes")
        )
        if type(image_id) is not int or image_id < 1 or image_id in ids:
            raise ValueError("invalid or repeated image identity")
        if not isinstance(name, str) or ":" not in name or name in names:
            raise ValueError("invalid or repeated historical backup path")
        root, relative = name.split(":", 1)
        if not media_backup.LABEL.fullmatch(root):
            raise ValueError("invalid media backup root")
        media_backup._relpath(relative)
        if not isinstance(digest, str) or not backup_set.DIGEST.fullmatch(digest):
            raise ValueError("invalid media SHA-256")
        if type(size) is not int or size < 1 or size > MAX_MEMBER:
            raise ValueError("image exceeds per-file scratch bound")
        if entry.get("historicalFileSizeVerified") is not True:
            raise ValueError("historical image size not verified")
        total += size
        if total > MAX_TOTAL:
            raise ValueError("candidate exceeds total scratch size bound")
        ids.add(image_id)
        names.add(name)
    return members


def member_path(output: Path, entry: dict) -> Path:
    label, relative = entry["backupMember"].split(":", 1)
    return isolated._files(output / "media" / label, relative)


def verify_scratch(output: Path, members: list[dict], db_digest: str, config_digest: str) -> None:
    expected = {
        output / "database" / "recovered.sqlite": (db_digest, db_audit.MAX_DB),
        output / "config" / "config.yml": (config_digest, db_audit.MAX_CONFIG),
    }
    for entry in members:
        expected[member_path(output, entry)] = (entry["sha256"], MAX_MEMBER)
    if len(expected) != len(members) + 2:
        raise ValueError("historical media paths overlap in scratch")
    permitted_dirs = {output, output / "database", output / "config", output / "media"}
    for path in expected:
        permitted_dirs.update(path.parents)
    for path in output.rglob("*"):
        st = path.lstat()
        if stat.S_ISDIR(st.st_mode):
            if path not in permitted_dirs:
                raise ValueError("unexpected scratch directory")
        elif not stat.S_ISREG(st.st_mode) or path not in expected and path != output / MARKER:
            raise ValueError("unexpected scratch entry or symlink")
    for path, (checksum, maximum) in expected.items():
        count, actual = db_audit.digest_regular(path, maximum)
        if count < 1 or actual != checksum:
            raise ValueError("recovered image/database/config fingerprint differs")
    if db_audit.sqlite_integrity(output / "database" / "recovered.sqlite")["integrityCheck"] is not True:
        raise ValueError("recovered SQLite integrity check failed")


def inputs(candidate_receipt: Path, set_receipt: Path):
    proof = historical.inspect(candidate_receipt, set_receipt)
    members = selected_members(proof)
    set_data = backup_set._load_record(set_receipt)
    _, candidate_hash = db_audit.digest_regular(candidate_receipt, historical.missing.MAX_RECEIPT)
    _, set_hash = db_audit.digest_regular(set_receipt, backup_set.RECEIPT_MAX)
    return proof, members, set_data, candidate_hash, set_hash


def expected_report(proof, members, output, database_sha, candidate_sha, set_sha):
    return {
        "format": "stashbooru-bounded-mapped-recovery",
        "version": 1,
        "readOnlyAgainstLibrary": True,
        "isolatedScratchOnly": True,
        "candidateGroup": proof["candidateGroup"],
        "candidateMethod": proof["candidateMethod"],
        "mediaSnapshot": proof["mediaSnapshot"],
        "candidateReceiptSHA256": candidate_sha,
        "backupSetReceiptSHA256": set_sha,
        "recoveredDatabaseSHA256": database_sha,
        "historicalPathMappingVerified": True,
        "restoredMediaCount": len(members),
        "restoredFileCount": len(members) + 2,
        "outputPath": str(output),
        "members": members,
        **{flag: False for flag in FALSE_FLAGS},
    }


def rehearse(candidate_receipt: Path, set_receipt: Path, output: Path) -> dict:
    proof, members, record, candidate_sha, set_sha = inputs(candidate_receipt, set_receipt)
    isolated.private_output(output, record)
    if output.exists() or output.is_symlink():
        raise ValueError("scratch destination must not already exist")
    output.mkdir(mode=0o700)
    for name in ("database", "config", "media"):
        (output / name).mkdir(mode=0o700)
    saved_db = Path(record["database"]["path"])
    restored_db = output / "database" / "recovered.sqlite"
    if record["database"]["kind"] == "sqlite-with-blobs-zip":
        db_audit.sqlite_from_zip(saved_db, restored_db)
        os.chmod(restored_db, 0o600)
    else:
        isolated.byte_copy(saved_db, restored_db, record["database"]["sha256"], db_audit.MAX_DB)
    _, db_hash = db_audit.digest_regular(restored_db, db_audit.MAX_DB)
    isolated.byte_copy(Path(record["config"]["path"]), output / "config" / "config.yml",
                       record["config"]["sha256"], db_audit.MAX_CONFIG)
    store = Path(record["media"]["store"])
    for entry in members:
        response = media_backup.restore(store, proof["mediaSnapshot"], output / "media",
                                        entry["backupMember"])
        if response.get("restored") != 1:
            raise ValueError("historical member did not restore exactly once")
    verify_scratch(output, members, db_hash, record["config"]["sha256"])
    fresh, selected, _, latest_candidate_sha, latest_set_sha = inputs(candidate_receipt, set_receipt)
    if (fresh != proof or selected != members or
            (latest_candidate_sha, latest_set_sha) != (candidate_sha, set_sha)):
        raise ValueError("source artifacts changed while reconstructing")
    report = expected_report(proof, members, output, db_hash, candidate_sha, set_sha)
    payload = json.dumps(report, sort_keys=True, separators=(",", ":")).encode() + b"\n"
    if len(payload) > MAX_MARKER:
        raise ValueError("scratch completion proof too large")
    fd = os.open(output / MARKER, os.O_WRONLY | os.O_CREAT | os.O_EXCL |
                 getattr(os, "O_NOFOLLOW", 0), 0o600)
    with os.fdopen(fd, "wb") as destination:
        destination.write(payload)
        destination.flush()
        os.fsync(destination.fileno())
    folder_fd = os.open(output, os.O_RDONLY)
    try:
        os.fsync(folder_fd)
    finally:
        os.close(folder_fd)
    return report


def audit(candidate_receipt: Path, set_receipt: Path, output: Path) -> dict:
    proof, members, record, candidate_sha, set_sha = inputs(candidate_receipt, set_receipt)
    isolated.private_output(output, record)
    if not output.exists():
        return {"status": "no_output", "verified": False,
                "canActivate": False, "originalEvictionEnabled": False}
    if output.is_symlink() or not output.is_dir() or output.resolve(strict=True) != output:
        raise ValueError("invalid private scratch directory")
    marker = output / MARKER
    if not marker.exists():
        if marker.is_symlink():
            raise ValueError("unsafe scratch marker")
        return {"status": "incomplete", "verified": False,
                "canActivate": False, "originalEvictionEnabled": False}
    db_audit.regular(marker, MAX_MARKER)
    saved = json.loads(marker.read_bytes())
    if (not isinstance(saved, dict) or
            not isinstance(saved.get("recoveredDatabaseSHA256"), str) or
            not backup_set.DIGEST.fullmatch(saved["recoveredDatabaseSHA256"])):
        raise ValueError("invalid completion marker")
    expected = expected_report(proof, members, output, saved["recoveredDatabaseSHA256"],
                               candidate_sha, set_sha)
    if saved != expected:
        raise ValueError("scratch marker differs from source evidence")
    verify_scratch(output, members, saved["recoveredDatabaseSHA256"], record["config"]["sha256"])
    after, selected, _, second_candidate_sha, second_set_sha = inputs(candidate_receipt, set_receipt)
    if (after != proof or selected != members or
            (second_candidate_sha, second_set_sha) != (candidate_sha, set_sha)):
        raise ValueError("source evidence changed during scratch audit")
    return {"status": "complete_verified", "verified": True, **expected}


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("rehearse", "audit"):
        command = commands.add_parser(name)
        command.add_argument("--candidate-receipt", required=True, type=Path)
        command.add_argument("--backup-set-receipt", required=True, type=Path)
        command.add_argument("--output", required=True, type=Path)
    args = parser.parse_args(argv)
    try:
        if args.command == "rehearse":
            report = rehearse(args.candidate_receipt, args.backup_set_receipt, args.output)
        else:
            report = audit(args.candidate_receipt, args.backup_set_receipt, args.output)
        print(json.dumps(report, sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, json.JSONDecodeError,
            sqlite3.DatabaseError, zipfile.BadZipFile, media_backup.BackupError) as exc:
        print("bounded multi-image scratch recovery refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
