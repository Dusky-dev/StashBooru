#!/usr/bin/env python3
"""Check custom-UI CLI serving with isolated, unconfigured server processes."""

import argparse
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def fetch(url):
    with urllib.request.urlopen(url, timeout=1) as response:
        return response.read().decode("utf-8")


def check_case(binary, root, output, name, arguments, ui_env, expected):
    case = root / name
    case.mkdir()
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]

    environment = {
        key: value for key, value in os.environ.items() if not key.startswith("STASH_")
    }
    environment["STASH_LOCAL_HTTPS_ENABLED"] = "false"
    if ui_env is not None:
        environment["STASH_UI"] = str(ui_env)
    command = [
        str(binary),
        "--config", str(case / "config.yml"),
        "--host", "127.0.0.1",
        "--port", str(port),
        "--nobrowser",
        *arguments,
    ]
    url = f"http://127.0.0.1:{port}"
    log = output / f"{name}.log"
    with log.open("w") as stream:
        process = subprocess.Popen(
            command, cwd=root, env=environment,
            stdout=stream, stderr=subprocess.STDOUT,
        )
        try:
            deadline = time.monotonic() + 30
            while True:
                if process.poll() is not None:
                    raise RuntimeError(f"{name}: server exited; see {log}")
                try:
                    index = fetch(url + "/")
                    break
                except (urllib.error.URLError, TimeoutError):
                    if time.monotonic() >= deadline:
                        raise RuntimeError(f"{name}: server did not start; see {log}")
                    time.sleep(0.1)

            if expected is None:
                assert "<html" in index.lower(), "embedded UI did not load"
                assert "custom-ui-fixture-" not in index, "custom UI leaked into default"
            else:
                assert expected in index, f"{name}: custom index was ignored"
                assert expected in fetch(url + "/images/123"), "SPA route lost custom UI"
                assert fetch(url + "/assets/probe.css") == f"/* {expected} */\n", (
                    f"{name}: asset did not come from selected directory"
                )
            print(f"{name}: passed", flush=True)
        finally:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=10)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--output", type=Path, help="directory for server logs")
    options = parser.parse_args()
    binary = options.binary.resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="stashbooru-custom-ui-") as temporary:
        root = Path(temporary)
        output = options.output.resolve() if options.output else root / "logs"
        output.mkdir(parents=True, exist_ok=True)
        first = root / "custom UI one"
        second = root / "custom UI two"
        for directory, marker in [
            (first, "custom-ui-fixture-one"),
            (second, "custom-ui-fixture-two"),
        ]:
            (directory / "assets").mkdir(parents=True)
            (directory / "index.html").write_text(
                f'<!doctype html><html><head><base href="/"></head>'
                f'<body>{marker}</body></html>\n', encoding="utf-8"
            )
            (directory / "assets" / "probe.css").write_text(
                f"/* {marker} */\n", encoding="utf-8"
            )

        cases = [
            ("long-flag", ["--ui-location", str(first)], None, "custom-ui-fixture-one"),
            ("short-flag", ["-u", str(second)], None, "custom-ui-fixture-two"),
            ("relative-flag", ["--ui-location", first.name], None, "custom-ui-fixture-one"),
            ("environment", [], second, "custom-ui-fixture-two"),
            ("environment-precedence", ["--ui-location", str(first)],
             second, "custom-ui-fixture-two"),
            ("embedded-default", [], None, None),
        ]
        for name, arguments, ui_env, expected in cases:
            check_case(binary, root, output, name, arguments, ui_env, expected)
    print("Six custom-UI server cases passed; all temporary instances removed.")


if __name__ == "__main__":
    main()
