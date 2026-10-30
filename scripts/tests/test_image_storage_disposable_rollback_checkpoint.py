"""Offline rollback rehearsal checkpoint: restart audit, no live changes."""
import json
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_backup_set as backup_set
import image_storage_disposable_all_file_rebase as all_file
import image_storage_disposable_path_rebase as image_rebase
import image_storage_disposable_rollback_checkpoint as checkpoint
import image_storage_disposable_snapshot_restore as restore
import media_library_backup as media


class OfflineRollbackCheckpointTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self.source = root / "media"
        self.store = root / "backup"
        self.private = root / "private"
        for item in (self.source, self.store):
            item.mkdir()
        self.private.mkdir(mode=0o700)
        (self.source / "one.png").write_bytes(b"first original media")
        (self.source / "two.png").write_bytes(b"second original image")
        self.originals = {
            name: (self.source / name).read_bytes() for name in ("one.png", "two.png")
        }
        self.snapshot = media.backup(self.store, {"images": self.source})["snapshot"]
        self.database = root / "original.sqlite"
        with sqlite3.connect(self.database) as db:
            db.executescript("""
                CREATE TABLE images(id INTEGER PRIMARY KEY);
                CREATE TABLE folders(id INTEGER PRIMARY KEY, path TEXT, zip_file_id INTEGER);
                CREATE TABLE files(id INTEGER PRIMARY KEY, basename TEXT, size INTEGER,
                    parent_folder_id INTEGER, zip_file_id INTEGER);
                CREATE TABLE images_files(image_id INTEGER, file_id INTEGER, "primary" INTEGER);
            """)
            db.execute("INSERT INTO folders VALUES (1, ?, NULL)", (str(self.source),))
            for i, name in enumerate(self.originals, start=1):
                db.execute("INSERT INTO images VALUES (?)", (i,))
                db.execute("INSERT INTO files VALUES (?, ?, ?, 1, NULL)",
                           (i, name, (self.source / name).stat().st_size))
                db.execute('INSERT INTO images_files VALUES (?, ?, 1)', (i, i))
        self.config = root / "config.yml"
        self.config.write_text("database: original.sqlite\n")
        self.receipt = self.private / "saved-backup-set.json"
        backup_set.create_receipt(self.database, self.config, self.store,
                                  self.snapshot, self.receipt)
        self.recovered = self.private / "snapshot"
        self.image_paths = self.private / "image-rebase"
        self.all_paths = self.private / "all-rebase"
        restore.rehearse(self.receipt, self.recovered)
        image_rebase.rebase(self.receipt, self.recovered, self.image_paths)
        all_file.rebase(self.receipt, self.recovered, self.image_paths, self.all_paths)
        self.output = self.private / "rollback-checkpoint"

    def prepare(self):
        return checkpoint.prepare(self.receipt, self.recovered, self.image_paths,
                                  self.all_paths, self.output)

    def audit(self):
        return checkpoint.audit(self.receipt, self.recovered, self.image_paths,
                                self.all_paths, self.output)

    def test_recovered_all_file_backup_checkpoint_is_independently_reauditable(self):
        result = self.prepare()
        self.assertTrue(result["verified"])
        self.assertEqual(result["status"], "checkpoint_created")
        self.assertTrue(result["fullSavedMediaRecoveryReverified"])
        self.assertEqual(result["savedMediaFiles"], 2)
        self.assertEqual(result["savedFileRecords"], 2)
        for name in checkpoint.FALSE_CLAIMS:
            self.assertFalse(result[name], name)
        self.assertEqual(self.audit()["status"], "complete_verified")
        self.assertEqual(self.audit()["allFileRebasedSQLiteSHA256"],
                         result["allFileRebasedSQLiteSHA256"])
        for name, original in self.originals.items():
            self.assertEqual((self.source / name).read_bytes(), original)
        self.assertEqual(self.config.read_text(), "database: original.sqlite\n")

    def test_deleting_live_originals_does_not_break_saved_recovery_audit(self):
        self.prepare()
        for name in self.originals:
            (self.source / name).unlink()
        self.source.rmdir()
        self.assertTrue(self.audit()["verified"])

    def test_interrupted_before_marker_requires_new_output(self):
        with patch.object(checkpoint, "_evidence", side_effect=[
                {"verified": True}, OSError("injected power loss")]):
            with self.assertRaises(OSError):
                self.prepare()
        self.assertFalse((self.output / checkpoint.MARKER).exists())
        self.assertEqual(self.audit()["status"], "incomplete")
        with self.assertRaises(ValueError):
            self.prepare()

    def test_missing_output_report_cannot_authorize_rollback(self):
        report = self.audit()
        self.assertEqual(report["status"], "no_output")
        self.assertFalse(report["verified"])
        for name in checkpoint.FALSE_CLAIMS:
            self.assertFalse(report[name])

    def test_reject_tampered_marker_privilege_and_extra_content(self):
        self.prepare()
        marker = self.output / checkpoint.MARKER
        saved = json.loads(marker.read_text())
        saved["canActivate"] = True
        marker.write_text(json.dumps(saved))
        with self.assertRaises(ValueError):
            self.audit()
        saved["canActivate"] = False
        marker.write_text(json.dumps(saved))
        self.assertTrue(self.audit()["verified"])
        (self.output / "extra").write_text("untrusted")
        with self.assertRaises(ValueError):
            self.audit()

    def test_tampered_rebased_sqlite_detected_after_restart(self):
        self.prepare()
        with sqlite3.connect(self.all_paths / "all-file-paths.sqlite") as db:
            db.execute("UPDATE folders SET path='/escape'")
        with self.assertRaises((ValueError, sqlite3.DatabaseError)):
            self.audit()

    def test_refuse_existing_output_and_parent_overlap(self):
        self.prepare()
        with self.assertRaises(ValueError):
            self.prepare()
        with self.assertRaises(ValueError):
            checkpoint.prepare(self.receipt, self.recovered, self.image_paths,
                               self.all_paths, self.recovered / "unsafe")
        with self.assertRaises(ValueError):
            checkpoint.prepare(self.receipt, self.recovered, self.image_paths,
                               self.all_paths, self.source / "unsafe")

    def test_symlink_marker_rejected(self):
        self.output.mkdir(mode=0o700)
        (self.output / checkpoint.MARKER).symlink_to(self.receipt)
        with self.assertRaises(ValueError):
            self.audit()


if __name__ == "__main__":
    unittest.main()
