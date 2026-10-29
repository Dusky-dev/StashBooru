"""Fail-closed checks of complete media backup and selected Image coverage."""
from __future__ import annotations

import copy
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_backup_preflight as preflight
import media_library_backup as backup


class BackupPreflightTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.media = root / "media"
        self.store = root / "backup"
        self.media.mkdir()
        self.store.mkdir()
        (self.media / "folder").mkdir()
        self.first = self.media / "folder" / "original.png"
        self.second = self.media / "variant.jpg"
        self.extra = self.media / "unselected.png"
        self.first.write_bytes(b"first image container" * 117)
        self.second.write_bytes(b"second image container" * 149)
        self.extra.write_bytes(b"third image container" * 63)
        self.group = {
            "id": "stack:5",
            "representative": "image:1",
            "members": [
                {"id": "image:1", "path": str(self.first)},
                {"id": "image:2", "path": str(self.second)},
            ],
        }
        self.snapshot = backup.backup(self.store, {"media": self.media})["snapshot"]
        self.spec = root / "stacks.json"
        self.spec.write_text(json.dumps({"stacks": [self.group]}))

    def originals(self):
        return {
            p.name: (hashlib.sha256(p.read_bytes()).hexdigest(), p.stat().st_mtime_ns)
            for p in (self.first, self.second, self.extra)
        }

    def check(self, group=None, snapshot=None):
        return preflight.preflight(
            group if group is not None else self.group,
            self.store,
            snapshot if snapshot is not None else self.snapshot,
        )

    def test_verified_snapshot_covers_exact_current_images_without_side_effects(self):
        before = self.originals()
        result = self.check()
        self.assertTrue(result["readOnly"])
        self.assertTrue(result["verified"])
        self.assertTrue(result["mediaBackupVerified"])
        self.assertEqual(result["selectedImagesCovered"], 2)
        self.assertEqual(result["snapshotFilesVerified"], 3)
        self.assertFalse(result["nativePackingEnabled"])
        self.assertFalse(result["nativeServingVerified"])
        self.assertFalse(result["databaseBackupVerified"])
        self.assertFalse(result["originalEvictionEnabled"])
        self.assertEqual(result["imageSHA256"]["image:1"], before[self.first.name][0])
        self.assertEqual(self.originals(), before)
        self.assertEqual(sorted(x.name for x in self.store.iterdir()), ["objects", "snapshots"])

    def test_modified_source_rejected_even_with_intact_old_backup(self):
        self.second.write_bytes(b"a modified current image")
        self.assertTrue(backup.verify(self.store, self.snapshot)["verified"])
        with self.assertRaisesRegex(backup.BackupError, "differs"):
            self.check()

    def test_corrupt_unselected_backup_object_fails_whole_snapshot_verification(self):
        manifest = backup._load_manifest(self.store, self.snapshot)
        entry = next(x for x in manifest["entries"] if x["path"] == "unselected.png")
        backup._object_path(self.store, entry["sha256"]).write_bytes(b"corrupt")
        with self.assertRaisesRegex(backup.BackupError, "failed verification"):
            self.check()

    def test_missing_selected_source_in_previous_snapshot_rejected(self):
        created_later = self.media / "new.png"
        created_later.write_bytes(b"not in snapshot")
        changed = copy.deepcopy(self.group)
        changed["members"][1]["path"] = str(created_later)
        with self.assertRaisesRegex(backup.BackupError, "not included"):
            self.check(changed)

    def test_duplicate_members_and_missing_representative_rejected(self):
        for mutate in (
            lambda g: g["members"].append(g["members"][0].copy()),
            lambda g: g.update(representative="image:999"),
            lambda g: g["members"][1].update(id="image:1"),
        ):
            group = copy.deepcopy(self.group)
            mutate(group)
            with self.assertRaises(backup.BackupError):
                self.check(group)

    def test_symlink_path_and_directory_alias_rejected(self):
        file_alias = self.media / "alias.png"
        file_alias.symlink_to(self.first)
        aliased = copy.deepcopy(self.group)
        aliased["members"][0]["path"] = str(file_alias)
        with self.assertRaisesRegex(backup.BackupError, "symlink"):
            self.check(aliased)
        folder_alias = self.media / "folder-alias"
        folder_alias.symlink_to(self.media / "folder", target_is_directory=True)
        aliased["members"][0]["path"] = str(folder_alias / "original.png")
        with self.assertRaisesRegex(backup.BackupError, "symlink"):
            self.check(aliased)

    def test_source_outside_snapshot_root_rejected(self):
        external = self.store.parent / "external.png"
        external.write_bytes(b"external")
        group = copy.deepcopy(self.group)
        group["members"][1]["path"] = str(external)
        with self.assertRaisesRegex(backup.BackupError, "not covered"):
            self.check(group)

    def test_invalid_snapshot_and_symlinked_store_rejected(self):
        with self.assertRaises(backup.BackupError):
            self.check(snapshot="not-a-snapshot")
        store_alias = self.store.parent / "store-alias"
        store_alias.symlink_to(self.store, target_is_directory=True)
        with self.assertRaisesRegex(backup.BackupError, "non-symlink"):
            preflight.preflight(self.group, store_alias, self.snapshot)

    def test_cli_success_and_failure_are_read_only(self):
        before = self.originals()
        cmd = [
            sys.executable, str(Path(preflight.__file__)),
            "--stacks", str(self.spec),
            "--group", "stack:5",
            "--store", str(self.store),
            "--snapshot", self.snapshot,
        ]
        done = subprocess.run(cmd, text=True, capture_output=True, check=True)
        report = json.loads(done.stdout)
        self.assertEqual(report["selectedImagesCovered"], 2)
        self.assertEqual(self.originals(), before)
        self.second.write_bytes(b"new content")
        failed = subprocess.run(cmd, text=True, capture_output=True)
        self.assertNotEqual(failed.returncode, 0)
        self.assertFalse(failed.stdout.strip())


if __name__ == "__main__":
    unittest.main()
