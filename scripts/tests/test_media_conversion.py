"""Run with python3 -m unittest discover -s scripts/tests -p test_media_conversion.py."""
import contextlib
import http.client
import importlib
import json
from pathlib import Path
import shutil
import sys
import tempfile
import threading
import types
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import media_conversion_worker as converter


class OptionsTests(unittest.TestCase):
    def test_reject_invalid_controls(self):
        for value in ({"format": "sh"}, {"effort": 10}, {"effort": 2.5}, {"quality": True},
                      {"quality": float("nan")}, {"distance": -1}, {"hardware": "cuda;exit"}, {"command": "ls"}):
            with self.subTest(value=value), self.assertRaises(ValueError):
                converter.options(value)

    def test_encoder_presence_is_not_gpu_support(self):
        with patch.object(converter, "run", side_effect=RuntimeError("no supported device")):
            self.assertFalse(converter.gpu_usable("av1_nvenc"))


@unittest.skipUnless(shutil.which("ffmpeg") and shutil.which("ffprobe"), "FFmpeg required")
class EncodeTests(unittest.TestCase):
    def setUp(self):
        from PIL import Image
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.frames = [Image.new("RGB", (128, 128), color) for color in ("red", "green", "blue")]
        self.frames[0].save(self.root / "still.png")
        self.frames[0].save(self.root / "animation.gif", save_all=True, append_images=self.frames[1:],
                            duration=[100, 200, 300], loop=0)

    def convert(self, source, fmt, **options):
        output = self.root / ("out-" + fmt + "." + converter.FORMATS[fmt][1])
        result = converter.convert(self.root / source, output, {"format": fmt, "effort": 3, **options})
        self.assertTrue(output.is_file())
        self.assertGreater(result["size"], 0)
        return result, output

    def test_mainstream_still_outputs(self):
        caps = converter.capabilities(probe_gpu=False)
        for fmt in ("jxl", "jpeg", "png", "webp", "avif", "tiff", "bmp"):
            if not next(f for f in caps["formats"] if f["id"] == fmt)["available"]:
                continue
            with self.subTest(format=fmt):
                result, _ = self.convert("still.png", fmt)
                self.assertEqual((result["width"], result["height"], result["frames"]), (128, 128, 1))

    def test_variable_frame_delays(self):
        for fmt in ("gif", "apng", "webp", "av1-mp4", "av1-mkv", "av1-webm", "h264", "hevc", "vp9", "mov"):
            with self.subTest(format=fmt):
                result, _ = self.convert("animation.gif", fmt)
                self.assertEqual(result["frames"], 3)
                self.assertAlmostEqual(result["duration"], 0.6, delta=0.025)

    def test_animated_jxl_round_trip(self):
        caps = converter.capabilities(probe_gpu=False)
        if not next(f for f in caps["formats"] if f["id"] == "ajxl")["available"]:
            self.skipTest("cjxl and djxl required")
        result, output = self.convert("animation.gif", "ajxl")
        self.assertEqual(result["frames"], 3)
        self.assertAlmostEqual(result["duration"], 0.6)
        result, _ = self.convert(output.name, "apng")
        self.assertEqual(result["frames"], 3)

    def test_finite_loop_count_is_preserved_or_output_rejected(self):
        from PIL import Image
        self.frames[0].save(self.root / "finite.gif", save_all=True, append_images=self.frames[1:],
                            duration=[100, 200, 300], loop=2)
        for fmt in ("gif", "apng", "webp"):
            with self.subTest(format=fmt):
                _, output = self.convert("finite.gif", fmt)
                with Image.open(output) as image:
                    self.assertEqual(image.info["loop"], 2 if fmt == "gif" else 3)
        if shutil.which("cjxl") and shutil.which("djxl"):
            try:
                self.convert("finite.gif", "ajxl")
            except RuntimeError as error:
                self.assertIn("loop count changed", str(error))
                self.assertFalse((self.root / "out-ajxl.jxl").exists())

    def test_refuses_to_flatten_animation_or_overwrite(self):
        original = (self.root / "animation.gif").read_bytes()
        with self.assertRaisesRegex(ValueError, "discard frames"):
            self.convert("animation.gif", "jpeg")
        self.assertEqual((self.root / "animation.gif").read_bytes(), original)
        with self.assertRaisesRegex(ValueError, "already exists"):
            converter.convert(self.root / "animation.gif", self.root / "still.png", {"format": "png"})

    def test_audio_preservation_and_explicit_drop(self):
        source = self.root / "audio.mp4"
        converter.run(converter.ffmpeg_prefix() + ["-f", "lavfi", "-i", "testsrc2=size=128x128:rate=10:duration=0.6",
                      "-f", "lavfi", "-i", "sine=frequency=400:duration=0.6", "-c:v", "libx264", "-c:a", "aac", str(source)])
        result, _ = self.convert(source.name, "av1-mp4")
        self.assertEqual(result["audioStreams"], 1)
        with self.assertRaisesRegex(ValueError, "cannot contain audio"):
            self.convert(source.name, "apng")

    def test_no_network_playlists(self):
        source = self.root / "playlist.m3u8"
        source.write_text("#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\nhttp://127.0.0.1/should-not-be-read\n#EXT-X-ENDLIST\n")
        with self.assertRaises(RuntimeError):
            self.convert(source.name, "h264")

    def test_remote_uses_existing_auth_and_streams_verified_output(self):
        # ML dependencies are irrelevant to the converter HTTP contract.
        stub = types.SimpleNamespace(_log=lambda message: None)
        with patch.dict(sys.modules, {"visual_embedding_worker": stub, "camie_tagger_worker": stub}):
            server_module = importlib.import_module("visual_embedding_server")
        with patch.object(server_module, "SERVER_TOKEN", "test-secret"):
            server = server_module.VisualEmbeddingHTTPServer(("127.0.0.1", 0), 1024 * 1024)
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            self.addCleanup(server.server_close)
            self.addCleanup(server.shutdown)
            with contextlib.closing(http.client.HTTPConnection("127.0.0.1", server.server_port)) as client:
                client.request("GET", "/v1/convert/capabilities")
                response = client.getresponse()
                self.assertEqual(response.status, 401)
                response.read()
            with contextlib.closing(http.client.HTTPConnection("127.0.0.1", server.server_port)) as client:
                client.request("POST", "/v1/convert", (self.root / "still.png").read_bytes(), {
                    "Authorization": "Bearer test-secret", "X-Stash-Conversion-Options": json.dumps({"format": "webp"})})
                response = client.getresponse()
                self.assertEqual(response.status, 200)
                self.assertTrue(response.getheader("X-Stash-Content-MD5"))
                self.assertEqual(len(response.read()), int(response.getheader("Content-Length")))


if __name__ == "__main__":
    unittest.main()
