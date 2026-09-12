#!/usr/bin/env python3
"""Persistent EVA02 visual-embedding and WD tagging worker for StashBooru.

Protocol: one JSON object per stdin line, one JSON object per stdout line.
Logs and download progress are written to stderr so stdout stays machine-readable.

The model is deliberately optional. Status/ping never downloads or loads it.
Only the explicit ``download`` operation downloads the pinned default model.
The tiny tag-label CSV may be fetched on first explicit ``tag`` request when
an already-installed model predates tagging support.
"""

from __future__ import annotations

import csv
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
TAGS_URL = (
    "https://huggingface.co/deepghs/wd14_tagger_with_embeddings/resolve/"
    f"{MODEL_REVISION}/SmilingWolf/wd-eva02-large-tagger-v3/tags_info.csv?download=true"
)
MODEL_SHA256 = "983f026df23214ba55f78cd5898f76f434fa9230837a4e93f1aaa6c37e284835"
MODEL_SIZE_BYTES = 1_260_436_067
MODEL_FILENAME = "model.onnx"
LEGACY_MODEL_FILENAME = "wd-eva02-large-tagger-v3-embedding.onnx"
TAGS_FILENAME = "tags_info.csv"
EMBEDDING_DIMENSIONS = 1024
DOWNLOAD_CHUNK = 8 * 1024 * 1024

# WD/Danbooru categories. Rating/meta labels are deliberately not exposed to
# Image Tagging: EVA02 is the lowest-authority source and should contribute
# useful taxonomy, not overwrite richer local/Danbooru/Camie metadata.
TAG_CATEGORIES = {
    "0": "general",
    "general": "general",
    "1": "artist",
    "artist": "artist",
    "3": "copyright",
    "copyright": "copyright",
    "4": "character",
    "character": "character",
}
TAG_THRESHOLDS = {
    "general": 0.35,
    "artist": 0.50,
    "copyright": 0.50,
    "character": 0.85,
}
MAX_TAGS = 200

_session: ort.InferenceSession | None = None
_input_name: str | None = None
_target_size: int | None = None
_tag_rows: list[tuple[str, str]] | None = None


def _log(message: str) -> None:
    print(f"[visual-embedding] {message}", file=sys.stderr, flush=True)


def _legacy_config_cache_dir() -> Path | None:
    stash_config = os.environ.get("STASH_CONFIG_FILE")
    if not stash_config:
        return None
    return Path(stash_config).expanduser().resolve().parent / "cache" / "visual-embeddings"


def _default_cache_dir() -> Path:
    configured = os.environ.get("STASH_EMBEDDING_CACHE_DIR")
    if configured:
        return Path(configured).expanduser()

    stash_cache = os.environ.get("STASH_CACHE")
    if stash_cache:
        return Path(stash_cache).expanduser() / "visual-embeddings"

    legacy = _legacy_config_cache_dir()
    if legacy is not None:
        return legacy

    xdg_cache = os.environ.get("XDG_CACHE_HOME")
    if xdg_cache:
        return Path(xdg_cache).expanduser() / "stashbooru" / "visual-embeddings"

    return Path.home() / ".cache" / "stashbooru" / "visual-embeddings"


def _model_candidates() -> list[Path]:
    override = os.environ.get("STASH_EMBEDDING_MODEL_PATH")
    if override:
        return [Path(override).expanduser()]

    directories = [_default_cache_dir()]
    legacy = _legacy_config_cache_dir()
    if legacy is not None and legacy not in directories:
        directories.append(legacy)

    candidates: list[Path] = []
    for directory in directories:
        candidates.append(directory / MODEL_FILENAME)
        candidates.append(directory / LEGACY_MODEL_FILENAME)
        candidates.append(directory / "wd-eva02-large-tagger-v3" / MODEL_FILENAME)
    return candidates


def _installed_model_path() -> Path | None:
    for path in _model_candidates():
        try:
            if not path.is_file():
                continue
            size = path.stat().st_size
            if size != MODEL_SIZE_BYTES:
                _log(
                    f"ignoring model candidate with unexpected size: {path} "
                    f"({size} bytes, expected {MODEL_SIZE_BYTES})"
                )
                continue
            return path
        except OSError as error:
            _log(f"unable to inspect model candidate {path}: {error}")
    return None


