"""Synthetic non-destructive whole-stack P12/P13 consumer-audit regressions."""
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
import image_storage_pack_proof as pack
import image_storage_shadow_audit as audit


class FullStackAuditTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        root = Path(self.tmp.name)
        self.media = root / "media"
        self.candidates = root / "candidates"
        self.media.mkdir()
        self.candidates.mkdir()
        rgba = np.random.default_rng(53).integers(0, 256, (86, 120, 4), dtype=np.uint8)
        first, second = self.media / "first.png", self.media / "second.png"
        Image.fromarray(rgba).save(first)
        rgba[8:10, 3:8, 1] ^= 42
        Image.fromarray(rgba).save(second)
        self.members = [
            {"id": "image:201", "path": str(first)},
            {"id": "image:202", "path": str(second)},
        ]
        self.group = {
            "id": "stack:20", "representative": "image:201",
            "members": self.members,
        }

    def spec(self, method):
        archive = self.candidates / (method + ".pack")
        report = pack.pack_proof(self.group, method, archive, 0, 0)
        return {
            "group": self.group["id"],
            "method": method,
            "archivePath": str(archive),
            "members": [dict(item) for item in self.members],
            "sourceSHA256": report["sourceSHA256"],
            "mediaBackupSnapshot": "20261029T102030Z-012345abcdef",
        }

    def originals(self):
        return [
            (Path(item["path"]).stat().st_mtime_ns,
             hashlib.sha256(Path(item["path"]).read_bytes()).hexdigest())
            for item in self.members
        ]

    def test_all_originals_match_both_byte_exact_methods(self):
        before = self.originals()
        for method in pack.METHODS:
            with self.subTest(method=method):
                report = audit.audit(self.spec(method))
                self.assertTrue(report["verified"])
                self.assertTrue(report["readOnly"])
                self.assertEqual(len(report["members"]), 2)
                self.assertTrue(all(item["byteExact"] for item in report["members"]))
                self.assertFalse(report["originalEvictionEnabled"])
                self.assertFalse(report["nativeServingActivated"])
                self.assertFalse(report["databaseModified"])
                self.assertEqual(before, self.originals())

    def test_changed_original_or_digest_fails_closed(self):
        spec = self.spec("delta")
        spec["sourceSHA256"]["image:202"] = "f" * 64
        with self.assertRaisesRegex(ValueError, "original differs"):
            audit.audit(spec)
        spec = self.spec("shared-tiles")
        Path(self.members[1]["path"]).write_bytes(b"MODIFIED")
        with self.assertRaises(ValueError):
            audit.audit(spec)

    def test_rejects_duplicate_member_or_missing_member(self):
        spec = self.spec("delta")
        spec["members"][1] = dict(spec["members"][0])
        with self.assertRaisesRegex(ValueError, "duplicate"):
            audit.audit(spec)
        spec = self.spec("shared-tiles")
        spec["sourceSHA256"].pop("image:202")
        with self.assertRaises(ValueError):
            audit.audit(spec)

    def test_corrupted_archive_and_symlink_are_refused(self):
        spec = self.spec("delta")
        alias = self.candidates / "alias.pack"
        try:
            alias.symlink_to(spec["archivePath"])
        except OSError as exc:
            self.skipTest(str(exc))
        spec["archivePath"] = str(alias)
        with self.assertRaisesRegex(ValueError, "non-symlink"):
            audit.audit(spec)
        spec = self.spec("shared-tiles")
        archive = Path(spec["archivePath"])
        archive.write_bytes(archive.read_bytes()[:-12])
        with self.assertRaises(Exception):
            audit.audit(spec)

    def test_cli_never_creates_extracted_files(self):
        spec = self.spec("delta")
        specfile = self.candidates / "input.json"
        specfile.write_text(json.dumps(spec))
        result = subprocess.run(
            [sys.executable, str(Path(audit.__file__)), "--spec", str(specfile)],
            capture_output=True, text=True, check=True,
        )
        self.assertTrue(json.loads(result.stdout)["verified"])
        self.assertEqual(
            sorted(path.name for path in self.candidates.iterdir()),
            ["delta.pack", "input.json"],
        )


if __name__ == "__main__":
    unittest.main()
