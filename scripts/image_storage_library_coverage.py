#!/usr/bin/env python3
"""Read-only saved-SQLite image-primary-file coverage audit for media backups.

Never reads active originals, changes a DB, restores files or enables packing.
A matching historical path and size is NOT a coherent snapshot or live proof.
"""
from __future__ import annotations

import argparse
from collections import Counter
import json
from pathlib import Path
import sqlite3
import sys
import tempfile
from urllib.parse import quote

import image_storage_backup_set as backup_set
import image_storage_db_config_audit as db_audit
import image_storage_historical_paths as historical
import media_library_backup as media_backup

MAX_SQL_ROWS = 250_000
MAX_SNAPSHOT_MEMBERS = 1_000_000
MAX_EXAMPLES = 20

QUERY = (
    'SELECT i.id, a.file_id, d.path, f.basename, f.size, f.zip_file_id, d.zip_file_id '
    'FROM images AS i '
    'LEFT JOIN images_files AS a ON a.image_id = i.id AND a."primary" = 1 '
    'LEFT JOIN files AS f ON f.id = a.file_id '
    'LEFT JOIN folders AS d ON d.id = f.parent_folder_id '
    'ORDER BY i.id, a.file_id LIMIT ?'
)


def media_index(manifest: dict) -> tuple[dict, dict]:
    roots = historical._roots(manifest["roots"])
    entries = manifest["entries"]
    if not isinstance(entries, list) or len(entries) > MAX_SNAPSHOT_MEMBERS:
        raise ValueError("snapshot is too large for bounded coverage audit")
    lookup = {}
    for entry in entries:
        if not isinstance(entry, dict) or entry.get("root") not in roots:
            raise ValueError("invalid original-media snapshot member")
        rel = entry.get("path")
        if not isinstance(rel, str):
            raise ValueError("invalid saved media path")
        media_backup._relpath(rel)
        key = (entry["root"], rel)
        if key in lookup:
            raise ValueError("duplicate saved backup member path")
        if (type(entry.get("size")) is not int or entry["size"] < 0 or
                not isinstance(entry.get("sha256"), str) or
                not backup_set.DIGEST.fullmatch(entry["sha256"])):
            raise ValueError("invalid backup entry digest or size")
        lookup[key] = entry
    return roots, lookup


def classify_primary(rows: list[tuple], roots: dict, lookup: dict) -> dict:
    ident = rows[0][0]
    result = {"imageID": ident, "status": "missingPrimary"}
    if len(rows) > 1:
        result["status"] = "ambiguousPrimary"
        return result
    _, file_id, folder, basename, size, zip_file_id, zip_folder_id = rows[0]
    if file_id is None or folder is None or basename is None:
        return result
    if zip_file_id is not None or zip_folder_id is not None:
        result["status"] = "unsupportedZIP"
        return result
    if type(size) is not int or size <= 0:
        result["status"] = "unknownSize"
        return result
    try:
        old_path = historical._historical_path(folder, basename)
    except ValueError:
        result["status"] = "unsafeHistoricalPath"
        return result
    matches = []
    for label, root in roots.items():
        try:
            relative = old_path.relative_to(root).as_posix()
        except ValueError:
            continue
        media_backup._relpath(relative)
        match = lookup.get((label, relative))
        if match is not None:
            matches.append((label, relative, match))
    if not matches:
        result["status"] = "notInSavedSnapshot"
    elif len(matches) > 1:
        result["status"] = "ambiguousSnapshotPath"
    else:
        label, relative, member = matches[0]
        if member["size"] != size:
            result["status"] = "sizeMismatch"
        else:
            result["status"] = "covered"
            result["memberKey"] = (label, relative)
    return result


