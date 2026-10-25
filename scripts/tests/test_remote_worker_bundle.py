"""Deterministic packaging and read-only remote worker capability smoke tests."""

from __future__ import annotations

import ast
from contextlib import redirect_stdout
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
from pathlib import Path
import sys
import tempfile
import threading
import unittest
from zipfile import ZipFile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import package_remote_worker as package
import remote_worker_check as checker


class BundleTests(unittest.TestCase):
    def test_archive_is_complete_deterministic_and_has_integrity_manifest(self):
        with tempfile.TemporaryDirectory() as directory:
            first = Path(directory) / "first.zip"
            second = Path(directory) / "second.zip"
            package.build_bundle(first)
            package.build_bundle(second)
            self.assertEqual(first.read_bytes(), second.read_bytes())
            with ZipFile(first) as bundle:
                names = [name.removeprefix(package.BUNDLE_ROOT + "/") for name in bundle.namelist()]
                expected = set(package.MODULES) | set(package.EXTRAS.values()) | {"manifest.json"}
                self.assertEqual(set(names), expected)
                self.assertTrue(all(name.startswith(package.BUNDLE_ROOT + "/") for name in bundle.namelist()))
                manifest = json.loads(bundle.read(package.BUNDLE_ROOT + "/manifest.json"))
                self.assertEqual(manifest["format"], 1)
                self.assertEqual(manifest["scripts"], list(package.MODULES))
                self.assertEqual(set(manifest["sha256"]), expected - {"manifest.json"})
                for filename, digest in manifest["sha256"].items():
                    self.assertEqual(
                        hashlib.sha256(bundle.read(package.BUNDLE_ROOT + "/" + filename)).hexdigest(),
                        digest,
                    )
                    self.assertNotIn("secret", filename.lower())
                for module in package.MODULES:
                    source = bundle.read(package.BUNDLE_ROOT + "/" + module)
                    compile(source, module, "exec")
                self.assertIn(b"onnxruntime", bundle.read(package.BUNDLE_ROOT + "/requirements.txt"))
                self.assertIn(b"remote_worker_check.py", bundle.read(package.BUNDLE_ROOT + "/README.md"))

    def test_all_local_worker_imports_and_api_families_are_bundled(self):
        included = set(package.MODULES)
        for module in package.MODULES:
            tree = ast.parse((package.SCRIPTS / module).read_text())
            for node in ast.walk(tree):
                if isinstance(node, ast.Import):
                    for alias in node.names:
                        root = alias.name.split(".")[0] + ".py"
                        if root.endswith("_worker.py"):
                            self.assertIn(root, included, f"{module} requires {root}")
                elif isinstance(node, ast.ImportFrom) and node.module:
                    root = node.module.split(".")[0] + ".py"
                    if root.endswith("_worker.py"):
                        self.assertIn(root, included, f"{module} requires {root}")
        server = (package.SCRIPTS / "visual_embedding_server.py").read_text()
        for _, endpoint, _ in checker.CHECKS:
            self.assertIn(endpoint, server)
        for endpoint in ("/v1/embed", "/v1/tag", "/v1/camie/tag",
                         "/v1/convert", "/v1/video-overlap/sample", "/v1/restoration"):
            self.assertIn(endpoint, server)


class CapabilityCheckTests(unittest.TestCase):
    def test_checks_all_families_with_bearer_auth_and_allows_missing_optional_camie(self):
        requests = []

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                requests.append((self.path, self.headers.get("Authorization")))
                if self.headers.get("Authorization") != "Bearer test-secret":
                    self.send_response(401)
                    self.end_headers()
                    return
                if self.path == "/v1/camie/status":
                    self.send_response(503)
                    self.end_headers()
                    return
                payload = {"ok": True}
                if self.path == "/v1/convert/capabilities":
                    payload = {"formats": [{"id": "jxl"}], "upscalers": []}
                elif self.path == "/v1/restoration/capabilities":
                    payload = {"available": False}
                elif self.path not in {"/v1/status", "/v1/video-overlap/capabilities"}:
                    self.send_response(404)
                    self.end_headers()
                    return
                body = json.dumps(payload).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, _format, *_args):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        base = f"http://127.0.0.1:{server.server_port}"
        output = io.StringIO()
        with redirect_stdout(output):
            self.assertEqual(checker.check(base, "test-secret"), 0)
        self.assertIn("OPTIONAL Camie", output.getvalue())
        self.assertIn("optional backend unavailable", output.getvalue())
        self.assertEqual([path for path, _ in requests], [ep for _, ep, _ in checker.CHECKS])
        self.assertTrue(all(token == "Bearer test-secret" for _, token in requests))
        self.assertNotIn("test-secret", output.getvalue())

        with redirect_stdout(io.StringIO()):
            self.assertEqual(checker.check(base, "wrong"), 1)
            self.assertEqual(checker.check("not-a-url", "test-secret"), 2)


if __name__ == "__main__":
    unittest.main()
