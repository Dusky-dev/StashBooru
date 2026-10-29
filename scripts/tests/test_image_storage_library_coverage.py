"""Saved SQLite image-path and verified media-snapshot coverage tests."""
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_library_coverage as coverage
import image_storage_backup_set as backup_set
import media_library_backup as media_backup


class ImageCoverageTests(unittest.TestCase):
    def setUp(self):
        t = tempfile.TemporaryDirectory()
        self.addCleanup(t.cleanup)
        root = Path(t.name)
        self.media = root / "media"
        self.store = root / "backups"
        self.private = root / "private"
        for folder in (self.media, self.store):
            folder.mkdir()
        self.private.mkdir(mode=0o700)
        (self.media / "one.png").write_bytes(b"one-image")
        (self.media / "two.png").write_bytes(b"two-image")
        self.snapshot = media_backup.backup(self.store, {"images": self.media})["snapshot"]
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
            (201, "one.png", 9), (202, "two.png", 9)])
        connection.executemany('INSERT INTO images_files VALUES (?, ?, 1)',
                               [(101, 201), (102, 202)])
        connection.commit()
        connection.close()
        self.config = root / "saved.yml"
        self.config.write_text("database: saved.sqlite\n")
        self.receipt = self.private / "set.json"
        backup_set.create_receipt(self.database, self.config, self.store,
                                  self.snapshot, self.receipt)

    def edit_sql(self, statement):
        conn = sqlite3.connect(self.database)
        conn.execute(statement)
        conn.commit()
        conn.close()
        # A change to the saved DB invalidates the old receipt; create a new
        # receipt instead of circumventing the required backup-set verification.
        self.receipt = self.private / (self.receipt.stem + "-new.json")
        backup_set.create_receipt(self.database, self.config, self.store,
                                  self.snapshot, self.receipt)

    def test_full_saved_image_path_and_size_coverage_without_originals(self):
        for original in self.media.iterdir():
            original.unlink()
        self.media.rmdir()
        report = coverage.inspect(self.receipt)
        self.assertEqual(report["totalSavedImages"], 2)
        self.assertEqual(report["coveredImageCount"], 2)
        self.assertTrue(report["allSavedPrimaryImagePathsCovered"])
        self.assertFalse(report["databaseMediaSnapshotCoherent"])
        self.assertFalse(report["canActivate"])
        self.assertFalse(report["originalEvictionEnabled"])
        self.assertFalse(self.media.exists())

    def test_missing_primary_is_reported_without_false_success(self):
        self.edit_sql("DELETE FROM images_files WHERE image_id=102")
        report = coverage.inspect(self.receipt)
        self.assertFalse(report["allSavedPrimaryImagePathsCovered"])
        self.assertEqual(report["counts"]["missingPrimary"], 1)
        self.assertEqual(report["problemExamples"][0]["imageID"], 102)

    def test_wrong_historical_path_is_not_covered_by_matching_content_elsewhere(self):
        self.edit_sql("UPDATE files SET basename='renamed.png' WHERE id=201")
        report = coverage.inspect(self.receipt)
        self.assertEqual(report["counts"]["notInSavedSnapshot"], 1)
        self.assertEqual(report["coveredImageCount"], 1)

    def test_saved_size_mismatch_is_reported(self):
        self.edit_sql("UPDATE files SET size=999 WHERE id=201")
        self.assertEqual(coverage.inspect(self.receipt)["counts"]["sizeMismatch"], 1)

    def test_zip_source_and_unscanned_size_fail_closed(self):
        self.edit_sql("UPDATE files SET zip_file_id=8 WHERE id=201")
        self.assertEqual(coverage.inspect(self.receipt)["counts"]["unsupportedZIP"], 1)
        self.edit_sql("UPDATE files SET zip_file_id=NULL, size=-1 WHERE id=201")
        self.assertEqual(coverage.inspect(self.receipt)["counts"]["unknownSize"], 1)

    def test_shared_primary_path_does_not_prove_two_independent_files(self):
        self.edit_sql('UPDATE images_files SET file_id=201 WHERE image_id=102')
        report = coverage.inspect(self.receipt)
        self.assertEqual(report["counts"]["sharedHistoricalSource"], 2)
        self.assertFalse(report["allSavedPrimaryImagePathsCovered"])

    def test_ambiguous_duplicate_primary_is_detected(self):
        self.edit_sql('INSERT INTO images_files VALUES (101, 202, 1)')
        self.assertEqual(coverage.inspect(self.receipt)["counts"]["ambiguousPrimary"], 1)

    def test_sqlite_zip_backup_is_supported_without_unpacking_blobs(self):
        archived = self.database.with_suffix(".zip")
        with zipfile.ZipFile(archived, "w") as target:
            target.write(self.database, "saved.sqlite")
            target.writestr("blobs/11/test", b"blob")
        receipt = self.private / "zip-set.json"
        backup_set.create_receipt(archived, self.config, self.store,
                                  self.snapshot, receipt)
        result = coverage.inspect(receipt)
        self.assertTrue(result["allSavedPrimaryImagePathsCovered"])
        self.assertFalse(result["fullLibraryRecovered"])

    def test_modified_backup_set_is_refused(self):
        self.config.write_text("database: something-else.sqlite\n")
        with self.assertRaises(ValueError):
            coverage.inspect(self.receipt)

    def test_example_bounds_are_enforced(self):
        with self.assertRaises(ValueError):
            coverage.inspect(self.receipt, max_examples=1000)


if __name__ == "__main__":
    unittest.main()
