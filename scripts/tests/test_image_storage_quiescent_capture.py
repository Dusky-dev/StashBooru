"""Operator-quiesced capture: SQLite WAL, source stability and fail-closed proof."""
from pathlib import Path
import json
import sqlite3
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_quiescent_capture as capture
import media_library_backup as media


class QuiescentCaptureTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self.media = root / "images"
        self.store = root / "backup-store"
        self.parent = root / "private"
        for directory in (self.media, self.store):
            directory.mkdir()
        self.parent.mkdir(mode=0o700)
        self.paths = (self.media / "one.png", self.media / "two.png")
        self.paths[0].write_bytes(b"synthetic-one-original")
        self.paths[1].write_bytes(b"synthetic-two-original")
        self.originals = {p.name: p.read_bytes() for p in self.paths}
        self.database = root / "live.sqlite"
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
            (201, self.paths[0].name, self.paths[0].stat().st_size),
            (202, self.paths[1].name, self.paths[1].stat().st_size),
        ])
        connection.executemany('INSERT INTO images_files VALUES (?, ?, 1)',
                               [(101, 201), (102, 202)])
        connection.commit()
        connection.close()
        self.config = root / "live.yml"
        self.config.write_text("database: live.sqlite\n")
        self.output = self.parent / "new-capture"

    def capture(self, acknowledged=True):
        return capture.capture(self.database, self.config, self.store,
                               {"images": self.media}, self.output,
                               acknowledged=acknowledged)

    def test_capture_then_offline_reaudit_after_original_loss(self):
        report = self.capture()
        self.assertTrue(report["sourceStabilityObserved"])
        self.assertEqual(report["coveredImageCount"], 2)
        self.assertEqual(report["savedImageCount"], 2)
        self.assertFalse(report["coherentSnapshotVerified"])
        self.assertFalse(report["canActivate"])
        self.assertFalse(report["originalEvictionEnabled"])
        self.assertEqual(capture.audit(self.output)["status"], "complete_verified")
        for path in self.paths:
            self.assertEqual(path.read_bytes(), self.originals[path.name])
        for path in self.paths:
            path.unlink()
        self.media.rmdir()
        self.assertTrue(capture.audit(self.output)["verified"])
        self.assertFalse(self.media.exists())

    def test_no_acknowledgement_or_existing_destination_refused(self):
        with self.assertRaises(ValueError):
            self.capture(acknowledged=False)
        self.assertFalse(self.output.exists())
        self.capture()
        with self.assertRaises(ValueError):
            self.capture()
        with self.assertRaises(ValueError):
            capture.capture(self.database, self.config, self.store,
                            {"images": self.media}, self.media / "unsafe",
                            acknowledged=True)

    def test_wal_transactions_included_in_online_sqlite_backup(self):
        connection = sqlite3.connect(self.database)
        self.addCleanup(connection.close)
        self.assertEqual(connection.execute("PRAGMA journal_mode=WAL").fetchone()[0], "wal")
        connection.execute("CREATE TABLE wal_check (id INTEGER)")
        connection.execute("INSERT INTO wal_check VALUES (57)")
        connection.commit()
        self.capture()
        recovered = sqlite3.connect(self.output / "database.sqlite")
        try:
            self.assertEqual(recovered.execute("SELECT id FROM wal_check").fetchall(), [(57,)])
        finally:
            recovered.close()
        self.assertTrue(capture.audit(self.output)["verified"])

    def test_media_changed_during_snapshot_fails_without_success_marker(self):
        actual = media.backup
        def modified(store, roots):
            result = actual(store, roots)
            self.paths[0].write_bytes(b"changed while backing up")
            return result
        with patch.object(media, "backup", modified):
            with self.assertRaises(ValueError):
                self.capture()
        self.assertFalse((self.output / capture.MARKER).exists())
        self.assertEqual(capture.audit(self.output)["status"], "incomplete")

    def test_database_modified_during_capture_fails_closed(self):
        actual = media.backup
        def changed_database(store, roots):
            result = actual(store, roots)
            with sqlite3.connect(self.database) as conn:
                conn.execute("INSERT INTO images VALUES (900)")
            return result
        with patch.object(media, "backup", changed_database):
            with self.assertRaises(ValueError):
                self.capture()
        self.assertFalse((self.output / capture.MARKER).exists())

    def test_config_changed_during_capture_fails_closed(self):
        actual = media.backup
        def changed_config(store, roots):
            result = actual(store, roots)
            self.config.write_text("newconfig: changed\n")
            return result
        with patch.object(media, "backup", changed_config):
            with self.assertRaises(ValueError):
                self.capture()
        self.assertFalse((self.output / capture.MARKER).exists())

    def test_historical_missing_primary_does_not_mark_capture_complete(self):
        with sqlite3.connect(self.database) as conn:
            conn.execute("DELETE FROM images_files WHERE image_id = 102")
        with self.assertRaises(ValueError):
            self.capture()
        self.assertFalse((self.output / capture.MARKER).exists())

    def test_saved_artifact_tamper_and_forged_capabilities_fail(self):
        self.capture()
        marker = self.output / capture.MARKER
        report = json.loads(marker.read_text())
        report["originalEvictionEnabled"] = True
        marker.write_text(json.dumps(report))
        with self.assertRaises(ValueError):
            capture.audit(self.output)
        report["originalEvictionEnabled"] = False
        marker.write_text(json.dumps(report))
        self.assertTrue(capture.audit(self.output)["verified"])
        (self.output / "database.sqlite").write_bytes(b"tampered")
        with self.assertRaises((ValueError, sqlite3.DatabaseError)):
            capture.audit(self.output)

    def test_unexpected_scratch_file_and_symlink_fail(self):
        self.capture()
        extra = self.output / "unexpected.txt"
        extra.write_text("unrecognized")
        with self.assertRaises(ValueError):
            capture.audit(self.output)
        extra.unlink()
        source = self.output / "config.yml"
        source.unlink()
        source.symlink_to(self.config)
        with self.assertRaises(ValueError):
            capture.audit(self.output)


if __name__ == "__main__":
    unittest.main()
