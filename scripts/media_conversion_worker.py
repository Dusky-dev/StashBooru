#!/usr/bin/env python3
"""Shared local/remote media encoder. Never changes the input or Stash metadata.

Only allowlisted codecs and numeric controls are accepted; no shell commands or
client supplied paths are accepted by the HTTP server. Output is fully decoded
and checked before the caller can activate it.
"""
from __future__ import annotations

import argparse
import json
import math
import os
from pathlib import Path
import shutil
import signal
import subprocess
import struct
import tempfile
import time
import zlib
from fractions import Fraction

import media_upscale_worker as upscaler

FFMPEG = os.environ.get("STASH_CONVERTER_FFMPEG", "ffmpeg")
FFPROBE = os.environ.get("STASH_CONVERTER_FFPROBE", "ffprobe")
TIMEOUT = int(os.environ.get("STASH_CONVERTER_TIMEOUT_SECONDS", "86400"))
THREADS = max(1, int(os.environ.get("STASH_CONVERTER_THREADS", "4")))
INPUT_FORMATS = "mov,matroska,webm,avi,asf,flv,mpeg,mpegts,ogg,nut,ivf,h264,hevc,mjpeg,image2,image2pipe,jpeg_pipe,png_pipe,apng,gif,webp_pipe,bmp_pipe,tiff_pipe,jpegxl_pipe,jpegxl_anim,ico,exr_pipe,j2k_pipe"
PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"
MAX_DECODED_WEBP_FRAME_BYTES = 512 * 1024 * 1024
MIN_NORMALIZED_FRAME_DELAY_SECONDS = 0.01

# id: (label, extension, family, encoder candidates, controls)
FORMATS = {
    "jxl": ("JPEG XL", "jxl", "image", ["cjxl", "libjxl"], ["quality", "effort", "decodingSpeed", "fasterDecoding"]),
    "ajxl": ("Animated JPEG XL (AJXL)", "jxl", "animation", ["cjxl"], ["quality", "effort", "decodingSpeed", "fasterDecoding"]),
    "av1-mp4": ("AV1 / MP4", "mp4", "video", ["libsvtav1", "libaom-av1"], ["quality", "effort", "decodingSpeed"]),
    "av1-mkv": ("AV1 / MKV", "mkv", "video", ["libsvtav1", "libaom-av1"], ["quality", "effort", "decodingSpeed"]),
    "av1-webm": ("AV1 / WebM", "webm", "video", ["libsvtav1", "libaom-av1"], ["quality", "effort", "decodingSpeed"]),
    "h264": ("H.264 / MP4", "mp4", "video", ["libx264"], ["quality", "effort"]),
    "hevc": ("HEVC / MP4", "mp4", "video", ["libx265"], ["quality", "effort"]),
    "vp9": ("VP9 / WebM", "webm", "video", ["libvpx-vp9"], ["quality", "effort"]),
    "mov": ("H.264 / MOV", "mov", "video", ["libx264"], ["quality", "effort"]),
    "jpeg": ("JPEG", "jpg", "image", ["mjpeg"], ["quality"]),
    "png": ("PNG", "png", "image", ["png"], ["effort"]),
    "webp": ("WebP (still or animated)", "webp", "animation", ["libwebp_anim", "libwebp"], ["quality", "effort", "lossless"]),
    "avif": ("AVIF", "avif", "image", ["libaom-av1"], ["quality", "effort", "decodingSpeed"]),
    "gif": ("GIF", "gif", "animation", ["gif"], []),
    "apng": ("Animated PNG", "png", "animation", ["apng"], ["effort"]),
    "tiff": ("TIFF", "tiff", "image", ["tiff"], []),
    "bmp": ("BMP", "bmp", "image", ["bmp"], []),
}
GPU = {
    "av1": ["av1_nvenc", "av1_qsv", "av1_vaapi"],
    "h264": ["h264_nvenc", "h264_qsv", "h264_vaapi"],
    "hevc": ["hevc_nvenc", "hevc_qsv", "hevc_vaapi"],
    "mov": ["h264_nvenc", "h264_qsv", "h264_vaapi"],
    "vp9": ["vp9_qsv", "vp9_vaapi"],
}
_capabilities_cache: tuple[float, dict] | None = None
_aom_decoding_speed_cache: tuple[float, bool] | None = None
AV1_DECODING_SPEED_FORMATS = {"av1-mp4", "av1-mkv", "av1-webm", "avif"}


