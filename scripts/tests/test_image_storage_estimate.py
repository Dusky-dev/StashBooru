"""Read-only P12/P13 image storage candidate accounting."""
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
import image_storage_estimate as estimate


class StorageEstimatorTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        rng = np.random.default_rng(91)
        pixels = rng.integers(0, 256, (96, 96, 4), dtype=np.uint8)
        self.first = self.root / "original.png"
        self.second = self.root / "variant.png"
        Image.fromarray(pixels).save(self.first)
        pixels[20:28, 30:40, 0] ^= 13
        Image.fromarray(pixels).save(self.second)
        self.group = {
            "id": "stack:7",
            "representative": "image:1",
            "members": [{"id": "image:1", "path": str(self.first)},
                        {"id": "image:2", "path": str(self.second)}],
        }

    def originals(self):
        return {
            file.name: (hashlib.sha256(file.read_bytes()).hexdigest(),
                        file.stat().st_mtime_ns)
            for file in (self.first, self.second)
        }

    def test_byte_exact_comparison_is_non_destructive(self):
        original = self.originals()
        report = estimate.estimate([self.group], minimum_bytes=0, minimum_percent=0)
        self.assertTrue(report["readOnly"])
        self.assertTrue(report["sourcesRetained"])
        self.assertEqual(report["groupsChecked"], 1)
        self.assertEqual(report["groupsFailed"], 0)
        row = report["results"][0]
        self.assertEqual(row["fidelity"], "byte-exact")
        self.assertEqual(len(row["methods"]), 2)
        self.assertTrue(all(v["verifiedDuringPlanning"] for v in row["methods"]))
        self.assertEqual(report["potentialSavedBytes"],
                         row["potentialSavedBytes"])
        self.assertEqual(
            report["hypotheticalBytesAfterFullActivation"],
            report["sourceBytes"] - report["potentialSavedBytes"])
        self.assertEqual(self.originals(), original)
        self.assertEqual(sorted(p.name for p in self.root.iterdir()),
                         ["original.png", "variant.png"])

    def test_non_selected_method_is_not_counted_as_savings(self):
        group = {"id": "broken", "members": [
            {"id": "image:1", "path": "/a/no-such-file.png"}]}
        result = estimate.estimate([group])
        self.assertEqual(result["groupsFailed"], 1)
        self.assertEqual(result["potentialSavedBytes"], 0)
        self.assertIn("error", result["results"][0])

    def test_no_packs_created_even_for_pixel_exact_mode(self):
        original = self.originals()
        report = estimate.estimate([self.group], "pixel-exact", 0, 0)
        self.assertEqual(report["fidelity"], "pixel-exact")
        self.assertEqual(report["groupsFailed"], 0)
        self.assertEqual(self.originals(), original)

    def test_validation_fails_on_invalid_threshold(self):
        with self.assertRaises(ValueError):
            estimate.estimate([self.group], minimum_percent=float("nan"))
        with self.assertRaises(ValueError):
            estimate.estimate([self.group], fidelity="unsafe")

    def test_cli_report_never_overwrites(self):
        spec = self.root / "stacks.json"
        spec.write_text(json.dumps({"stacks": [self.group]}))
        output = self.root / "report.json"
        script = Path(estimate.__file__)
        cmd = [sys.executable, str(script), "--stacks", str(spec),
               "--output", str(output), "--min-bytes", "0",
               "--min-percent", "0"]
        first = subprocess.run(cmd, capture_output=True, text=True, check=True)
        result = json.loads(first.stdout)
        self.assertEqual(result["groupsFailed"], 0)
        self.assertEqual(result["fidelity"], "byte-exact")
        self.assertEqual(json.loads(output.read_text()), result)
        output.write_text("do not replace")
        second = subprocess.run(cmd, capture_output=True, text=True)
        self.assertNotEqual(second.returncode, 0)
        self.assertEqual(output.read_text(), "do not replace")


if __name__ == "__main__":
    unittest.main()
