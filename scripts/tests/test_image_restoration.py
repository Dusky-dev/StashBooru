"""Real Pillow/HTTP checks; diffusion inference is a declared stub, never a GPU claim."""
import contextlib
import hashlib
import http.client
import importlib
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import threading
import types
import unittest
from unittest.mock import patch
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_restoration_worker as restoration
from PIL import Image


class RestorationTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.source = Image.new("RGBA", (64, 48), (17, 40, 90, 123))
        self.mask = Image.new("L", self.source.size)
        self.mask.paste(255, (24, 16, 40, 32))
        self.caps = {"available": True, "cpu": True, "gpu": False, "signature": "fixture",
                     "model": restoration.MODEL_ID, "revision": "a" * 40}

    def bundle(self, opts=None, reference=False):
        request = self.root / "input.zip"
        with zipfile.ZipFile(request, "w") as archive:
            archive.writestr("source", restoration.png_bytes(self.source))
            archive.writestr("mask.png", restoration.png_bytes(self.mask))
            archive.writestr("options.json", json.dumps(opts if opts is not None else {"operation": "prepare"}))
            if reference:
                archive.writestr("reference", restoration.png_bytes(Image.new("RGB", self.source.size, "green")))
        return request

    def test_missing_model_is_actionable_and_does_not_download(self):
        with patch.dict(os.environ, {"STASH_RESTORATION_MODEL_DIR": "", "STASH_RESTORATION_REVISION": ""}):
            caps = restoration.capabilities()
            self.assertFalse(caps["available"])
            self.assertIn("No automatic download", caps["notice"])
            with self.assertRaisesRegex(ValueError, "STASH_RESTORATION_MODEL_DIR"):
                restoration.inpaint(self.source, self.mask, None, restoration.options({}), None)

    def test_prepare_preserves_rgba_and_source_bytes(self):
        request = self.bundle()
        before = request.read_bytes()
        output = self.root / "result.zip"
        receipt = restoration.process(request, output)
        with zipfile.ZipFile(output) as archive:
            normalized = restoration.read_image(archive.read("source.png"))
            self.assertEqual(normalized.tobytes(), self.source.tobytes())
            self.assertEqual(hashlib.sha256(archive.read("source.png")).hexdigest(), receipt["outputSHA256"])
        self.assertEqual(request.read_bytes(), before)

    def test_cancelled_process_publishes_no_result(self):
        request = self.bundle()
        with self.assertRaisesRegex(RuntimeError, "cancelled"):
            restoration.process(request, self.root / "result.zip", lambda: True)
        self.assertFalse((self.root / "result.zip").exists())

    def test_animation_high_bit_depth_and_wrong_mask_rejected(self):
        stream = io.BytesIO()
        self.source.save(stream, "WEBP", save_all=True, append_images=[Image.new("RGBA", self.source.size, "red")], duration=100)
        with self.assertRaisesRegex(ValueError, "still"):
            restoration.read_image(stream.getvalue())
        stream = io.BytesIO()
        Image.new("I;16", (10, 10)).save(stream, "PNG")
        with self.assertRaises(ValueError):
            restoration.read_image(stream.getvalue())
        with self.assertRaisesRegex(ValueError, "grayscale"):
            restoration.read_image(restoration.png_bytes(self.source), mask=True)
        self.mask = Image.new("L", (1, 1), 255)
        with self.assertRaisesRegex(ValueError, "dimensions"):
            restoration.process(self.bundle({}), self.root / "result.zip")

    def test_archive_paths_and_duplicate_entries_rejected(self):
        request = self.root / "bad.zip"
        with zipfile.ZipFile(request, "w") as archive:
            archive.writestr("../escape", b"no")
        with self.assertRaisesRegex(ValueError, "entries"):
            restoration.process(request, self.root / "result.zip")
        self.assertFalse((self.root.parent / "escape").exists())

    def fake_pipeline(self, flagged=False):
        class Pipeline:
            unet = types.SimpleNamespace(config=types.SimpleNamespace(in_channels=9))
            safety_checker = object()
            calls = []

            @classmethod
            def from_pretrained(cls, path, **kwargs):
                cls.calls.append((path, kwargs))
                return cls()

            def to(self, device): pass
            def enable_attention_slicing(self): pass
            def enable_vae_slicing(self): pass
            def set_progress_bar_config(self, **kwargs): pass
            def __call__(self, **kwargs):
                Pipeline.calls.append(kwargs)
                kwargs["callback_on_step_end"](self, 0, 0, {})
                return types.SimpleNamespace(images=[Image.new("RGB", (512, 512), "red")], nsfw_content_detected=[flagged])

        class Generator:
            def __init__(self, **kwargs): pass
            def manual_seed(self, seed): return self

        torch = types.SimpleNamespace(float16="fp16", float32="fp32", Generator=Generator, inference_mode=contextlib.nullcontext)
        return Pipeline, {"torch": torch, "diffusers": types.SimpleNamespace(StableDiffusionInpaintPipeline=Pipeline)}

    def test_adapter_composites_exact_pixels_and_bounds_inference_with_reference(self):
        pipeline, modules = self.fake_pipeline()
        opts = restoration.options({"signature": "fixture", "strength": .75})
        with patch.dict(sys.modules, modules), patch.dict(os.environ, {"STASH_RESTORATION_MODEL_DIR": "/worker/model"}), patch.object(restoration, "capabilities", return_value=self.caps):
            output, receipt = restoration.inpaint(self.source, self.mask, Image.new("RGB", self.source.size, "green"), opts, None)
        self.assertEqual(output.size, self.source.size)
        for y in range(self.source.height):
            for x in range(self.source.width):
                self.assertEqual(output.getpixel((x, y))[3], 123)
                if self.mask.getpixel((x, y)) == 0:
                    self.assertEqual(output.getpixel((x, y)), self.source.getpixel((x, y)))
        self.assertEqual(output.getpixel((30, 24))[:3], (255, 0, 0))
        self.assertEqual(receipt["workSize"], [512, 512])
        self.assertTrue(pipeline.calls[0][1]["local_files_only"])
        self.assertTrue(pipeline.calls[0][1]["use_safetensors"])
        self.assertEqual(pipeline.calls[0][1]["variant"], "fp16")
        self.assertEqual(pipeline.calls[1]["image"].size, (512, 512))

    def test_gpu_only_cpu_disabled_model_change_and_blocked_output(self):
        _, modules = self.fake_pipeline(flagged=True)
        opts = restoration.options({"signature": "fixture"})
        with patch.dict(sys.modules, modules), patch.dict(os.environ, {"STASH_RESTORATION_MODEL_DIR": "/worker/model"}), patch.object(restoration, "capabilities", return_value=self.caps):
            with self.assertRaisesRegex(ValueError, "CUDA"):
                restoration.inpaint(self.source, self.mask, None, {**opts, "hardware": "gpu"}, None)
            with self.assertRaisesRegex(ValueError, "model/revision"):
                restoration.inpaint(self.source, self.mask, None, {**opts, "signature": "old"}, None)
            with self.assertRaisesRegex(ValueError, "blocked"):
                restoration.inpaint(self.source, self.mask, None, opts, None)
            with patch.object(restoration, "capabilities", return_value={**self.caps, "cpu": False}):
                with self.assertRaisesRegex(ValueError, "CPU execution is disabled"):
                    restoration.inpaint(self.source, self.mask, None, {**opts, "hardware": "cpu"}, None)

    def test_soft_mask_edge_blends_only_selected_pixels(self):
        _, modules = self.fake_pipeline()
        mask = Image.new("L", self.source.size)
        amounts = (0, 32, 64, 128, 192, 255)
        for x, amount in enumerate(amounts, start=24):
            mask.putpixel((x, 20), amount)
        with patch.dict(sys.modules, modules), patch.dict(os.environ, {"STASH_RESTORATION_MODEL_DIR": "/worker/model"}), patch.object(restoration, "capabilities", return_value=self.caps):
            output, _ = restoration.inpaint(self.source, mask, None, restoration.options({"signature": "fixture"}), None)
        for y in range(self.source.height):
            for x in range(self.source.width):
                amount = mask.getpixel((x, y))
                expected = tuple((new * amount + old * (255 - amount) + 127) // 255
                                 for new, old in zip((255, 0, 0), (17, 40, 90))) + (123,)
                self.assertEqual(output.getpixel((x, y)), expected)

    def test_authenticated_remote_prepare_and_upload_cleanup(self):
        stub = types.SimpleNamespace(_log=lambda message: None)
        with patch.dict(sys.modules, {"visual_embedding_worker": stub, "camie_tagger_worker": stub}):
            server_module = importlib.import_module("visual_embedding_server")
        upload_dir = self.root / "uploads"
        upload_dir.mkdir()
        with patch.object(server_module, "SERVER_TOKEN", "fixture-token"), patch.dict(os.environ, {"STASH_EMBEDDING_SERVER_TEMP_DIR": str(upload_dir)}):
            server = server_module.VisualEmbeddingHTTPServer(("127.0.0.1", 0), 1024 * 1024)
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            try:
                with contextlib.closing(http.client.HTTPConnection("127.0.0.1", server.server_port)) as client:
                    client.request("GET", "/v1/restoration/capabilities")
                    response = client.getresponse()
                    self.assertEqual(response.status, 401)
                    response.read()
                with contextlib.closing(http.client.HTTPConnection("127.0.0.1", server.server_port)) as client:
                    client.request("POST", "/v1/restoration", self.bundle().read_bytes(), {"Authorization": "Bearer fixture-token"})
                    response = client.getresponse()
                    self.assertEqual(response.status, 200)
                    data = response.read()
                    self.assertEqual(hashlib.sha256(data).hexdigest(), response.getheader("X-Stash-Content-SHA256"))
                with server_module._conversion_lock:
                    self.assertEqual(list(upload_dir.iterdir()), [])
                with server_module._inference_lock, contextlib.closing(http.client.HTTPConnection("127.0.0.1", server.server_port)) as client:
                    client.request("POST", "/v1/restoration", self.bundle().read_bytes(), {"Authorization": "Bearer fixture-token"})
                    response = client.getresponse()
                    self.assertEqual(response.status, 503)
                    response.read()
            finally:
                server.shutdown()
                server.server_close()


if __name__ == "__main__":
    unittest.main()
