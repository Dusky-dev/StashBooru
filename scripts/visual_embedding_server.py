#!/usr/bin/env python3
"""HTTP inference server for StashBooru visual embeddings and optional taggers.

The server intentionally owns no Stash metadata, embedding database, or tag
assignments. It receives image bytes, performs inference, and returns results.
"""

from __future__ import annotations

import argparse
import hmac
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import tempfile
import threading
from typing import Any
from urllib.parse import parse_qs, urlsplit

import visual_embedding_worker as worker

try:
    import camie_tagger_worker as camie

    _camie_import_error = ""
except Exception as error:  # Camie is deliberately optional.
    camie = None
    _camie_import_error = str(error)

DEFAULT_HOST = os.environ.get("STASH_EMBEDDING_SERVER_HOST", "0.0.0.0")
DEFAULT_PORT = int(os.environ.get("STASH_EMBEDDING_SERVER_PORT", "8000"))
DEFAULT_MAX_UPLOAD_MB = int(os.environ.get("STASH_EMBEDDING_SERVER_MAX_UPLOAD_MB", "512"))
SERVER_TOKEN = os.environ.get("STASH_EMBEDDING_SERVER_TOKEN", "").strip()

_inference_lock = threading.Lock()


def _log(message: str) -> None:
    worker._log(f"remote-server: {message}")


def _authorized(header_value: str | None) -> bool:
    if not SERVER_TOKEN:
        return True
    if not header_value or not header_value.startswith("Bearer "):
        return False
    provided = header_value[len("Bearer ") :]
    return hmac.compare_digest(provided, SERVER_TOKEN)


class VisualEmbeddingHandler(BaseHTTPRequestHandler):
    server_version = "StashBooruVisualEmbedding/3"

    def log_message(self, format: str, *args: Any) -> None:
        _log(format % args)

    def _send_json(self, status: HTTPStatus | int, payload: dict[str, Any]) -> None:
        data = json.dumps(payload, separators=(",", ":"), allow_nan=False).encode("utf-8")
        self.send_response(int(status))
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def _reject_unauthorized(self) -> None:
        self._send_json(HTTPStatus.UNAUTHORIZED, {"ok": False, "error": "unauthorized"})

    def _embedding_status(self) -> dict[str, Any]:
        return {
            "ok": True,
            **worker._status_payload(),
            "available_providers": worker.ort.get_available_providers(),
        }

    def _camie_status(self) -> dict[str, Any] | None:
        if camie is None:
            return None
        return {
            "ok": True,
            **camie._status_payload(),
            "available_providers": worker.ort.get_available_providers(),
        }

    def do_GET(self) -> None:
        if not _authorized(self.headers.get("Authorization")):
            self._reject_unauthorized()
            return

        path = urlsplit(self.path).path.rstrip("/")
        if path == "/v1/status":
            self._send_json(HTTPStatus.OK, self._embedding_status())
            return
        if path == "/v1/camie/status":
            status = self._camie_status()
            if status is None:
                self._send_json(
                    HTTPStatus.SERVICE_UNAVAILABLE,
                    {
                        "ok": False,
                        "error": f"Camie worker is unavailable: {_camie_import_error or 'not installed'}",
                    },
                )
            else:
                self._send_json(HTTPStatus.OK, status)
            return

        self._send_json(HTTPStatus.NOT_FOUND, {"ok": False, "error": "not found"})

    def _read_upload_to_temp(self) -> Path | None:
        raw_length = self.headers.get("Content-Length")
        try:
            content_length = int(raw_length or "")
        except ValueError:
            content_length = -1

        max_upload_bytes = self.server.max_upload_bytes  # type: ignore[attr-defined]
        if content_length <= 0:
            self._send_json(
                HTTPStatus.LENGTH_REQUIRED,
                {"ok": False, "error": "Content-Length is required"},
            )
            return None
        if content_length > max_upload_bytes:
            self._send_json(
                HTTPStatus.REQUEST_ENTITY_TOO_LARGE,
                {
                    "ok": False,
                    "error": f"image is too large ({content_length} bytes; limit {max_upload_bytes})",
                },
            )
            return None

        temp_dir = os.environ.get("STASH_EMBEDDING_SERVER_TEMP_DIR")
        fd, temp_name = tempfile.mkstemp(prefix="stashbooru-remote-", suffix=".img", dir=temp_dir)
        temp_path = Path(temp_name)
        try:
            remaining = content_length
            with os.fdopen(fd, "wb") as output:
                while remaining > 0:
                    chunk = self.rfile.read(min(8 * 1024 * 1024, remaining))
                    if not chunk:
                        raise RuntimeError("client disconnected before the complete image was received")
                    output.write(chunk)
                    remaining -= len(chunk)
            return temp_path
        except Exception:
            try:
                os.close(fd)
            except OSError:
                pass
            try:
                temp_path.unlink()
            except FileNotFoundError:
                pass
            raise

    def do_POST(self) -> None:
        if not _authorized(self.headers.get("Authorization")):
            self._reject_unauthorized()
            return

        parsed = urlsplit(self.path)
        path = parsed.path.rstrip("/")
        if path not in ("/v1/embed", "/v1/tag", "/v1/camie/tag"):
            self._send_json(HTTPStatus.NOT_FOUND, {"ok": False, "error": "not found"})
            return

        if path == "/v1/camie/tag" and camie is None:
            self._send_json(
                HTTPStatus.SERVICE_UNAVAILABLE,
                {
                    "ok": False,
                    "error": f"Camie worker is unavailable: {_camie_import_error or 'not installed'}",
                },
            )
            return

        temp_path: Path | None = None
        try:
            temp_path = self._read_upload_to_temp()
            if temp_path is None:
                return

            with _inference_lock:
                if path == "/v1/embed":
                    embedding = worker.embed(str(temp_path))
                    payload = {
                        "ok": True,
                        **worker._status_payload(),
                        "embedding": embedding,
                    }
                elif path == "/v1/tag":
                    query = parse_qs(parsed.query)
                    threshold = float(
                        query.get("threshold", [worker.DEFAULT_TAG_THRESHOLD])[0]
                    )
                    limit = int(query.get("limit", [worker.DEFAULT_TAG_LIMIT])[0])
                    tags = worker.tag(str(temp_path), threshold=threshold, limit=limit)
                    payload = {
                        "ok": True,
                        **worker._status_payload(),
                        "threshold": threshold,
                        "limit": limit,
                        "tags": tags,
                    }
                else:
                    assert camie is not None
                    query = parse_qs(parsed.query)
                    threshold = float(query.get("threshold", [camie.DEFAULT_THRESHOLD])[0])
                    limit = int(query.get("limit", [camie.DEFAULT_LIMIT])[0])
                    tags = camie.tag(str(temp_path), threshold=threshold, limit=limit)
                    payload = {
                        "ok": True,
                        **camie._status_payload(),
                        "threshold": threshold,
                        "tags": tags,
                    }

            self._send_json(HTTPStatus.OK, payload)
        except ValueError as error:
            _log(f"invalid inference request: {error}")
            self._send_json(HTTPStatus.BAD_REQUEST, {"ok": False, "error": str(error)})
        except Exception as error:
            _log(f"inference request failed: {error}")
            self._send_json(
                HTTPStatus.INTERNAL_SERVER_ERROR,
                {"ok": False, "error": str(error)},
            )
        finally:
            if temp_path is not None:
                try:
                    temp_path.unlink()
                except FileNotFoundError:
                    pass


class VisualEmbeddingHTTPServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, server_address: tuple[str, int], max_upload_bytes: int):
        super().__init__(server_address, VisualEmbeddingHandler)
        self.max_upload_bytes = max_upload_bytes


def main() -> int:
    parser = argparse.ArgumentParser(description="StashBooru remote ML inference worker")
    parser.add_argument("--host", default=DEFAULT_HOST)
    parser.add_argument("--port", type=int, default=DEFAULT_PORT)
    parser.add_argument("--max-upload-mb", type=int, default=DEFAULT_MAX_UPLOAD_MB)
    parser.add_argument(
        "--download-model",
        action="store_true",
        help="download the pinned EVA02 embedding/tagging model before starting if it is not installed",
    )
    args = parser.parse_args()

    if args.max_upload_mb <= 0:
        parser.error("--max-upload-mb must be greater than zero")

    if args.download_model:
        if worker._installed_model_path() is None:
            worker._download_model(worker._download_destination())
        if worker._installed_tags_info_path() is None:
            worker._download_tags_info(worker._tags_info_destination())

    status = worker._status_payload()
    _log(
        f"starting on {args.host}:{args.port}; installed={status['installed']}; "
        f"model_path={status['model_path']}; available_providers={worker.ort.get_available_providers()}"
    )
    if camie is not None:
        camie_status = camie._status_payload()
        _log(
            f"Camie optional tagger: installed={camie_status['installed']}; "
            f"model_path={camie_status['model_path']}; metadata_path={camie_status['metadata_path']}"
        )
    elif _camie_import_error:
        _log(f"Camie optional tagger unavailable: {_camie_import_error}")

    if not SERVER_TOKEN:
        _log("warning: STASH_EMBEDDING_SERVER_TOKEN is not set; server accepts unauthenticated requests")

    server = VisualEmbeddingHTTPServer(
        (args.host, args.port),
        max_upload_bytes=args.max_upload_mb * 1024 * 1024,
    )
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        _log("shutting down")
    finally:
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
