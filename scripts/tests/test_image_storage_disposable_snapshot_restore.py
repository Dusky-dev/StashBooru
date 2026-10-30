"""Disposable full-snapshot media recovery without live original sources."""
from pathlib import Path
import json
import sqlite3
import sys
import tempfile
import unittest
from unittest.mock import patch
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_disposable_snapshot_restore as restore
import image_storage_backup_set as backup_set
import media_library_backup as media


class FullSnapshotTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self.media = root / "originals"
        self.store = root / "saved-media"
        self.private = root / "private"
        self.media.mkdir()
        self.store.mkdir()
        self.private.mkdir(mode=0o700)
        self.sources = [self.media / "first.png", self.media / "second.png",
                        self.media / "unindexed-sidecar.txt"]
        for i, item in enumerate(self.sources):
            item.write_bytes(("synthetic-file-" + str(i)).encode())
        self.original = {p.name: p.read_bytes() for p in self.sources}
        self.snapshot = media.backup(self.store, {"images": self.media})["snapshot"]
        self.database = root / "saved.sqlite"
        connection = sqlite3.connect(self.database)
        connection.executescript("""
            CREATE TABLE images (id INTEGER PRIMARY KEY);
            CREATE TABLE folders (id INTEGER PRIMARY KEY, path TEXT, zip_file_id INTEGER);
            CREATE TABLE files (id INTEGER PRIMARY KEY, basename TEXT, size INTEGER,
                                parent_folder_id INTEGER, zip_file_id INTEGER);
            CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" INTEGER);
        """)
        connection.execute("INSERT INTO folders VALUES (1, ?, NULL)", (str(self.media),))
        connection.executemany("INSERT INTO images VALUES (?)", [(101,), (102,)])
        connection.executemany("INSERT INTO files VALUES (?, ?, ?, 1, NULL)", [
            (11, "first.png", self.sources[0].stat().st_size),
            (12, "second.png", self.sources[1].stat().st_size),
        ])
        connection.executemany('INSERT INTO images_files VALUES (?, ?, 1)',
                               [(101, 11), (102, 12)])
        connection.commit()
        connection.close()
        self.config = root / "saved.yml"
        self.config.write_text("database: saved.sqlite\n")
        self.receipt = self.private / "backup-set.json"
        backup_set.create_receipt(self.database, self.config, self.store, self.snapshot,
                                  self.receipt)
        self.output = self.private / "recovered-scratch"

    def recover(self, **kwargs):
        return restore.rehearse(self.receipt, self.output, **kwargs)

    def audit(self):
        return restore.audit(self.receipt, self.output)

    def test_restores_all_snapshot_members_including_nonindexed_sidecar(self):
        report = self.recover()
        self.assertEqual(report["mediaFilesRestored"], 3)
        self.assertEqual(report["savedImageCount"], 2)
        self.assertTrue(report["allSavedMediaSnapshotMembersRestored"])
        self.assertTrue(report["allSavedPrimaryImagePathsCovered"])
        for key in restore.BLOCKED:
            self.assertFalse(report[key])
        self.assertTrue(self.audit()["verified"])
        self.assertEqual(self.audit()["status"], "complete_verified")
        for name, content in self.original.items():
            self.assertEqual((self.output / "media" / "images" / name).read_bytes(), content)
            self.assertEqual((self.media / name).read_bytes(), content)

    def test_original_media_root_can_be_deleted_before_full_restore(self):
        for path in self.sources:
            path.unlink()
        self.media.rmdir()
        result = self.recover()
        self.assertEqual(result["mediaFilesRestored"], 3)
        self.assertTrue(self.audit()["verified"])
        self.assertFalse(self.media.exists())

    def test_no_scratch_materialization_when_limits_exceeded(self):
        with self.assertRaises(ValueError):
            self.recover(max_files=2)
        self.assertFalse(self.output.exists())
        with self.assertRaises(ValueError):
            self.recover(max_bytes=1)
        self.assertFalse(self.output.exists())
        with self.assertRaises(ValueError):
            self.recover(max_files=0)
        self.assertFalse(self.output.exists())

    def test_reject_existing_output_and_protected_locations(self):
        self.recover()
        with self.assertRaises(ValueError):
            self.recover()
        with self.assertRaises(ValueError):
            restore.rehearse(self.receipt, self.media / "new", max_files=8)
        with self.assertRaises(ValueError):
            restore.rehearse(self.receipt, self.store / "new", max_files=8)

    def test_incomplete_interrupted_restore_is_not_verified(self):
        self.assertEqual(self.audit()["status"], "no_output")
        actual = media.restore
        def failed(*args):
            actual(*args)
            raise OSError("synthetic interruption after media copy")
        with patch.object(media, "restore", failed):
            with self.assertRaises(OSError):
                self.recover()
        self.assertEqual(self.audit()["status"], "incomplete")
        self.assertFalse((self.output / restore.MARKER).exists())

    def test_tampered_file_marker_and_rogue_file_refused(self):
        self.recover()
        selected = self.output / "media" / "images" / "first.png"
        selected.write_bytes(b"tampered")
        with self.assertRaises((ValueError, media.BackupError)):
            self.audit()
        selected.write_bytes(self.original["first.png"])
        forged = self.output / restore.MARKER
        report = json.loads(forged.read_text())
        report["originalEvictionEnabled"] = True
        forged.write_text(json.dumps(report))
        with self.assertRaises(ValueError):
            self.audit()
        report["originalEvictionEnabled"] = False
        forged.write_text(json.dumps(report))
        self.assertTrue(self.audit()["verified"])
        (self.output / "media" / "images" / "rogue").write_bytes(b"unexpected")
        with self.assertRaises(ValueError):
            self.audit()

    def test_corrupt_saved_media_and_missing_database_fail_closed(self):
        first = media._load_manifest(self.store, self.snapshot)["entries"][0]
        media._object_path(self.store, first["sha256"]).write_bytes(b"corrupted")
        with self.assertRaises((ValueError, media.BackupError)):
            self.recover()
        self.assertFalse(self.output.exists())

    def test_sqlite_zip_copy_has_no_blobs_but_clearly_disclaims_full_restore(self):
        archive = self.database.with_suffix(".zip")
        with zipfile.ZipFile(archive, "w") as target:
            target.write(self.database, "saved.sqlite")
            target.writestr("blobs/aa/synthetic", b"not-restored")
        receipt = self.private / "zip-set.json"
        backup_set.create_receipt(archive, self.config, self.store, self.snapshot, receipt)
        self.receipt = receipt
        result = self.recover()
        self.assertTrue(self.audit()["verified"])
        self.assertFalse(result["ZIPDatabaseBlobsRestored"])
        self.assertFalse(result["fullLibraryRecovered"])
        self.assertFalse((self.output / "blobs").exists())

    def test_saved_database_catalogue_gap_refuses_restore(self):
        # Modify DB first, then issue a fresh verified backup-set receipt.
        with sqlite3.connect(self.database) as conn:
            conn.execute("DELETE FROM images_files WHERE image_id=102")
        new_receipt = self.private / "changed-set.json"
        backup_set.create_receipt(self.database, self.config, self.store,
                                  self.snapshot, new_receipt)
        self.receipt = new_receipt
        with self.assertRaises(ValueError):
            self.recover()
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