def _download_destination() -> Path:
    override = os.environ.get("STASH_EMBEDDING_MODEL_PATH")
    if override:
        return Path(override).expanduser()
    return _default_cache_dir() / MODEL_FILENAME


def _model_path() -> Path:
    return _installed_model_path() or _download_destination()


def _tags_path() -> Path:
    configured = os.environ.get("STASH_EVA02_TAGS_PATH")
    if configured:
        return Path(configured).expanduser()
    model = _installed_model_path()
    if model is not None:
        return model.parent / TAGS_FILENAME
    return _download_destination().parent / TAGS_FILENAME


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
        _log(f"downloading EVA02-Large embedding/tagger model to {destination}")
        request = urllib.request.Request(
            MODEL_URL,
            headers={"User-Agent": "StashBooru visual-embedding-worker/1"},
        )
        with urllib.request.urlopen(request, timeout=60) as response, temp_path.open("wb") as output:
            expected = response.headers.get("Content-Length")
            expected_bytes = int(expected) if expected and expected.isdigit() else MODEL_SIZE_BYTES
            downloaded = 0
            next_report = 64 * 1024 * 1024
            while True:
                chunk = response.read(DOWNLOAD_CHUNK)
                if not chunk:
                    break
                output.write(chunk)
                downloaded += len(chunk)
                if downloaded >= next_report:
                    _log(
                        f"model download: {downloaded / 1024**3:.2f}/"
                        f"{expected_bytes / 1024**3:.2f} GiB"
                    )
                    next_report += 64 * 1024 * 1024

        actual_size = temp_path.stat().st_size
        if actual_size != MODEL_SIZE_BYTES:
            raise RuntimeError(
                f"downloaded model size mismatch: expected {MODEL_SIZE_BYTES}, got {actual_size}"
            )

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


