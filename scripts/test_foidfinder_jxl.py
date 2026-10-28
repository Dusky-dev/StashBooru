"""Regression tests for the PixAI remote worker's JPEG XL input handling."""
from __future__ import annotations

from pathlib import Path
from types import SimpleNamespace
import tempfile
import unittest
from unittest.mock import patch

from foidfinder_worker import _decode_pixai_image


JXL_CODESTREAM = bytes.fromhex("ff0a") + b"test codestream"
JXL_CONTAINER = bytes.fromhex("0000000c4a584c200d0a870a") + b"test container"


class PixAIJXLTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.input = Path(self.directory.name) / "uploaded.img"

    def test_ordinary_formats_are_passed_directly_to_pillow(self):
        for data in (b"\x89PNG\r\n\x1a\nsample", b"\xff\xd8\xffsample", b"RIFF1234WEBP"):
            with self.subTest(data=data[:4]):
                self.input.write_bytes(data)
                with patch("foidfinder_worker.subprocess.run") as run:
                    self.assertEqual(_decode_pixai_image(self.input, lambda raw: raw), data)
                    run.assert_not_called()

    def test_jxl_codestream_is_transcoded_by_ffmpeg(self):
        self.input.write_bytes(JXL_CODESTREAM)

        def run(command, **kwargs):
            self.assertIn("-frames:v", command)
            Path(command[-1]).write_bytes(b"decoded png")
            return SimpleNamespace(returncode=0, stderr=b"")

        with patch("foidfinder_worker.shutil.which", side_effect=lambda name: "/usr/bin/ffmpeg" if name == "ffmpeg" else None):
            with patch("foidfinder_worker.subprocess.run", side_effect=run) as process:
                self.assertEqual(_decode_pixai_image(self.input, lambda raw: raw), b"decoded png")
                self.assertEqual(process.call_count, 1)
        self.assertEqual(self.input.read_bytes(), JXL_CODESTREAM)

    def test_jxl_container_falls_back_to_djxl(self):
        self.input.write_bytes(JXL_CONTAINER)

        def run(command, **kwargs):
            if command[0] == "ffmpeg":
                return SimpleNamespace(returncode=1, stderr=b"unsupported codec")
            Path(command[-1]).write_bytes(b"decoded from djxl")
            return SimpleNamespace(returncode=0, stderr=b"")

        with patch("foidfinder_worker.shutil.which", return_value="/usr/bin/tool"):
            with patch("foidfinder_worker.subprocess.run", side_effect=run) as process:
                self.assertEqual(_decode_pixai_image(self.input, lambda raw: raw), b"decoded from djxl")
                self.assertEqual(process.call_count, 2)

    def test_djxl_animated_first_frame(self):
        self.input.write_bytes(JXL_CONTAINER)

        def run(command, **kwargs):
            Path(command[-1]).with_name("frame-0.png").write_bytes(b"first frame")
            return SimpleNamespace(returncode=0, stderr=b"")

        with patch("foidfinder_worker.shutil.which", side_effect=lambda name: "/usr/bin/djxl" if name == "djxl" else None):
            with patch("foidfinder_worker.subprocess.run", side_effect=run):
                self.assertEqual(_decode_pixai_image(self.input, lambda raw: raw), b"first frame")

    def test_missing_decoder_returns_install_hint(self):
        self.input.write_bytes(JXL_CODESTREAM)
        with patch("foidfinder_worker.shutil.which", return_value=None):
            with self.assertRaisesRegex(RuntimeError, "JPEG XL tagging needs"):
                _decode_pixai_image(self.input, lambda raw: raw)

    def test_failed_decoder_reports_error(self):
        self.input.write_bytes(JXL_CODESTREAM)
        with patch("foidfinder_worker.shutil.which", side_effect=lambda name: "/usr/bin/ffmpeg" if name == "ffmpeg" else None):
            with patch("foidfinder_worker.subprocess.run", return_value=SimpleNamespace(returncode=1, stderr=b"decode failure")):
                with self.assertRaisesRegex(RuntimeError, "decode failure"):
                    _decode_pixai_image(self.input, lambda raw: raw)

    def test_decoded_output_is_bounded(self):
        self.input.write_bytes(JXL_CODESTREAM)

        def run(command, **kwargs):
            Path(command[-1]).write_bytes(b"12")
            return SimpleNamespace(returncode=0, stderr=b"")

        with patch("foidfinder_worker._MAX_DECODED_JXL_BYTES", 1):
            with patch("foidfinder_worker.shutil.which", side_effect=lambda name: "/usr/bin/ffmpeg" if name == "ffmpeg" else None):
                with patch("foidfinder_worker.subprocess.run", side_effect=run):
                    with self.assertRaisesRegex(ValueError, "PNG limit"):
                        _decode_pixai_image(self.input, lambda raw: raw)


if __name__ == "__main__":
    unittest.main()
