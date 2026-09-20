"""Run with python3 -m unittest discover -s scripts/tests -p test_media_conversion.py."""
import contextlib
import http.client
import importlib
import json
import os
from pathlib import Path
import shutil
import sys
import struct
import tempfile
import threading
import time
import types
import unittest
import zlib
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import media_conversion_worker as converter


class OptionsTests(unittest.TestCase):
    def test_reject_invalid_controls(self):
        for value in ({"format": "sh"}, {"effort": 10}, {"effort": 2.5}, {"quality": True},
                      {"quality": float("nan")}, {"distance": -1}, {"hardware": "cuda;exit"}, {"command": "ls"}):
            with self.subTest(value=value), self.assertRaises(ValueError):
                converter.options(value)
        for value in ({"upscaler": "shell"}, {"upscaleScale": True}, {"upscaleScale": 3}):
            with self.assertRaises(ValueError):
                converter.options(value)

    def test_encoder_presence_is_not_gpu_support(self):
        with patch.object(converter, "run", side_effect=RuntimeError("no supported device")):
            self.assertFalse(converter.gpu_usable("av1_nvenc"))

    def test_quality_scale_and_legacy_distance(self):
        for quality, distance in ((100, 0), (90, 1), (80, 1.9), (30, 6.4), (0, 25)):
            with self.subTest(quality=quality):
                self.assertAlmostEqual(converter.options({"format": "jxl", "quality": quality})["distance"], distance)
        self.assertEqual(converter.options({"quality": 80, "distance": 0})["distance"], 0)
        self.assertEqual(converter.options({})["hardware"], "auto")


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

    def test_jxl_quality_100_preserves_pixels(self):
        if not (shutil.which("cjxl") and shutil.which("djxl")):
            self.skipTest("cjxl and djxl required")
        from PIL import Image
        source = Image.new("RGB", (128, 128))
        source.putdata([((x * 17 + y) % 256, (y * 29 + x) % 256, (x * y) % 256)
                        for y in range(128) for x in range(128)])
        source.save(self.root / "pixels.png")
        _, output = self.convert("pixels.png", "jxl", quality=100)
        decoded_dir = self.root / "decoded"
        decoded_dir.mkdir()
        decoded = converter.prepare_input(output, decoded_dir)
        with Image.open(decoded) as image:
            self.assertEqual(source.tobytes(), image.convert("RGB").tobytes())

    def test_fractional_apng_timing_does_not_accumulate_jxl_rounding(self):
        if not (shutil.which("cjxl") and shutil.which("djxl")):
            self.skipTest("cjxl and djxl required")
        from PIL import Image
        frames = [Image.new("RGB", (32, 32), ((i * 11) % 256, (i * 23) % 256, (i * 7) % 256)) for i in range(120)]
        source = self.root / "fractional.png"
        frames[0].save(source, save_all=True, append_images=frames[1:], duration=33, loop=0)
        raw = source.read_bytes()
        chunks, position = [raw[:8]], 8
        while position < len(raw):
            length = int.from_bytes(raw[position:position + 4], "big")
            chunk = bytearray(raw[position + 4:position + 8 + length])
            position += length + 12
            if chunk[:4] == b"fcTL":
                chunk[24:28] = struct.pack(">HH", 1, 30)
            chunks.append(struct.pack(">I", length) + chunk + struct.pack(">I", zlib.crc32(chunk)))
        source.write_bytes(b"".join(chunks))
        original = source.read_bytes()
        result, _ = self.convert(source.name, "ajxl", hardware="cpu")
        self.assertEqual(result["frames"], 120)
        self.assertAlmostEqual(result["duration"], 4, delta=0.001)
        self.assertEqual(source.read_bytes(), original)

    def test_video_duration_excludes_trailing_audio(self):
        source = self.root / "trailing-audio.mkv"
        converter.run(converter.ffmpeg_prefix() + ["-f", "lavfi", "-i", "testsrc2=size=128x128:rate=25:duration=0.6",
                      "-f", "lavfi", "-i", "sine=frequency=440:duration=1.2", "-c:v", "libx264", "-c:a", "pcm_s16le", str(source)])
        result, _ = self.convert(source.name, "av1-mp4", hardware="cpu")
        self.assertEqual(result["frames"], 15)
        self.assertEqual(result["audioStreams"], 1)
        self.assertAlmostEqual(result["duration"], 0.6, delta=0.002)

    def test_optional_upscaling_verifies_size_and_rejects_animation(self):
        from PIL import Image
        def enlarge(source, output, options, width, height, run, cancelled):
            with Image.open(source) as image:
                image.resize((width * 2, height * 2)).save(output)
        with patch.object(converter.upscaler, "upscale", side_effect=enlarge):
            result, _ = self.convert("still.png", "webp", upscaler="waifu2x", upscaleScale=2)
            self.assertEqual((result["width"], result["height"], result["frames"]), (256, 256, 1))
            self.assertEqual(result["upscaler"], "waifu2x")
            with self.assertRaisesRegex(ValueError, "still images only"):
                converter.convert(self.root / "animation.gif", self.root / "upscaled.gif", {"format": "gif", "upscaler": "seedvr2"})
            with self.assertRaisesRegex(RuntimeError, "expected"):
                converter.convert(self.root / "still.png", self.root / "wrong-size.png", {"format": "png", "upscaler": "waifu2x", "upscaleScale": 4})
            self.assertFalse((self.root / "wrong-size.png").exists())

    @unittest.skipUnless(os.name == "posix", "executable CLI fixture requires POSIX")
    def test_waifu_cli_cpu_fallback_and_codec_pipeline(self):
        executable = self.root / "waifu fixture"
        executable.write_text("#!/usr/bin/env python3\nimport sys\nfrom PIL import Image\na=sys.argv[1:]\n"
                              "if '-g' not in a: sys.exit(1)\n"
                              "assert a[a.index('-g')+1]=='-1'\n"
                              "im=Image.open(a[a.index('-i')+1]); s=int(a[a.index('-s')+1])\n"
                              "im.resize((im.width*s,im.height*s)).save(a[a.index('-o')+1])\n")
        executable.chmod(0o700)
        models = self.root / "fixture models"
        models.mkdir()
        (models / "fixture.param").write_text("test fixture, not model weights")
        (models / "fixture.bin").write_bytes(b"test fixture")
        with patch.dict(os.environ, {"STASH_WAIFU2X": str(executable), "STASH_WAIFU2X_MODELS": str(models)}):
            result, _ = self.convert("still.png", "png", upscaler="waifu2x", upscaleScale=2, hardware="auto")
            self.assertEqual((result["width"], result["height"]), (256, 256))

    def test_seed_cli_requires_installed_models_and_runs_offline(self):
        cli = self.root / "inference_cli.py"
        cli.write_text("# CLI fixture\n")
        models = self.root / "seed-models"
        models.mkdir()
        with patch.dict(os.environ, {"STASH_SEEDVR2_CLI": str(cli), "STASH_SEEDVR2_MODELS": str(models), "STASH_SEEDVR2_MODEL": "test.safetensors"}):
            opts = converter.options({"upscaler": "seedvr2", "upscaleScale": 2})
            with self.assertRaisesRegex(ValueError, "downloaded"):
                converter.upscaler.upscale(self.root / "still.png", self.root / "seed.png", opts, 128, 128, converter.run)
            for name in ("test.safetensors", "ema_vae_fp16.safetensors"):
                (models / name).write_bytes(b"test fixture")
            calls = []
            def record(args, cancelled, **kwargs):
                calls.append((args, kwargs))
            converter.upscaler.upscale(self.root / "still.png", self.root / "seed.png", opts, 128, 128, record)
            self.assertEqual(len(calls), 1)
            self.assertTrue(calls[0][1]["offline_models"])
            self.assertEqual(calls[0][0][calls[0][0].index("--resolution") + 1], "256")

    @unittest.skipUnless(os.name == "posix" and Path("/proc").exists(), "Linux process-tree assertion")
    def test_http_cancellation_stops_child_processes(self):
        pid_file = self.root / "child.pid"
        script = "import subprocess,sys,time; from pathlib import Path; p=subprocess.Popen([sys.executable,'-c','import time; time.sleep(30)']); Path(sys.argv[1]).write_text(str(p.pid)); time.sleep(30)"
        with self.assertRaisesRegex(RuntimeError, "cancelled"):
            converter.run([sys.executable, "-c", script, str(pid_file)], cancelled=pid_file.exists, timeout=5)
        pid = int(pid_file.read_text())
        status = Path(f"/proc/{pid}/stat")
        deadline = time.monotonic() + 2
        while status.exists() and status.read_text().split()[2] != "Z" and time.monotonic() < deadline:
            time.sleep(0.02)
        self.assertTrue(not status.exists() or status.read_text().split()[2] == "Z")

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

    def test_auto_uses_cpu_when_hardware_cannot_keep_bit_depth(self):
        source = self.root / "ten-bit.mkv"
        converter.run(converter.ffmpeg_prefix() + ["-f", "lavfi", "-i",
                      "testsrc2=size=128x128:rate=10:duration=0.6,format=yuv420p10le",
                      "-c:v", "ffv1", str(source)])
        caps = {"formats": [{"id": "h264", "cpu": ["libx264"], "gpu": ["h264_nvenc"]}]}
        with patch.object(converter, "capabilities", return_value=caps):
            result, _ = self.convert(source.name, "h264", hardware="auto")
            self.assertEqual(result["encoder"], "libx264")
            self.assertIn("10", result["pixelFormat"])
            with self.assertRaisesRegex(ValueError, "cannot preserve high bit depth"):
                converter.convert(source, self.root / "gpu.mp4", {"format": "h264", "hardware": "gpu"})

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
