"""Entire saved file catalogue rebase: private copy only, no live paths."""
from pathlib import Path
import json
import sqlite3
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_disposable_all_file_rebase as all_files
import image_storage_disposable_path_rebase as image_rebase
import image_storage_disposable_snapshot_restore as restored
import image_storage_backup_set as backup_set
import media_library_backup as backup


class CompleteFileRebaseTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name)
        self.media = root / "images"
        self.other = root / "other-media"
        self.store = root / "backup-store"
        self.private = root / "private"
        for folder in (self.media, self.other, self.store):
            folder.mkdir()
        self.private.mkdir(mode=0o700)
        (self.media / "first.png").write_bytes(b"first-image-contents")
        (self.media / "second.png").write_bytes(b"second-image-contents")
        (self.other / "clip.mp4").write_bytes(b"synthetic-nonimage-file")
        self.snapshot = backup.backup(self.store, {
            "images": self.media, "scenes": self.other,
        })["snapshot"]
        self.database = root / "source.sqlite"
        connection = sqlite3.connect(self.database)
        # Real StashBooru stores the persistent WAL journal mode in its DB.
        self.assertEqual(connection.execute("PRAGMA journal_mode=WAL").fetchone(), ("wal",))
        connection.executescript("""
            CREATE TABLE images (id INTEGER PRIMARY KEY);
            CREATE TABLE folders (id INTEGER PRIMARY KEY, path TEXT, zip_file_id INTEGER);
            CREATE TABLE files (id INTEGER PRIMARY KEY, basename TEXT, size INTEGER,
                                parent_folder_id INTEGER, zip_file_id INTEGER);
            CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" INTEGER);
        """)
        connection.executemany("INSERT INTO folders VALUES (?, ?, NULL)", [
            (1, str(self.media)), (2, str(self.other)),
        ])
        connection.executemany("INSERT INTO images VALUES (?)", [(101,), (102,)])
        for file_id, folder_id, basename, path in [
            (11, 1, "first.png", self.media / "first.png"),
            (12, 1, "second.png", self.media / "second.png"),
            (13, 2, "clip.mp4", self.other / "clip.mp4"),
        ]:
            connection.execute("INSERT INTO files VALUES (?, ?, ?, ?, NULL)",
                               (file_id, basename, path.stat().st_size, folder_id))
        connection.executemany('INSERT INTO images_files VALUES (?, ?, 1)',
                               [(101, 11), (102, 12)])
        connection.commit()
        connection.close()
        self.config = root / "saved.yml"
        self.config.write_text("database: source.sqlite\n")
        self.receipt = self.private / "backup-set.json"
        backup_set.create_receipt(self.database, self.config, self.store,
                                  self.snapshot, self.receipt)
        self.recovered = self.private / "full-recovery"
        restored.rehearse(self.receipt, self.recovered)
        self.image_output = self.private / "image-rebase"
        image_rebase.rebase(self.receipt, self.recovered, self.image_output)
        self.output = self.private / "all-rebase"

    def run_rebase(self):
        return all_files.rebase(self.receipt, self.recovered, self.image_output, self.output)

    def run_audit(self):
        return all_files.audit(self.receipt, self.recovered, self.image_output, self.output)

    def test_wal_source_clones_verify_from_the_sqlite_file_without_sidecars(self):
        self.assertEqual(sqlite3.connect(self.database).execute(
            "PRAGMA journal_mode").fetchone(), ("wal",))
        self.assertEqual(self.run_rebase()["allSavedRegularFileFoldersRebased"], True)
        self.assertEqual(self.run_audit()["status"], "complete_verified")
        output_db = self.output / "all-file-paths.sqlite"
        with sqlite3.connect(output_db) as connection:
            self.assertEqual(connection.execute("PRAGMA journal_mode").fetchone(), ("delete",))
            self.assertEqual(connection.execute("SELECT path FROM folders WHERE id=2").fetchone(),
                             (str(self.recovered / "media" / "scenes"),))
        self.assertFalse((self.output / "all-file-paths.sqlite-wal").exists())
        self.assertFalse((self.output / "all-file-paths.sqlite-shm").exists())

    def test_every_saved_file_path_maps_to_disposable_media(self):
        result = self.run_rebase()
        self.assertEqual(result["savedFileRecordsVerified"], 3)
        self.assertEqual(result["savedFolderRecordsRebased"], 2)
        self.assertTrue(result["allSavedRegularFileFoldersRebased"])
        self.assertTrue(self.run_audit()["verified"])
        for key in all_files.FLAGS:
            self.assertFalse(result[key])
        with sqlite3.connect(self.output / "all-file-paths.sqlite") as db:
            mapped = dict(db.execute("SELECT id, path FROM folders").fetchall())
        self.assertEqual(mapped, {
            1: str(self.recovered / "media" / "images"),
            2: str(self.recovered / "media" / "scenes"),
        })
        with sqlite3.connect(self.database) as db:
            original = dict(db.execute("SELECT id, path FROM folders"))
        self.assertEqual(original, {1: str(self.media), 2: str(self.other)})
        with sqlite3.connect(self.recovered / "database" / "recovered.sqlite") as db:
            saved = dict(db.execute("SELECT id, path FROM folders"))
        self.assertEqual(saved, original)
        self.assertEqual((self.other / "clip.mp4").read_bytes(),
                         (self.recovered / "media" / "scenes" / "clip.mp4").read_bytes())

    def test_missing_all_original_media_roots_does_not_block_rebase(self):
        for root in (self.media, self.other):
            for path in root.iterdir():
                path.unlink()
            root.rmdir()
        self.assertTrue(self.run_rebase()["allSavedRegularFileFoldersRebased"])
        self.assertEqual(self.run_audit()["status"], "complete_verified")

    def test_existing_output_or_nested_live_path_refused(self):
        self.run_rebase()
        with self.assertRaises(ValueError):
            self.run_rebase()
        with self.assertRaises(ValueError):
            all_files.rebase(self.receipt, self.recovered, self.image_output,
                             self.media / "unsafe")
        with self.assertRaises(ValueError):
            all_files.rebase(self.receipt, self.recovered, self.image_output,
                             self.image_output / "unsafe")

    def test_extra_media_file_in_scratch_and_bad_marker_fail(self):
        self.run_rebase()
        marker = self.output / all_files.MARKER
        saved = json.loads(marker.read_text())
        saved["canActivate"] = True
        marker.write_text(json.dumps(saved))
        with self.assertRaises(ValueError):
            self.run_audit()
        saved["canActivate"] = False
        marker.write_text(json.dumps(saved))
        self.assertTrue(self.run_audit()["verified"])
        (self.output / "extra").write_text("unauthorized")
        with self.assertRaises(ValueError):
            self.run_audit()

    def test_modified_rebased_sqlite_is_not_silently_accepted(self):
        self.run_rebase()
        with sqlite3.connect(self.output / "all-file-paths.sqlite") as db:
            db.execute("UPDATE folders SET path='/unsafe' WHERE id=2")
        with self.assertRaises(ValueError):
            self.run_audit()

    def test_unmapped_extra_folder_fails_before_writes(self):
        with sqlite3.connect(self.recovered / "database" / "recovered.sqlite") as db:
            db.execute("INSERT INTO folders VALUES (99, '/unused', NULL)")
        with self.assertRaises((ValueError, sqlite3.DatabaseError)):
            self.run_rebase()
        self.assertFalse(self.output.exists())

    def test_zip_nonimage_source_fails_closed(self):
        # Build a separate valid saved backup set with a zipped nonimage entry
        # before running the prerequisite phases.
        with sqlite3.connect(self.database) as conn:
            conn.execute("UPDATE files SET zip_file_id=9 WHERE id=13")
        replacement = self.private / "changed-set.json"
        backup_set.create_receipt(self.database, self.config, self.store, self.snapshot, replacement)
        second_restore = self.private / "second-full"
        second_images = self.private / "second-images"
        restored.rehearse(replacement, second_restore)
        image_rebase.rebase(replacement, second_restore, second_images)
        with self.assertRaises(ValueError):
            all_files.rebase(replacement, second_restore, second_images, self.output)
        self.assertFalse(self.output.exists())

    def test_interrupted_sqlite_copy_never_publishes_marker(self):
        with patch.object(all_files, "_check_rebased", side_effect=OSError("interruption")):
            with self.assertRaises(OSError):
                self.run_rebase()
        self.assertEqual(self.run_audit()["status"], "incomplete")
        self.assertFalse((self.output / all_files.MARKER).exists())


if __name__ == "__main__":
    unittest.main()
