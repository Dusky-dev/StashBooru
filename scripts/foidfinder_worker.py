#!/usr/bin/env python3
"""Opt-in PixAI v1 / foidfinder inference and model installer.

Inference lives in an isolated Python 3.12+ environment. Neither model files
nor the runtime are fetched on startup, status or tagging.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import threading
from typing import Any

from model_asset_download import fetch_model_file
import visual_embedding_worker as visual

MODEL_ID = "alpindale/PixAI-v1.0-INT8"
BACKEND_SHA = "6d84cb240180f774b330ce6977401183fb543234"
DEFAULT_THRESHOLD = 0.35
DEFAULT_LIMIT = 50
MAX_LIMIT = 200
_guard = threading.RLock()
_process: subprocess.Popen[str] | None = None
_revision = BACKEND_SHA[:12]


def _root() -> Path:
    return Path(os.environ.get("STASH_PIXAI_CACHE_DIR", str(visual._default_cache_dir() / "pixai"))).expanduser()


def _model_dir() -> Path:
    return _root() / "model"


def _runtime() -> Path:
    return _root() / "runtime"


def _python() -> Path:
    return _runtime() / ("Scripts/python.exe" if os.name == "nt" else "bin/python")


def status() -> dict[str, Any]:
    model = _model_dir()
    installed = (model / "model.safetensors").is_file() and (model / "config.json").is_file()
    return {
        "model": MODEL_ID,
        "revision": _revision,
        "backend": "foidfinder",
        "installed": installed and _python().is_file(),
        "model_exists": installed,
        "runtime_exists": _python().is_file(),
        "loaded": _process is not None and _process.poll() is None,
        "model_path": str(model),
        "license": "Apache-2.0",
    }


def _install_runtime() -> None:
    if _python().is_file():
        return
    # Prefer uv for its ability to provision a compatible Python independently.
    uv = shutil.which("uv")
    if uv:
        subprocess.run([uv, "venv", "--python", "3.12", str(_runtime())], check=True)
        installer = [uv, "pip", "install", "--python", str(_python())]
    else:
        if sys.version_info < (3, 12):
            raise RuntimeError("Python 3.12+ or uv is required to install foidfinder")
        subprocess.run([sys.executable, "-m", "venv", str(_runtime())], check=True)
        installer = [str(_python()), "-m", "pip", "install"]
    extra = "mlx" if sys.platform == "darwin" else "cuda"
    source = f"foidfinder[{extra}] @ git+https://github.com/AlpinDale/foidfinder.git@{BACKEND_SHA}"
    try:
        subprocess.run(installer + [source], check=True)
    except Exception:
        shutil.rmtree(_runtime(), ignore_errors=True)
        raise


def install() -> dict[str, Any]:
    with _guard:
        if sys.platform != "darwin" and not shutil.which("nvcc") and not os.environ.get("FOIDFINDER_KERNELS"):
            raise RuntimeError("foidfinder CUDA inference needs nvcc (CUDA Toolkit 12.8+) or prebuilt kernels")
        _install_runtime()
        model = _model_dir()
        for filename in ("config.json", "model.safetensors"):
            target = model / filename
            if not target.is_file():
                fetch_model_file(MODEL_ID, filename, target)
        config = json.loads((model / "config.json").read_text())
        if not isinstance(config.get("tags"), list) or len(config["tags"]) != 30877:
            raise RuntimeError("PixAI model has an unexpected tag vocabulary")
    return status()


def _stop() -> None:
    global _process
    if _process is not None:
        try:
            _process.terminate()
            _process.wait(timeout=5)
        except Exception:
            _process.kill()
            _process.wait()
        _process = None


def delete() -> dict[str, Any]:
    with _guard:
        _stop()
        shutil.rmtree(_root(), ignore_errors=False) if _root().exists() else None
    return status()


def tag(path: str, threshold: float = DEFAULT_THRESHOLD, limit: int = DEFAULT_LIMIT) -> list[dict[str, Any]]:
    global _process
    if not 0 < threshold < 1 or not 1 <= limit <= MAX_LIMIT:
        raise ValueError("invalid PixAI threshold or tag limit")
    if not status()["installed"]:
        raise RuntimeError("PixAI is not installed; use the Install button first")
    with _guard:
        if _process is None or _process.poll() is not None:
            _stop()
            _process = subprocess.Popen(
                [str(_python()), str(Path(__file__).resolve()), "--model-process"],
                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=sys.stderr,
                text=True, bufsize=1,
            )
        assert _process.stdin is not None and _process.stdout is not None
        _process.stdin.write(json.dumps({"path": path, "threshold": threshold, "limit": limit}) + "\n")
        _process.stdin.flush()
        response = _process.stdout.readline()
        if not response:
            _stop()
            raise RuntimeError("foidfinder exited during inference; check worker logs and CUDA toolkit")
        result = json.loads(response)
        if "error" in result:
            raise RuntimeError(result["error"])
        return result["tags"]


def _run_model_process() -> int:
    from foidfinder.backends import load
    from foidfinder.image import decode
    from foidfinder.tagging import TagIndex

    model = _model_dir()
    backend = load("auto", model)
    tags = TagIndex.from_model_dir(model)
    for raw in sys.stdin:
        try:
            command = json.loads(raw)
            with open(command["path"], "rb") as source:
                image = decode(source.read())
            try:
                predictions = tags.result(backend.predict(image))
            finally:
                image.close()
            selected = []
            for category, entries in predictions["tags"].items():
                for value in entries[: command["limit"]]:
                    if value["score"] >= command["threshold"]:
                        selected.append({"name": value["tag"], "category": category, "score": value["score"]})
            if predictions["rating"] is not None:
                value = predictions["rating"]
                selected.append({"name": value["tag"], "category": "rating", "score": value["score"]})
            print(json.dumps({"tags": selected}), flush=True)
        except Exception as error:
            print(json.dumps({"error": str(error)}), flush=True)
    return 0


def _handle(request: dict[str, Any]) -> dict[str, Any]:
    op = request.get("op")
    if op in ("ping", "status"):
        return {"ok": True, **status()}
    if op == "download":
        return {"ok": True, **install()}
    if op == "delete":
        return {"ok": True, **delete()}
    if op == "tag":
        return {"ok": True, **status(), "tags": tag(str(request["path"]), float(request.get("threshold", DEFAULT_THRESHOLD)), int(request.get("limit", DEFAULT_LIMIT)))}
    if op == "shutdown":
        _stop()
        return {"ok": True, "shutdown": True}
    raise ValueError("unknown foidfinder operation")


def main() -> int:
    if "--model-process" in sys.argv:
        return _run_model_process()
    for raw in sys.stdin:
        try:
            request = json.loads(raw)
            result = _handle(request)
            result["id"] = request.get("id")
        except Exception as error:
            result = {"id": request.get("id") if isinstance(request, dict) else None, "ok": False, "error": str(error)}
        print(json.dumps(result), flush=True)
        if result.get("shutdown"):
            break
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