def run(args: list[str], cancelled=None, timeout=TIMEOUT, stdout_file=None, offline_models=False) -> str:
    # File-backed logs bound memory even for a long failing encode.
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        env = {**os.environ, "HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1"} if offline_models else None
        # HTTP cancellation owns this process tree. Local subprocesses instead
        # inherit the group already managed by Stash's Go client.
        own_group = cancelled is not None and os.name == "posix"
        proc = subprocess.Popen(args, stdin=subprocess.DEVNULL, stdout=stdout_file if stdout_file is not None else stdout, stderr=stderr, env=env, start_new_session=own_group)
        started = time.monotonic()
        success = False
        try:
            while proc.poll() is None:
                if cancelled and cancelled():
                    raise RuntimeError("conversion cancelled")
                if time.monotonic() - started > timeout:
                    raise RuntimeError("encoder timed out")
                time.sleep(0.1)
            if proc.returncode:
                stderr.seek(max(0, stderr.tell() - 4096))
                raise RuntimeError(stderr.read().decode("utf-8", "replace").strip() or "encoder failed")
            success = True
            if stdout_file is not None:
                return ""
            stdout.seek(0)
            return stdout.read(4 * 1024 * 1024).decode("utf-8", "replace")
        finally:
            if not success and own_group:
                kill_process_tree(proc.pid)
            elif proc.poll() is None:
                if os.name == "nt" and cancelled is not None:
                    subprocess.run(["taskkill", "/PID", str(proc.pid), "/T", "/F"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
                else:
                    proc.kill()
            proc.wait()


def ffmpeg_prefix() -> list[str]:
    return [FFMPEG, "-nostdin", "-hide_banner", "-loglevel", "error", "-threads", str(THREADS),
            "-filter_threads", "1", "-filter_complex_threads", "1"]


def linux_descendant_pids(root_pid: int) -> list[int]:
    """Return child process IDs deepest-first for restricted Linux containers."""
    proc_root = Path("/proc")
    if not proc_root.is_dir():
        return []
    children: dict[int, list[int]] = {}
    for entry in proc_root.iterdir():
        if not entry.name.isdigit():
            continue
        try:
            stat = (entry / "stat").read_text()
            fields = stat[stat.rfind(")") + 2:].split()
            if len(fields) > 1:
                children.setdefault(int(fields[1]), []).append(int(entry.name))
        except (OSError, ValueError):
            continue

    result: list[int] = []

    def visit(parent: int) -> None:
        for child in children.get(parent, []):
            visit(child)
            result.append(child)

    visit(root_pid)
    return result


def kill_process_tree(root_pid: int) -> None:
    """Stop our encoder and any descendants, even if they left its process group."""
    for pid in linux_descendant_pids(root_pid):
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
    try:
        os.killpg(root_pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    try:
        os.kill(root_pid, signal.SIGKILL)
    except ProcessLookupError:
        pass


def device_args(encoder: str) -> list[str]:
    if encoder.endswith("_vaapi"):
        return ["-vaapi_device", os.environ.get("STASH_CONVERTER_VAAPI_DEVICE", "/dev/dri/renderD128")]
    return []


def gpu_usable(encoder: str) -> bool:
    try:
        run(ffmpeg_prefix() + device_args(encoder) + ["-f", "lavfi", "-i", "color=size=128x128:rate=1",
            "-frames:v", "1"] + (["-vf", "format=nv12,hwupload"] if encoder.endswith("_vaapi") else []) +
            ["-c:v", encoder, "-f", "null", "-"], timeout=15)
        return True
    except (OSError, RuntimeError):
        return False


def aom_decoding_speed_usable() -> bool:
    """Probe the actual libaom option instead of inferring it from version strings."""
    global _aom_decoding_speed_cache
    if _aom_decoding_speed_cache and time.monotonic() - _aom_decoding_speed_cache[0] < 300:
        return _aom_decoding_speed_cache[1]
    try:
        run(ffmpeg_prefix() + ["-f", "lavfi", "-i", "color=size=128x128:rate=1", "-frames:v", "1",
            "-c:v", "libaom-av1", "-aom-params", "enable-low-complexity-decode=1", "-f", "null", "-"], timeout=20)
        supported = True
    except (OSError, RuntimeError):
        supported = False
    _aom_decoding_speed_cache = time.monotonic(), supported
    return supported


def capabilities(probe_gpu=True, only_format=None, probe_decoding_speed=True) -> dict:
    global _capabilities_cache
    if _capabilities_cache and time.monotonic() - _capabilities_cache[0] < 300:
        return _capabilities_cache[1]
    available = run([FFMPEG, "-hide_banner", "-encoders"], timeout=20)
    encoders = {line.split()[1] for line in available.splitlines() if len(line.split()) > 1}
    gpu = {codec: [e for e in candidates if e in encoders and gpu_usable(e)] for codec, candidates in GPU.items()
           if probe_gpu and (only_format is None or codec == only_format.split("-")[0])}
    jxl_tools = True
    for tool in ("cjxl", "djxl"):
        try:
            run([tool, "--version"], timeout=10)
        except (OSError, RuntimeError):
            jxl_tools = False
    faster_decoding = False
    if jxl_tools:
        try:
            help_result = subprocess.run(["cjxl", "--help"], stdout=subprocess.PIPE,
                                         stderr=subprocess.STDOUT, timeout=10, check=False)
            faster_decoding = b"--faster_decoding" in help_result.stdout
        except (OSError, subprocess.TimeoutExpired):
            faster_decoding = False
    av1_decoding_speed = (
        probe_decoding_speed and "libaom-av1" in encoders and aom_decoding_speed_usable()
    )
    formats = []
    for key, (label, ext, family, candidates, controls) in FORMATS.items():
        cpu = [e for e in candidates if e == "cjxl" and jxl_tools]
        cpu += [e for e in candidates if e != "cjxl" and e in encoders]
        hardware = gpu.get(key.split("-")[0], [])
        controls = [c for c in controls if (
            (c not in ("decodingSpeed", "fasterDecoding")) or
            (key in ("jxl", "ajxl") and faster_decoding) or
            (key in AV1_DECODING_SPEED_FORMATS and av1_decoding_speed)
        )]
        formats.append({"id": key, "label": label, "extension": ext, "family": family,
                        "cpu": cpu, "gpu": hardware, "controls": controls,
                        "decodingSpeedLevels": 4 if key in ("jxl", "ajxl") and faster_decoding else 1 if key in AV1_DECODING_SPEED_FORMATS and av1_decoding_speed else 0,
                        "available": bool(cpu or hardware)})
    value = {"formats": formats, "upscalers": upscaler.capabilities(), "version": 2}
    if probe_gpu and only_format is None:
        _capabilities_cache = time.monotonic(), value
    return value


def options(raw: dict) -> dict:
    if not isinstance(raw, dict) or set(raw) - {"format", "hardware", "quality", "effort", "distance", "decodingSpeed", "fasterDecoding", "lossless", "allowLarger", "dropAudio", "allowAlphaLoss", "upscaler", "upscaleScale"}:
        raise ValueError("unknown conversion option")
    raw = dict(raw)
    generic_speed = raw.pop("decodingSpeed", None)
    legacy_speed = raw.pop("fasterDecoding", None)
    if generic_speed is not None and legacy_speed is not None and generic_speed != legacy_speed:
        raise ValueError("decodingSpeed and legacy fasterDecoding must match when both are supplied")
    decode_speed = generic_speed if generic_speed is not None else legacy_speed if legacy_speed is not None else 0
    o = {"format": "jxl", "hardware": "auto", "quality": 90 if raw.get("format", "jxl") in ("jxl", "ajxl") else 80, "effort": 7, "distance": 1, "decodingSpeed": decode_speed,
         "lossless": False, "allowLarger": False, "dropAudio": False, "allowAlphaLoss": False,
         "upscaler": "", "upscaleScale": 2, **raw}
    if o["format"] not in FORMATS or o["hardware"] not in ("cpu", "gpu", "auto"):
        raise ValueError("invalid format or hardware mode")
    for name, low, high in (("quality", 0, 100), ("effort", 1, 9), ("distance", 0, 25)):
        v = o[name]
        if isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v) or not low <= v <= high:
            raise ValueError(f"{name} must be between {low} and {high}")
    if int(o["effort"]) != o["effort"]:
        raise ValueError("effort must be an integer")
    if o["upscaler"] not in ("", "waifu2x", "seedvr2") or isinstance(o["upscaleScale"], bool) or o["upscaleScale"] not in (2, 4):
        raise ValueError("choose waifu2x or SeedVR2 and a scale of 2 or 4")
    o["upscaleScale"] = int(o["upscaleScale"])
    speed_limit = 4 if o["format"] in ("jxl", "ajxl") else 1 if o["format"] in AV1_DECODING_SPEED_FORMATS else 0
    if isinstance(o["decodingSpeed"], bool) or not isinstance(o["decodingSpeed"], int) or not 0 <= o["decodingSpeed"] <= speed_limit:
        raise ValueError(f"decodingSpeed must be an integer between 0 and {speed_limit} for {o['format']}")
    for name in ("lossless", "allowLarger", "dropAudio", "allowAlphaLoss"):
        if not isinstance(o[name], bool):
            raise ValueError(f"{name} must be a boolean")
    if o["format"] in ("jxl", "ajxl") and "distance" not in raw:
        q = o["quality"]
        # libjxl's JxlEncoderDistanceFromQuality. Keep explicit legacy distance
        # requests working, including jobs sent by older StashBooru servers.
        o["distance"] = 0 if q >= 100 else 0.1 + (100 - q) * 0.09 if q >= 30 else 53 / 3000 * q * q - 23 / 20 * q + 25
    return o


def jxl_encoder_args(source: Path, output: Path, o: dict) -> list[str]:
    args = ["cjxl", str(source), str(output), "--distance=" + str(o["distance"]),
            "--effort=" + str(int(o["effort"])), "--num_threads=" + str(THREADS)]
    if o["decodingSpeed"] > 0:
        args.append("--faster_decoding=" + str(o["decodingSpeed"]))
    return args


def write_png_chunk(stream, name: bytes, data: bytes) -> None:
    stream.write(struct.pack(">I", len(data)))
    stream.write(name)
    stream.write(data)
    stream.write(struct.pack(">I", zlib.crc32(name + data) & 0xFFFFFFFF))


def png_chunks(path: Path):
    with path.open("rb") as stream:
        if stream.read(len(PNG_SIGNATURE)) != PNG_SIGNATURE:
            raise RuntimeError("Pillow produced an invalid PNG frame")
        while True:
            length = stream.read(4)
            if len(length) != 4:
                raise RuntimeError("truncated PNG frame")
            size = struct.unpack(">I", length)[0]
            name = stream.read(4)
            data = stream.read(size)
            crc = stream.read(4)
            if len(name) != 4 or len(data) != size or len(crc) != 4:
                raise RuntimeError("truncated PNG frame chunk")
            if zlib.crc32(name + data) & 0xFFFFFFFF != struct.unpack(">I", crc)[0]:
                raise RuntimeError("corrupt PNG frame chunk")
            yield name, data
            if name == b"IEND":
                return


def normalize_animation_durations(durations: list[float]) -> tuple[list[float], int]:
    """Give zero-delay frames one 10 ms tick so encoders can retain each frame."""
    normalized, repaired = [], 0
    for duration in durations:
        try:
            value = float(duration)
        except (TypeError, ValueError):
            raise ValueError("animation contains an invalid frame delay") from None
        if not math.isfinite(value) or value < 0:
            raise ValueError("animation contains an invalid frame delay")
        if value == 0:
            value = MIN_NORMALIZED_FRAME_DELAY_SECONDS
            repaired += 1
        normalized.append(value)
    return normalized, repaired


def animation_play_count(image) -> int:
    loop = image.info.get("loop")
    if image.format == "GIF":
        # GIF's Netscape extension stores repeats after the first play;
        # APNG and WebP store total plays, with 0 meaning infinite.
        return 0 if loop == 0 else int(loop or 0) + 1
    return int(loop if loop is not None else (1 if image.format == "PNG" else 0))


def animation_to_apng(image, output: Path, temporary_frame: Path, durations: list[float],
                      plays: int, cancelled=None) -> None:
    """Normalize an animation one composited frame at a time without merging frames."""
    width, height = image.size
    frame_count = int(getattr(image, "n_frames", 1))
    if frame_count < 2 or len(durations) != frame_count:
        raise ValueError("could not read the complete animation frame sequence")
    icc_profile = image.info.get("icc_profile")
    sequence = 0
    try:
        with output.open("xb") as destination:
            destination.write(PNG_SIGNATURE)
            for index in range(frame_count):
                if cancelled and cancelled():
                    raise RuntimeError("conversion cancelled")
                image.seek(index)
                image.load()
                delay = Fraction(str(durations[index]))
                if delay <= 0:
                    raise ValueError("animation contains zero-duration frames; cannot safely preserve its timing")
                delay = delay.limit_denominator(65535)
                if delay.numerator > 65535 or delay.denominator > 65535:
                    raise ValueError("animation frame delay cannot be represented safely in APNG")

                # Pillow coalesces frame disposal/blend operations when
                # converting each frame. Keep one full-canvas frame resident.
                frame = image.convert("RGBA")
                if frame.size != (width, height):
                    raise ValueError("animation frame dimensions do not match its canvas")
                save_options = {"format": "PNG"}
                if icc_profile:
                    save_options["icc_profile"] = icc_profile
                frame.save(temporary_frame, **save_options)

                wrote_header = False
                wrote_frame_control = False
                for name, data in png_chunks(temporary_frame):
                    if name == b"IHDR":
                        if index == 0:
                            write_png_chunk(destination, name, data)
                            write_png_chunk(destination, b"acTL", struct.pack(">II", frame_count, plays))
                        wrote_header = True
                    elif name == b"IDAT":
                        if not wrote_header:
                            raise RuntimeError("PNG frame is missing its image header")
                        if not wrote_frame_control:
                            write_png_chunk(destination, b"fcTL", struct.pack(
                                ">IIIIIHHBB", sequence, width, height, 0, 0,
                                delay.numerator, delay.denominator, 0, 0,
                            ))
                            sequence += 1
                            wrote_frame_control = True
                        if index == 0:
                            write_png_chunk(destination, b"IDAT", data)
                        else:
                            write_png_chunk(destination, b"fdAT", struct.pack(">I", sequence) + data)
                            sequence += 1
                    elif name not in (b"IEND", b"IHDR") and index == 0:
                        # Preserve PNG color-profile and other ancillary chunks
                        # from the generated frame before its first IDAT chunk.
                        write_png_chunk(destination, name, data)
                if not wrote_frame_control:
                    raise RuntimeError("PNG frame contains no image data")
                del frame
            write_png_chunk(destination, b"IEND", b"")
    except BaseException:
        output.unlink(missing_ok=True)
        raise
    finally:
        temporary_frame.unlink(missing_ok=True)


def prepare_input(source: Path, directory: Path, cancelled=None, known_single_frame=False) -> Path:
    # djxl supports animation even on FFmpeg builds whose JXL decoder is still-only.
    with source.open("rb") as stream:
        signature = stream.read(12)
    if signature.startswith(b"\xff\x0a") or signature == b"\x00\x00\x00\x0cJXL \r\n\x87\n":
        if shutil.which("djxl"):
            decoded = directory / "decoded.png"
            run(["djxl", str(source), str(decoded), "--num_threads=" + str(THREADS)], cancelled)
            return decoded
        if not known_single_frame:
            raise ValueError("djxl is required to read JPEG XL inputs without risking animation loss")
    is_gif = signature[:6] in (b"GIF87a", b"GIF89a")
    is_png = signature.startswith(PNG_SIGNATURE)
    is_webp = signature[:4] == b"RIFF" and signature[8:] == b"WEBP"
    if is_gif or is_png or is_webp:
        from PIL import Image
        with Image.open(source) as im:
            count = int(getattr(im, "n_frames", 1))
            if count > 1 and im.format in ("GIF", "PNG", "WEBP"):
                if im.info.get("default_image"):
                    raise ValueError("APNG with a separate poster frame is not supported by this conversion path")
                plays = animation_play_count(im)
                if im.format == "WEBP":
                    raw_durations = webp_animation_durations(source, count)
                else:
                    raw_durations = []
                    for index in range(count):
                        im.seek(index)
                        raw_durations.append(im.info.get("duration") or 0)
                im.seek(0)
                raw_durations = [float(value) / 1000 for value in raw_durations]
                durations, repaired = normalize_animation_durations(raw_durations)
                # FFmpeg versions vary in animated WebP support. Also rebuild
                # GIF/APNG files with zero holds so every encoder sees the same
                # positive, verified frame timeline.
                if im.format == "WEBP" or repaired:
                    decoded = directory / "decoded.png"
                    frame_bytes = im.width * im.height * 4
                    if frame_bytes > MAX_DECODED_WEBP_FRAME_BYTES:
                        raise ValueError("animated image frame exceeds the 512 MiB decoded-frame budget")
                    animation_to_apng(im, decoded, directory / "animation-frame.png",
                                      durations, plays, cancelled)
                    return decoded
    return source


def animation_metadata(source: Path) -> dict:
    from PIL import Image, UnidentifiedImageError
    try:
        im = Image.open(source)
    except (UnidentifiedImageError, OSError):
        return {}
    with im:
        if im.format not in ("GIF", "PNG", "WEBP"):
            return {}
        count = getattr(im, "n_frames", 1)
        info = dict(im.info)
        if info.get("default_image"):
            raise ValueError("APNG with a separate poster frame is not supported by this conversion path")
        durations, alpha = [], False
        for i in range(count):
            im.seek(i)
            im.load()
            alpha |= im.convert("RGBA").getextrema()[3][0] < 255
            durations.append(float(im.info.get("duration") or 0) / 1000)
        if count == 1:
            return {"alpha": alpha}
        if any(d <= 0 for d in durations):
            raise ValueError("animation contains zero-duration frames; cannot safely preserve its timing")
        plays = int(info.get("loop", 1))
        if im.format == "GIF":
            plays = 0 if info.get("loop") == 0 else int(info.get("loop", 0)) + 1
        return {"durations": durations, "plays": plays, "alpha": alpha,
                "duration": sum(durations), "frames": count}


def webp_final_duration(path: Path, seconds: float) -> None:
    # FFmpeg's WebP muxer guesses the last delay. ANMF stores an independent
    # 24-bit millisecond duration; replace only that field, not image data.
    last = None
    with path.open("r+b") as stream:
        if stream.read(12)[8:] != b"WEBP":
            raise RuntimeError("invalid WebP output")
        while header := stream.read(8):
            if len(header) != 8:
                raise RuntimeError("truncated WebP chunk")
            size = int.from_bytes(header[4:], "little")
            if header[:4] == b"ANMF":
                last = stream.tell() + 12
            stream.seek(size + (size % 2), 1)
        if last is not None:
            stream.seek(last)
            stream.write(round(seconds * 1000).to_bytes(3, "little"))


def webp_chunks(path: Path):
    with path.open("rb") as stream:
        header = stream.read(12)
        if len(header) != 12 or header[:4] != b"RIFF" or header[8:] != b"WEBP":
            raise RuntimeError("still WebP encoder produced an invalid file")
        while True:
            chunk_header = stream.read(8)
            if not chunk_header:
                return
            if len(chunk_header) != 8:
                raise RuntimeError("truncated still WebP chunk")
            name = chunk_header[:4]
            size = int.from_bytes(chunk_header[4:], "little")
            data_offset = stream.tell()
            if len(stream.read(size)) != size:
                raise RuntimeError("truncated still WebP image data")
            has_padding = bool(size & 1)
            if has_padding and len(stream.read(1)) != 1:
                raise RuntimeError("truncated still WebP padding")
            yield name, data_offset, size


def webp_animation_durations(path: Path, expected_frames: int) -> list[int]:
    """Read exact ANMF millisecond holds without decoding every WebP frame twice."""
    durations = []
    file_size = path.stat().st_size
    with path.open("rb") as stream:
        header = stream.read(12)
        if len(header) != 12 or header[:4] != b"RIFF" or header[8:] != b"WEBP":
            raise ValueError("invalid animated WebP container")
        riff_end = 8 + int.from_bytes(header[4:8], "little")
        if riff_end < 12 or riff_end > file_size:
            raise ValueError("truncated animated WebP container")
        while stream.tell() < riff_end:
            if riff_end - stream.tell() < 8:
                raise ValueError("truncated animated WebP chunk header")
            chunk_header = stream.read(8)
            name = chunk_header[:4]
            size = int.from_bytes(chunk_header[4:], "little")
            data_start = stream.tell()
            chunk_end = data_start + size + (size & 1)
            if chunk_end > riff_end:
                raise ValueError("truncated animated WebP chunk")
            if name == b"ANMF":
                if size < 16:
                    raise ValueError("animated WebP frame metadata is truncated")
                stream.seek(data_start + 12)
                delay = stream.read(3)
                if len(delay) != 3:
                    raise ValueError("animated WebP frame metadata is truncated")
                durations.append(int.from_bytes(delay, "little"))
            stream.seek(chunk_end)
    if len(durations) != expected_frames:
        raise ValueError("animated WebP frame count does not match its container")
    return durations


def write_webp_chunk(stream, name: bytes, data: bytes) -> None:
    stream.write(name)
    stream.write(len(data).to_bytes(4, "little"))
    stream.write(data)
    if len(data) & 1:
        stream.write(b"\x00")


def pillow_webp_available() -> bool:
    try:
        from PIL import features
        return bool(features.check("webp"))
    except (ImportError, ValueError):
        return False


def animated_webp_encode(source: Path, metadata_source: Path, output: Path, temporary_frame: Path,
                         o: dict, durations: list[float], plays: int, cancelled=None) -> None:
    """Encode WebP frames independently and mux them without dropping repeats."""
    from PIL import Image

    with Image.open(source) as image:
        frame_count = int(getattr(image, "n_frames", 1))
        if frame_count < 2 or len(durations) != frame_count:
            raise ValueError("Pillow could not read the complete animation frame sequence")
        width, height = image.size
        if width < 1 or height < 1 or width > 16_384 or height > 16_384:
            raise ValueError("animation dimensions cannot be represented in WebP")
        if width * height * 4 > MAX_DECODED_WEBP_FRAME_BYTES:
            raise ValueError("animation frame exceeds the 512 MiB decoded-frame budget")

        profile_source = image
        if metadata_source != source:
            try:
                profile_source = Image.open(metadata_source)
            except (OSError, ValueError):
                profile_source = image
        close_profile_source = profile_source is not image
        try:
            info = profile_source.info
            icc_profile = info.get("icc_profile") or b""
            exif = info.get("exif") or b""
            xmp = info.get("xmp") or b""
            if isinstance(icc_profile, str):
                icc_profile = icc_profile.encode("latin-1")
            if isinstance(exif, str):
                exif = exif.encode("latin-1")
            if isinstance(xmp, str):
                xmp = xmp.encode("utf-8")
        finally:
            if close_profile_source:
                profile_source.close()

        loop_count = int(plays)
        if not 0 <= loop_count <= 65_535:
            raise ValueError("animation loop count cannot be represented in WebP")
        cumulative_ms = Fraction(0)
        previous_ms = 0
        frame_delays = []
        for seconds in durations:
            if not math.isfinite(float(seconds)) or seconds <= 0:
                raise ValueError("animation contains zero-duration frames; cannot safely preserve its timing")
            cumulative_ms += Fraction(str(seconds)) * 1000
            boundary_ms = int(cumulative_ms + Fraction(1, 2))
            delay_ms = boundary_ms - previous_ms
            if not 1 <= delay_ms <= 0xFFFFFF:
                raise ValueError("animation frame delay cannot be represented safely in WebP")
            frame_delays.append(delay_ms)
            previous_ms = boundary_ms

        temporary_frame.parent.mkdir(parents=True, exist_ok=True)
        try:
            with output.open("xb") as destination:
                destination.write(b"RIFF\x00\x00\x00\x00WEBP")
                # Set the alpha flag when any frame contains transparency. The
                # frame encoder may also choose an alpha-capable bitstream.
                has_alpha = False
                vp8x_flags_offset = None
                vp8x_flags = 0
                for index in range(frame_count):
                    if cancelled and cancelled():
                        raise RuntimeError("conversion cancelled")
                    image.seek(index)
                    image.load()
                    frame = image.convert("RGBA")
                    if frame.size != (width, height):
                        raise ValueError("animation frame dimensions do not match the canvas")
                    frame_has_alpha = frame.getchannel("A").getextrema()[0] < 255
                    has_alpha |= frame_has_alpha
                    frame.save(temporary_frame, format="WEBP", quality=o["quality"],
                               method=round(o["effort"] * 6 / 9), lossless=o["lossless"], exact=True)
                    image_chunks = [(name, offset, size) for name, offset, size in webp_chunks(temporary_frame)
                                    if name in (b"ALPH", b"VP8 ", b"VP8L")]
                    if not image_chunks or not any(name in (b"VP8 ", b"VP8L") for name, _, _ in image_chunks):
                        raise RuntimeError("still WebP encoder returned no image bitstream")
                    has_alpha |= any(name == b"ALPH" for name, _, _ in image_chunks)

                    if index == 0:
                        vp8x_flags = 0x02
                        if has_alpha:
                            vp8x_flags |= 0x10
                        if icc_profile:
                            vp8x_flags |= 0x20
                        if exif:
                            vp8x_flags |= 0x08
                        if xmp:
                            vp8x_flags |= 0x04
                        vp8x = bytes([vp8x_flags, 0, 0, 0]) + (width - 1).to_bytes(3, "little") + (height - 1).to_bytes(3, "little")
                        write_webp_chunk(destination, b"VP8X", vp8x)
                        vp8x_flags_offset = destination.tell() - len(vp8x)
                        if icc_profile:
                            write_webp_chunk(destination, b"ICCP", icc_profile)
                        write_webp_chunk(destination, b"ANIM", bytes(4) + struct.pack("<H", loop_count))
                    elif has_alpha and not (vp8x_flags & 0x10):
                        # A later frame can introduce transparency even if the
                        # first frame is opaque. Patch VP8X while retaining the
                        # stream position at the end of the already-written data.
                        current_position = destination.tell()
                        vp8x_flags |= 0x10
                        destination.seek(vp8x_flags_offset)
                        destination.write(bytes([vp8x_flags]))
                        destination.seek(current_position)

                    frame_header = (bytes(6) + (width - 1).to_bytes(3, "little") +
                                    (height - 1).to_bytes(3, "little") + frame_delays[index].to_bytes(3, "little") + b"\x02")
                    inner_size = sum(8 + size + (size & 1) for _, _, size in image_chunks)
                    payload_size = len(frame_header) + inner_size
                    destination.write(b"ANMF")
                    destination.write(payload_size.to_bytes(4, "little"))
                    destination.write(frame_header)
                    with temporary_frame.open("rb") as encoded:
                        for name, offset, size in image_chunks:
                            encoded.seek(offset - 8)
                            remaining = 8 + size + (size & 1)
                            while remaining:
                                block = encoded.read(min(1024 * 1024, remaining))
                                if not block:
                                    raise RuntimeError("truncated still WebP image bitstream")
                                destination.write(block)
                                remaining -= len(block)
                    del frame

                if exif:
                    write_webp_chunk(destination, b"EXIF", exif)
                if xmp:
                    write_webp_chunk(destination, b"XMP ", xmp)
                end = destination.tell()
                riff_size = end - 8
                if riff_size > 0xFFFFFFFF:
                    raise ValueError("encoded WebP exceeds the RIFF container size limit")
                destination.seek(4)
                destination.write(struct.pack("<I", riff_size))
        except BaseException:
            output.unlink(missing_ok=True)
            raise
        finally:
            temporary_frame.unlink(missing_ok=True)


def jxl_timing_input(source: Path, output: Path, durations: list[float]) -> Path:
    # cjxl's APNG reader uses millisecond ticks. Independent rounding drifts at
    # rates such as 30/60 fps. Round cumulative boundaries instead (33,34,33 ms),
    # retaining total duration to 1 ms without decoding/recompressing PNG pixels.
    elapsed = Fraction(0)
    previous_ms, index = 0, 0
    with source.open("rb") as src, output.open("xb") as dst:
        signature = src.read(8)
        if signature != b"\x89PNG\r\n\x1a\n":
            raise ValueError("JPEG XL animation requires an APNG intermediate")
        dst.write(signature)
        while header := src.read(8):
            if len(header) != 8:
                raise ValueError("truncated APNG chunk")
            length, kind = struct.unpack(">I4s", header)
            dst.write(header)
            if kind == b"fcTL":
                if length != 26 or index >= len(durations):
                    raise ValueError("APNG frame count does not match the source timing")
                payload = bytearray(src.read(length))
                if len(payload) != length or len(src.read(4)) != 4:
                    raise ValueError("truncated APNG frame control")
                elapsed += Fraction(durations[index]).limit_denominator(1000000)
                boundary_ms = round(elapsed * 1000)
                delay = Fraction(boundary_ms - previous_ms, 1000)
                if delay <= 0 or delay.numerator > 65535:
                    raise ValueError("frame delay cannot be represented safely by the JPEG XL APNG reader")
                payload[20:24] = struct.pack(">HH", delay.numerator, delay.denominator)
                dst.write(payload)
                dst.write(struct.pack(">I", zlib.crc32(kind + payload)))
                previous_ms, index = boundary_ms, index + 1
            else:
                remaining = length + 4
                while remaining:
                    chunk = src.read(min(remaining, 1024 * 1024))
                    if not chunk:
                        raise ValueError("truncated APNG data")
                    dst.write(chunk)
                    remaining -= len(chunk)
        if index != len(durations):
            raise ValueError("APNG frame count does not match the source timing")
    return output


def probe(source: Path, cancelled=None) -> dict:
    data = json.loads(run([FFPROBE, "-v", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", INPUT_FORMATS,
        "-count_frames", "-show_streams", "-show_format", "-of", "json", str(source)], cancelled))
    videos = [s for s in data["streams"] if s["codec_type"] == "video"]
    if len(videos) != 1:
        raise ValueError("conversion requires exactly one video/image stream")
    v = videos[0]
    audio = [s for s in data["streams"] if s["codec_type"] == "audio"]
    frames = int(v.get("nb_read_frames", 0))
    if not frames or not v.get("width") or not v.get("height"):
        raise ValueError("input could not be decoded completely")
    try:
        rate = float(Fraction(v.get("avg_frame_rate", "0/1")))
    except (ValueError, ZeroDivisionError):
        rate = 0
    duration = float(v.get("duration", data.get("format", {}).get("duration", 0)))
    if not duration and rate:
        duration = frames / rate
    result = {"width": v["width"], "height": v["height"], "frames": frames, "duration": duration,
            "frameRate": rate, "videoCodec": v["codec_name"], "audioCodec": audio[0]["codec_name"] if audio else "",
            "audioStreams": len(audio), "streams": len(data["streams"]),
            "bitRate": int(data.get("format", {}).get("bit_rate", 0)),
            "pixelFormat": v.get("pix_fmt", ""),
            "colorTransfer": v.get("color_transfer", ""), "colorPrimaries": v.get("color_primaries", ""),
            "colorSpace": v.get("color_space", ""),
            "alpha": "a" in v.get("pix_fmt", "").replace("gray", "") or v.get("pix_fmt") == "pal8"}
    animation = animation_metadata(source)
    if frames > 1 and not animation.get("durations"):
        # Container duration may include longer audio or a nonzero start time.
        # Compare the presented video span, including its last frame's hold.
        result["duration"] = video_duration(source, rate, cancelled)
    result.update(animation)
    if animation.get("durations"):
        # FFprobe may report the common frame cadence, excluding a longer final
        # hold. Use the complete animation timeline, including that last frame,
        # consistently with the scanner and converted-image metadata.
        result["frameRate"] = animation["frames"] / animation["duration"]
    return result


def video_duration(source: Path, rate: float, cancelled=None) -> float:
    first = last = None
    last_delay = 0.0
    with tempfile.TemporaryFile() as packets:
        run([FFPROBE, "-v", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", INPUT_FORMATS,
             "-select_streams", "v:0", "-show_packets", "-show_entries", "packet=pts_time,duration_time",
             "-of", "csv=p=0", str(source)], cancelled, stdout_file=packets)
        packets.seek(0)
        for line in packets:
            fields = line.strip().split(b",")
            try:
                pts = float(fields[0])
                delay = float(fields[1]) if len(fields) > 1 and fields[1] != b"N/A" else 0.0
            except (ValueError, IndexError):
                continue
            if not math.isfinite(pts) or not math.isfinite(delay):
                continue
            first = pts if first is None else min(first, pts)
            if last is None or pts > last:
                last, last_delay = pts, delay
    if first is None or last is None:
        raise ValueError("could not determine video presentation duration")
    if last_delay <= 0 and rate > 0:
        last_delay = 1 / rate
    if last_delay <= 0:
        raise ValueError("could not determine the final video frame duration")
    return last - first + last_delay


def input_args(source: Path) -> list[str]:
    args = ["-protocol_whitelist", "file,pipe", "-format_whitelist", INPUT_FORMATS]
    with source.open("rb") as stream:
        if stream.read(6) in (b"GIF87a", b"GIF89a"):
            # FFmpeg's GIF demuxer otherwise replaces short delays on some versions.
            args += ["-min_delay", "0"]
    return args + ["-i", str(source)]


def video_quality(encoder: str, o: dict) -> list[str]:
    q, effort = o["quality"], int(o["effort"])
    crf = round((100 - q) * (63 if "av1" in encoder or "vpx" in encoder else 51) / 100)
    if encoder.endswith("_nvenc"):
        return ["-rc", "vbr", "-cq", str(crf), "-b:v", "0", "-preset", "p" + str(min(7, effort))]
    if encoder.endswith("_qsv"):
        return ["-global_quality", str(max(1, crf)), "-preset", str(8 - min(7, effort))]
    if encoder.endswith("_vaapi"):
        return ["-rc_mode", "CQP", "-qp", str(max(1, crf))]
    if encoder == "libsvtav1":
        return ["-crf", str(crf), "-preset", str(13 - effort), "-svtav1-params", "lp=" + str(THREADS)]
    if encoder == "libaom-av1":
        cpu_used = 9 - effort
        args = ["-crf", str(crf), "-b:v", "0"]
        if o["decodingSpeed"] > 0:
            # libaom's low-complexity decode tool is defined for cpu-used 1–3.
            cpu_used = min(3, max(1, cpu_used))
            args += ["-aom-params", "enable-low-complexity-decode=1"]
        return args + ["-cpu-used", str(cpu_used)]
    if encoder == "libvpx-vp9":
        return ["-crf", str(crf), "-b:v", "0", "-cpu-used", str(9 - effort)]
    return ["-crf", str(crf), "-preset", ["ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"][effort - 1]]


def convert(source: Path, output: Path, raw: dict, cancelled=None) -> dict:
    o = options(raw)
    fmt = o["format"]
    _, ext, family, _, _ = FORMATS[fmt]
    if output.exists():
        raise ValueError("output already exists")
    # The processor selection controls inference for image upscales. PNG/JXL
    # encoding uses CPU codecs even when the upscaler must run on the GPU.
    low_complexity_av1 = o["decodingSpeed"] > 0 and fmt in AV1_DECODING_SPEED_FORMATS
    if low_complexity_av1 and o["hardware"] == "gpu":
        raise ValueError("AV1 decode-speed mode requires the libaom CPU encoder; choose CPU or Auto")
    encoding_hardware = "cpu" if low_complexity_av1 or (o["upscaler"] and family != "video") else o["hardware"]
    cap = next(f for f in capabilities(
        encoding_hardware != "cpu", fmt, probe_decoding_speed=low_complexity_av1
    )["formats"] if f["id"] == fmt)
    if low_complexity_av1 and ("decodingSpeed" not in cap["controls"] or "libaom-av1" not in cap["cpu"]):
        raise ValueError("this worker does not support AV1 low-complexity decode mode with libaom")
    candidates = cap["gpu"] if encoding_hardware == "gpu" else cap["cpu"]
    if encoding_hardware == "auto":
        candidates = cap["gpu"] or cap["cpu"]
    if low_complexity_av1:
        candidates = ["libaom-av1"]
    if not candidates:
        raise ValueError(f"{fmt} has no working {encoding_hardware} encoder on this worker")
    encoder = candidates[0]
    started = time.monotonic()
    with tempfile.TemporaryDirectory(prefix="stash-convert-", dir=output.parent) as temporary:
        directory = Path(temporary)
        decoded = prepare_input(source, directory, cancelled)
        before = probe(decoded, cancelled)
        if before["streams"] != 1 + before["audioStreams"]:
            raise ValueError("embedded subtitles, attachments or data tracks require a separate remux; source kept")
        if family == "image" and before["frames"] > 1:
            raise ValueError("choose an animated output; still-image conversion would discard frames")
        if before["audioStreams"] and family != "video" and not o["dropAudio"]:
            raise ValueError("output cannot contain audio; explicitly enable discard audio to continue")
        if before["alpha"] and (family == "video" or fmt in ("jpeg", "avif")) and not o["allowAlphaLoss"]:
            raise ValueError("output may discard transparency; explicitly allow transparency loss to continue")
        if o["upscaler"]:
            if before["frames"] != 1 or before["audioStreams"]:
                raise ValueError("optional upscaling currently supports still images only; disable it for animations/videos")
            normalized = directory / "upscale-input.png"
            run(ffmpeg_prefix() + input_args(decoded) + ["-frames:v", "1", str(normalized)], cancelled)
            enlarged = directory / "upscaled.png"
            upscaler.upscale(normalized, enlarged, o, before["width"], before["height"], run, cancelled)
            checked_upscale = probe(enlarged, cancelled)
            expected = (before["width"] * o["upscaleScale"], before["height"] * o["upscaleScale"], 1)
            actual = (checked_upscale["width"], checked_upscale["height"], checked_upscale["frames"])
            if actual != expected:
                raise RuntimeError(f"upscaler returned {actual}; expected {expected}; source kept")
            if before["alpha"] and not checked_upscale["alpha"]:
                raise RuntimeError("upscaler discarded transparency; source kept")
            decoded, before = enlarged, checked_upscale
        high_depth = any(bit in before["pixelFormat"] for bit in ("10", "12", "16"))
        if high_depth and encoder.startswith("h264_"):
            if o["hardware"] == "auto" and cap["cpu"]:
                encoder = cap["cpu"][0]
            else:
                raise ValueError("this hardware H.264 encoder cannot preserve high bit depth; choose HEVC/AV1 or CPU")
        direct_webp_animation = False
        if encoder == "cjxl":
            intermediate = decoded
            # cjxl's native GIF reader preserves GIF centisecond timing. Routing an
            # infinite-loop GIF through APNG can produce slow-motion JXL timing on
            # some libjxl versions, so keep GIF native whenever loop semantics do
            # not need normalization. Finite GIF loops still use APNG because GIF
            # stores repeat counts while APNG/JXL store total plays.
            direct_gif = before["videoCodec"] == "gif" and before.get("plays", 0) == 0
            # Normalize repeat semantics to APNG's total play count for formats
            # that still need the intermediate.
            if not direct_gif and before["videoCodec"] not in ("mjpeg", "png", "apng"):
                intermediate = directory / "intermediate.png"
                intermediate_args = ffmpeg_prefix() + input_args(decoded) + ["-an",
                    "-fps_mode", "passthrough", "-enc_time_base", "-1", "-c:v", "apng" if before["frames"] > 1 else "png",
                    "-f", "apng" if before["frames"] > 1 else "image2"]
                if before["frames"] > 1:
                    intermediate_args += ["-plays", str(before.get("plays", 0))]
                    if before.get("durations"):
                        intermediate_args += ["-final_delay", str(Fraction(before["durations"][-1]).limit_denominator(100000))]
                run(intermediate_args + [str(intermediate)], cancelled)
            if before.get("durations") and not direct_gif:
                intermediate = jxl_timing_input(intermediate, directory / "jxl-timed.png", before["durations"])
            args = jxl_encoder_args(intermediate, output, o)
            if before["videoCodec"] == "mjpeg" and o["distance"] != 0:
                args.append("--lossless_jpeg=0")
        elif (fmt == "webp" and before["frames"] > 1 and before.get("durations") and
              encoder == "libwebp_anim" and pillow_webp_available()):
            # libwebp_anim may coalesce repeated frames and can assign an extra
            # nominal hold to the last frame. Encode still frames separately,
            # then mux exact frame durations into the animation container.
            direct_webp_animation = True
            args = []
        else:
            args = ffmpeg_prefix() + ["-n"] + device_args(encoder) + input_args(decoded) + [
                "-map", "[converted]" if fmt == "gif" else "0:v:0", "-map_metadata", "0", "-map_chapters", "0", "-fps_mode", "passthrough",
                "-threads", str(THREADS), "-c:v", encoder]
            if family == "video":
                args += video_quality(encoder, o)
                if before.get("durations"):
                    if fmt in ("h264", "hevc", "mov"):
                        # Reordered B frames can put the final display frame
                        # outside a short animation's MP4 edit list.
                        args += ["-bf", "0"]
                    last = before["durations"][-1]
                    start = before["duration"] - last
                    # Preserve the last frame's hold time when the encoder uses
                    # a nominal frame rate for packet duration (notably AV1).
                    args += ["-bsf:v", f"setts=duration='if(gte(PTS*TB,{start - 0.000001}),{last}/TB,DURATION)'"]
                if encoder.endswith("_vaapi"):
                    args += ["-vf", "format=" + ("p010le" if high_depth else "nv12") + ",hwupload"]
                else:
                    pix = "p010le" if encoder.endswith(("_qsv", "_nvenc")) else "yuv420p10le"
                    args += ["-pix_fmt", pix if high_depth else "yuv420p"]
                for option, key in (("-color_trc", "colorTransfer"), ("-color_primaries", "colorPrimaries"), ("-colorspace", "colorSpace")):
                    if before[key] and before[key] not in ("unknown", "unspecified", "gbr", "rgb"):
                        args += [option, before[key]]
                if not o["dropAudio"]:
                    args += ["-map", "0:a?", "-c:a", "libopus" if ext == "webm" else "aac", "-b:a", "192k"]
                if ext in ("mp4", "mov"):
                    args += ["-movflags", "+faststart"]
                if fmt == "hevc":
                    args += ["-tag:v", "hvc1"]
            else:
                args += ["-an"]
                if family == "image":
                    args += ["-frames:v", "1"]
                if fmt == "jxl":
                    args += ["-distance", str(o["distance"]), "-effort", str(int(o["effort"]))]
                elif fmt == "jpeg":
                    args += ["-q:v", str(round(2 + (100 - o["quality"]) * 29 / 100))]
                elif fmt in ("png", "apng"):
                    args += ["-compression_level", str(int(o["effort"]))]
                elif fmt == "webp":
                    args += ["-quality", str(o["quality"]), "-compression_level", str(round(o["effort"] * 6 / 9)),
                             "-lossless", "1" if o["lossless"] else "0", "-loop", str(before.get("plays", 0))]
                elif fmt == "avif":
                    args += video_quality(encoder, o) + ["-still-picture", "1"]
                elif fmt == "gif":
                    plays = before.get("plays", 0)
                    args += ["-filter_complex", "[0:v]split[a][b];[a]palettegen=reserve_transparent=1[p];[b][p]paletteuse[converted]",
                             "-loop", str(0 if plays == 0 else -1 if plays == 1 else plays - 1)]
                if fmt == "apng":
                    args += ["-f", "apng", "-plays", str(before.get("plays", 0))]
                    if before.get("durations"):
                        args += ["-final_delay", str(Fraction(before["durations"][-1]).limit_denominator(100000))]
            args += [str(output)]
        try:
            if direct_webp_animation:
                animated_webp_encode(decoded, source, output, directory / "webp-output-frame.webp",
                                      o, before["durations"], before["plays"], cancelled)
            else:
                run(args, cancelled)
            if fmt == "webp" and before.get("durations") and not direct_webp_animation:
                webp_final_duration(output, before["durations"][-1])
            # A successful process exit alone does not prove that animation survived.
            check_dir = directory / "check"
            check_dir.mkdir()
            checked = prepare_input(output, check_dir, cancelled, known_single_frame=before["frames"] == 1)
            after = probe(checked, cancelled)
            run(ffmpeg_prefix() + ["-xerror", "-protocol_whitelist", "file,pipe", "-format_whitelist", INPUT_FORMATS,
                "-i", str(checked), "-map", "0:v:0", "-map", "0:a?", "-f", "null", "-"], cancelled)
            if (before["width"], before["height"], before["frames"]) != (after["width"], after["height"], after["frames"]):
                raise RuntimeError(f"verification failed: dimensions or frame count changed "
                                   f"({before['width']}x{before['height']}/{before['frames']} → "
                                   f"{after['width']}x{after['height']}/{after['frames']}; "
                                   f"{fmt}/{encoder}); source kept")
            if before["frames"] > 1 and abs(before["duration"] - after["duration"]) > max(0.025, before["duration"] * 0.001):
                raise RuntimeError(f"verification failed: animation/video duration changed "
                                   f"({before['duration']:.6f}s → {after['duration']:.6f}s; "
                                   f"{before['frames']} frames, {fmt}/{encoder}); source kept")
            if before.get("durations") and after.get("durations"):
                if any(abs(a - b) > 0.011 for a, b in zip(before["durations"], after["durations"])):
                    raise RuntimeError("verification failed: individual frame delays changed")
                if before["plays"] != after["plays"]:
                    raise RuntimeError("verification failed: animation loop count changed")
            if family == "video" and not o["dropAudio"] and before["audioStreams"] != after["audioStreams"]:
                raise RuntimeError("verification failed: audio stream missing")
            codec = "jpegxl" if fmt in ("jxl", "ajxl") else "webp" if fmt == "webp" else after["videoCodec"]
            after.update({"format": ext, "videoCodec": codec,
                          "encoder": encoder, "upscaler": o["upscaler"], "seconds": time.monotonic() - started, "size": output.stat().st_size})
            after.pop("durations", None)  # Binary response metadata must fit HTTP headers.
            return after
        except BaseException as error:
            output.unlink(missing_ok=True)
            if o["hardware"] == "auto" and encoder in cap["gpu"] and isinstance(error, RuntimeError) and not (cancelled and cancelled()):
                return convert(source, output, {**o, "hardware": "cpu"}, cancelled)
            raise


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("capabilities", "convert"))
    parser.add_argument("--input", type=Path)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--options", default="{}")
    args = parser.parse_args()
    try:
        result = capabilities() if args.operation == "capabilities" else convert(args.input, args.output, json.loads(args.options))
        print(json.dumps(result, allow_nan=False))
    except Exception as error:
        parser.exit(1, str(error) + "\n")


if __name__ == "__main__":
    main()
