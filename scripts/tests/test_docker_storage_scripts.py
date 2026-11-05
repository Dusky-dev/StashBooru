"""Regression: production Docker includes the Python storage task bundle.

These assertions check package wiring without needing Docker privileges on CI.
The estimate execution tests remain in test_image_storage_estimate.py.
"""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
DOCKERFILE = ROOT / "docker" / "build" / "x86_64" / "Dockerfile"
SCRIPTS = ROOT / "scripts"
RUNTIME_ENTRY_POINTS = (
    "image_storage_estimate.py",
    "image_delta_storage.py",
    "gallery_block_storage.py",
    "media_library_backup.py",
    "image_storage_native_candidate.py",
    "image_storage_pack_proof.py",
    "image_storage_shadow_member.py",
    "image_storage_candidate_reconstruct.py",
)


class DockerStorageScriptsTests(unittest.TestCase):
    def test_runtime_image_copies_complete_python_storage_tree(self):
        dockerfile = DOCKERFILE.read_text(encoding="utf-8")
        self.assertRegex(dockerfile, r"(?m)^FROM alpine:latest$")
        self.assertRegex(dockerfile, r"(?m)^WORKDIR /$")
        self.assertRegex(
            dockerfile, r"(?m)^COPY --from=backend /stash/scripts /scripts$",
        )
        self.assertRegex(
            dockerfile,
            r"(?m)^ENV STASH_IMAGE_STORAGE_ESTIMATE_SCRIPT=/scripts/image_storage_estimate\.py$",
        )
        for dep in ("python3", "py3-numpy", "py3-pillow"):
            self.assertRegex(dockerfile, rf"(?m)^    {re.escape(dep)} \\?$")
        for script in RUNTIME_ENTRY_POINTS:
            path = SCRIPTS / script
            self.assertTrue(path.is_file(), f"missing runtime script {script}")
            compile(path.read_bytes(), str(path), "exec")


if __name__ == "__main__":
    unittest.main()
