#!/usr/bin/env python3
"""Generate labelled, synthetic real-codec P10 fixtures; optionally test in Go."""

from pathlib import Path
import argparse
import json
import os
import random
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def frame(seed):
    rng = random.Random(seed)
    palette = [[rng.randrange(30, 230) for _ in range(3)] for _ in range(16 * 12)]
    return bytes(channel for y in range(96) for x in range(128)
                 for channel in palette[(y // 8) * 16 + x // 8])


def encode(path, seeds, audio=None):
    command = ["ffmpeg", "-v", "error", "-y", "-threads", "2", "-f", "rawvideo", "-pix_fmt", "rgb24",
               "-s", "128x96", "-r", "12", "-i", "pipe:0"]
    if audio:
        command += ["-f", "lavfi", "-i", f"sine=frequency={audio}:sample_rate=48000:duration={len(seeds)}"]
    command += ["-c:v", "libx264", "-threads", "2", "-preset", "ultrafast", "-crf", "18", "-pix_fmt", "yuv420p"]
    if audio:
        command += ["-c:a", "aac", "-b:a", "96k", "-shortest"]
    command += [str(path)]
    with subprocess.Popen(command, stdin=subprocess.PIPE, stderr=subprocess.PIPE) as process:
        for seed in seeds:
            data = frame(seed) if seed >= 0 else bytes(128 * 96 * 3)
            process.stdin.write(data * 12)
        process.stdin.close()
        error = process.stderr.read().decode(errors="replace")
        if process.wait():
            raise RuntimeError(error)


def transcode(source, output, *args):
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-threads", "2", "-i", str(source), *args,
                    "-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "26",
                    "-pix_fmt", "yuv420p", str(output)], check=True)


def generate(root):
    root = Path(root)
    root.mkdir(parents=True, exist_ok=True)
    seeds = list(range(100, 124))
    encode(root / "base.mp4", seeds, audio=440)
    (root / "exact.mp4").write_bytes((root / "base.mp4").read_bytes())
    transcode(root / "base.mp4", root / "reencode.mp4", "-c:a", "copy")
    transcode(root / "base.mp4", root / "resolution.mp4", "-vf", "scale=64:48", "-c:a", "copy")
    transcode(root / "base.mp4", root / "bars.mp4", "-vf", "scale=144:108,pad=192:108:24:0:black", "-c:a", "copy")
    encode(root / "trim.mp4", seeds[5:19])
    encode(root / "intro.mp4", list(range(300, 308)) + seeds)
    encode(root / "clip.mp4", seeds[6:14])
    encode(root / "compilation.mp4", list(range(400, 415)) + seeds[6:14] + list(range(600, 613)))
    encode(root / "reordered.mp4", seeds[12:] + seeds[:12])
    encode(root / "partial.mp4", seeds[12:] + list(range(500, 512)))
    encode(root / "audio.mp4", seeds, audio=880)
    encode(root / "title_a.mp4", [999] * 8 + seeds[:16])
    encode(root / "title_b.mp4", [999] * 8 + list(range(700, 716)))
    encode(root / "black.mp4", [-1] * 24)
    transcode(root / "base.mp4", root / "vfr.mp4", "-vf", "select='not(eq(mod(n,5),0))'", "-fps_mode", "vfr", "-c:a", "copy")
    transcode(root / "base.mp4", root / "speed.mp4", "-vf", "setpts=PTS/1.5", "-an")
    cases = [
        {"name": "exact", "a": "base.mp4", "b": "exact.mp4", "class": "exact-file"},
        {"name": "identical reencode", "a": "base.mp4", "b": "reencode.mp4", "class": "near-complete-visual"},
        {"name": "resolution", "a": "base.mp4", "b": "resolution.mp4", "class": "near-complete-visual"},
        {"name": "black bars", "a": "base.mp4", "b": "bars.mp4", "class": "near-complete-visual"},
        {"name": "trim start/end", "a": "base.mp4", "b": "trim.mp4", "class": "contained-clip", "offset": -5},
        {"name": "inserted intro", "a": "base.mp4", "b": "intro.mp4", "class": "contained-clip", "offset": 8},
        {"name": "short in long compilation", "a": "clip.mp4", "b": "compilation.mp4", "class": "contained-clip", "offset": 15},
        {"name": "reordered compilation", "a": "base.mp4", "b": "reordered.mp4", "class": "compilation-segments", "intervals": 2},
        {"name": "partial overlap", "a": "base.mp4", "b": "partial.mp4", "class": "partial-overlap", "offset": -12},
        {"name": "different audio", "a": "base.mp4", "b": "audio.mp4", "class": "near-complete-visual", "audioDiff": True},
        {"name": "unrelated shared title", "a": "title_a.mp4", "b": "title_b.mp4", "class": ""},
        {"name": "black", "a": "base.mp4", "b": "black.mp4", "class": ""},
        {"name": "variable frame rate", "a": "base.mp4", "b": "vfr.mp4", "class": "near-complete-visual"},
        {"name": "optional speed alteration", "a": "base.mp4", "b": "speed.mp4", "class": ""},
    ]
    (root / "cases.json").write_text(json.dumps(cases, indent=2))
    return cases


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--test", action="store_true")
    args = parser.parse_args()
    if args.test:
        with tempfile.TemporaryDirectory(prefix="stash-p10-codecs-") as temporary:
            generate(temporary)
            env = {**os.environ, "STASH_VIDEO_OVERLAP_FIXTURES": temporary,
                   "STASH_VIDEO_OVERLAP_WORKER": str(ROOT / "scripts/video_overlap_worker.py")}
            return subprocess.run(["go", "test", "./pkg/videooverlap", "-run", "TestRealVideoIntervals", "-v", "-count=1"], cwd=ROOT, env=env).returncode
    if args.output is None:
        parser.error("use --output or --test")
    generate(args.output)
    print(f"Generated 14 labelled cases in {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
