"""Safety tests for opt-in offline byte-exact P12/P13 publication and recovery."""
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
import image_storage_pack_proof as proof
import image_delta_storage as delta
import gallery_block_storage as blocks


class PackProofTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.media = root / "media"
        self.output = root / "output"
        self.media.mkdir()
        self.output.mkdir()
        pixels = np.random.default_rng(173).integers(
            0, 256, (112, 144, 4), dtype=np.uint8
        )
        self.first = self.media / "original.png"
        self.second = self.media / "variant.png"
        Image.fromarray(pixels).save(self.first)
        changed = pixels.copy()
        changed[19:25, 28:37, 0] ^= 9
        Image.fromarray(changed).save(self.second)
        self.group = {
            "id": "stack:12",
            "representative": "image:100",
            "members": [
                {"id": "image:100", "path": str(self.first)},
                {"id": "image:101", "path": str(self.second)},
            ],
        }

    def signatures(self):
        return {
            path.name: (hashlib.sha256(path.read_bytes()).hexdigest(),
                        path.stat().st_mtime_ns)
            for path in (self.first, self.second)
        }

    def test_both_methods_produce_byte_exact_archives_without_changing_sources(self):
        originals = self.signatures()
        for method in proof.METHODS:
            with self.subTest(method=method):
                archive = self.output / (method + ".pack")
                report = proof.pack_proof(self.group, method, archive, 0, 0)
                self.assertTrue(report["verified"])
                self.assertFalse(report["originalEvictionEnabled"])
                self.assertFalse(report["nativeServing"])
                self.assertFalse(report["backupVerified"])
                self.assertEqual(report["fidelity"], "byte-exact")
                self.assertEqual(report["sourceFiles"], 2)
                self.assertGreater(report["retainedOriginalsPlusArchiveBytes"],
                                   report["sourceBytes"])
                self.assertEqual(self.signatures(), originals)
                with proof.open_reader(method, archive) as reader:
                    self.assertTrue(reader.verify()["verified"])
                    for member in self.group["members"]:
                        self.assertEqual(
                            reader.reconstruct(member["id"]),
                            Path(member["path"]).read_bytes(),
                        )
                receipt_dir = self.output / (method + "-reconstructed")
                if method == "delta":
                    receipt = delta.extract(archive, receipt_dir)
                else:
                    receipt = blocks.extract(archive, receipt_dir)
                self.assertTrue(receipt["complete"])
                self.assertEqual(
                    {p["sourceSHA256"] for p in receipt["members"]},
                    {value[0] for value in originals.values()},
                )
                self.assertEqual(self.signatures(), originals)

    def test_interrupted_publication_can_resume_without_originals(self):
        originals = self.signatures()
        for method in proof.METHODS:
            with self.subTest(method=method):
                target = self.output / (method + "-resumed.pack")
                staged = []

                def interrupted(phase, candidate):
                    if phase == "verified":
                        staged.append(candidate)
                        raise InterruptedError("simulate crash before publication")

                with self.assertRaises(InterruptedError):
                    proof.pack_proof(self.group, method, target, 0, 0, interrupted)
                self.assertFalse(target.exists())
                self.assertEqual(self.signatures(), originals)
                self.assertEqual(len(staged), 1)
                self.assertTrue(staged[0].exists())
                # This proves the archive can be published without touching
                # the primary media; existing native media stays authoritative.
                result = proof.resume_pack(method, staged[0], target)
                self.assertTrue(result["verified"])
                self.assertTrue(target.is_file())
                self.assertEqual(self.signatures(), originals)

    def test_refuses_to_replace_existing_file_and_symlink_sources(self):
        target = self.output / "occupied.pack"
        target.write_bytes(b"keep this")
        with self.assertRaises(ValueError):
            proof.pack_proof(self.group, "delta", target)
        self.assertEqual(target.read_bytes(), b"keep this")
        link = self.media / "shortcut.png"
        link.symlink_to(self.first)
        modified = {
            **self.group,
            "members": [
                {**self.group["members"][0], "path": str(link)},
                self.group["members"][1],
            ],
        }
        with self.assertRaises(ValueError):
            proof.pack_proof(modified, "delta", self.output / "unsafe.pack")
        with self.assertRaises(ValueError):
            proof.pack_proof(self.group, "delta", self.media / "inside-library.pack")
        self.assertFalse((self.output / "unsafe.pack").exists())

    def test_source_change_before_publication_fails_closed(self):
        destination = self.output / "changed.pack"

        def mutate(phase, candidate):
            if phase == "verified":
                # Synthetic change: simulate another program modifying media.
                self.second.write_bytes(self.second.read_bytes() + b"changed")
        with self.assertRaisesRegex(ValueError, "source images changed"):
            proof.pack_proof(self.group, "delta", destination, 0, 0, mutate)
        self.assertFalse(destination.exists())

    def test_corrupt_staged_candidate_is_never_published(self):
        archive = self.output / "good.pack"
        proof.pack_proof(self.group, "delta", archive, 0, 0)
        corrupt = self.output / "corrupt.pack"
        corrupt.write_bytes(archive.read_bytes()[:-16])
        destination = self.output / "recovered.pack"
        with self.assertRaises(Exception):
            proof.resume_pack("delta", corrupt, destination)
        self.assertFalse(destination.exists())

    def test_cli_requires_explicit_group_and_never_overwrites(self):
        spec = self.output / "stacks.json"
        spec.write_text(json.dumps({"stacks": [self.group]}))
        archive = self.output / "cli.pack"
        cmd = [
            sys.executable, str(Path(proof.__file__)), "pack",
            "--stacks", str(spec), "--group", "stack:12",
            "--method", "delta", "--output", str(archive),
            "--min-bytes", "0", "--min-percent", "0",
        ]
        first = subprocess.run(cmd, capture_output=True, text=True, check=True)
        self.assertTrue(json.loads(first.stdout)["verified"])
        saved = archive.read_bytes()
        second = subprocess.run(cmd, capture_output=True, text=True)
        self.assertNotEqual(second.returncode, 0)
        self.assertEqual(archive.read_bytes(), saved)


if __name__ == "__main__":
    unittest.main()
