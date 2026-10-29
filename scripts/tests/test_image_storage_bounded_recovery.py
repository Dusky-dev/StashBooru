"""Fail-closed recovery of bounded mapped candidate images to private scratch."""
from pathlib import Path
import hashlib
import json
import sqlite3
import sys
import tempfile
import unittest
import zipfile

from PIL import Image

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_bounded_recovery as recovery
import image_storage_backup_set as backup_set
import image_storage_pack_proof as packing
import media_library_backup as backup


class BoundedRecoveryTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        root = Path(tmp.name)
        self.media = root / "old-library"
        self.store = root / "backup-store"
        self.private = root / "private"
        self.media.mkdir()
        self.store.mkdir()
        self.private.mkdir(mode=0o700)
        one, two = self.media / "first image.png", self.media / "second image.png"
        picture = Image.new("RGB", (128, 128), (80, 100, 120))
        picture.save(one)
        picture.putpixel((64, 64), (100, 120, 140))
        picture.save(two)
        self.original_bytes = {one.name: one.read_bytes(), two.name: two.read_bytes()}
        self.snapshot = backup.backup(self.store, {"images": self.media})["snapshot"]
        self.database = root / "database.sqlite"
        conn = sqlite3.connect(self.database)
        conn.executescript("""
            CREATE TABLE folders (id INTEGER PRIMARY KEY, path TEXT, zip_file_id INTEGER);
            CREATE TABLE files (id INTEGER PRIMARY KEY, parent_folder_id INTEGER,
                                basename TEXT, size INTEGER, zip_file_id INTEGER);
            CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" BOOLEAN);
        """)
        conn.execute("INSERT INTO folders VALUES (1, ?, NULL)", (str(self.media),))
        for i, path in enumerate((one, two)):
            conn.execute("INSERT INTO files VALUES (?, 1, ?, ?, NULL)",
                         (i + 11, path.name, len(self.original_bytes[path.name])))
            conn.execute('INSERT INTO images_files VALUES (?, ?, 1)', (i + 201, i + 11))
        conn.commit()
        conn.close()
        self.config = root / "saved.yml"
        self.config.write_text("database: database.sqlite\n")
        self.set_receipt = self.private / "set.json"
        backup_set.create_receipt(self.database, self.config, self.store, self.snapshot, self.set_receipt)
        self.candidate_id = "stack-17-delta-" + "a" * 24
        archive = self.private / (self.candidate_id + ".pack")
        group = {"id": "stack:17", "representative": "image:201", "members": [
            {"id": "image:201", "path": str(one)}, {"id": "image:202", "path": str(two)}
        ]}
        proof = packing.pack_proof(group, "delta", archive, 0, 0)
        proof.update({
            "archivePath": str(archive), "group": "stack:17",
            "mediaBackupSnapshot": self.snapshot, "backupVerified": True,
            "nativePackingCandidateOnly": True, "nativeServingVerified": False,
            "databaseBackupVerified": False, "archiveActivated": False,
        })
        self.candidate_receipt = self.private / (self.candidate_id + ".receipt.json")
        self.candidate_receipt.write_text(json.dumps({
            "version": 1, "archiveSHA256": hashlib.sha256(archive.read_bytes()).hexdigest(),
            "report": proof,
        }))
        self.output = self.private / "new-isolated-recovery"

    def do_rehearsal(self):
        return recovery.rehearse(self.candidate_receipt, self.set_receipt, self.output)

    def do_audit(self):
        return recovery.audit(self.candidate_receipt, self.set_receipt, self.output)

    def test_restore_all_mapped_members_and_keep_originals(self):
        report = self.do_rehearsal()
        self.assertEqual(report["restoredMediaCount"], 2)
        self.assertEqual(report["restoredFileCount"], 4)
        self.assertTrue(report["historicalPathMappingVerified"])
        for key in ("databaseMediaSnapshotCoherent", "fullLibraryRecovered",
                    "nativeServingVerified", "canActivate", "originalEvictionEnabled"):
            self.assertFalse(report[key])
        self.assertEqual(self.do_audit()["status"], "complete_verified")
        for name, expected in self.original_bytes.items():
            self.assertEqual((self.output / "media" / "images" / name).read_bytes(), expected)
            self.assertEqual((self.media / name).read_bytes(), expected)

    def test_shared_tiles_candidate_restores_all_members(self):
        original = self.candidate_receipt
        archive = self.private / ("stack-17-shared-tiles-" + "b" * 24 + ".pack")
        first = self.media / "first image.png"
        second = self.media / "second image.png"
        group = {"id": "stack:17", "representative": "image:201", "members": [
            {"id": "image:201", "path": str(first)},
            {"id": "image:202", "path": str(second)},
        ]}
        proof = packing.pack_proof(group, "shared-tiles", archive, 0, 0)
        proof.update({
            "archivePath": str(archive), "group": "stack:17",
            "mediaBackupSnapshot": self.snapshot, "backupVerified": True,
            "nativePackingCandidateOnly": True, "nativeServingVerified": False,
            "databaseBackupVerified": False, "archiveActivated": False,
        })
        self.candidate_receipt = archive.with_suffix(".receipt.json")
        self.candidate_receipt.write_text(json.dumps({
            "version": 1,
            "archiveSHA256": hashlib.sha256(archive.read_bytes()).hexdigest(),
            "report": proof,
        }))
        report = self.do_rehearsal()
        self.assertEqual(report["candidateMethod"], "shared-tiles")
        self.assertEqual(report["restoredMediaCount"], 2)
        self.assertTrue(self.do_audit()["verified"])
        self.assertTrue(original.exists())

    def test_restore_without_any_original_files(self):
        for path in self.media.iterdir():
            path.unlink()
        self.media.rmdir()
        self.assertEqual(self.do_rehearsal()["restoredMediaCount"], 2)
        self.assertTrue(self.do_audit()["verified"])
        self.assertFalse(self.media.exists())

    def test_incomplete_and_duplicate_output_never_pass(self):
        self.assertEqual(self.do_audit()["status"], "no_output")
        self.output.mkdir(mode=0o700)
        self.assertEqual(self.do_audit()["status"], "incomplete")
        with self.assertRaises(ValueError):
            self.do_rehearsal()
        self.output.rmdir()
        with self.assertRaises(ValueError):
            recovery.rehearse(self.candidate_receipt, self.set_receipt, self.media / "unsafe")

    def test_scratched_bytes_extra_files_and_forged_claims_fail(self):
        self.do_rehearsal()
        media = self.output / "media" / "images" / "first image.png"
        media.write_bytes(b"tampered")
        with self.assertRaises(ValueError):
            self.do_audit()
        media.write_bytes(self.original_bytes[media.name])
        stray = self.output / "media" / "images" / "stray"
        stray.write_bytes(b"x")
        with self.assertRaises(ValueError):
            self.do_audit()
        stray.unlink()
        marker = self.output / recovery.MARKER
        saved = json.loads(marker.read_text())
        saved["originalEvictionEnabled"] = True
        marker.write_text(json.dumps(saved))
        with self.assertRaises(ValueError):
            self.do_audit()

    def test_database_historical_path_mismatch_prevents_any_output(self):
        conn = sqlite3.connect(self.database)
        conn.execute("UPDATE files SET basename='other.png' WHERE id=11")
        conn.commit()
        conn.close()
        with self.assertRaises(ValueError):
            self.do_rehearsal()
        self.assertFalse(self.output.exists())

    def test_zip_sqlite_restores_only_extracted_database_and_two_images(self):
        archive = self.database.with_suffix(".zip")
        with zipfile.ZipFile(archive, "w") as out:
            out.write(self.database, "saved.sqlite")
            out.writestr("blobs/aa/unused", b"blob")
        zipped = self.private / "zip-set.json"
        backup_set.create_receipt(archive, self.config, self.store, self.snapshot, zipped)
        report = recovery.rehearse(self.candidate_receipt, zipped, self.output)
        self.assertEqual(report["restoredMediaCount"], 2)
        self.assertTrue(recovery.audit(self.candidate_receipt, zipped, self.output)["verified"])
        self.assertFalse((self.output / "blobs").exists())


if __name__ == "__main__":
    unittest.main()
