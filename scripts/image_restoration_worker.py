#!/usr/bin/env python3
"""Bounded, offline SD1.5 inpainting adapter; owns no catalogue or model downloads."""
from __future__ import annotations

import argparse
import hashlib
import importlib.util
import importlib.metadata
import io
import json
import os
from pathlib import Path
import re
import sys
import time
import zipfile

PROTOCOL = 1
MAX_BYTES = 96 * 1024 * 1024
MAX_PIXELS = 16 * 1024 * 1024
MODEL_ID = "stable-diffusion-v1-5/stable-diffusion-inpainting"
ADAPTER_SHA256 = hashlib.sha256(Path(__file__).read_bytes()).digest()
NOTICE = ("Install Pillow, torch, diffusers, transformers, accelerate and safetensors in the worker Python; "
          "explicitly install the SD1.5 inpainting Diffusers snapshot (including safety_checker), then set "
          "STASH_RESTORATION_MODEL_DIR and STASH_RESTORATION_REVISION to its pinned commit. Include the fp16 safetensors variant for all components. No automatic download.")


def capabilities():
    directory = Path(os.environ.get("STASH_RESTORATION_MODEL_DIR", ""))
    revision = os.environ.get("STASH_RESTORATION_REVISION", "")
    installed = bool(os.environ.get("STASH_RESTORATION_MODEL_DIR")) and (directory / "model_index.json").is_file()
    if installed:
        try:
            index = json.loads((directory / "model_index.json").read_text())
            unet = json.loads((directory / "unet" / "config.json").read_text())
            installed = index.get("_class_name") == "StableDiffusionInpaintPipeline" and unet.get("in_channels") == 9
            for component in ("unet", "vae", "text_encoder", "safety_checker"):
                installed = installed and bool(list((directory / component).glob("*.fp16.safetensors")))
        except (OSError, ValueError):
            installed = False
    deps = all(importlib.util.find_spec(name) is not None for name in
               ("PIL", "torch", "diffusers", "transformers", "accelerate", "safetensors"))
    gpu = False
    if deps:
        import torch
        gpu = bool(torch.cuda.is_available())
    cpu = os.environ.get("STASH_RESTORATION_ALLOW_CPU", "") == "1"
    valid_revision = bool(re.fullmatch(r"[a-f0-9]{40}", revision))
    signature = ""
    if installed and valid_revision:
        digest = hashlib.sha256((MODEL_ID + revision + str(PROTOCOL)).encode())
        digest.update(ADAPTER_SHA256)
        for dependency in ("Pillow", "torch", "diffusers", "transformers", "accelerate", "safetensors"):
            try:
                digest.update((dependency + importlib.metadata.version(dependency)).encode())
            except importlib.metadata.PackageNotFoundError:
                digest.update((dependency + ":missing").encode())
        for path in sorted(directory.rglob("*")):
            if path.is_file():
                stat = path.stat()
                digest.update(f"{path.relative_to(directory)}:{stat.st_size}:{stat.st_mtime_ns}".encode())
        signature = digest.hexdigest()
    available = installed and deps and valid_revision and (gpu or cpu)
    return {"protocol": PROTOCOL, "model": MODEL_ID, "revision": revision,
            "signature": signature, "available": available, "cpu": cpu, "gpu": gpu,
            "maxPixels": MAX_PIXELS, "workSize": 512, "reference": "starting-pixels",
            "notice": "" if available else NOTICE + (" Enable STASH_RESTORATION_ALLOW_CPU=1 for slow CPU execution." if not gpu else "")}


def options(value):
    allowed = {"operation", "hardware", "prompt", "negativePrompt", "seed", "steps", "guidance", "strength", "signature"}
    if not isinstance(value, dict) or set(value) - allowed:
        raise ValueError("unknown restoration options")
    defaults = {"operation": "generate", "hardware": "auto", "prompt": "", "negativePrompt": "",
                "seed": 42, "steps": 20, "guidance": 7.5, "strength": 1.0, "signature": ""}
    defaults.update(value)
    if defaults["operation"] not in ("prepare", "generate") or defaults["hardware"] not in ("auto", "gpu", "cpu"):
        raise ValueError("invalid restoration operation/hardware")
    for key in ("prompt", "negativePrompt", "signature"):
        if not isinstance(defaults[key], str) or len(defaults[key]) > 2000:
            raise ValueError("restoration text is too long")
    for key, low, high in (("seed", 0, 2147483647), ("steps", 5, 50), ("guidance", 1, 15), ("strength", .05, 1)):
        v = defaults[key]
        if isinstance(v, bool) or not isinstance(v, (int, float)) or not low <= v <= high:
            raise ValueError(f"invalid {key}")
    if not isinstance(defaults["seed"], int) or not isinstance(defaults["steps"], int):
        raise ValueError("seed and steps must be integers")
    if int(defaults["steps"] * defaults["strength"]) < 1:
        raise ValueError("strength and steps must permit at least one diffusion step")
    return defaults


