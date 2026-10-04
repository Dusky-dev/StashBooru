#!/usr/bin/env python3
"""Bounded PTS-based video sampling. No catalogue writes, models or downloads."""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import math
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

ALGORITHM = "pts-phash-dhash-rgb-bars-v1"
MAX_FRAMES = 3600
MAX_SECONDS = 24 * 3600
INPUT_FORMATS = "mov,matroska,webm,avi,mpegts,mpeg,mpegvideo,flv,ogg,asf,nut,rm,h264,hevc,av1,ivf"
FFMPEG = os.environ.get("STASH_CONVERTER_FFMPEG", "ffmpeg")
FFPROBE = os.environ.get("STASH_CONVERTER_FFPROBE", "ffprobe")
PTS = re.compile(rb"\bn:\s*\d+\s+pts:\s*-?\d+\s+pts_time:([-+\d.eE]+)")


def run(command, cancelled=None, limit=12 * 1024 * 1024, timeout=1800):
    # Unlinked spool files bound memory and disappear on process cancellation.
    with tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as errors:
        process = subprocess.Popen(command, stdout=output, stderr=errors, stdin=subprocess.DEVNULL)
        started = time.monotonic()
        try:
            while process.poll() is None:
                if cancelled and cancelled():
                    raise RuntimeError("video indexing cancelled")
                if time.monotonic() - started > timeout:
                    raise RuntimeError("video decoder exceeded its time limit")
                if os.fstat(output.fileno()).st_size > limit or os.fstat(errors.fileno()).st_size > 4 * 1024 * 1024:
                    raise RuntimeError("video decoder exceeded its output limit")
                time.sleep(.05)
            if cancelled and cancelled():
                raise RuntimeError("video indexing cancelled")
            if os.fstat(output.fileno()).st_size > limit or os.fstat(errors.fileno()).st_size > 4 * 1024 * 1024:
                raise RuntimeError("video decoder exceeded its output limit")
            output.seek(0)
            errors.seek(0)
            data, log = output.read(), errors.read()
            if process.returncode:
                raise RuntimeError("video decoder failed: " + log[-4096:].decode(errors="replace"))
            return data, log
        finally:
            if process.poll() is None:
                process.kill()
            process.wait()


def capabilities():
    versions = []
    for tool in (FFMPEG, FFPROBE):
        data, _ = run([tool, "-version"], limit=65536, timeout=15)
        versions.append(data.decode(errors="replace").strip())
    code = Path(__file__).read_bytes()
    signature = hashlib.sha256(code + "\n".join(versions).encode()).hexdigest()
    return {"algorithm": ALGORITHM, "signature": signature, "maxFrames": MAX_FRAMES,
            "decoder": versions[0].splitlines()[0], "processor": "CPU"}


def options(raw):
    step = float(raw.get("sampleSeconds", 1))
    if not math.isfinite(step) or not .5 <= step <= 10:
        raise ValueError("sample interval must be between 0.5 and 10 seconds")
    audio = raw.get("audioDigest", False)
    if not isinstance(audio, bool):
        raise ValueError("audioDigest must be boolean")
    return {"sampleSeconds": step, "audioDigest": audio}


def file_hash(source, cancelled=None):
    digest = hashlib.sha256()
    with source.open("rb") as stream:
        while data := stream.read(1024 * 1024):
            if cancelled and cancelled():
                raise RuntimeError("video indexing cancelled")
            digest.update(data)
    return digest.hexdigest()


def number(value, default=0):
    try:
        ret = float(value)
        return ret if math.isfinite(ret) else default
    except (ValueError, TypeError):
        return default


