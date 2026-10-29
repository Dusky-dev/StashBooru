#!/usr/bin/env python3
"""Read-only SQLite and configuration backup inspection for storage recovery.

This is NOT database restoration, a coordinated backup, native library
activation, or permission to delete originals. Only explicit backup paths.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import sqlite3
import stat
import sys
import tempfile
import zipfile
from urllib.parse import quote

MAX_DB = 512 * 1024 * 1024
MAX_CONFIG = 8 * 1024 * 1024
MAX_ZIP = 1024 * 1024 * 1024
MAX_MEMBERS = 250000
COPY_CHUNK = 1024 * 1024


def regular(path: Path, max_size: int) -> os.stat_result:
    if not path.is_absolute() or path.resolve(strict=True) != path or path.is_symlink():
        raise ValueError("backup must have an absolute, canonical, non-symlink path")
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= max_size:
        raise ValueError("backup is missing, empty, oversized, or not regular")
    return info


def digest_regular(path: Path, max_size: int) -> tuple[int, str]:
    initial = regular(path, max_size)
    digest = hashlib.sha256()
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    with os.fdopen(os.open(path, flags), "rb") as source:
        opened = os.fstat(source.fileno())
        if not os.path.samestat(initial, opened):
            raise ValueError("backup identity changed while opening")
        length = 0
        while block := source.read(COPY_CHUNK):
            length += len(block)
            if length > max_size:
                raise ValueError("backup grew past its safety limit")
            digest.update(block)
        last_open = os.fstat(source.fileno())
    final = path.lstat()
    stamp = lambda info: (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    if length != initial.st_size or stamp(initial) != stamp(last_open) or stamp(initial) != stamp(final):
        raise ValueError("backup changed while verifying")
    return length, digest.hexdigest()


def sqlite_integrity(path: Path) -> dict:
    # SQLite must not journal, migrate, checkpoint, or write into the backup.
    uri = "file:" + quote(str(path), safe="/") + "?mode=ro&immutable=1"
    connection = sqlite3.connect(uri, uri=True, timeout=5)
    try:
        connection.execute("PRAGMA query_only=ON")
        rows = connection.execute("PRAGMA integrity_check").fetchall()
        if rows != [("ok",)]:
            raise ValueError("SQLite integrity_check failed")
        tables = connection.execute(
            "SELECT count(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        ).fetchone()[0]
        if tables < 1:
            raise ValueError("SQLite backup has no application tables")
        return {"integrityCheck": True, "applicationTables": tables,
                "userVersion": connection.execute("PRAGMA user_version").fetchone()[0]}
    finally:
        connection.close()


def sqlite_from_zip(archive: Path, destination: Path) -> dict:
    with zipfile.ZipFile(archive) as source:
        members = source.infolist()
        if not 1 <= len(members) <= MAX_MEMBERS:
            raise ValueError("invalid bounded ZIP entry count")
        names = set()
        sqlite_members = []
        blobs = 0
        for member in members:
            name = member.filename
            pure = PurePosixPath(name)
            if (not name or name.startswith("/") or "\\" in name or
                    any(part in ("", ".", "..") for part in name.split("/")) or
                    pure.is_absolute() or name in names or member.is_dir() or
                    (member.external_attr >> 16) & 0o170000 == stat.S_IFLNK):
                raise ValueError("ZIP contains an unsafe or duplicate member")
            names.add(name)
            if member.file_size < 0 or member.file_size > MAX_DB:
                raise ValueError("ZIP member exceeds bounded size")
            if "/" not in name and name.lower().endswith(".sqlite"):
                sqlite_members.append(member)
            elif name.startswith("blobs/"):
                blobs += 1
            else:
                raise ValueError("unexpected database-backup ZIP member")
        if len(sqlite_members) != 1:
            raise ValueError("database ZIP requires exactly one root SQLite file")
        selected = sqlite_members[0]
        # Private temporary extraction only. No writes to live or backup paths.
        with source.open(selected) as src, destination.open("xb") as out:
            count = 0
            while block := src.read(COPY_CHUNK):
                count += len(block)
                if count > MAX_DB:
                    raise ValueError("ZIP SQLite expansion exceeds safety limit")
                out.write(block)
            out.flush()
            os.fsync(out.fileno())
        if count != selected.file_size:
            raise ValueError("SQLite ZIP entry has inconsistent size")
        return {"kind": "sqlite-with-blobs-zip", "blobEntries": blobs,
                "databaseEntry": selected.filename, "databaseBytes": count}


def inspect(database: Path, config: Path) -> dict:
    database = Path(database)
    config = Path(config)
    if database == config:
        raise ValueError("database and configuration backup paths must differ")
    if config.suffix.lower() not in (".yml", ".yaml"):
        raise ValueError("configuration backup must be a saved YAML file")
    db_cap = MAX_ZIP if database.suffix.lower() == ".zip" else MAX_DB
    db_size, db_hash = digest_regular(database, db_cap)
    config_size, config_hash = digest_regular(config, MAX_CONFIG)
    kind = "sqlite"
    extra = {"databaseBytes": db_size, "blobEntries": 0}
    if database.suffix.lower() == ".zip":
        with tempfile.TemporaryDirectory(prefix="stash-storage-backup-audit-") as folder:
            temp = Path(folder) / "readonly-audit.sqlite"
            extra = sqlite_from_zip(database, temp)
            info = sqlite_integrity(temp)
    elif database.suffix.lower() in (".sqlite", ".db"):
        info = sqlite_integrity(database)
    else:
        raise ValueError("expected a .sqlite/.db database backup or Stash ZIP backup")
    # Hashes are identity evidence, not a proof that two independently provided
    # snapshots correspond to the same moment or current live configuration.
    db_size_after, db_hash_after = digest_regular(database, db_cap)
    config_size_after, config_hash_after = digest_regular(config, MAX_CONFIG)
    if (db_size_after, db_hash_after, config_size_after, config_hash_after) != (
            db_size, db_hash, config_size, config_hash):
        raise ValueError("backups changed during SQLite integrity verification")
    return {
        "readOnlyBackupInspection": True,
        "databaseKind": extra.get("kind", kind),
        "databaseBytes": db_size,
        "databaseSHA256": db_hash,
        "sqliteIntegrityVerified": info["integrityCheck"],
        "sqliteApplicationTables": info["applicationTables"],
        "sqliteUserVersion": info["userVersion"],
        "archiveBlobsPresent": extra.get("blobEntries", 0) > 0,
        "archiveBlobsIndividuallyVerified": False,
        "configBytes": config_size,
        "configSHA256": config_hash,
        "configSemanticValidationVerified": False,
        "databaseConfigSnapshotPairVerified": False,
        "databaseRestoreVerified": False,
        "configRestoreVerified": False,
        "nativeMediaServingVerified": False,
        "transactionalRollbackVerified": False,
        "canActivate": False,
        "originalEvictionEnabled": False,
        "blockers": [
            "Database and configuration backup files were supplied independently; coherent snapshot identity unverified.",
            "Configuration contents were hashed but not parsed or restored.",
            "Filesystem blob archive members were not independently authenticated.",
            "No isolated live-database restoration, native route integration, transactional rollback or original-file eviction.",
        ],
    }


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--database", required=True, help="Absolute path to saved SQLite or SQLite+blobs ZIP backup")
    parser.add_argument("--config", required=True, help="Absolute path to a separately saved config.yml/.yaml backup")
    args = parser.parse_args(argv)
    try:
        print(json.dumps(inspect(Path(args.database), Path(args.config)), sort_keys=True))
        return 0
    except (ValueError, OSError, sqlite3.DatabaseError, zipfile.BadZipFile, RuntimeError) as exc:
        print("backup readiness inspection refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
