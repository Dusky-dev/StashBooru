"""Byte-exact offline shadow serving never substitutes native image files."""
import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import numpy as np
from PIL import Image

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_pack_proof as proof
import image_storage_shadow_member as shadow


class ShadowImageProofTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        pixels = np.random.default_rng(51).integers(0, 256, (80, 104, 4), dtype=np.uint8)
        self.a = root / "one.png"
        self.b = root / "two.png"
        Image.fromarray(pixels).save(self.a)
        pixels[10:16, 4:9, 2] ^= 25
        Image.fromarray(pixels).save(self.b)
        self.group = {
            "id": "stack:51",
            "representative": "image:511",
            "members": [
                {"id": "image:511", "path": str(self.a)},
                {"id": "image:512", "path": str(self.b)},
            ],
        }

    def candidate(self, method):
        archive = self.a.parent / (method + ".pack")
        report = proof.pack_proof(self.group, method, archive, 0, 0)
        return archive, report

    def test_shadow_member_byte_exact_p12_and_p13(self):
        originals = {f.name: (f.stat().st_mtime_ns, f.read_bytes()) for f in (self.a, self.b)}
        for method in proof.METHODS:
            with self.subTest(method=method):
                archive, report = self.candidate(method)
                for member in self.group["members"]:
                    original = Path(member["path"])
                    rebuilt = shadow.shadow_member(
                        archive, method, member["id"],
                        report["sourceSHA256"][member["id"]], original
                    )
                    self.assertEqual(rebuilt, original.read_bytes())
                    self.assertEqual(hashlib.sha256(rebuilt).hexdigest(),
                                     report["sourceSHA256"][member["id"]])
        self.assertEqual(originals, {
            f.name: (f.stat().st_mtime_ns, f.read_bytes()) for f in (self.a, self.b)
        })

    def test_shadow_refuses_modified_source_and_wrong_sha(self):
        archive, report = self.candidate("delta")
        with self.assertRaisesRegex(ValueError, "current native original"):
            shadow.shadow_member(archive, "delta", "image:511", "f" * 64, self.a)
        self.a.write_bytes(b"changed!")
        with self.assertRaises(ValueError):
            shadow.shadow_member(archive, "delta", "image:511",
                                 report["sourceSHA256"]["image:511"], self.a)

    def test_shadow_refuses_wrong_identity_and_corrupt_archive(self):
        archive, report = self.candidate("shared-tiles")
        with self.assertRaises(ValueError):
            shadow.shadow_member(archive, "shared-tiles", "image:999",
                                 report["sourceSHA256"]["image:511"], self.a)
        archive.write_bytes(archive.read_bytes()[:-11])
        with self.assertRaises(Exception):
            shadow.shadow_member(archive, "shared-tiles", "image:511",
                                 report["sourceSHA256"]["image:511"], self.a)

    def test_shadow_requires_regular_paths_and_absolute_original(self):
        archive, report = self.candidate("delta")
        alias = archive.with_name("alias.pack")
        try:
            alias.symlink_to(archive)
        except OSError as exc:
            self.skipTest(str(exc))
        with self.assertRaisesRegex(ValueError, "non-symlink"):
            shadow.shadow_member(alias, "delta", "image:511",
                                 report["sourceSHA256"]["image:511"], self.a)
        with self.assertRaisesRegex(ValueError, "absolute"):
            shadow.shadow_member(archive, "delta", "image:511",
                                 report["sourceSHA256"]["image:511"], "relative.png")

    def test_cli_binary_output_has_no_json_or_logs(self):
        archive, report = self.candidate("delta")
        cmd = [
            sys.executable, str(Path(shadow.__file__)),
            "--archive", str(archive), "--method", "delta",
            "--member", "image:511",
            "--sha256", report["sourceSHA256"]["image:511"],
            "--original", str(self.a),
        ]
        good = subprocess.run(cmd, capture_output=True, check=True)
        self.assertEqual(good.stdout, self.a.read_bytes())
        self.assertFalse(good.stderr)
        bad = subprocess.run(cmd[:-2] + ["--original", str(self.b)], capture_output=True)
        self.assertNotEqual(bad.returncode, 0)
        self.assertFalse(bad.stdout)


if __name__ == "__main__":
    unittest.main()
