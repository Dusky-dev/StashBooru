#!/usr/bin/env python3
"""Read-only, fail-closed candidate image ID -> historical media backup path proof.

Consumes a saved P12/P13 receipt and a separately verified backup-set receipt.
Does not use or modify active catalogue/media paths. Historical database
and media snapshots are NOT proven to have been taken atomically.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import sys
import tempfile
from urllib.parse import quote

import image_storage_backup_set as backup_set
import image_storage_db_config_audit as db_audit
import image_storage_missing_original_rehearsal as missing
import image_storage_pack_proof as packing
import media_library_backup as media_backup

MAX_IMAGE = 64 * 1024 * 1024


def _historical_path(folder: str, basename: str) -> Path:
    # Consult historical strings only. Do not resolve or stat the live
    # (potentially missing) media path to "verify" its old identity.
    if not isinstance(folder, str) or not isinstance(basename, str):
        raise ValueError("missing historical image path")
    if (not os.path.isabs(folder) or "\x00" in folder or "\x00" in basename or
            folder != os.path.normpath(folder) or not basename or
            basename in (".", "..") or "/" in basename or "\\" in basename):
        raise ValueError("unsafe or noncanonical historical image path")
    joined = os.path.join(folder, basename)
    if joined != os.path.normpath(joined):
        raise ValueError("unsafe historical source pathname")
    return Path(joined)


def _roots(roots: dict) -> dict[str, Path]:
    if not isinstance(roots, dict) or not roots:
        raise ValueError("snapshot is missing original-media root evidence")
    checked = {}
    for label, root in roots.items():
        if not isinstance(label, str) or not media_backup.LABEL.fullmatch(label):
            raise ValueError("invalid backup root label")
        if not isinstance(root, str) or not os.path.isabs(root) or root != os.path.normpath(root):
            raise ValueError("historical backup root path is unsafe")
        checked[label] = Path(root)
    return checked


def map_historical_paths(database: Path, expected: dict, roots: dict, entries: list) -> list[dict]:
    if not isinstance(expected, dict) or not 2 <= len(expected) <= 16:
        raise ValueError("expected 2-16 archived image identities")
    verified_roots = _roots(roots)
    manifest_members = {}
    for entry in entries:
        if not isinstance(entry, dict) or entry.get("root") not in verified_roots:
            raise ValueError("backup snapshot contains an invalid member")
        rel = entry.get("path")
        if not isinstance(rel, str):
            raise ValueError("invalid backed-up pathname")
        media_backup._relpath(rel)
        pair = (entry["root"], rel)
        if pair in manifest_members:
            raise ValueError("backup contains duplicate member paths")
        if (not isinstance(entry.get("sha256"), str) or
                not backup_set.DIGEST.fullmatch(entry["sha256"]) or
                type(entry.get("size")) is not int or entry["size"] < 0):
            raise ValueError("backup entry checksum or size invalid")
        manifest_members[pair] = entry

    uri = "file:" + quote(str(database), safe="/") + "?mode=ro&immutable=1"
    conn = sqlite3.connect(uri, uri=True, timeout=5)
    try:
        conn.execute("PRAGMA query_only=ON")
        output = []
        for identity, checksum in sorted(expected.items()):
            if (not isinstance(identity, str) or not identity.startswith("image:") or
                    not identity[6:].isdigit() or str(int(identity[6:])) != identity[6:] or
                    int(identity[6:]) < 1 or not isinstance(checksum, str) or
                    not backup_set.DIGEST.fullmatch(checksum)):
                raise ValueError("candidate image ID or checksum is unsafe")
            image_id = int(identity[6:])
            if image_id > 2**63-1:
                raise ValueError("candidate image identity is outside SQLite range")
            rows = conn.execute(
                'SELECT d.path, f.basename, f.size, f.zip_file_id, d.zip_file_id '
                'FROM images_files AS a '
                'JOIN files AS f ON f.id = a.file_id '
                'JOIN folders AS d ON d.id = f.parent_folder_id '
                'WHERE a.image_id = ? AND a."primary" = 1',
                (image_id,),
            ).fetchall()
            if len(rows) != 1 or rows[0][3] is not None or rows[0][4] is not None:
                raise ValueError("image ID has no unique regular primary historical file")
            folder, basename, file_size, _, _ = rows[0]
            source = _historical_path(folder, basename)
            matches = []
            for label, original_root in verified_roots.items():
                try:
                    relative = source.relative_to(original_root).as_posix()
                except ValueError:
                    continue
                media_backup._relpath(relative)
                found = manifest_members.get((label, relative))
                if found is not None:
                    matches.append((label, relative, found))
            if len(matches) != 1:
                raise ValueError("historical image path has no unique saved backup member")
            label, relative, entry = matches[0]
            if checksum != entry["sha256"]:
                raise ValueError("candidate content differs from the historical path's saved backup")
            if type(file_size) is not int:
                raise ValueError("historical primary image file has invalid size")
            size_verified = file_size == entry["size"]
            if file_size > 0 and not size_verified:
                raise ValueError("historical image size disagrees with media snapshot")
            output.append({
                "imageID": image_id,
                "backupMember": label + ":" + relative,
                "sha256": checksum,
                "bytes": entry["size"],
                "historicalFileSizeVerified": size_verified,
            })
        if len(output) != len(expected):
            raise ValueError("not all candidate images were mapped")
        return output
    finally:
        conn.close()


def inspect(candidate_receipt: Path, backup_set_receipt: Path) -> dict:
    if not candidate_receipt.is_absolute() or not backup_set_receipt.is_absolute():
        raise ValueError("both saved receipt paths must be absolute")
    backup_set.verify_receipt(backup_set_receipt)
    set_record = backup_set._load_record(backup_set_receipt)
    archive, candidate = missing._read_receipt(candidate_receipt)
    store = Path(set_record["media"]["store"])
    snapshot = set_record["media"]["snapshot"]
    if candidate["mediaBackupSnapshot"] != snapshot:
        raise ValueError("candidate and backup-set media snapshot identities differ")
    manifest = media_backup._load_manifest(store, snapshot)
    if media_backup.verify(store, snapshot).get("verified") is not True:
        raise ValueError("saved original-media objects are not verified")
    with packing.open_reader(candidate["method"], archive) as reader:
        result = reader.verify()
        if (result.get("verified") is not True or result.get("fidelity") != "byte-exact" or
                set(reader.entries) != set(candidate["sourceSHA256"])):
            raise ValueError("packed archive member identity or byte-exact proof invalid")
        for identity, digest in candidate["sourceSHA256"].items():
            restored = reader.reconstruct(identity)
            if len(restored) > MAX_IMAGE or hashlib.sha256(restored).hexdigest() != digest:
                raise ValueError("candidate member did not reconstruct to its original fingerprint")

    database = Path(set_record["database"]["path"])
    with tempfile.TemporaryDirectory(prefix="stash-candidate-path-map-") as tmp:
        if set_record["database"]["kind"] == "sqlite-with-blobs-zip":
            scratch = Path(tmp) / "readonly.sqlite"
            db_audit.sqlite_from_zip(database, scratch)
            database = scratch
        if not db_audit.sqlite_integrity(database)["integrityCheck"]:
            raise ValueError("saved SQLite backup failed integrity check")
        matches = map_historical_paths(
            database, candidate["sourceSHA256"], manifest["roots"], manifest["entries"]
        )
    # Recheck immutable input artifacts after the point-in-time inspection.
    backup_set.verify_receipt(backup_set_receipt)
    after, proof_after = missing._read_receipt(candidate_receipt)
    if after != archive or proof_after != candidate:
        raise ValueError("candidate proof changed during historical-path inspection")
    return {
        "readOnly": True,
        "candidateGroup": candidate["group"],
        "candidateMethod": candidate["method"],
        "mediaSnapshot": snapshot,
        "candidateArchiveSHA256Verified": True,
        "mediaBackupObjectsVerified": True,
        "savedSQLiteIntegrityVerified": True,
        "historicalPathMappingVerified": True,
        "members": matches,
        "databaseMediaSnapshotCoherent": False,
        "currentCataloguePathsVerified": False,
        "fullLibraryRecovered": False,
        "liveServingVerified": False,
        "transactionalRollbackVerified": False,
        "canActivate": False,
        "originalEvictionEnabled": False,
    }


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate-receipt", type=Path, required=True)
    parser.add_argument("--backup-set-receipt", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        print(json.dumps(inspect(args.candidate_receipt, args.backup_set_receipt), sort_keys=True))
        return 0
    except (ValueError, OSError, KeyError, TypeError, sqlite3.DatabaseError,
            media_backup.BackupError) as exc:
        print("historical candidate path mapping refused: " + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
