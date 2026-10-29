"""Isolated DB/config/media recovery; no live library mutations."""
import json
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_isolated_recovery as isolated
import image_storage_backup_set as backup_set
import media_library_backup as backup


class IsolatedRecoveryTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        root = Path(self.tmp.name)
        self.media = root / "originals"
        self.store = root / "backups"
        self.private = root / "private"
        self.media.mkdir()
        self.store.mkdir()
        self.private.mkdir(mode=0o700)
        (self.media / "test image.png").write_bytes(b"sample media bytes")
        self.snapshot = backup.backup(self.store, {"images": self.media})["snapshot"]
        self.database = root / "saved.sqlite"
        sql = sqlite3.connect(self.database)
        sql.execute("CREATE TABLE images (id INTEGER PRIMARY KEY)")
        sql.execute("INSERT INTO images VALUES (7)")
        sql.commit()
        sql.close()
        self.config = root / "config.yml"
        self.config.write_bytes(b"database: saved.sqlite\n")
        self.receipt = self.private / "backup-set.json"
        backup_set.create_receipt(self.database, self.config, self.store,
                                  self.snapshot, self.receipt)
        self.output = self.private / "recovery"
        self.member = "images:test image.png"

    def test_complete_scratch_db_config_media_does_not_modify_sources(self):
        before = (self.database.read_bytes(), self.config.read_bytes(),
                  (self.media / "test image.png").read_bytes())
        report = isolated.rehearse(self.receipt, self.output, self.member)
        self.assertTrue(report["readOnlyAgainstLibrary"])
        self.assertEqual(report["recoveredFiles"], 3)
        self.assertEqual(isolated.audit(self.receipt, self.output)["status"], "complete_verified")
        for unsafe in ("canActivate", "originalEvictionEnabled", "databaseRestoreIntegrated",
                       "configurationRestoreIntegrated", "transactionalRollbackVerified",
                       "nativeServingVerified", "coherentSnapshotVerified"):
            self.assertFalse(report[unsafe])
        self.assertEqual(before, (self.database.read_bytes(), self.config.read_bytes(),
                                 (self.media / "test image.png").read_bytes()))
        db = sqlite3.connect(self.output / "database" / "recovered.sqlite")
        self.assertEqual(db.execute("SELECT id FROM images").fetchone()[0], 7)
        db.close()
        self.assertEqual((self.output / "media" / "images" / "test image.png").read_bytes(),
                         before[-1])

    def test_backup_recovery_after_original_media_disappears(self):
        (self.media / "test image.png").unlink()
        self.media.rmdir()
        report = isolated.rehearse(self.receipt, self.output, self.member)
        self.assertEqual(report["mediaMember"], self.member)
        self.assertTrue(isolated.audit(self.receipt, self.output)["verified"])

    def test_sqlite_zip_backup_can_restore_only_private_sqlite_copy(self):
        archive = self.database.with_suffix(".zip")
        with zipfile.ZipFile(archive, "w") as zipped:
            zipped.write(self.database, "stash.sqlite")
            zipped.writestr("blobs/aa/some-blob", b"extra blob")
        other_receipt = self.private / "zip-backup-set.json"
        backup_set.create_receipt(archive, self.config, self.store,
                                  self.snapshot, other_receipt)
        result = isolated.rehearse(other_receipt, self.output, self.member)
        self.assertTrue(result["isolatedScratchOnly"])
        self.assertEqual(isolated.audit(other_receipt, self.output)["status"], "complete_verified")
        self.assertFalse(result["completeMediaLibraryRecovered"])
        self.assertFalse(result["databaseRestoreIntegrated"])

    def test_missing_output_and_interrupted_are_not_success(self):
        self.assertEqual(isolated.audit(self.receipt, self.output)["status"], "no_output")
        self.output.mkdir(mode=0o700)
        self.assertEqual(isolated.audit(self.receipt, self.output)["status"], "incomplete")
        with self.assertRaises(ValueError):
            isolated.rehearse(self.receipt, self.output, self.member)

    def test_tampering_with_recovered_files_or_marker_is_refused(self):
        isolated.rehearse(self.receipt, self.output, self.member)
        target = self.output / "config" / "config.yml"
        target.write_bytes(b"tampered")
        with self.assertRaises(ValueError):
            isolated.audit(self.receipt, self.output)
        target.write_bytes(self.config.read_bytes())
        marker = self.output / isolated.MARKER
        report = json.loads(marker.read_text())
        report["canActivate"] = True
        marker.write_text(json.dumps(report))
        with self.assertRaises(ValueError):
            isolated.audit(self.receipt, self.output)

    def test_forbidden_live_media_root_and_existing_target(self):
        with self.assertRaises(ValueError):
            isolated.rehearse(self.receipt, self.media / "output", self.member)
        with self.assertRaises(ValueError):
            isolated.rehearse(self.receipt, self.store / "output", self.member)
        with self.assertRaises(ValueError):
            isolated.rehearse(self.receipt, self.private / "uncreated" / "target", self.member)

    def test_missing_media_member_cannot_be_reconstructed_by_guess(self):
        with self.assertRaises(ValueError):
            isolated.rehearse(self.receipt, self.output, "images:../outside")
        with self.assertRaises(ValueError):
            isolated.rehearse(self.receipt, self.output, "images:missing.png")


if __name__ == "__main__":
    unittest.main()
