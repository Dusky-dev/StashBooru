#!/usr/bin/env python3
"""Opt-in, non-destructive media-file backup snapshots for StashBooru.

Only explicitly selected filesystem roots are backed up. This tool never removes
or replaces a source file. It never touches the StashBooru database.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import secrets
import stat
import sys
import tempfile
from datetime import datetime, timezone

CHUNK = 1024 * 1024
VERSION = 1
LABEL = re.compile(r"^[a-zA-Z][a-zA-Z0-9_-]{0,39}$")
DIGEST = re.compile(r"^[0-9a-f]{64}$")
SNAPSHOT = re.compile(r"^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$")


class BackupError(Exception):
    pass


def _regular(path: Path) -> os.stat_result:
    try:
        info = path.lstat()
    except OSError as error:
        raise BackupError(f"cannot inspect {path}: {error}") from error
    if not stat.S_ISREG(info.st_mode):
        raise BackupError(f"not a regular file (symlinks not allowed): {path}")
    return info


def _safe_directory(path: Path) -> None:
    if path.is_symlink() or not path.is_dir():
        raise BackupError(f"directory missing or symlinked: {path}")


def _under(a: Path, b: Path) -> bool:
    return a == b or b in a.parents


def _validate_location(store: Path, roots: dict[str, Path]) -> None:
    _safe_directory(store)
    for label, root in roots.items():
        if not LABEL.fullmatch(label):
            raise BackupError(f"invalid root label: {label!r}")
        _safe_directory(root)
        if _under(root.resolve(), store.resolve()) or _under(store.resolve(), root.resolve()):
            raise BackupError(f"store and media root must be separate: {label}")
    for folder in ("objects", "snapshots"):
        location = store / folder
        if location.exists() or location.is_symlink():
            _safe_directory(location)
        else:
            location.mkdir(mode=0o700)


def _relpath(value: str) -> PurePosixPath:
    if (not value or "\\" in value or "\x00" in value or
            any(segment in ("", ".", "..") for segment in value.split("/")) or
            value.startswith("/")):
        raise BackupError(f"unsafe relative path: {value!r}")
    path = PurePosixPath(value)
    if path.is_absolute() or not path.parts:
        raise BackupError(f"unsafe relative path: {value!r}")
    return path


def _object_path(store: Path, digest: str) -> Path:
    if not isinstance(digest, str) or not DIGEST.fullmatch(digest):
        raise BackupError("invalid object checksum")
    return store / "objects" / digest[:2] / digest


def _fsync_directory(path: Path) -> None:
    if os.name != "nt":
        fd = os.open(path, os.O_RDONLY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)


def _read_hash(path: Path) -> tuple[int, str]:
    _regular(path)
    checksum = hashlib.sha256()
    size = 0
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    with os.fdopen(os.open(path, flags), "rb") as source:
        if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
            raise BackupError(f"not a regular file: {path}")
        while chunk := source.read(CHUNK):
            checksum.update(chunk)
            size += len(chunk)
    return size, checksum.hexdigest()


def _source_files(root: Path):
    def onerror(error: OSError) -> None:
        raise BackupError(f"cannot walk media root: {error}") from error

    for parent, directories, files in os.walk(root, topdown=True, followlinks=False, onerror=onerror):
        directories.sort()
        files.sort()
        for directory in directories:
            _safe_directory(Path(parent) / directory)
        for name in files:
            source = Path(parent) / name
            _regular(source)
            relative = source.relative_to(root).as_posix()
            _relpath(relative)
            yield source, relative


def _store_file(source: Path, store: Path) -> tuple[int, str, int, int]:
    """Copy through an fsynced temporary file; publish immutable blobs without overwrite."""
    before = _regular(source)
    bucket = store / "objects" / "tmp"
    bucket.mkdir(mode=0o700, exist_ok=True)
    _safe_directory(bucket)
    temporary = None
    checksum = hashlib.sha256()
    size = 0
    try:
        fd, filename = tempfile.mkstemp(prefix=".backup-", dir=bucket)
        temporary = Path(filename)
        with os.fdopen(fd, "wb") as target:
            flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
            with os.fdopen(os.open(source, flags), "rb") as current:
                opened = os.fstat(current.fileno())
                if not stat.S_ISREG(opened.st_mode):
                    raise BackupError(f"source is not regular: {source}")
                for field in ("st_dev", "st_ino", "st_size", "st_mtime_ns", "st_ctime_ns"):
                    if getattr(opened, field) != getattr(before, field):
                        raise BackupError(f"source changed before backup: {source}")
                while data := current.read(CHUNK):
                    checksum.update(data)
                    size += len(data)
                    target.write(data)
                after = os.fstat(current.fileno())
                for field in ("st_dev", "st_ino", "st_size", "st_mtime_ns", "st_ctime_ns"):
                    if getattr(after, field) != getattr(before, field):
                        raise BackupError(f"source changed during backup: {source}")
                now = _regular(source)
                for field in ("st_dev", "st_ino", "st_size", "st_mtime_ns", "st_ctime_ns"):
                    if getattr(now, field) != getattr(before, field):
                        raise BackupError(f"source path changed during backup: {source}")
            target.flush()
            os.fsync(target.fileno())
        digest = checksum.hexdigest()
        object_path = _object_path(store, digest)
        bucket = object_path.parent
        bucket.mkdir(mode=0o700, exist_ok=True)
        _safe_directory(bucket)
        if object_path.exists() or object_path.is_symlink():
            actual_size, actual_digest = _read_hash(object_path)
            if actual_size != size or actual_digest != digest:
                raise BackupError(f"existing backup object corrupted: {object_path}")
        else:
            try:
                os.link(temporary, object_path)
            except FileExistsError:
                actual_size, actual_digest = _read_hash(object_path)
                if actual_size != size or actual_digest != digest:
                    raise BackupError(f"concurrent backup object corrupted: {object_path}")
            _fsync_directory(bucket)
        return size, digest, before.st_mtime_ns, stat.S_IMODE(before.st_mode)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def _snapshot_path(store: Path, identifier: str) -> Path:
    if not SNAPSHOT.fullmatch(identifier):
        raise BackupError("invalid snapshot identifier")
    return store / "snapshots" / (identifier + ".json")


def _publish_json(path: Path, value: dict) -> None:
    _safe_directory(path.parent)
    temporary = None
    try:
        fd, filename = tempfile.mkstemp(prefix=".snapshot-", dir=path.parent)
        temporary = Path(filename)
        with os.fdopen(fd, "w", encoding="utf-8") as output:
            json.dump(value, output, indent=2, sort_keys=True)
            output.write("\n")
            output.flush()
            os.fsync(output.fileno())
        os.link(temporary, path)
        _fsync_directory(path.parent)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def backup(store: Path, roots: dict[str, Path]) -> dict:
    store = store.absolute()
    roots = {key: value.absolute() for key, value in roots.items()}
    _validate_location(store, roots)
    identifier = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + secrets.token_hex(6)
    entries = []
    for label, root in sorted(roots.items()):
        for source, relative in _source_files(root):
            size, digest, mtime_ns, mode = _store_file(source, store)
            entries.append({"root": label, "path": relative, "size": size,
                            "sha256": digest, "mtime_ns": mtime_ns, "mode": mode})
    payload = {
        "format": "stashbooru-media-backup",
        "version": VERSION,
        "snapshot": identifier,
        "roots": {label: str(root) for label, root in sorted(roots.items())},
        "entries": entries,
    }
    _publish_json(_snapshot_path(store, identifier), payload)
    return {"snapshot": identifier, "files": len(entries), "logicalBytes": sum(x["size"] for x in entries)}


def _load_manifest(store: Path, identifier: str) -> dict:
    _safe_directory(store)
    _safe_directory(store / "snapshots")
    filename = _snapshot_path(store, identifier)
    if filename.is_symlink():
        raise BackupError("snapshot is a symlink")
    try:
        value = json.loads(filename.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise BackupError(f"cannot read manifest: {error}") from error
    if (not isinstance(value, dict) or value.get("format") != "stashbooru-media-backup"
            or value.get("version") != VERSION or value.get("snapshot") != identifier
            or not isinstance(value.get("roots"), dict) or not isinstance(value.get("entries"), list)):
        raise BackupError("unsupported or invalid snapshot")
    for label, path in value["roots"].items():
        if not isinstance(label, str) or not LABEL.fullmatch(label) or not isinstance(path, str):
            raise BackupError("invalid snapshot root")
    seen = set()
    for entry in value["entries"]:
        if not isinstance(entry, dict) or not isinstance(entry.get("root"), str):
            raise BackupError("invalid snapshot member")
        label = entry["root"]
        relative = entry.get("path")
        if label not in value["roots"] or not isinstance(relative, str):
            raise BackupError("invalid snapshot member path")
        _relpath(relative)
        if (label, relative) in seen:
            raise BackupError("duplicate snapshot member")
        seen.add((label, relative))
        if type(entry.get("size")) is not int or entry["size"] < 0:
            raise BackupError("invalid snapshot size")
        if type(entry.get("mtime_ns")) is not int or not 0 <= entry["mtime_ns"] <= 2**63 - 1:
            raise BackupError("invalid snapshot timestamp")
        if type(entry.get("mode")) is not int or not 0 <= entry["mode"] <= 0o777:
            raise BackupError("invalid snapshot permissions")
        _object_path(store, entry.get("sha256"))
    return value


def verify(store: Path, identifier: str) -> dict:
    manifest = _load_manifest(store, identifier)
    seen = set()
    total = 0
    for entry in manifest["entries"]:
        digest = entry["sha256"]
        if digest not in seen:
            length, actual = _read_hash(_object_path(store, digest))
            if length != entry["size"] or actual != digest:
                raise BackupError(f"backup object failed verification: {digest}")
            seen.add(digest)
        total += entry["size"]
    return {"snapshot": identifier, "verified": True, "files": len(manifest["entries"]),
            "objects": len(seen), "logicalBytes": total}


def _restore_destination(output: Path, label: str, relative: str) -> Path:
    path = output / label
    if not path.exists():
        path.mkdir(mode=0o700)
    _safe_directory(path)
    parts = _relpath(relative).parts
    for segment in parts[:-1]:
        path = path / segment
        if not path.exists():
            path.mkdir(mode=0o700)
        _safe_directory(path)
    return path / parts[-1]


def restore(store: Path, identifier: str, output: Path, member: str | None) -> dict:
    manifest = _load_manifest(store, identifier)
    if not output.exists():
        output.mkdir(parents=True, mode=0o700)
    _safe_directory(output)
    if _under(output.resolve(), store.resolve()) or _under(store.resolve(), output.resolve()):
        raise BackupError("restore output and backup store must be separate")
    selection = manifest["entries"]
    if member is not None:
        selection = [e for e in selection if f"{e['root']}:{e['path']}" == member]
        if len(selection) != 1:
            raise BackupError("requested backup member not found")
    # Verify the entire requested recovery set before creating any restored files.
    for entry in selection:
        source = _object_path(store, entry["sha256"])
        size, digest = _read_hash(source)
        if size != entry["size"] or digest != entry["sha256"]:
            raise BackupError(f"backup object failed verification: {digest}")
        destination = output / entry["root"] / Path(*_relpath(entry["path"]).parts)
        if destination.exists() or destination.is_symlink():
            raise BackupError(f"restore would overwrite an existing file: {destination}")
    count = 0
    for entry in selection:
        destination = _restore_destination(output, entry["root"], entry["path"])
        source = _object_path(store, entry["sha256"])
        temporary = None
        try:
            fd, filename = tempfile.mkstemp(prefix=".restore-", dir=destination.parent)
            temporary = Path(filename)
            digest = hashlib.sha256()
            length = 0
            with os.fdopen(fd, "wb") as target, source.open("rb") as inp:
                while data := inp.read(CHUNK):
                    digest.update(data)
                    length += len(data)
                    target.write(data)
                target.flush()
                os.fsync(target.fileno())
            if digest.hexdigest() != entry["sha256"] or length != entry["size"]:
                raise BackupError("backup object changed during restore")
            os.chmod(temporary, entry["mode"])
            os.utime(temporary, ns=(entry["mtime_ns"], entry["mtime_ns"]))
            os.link(temporary, destination)  # atomic, never replace a current file
            _fsync_directory(destination.parent)
            count += 1
        finally:
            if temporary is not None:
                temporary.unlink(missing_ok=True)
    return {"snapshot": identifier, "restored": count, "output": str(output)}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    create = commands.add_parser("backup", help="create a new immutable media snapshot")
    create.add_argument("--store", type=Path, required=True)
    create.add_argument("--root", action="append", required=True, metavar="LABEL=DIR")
    check = commands.add_parser("verify", help="verify snapshot blobs and manifests")
    check.add_argument("--store", type=Path, required=True)
    check.add_argument("--snapshot", required=True)
    recover = commands.add_parser("restore", help="restore to a separate directory, never overwrite")
    recover.add_argument("--store", type=Path, required=True)
    recover.add_argument("--snapshot", required=True)
    recover.add_argument("--output", type=Path, required=True)
    recover.add_argument("--member", help="optional LABEL:relative/path for one file")
    args = parser.parse_args(argv)
    try:
        store = args.store.absolute()
        if args.command == "backup":
            roots = {}
            for specification in args.root:
                if "=" not in specification:
                    raise BackupError("roots must use LABEL=DIR")
                label, location = specification.split("=", 1)
                if label in roots or not location:
                    raise BackupError(f"invalid or repeated root label: {label}")
                roots[label] = Path(location)
            result = backup(store, roots)
        elif args.command == "verify":
            result = verify(store, args.snapshot)
        else:
            result = restore(store, args.snapshot, args.output.absolute(), args.member)
    except (BackupError, OSError, ValueError) as error:
        print(f"media backup: {error}", file=sys.stderr)
        return 1
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
