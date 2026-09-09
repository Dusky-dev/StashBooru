#!/usr/bin/env python3
"""Persistent visual embedding worker for StashBooru.

Protocol: one JSON object per stdin line, one JSON object per stdout line.
Logs and download progress are written to stderr so stdout stays machine-readable.
"""

from __future__ import annotations

import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import urllib.request
from typing import Any

import numpy as np
import onnxruntime as ort
from PIL import Image, ImageOps

MODEL_ID = "deepghs/wd14_tagger_with_embeddings:SmilingWolf/wd-eva02-large-tagger-v3"
MODEL_REVISION = "02fcdebd8afb52d5697a91efa4ca1c522b632581"
MODEL_URL = (
    "https://huggingface.co/deepghs/wd14_tagger_with_embeddings/resolve/"
    f"{MODEL_REVISION}/SmilingWolf/wd-eva02-large-tagger-v3/model.onnx?download=true"
)
MODEL_SHA256 = "983f026df23214ba55f78cd5898f76f434fa9230837a4e93f1aaa6c37e284835"
EMBEDDING_DIMENSIONS = 1024
DOWNLOAD_CHUNK = 8 * 1024 * 1024

_session: ort.InferenceSession | None = None
_input_name: str | None = None
_target_size: int | None = None


def _log(message: str) -> None:
    print(f"[visual-embedding] {message}", file=sys.stderr, flush=True)


def _default_cache_dir() -> Path:
    configured = os.environ.get("STASH_EMBEDDING_CACHE_DIR")
    if configured:
        return Path(configured).expanduser()

    stash_config = os.environ.get("STASH_CONFIG_FILE")
    if stash_config:
        return Path(stash_config).expanduser().resolve().parent / "cache" / "visual-embeddings"

    xdg_cache = os.environ.get("XDG_CACHE_HOME")
    if xdg_cache:
        return Path(xdg_cache).expanduser() / "stashbooru" / "visual-embeddings"

    return Path.home() / ".cache" / "stashbooru" / "visual-embeddings"


def _model_path() -> Path:
    override = os.environ.get("STASH_EMBEDDING_MODEL_PATH")
    if override:
        return Path(override).expanduser()
    return _default_cache_dir() / "wd-eva02-large-tagger-v3-embedding.onnx"


def _sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while chunk := handle.read(DOWNLOAD_CHUNK):
            digest.update(chunk)
    return digest.hexdigest()


