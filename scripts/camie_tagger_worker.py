#!/usr/bin/env python3
"""Optional Camie Tagger v2 inference worker for StashBooru.

Protocol: one JSON object per stdin line, one JSON object per stdout line.
The GPL-3.0 Camie model is never bundled or downloaded by StashBooru. Users
supply the ONNX model and metadata JSON themselves.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import sys
from typing import Any

import numpy as np
import onnxruntime as ort
from PIL import Image

import visual_embedding_worker as visual

MODEL_ID = "Camais03/camie-tagger-v2"
MODEL_FILENAME = "camie-tagger-v2.onnx"
METADATA_FILENAME = "camie-tagger-v2-metadata.json"
DEFAULT_THRESHOLD = 0.492
DEFAULT_LIMIT = 50
MAX_LIMIT = 200

_session: ort.InferenceSession | None = None
_input_name: str | None = None
_target_size: int | None = None
_metadata: dict[str, Any] | None = None
_idx_to_tag: dict[int, str] | None = None
_tag_to_category: dict[str, str] | None = None
_tag_count: int = 0


def _log(message: str) -> None:
    visual._log(f"camie: {message}")


def _cache_dir() -> Path:
    configured = os.environ.get("STASH_CAMIE_CACHE_DIR", "").strip()
    if configured:
        return Path(configured).expanduser()
    return visual._default_cache_dir() / "camie"


def _model_path() -> Path:
    configured = os.environ.get("STASH_CAMIE_MODEL_PATH", "").strip()
    if configured:
        return Path(configured).expanduser()
    return _cache_dir() / MODEL_FILENAME


def _metadata_path() -> Path:
    configured = os.environ.get("STASH_CAMIE_METADATA_PATH", "").strip()
    if configured:
        return Path(configured).expanduser()
    return _cache_dir() / METADATA_FILENAME


def _load_metadata() -> tuple[dict[int, str], dict[str, str], int, int]:
    global _metadata, _idx_to_tag, _tag_to_category, _tag_count
    if _metadata is not None and _idx_to_tag is not None and _tag_to_category is not None:
        image_size = int(_metadata.get("model_info", {}).get("img_size", 512))
        return _idx_to_tag, _tag_to_category, _tag_count, image_size

    path = _metadata_path()
    if not path.is_file():
        raise FileNotFoundError(f"Camie metadata is not installed: {path}")

    with path.open("r", encoding="utf-8") as handle:
        metadata = json.load(handle)

    try:
        dataset_info = metadata["dataset_info"]
        tag_mapping = dataset_info["tag_mapping"]
        raw_idx_to_tag = tag_mapping["idx_to_tag"]
        raw_tag_to_category = tag_mapping["tag_to_category"]
        total_tags = int(dataset_info["total_tags"])
    except (KeyError, TypeError, ValueError) as error:
        raise ValueError(f"invalid Camie metadata structure: {error}") from error

    if not isinstance(raw_idx_to_tag, dict) or not isinstance(raw_tag_to_category, dict):
        raise ValueError("invalid Camie metadata tag mappings")

    idx_to_tag: dict[int, str] = {}
    for key, value in raw_idx_to_tag.items():
        try:
            index = int(key)
        except (TypeError, ValueError) as error:
            raise ValueError(f"invalid Camie tag index {key!r}") from error
        if not isinstance(value, str) or not value:
            raise ValueError(f"invalid Camie tag name for index {key!r}")
        idx_to_tag[index] = value

    if total_tags <= 0 or len(idx_to_tag) != total_tags:
        raise ValueError(
            f"Camie metadata reports {total_tags} tags but contains {len(idx_to_tag)} indexed names"
        )

    tag_to_category = {
        str(tag): str(category)
        for tag, category in raw_tag_to_category.items()
        if isinstance(tag, str) and isinstance(category, str)
    }

    image_size = int(metadata.get("model_info", {}).get("img_size", 512))
    if image_size <= 0:
        raise ValueError(f"invalid Camie image size {image_size}")

    _metadata = metadata
    _idx_to_tag = idx_to_tag
    _tag_to_category = tag_to_category
    _tag_count = total_tags
    return idx_to_tag, tag_to_category, total_tags, image_size


def _status_payload() -> dict[str, Any]:
    model_path = _model_path()
    metadata_path = _metadata_path()
    model_exists = model_path.is_file()
    metadata_exists = metadata_path.is_file()
    tag_count = 0
    status_error = ""

    if metadata_exists:
        try:
            _, _, tag_count, _ = _load_metadata()
        except Exception as error:
            status_error = str(error)

    return {
        "model": MODEL_ID,
        "model_path": str(model_path),
        "metadata_path": str(metadata_path),
        "model_exists": model_exists,
        "metadata_exists": metadata_exists,
        "installed": model_exists and metadata_exists and not status_error,
        "loaded": _session is not None,
        "tag_count": tag_count,
        "error": status_error,
    }


def _ensure_installed() -> tuple[Path, dict[int, str], dict[str, str], int, int]:
    model_path = _model_path()
    metadata_path = _metadata_path()
    missing: list[str] = []
    if not model_path.is_file():
        missing.append(str(model_path))
    if not metadata_path.is_file():
        missing.append(str(metadata_path))
    if missing:
        raise FileNotFoundError(
            "Camie Tagger v2 is optional and is not installed. Place the required file(s) at: "
            + ", ".join(missing)
        )

    idx_to_tag, tag_to_category, tag_count, image_size = _load_metadata()
    return model_path, idx_to_tag, tag_to_category, tag_count, image_size


def _load_session() -> tuple[ort.InferenceSession, str, int, dict[int, str], dict[str, str], int]:
    global _session, _input_name, _target_size
    model_path, idx_to_tag, tag_to_category, tag_count, metadata_size = _ensure_installed()

    if _session is not None and _input_name is not None and _target_size is not None:
        return _session, _input_name, _target_size, idx_to_tag, tag_to_category, tag_count

    options = ort.SessionOptions()
    options.log_severity_level = 3
    providers = visual._providers()
    _log(f"loading {MODEL_ID} from {model_path}")
    _session = (
        ort.InferenceSession(str(model_path), sess_options=options, providers=providers)
        if providers
        else ort.InferenceSession(str(model_path), sess_options=options)
    )

    inputs = _session.get_inputs()
    if len(inputs) != 1:
        raise RuntimeError(f"expected one Camie ONNX input, got {len(inputs)}")
    model_input = inputs[0]
    shape = model_input.shape
    if len(shape) != 4:
        raise RuntimeError(f"unexpected Camie ONNX input shape: {shape}")

    channels = shape[1]
    height = shape[2]
    width = shape[3]
    if channels not in (3, "3", None):
        raise RuntimeError(f"expected Camie NCHW RGB input, got {shape}")

    target_size = metadata_size
    if isinstance(height, int) and isinstance(width, int):
        if height <= 0 or width <= 0 or height != width:
            raise RuntimeError(f"expected a fixed square Camie input, got {shape}")
        target_size = height

    _input_name = model_input.name
    _target_size = target_size
    _log(f"model loaded; providers={_session.get_providers()}, input={shape}, tags={tag_count}")
    return _session, _input_name, _target_size, idx_to_tag, tag_to_category, tag_count


def _prepare_image(path: Path, image_size: int) -> np.ndarray:
    image = visual._load_image(path)
    try:
        if image.mode != "RGB":
            image = image.convert("RGB")

        width, height = image.size
        if width <= 0 or height <= 0:
            raise ValueError(f"invalid image dimensions {width}x{height}")

        aspect_ratio = width / height
        if aspect_ratio > 1:
            new_width = image_size
            new_height = max(1, int(new_width / aspect_ratio))
        else:
            new_height = image_size
            new_width = max(1, int(new_height * aspect_ratio))

        resized = image.resize((new_width, new_height), Image.Resampling.LANCZOS)
        padded = Image.new("RGB", (image_size, image_size), (124, 116, 104))
        padded.paste(
            resized,
            ((image_size - new_width) // 2, (image_size - new_height) // 2),
        )

        array = np.asarray(padded, dtype=np.float32) / np.float32(255.0)
        mean = np.asarray([0.485, 0.456, 0.406], dtype=np.float32)
        std = np.asarray([0.229, 0.224, 0.225], dtype=np.float32)
        array = (array - mean) / std
        array = np.transpose(array, (2, 0, 1))
        return np.expand_dims(np.ascontiguousarray(array, dtype=np.float32), axis=0)
    finally:
        image.close()


def _sigmoid(logits: np.ndarray) -> np.ndarray:
    clipped = np.clip(logits.astype(np.float32, copy=False), -80.0, 80.0)
    return np.float32(1.0) / (np.float32(1.0) + np.exp(-clipped))


def tag(path_value: str, threshold: float = DEFAULT_THRESHOLD, limit: int = DEFAULT_LIMIT) -> list[dict[str, Any]]:
    if not 0.0 < threshold < 1.0:
        raise ValueError("Camie threshold must be greater than 0 and less than 1")
    if limit < 1 or limit > MAX_LIMIT:
        raise ValueError(f"Camie category limit must be between 1 and {MAX_LIMIT}")

    path = Path(path_value).expanduser()
    if not path.is_file():
        raise FileNotFoundError(f"image does not exist: {path}")

    session, input_name, target_size, idx_to_tag, tag_to_category, tag_count = _load_session()
    tensor = _prepare_image(path, target_size)
    outputs = session.run(None, {input_name: tensor})
    if not outputs:
        raise RuntimeError("Camie ONNX model returned no outputs")

    # Camie v2 exposes initial logits first and refined logits second. The
    # author's ONNX inference example uses the refined output when available.
    logits = np.asarray(outputs[1] if len(outputs) >= 2 else outputs[0], dtype=np.float32)
    if logits.ndim != 2 or logits.shape[0] != 1 or logits.shape[1] != tag_count:
        shapes = [list(np.asarray(output).shape) for output in outputs]
        raise RuntimeError(
            f"expected Camie refined logits shaped [1,{tag_count}], got outputs {shapes}"
        )

    probabilities = _sigmoid(logits[0])
    indices = np.flatnonzero(probabilities >= np.float32(threshold))
    ranked = sorted(indices.tolist(), key=lambda index: float(probabilities[index]), reverse=True)

    category_counts: dict[str, int] = {}
    predictions: list[dict[str, Any]] = []
    for index in ranked:
        tag_name = idx_to_tag.get(index)
        if tag_name is None:
            continue
        category = tag_to_category.get(tag_name, "general")
        if category_counts.get(category, 0) >= limit:
            continue
        score = float(probabilities[index])
        if not np.isfinite(score):
            continue
        predictions.append({"name": tag_name, "category": category, "score": score})
        category_counts[category] = category_counts.get(category, 0) + 1

    return predictions


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

    if operation == "tag":
        path = request.get("path")
        if not isinstance(path, str) or not path:
            raise ValueError("tag requires a non-empty path")
        threshold = float(request.get("threshold", DEFAULT_THRESHOLD))
        limit = int(request.get("limit", DEFAULT_LIMIT))
        predictions = tag(path, threshold=threshold, limit=limit)
        _response(
            request_id,
            ok=True,
            **_status_payload(),
            threshold=threshold,
            tags=predictions,
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
