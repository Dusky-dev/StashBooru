#!/usr/bin/env python3
"""Read-only capability checks for a StashBooru remote worker."""
from __future__ import annotations

import argparse
import json
import os
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

CHECKS = (
    ("EVA02 embedding / tagging", "/v1/status", False),
    ("Camie tagging (optional model)", "/v1/camie/status", True),
    ("PixAI / foidfinder tagging (optional model)", "/v1/pixai/status", True),
    ("Media conversion / upscaling", "/v1/convert/capabilities", False),
    ("Video overlap sampling", "/v1/video-overlap/capabilities", False),
    ("Image restoration (optional model)", "/v1/restoration/capabilities", False),
)


def check(url: str, token: str, timeout: float = 10) -> int:
    base = url.strip().rstrip("/")
    if not base.startswith(("http://", "https://")):
        print("Worker URL must start with http:// or https://")
        return 2
    failures = 0
    for label, endpoint, optional in CHECKS:
        request = Request(base + endpoint)
        if token:
            request.add_header("Authorization", "Bearer " + token)
        try:
            with urlopen(request, timeout=timeout) as response:
                payload = json.load(response)
                if not isinstance(payload, dict):
                    raise ValueError("expected a JSON object")
                if payload.get("ok") is False:
                    raise ValueError(str(payload.get("error", "worker returned ok=false")))
                detail = ""
                if "installed" in payload:
                    detail = "model installed" if payload["installed"] else "model not installed"
                elif "available" in payload:
                    detail = "available" if payload["available"] else "optional backend unavailable"
                elif "formats" in payload:
                    detail = f"{len(payload['formats'])} output formats"
                print(f"OK    {label}: {detail or 'endpoint reachable'}")
        except HTTPError as error:
            if optional and error.code == 503:
                print(f"OPTIONAL {label}: not installed or unavailable (HTTP 503)")
                continue
            failures += 1
            print(f"FAIL  {label}: HTTP {error.code}")
        except (URLError, OSError, ValueError, json.JSONDecodeError) as error:
            failures += 1
            print(f"FAIL  {label}: {error}")
    print("Each optional model/codec requires installation on the worker machine.")
    return 1 if failures else 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", required=True, help="Worker origin, e.g. http://gpu-pc:8000")
    parser.add_argument("--timeout", type=float, default=10)
    args = parser.parse_args()
    return check(args.url, os.environ.get("STASH_REMOTE_WORKER_TOKEN", os.environ.get("STASH_EMBEDDING_SERVER_TOKEN", "")), args.timeout)


if __name__ == "__main__":
    raise SystemExit(main())