def read_image(data, mask=False):
    from PIL import Image, ImageOps
    Image.MAX_IMAGE_PIXELS = MAX_PIXELS
    with Image.open(io.BytesIO(data)) as image:
        if image.width * image.height > MAX_PIXELS or max(image.size) > 8192:
            raise ValueError("image exceeds 16 megapixels / 8192 pixels per side")
        if getattr(image, "n_frames", 1) != 1:
            raise ValueError("restoration supports still images only")
        if image.format not in ("PNG", "JPEG", "WEBP") or image.mode not in ("1", "L", "LA", "P", "RGB", "RGBA"):
            raise ValueError("use an 8-bit PNG, JPEG or WebP copy; high-bit-depth/CMYK/other formats are unsupported")
        # Pillow downconverts 16-bit RGB PNGs; reject these before silently losing precision.
        if image.format == "PNG" and len(data) > 24 and data[24] != 8:
            raise ValueError("only 8-bit PNG input is supported")
        image.load()
        if mask:
            if image.format != "PNG" or image.mode != "L":
                raise ValueError("mask must be an 8-bit grayscale PNG")
            return image.copy()
        normalized = ImageOps.exif_transpose(image).convert("RGBA")
        normalized.info = {"icc_profile": image.info["icc_profile"]} if image.info.get("icc_profile") else {}
        return normalized


def png_bytes(image):
    stream = io.BytesIO()
    image.save(stream, format="PNG", **image.info)
    return stream.getvalue()


