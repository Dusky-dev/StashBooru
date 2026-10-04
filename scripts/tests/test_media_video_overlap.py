"""Bounded worker, PTS/VFR, rotation, upload, and cleanup regression coverage."""

from pathlib import Path
import http.client
import importlib
import json
import os
import shutil
import sys
import tempfile
import threading
import types
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import video_overlap_worker as worker
from video_overlap_fixtures import encode, transcode


class VideoOverlapOptions(unittest.TestCase):
    def test_explicit_bounds_and_version_identity(self):
        for value in [0, .1, 11, float("inf"), float("nan")]:
            with self.assertRaises(ValueError):
                worker.options({"sampleSeconds": value})
        with self.assertRaises(ValueError):
            worker.options({"audioDigest": "yes"})
        with patch.object(worker, "run", return_value=(b"codec version one", b"")):
            first = worker.capabilities()
        with patch.object(worker, "run", return_value=(b"codec version two", b"")):
            self.assertNotEqual(first["signature"], worker.capabilities()["signature"])
        self.assertEqual(worker.ALGORITHM, first["algorithm"])


@unittest.skipUnless(shutil.which("ffmpeg") and shutil.which("ffprobe"), "FFmpeg/ffprobe unavailable")
class VideoOverlapCodecs(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix="p10-python-")
        cls.root = Path(cls.temp.name)
        cls.source = cls.root / "base.mp4"
        encode(cls.source, list(range(30, 38)), audio=440)

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def test_real_samples_digest_optional_audio_and_identity(self):
        result = worker.sample(self.source, {"audioDigest": True})
        self.assertEqual(8, len(result["frames"]))
        self.assertEqual(list(range(8)), [f["time"] for f in result["frames"]])
        self.assertEqual(worker.file_hash(self.source), result["sha256"])
        self.assertEqual(worker.capabilities()["signature"], result["decoder"])
        self.assertEqual(1, result["media"]["audioTracks"])
        self.assertEqual(64, len(result["media"]["audioSHA256"]))
        self.assertNotIn("audioSHA256", worker.sample(self.source, {})["media"])

    def test_vfr_retains_existing_presentation_times(self):
        target = self.root / "vfr.mp4"
        transcode(self.source, target, "-vf", "select='not(eq(mod(n,5),0))'", "-fps_mode", "vfr", "-c:a", "copy")
        result = worker.sample(target, {})
        times = [f["time"] for f in result["frames"]]
        self.assertGreater(times[0], 0, "the absent zero-time frame must not be invented")
        self.assertTrue(all(a < b for a, b in zip(times, times[1:])))
        self.assertTrue(any(abs(t - round(t)) > .01 for t in times))

    def test_rotation_is_displayed_and_cancellation_cleans_spools(self):
        import subprocess
        rotated = self.root / "rotated.mp4"
        subprocess.run(["ffmpeg", "-v", "error", "-y", "-display_rotation:v:0", "90",
                        "-i", str(self.source), "-c", "copy", str(rotated)], check=True)
        result = worker.sample(rotated, {})
        self.assertEqual((96, 128), (result["media"]["width"], result["media"]["height"]))
        self.assertEqual(90, abs(result["media"]["rotation"]))
        before = set(self.root.iterdir())
        with self.assertRaisesRegex(RuntimeError, "cancelled"):
            worker.sample(self.source, {}, lambda: True)
        self.assertEqual(before, set(self.root.iterdir()))

    def test_remote_auth_sampling_busy_and_upload_cleanup(self):
        fake = types.SimpleNamespace(_log=lambda _: None, ort=types.SimpleNamespace(get_available_providers=lambda: ["CPUExecutionProvider"]))
        with patch.dict(sys.modules, {"visual_embedding_worker": fake, "camie_tagger_worker": None}):
            sys.modules.pop("visual_embedding_server", None)
            server_module = importlib.import_module("visual_embedding_server")
        with patch.object(server_module, "SERVER_TOKEN", "fixture-token"), patch.dict(os.environ, {"STASH_EMBEDDING_SERVER_TEMP_DIR": str(self.root)}):
            server = server_module.VisualEmbeddingHTTPServer(("127.0.0.1", 0), 16 * 1024 * 1024)
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            def request(path, body=None, auth=True):
                connection = http.client.HTTPConnection("127.0.0.1", server.server_port, timeout=30)
                headers = {"Authorization": "Bearer fixture-token"} if auth else {}
                if body is not None:
                    headers["X-Stash-Video-Options"] = json.dumps({"audioDigest": True})
                connection.request("GET" if body is None else "POST", path, body, headers)
                response = connection.getresponse()
                status, data = response.status, response.read()
                connection.close()
                return status, json.loads(data)
            try:
                self.assertEqual(401, request("/v1/video-overlap/capabilities", auth=False)[0])
                self.assertEqual(worker.ALGORITHM, request("/v1/video-overlap/capabilities")[1]["algorithm"])
                before = set(self.root.iterdir())
                status, sampled = request("/v1/video-overlap/sample", self.source.read_bytes())
                self.assertEqual(200, status)
                self.assertEqual(worker.file_hash(self.source), sampled["sha256"])
                self.assertEqual(8, len(sampled["frames"]))
                self.assertEqual(before, set(self.root.iterdir()))
                with server_module._conversion_lock:
                    self.assertEqual(503, request("/v1/video-overlap/sample", self.source.read_bytes())[0])
            finally:
                server.shutdown();server.server_close();thread.join(timeout=5)
                sys.modules.pop("visual_embedding_server", None)