def audit_database(database: Path, roots: dict, lookup: dict, max_examples: int = 10) -> dict:
    if type(max_examples) is not int or max_examples < 0 or max_examples > MAX_EXAMPLES:
        raise ValueError("invalid bounded example count")
    uri = "file:" + quote(str(database), safe="/") + "?mode=ro&immutable=1"
    connection = sqlite3.connect(uri, uri=True, timeout=5)
    try:
        connection.execute("PRAGMA query_only=ON")
        cursor = connection.execute(QUERY, (MAX_SQL_ROWS + 1,))
        inspected = []
        current_id, current_rows = None, []
        count_rows = 0
        for row in cursor:
            count_rows += 1
            if count_rows > MAX_SQL_ROWS:
                raise ValueError("image primary join exceeds safe audit row bound")
            image_id = row[0]
            if type(image_id) is not int or image_id <= 0:
                raise ValueError("invalid saved image identity")
            if current_id is not None and image_id != current_id:
                inspected.append(classify_primary(current_rows, roots, lookup))
                current_rows = []
            current_id = image_id
            current_rows.append(row)
        if current_rows:
            inspected.append(classify_primary(current_rows, roots, lookup))
        occurrences = Counter(
            item["memberKey"] for item in inspected if "memberKey" in item
        )
        for item in inspected:
            if "memberKey" in item and occurrences[item["memberKey"]] > 1:
                item["status"] = "sharedHistoricalSource"
        counts = Counter(item["status"] for item in inspected)
        failures = sorted((x for x in inspected if x["status"] != "covered"),
                          key=lambda x: x["imageID"])
        examples = [{"imageID": x["imageID"], "reason": x["status"]}
                    for x in failures[:max_examples]]
        total = len(inspected)
        complete = total > 0 and counts["covered"] == total
        return {
            "totalSavedImages": total,
            "coveredImageCount": counts["covered"],
            "problemImageCount": total - counts["covered"],
            "counts": dict(sorted(counts.items())),
            "problemExamples": examples,
            "exampleLimit": max_examples,
            "allSavedPrimaryImagePathsCovered": complete,
        }
    finally:
        connection.close()


def inspect(backup_set_receipt: Path, max_examples: int = 10) -> dict:
    if not backup_set_receipt.is_absolute() or backup_set_receipt.is_symlink():
        raise ValueError("backup-set receipt must be an absolute regular path")
    backup_set.verify_receipt(backup_set_receipt)
    record = backup_set._load_record(backup_set_receipt)
    manifest = media_backup._load_manifest(Path(record["media"]["store"]),
                                           record["media"]["snapshot"])
    roots, lookup = media_index(manifest)
    with tempfile.TemporaryDirectory(prefix="stash-storage-image-coverage-") as scratch:
        database = Path(record["database"]["path"])
        if record["database"]["kind"] == "sqlite-with-blobs-zip":
            database_copy = Path(scratch) / "saved.sqlite"
            db_audit.sqlite_from_zip(database, database_copy)
            database = database_copy
        if db_audit.sqlite_integrity(database)["integrityCheck"] is not True:
            raise ValueError("saved SQLite backup integrity failed")
        inspected = audit_database(database, roots, lookup, max_examples)
    backup_set.verify_receipt(backup_set_receipt)
    return {
        "format": "stashbooru-saved-image-media-coverage",
        "version": 1,
        "readOnly": True,
        "mediaBackupObjectsVerified": True,
        "savedDatabaseIntegrityVerified": True,
        "originalFilesRead": False,
        "savedMediaSnapshot": record["media"]["snapshot"],
        **inspected,
        "databaseMediaSnapshotCoherent": False,
        "allSavedMediaConsumersVerified": False,
        "liveCatalogueVerified": False,
        "fullLibraryRecovered": False,
        "transactionalRollbackVerified": False,
        "canActivate": False,
        "originalEvictionEnabled": False,
    }


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--backup-set-receipt", required=True, type=Path)
    parser.add_argument("--max-examples", type=int, default=10)
    args = parser.parse_args(argv)
    try:
        print(json.dumps(inspect(args.backup_set_receipt, args.max_examples),
                         sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, sqlite3.DatabaseError,
            media_backup.BackupError) as exc:
        print("saved image-media coverage refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
