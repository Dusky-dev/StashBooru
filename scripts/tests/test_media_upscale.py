"""Regression tests for optional image-upscaler hardware selection."""
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import sys
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import media_upscale_worker as upscaler


class Waifu2xHardwareTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.models = self.root / "models-cunet"
        self.models.mkdir()
        self.configuration = {
            "waifu": "/usr/bin/waifu2x-ncnn-vulkan",
            "waifu_models": self.models,
            "seed_cli": Path(),
            "seed_models": Path(),
            "seed_model": "seed.safetensors",
            "seed_python": sys.executable,
        }
        self.source = self.root / "input.png"
        self.output = self.root / "output.png"
        self.options = {"upscaler": "waifu2x", "upscaleScale": 2, "hardware": "auto"}

    def call(self, run, options=None):
        with patch.object(upscaler, "configuration", return_value=self.configuration), \
             patch.object(upscaler, "capabilities", return_value=[{
                 "id": "waifu2x", "available": True, "cpu": True, "notice": "ready"
             }]):
            upscaler.upscale(
                self.source, self.output, options or self.options,
                128, 128, run,
            )

    def test_prefer_gpu_tries_gpu_first_then_cpu(self):
        calls = []

        def run(args, cancelled=None, **kwargs):
            calls.append(args)
            if len(calls) == 1:
                raise RuntimeError("GPU processing failed")

        self.call(run)
        self.assertEqual(len(calls), 2)
        self.assertNotIn("-g", calls[0])
        self.assertEqual(calls[1][calls[1].index("-g") + 1], "-1")

    def test_explicit_gpu_does_not_fallback(self):
        calls = []

        def run(args, cancelled=None, **kwargs):
            calls.append(args)
            raise RuntimeError("GPU processing failed")

        with self.assertRaisesRegex(RuntimeError, "GPU processing failed"):
            self.call(run, {**self.options, "hardware": "gpu"})
        self.assertEqual(len(calls), 1)
        self.assertNotIn("-g", calls[0])

    def test_explicit_gpu_vulkan_failure_does_not_fallback(self):
        calls = []

        def run(args, cancelled=None, **kwargs):
            calls.append(args)
            raise RuntimeError("vkCreateInstance failed -9")

        with self.assertRaisesRegex(RuntimeError, "working Vulkan ICD"):
            self.call(run, {**self.options, "hardware": "gpu"})
        self.assertEqual(len(calls), 1)
        self.assertNotIn("-g", calls[0])

    def test_explicit_cpu_uses_cpu_device(self):
        calls = []

        def run(args, cancelled=None, **kwargs):
            calls.append(args)

        self.call(run, {**self.options, "hardware": "cpu"})
        self.assertEqual(len(calls), 1)
        self.assertEqual(calls[0][calls[0].index("-g") + 1], "-1")

    def test_vulkan_initialization_failure_is_not_retried(self):
        calls = []

        def run(args, cancelled=None, **kwargs):
            calls.append(args)
            raise RuntimeError("vkCreateInstance failed -9")

        with self.assertRaisesRegex(RuntimeError, "working Vulkan ICD"):
            self.call(run)
        self.assertEqual(len(calls), 1)

    def test_model_directory_name_must_match_upstream_names(self):
        self.assertTrue(upscaler._waifu2x_model_dir_supported(Path("/models/models-cunet")))
        self.assertTrue(upscaler._waifu2x_model_dir_supported(Path("/models/models-upconv_7_anime_style_art_rgb")))
        self.assertFalse(upscaler._waifu2x_model_dir_supported(Path("/models/anime")))


if __name__ == "__main__":
    unittest.main()
