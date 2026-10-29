"""Backup-set receipts: immutable identification is not atomic snapshot proof."""
import json
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_backup_set as paired
import media_library_backup as backup


class BackupSetTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.media = self.root / "originals"
        self.store = self.root / "media-backups"
        self.private = self.root / "private"
        self.media.mkdir()
        self.store.mkdir()
        self.private.mkdir(mode=0o700)
        (self.media / "one.png").write_bytes(b"fake media 1")
        (self.media / "two.png").write_bytes(b"fake media 2")
        self.snapshot = backup.backup(self.store, {"images": self.media})["snapshot"]
        self.database = self.root / "saved.sqlite"
        sql = sqlite3.connect(self.database)
        sql.execute("CREATE TABLE images (id INTEGER PRIMARY KEY)")
        sql.execute("INSERT INTO images VALUES (1)")
        sql.commit()
        sql.close()
        self.config = self.root / "saved-config.yml"
        self.config.write_text("database: saved.sqlite\n", encoding="utf-8")
        self.receipt = self.private / "set.receipt.json"

    def create(self):
        return paired.create_receipt(self.database, self.config, self.store,
                                     self.snapshot, self.receipt)

    def test_create_verify_is_private_and_does_not_authorize_activation(self):
        before_db, before_config = self.database.read_bytes(), self.config.read_bytes()
        result = self.create()
        self.assertTrue(result["verified"])
        self.assertFalse(result["coherentSnapshotVerified"])
        self.assertFalse(result["canActivate"])
        self.assertFalse(result["originalEvictionEnabled"])
        record = paired._load_record(self.receipt)
        self.assertEqual(record["media"]["files"], 2)
        self.assertFalse(record["claims"]["transactionalRollbackVerified"])
        self.assertEqual(paired.verify_receipt(self.receipt)["mediaObjectsVerified"], 2)
        self.assertEqual(before_db, self.database.read_bytes())
        self.assertEqual(before_config, self.config.read_bytes())

    def test_reject_existing_receipt_and_media_root_target(self):
        self.create()
        with self.assertRaises(ValueError):
            self.create()
        with self.assertRaises(ValueError):
            paired.create_receipt(self.database, self.config, self.store, self.snapshot,
                                  self.media / "receipt.json")
        with self.assertRaises(ValueError):
            paired.create_receipt(self.database, self.config, self.store, self.snapshot,
                                  self.store / "receipt.json")

    def test_changed_config_or_database_fails_closed(self):
        self.create()
        self.config.write_text("database: replaced.sqlite\n")
        with self.assertRaises(ValueError):
            paired.verify_receipt(self.receipt)
        self.config.write_text("database: saved.sqlite\n")
        self.database.write_bytes(b"corrupted")
        with self.assertRaises((ValueError, sqlite3.DatabaseError)):
            paired.verify_receipt(self.receipt)

    def test_changed_media_object_refused(self):
        self.create()
        manifest = backup._load_manifest(self.store, self.snapshot)
        obj = backup._object_path(self.store, manifest["entries"][0]["sha256"])
        obj.write_bytes(b"modified")
        with self.assertRaises((ValueError, backup.BackupError)):
            paired.verify_receipt(self.receipt)

    def test_forged_capabilities_and_manifest_are_refused(self):
        self.create()
        record = json.loads(self.receipt.read_text())
        record["claims"]["canActivate"] = True
        self.receipt.write_text(json.dumps(record))
        with self.assertRaises(ValueError):
            paired.verify_receipt(self.receipt)
        record["claims"]["canActivate"] = False
        record["media"]["manifestSHA256"] = "a" * 64
        self.receipt.write_text(json.dumps(record))
        with self.assertRaises(ValueError):
            paired.verify_receipt(self.receipt)

    def test_symlink_and_private_directory_policy(self):
        symlink = self.private / "linked.json"
        symlink.symlink_to(self.receipt)
        with self.assertRaises((ValueError, OSError)):
            paired.verify_receipt(symlink)
        public = self.root / "public"
        public.mkdir(mode=0o755)
        with self.assertRaises(ValueError):
            paired.create_receipt(self.database, self.config, self.store, self.snapshot,
                                  public / "public.json")


if __name__ == "__main__":
    unittest.main()
