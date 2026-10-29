"""Missing-original recovery rehearsal with synthetic missing source files only."""
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
import image_storage_missing_original_rehearsal as orphan
import media_library_backup as backup


class MissingOriginalRehearsalTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name)
        self.media = self.root / "media"
        self.store = self.root / "backup"
        self.private = self.root / "candidate-private"
        for folder in (self.media, self.store, self.private):
            folder.mkdir()
        rgba = np.random.default_rng(509).integers(0, 256, (76, 97, 4), dtype=np.uint8)
        a, b = self.media / "first.png", self.media / "second.png"
        Image.fromarray(rgba).save(a)
        rgba[3:7, 9:14, 1] ^= 8
        Image.fromarray(rgba).save(b)
        self.originals = [a, b]
        self.member_ids = ("image:171", "image:172")
        self.group = {
            "id": "stack:17",
            "representative": "image:171",
            "members": [
                {"id": ident, "path": str(path)}
                for ident, path in zip(self.member_ids, self.originals)
            ],
        }
        self.snapshot = backup.backup(self.store, {"images": self.media})["snapshot"]
        self.before = {path.name: path.read_bytes() for path in self.originals}

    def candidate(self, method):
        cid = f"stack-17-{method}-" + "a" * 24
        archive = self.private / (cid + ".pack")
        pack = packing.pack_proof(self.group, method, archive, 0, 0)
        pack.update({
            "archivePath": str(archive),
            "group": "stack:17",
            "mediaBackupSnapshot": self.snapshot,
            "backupVerified": True,
            "nativePackingCandidateOnly": True,
            "nativeServingVerified": False,
            "databaseBackupVerified": False,
            "archiveActivated": False,
        })
        receipt = self.private / (cid + ".receipt.json")
        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
        receipt.write_text(json.dumps({
            "version": 1, "archiveSHA256": digest, "report": pack
        }))
        return receipt, archive

    def remove_only_synthetic_originals(self):
        for path in self.originals:
            path.unlink()
        self.assertTrue(all(not path.exists() for path in self.originals))

    def test_recovers_both_methods_even_when_originals_are_missing(self):
        originals = dict(self.before)
        receipts = [self.candidate(method)[0] for method in packing.METHODS]
        self.remove_only_synthetic_originals()
        for method, receipt in zip(packing.METHODS, receipts):
            with self.subTest(method=method):
                output = self.private / (method + "-orphan-rehearsal")
                report = orphan.rehearse(receipt, self.store, output)
                self.assertTrue(report["verified"])
                self.assertTrue(report["originalSourcesNotRead"])
                self.assertFalse(report["requiresOriginalSources"])
                self.assertTrue(report["backupContentDigestsCovered"])
                self.assertFalse(report["sourceToBackupPathMappingVerified"])
                self.assertFalse(report["databaseRecoveryVerified"])
                self.assertFalse(report["canActivate"])
                self.assertFalse(report["originalEvictionEnabled"])
                self.assertEqual(report["restoredFiles"], 2)
                self.assertEqual(sorted(path.suffix for path in output.glob("*.png")), [".png", ".png"])
                self.assertEqual(
                    set(report["restoredSHA256"].values()),
                    {hashlib.sha256(data).hexdigest() for data in originals.values()}
                )
                self.assertTrue(all(not path.exists() for path in self.originals))

    def test_same_size_archive_tamper_fails_without_output(self):
        receipt, archive = self.candidate("delta")
        data = bytearray(archive.read_bytes())
        data[len(data) // 2] ^= 1
        archive.write_bytes(data)
        self.remove_only_synthetic_originals()
        output = self.private / "reject"
        with self.assertRaisesRegex(ValueError, "hash or size"):
            orphan.rehearse(receipt, self.store, output)
        self.assertFalse(output.exists())

    def test_backup_object_corruption_refused_without_output(self):
        receipt, _ = self.candidate("shared-tiles")
        digest = hashlib.sha256(self.before["first.png"]).hexdigest()
        entry = backup._object_path(self.store, digest)
        entry.write_bytes(b"CORRUPTED")
        self.remove_only_synthetic_originals()
        output = self.private / "reject"
        with self.assertRaises(backup.BackupError):
            orphan.rehearse(receipt, self.store, output)
        self.assertFalse(output.exists())

    def test_absent_source_is_not_silently_restored_into_original_directory(self):
        receipt, _ = self.candidate("delta")
        self.remove_only_synthetic_originals()
        with self.assertRaisesRegex(ValueError, "direct child"):
            orphan.rehearse(receipt, self.store, self.media / "do-not-restore")
        with self.assertRaisesRegex(ValueError, "direct child"):
            orphan.rehearse(receipt, self.store, self.store / "do-not-restore")
        self.assertTrue(all(not path.exists() for path in self.originals))

    def test_symlink_receipt_and_existing_output_refused(self):
        receipt, _ = self.candidate("delta")
        output = self.private / "existing"
        output.mkdir()
        with self.assertRaises(ValueError):
            orphan.rehearse(receipt, self.store, output)
        alias = self.private / "alias.receipt.json"
        try:
            alias.symlink_to(receipt)
        except OSError as exc:
            self.skipTest(str(exc))
        with self.assertRaises(ValueError):
            orphan.rehearse(alias, self.store, self.private / "never-created")

    def test_missing_sources_cli_reports_strict_nonactivation(self):
        receipt, _ = self.candidate("delta")
        self.remove_only_synthetic_originals()
        output = self.private / "from-cli"
        command = [
            sys.executable, str(Path(orphan.__file__)),
            "--receipt", str(receipt), "--store", str(self.store),
            "--output", str(output),
        ]
        run = subprocess.run(command, check=True, capture_output=True, text=True)
        report = json.loads(run.stdout)
        self.assertFalse(report["canActivate"])
        self.assertTrue(report["archiveSHA256Verified"])
        self.assertTrue((output / "extraction.json").exists())
        again = subprocess.run(command, capture_output=True, text=True)
        self.assertNotEqual(again.returncode, 0)
        self.assertTrue(all(not path.exists() for path in self.originals))


if __name__ == "__main__":
    unittest.main()
