"""Synthetic, stdlib-only P12/P13 scratch recovery restart audit regressions."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_scratch_audit as audit


class ScratchAuditTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.cid = "stack-17-delta-" + "a" * 24
        self.receipt = self.root / (self.cid + ".receipt.json")
        self.output = self.root / ("stack-17-orphan-rehearsal-" + "b" * 24)
        self.original = {"image:171": b"first recovered image",
                         "image:172": b"second recovered image"}
        self.digest = {key: hashlib.sha256(value).hexdigest()
                       for key, value in self.original.items()}
        self.receipt.write_text(json.dumps({
            "version": 1,
            "report": {"group": "stack:17", "method": "delta",
                       "fidelity": "byte-exact", "verified": True,
                       "backupVerified": True, "sourceSHA256": self.digest},
        }))

    def complete(self):
        self.output.mkdir()
        members = []
        for identity, content in self.original.items():
            name = identity.replace(":", "-") + ".png"
            (self.output / name).write_bytes(content)
            members.append({"id": identity, "name": name,
                            "exportSHA256": self.digest[identity]})
        (self.output / "extraction.json").write_text(json.dumps({
            "version": 1, "complete": True, "fidelity": "byte-exact",
            "members": members,
        }))

    def test_missing_output_is_not_verified(self):
        result = audit.audit(self.receipt, self.output)
        self.assertEqual(result["state"], "no_output")
        self.assertFalse(result["canActivate"])
        self.assertFalse(self.output.exists())

    def test_verified_complete_scratch_is_still_not_live_recovery(self):
        self.complete()
        before = {x.name: x.read_bytes() for x in self.output.iterdir()}
        result = audit.audit(self.receipt, self.output)
        self.assertEqual(result["state"], "complete_verified")
        self.assertEqual(result["scratchFilesVerified"], 2)
        self.assertTrue(result["complete"])
        for key in ("canActivate", "originalEvictionEnabled",
                    "crashSafeLiveRollbackVerified", "databaseRecoveryVerified",
                    "nativeServingVerified", "candidateArchiveReverified",
                    "mediaBackupReverified"):
            self.assertFalse(result[key])
        self.assertEqual(before, {x.name: x.read_bytes() for x in self.output.iterdir()})

    def test_incomplete_directory_without_marker_never_certifies(self):
        self.output.mkdir()
        (self.output / "image-171.png").write_bytes(self.original["image:171"])
        result = audit.audit(self.receipt, self.output)
        self.assertEqual(result["state"], "incomplete")
        self.assertFalse(result["complete"])
        self.assertEqual(result["scratchFilesVerified"], 0)

    def test_modified_member_rejected(self):
        self.complete()
        (self.output / "image-171.png").write_bytes(b"corrupted")
        with self.assertRaisesRegex(ValueError, "SHA-256"):
            audit.audit(self.receipt, self.output)

    def test_missing_member_with_marker_rejected(self):
        self.complete()
        (self.output / "image-172.png").unlink()
        with self.assertRaisesRegex(ValueError, "missing"):
            audit.audit(self.receipt, self.output)

    def test_unexpected_file_rejected(self):
        self.complete()
        (self.output / "surprise.txt").write_text("do not trust")
        with self.assertRaises(ValueError):
            audit.audit(self.receipt, self.output)

    def test_symlink_in_scratch_refused(self):
        self.complete()
        (self.output / "image-172.png").unlink()
        (self.output / "image-172.png").symlink_to(self.receipt)
        with self.assertRaises(ValueError):
            audit.audit(self.receipt, self.output)

    def test_symlink_output_refused(self):
        other = self.root / "elsewhere"
        other.mkdir()
        self.output.symlink_to(other, target_is_directory=True)
        with self.assertRaises(ValueError):
            audit.audit(self.receipt, self.output)

    def test_traversal_name_and_candidate_mismatch_rejected(self):
        self.complete()
        path = self.output / "extraction.json"
        record = json.loads(path.read_text())
        record["members"][0]["name"] = "../outside.png"
        path.write_text(json.dumps(record))
        with self.assertRaises(ValueError):
            audit.audit(self.receipt, self.output)
        wrong = self.root / ("stack-18-orphan-rehearsal-" + "b" * 24)
        with self.assertRaises(ValueError):
            audit.audit(self.receipt, wrong)

    def test_duplicate_member_identity_rejected(self):
        self.complete()
        path = self.output / "extraction.json"
        record = json.loads(path.read_text())
        record["members"][1]["id"] = record["members"][0]["id"]
        path.write_text(json.dumps(record))
        with self.assertRaises(ValueError):
            audit.audit(self.receipt, self.output)

    def test_command_prints_non_activation_receipt(self):
        self.complete()
        proc = subprocess.run([sys.executable, str(Path(audit.__file__)),
                               "--receipt", str(self.receipt),
                               "--output", str(self.output)],
                              capture_output=True, text=True, check=True)
        report = json.loads(proc.stdout)
        self.assertEqual(report["state"], "complete_verified")
        self.assertFalse(report["originalEvictionEnabled"])


if __name__ == "__main__":
    unittest.main()
