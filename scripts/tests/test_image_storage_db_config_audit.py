"""Read-only SQLite/config backup inspection with synthetic, disposable fixtures."""
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile
import unittest
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_db_config_audit as audit


class BackupReadinessTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.database = self.root / "stash-backup.sqlite"
        self.config = self.root / "config-backup.yml"
        db = sqlite3.connect(self.database)
        db.execute("CREATE TABLE image (id INTEGER PRIMARY KEY, title TEXT NOT NULL)")
        db.execute("INSERT INTO image VALUES (1, 'test image')")
        db.commit()
        db.close()
        self.config.write_text("database: stash.sqlite\nplugins: []\n", encoding="utf-8")

    def test_verified_sqlite_config_are_not_coherent_or_restorable(self):
        before_db, before_cfg = self.database.read_bytes(), self.config.read_bytes()
        report = audit.inspect(self.database, self.config)
        self.assertTrue(report["sqliteIntegrityVerified"])
        self.assertTrue(report["readOnlyBackupInspection"])
        self.assertEqual(report["sqliteApplicationTables"], 1)
        self.assertFalse(report["databaseConfigSnapshotPairVerified"])
        for key in ("databaseRestoreVerified", "configRestoreVerified", "canActivate",
                    "transactionalRollbackVerified", "originalEvictionEnabled",
                    "archiveBlobsIndividuallyVerified"):
            self.assertFalse(report[key])
        self.assertEqual(before_db, self.database.read_bytes())
        self.assertEqual(before_cfg, self.config.read_bytes())

    def test_zip_sqlite_and_blobs_are_only_partially_verified(self):
        archive = self.root / "stash-backup.zip"
        with zipfile.ZipFile(archive, "w") as zipped:
            zipped.write(self.database, "stash.sqlite")
            zipped.writestr("blobs/ab/hash", b"blob")
        report = audit.inspect(archive, self.config)
        self.assertEqual(report["databaseKind"], "sqlite-with-blobs-zip")
        self.assertTrue(report["archiveBlobsPresent"])
        self.assertFalse(report["archiveBlobsIndividuallyVerified"])

    def test_archive_traversal_and_duplicate_files_refused(self):
        archive = self.root / "stash-backup.zip"
        with zipfile.ZipFile(archive, "w") as zipped:
            zipped.write(self.database, "stash.sqlite")
            zipped.writestr("../escape", b"bad")
        with self.assertRaises(ValueError):
            audit.inspect(archive, self.config)
        with zipfile.ZipFile(archive, "w") as zipped:
            zipped.write(self.database, "stash.sqlite")
            zipped.write(self.database, "other.sqlite")
        with self.assertRaises(ValueError):
            audit.inspect(archive, self.config)

    def test_broken_sqlite_and_missing_config_refused(self):
        self.database.write_bytes(b"not a SQLite database")
        with self.assertRaises((sqlite3.DatabaseError, ValueError)):
            audit.inspect(self.database, self.config)
        self.database.unlink()
        with self.assertRaises(OSError):
            audit.inspect(self.database, self.config)

    def test_symlinked_database_and_config_refused(self):
        linked = self.root / "linked.sqlite"
        linked.symlink_to(self.database)
        with self.assertRaises(ValueError):
            audit.inspect(linked, self.config)
        other = self.root / "other.yml"
        other.symlink_to(self.config)
        with self.assertRaises(ValueError):
            audit.inspect(self.database, other)

    def test_cli_produces_safety_flags_without_restoring(self):
        result = subprocess.run([sys.executable, str(Path(audit.__file__)),
                                 "--database", str(self.database),
                                 "--config", str(self.config)],
                                capture_output=True, text=True, check=True)
        data = json.loads(result.stdout)
        self.assertFalse(data["canActivate"])
        self.assertEqual(data["databaseKind"], "sqlite")


if __name__ == "__main__":
    unittest.main()
