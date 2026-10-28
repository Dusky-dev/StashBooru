"""Non-destructive media snapshot, incremental reuse and recovery tests."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import media_library_backup as backup


class MediaLibraryBackupTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.media = self.home / "media"
        self.media.mkdir()
        self.store = self.home / "backup"
        self.store.mkdir()
        (self.media / "nested").mkdir()
        (self.media / "nested" / "original.png").write_bytes(b"not really png; original source bytes" * 19)
        (self.media / "same.png").write_bytes((self.media / "nested" / "original.png").read_bytes())
        self.original = (self.media / "nested" / "original.png").read_bytes()

    def snapshot(self):
        return backup.backup(self.store, {"media": self.media})["snapshot"]

    def test_roundtrip_and_individual_recovery(self):
        snapshot = self.snapshot()
        verified = backup.verify(self.store, snapshot)
        self.assertEqual(verified["files"], 2)
        self.assertEqual(verified["objects"], 1)
        self.assertTrue(verified["verified"])
        output = self.home / "recovered"
        result = backup.restore(self.store, snapshot, output, "media:nested/original.png")
        self.assertEqual(result["restored"], 1)
        self.assertEqual((output / "media/nested/original.png").read_bytes(), self.original)
        self.assertFalse((output / "media/same.png").exists())
        self.assertEqual((self.media / "same.png").read_bytes(), self.original)

    def test_incremental_snapshot_reuses_immutable_blobs(self):
        first = self.snapshot()
        (self.media / "nested" / "original.png").write_bytes(b"updated source bytes")
        second = self.snapshot()
        self.assertNotEqual(first, second)
        self.assertEqual(backup.verify(self.store, first)["objects"], 1)
        self.assertEqual(backup.verify(self.store, second)["objects"], 2)
        self.assertEqual(len(list((self.store / "objects").glob("[0-9a-f][0-9a-f]/*"))), 2)
        old = self.home / "old"
        new = self.home / "new"
        backup.restore(self.store, first, old, None)
        backup.restore(self.store, second, new, None)
        self.assertEqual((old / "media/nested/original.png").read_bytes(), self.original)
        self.assertEqual((new / "media/nested/original.png").read_bytes(), b"updated source bytes")

    def test_failed_collision_leaves_original_unchanged(self):
        identifier = self.snapshot()
        output = self.home / "recovered"
        dest = output / "media/nested/original.png"
        dest.parent.mkdir(parents=True)
        dest.write_bytes(b"private existing file")
        with self.assertRaisesRegex(backup.BackupError, "overwrite"):
            backup.restore(self.store, identifier, output, None)
        self.assertEqual(dest.read_bytes(), b"private existing file")
        self.assertFalse((output / "media/same.png").exists())

    def test_corrupt_or_missing_blob_blocks_verify_and_restore(self):
        identifier = self.snapshot()
        item = backup._load_manifest(self.store, identifier)["entries"][0]
        blob = backup._object_path(self.store, item["sha256"])
        blob.write_bytes(b"corrupted!")
        with self.assertRaises(backup.BackupError):
            backup.verify(self.store, identifier)
        with self.assertRaises(backup.BackupError):
            backup.restore(self.store, identifier, self.home / "restored", None)
        self.assertFalse((self.home / "restored" / "media").exists())

    def test_symlink_and_overlap_fail_closed(self):
        if not hasattr(os, "symlink"):
            self.skipTest("no symlinks")
        (self.media / "link").symlink_to(self.home, target_is_directory=True)
        with self.assertRaisesRegex(backup.BackupError, "symlink"):
            self.snapshot()
        (self.media / "link").unlink()
        (self.media / "link.png").symlink_to(self.media / "same.png")
        with self.assertRaisesRegex(backup.BackupError, "symlink"):
            self.snapshot()
        (self.media / "link.png").unlink()
        with self.assertRaisesRegex(backup.BackupError, "separate"):
            backup.backup(self.media, {"root": self.media})

    def test_tampered_manifest_rejects_path_escape_and_duplicates(self):
        identifier = self.snapshot()
        file = backup._snapshot_path(self.store, identifier)
        original = json.loads(file.read_text())
        altered = json.loads(json.dumps(original))
        altered["entries"][0]["path"] = "../../outside"
        file.write_text(json.dumps(altered))
        with self.assertRaisesRegex(backup.BackupError, "unsafe"):
            backup.verify(self.store, identifier)
        altered = json.loads(json.dumps(original))
        altered["entries"].append(altered["entries"][0].copy())
        file.write_text(json.dumps(altered))
        with self.assertRaisesRegex(backup.BackupError, "duplicate"):
            backup.verify(self.store, identifier)

    def test_existing_malformed_object_is_never_replaced(self):
        digest = backup._read_hash(self.media / "same.png")[1]
        obj = backup._object_path(self.store, digest)
        obj.parent.mkdir(parents=True)
        obj.write_bytes(b"wrong data")
        with self.assertRaisesRegex(backup.BackupError, "corrupted"):
            self.snapshot()
        self.assertEqual(obj.read_bytes(), b"wrong data")
        self.assertEqual(list((self.store / "snapshots").glob("*.json")), [])

    def test_nonregular_source_rejects_entire_snapshot(self):
        (self.media / "special").mkdir()
        (self.media / "special" / "work").write_bytes(b"okay")
        # Opening a FIFO is not permitted and must never stall the backup.
        if hasattr(os, "mkfifo"):
            os.mkfifo(self.media / "special" / "pipe")
            with self.assertRaisesRegex(backup.BackupError, "regular"):
                self.snapshot()
        self.assertEqual(list((self.store / "snapshots").glob("*.json")), [])

    def test_cli_json_roundtrip_and_missing_member(self):
        script = Path(backup.__file__)
        command = [sys.executable, str(script), "backup", "--store", str(self.store),
                   "--root", f"media={self.media}"]
        finished = subprocess.run(command, text=True, capture_output=True, check=True)
        snapshot = json.loads(finished.stdout)["snapshot"]
        verify = subprocess.run([sys.executable, str(script), "verify", "--store", str(self.store),
                                 "--snapshot", snapshot], text=True, capture_output=True, check=True)
        self.assertTrue(json.loads(verify.stdout)["verified"])
        missing = subprocess.run([sys.executable, str(script), "restore", "--store", str(self.store),
                                  "--snapshot", snapshot, "--output", str(self.home / "restored"),
                                  "--member", "media:unknown.png"], text=True, capture_output=True)
        self.assertNotEqual(missing.returncode, 0)
        self.assertIn("not found", missing.stderr)


if __name__ == "__main__":
    unittest.main()
