"""Exercise publication guards without contacting GitHub or publishing anything."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "release-stashbooru.sh"
ASSETS = ["Stash.app.zip", "stash-macos", "stash-win.exe", "stash-linux", "stash-linux-arm64v8", "stash-linux-arm32v7", "stash-linux-arm32v6", "stash-freebsd", "stash-ui.zip"]


class ReleaseGuards(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for folder in ["bin", "internal/build", "docs/releases", "dist"]:
            (self.root / folder).mkdir(parents=True)
        (self.root / "internal/build/stashbooru-version.txt").write_text("1.0.0\n")
        (self.root / "docs/releases/stashbooru-1.0.0.md").write_text("Release notes\n")
        (self.root / "CHECKSUMS_SHA1").write_text("legacy checksum fixture\n")
        for asset in ASSETS:
            (self.root / "dist" / asset).write_text("binary fixture " + asset)
        for name, body in {
            "git": '#!/bin/bash\nif [[ "$1" == "rev-parse" ]]; then echo abc; else exit "$TAG_STATUS"; fi\n',
            "gh": '#!/usr/bin/env python3\nimport os,sys,json\nwith open("calls", "a") as f: f.write(json.dumps(sys.argv[1:])+"\\n")\nif os.environ.get("UPLOAD_FAIL") and sys.argv[2] == "create": sys.exit(1)\n',
        }.items():
            path = self.root / "bin" / name
            path.write_text(body)
            path.chmod(0o755)
        self.env = dict(os.environ, PATH=str(self.root / "bin") + ":" + os.environ["PATH"], GITHUB_REPOSITORY="Dusky-dev/StashBooru", GITHUB_REF="refs/heads/releases/stashbooru-1.0.0", GITHUB_SHA="abc", TAG_STATUS="2")

    def run_release(self):
        return subprocess.run(["bash", str(SCRIPT)], cwd=self.root, env=self.env, capture_output=True)

    def calls(self):
        path = self.root / "calls"
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def test_complete_assets_are_drafted_before_publication(self):
        result = self.run_release()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        self.assertEqual([call[:2] for call in calls], [["release", "create"], ["release", "edit"]])
        self.assertIn("--draft", calls[0])
        self.assertIn("--draft=false", calls[1])
        self.assertIn("--latest", calls[1])
        self.assertEqual(len((self.root / "CHECKSUMS_SHA256").read_text().splitlines()), len(ASSETS))
        self.assertEqual(subprocess.run(["sha256sum", "-c", "../CHECKSUMS_SHA256"], cwd=self.root / "dist", capture_output=True).returncode, 0)

    def test_existing_tag_and_network_failure_stop_publication(self):
        for status in ["0", "128"]:
            self.env["TAG_STATUS"] = status
            self.assertNotEqual(self.run_release().returncode, 0)
            self.assertEqual(self.calls(), [])

    def test_wrong_branch_or_repository_stops_publication(self):
        for key, value in [("GITHUB_REF", "refs/heads/develop"), ("GITHUB_REPOSITORY", "stashapp/stash"), ("GITHUB_SHA", "wrong")]:
            old = self.env[key]
            self.env[key] = value
            self.assertNotEqual(self.run_release().returncode, 0)
            self.assertEqual(self.calls(), [])
            self.env[key] = old

    def test_missing_asset_stops_publication(self):
        (self.root / "dist/stash-linux").unlink()
        self.assertNotEqual(self.run_release().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_upload_failure_never_publishes_draft(self):
        self.env["UPLOAD_FAIL"] = "1"
        self.assertNotEqual(self.run_release().returncode, 0)
        self.assertEqual(len(self.calls()), 1)
        self.assertEqual(self.calls()[0][:2], ["release", "create"])


if __name__ == "__main__":
    unittest.main()
