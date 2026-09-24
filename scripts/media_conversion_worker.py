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

# id: (label, extension, family, encoder candidates, controls)
FORMATS = {
    "jxl": ("JPEG XL", "jxl", "image", ["cjxl", "libjxl"], ["quality", "effort"]),
    "ajxl": ("Animated JPEG XL (AJXL)", "jxl", "animation", ["cjxl"], ["quality", "effort"]),
    "av1-mp4": ("AV1 / MP4", "mp4", "video", ["libsvtav1", "libaom-av1"], ["quality", "effort"]),
    "av1-mkv": ("AV1 / MKV", "mkv", "video", ["libsvtav1", "libaom-av1"], ["quality", "effort"]),
    "av1-webm": ("AV1 / WebM", "webm", "video", ["libsvtav1", "libaom-av1"], ["quality", "effort"]),
    "h264": ("H.264 / MP4", "mp4", "video", ["libx264"], ["quality", "effort"]),
    "hevc": ("HEVC / MP4", "mp4", "video", ["libx265"], ["quality", "effort"]),
    "vp9": ("VP9 / WebM", "webm", "video", ["libvpx-vp9"], ["quality", "effort"]),
    "mov": ("H.264 / MOV", "mov", "video", ["libx264"], ["quality", "effort"]),
    "jpeg": ("JPEG", "jpg", "image", ["mjpeg"], ["quality"]),
    "png": ("PNG", "png", "image", ["png"], ["effort"]),
    "webp": ("WebP (still or animated)", "webp", "animation", ["libwebp_anim", "libwebp"], ["quality", "effort", "lossless"]),
    "avif": ("AVIF", "avif", "image", ["libaom-av1"], ["quality", "effort"]),
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
                try:
                    os.killpg(proc.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            elif proc.poll() is None:
                if os.name == "nt" and cancelled is not None:
                    subprocess.run(["taskkill", "/PID", str(proc.pid), "/T", "/F"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
                else:
                    proc.kill()
            proc.wait()


def ffmpeg_prefix() -> list[str]:
    return [FFMPEG, "-nostdin", "-hide_banner", "-loglevel", "error", "-threads", str(THREADS),
            "-filter_threads", "1", "-filter_complex_threads", "1"]


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


def capabilities(probe_gpu=True, only_format=None) -> dict:
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
    formats = []
    for key, (label, ext, family, candidates, controls) in FORMATS.items():
        cpu = [e for e in candidates if e == "cjxl" and jxl_tools]
        cpu += [e for e in candidates if e != "cjxl" and e in encoders]
        hardware = gpu.get(key.split("-")[0], [])
        formats.append({"id": key, "label": label, "extension": ext, "family": family,
                        "cpu": cpu, "gpu": hardware, "controls": controls,
                        "available": bool(cpu or hardware)})
    value = {"formats": formats, "upscalers": upscaler.capabilities(), "version": 2}
    if probe_gpu and only_format is None:
        _capabilities_cache = time.monotonic(), value
    return value


def options(raw: dict) -> dict:
    if not isinstance(raw, dict) or set(raw) - {"format", "hardware", "quality", "effort", "distance", "lossless", "allowLarger", "dropAudio", "allowAlphaLoss", "upscaler", "upscaleScale"}:
        raise ValueError("unknown conversion option")
    o = {"format": "jxl", "hardware": "auto", "quality": 90 if raw.get("format", "jxl") in ("jxl", "ajxl") else 80, "effort": 7, "distance": 1,
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
    for name in ("lossless", "allowLarger", "dropAudio", "allowAlphaLoss"):
        if not isinstance(o[name], bool):
            raise ValueError(f"{name} must be a boolean")
    if o["format"] in ("jxl", "ajxl") and "distance" not in raw:
        q = o["quality"]
        # libjxl's JxlEncoderDistanceFromQuality. Keep explicit legacy distance
        # requests working, including jobs sent by older StashBooru servers.
        o["distance"] = 0 if q >= 100 else 0.1 + (100 - q) * 0.09 if q >= 30 else 53 / 3000 * q * q - 23 / 20 * q + 25
    return o


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
    # Older FFmpeg cannot decode animated WebP. Pillow coalesces disposal/blend
    # correctly; enforce a decoded-memory budget before collecting frames.
    if signature[:4] == b"RIFF" and signature[8:] == b"WEBP":
        from PIL import Image
        with Image.open(source) as im:
            count = getattr(im, "n_frames", 1)
            if count > 1:
                if im.width * im.height * count * 4 > 512 * 1024 * 1024:
                    raise ValueError("animated WebP exceeds the 512 MiB decoded-frame budget")
                frames, durations = [], []
                for index in range(count):
                    im.seek(index)
                    frames.append(im.convert("RGBA"))
                    durations.append(im.info.get("duration", 100))
                decoded = directory / "decoded.png"
                frames[0].save(decoded, format="PNG", save_all=True, append_images=frames[1:],
                               duration=durations, loop=im.info.get("loop", 0))
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
            durations.append(float(im.info.get("duration", 0)) / 1000)
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
    if encoder in ("libaom-av1", "libvpx-vp9"):
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
    encoding_hardware = "cpu" if o["upscaler"] and family != "video" else o["hardware"]
    cap = next(f for f in capabilities(encoding_hardware != "cpu", fmt)["formats"] if f["id"] == fmt)
    candidates = cap["gpu"] if encoding_hardware == "gpu" else cap["cpu"]
    if encoding_hardware == "auto":
        candidates = cap["gpu"] or cap["cpu"]
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
        if encoder == "cjxl":
            intermediate = decoded
            # Normalize GIF repeat semantics to APNG's total play count before
            # cjxl; older libjxl treats GIF repetitions as total plays.
            if before["videoCodec"] not in ("mjpeg", "png", "apng"):
                intermediate = directory / "intermediate.png"
                intermediate_args = ffmpeg_prefix() + input_args(decoded) + ["-an",
                    "-fps_mode", "passthrough", "-enc_time_base", "-1", "-c:v", "apng" if before["frames"] > 1 else "png",
                    "-f", "apng" if before["frames"] > 1 else "image2"]
                if before["frames"] > 1:
                    intermediate_args += ["-plays", str(before.get("plays", 0))]
                    if before.get("durations"):
                        intermediate_args += ["-final_delay", str(Fraction(before["durations"][-1]).limit_denominator(100000))]
                run(intermediate_args + [str(intermediate)], cancelled)
            if before.get("durations"):
                intermediate = jxl_timing_input(intermediate, directory / "jxl-timed.png", before["durations"])
            args = ["cjxl", str(intermediate), str(output), "--distance=" + str(o["distance"]),
                    "--effort=" + str(int(o["effort"])), "--num_threads=" + str(THREADS)]
            if before["videoCodec"] == "mjpeg" and o["distance"] != 0:
                args.append("--lossless_jpeg=0")
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
            run(args, cancelled)
            if fmt == "webp" and before.get("durations"):
                webp_final_duration(output, before["durations"][-1])
            # A successful process exit alone does not prove that animation survived.
            check_dir = directory / "check"
            check_dir.mkdir()
            checked = prepare_input(output, check_dir, cancelled, known_single_frame=before["frames"] == 1)
            after = probe(checked, cancelled)
            run(ffmpeg_prefix() + ["-xerror", "-protocol_whitelist", "file,pipe", "-format_whitelist", INPUT_FORMATS,
                "-i", str(checked), "-map", "0:v:0", "-map", "0:a?", "-f", "null", "-"], cancelled)
            if (before["width"], before["height"], before["frames"]) != (after["width"], after["height"], after["frames"]):
                raise RuntimeError("verification failed: dimensions or frame count changed")
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
            after.update({"format": ext, "videoCodec": "jpegxl" if fmt in ("jxl", "ajxl") else after["videoCodec"],
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