def _download_tags(destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    fd, temp_name = tempfile.mkstemp(prefix=destination.name + ".", suffix=".part", dir=destination.parent)
    os.close(fd)
    temp_path = Path(temp_name)
    try:
        _log(f"downloading EVA02 tag labels to {destination}")
        request = urllib.request.Request(
            TAGS_URL,
            headers={"User-Agent": "StashBooru visual-embedding-worker/1"},
        )
        with urllib.request.urlopen(request, timeout=60) as response, temp_path.open("wb") as output:
            while chunk := response.read(1024 * 1024):
                output.write(chunk)
        if temp_path.stat().st_size == 0:
            raise RuntimeError("downloaded EVA02 tag labels are empty")
        os.replace(temp_path, destination)
    finally:
        try:
            temp_path.unlink()
        except FileNotFoundError:
            pass


def _ensure_model() -> Path:
    path = _installed_model_path()
    if path is None:
        expected = _download_destination()
        raise FileNotFoundError(
            f"visual embedding model is not installed: {expected}. "
            "Install it explicitly from StashBooru settings or set STASH_EMBEDDING_MODEL_PATH."
        )

    if os.environ.get("STASH_EMBEDDING_VERIFY_MODEL", "0") == "1":
        actual_hash = _sha256(path)
        if actual_hash != MODEL_SHA256:
            raise RuntimeError(
                f"visual embedding model checksum mismatch: expected {MODEL_SHA256}, got {actual_hash}"
            )
    return path


def _status_payload() -> dict[str, Any]:
    installed_path = _installed_model_path()
    path = installed_path or _download_destination()
    return {
        "model": MODEL_ID,
        "model_revision": MODEL_REVISION,
        "dimensions": EMBEDDING_DIMENSIONS,
        "model_path": str(path),
        "installed": installed_path is not None,
        "loaded": _session is not None,
    }


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
    _session = (
        ort.InferenceSession(str(model_path), sess_options=session_options, providers=providers)
        if providers
        else ort.InferenceSession(str(model_path), sess_options=session_options)
    )

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


def _load_tag_rows() -> list[tuple[str, str]]:
    global _tag_rows
    if _tag_rows is not None:
        return _tag_rows

    path = _tags_path()
    if not path.is_file():
        # This is intentionally the only implicit download: the heavyweight
        # model must already have been installed explicitly; labels are only a
        # small companion file required to interpret its existing tag output.
        _download_tags(path)

    rows: list[tuple[str, str]] = []
    with path.open("r", encoding="utf-8-sig", newline="") as handle:
        reader = csv.DictReader(handle)
        fieldnames = {str(name).strip().lower(): name for name in (reader.fieldnames or [])}
        name_field = fieldnames.get("name") or fieldnames.get("tag")
        category_field = fieldnames.get("category")
        if name_field is None or category_field is None:
            raise RuntimeError(
                f"unexpected EVA02 tag label columns: {reader.fieldnames}; expected name/tag and category"
            )
        for row in reader:
            raw_name = str(row.get(name_field, "")).strip()
            raw_category = str(row.get(category_field, "")).strip().lower()
            if raw_name:
                rows.append((raw_name, raw_category))

    if not rows:
        raise RuntimeError("EVA02 tag label file contains no labels")
    _tag_rows = rows
    return rows


def _tag_scores_from_outputs(outputs: list[np.ndarray], count: int) -> np.ndarray:
    candidates: list[np.ndarray] = []
    for output in outputs:
        array = np.asarray(output)
        if array.size == count:
            candidates.append(array.reshape(-1))
    if len(candidates) != 1:
        shapes = [list(np.asarray(output).shape) for output in outputs]
        raise RuntimeError(
            f"expected exactly one EVA02 tag output with {count} values, got shapes {shapes}"
        )
    return candidates[0].astype(np.float32, copy=False)


def _run_model(path_value: str) -> tuple[list[np.ndarray], Path]:
    path = Path(path_value).expanduser()
    if not path.is_file():
        raise FileNotFoundError(f"image does not exist: {path}")
    session, input_name, target_size = _load_session()
    tensor = _prepare_image(path, target_size)
    return session.run(None, {input_name: tensor}), path


def embed(path_value: str) -> list[float]:
    outputs, _ = _run_model(path_value)
    embedding = _embedding_from_outputs(outputs)
    return embedding.tolist()


def tag(path_value: str) -> list[dict[str, Any]]:
    rows = _load_tag_rows()
    outputs, _ = _run_model(path_value)
    scores = _tag_scores_from_outputs(outputs, len(rows))

    tags: list[dict[str, Any]] = []
    for (raw_name, raw_category), score_value in zip(rows, scores, strict=True):
        category = TAG_CATEGORIES.get(raw_category)
        if category is None:
            continue
        score = float(score_value)
        if not np.isfinite(score) or score < TAG_THRESHOLDS[category]:
            continue
        tags.append(
            {
                "name": raw_name.replace("_", " "),
                "raw_name": raw_name,
                "category": category,
                "score": score,
                "source": "eva02",
            }
        )

    tags.sort(key=lambda item: float(item["score"]), reverse=True)
    return tags[:MAX_TAGS]


def _response(request_id: Any, **payload: Any) -> None:
    message = {"id": request_id, **payload}
    sys.stdout.write(json.dumps(message, separators=(",", ":"), allow_nan=False) + "\n")
    sys.stdout.flush()


def _handle(request: dict[str, Any]) -> None:
    request_id = request.get("id")
    operation = request.get("op")

    if operation in ("ping", "status"):
        _response(request_id, ok=True, **_status_payload())
        return

    if operation == "download":
        if _installed_model_path() is None:
            _download_model(_download_destination())
        if not _tags_path().is_file():
            _download_tags(_tags_path())
        _response(request_id, ok=True, **_status_payload())
        return

    if operation == "embed":
        path = request.get("path")
        if not isinstance(path, str) or not path:
            raise ValueError("embed requires a non-empty path")
        vector = embed(path)
        _response(request_id, ok=True, **_status_payload(), embedding=vector)
        return

    if operation == "tag":
        path = request.get("path")
        if not isinstance(path, str) or not path:
            raise ValueError("tag requires a non-empty path")
        tags = tag(path)
        _response(request_id, ok=True, **_status_payload(), tags=tags)
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
