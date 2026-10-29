#!/usr/bin/env python3
"""Read-only P12/P13 full-stack member audit; no media activation or extraction."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import sys

import image_storage_pack_proof as proof
import image_storage_shadow_member as shadow
import image_delta_storage as delta

MAX_MEMBERS = 16


def audit(spec):
    group = spec.get("group")
    method = spec.get("method")
    archive = Path(spec.get("archivePath", ""))
    expected = spec.get("sourceSHA256")
    members = spec.get("members")
    snapshot = spec.get("mediaBackupSnapshot")
    if (not isinstance(group, str) or not group.startswith("stack:") or
            method not in proof.METHODS or
            not isinstance(members, list) or not 2 <= len(members) <= MAX_MEMBERS or
            not isinstance(expected, dict) or len(expected) != len(members) or
            not isinstance(snapshot, str) or not snapshot):
        raise ValueError("invalid bounded shadow audit specification")
    if not archive.is_absolute() or archive.is_symlink():
        raise ValueError("candidate archive must be an absolute non-symlink file")
    identities = set()
    paths = set()
    for member in members:
        if not isinstance(member, dict):
            raise ValueError("invalid native group member")
        identity, source = member.get("id"), member.get("path")
        digest = expected.get(identity)
        if (not isinstance(identity, str) or not identity.startswith("image:") or
                identity in identities or not isinstance(source, str) or source in paths or
                not isinstance(digest, str) or not delta.HASH.fullmatch(digest)):
            raise ValueError("duplicate or invalid native image identity, path or digest")
        identities.add(identity)
        paths.add(source)
    if identities != set(expected):
        raise ValueError("shadow audit members do not match candidate proof")
    if len(identities) != len(members):
        raise ValueError("invalid member count")

    audited = []
    with proof.open_reader(method, archive) as reader:
        fidelity = reader.manifest["fidelity"] if method == "delta" else reader.index["fidelity"]
        if fidelity != "byte-exact" or set(reader.entries) != identities:
            raise ValueError("candidate fidelity or member identities have changed")
        for member in members:
            identity = member["id"]
            source = shadow.regular_bytes(member["path"])
            sha = hashlib.sha256(source).hexdigest()
            if sha != expected[identity]:
                raise ValueError("native original differs from archive SHA-256")
            restored = reader.reconstruct(identity)
            if restored != source or hashlib.sha256(restored).hexdigest() != sha:
                raise ValueError("archived member does not reconstruct to native original")
            audited.append({
                "id": identity, "bytes": len(restored), "sha256": sha,
                "byteExact": True, "originalPresent": True,
            })
    return {
        "verified": True,
        "readOnly": True,
        "group": group,
        "method": method,
        "mediaBackupSnapshot": snapshot,
        "members": audited,
        "archiveActivated": False,
        "databaseModified": False,
        "nativeServingActivated": False,
        "originalEvictionEnabled": False,
        "note": "Point-in-time original equality only; native thumbnails/previews and serving not proven.",
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--spec", required=True)
    args = parser.parse_args(argv)
    try:
        spec_path = Path(args.spec)
        if not spec_path.is_absolute() or spec_path.is_symlink():
            raise ValueError("audit specification must be an absolute regular file")
        spec = json.loads(delta.read_regular(spec_path, 128 * 1024))
        print(json.dumps(audit(spec), sort_keys=True, allow_nan=False))
        return 0
    except (ValueError, OSError, KeyError, TypeError, json.JSONDecodeError) as error:
        print("shadow stack audit refused: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