def _download_model(destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    fd, temp_name = tempfile.mkstemp(prefix=destination.name + ".", suffix=".part", dir=destination.parent)
    os.close(fd)
    temp_path = Path(temp_name)

    try:
        _log(f"downloading EVA02-Large embedding model to {destination}")
        request = urllib.request.Request(
            MODEL_URL,
            headers={"User-Agent": "StashBooru visual-embedding-worker/1"},
        )
        with urllib.request.urlopen(request, timeout=60) as response, temp_path.open("wb") as output:
            expected = response.headers.get("Content-Length")
            expected_bytes = int(expected) if expected and expected.isdigit() else None
            downloaded = 0
            next_report = 256 * 1024 * 1024
            while True:
                chunk = response.read(DOWNLOAD_CHUNK)
                if not chunk:
                    break
                output.write(chunk)
                downloaded += len(chunk)
                if downloaded >= next_report:
                    if expected_bytes:
                        _log(f"model download: {downloaded / 1024**3:.2f}/{expected_bytes / 1024**3:.2f} GiB")
                    else:
                        _log(f"model download: {downloaded / 1024**3:.2f} GiB")
                    next_report += 256 * 1024 * 1024

        actual_hash = _sha256(temp_path)
        if actual_hash != MODEL_SHA256:
            raise RuntimeError(
                f"downloaded model checksum mismatch: expected {MODEL_SHA256}, got {actual_hash}"
            )
        os.replace(temp_path, destination)
        _log("model download complete")
    finally:
        try:
            temp_path.unlink()
        except FileNotFoundError:
            pass


def _ensure_model() -> Path:
    path = _model_path()
    if path.exists():
        if os.environ.get("STASH_EMBEDDING_VERIFY_MODEL", "0") == "1":
            actual_hash = _sha256(path)
            if actual_hash != MODEL_SHA256:
                _log("cached model checksum is invalid; downloading a clean copy")
                path.unlink()
            else:
                return path
        else:
            return path

    _download_model(path)
    return path


def _providers() -> list[str] | None:
    requested = os.environ.get("STASH_EMBEDDING_ONNX_PROVIDERS", "").strip()
    if not requested:
        return None
    return [item.strip() for item in requested.split(",") if item.strip()]


def _load_session() -> tuple[ort.InferenceSession, str, int]:
    global _session, _input_name, _target_size
    if _session is not None and _input_name is not None and _target_size is not None:
        return _session, _input_name, _target_size

    model_path = _ensure_model()
    session_options = ort.SessionOptions()
    session_options.log_severity_level = 3
    providers = _providers()
    _log(f"loading {MODEL_ID} from {model_path}")
    _session = ort.InferenceSession(
        str(model_path),
        sess_options=session_options,
        providers=providers,
    ) if providers else ort.InferenceSession(str(model_path), sess_options=session_options)

    inputs = _session.get_inputs()
    if len(inputs) != 1:
        raise RuntimeError(f"expected one ONNX input, got {len(inputs)}")
    model_input = inputs[0]
    shape = model_input.shape
    if len(shape) != 4:
        raise RuntimeError(f"unexpected ONNX input shape: {shape}")

    height, width = shape[1], shape[2]
    if not isinstance(height, int) or not isinstance(width, int) or height <= 0 or height != width:
        raise RuntimeError(f"expected a fixed square NHWC input, got {shape}")

    _input_name = model_input.name
    _target_size = height
    _log(f"model loaded; providers={_session.get_providers()}, input={shape}")
    return _session, _input_name, _target_size


def _load_with_pillow(path: Path) -> Image.Image:
    with Image.open(path) as image:
        image.load()
        image = ImageOps.exif_transpose(image)
        return image.copy()


def _load_with_ffmpeg(path: Path) -> Image.Image:
    process = subprocess.run(
        [
            "ffmpeg",
            "-v",
            "error",
            "-i",
            str(path),
            "-frames:v",
            "1",
            "-f",
            "image2pipe",
            "-vcodec",
            "png",
            "pipe:1",
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if process.returncode != 0:
        stderr = process.stderr.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"ffmpeg could not decode image: {stderr or 'unknown error'}")
    with Image.open(io.BytesIO(process.stdout)) as image:
        image.load()
        return image.copy()


def _load_image(path: Path) -> Image.Image:
    try:
        return _load_with_pillow(path)
    except Exception as pillow_error:
        try:
            return _load_with_ffmpeg(path)
        except Exception as ffmpeg_error:
            raise RuntimeError(
                f"unable to decode {path}: Pillow: {pillow_error}; ffmpeg: {ffmpeg_error}"
            ) from ffmpeg_error


def _prepare_image(path: Path, target_size: int) -> np.ndarray:
    image = _load_image(path)
    try:
        if image.mode not in ("RGB", "RGBA"):
            image = image.convert("RGBA" if "A" in image.getbands() else "RGB")

        side = max(image.width, image.height)
        square = Image.new("RGB", (side, side), (255, 255, 255))
        offset = ((side - image.width) // 2, (side - image.height) // 2)
        if image.mode == "RGBA":
            square.paste(image.convert("RGB"), offset, image.getchannel("A"))
        else:
            square.paste(image, offset)

        resized = square.resize((target_size, target_size), Image.Resampling.BICUBIC)
        array = np.asarray(resized, dtype=np.float32)
        array = array[:, :, ::-1]
        return np.expand_dims(np.ascontiguousarray(array), axis=0)
    finally:
        image.close()


def _embedding_from_outputs(outputs: list[np.ndarray]) -> np.ndarray:
    candidates: list[np.ndarray] = []
    for output in outputs:
        array = np.asarray(output)
        if array.ndim >= 1 and array.shape[-1] == EMBEDDING_DIMENSIONS:
            candidates.append(array)
    if len(candidates) != 1:
        shapes = [list(np.asarray(output).shape) for output in outputs]
        raise RuntimeError(
            f"expected exactly one {EMBEDDING_DIMENSIONS}-D model output, got shapes {shapes}"
        )

    embedding = np.asarray(candidates[0]).reshape(-1, EMBEDDING_DIMENSIONS)[0].astype(np.float32, copy=False)
    norm = float(np.linalg.norm(embedding))
    if not np.isfinite(norm) or norm <= 0:
        raise RuntimeError(f"invalid embedding norm {norm}")
    normalized = embedding / np.float32(norm)
    if not np.all(np.isfinite(normalized)):
        raise RuntimeError("embedding contains non-finite values")
    return normalized


def embed(path_value: str) -> list[float]:
    path = Path(path_value).expanduser()
    if not path.is_file():
        raise FileNotFoundError(f"image does not exist: {path}")

    session, input_name, target_size = _load_session()
    tensor = _prepare_image(path, target_size)
    outputs = session.run(None, {input_name: tensor})
    embedding = _embedding_from_outputs(outputs)
    return embedding.tolist()


def _response(request_id: Any, **payload: Any) -> None:
    message = {"id": request_id, **payload}
    sys.stdout.write(json.dumps(message, separators=(",", ":"), allow_nan=False) + "\n")
    sys.stdout.flush()


def _handle(request: dict[str, Any]) -> None:
    request_id = request.get("id")
    operation = request.get("op")

    if operation == "ping":
        _response(
            request_id,
            ok=True,
            model=MODEL_ID,
            model_revision=MODEL_REVISION,
            dimensions=EMBEDDING_DIMENSIONS,
            loaded=_session is not None,
        )
        return

    if operation == "embed":
        path = request.get("path")
        if not isinstance(path, str) or not path:
            raise ValueError("embed requires a non-empty path")
        vector = embed(path)
        _response(
            request_id,
            ok=True,
            model=MODEL_ID,
            model_revision=MODEL_REVISION,
            dimensions=EMBEDDING_DIMENSIONS,
            embedding=vector,
        )
        return

    if operation == "shutdown":
        _response(request_id, ok=True)
        raise SystemExit(0)

    raise ValueError(f"unknown operation: {operation!r}")


def main() -> int:
    for raw_line in sys.stdin:
        raw_line = raw_line.strip()
        if not raw_line:
            continue
        request_id: Any = None
        try:
            request = json.loads(raw_line)
            if not isinstance(request, dict):
                raise ValueError("request must be a JSON object")
            request_id = request.get("id")
            _handle(request)
        except SystemExit:
            return 0
        except Exception as error:
            _log(f"request failed: {error}")
            _response(request_id, ok=False, error=str(error))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
