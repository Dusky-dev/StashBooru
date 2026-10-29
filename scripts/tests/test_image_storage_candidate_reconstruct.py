"""Isolated recovery proof: originals and live catalogue are never touched."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import numpy as np
from PIL import Image

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_pack_proof as packing
import image_storage_candidate_reconstruct as recovery


class IsolatedReconstructionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.media = root / "media"
        self.output = root / "candidates"
        self.media.mkdir()
        self.output.mkdir()
        pixels = np.random.default_rng(17).integers(
            0, 256, (96, 120, 4), dtype=np.uint8
        )
        self.a = self.media / "first.png"
        self.b = self.media / "second.png"
        Image.fromarray(pixels).save(self.a)
        pixels[6:9, 11:18, 0] ^= 47
        Image.fromarray(pixels).save(self.b)
        self.group = {
            "id": "stack:17",
            "representative": "image:171",
            "members": [
                {"id": "image:171", "path": str(self.a)},
                {"id": "image:172", "path": str(self.b)},
            ],
        }

    def originals(self):
        return {
            path.name: (path.stat().st_mtime_ns,
                        hashlib.sha256(path.read_bytes()).hexdigest())
            for path in (self.a, self.b)
        }

    def evidence(self, method):
        archive = self.output / (method + ".pack")
        report = packing.pack_proof(self.group, method, archive, 0, 0)
        return {
            "archivePath": str(archive),
            "method": method,
            "group": self.group["id"],
            "mediaBackupSnapshot": "20261029T102030Z-012345abcdef",
            "sourceSHA256": report["sourceSHA256"],
        }

    def test_p12_p13_reconstruct_every_original_byte_in_new_folder(self):
        before = self.originals()
        for method in packing.METHODS:
            with self.subTest(method=method):
                evidence = self.evidence(method)
                output = self.output / (method + "-recovery")
                receipt = recovery.reconstruct(evidence, output)
                self.assertTrue(receipt["complete"])
                self.assertTrue(receipt["verified"])
                self.assertTrue(receipt["byteExact"])
                self.assertTrue(receipt["sourcesRetained"])
                self.assertFalse(receipt["originalEvictionEnabled"])
                self.assertFalse(receipt["archiveActivated"])
                self.assertFalse(receipt["databaseModified"])
                self.assertEqual(receipt["restoredSHA256"], evidence["sourceSHA256"])
                self.assertEqual(receipt["restoredFiles"], 2)
                self.assertTrue((output / "extraction.json").is_file())
                self.assertEqual(len(list(output.iterdir())), 3)
                self.assertEqual(before, self.originals())

    def test_wrong_source_digests_refused_without_writing(self):
        evidence = self.evidence("delta")
        evidence["sourceSHA256"]["image:171"] = "f" * 64
        output = self.output / "refused"
        with self.assertRaisesRegex(ValueError, "original image bytes"):
            recovery.reconstruct(evidence, output)
        self.assertFalse(output.exists())

    def test_missing_member_refused_without_writing(self):
        evidence = self.evidence("delta")
        evidence["sourceSHA256"] = {"image:171": evidence["sourceSHA256"]["image:171"]}
        output = self.output / "refused"
        with self.assertRaisesRegex(ValueError, "2–16"):
            recovery.reconstruct(evidence, output)
        self.assertFalse(output.exists())

    def test_corrupted_archive_never_creates_recovered_files(self):
        evidence = self.evidence("shared-tiles")
        archive = Path(evidence["archivePath"])
        archive.write_bytes(archive.read_bytes()[:-27])
        output = self.output / "corrupt-recovery"
        with self.assertRaises(Exception):
            recovery.reconstruct(evidence, output)
        self.assertFalse(output.exists())

    def test_refuses_existing_output_and_symlink_archive(self):
        evidence = self.evidence("delta")
        existing = self.output / "existing"
        existing.mkdir()
        with self.assertRaisesRegex(ValueError, "new absolute directory"):
            recovery.reconstruct(evidence, existing)
        self.assertTrue(existing.is_dir())
        symlink = self.output / "archive-alias.pack"
        try:
            symlink.symlink_to(evidence["archivePath"])
        except OSError as exc:
            self.skipTest(f"symlinks unavailable: {exc}")
        evidence["archivePath"] = str(symlink)
        with self.assertRaisesRegex(ValueError, "non-symlink"):
            recovery.reconstruct(evidence, self.output / "never-created")

    def test_cli_uses_private_spec_and_never_overwrites(self):
        before = self.originals()
        evidence = self.evidence("delta")
        spec = self.output / "proof.json"
        spec.write_text(json.dumps(evidence))
        restored = self.output / "cli-recovered"
        command = [
            sys.executable, str(Path(recovery.__file__)),
            "--evidence", str(spec), "--output", str(restored)
        ]
        result = subprocess.run(command, capture_output=True, text=True, check=True)
        self.assertTrue(json.loads(result.stdout)["verified"])
        attempt = subprocess.run(command, capture_output=True, text=True)
        self.assertNotEqual(attempt.returncode, 0)
        self.assertEqual(before, self.originals())


if __name__ == "__main__":
    unittest.main()