def sample(source: Path, raw: dict, cancelled=None):
    config = options(raw)
    before = source.stat()
    if not source.is_file() or before.st_size <= 0:
        raise ValueError("select a nonempty regular video file")
    digest = file_hash(source, cancelled)
    caps = capabilities()
    restriction = ["-protocol_whitelist", "file,pipe", "-format_whitelist", INPUT_FORMATS]
    data, _ = run([FFPROBE, "-v", "error", *restriction, "-show_format", "-show_streams",
                   "-of", "json", str(source)], cancelled, limit=1024 * 1024, timeout=60)
    probed = json.loads(data)
    videos = [s for s in probed.get("streams", []) if s.get("codec_type") == "video"
              and not s.get("disposition", {}).get("attached_pic")]
    if not videos:
        raise ValueError("no playable video stream")
    video = videos[0]
    duration = number(probed.get("format", {}).get("duration"), number(video.get("duration")))
    if not 0 < duration <= MAX_SECONDS:
        raise ValueError("video duration must be known and at most 24 hours")
    step = max(config["sampleSeconds"], duration / (MAX_FRAMES - 1))
    # Select existing decoded frames by their PTS. Do not synthesize CFR frames.
    # copyts/start_at_zero retains offsets relative to the playable container.
    filters = (f"select='isnan(prev_selected_t)+gte(t-prev_selected_t,{step:.9f})',"
               "scale=32:32:force_original_aspect_ratio=decrease:flags=area,"
               "pad=32:32:(ow-iw)/2:(oh-ih)/2,setsar=1,format=rgb24,showinfo")
    data, log = run([FFMPEG, "-hide_banner", "-nostdin", "-nostats", "-loglevel", "info",
                     "-threads", "2", "-filter_threads", "1", "-copyts", "-start_at_zero",
                     *restriction, "-i", str(source), "-map", f"0:{video['index']}", "-an", "-sn",
                     "-vf", filters, "-fps_mode", "passthrough", "-frames:v", str(MAX_FRAMES),
                     "-threads", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1"], cancelled)
    frame_bytes = 32 * 32 * 3
    times = [float(m) for m in PTS.findall(log)]
    count = len(data) // frame_bytes
    if not count or len(data) % frame_bytes or len(times) != count or count > MAX_FRAMES:
        raise RuntimeError("decoder did not return one presentation timestamp per sample")
    if any(not math.isfinite(t) or t < 0 or t > duration + 1 for t in times) or any(a >= b for a, b in zip(times, times[1:])):
        raise RuntimeError("video has invalid or discontinuous presentation timestamps")
    audio = [s for s in probed["streams"] if s.get("codec_type") == "audio"]
    subtitles = [s for s in probed["streams"] if s.get("codec_type") == "subtitle"]
    rotation = int(number(video.get("tags", {}).get("rotate")))
    for side in video.get("side_data_list", []):
        rotation = int(number(side.get("rotation"), rotation))
    width, height = int(video.get("width", 0)), int(video.get("height", 0))
    if abs(rotation) % 180 == 90:
        width, height = height, width
    media = {"duration": duration, "width": width, "height": height, "codec": video.get("codec_name", ""),
             "bitRate": int(number(video.get("bit_rate"), number(probed.get("format", {}).get("bit_rate")))),
             "rotation": rotation, "audioTracks": len(audio), "subtitleTracks": len(subtitles),
             "audioCodecs": [s.get("codec_name", "") for s in audio]}
    if config["audioDigest"] and audio:
        # Supporting whole-track evidence, deliberately not an audio similarity
        # score. A trim, delay, mix or re-encode may change this digest.
        decoded, _ = run([FFMPEG, "-v", "error", "-nostdin", "-threads", "2", *restriction,
                           "-i", str(source), "-map", f"0:{audio[0]['index']}", "-vn", "-sn", "-ac", "1",
                           "-ar", "8000", "-t", str(duration), "-c:a", "pcm_s16le", "-f", "hash",
                           "-hash", "sha256", "pipe:1"], cancelled, limit=4096)
        media["audioSHA256"] = decoded.decode().strip().split("=", 1)[1]
    after = source.stat()
    if (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns) or digest != file_hash(source, cancelled):
        raise RuntimeError("source changed during indexing; retry after scanning")
    return {"algorithm": ALGORITHM, "decoder": caps["signature"], "sha256": digest, "step": step, "media": media,
            "frames": [{"time": t, "rgb": base64.b64encode(data[i * frame_bytes:(i + 1) * frame_bytes]).decode()}
                       for i, t in enumerate(times)]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("capabilities", "sample"))
    parser.add_argument("--input", type=Path)
    parser.add_argument("--options", default="{}")
    args = parser.parse_args()
    try:
        result = capabilities() if args.operation == "capabilities" else sample(args.input, json.loads(args.options))
        print(json.dumps(result, separators=(",", ":"), allow_nan=False))
        return 0
    except Exception as error:
        import sys
        print(str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