def inpaint(source, mask, reference, opts, cancelled):
    from PIL import Image, ImageOps
    caps = capabilities()
    if not caps["available"]:
        raise ValueError(caps["notice"])
    import torch
    from diffusers import StableDiffusionInpaintPipeline
    if opts["signature"] != caps["signature"]:
        raise ValueError("installed model/revision changed; prepare a new restoration session")
    hardware = opts["hardware"]
    if hardware == "gpu" and not caps["gpu"]:
        raise ValueError("GPU-only execution requires a CUDA GPU on this worker")
    if hardware == "cpu" and not caps["cpu"]:
        raise ValueError("CPU execution is disabled; set STASH_RESTORATION_ALLOW_CPU=1 on the worker")
    device = "cuda" if hardware != "cpu" and caps["gpu"] else "cpu"
    if device == "cpu" and not caps["cpu"]:
        raise ValueError("this worker does not support CPU restoration")
    started = time.monotonic()

    def check():
        if (cancelled and cancelled()) or time.monotonic() - started > 900:
            raise RuntimeError("restoration cancelled or exceeded 15-minute limit")

    def callback(_pipeline, _step, _timestep, kwargs):
        check()
        return kwargs

    check()
    pipeline = StableDiffusionInpaintPipeline.from_pretrained(
        os.environ["STASH_RESTORATION_MODEL_DIR"], local_files_only=True, use_safetensors=True, variant="fp16",
        torch_dtype=torch.float16 if device == "cuda" else torch.float32)
    if pipeline.unet.config.in_channels != 9 or pipeline.safety_checker is None:
        raise ValueError("install the dedicated 9-channel inpainting model with its safety checker")
    pipeline.to(device)
    pipeline.enable_attention_slicing()
    pipeline.enable_vae_slicing()
    pipeline.set_progress_bar_config(disable=True)
    check()
    box = mask.getbbox()
    if box is None:
        raise ValueError("paint a non-empty mask")
    left, top, right, bottom = box
    box = (max(0, left - 32), max(0, top - 32), min(source.width, right + 32), min(source.height, bottom + 32))
    original = source.crop(box).convert("RGB")
    crop_mask = mask.crop(box)
    if reference is not None:
        if opts["strength"] >= 1:
            raise ValueError("starting-pixel reference requires strength below 1")
        guide = ImageOps.fit(reference.convert("RGB"), original.size, method=Image.Resampling.LANCZOS)
        original = Image.composite(guide, original, crop_mask)
    ratio = min(512 / original.width, 512 / original.height)
    size = (max(1, round(original.width * ratio)), max(1, round(original.height * ratio)))
    offset = ((512 - size[0]) // 2, (512 - size[1]) // 2)
    canvas = Image.new("RGB", (512, 512))
    canvas.paste(original.resize(size, Image.Resampling.LANCZOS), offset)
    work_mask = Image.new("L", (512, 512))
    work_mask.paste(crop_mask.resize(size, Image.Resampling.NEAREST), offset)
    with torch.inference_mode():
        result = pipeline(prompt=opts["prompt"], negative_prompt=opts["negativePrompt"], image=canvas,
                          mask_image=work_mask, width=512, height=512,
                          num_inference_steps=opts["steps"], guidance_scale=opts["guidance"],
                          strength=opts["strength"], generator=torch.Generator(device=device).manual_seed(opts["seed"]),
                          callback_on_step_end=callback)
    check()
    if result.nsfw_content_detected is None or any(result.nsfw_content_detected):
        raise ValueError("restoration is limited to non-explicit imagery; this generated result was blocked")
    generated = result.images[0].crop((offset[0], offset[1], offset[0] + size[0], offset[1] + size[1]))
    generated = generated.resize(original.size, Image.Resampling.LANCZOS)
    result_canvas = source.convert("RGB")
    result_canvas.paste(generated, box)
    # The generator never decides unmasked pixels or alpha. This final blend is exact at mask=0.
    output = Image.composite(result_canvas, source.convert("RGB"), mask).convert("RGBA")
    output.putalpha(source.getchannel("A"))
    output.info = source.info.copy()
    del pipeline
    if device == "cuda":
        torch.cuda.empty_cache()
    return output, {"model": caps["model"], "revision": caps["revision"], "signature": caps["signature"],
                    "hardware": device, "crop": list(box), "workSize": [512, 512], "seconds": time.monotonic() - started}


def process(request_path, output_path, cancelled=None, generate=inpaint):
    if Path(request_path).stat().st_size > MAX_BYTES:
        raise ValueError("restoration upload exceeds 96 MiB")
    with zipfile.ZipFile(request_path) as archive:
        entries = archive.infolist()
        allowed = {"source", "mask.png", "reference", "options.json"}
        if len(entries) > 4 or len({e.filename for e in entries}) != len(entries) or any(e.filename not in allowed for e in entries):
            raise ValueError("invalid restoration bundle entries")
        if sum(e.file_size for e in entries) > MAX_BYTES or any(e.file_size > 64 * 1024 * 1024 for e in entries):
            raise ValueError("restoration bundle expands beyond limit")
        raw_options = archive.read("options.json")
        if len(raw_options) > 8192:
            raise ValueError("options too large")
        opts = options(json.loads(raw_options))
        source = read_image(archive.read("source"))
        receipt = {"protocol": PROTOCOL, "width": source.width, "height": source.height,
                   "normalization": "8-bit RGBA; orientation baked; ICC retained; other embedded metadata omitted"}
        if opts["operation"] == "prepare":
            image, name = source, "source.png"
        else:
            if cancelled and cancelled():
                raise RuntimeError("restoration cancelled")
            mask = read_image(archive.read("mask.png"), mask=True)
            if mask.size != source.size or mask.getbbox() is None:
                raise ValueError("mask must match source dimensions and contain painted pixels")
            reference = read_image(archive.read("reference")) if "reference" in archive.namelist() else None
            image, metadata = generate(source, mask, reference, opts, cancelled)
            receipt.update(metadata)
            name = "output.png"
        if cancelled and cancelled():
            raise RuntimeError("restoration cancelled")
        data = png_bytes(image)
        receipt["outputSHA256"] = hashlib.sha256(data).hexdigest()
        with zipfile.ZipFile(output_path, "x", compression=zipfile.ZIP_STORED) as out:
            out.writestr(name, data)
            out.writestr("receipt.json", json.dumps(receipt, allow_nan=False))
    return receipt


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=("capabilities", "process"))
    parser.add_argument("--input")
    parser.add_argument("--output")
    args = parser.parse_args()
    try:
        result = capabilities() if args.operation == "capabilities" else process(args.input, args.output)
        print(json.dumps(result, allow_nan=False))
        return 0
    except Exception as error:
        print(str(error), file=sys.stderr)
        if args.output:
            Path(args.output).unlink(missing_ok=True)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
