"""Native-job packing checks require a verified backup and retain originals."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

import numpy as np
from PIL import Image

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_native_candidate as native
import image_storage_pack_proof as pack
import media_library_backup as backup


class NativeCandidateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.media = root / "media"
        self.store = root / "backup"
        self.output = root / "candidates"
        for folder in (self.media, self.store, self.output):
            folder.mkdir()
        pixels = np.random.default_rng(212).integers(
            0, 256, (110, 140, 4), dtype=np.uint8
        )
        self.first = self.media / "original.png"
        self.second = self.media / "variant.png"
        Image.fromarray(pixels).save(self.first)
        pixels[13:16, 17:23, 1] ^= 12
        Image.fromarray(pixels).save(self.second)
        self.group = {
            "id": "stack:9",
            "representative": "image:100",
            "members": [
                {"id": "image:100", "path": str(self.first)},
                {"id": "image:101", "path": str(self.second)},
            ],
        }
        self.snapshot = backup.backup(self.store, {"media": self.media})["snapshot"]

    def originals(self):
        return {p.name: (p.stat().st_mtime_ns, hashlib.sha256(p.read_bytes()).hexdigest())
                for p in (self.first, self.second)}

    def test_p12_p13_publish_only_independently_verifiable_byte_exact_archives(self):
        before = self.originals()
        for method in pack.METHODS:
            with self.subTest(method=method):
                output = self.output / (method + ".pack")
                evidence = native.candidate(
                    self.group, self.store, self.snapshot, method, output
                )
                self.assertTrue(evidence["backupVerified"])
                self.assertTrue(evidence["verified"])
                self.assertTrue(evidence["nativePackingCandidateOnly"])
                self.assertFalse(evidence["archiveActivated"])
                self.assertFalse(evidence["nativeServingVerified"])
                self.assertFalse(evidence["databaseBackupVerified"])
                self.assertFalse(evidence["originalEvictionEnabled"])
                self.assertEqual(evidence["fidelity"], "byte-exact")
                self.assertEqual(evidence["mediaBackupSnapshot"], self.snapshot)
                self.assertEqual(evidence["group"], self.group["id"])
                with pack.open_reader(method, output) as reader:
                    self.assertTrue(reader.verify()["verified"])
                    for item in self.group["members"]:
                        self.assertEqual(reader.reconstruct(item["id"]),
                                         Path(item["path"]).read_bytes())
                self.assertEqual(before, self.originals())

    def test_stale_original_rejected_before_any_archive_is_written(self):
        before = self.originals()
        self.second.write_bytes(b"changed since backup")
        output = self.output / "stale.pack"
        with self.assertRaisesRegex(backup.BackupError, "differs"):
            native.candidate(self.group, self.store, self.snapshot, "delta", output)
        self.assertFalse(output.exists())
        self.assertNotEqual(before, self.originals())

    def test_missing_and_corrupted_backup_rejected(self):
        output = self.output / "missing.pack"
        with self.assertRaises(backup.BackupError):
            native.candidate(self.group, self.store, "not-a-snapshot", "delta", output)
        self.assertFalse(output.exists())
        entry = backup._load_manifest(self.store, self.snapshot)["entries"][0]
        backup._object_path(self.store, entry["sha256"]).write_bytes(b"corrupted backup")
        with self.assertRaises(backup.BackupError):
            native.candidate(self.group, self.store, self.snapshot, "delta", output)
        self.assertFalse(output.exists())

    def test_existing_archive_is_not_overwritten(self):
        archive = self.output / "existing.pack"
        archive.write_bytes(b"keep existing bytes")
        with self.assertRaisesRegex(ValueError, "replace"):
            native.candidate(self.group, self.store, self.snapshot, "delta", archive)
        self.assertEqual(archive.read_bytes(), b"keep existing bytes")

    def test_backup_reverified_at_publication_after_stage_must_fail_closed(self):
        before = self.originals()
        output = self.output / "refuse.pack"
        original_pack = pack.pack_proof

        def tampered(group, method, path, min_bytes, min_percent, checkpoint):
            def inject(phase, candidate):
                if phase == "verified":
                    entry = backup._load_manifest(self.store, self.snapshot)["entries"][0]
                    backup._object_path(self.store, entry["sha256"]).write_bytes(b"corrupt during pack")
                checkpoint(phase, candidate)
            return original_pack(group, method, path, min_bytes, min_percent, inject)

        with mock.patch.object(native.proof, "pack_proof", side_effect=tampered):
            with self.assertRaises(backup.BackupError):
                native.candidate(self.group, self.store, self.snapshot, "delta", output)
        self.assertFalse(output.exists())
        self.assertEqual(before, self.originals())

    def test_cli_reports_verified_backup_without_exposing_activation(self):
        spec = self.output / "catalogue-spec.json"
        spec.write_text(json.dumps({"stacks": [self.group]}))
        output = self.output / "cli.pack"
        command = [
            sys.executable, str(Path(native.__file__)),
            "--stacks", str(spec), "--group", "stack:9",
            "--store", str(self.store), "--snapshot", self.snapshot,
            "--method", "delta", "--output", str(output),
        ]
        done = subprocess.run(command, capture_output=True, text=True, check=True)
        report = json.loads(done.stdout)
        self.assertEqual(report["sourceFiles"], 2)
        self.assertTrue(report["backupVerified"])
        self.assertFalse(report["originalEvictionEnabled"])
        self.assertTrue(output.exists())
        retry = subprocess.run(command, capture_output=True, text=True)
        self.assertNotEqual(retry.returncode, 0)
        self.assertFalse(retry.stdout.strip())


if __name__ == "__main__":
    unittest.main()
