"""Optional still-image upscalers, invoked through the converter's restore flow.

Executables, models and Python environments are configured by the worker owner,
never by an HTTP request. This module does not download models or import Torch.
"""
from __future__ import annotations

import os
from pathlib import Path
import shutil
import sys


WAIFU2X_MODEL_DIR_NAMES = (
    "models-cunet",
    "models-upconv_7_anime_style_art_rgb",
    "models-upconv_7_photo",
)


def configuration() -> dict:
    waifu = shutil.which(os.environ.get("STASH_WAIFU2X", "waifu2x-ncnn-vulkan"))
    waifu_models = Path(os.environ.get("STASH_WAIFU2X_MODELS", str(Path(waifu).parent / "models-cunet") if waifu else ""))
    seed_cli = Path(os.environ.get("STASH_SEEDVR2_CLI", ""))
    seed_models = Path(os.environ.get("STASH_SEEDVR2_MODELS", ""))
    seed_model = os.environ.get("STASH_SEEDVR2_MODEL", "seedvr2_ema_3b_fp8_e4m3fn.safetensors")
    return {"waifu": waifu, "waifu_models": waifu_models, "seed_cli": seed_cli,
            "seed_models": seed_models, "seed_model": seed_model,
            "seed_python": os.environ.get("STASH_SEEDVR2_PYTHON", sys.executable)}


def _waifu2x_model_dir_supported(path: Path) -> bool:
    value = str(path)
    return any(name in value for name in WAIFU2X_MODEL_DIR_NAMES)


def _waifu2x_vulkan_init_error(error: RuntimeError) -> bool:
    message = str(error).lower()
    return "vkcreateinstance failed" in message or "vk_error_incompatible_driver" in message


def _waifu2x_vulkan_help(error: RuntimeError) -> RuntimeError:
    return RuntimeError(
        f"{error}\nwaifu2x could not initialize Vulkan. Upstream waifu2x-ncnn-vulkan initializes Vulkan "
        "even for -g -1 CPU mode, so CPU fallback cannot bypass this error. In Docker, expose the GPU "
        "through the NVIDIA Container Toolkit (or the appropriate host GPU runtime), include the graphics "
        "driver capability, and make a compatible Vulkan ICD/loader visible inside the container."
    )


def capabilities() -> list[dict]:
    c = configuration()
    model_files = any(c["waifu_models"].glob("*.param")) and any(c["waifu_models"].glob("*.bin"))
    waifu = bool(c["waifu"] and model_files and _waifu2x_model_dir_supported(c["waifu_models"]))
    seed = bool(c["seed_cli"].is_file() and (c["seed_models"] / c["seed_model"]).is_file()
                and (c["seed_models"] / "ema_vae_fp16.safetensors").is_file())
    if c["waifu"] and model_files and not _waifu2x_model_dir_supported(c["waifu_models"]):
        waifu_notice = "Model directory name must contain models-cunet, models-upconv_7_anime_style_art_rgb, or models-upconv_7_photo."
    elif waifu:
        waifu_notice = "Still images; Vulkan GPU preferred with CPU processing fallback when Vulkan initializes successfully."
    else:
        waifu_notice = "Install waifu2x-ncnn-vulkan and configure a supported model directory."
    return [
        {"id": "waifu2x", "label": "waifu2x", "available": waifu, "cpu": True,
         "notice": waifu_notice},
        {"id": "seedvr2", "label": "SeedVR2", "available": seed, "cpu": False,
         "notice": "Still images; requires a working SeedVR2 GPU environment." if seed else "Configure the SeedVR2 CLI, Python environment and downloaded DiT/VAE models."},
    ]


def upscale(source: Path, output: Path, options: dict, width: int, height: int, run, cancelled=None) -> None:
    model, scale = options["upscaler"], options["upscaleScale"]
    c = configuration()
    cap = next((value for value in capabilities() if value["id"] == model), None)
    if not cap or not cap["available"]:
        raise ValueError(cap["notice"] if cap else "unknown upscaler")
    if model == "waifu2x":
        args = [c["waifu"], "-i", str(source), "-o", str(output), "-n", "-1", "-s", str(scale),
                "-m", str(c["waifu_models"].resolve()), "-t", "0", "-f", "png"]
        if options["hardware"] == "cpu":
            try:
                run(args + ["-g", "-1"], cancelled)
            except RuntimeError as error:
                if _waifu2x_vulkan_init_error(error):
                    raise _waifu2x_vulkan_help(error) from error
                raise
            return
        try:
            run(args, cancelled)
        except RuntimeError as gpu_error:
            if _waifu2x_vulkan_init_error(gpu_error):
                raise _waifu2x_vulkan_help(gpu_error) from gpu_error
            if options["hardware"] != "auto" or (cancelled and cancelled()):
                raise
            output.unlink(missing_ok=True)
            try:
                run(args + ["-g", "-1"], cancelled)
            except RuntimeError as cpu_error:
                if _waifu2x_vulkan_init_error(cpu_error):
                    raise _waifu2x_vulkan_help(cpu_error) from cpu_error
                raise RuntimeError(
                    f"waifu2x GPU attempt failed: {gpu_error}; CPU fallback failed: {cpu_error}"
                ) from cpu_error
        return
    if options["hardware"] == "cpu":
        raise ValueError("SeedVR2 requires GPU; choose Prefer GPU or GPU")
    blocks = int(os.environ.get("STASH_SEEDVR2_BLOCKS_TO_SWAP", "32"))
    args = [c["seed_python"], str(c["seed_cli"].resolve()), str(source), "--output", str(output),
            "--output_format", "png", "--model_dir", str(c["seed_models"].resolve()),
            "--dit_model", c["seed_model"], "--resolution", str(min(width, height) * scale),
            "--max_resolution", "0", "--batch_size", "1", "--seed", "42",
            "--dit_offload_device", "cpu", "--vae_offload_device", "cpu",
            "--blocks_to_swap", str(blocks), "--vae_encode_tiled", "--vae_decode_tiled"]
    # Upstream may auto-download missing weights. Require local models above and
    # force its Hugging Face client offline for this invocation.
    run(args, cancelled, offline_models=True)
