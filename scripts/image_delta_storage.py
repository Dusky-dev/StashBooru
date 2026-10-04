#!/usr/bin/env python3
"""P12 offline benchmark and immutable, self-contained recovery prototype.

Never opens the catalogue, activates files, downloads models, or removes sources.
The archive format is experimental; byte-exact and pixel-exact are separate modes.
"""
from __future__ import annotations

import argparse
import base64
import hashlib
import importlib.util
import io
import json
import math
import os
from pathlib import Path
import re
import shutil
import struct
import sys
import tempfile
import time
import zipfile
import zlib
from dataclasses import dataclass

VERSION = 1
MAX_FILE = 64 * 1024 * 1024
MAX_GROUP = 256 * 1024 * 1024
MAX_PACK = 384 * 1024 * 1024
MAX_PIXELS = 16 * 1024 * 1024
MAX_METADATA = 1024 * 1024
MAX_MANIFEST = 24 * 1024 * 1024
MAX_MEMBERS = 16
MAX_GROUPS = 32
MAX_SECONDS = 900
PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"
PNG_EMBED = {"cHRM", "gAMA", "iCCP", "sRGB", "pHYs", "tEXt", "zTXt", "iTXt", "eXIf", "tIME"}
HASH = re.compile(r"[0-9a-f]{64}\Z")
SAFE_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9_. -]{0,179}\Z")


def json_bytes(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False).encode()


def require_dependencies():
    missing = [name for name in ("PIL", "numpy") if importlib.util.find_spec(name) is None]
    if missing:
        raise ValueError("Install Pillow and NumPy in a dedicated Python environment: "
                         "python3 -m venv .venv-p12; .venv-p12/bin/python -m pip install Pillow numpy. "
                         "Run this tool with .venv-p12/bin/python. Nothing is downloaded automatically.")


def sha(data):
    return hashlib.sha256(data).hexdigest()


def integer(value, low, high, label):
    if isinstance(value, bool) or not isinstance(value, int) or not low <= value <= high:
        raise ValueError(f"invalid {label}")
    return value


def check_deadline(started):
    if time.monotonic() - started > MAX_SECONDS:
        raise ValueError("P12 operation exceeded the 15-minute limit; use a smaller stack")


def unb64(value):
    if not isinstance(value, str) or len(value) > MAX_METADATA * 2:
        raise ValueError("invalid metadata payload")
    return base64.b64decode(value, validate=True)


def b64(value):
    return base64.b64encode(value).decode("ascii")


def read_regular(path, limit):
    descriptor = os.open(path, os.O_RDONLY | getattr(os, "O_NONBLOCK", 0))
    with os.fdopen(descriptor, "rb") as source:
        import stat
        if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
            raise ValueError("input must be a regular file")
        if os.fstat(source.fileno()).st_size > limit:
            raise ValueError(f"input exceeds {limit} bytes")
        result = source.read(limit + 1)
    if len(result) > limit:
        raise ValueError(f"input exceeds {limit} bytes")
    return result


def decompress(data, length):
    integer(length, 0, MAX_FILE, "decoded payload length")
    decoder = zlib.decompressobj()
    result = decoder.decompress(data, length + 1)
    if len(result) != length or not decoder.eof or decoder.unused_data or decoder.unconsumed_tail:
        raise ValueError("invalid, oversized or trailing compressed payload")
    return result


def xor_bytes(target, prediction):
    import numpy as np
    result = np.frombuffer(target, dtype=np.uint8).copy()
    overlap = min(len(target), len(prediction))
    result[:overlap] ^= np.frombuffer(prediction, dtype=np.uint8, count=overlap)
    return result.tobytes()


def png_chunk(kind, data):
    encoded = kind.encode("ascii")
    return struct.pack(">I", len(data)) + encoded + data + struct.pack(">I", zlib.crc32(encoded + data))


def png_ancillary(data):
    chunks, offset, total = [], 8, 0
    while offset < len(data):
        if offset + 12 > len(data):
            raise ValueError("truncated PNG chunk")
        length = struct.unpack_from(">I", data, offset)[0]
        end = offset + 12 + length
        if end > len(data):
            raise ValueError("truncated PNG payload")
        kind_bytes = data[offset + 4:offset + 8]
        value = data[offset + 8:end - 4]
        if zlib.crc32(kind_bytes + value) != struct.unpack_from(">I", data, end - 4)[0]:
            raise ValueError("PNG chunk checksum failed")
        kind = kind_bytes.decode("ascii")
        if kind not in ("IHDR", "IDAT", "IEND"):
            total += length
            if total > MAX_METADATA:
                raise ValueError("pixel-exact metadata exceeds 1 MiB; use byte-exact")
            # Palette/transparency and unknown ancillary bytes remain in the sidecar.
            chunks.append({"type": kind, "data": b64(value), "embed": kind in PNG_EMBED})
        offset = end
        if kind == "IEND":
            if end != len(data):
                raise ValueError("PNG has trailing bytes; use byte-exact")
            break
    return chunks


def decode_pixels(data):
    """Stored raster, no orientation/profile conversion; no silent 16-bit RGB truncation."""
    import numpy as np
    from PIL import Image
    Image.MAX_IMAGE_PIXELS = MAX_PIXELS
    with Image.open(io.BytesIO(data)) as image:
        if image.format not in ("PNG", "JPEG", "WEBP"):
            raise ValueError("pixel-exact supports PNG/JPEG/WebP; use byte-exact for other formats")
        if image.width * image.height > MAX_PIXELS or max(image.size) > 16384:
            raise ValueError("pixel-exact canvas exceeds 16 megapixels / 16384 per side")
        if getattr(image, "n_frames", 1) != 1:
            raise ValueError("pixel-exact supports still images only; use byte-exact for animation")
        gray16 = data.startswith(PNG_SIGNATURE) and len(data) >= 26 and data[24:26] == b"\x10\x00"
        if data.startswith(PNG_SIGNATURE) and len(data) >= 26 and data[24] == 16 and not gray16:
            raise ValueError("16-bit multichannel PNG requires a lossless decoder; use byte-exact")
        if not gray16 and image.mode not in ("1", "L", "LA", "P", "RGB", "RGBA"):
            raise ValueError(f"unsupported pixel-exact {image.mode}; use byte-exact")
        image.load()
        descriptor = {"width": image.width, "height": image.height, "plane": "gray16le" if gray16 else "rgba8",
                      "channels": 1 if gray16 else 4, "bits": 16 if gray16 else 8,
                      "alpha": "none" if gray16 else "straight", "orientation": "stored-raster",
                      "color": "source-profile-no-conversion"}
        if gray16:
            pixels = np.asarray(image, dtype="<u2").tobytes()
        else:
            pixels = image.convert("RGBA").tobytes()
        metadata = {"format": image.format, "mode": image.mode, "chunks": [], "segments": []}
        if data.startswith(PNG_SIGNATURE):
            metadata["chunks"] = png_ancillary(data)
        else:
            # Known rendering metadata is embedded in exports; other JPEG APP/COM
            # data is retained verbatim in extraction.json, not interpreted.
            if image.info.get("icc_profile"):
                metadata["chunks"].append({"type": "iCCP", "data": b64(b"ICC\0\0" + zlib.compress(image.info["icc_profile"], 9)), "embed": True})
            if image.info.get("exif"):
                exif = image.info["exif"]
                if exif.startswith(b"Exif\0\0"):
                    exif = exif[6:]
                metadata["chunks"].append({"type": "eXIf", "data": b64(exif), "embed": True})
            for kind, value in getattr(image, "applist", []):
                metadata["segments"].append({"type": kind, "data": b64(value)})
        if len(json_bytes(metadata)) > MAX_METADATA:
            raise ValueError("pixel-exact metadata exceeds 1 MiB; use byte-exact")
        return pixels, descriptor, metadata


def encode_png(pixels, descriptor, metadata):
    """Version-1 export: filter 0, zlib level 9, RGBA8 or grayscale16 big endian."""
    import numpy as np
    width, height = descriptor["width"], descriptor["height"]
    gray = descriptor["plane"] == "gray16le"
    if gray:
        pixels = np.frombuffer(pixels, dtype="<u2").astype(">u2").tobytes()
    row_bytes = width * (2 if gray else 4)
    filtered = b"".join(b"\0" + pixels[y * row_bytes:(y + 1) * row_bytes] for y in range(height))
    result = PNG_SIGNATURE + png_chunk("IHDR", struct.pack(">IIBBBBB", width, height, 16 if gray else 8, 0 if gray else 6, 0, 0, 0))
    for item in metadata.get("chunks", []):
        if item["embed"]:
            result += png_chunk(item["type"], unb64(item["data"]))
    return result + png_chunk("IDAT", zlib.compress(filtered, 9)) + png_chunk("IEND", b"")


def geometry_options(base, target, hints):
    bw, bh, tw, th = base["width"], base["height"], target["width"], target["height"]
    if tw * bh <= th * bw:
        fw, fh = tw, max(1, bh * tw // bw)
    else:
        fw, fh = max(1, bw * th // bh), th
    options = [{"width": bw, "height": bh, "left": 0, "top": 0},
               {"width": bw, "height": bh, "left": (tw - bw) // 2, "top": (th - bh) // 2},
               {"width": fw, "height": fh, "left": (tw - fw) // 2, "top": (th - fh) // 2}]
    options.extend(hints)
    return [json.loads(value) for value in sorted({json_bytes(g) for g in options})]


def validate_geometry(geometry):
    if not isinstance(geometry, dict) or set(geometry) != {"width", "height", "left", "top"}:
        raise ValueError("invalid predictor geometry")
    for key in ("width", "height"):
        integer(geometry[key], 1, 16384, "predictor " + key)
    for key in ("left", "top"):
        integer(geometry[key], -16384, 16384, "predictor " + key)


def predict(base_pixels, base, target, geometry):
    """Integer floor nearest; zero outside canvas; never stretches aspect by default."""
    import numpy as np
    validate_geometry(geometry)
    if base["plane"] != target["plane"]:
        raise ValueError("predictor planes differ")
    dtype = "<u2" if base["plane"] == "gray16le" else "u1"
    channels = base["channels"]
    source = np.frombuffer(base_pixels, dtype=dtype).reshape(base["height"], base["width"], channels)
    output = np.zeros((target["height"], target["width"], channels), dtype=dtype)
    left, top, width, height = (geometry[k] for k in ("left", "top", "width", "height"))
    x0, y0 = max(0, left), max(0, top)
    x1, y1 = min(target["width"], left + width), min(target["height"], top + height)
    if x1 > x0 and y1 > y0:
        xs = (np.arange(x0, x1, dtype=np.int64) - left) * base["width"] // width
        ys = (np.arange(y0, y1, dtype=np.int64) - top) * base["height"] // height
        output[y0:y1, x0:x1] = source[ys[:, None], xs[None, :]]
    return output.tobytes()


@dataclass
class Prepared:
    entry: dict
    original: bytes
    pixels: bytes | None
    hints: dict
    lossless_bytes: int
    lossless_receipt: dict | None


def export_name(entry, fidelity):
    name = f"{entry['order']:04d}-" + entry["name"]
    return str(Path(name).with_suffix(".png")) if fidelity == "pixel-exact" else name


def export_record(entry, data, fidelity):
    return {"id": entry["id"], "name": export_name(entry, fidelity), "order": entry["order"],
            "sourceName": entry["sourceName"],
            "sourceSHA256": entry["sourceSHA256"], "sourceMD5": entry["sourceMD5"],
            "exportSHA256": sha(data), "exportMD5": hashlib.md5(data, usedforsecurity=False).hexdigest(),
            "metadata": entry.get("metadata"), "representationSHA256": entry["representationSHA256"]}


def source_image_info(data):
    from PIL import Image
    try:
        with Image.open(io.BytesIO(data)) as image:
            return {"width": image.width, "height": image.height, "format": image.format,
                    "mode": image.mode, "frames": getattr(image, "n_frames", 1),
                    "bits": data[24] if data.startswith(PNG_SIGNATURE) and len(data) >= 26 else (8 if image.format in ("JPEG", "WEBP") else None),
                    "orientation": int(image.getexif().get(274, 1)),
                    "alpha": "A" in image.mode or "transparency" in image.info}
    except (OSError, ValueError, Image.DecompressionBombError):
        return {"format": "unknown", "headerUnavailable": True}


def prepare(group, fidelity):
    members = group.get("members")
    if not isinstance(members, list) or not 1 <= len(members) <= MAX_MEMBERS:
        raise ValueError("a stack must contain 1–16 members")
    identities, result, budget = set(), [], 0
    for index, member in enumerate(members):
        if not isinstance(member, dict):
            raise ValueError("invalid member record")
        identity = member.get("id")
        if not isinstance(identity, str) or not 1 <= len(identity) <= 100 or identity in identities:
            raise ValueError("member IDs must be unique, non-empty strings of at most 100 characters")
        identities.add(identity)
        try:
            path = Path(member["path"])
            original = read_regular(path, MAX_FILE)
            name = member.get("name", path.name)
            if not isinstance(name, str) or not SAFE_NAME.fullmatch(name):
                raise ValueError("set a portable member name (letters, digits, spaces, dot, underscore, hyphen)")
            hints = member.get("predictions", {})
            if not isinstance(hints, dict) or len(hints) > MAX_MEMBERS:
                raise ValueError("invalid predictions")
            for geometries in hints.values():
                if not isinstance(geometries, list) or len(geometries) > 4:
                    raise ValueError("use at most four geometry hints per base")
                for geometry in geometries:
                    validate_geometry(geometry)
            entry = {"id": identity, "name": name, "sourceName": path.name, "order": index, "sourceSHA256": sha(original),
                     "sourceMD5": hashlib.md5(original, usedforsecurity=False).hexdigest(), "sourceBytes": len(original),
                     "sourceImage": source_image_info(original)}
            pixels = None
            lossless = 0
            lossless_receipt = None
            exported = None
            if fidelity == "pixel-exact":
                pixels, descriptor, metadata = decode_pixels(original)
                entry.update({"image": descriptor, "metadata": metadata, "metadataSHA256": sha(json_bytes(metadata))})
                exported = encode_png(pixels, descriptor, metadata)
                lossless = len(exported)
            payload = original if pixels is None else pixels
            entry.update({"representationSHA256": sha(payload), "representationBytes": len(payload)})
            if exported is not None:
                lossless_receipt = export_record(entry, exported, fidelity)
            budget += len(original) + (len(pixels) if pixels is not None else 0)
            if budget > MAX_GROUP:
                raise ValueError("stack source plus canonical planes exceed 256 MiB; split the stack")
            result.append(Prepared(entry, original, pixels, hints, lossless, lossless_receipt))
        except (ValueError, OSError, KeyError) as error:
            raise ValueError(f"{identity}: {error}") from error
    representative = group.get("representative", members[0]["id"])
    if representative not in identities:
        raise ValueError("representative must be a member; it does not select the compression base")
    return result, representative


def blob_record(data, encoding):
    return {"blob": sha(data), "blobBytes": len(data), "encoding": encoding}, data


def full_entry(member):
    data = member.original if member.pixels is None else member.pixels
    options = [blob_record(data, "raw"), blob_record(zlib.compress(data, 9), "zlib")]
    # Deterministic lossless PNG is an independent-storage competitor, not a delta.
    if member.pixels is not None:
        options.append(blob_record(encode_png(data, member.entry["image"], {"chunks": []}), "png"))
    record, payload = min(options, key=lambda item: (len(item[1]), item[0]["encoding"]))
    return {**member.entry, "kind": "full", **record}, payload


def archive_items(manifest, blobs):
    encoded = json_bytes(manifest)
    return {"manifest.json": encoded, "manifest.sha256": (sha(encoded) + "\n").encode(),
            **{"blobs/" + digest: payload for digest, payload in sorted(blobs.items())}}


def archive_size(manifest, blobs):
    # ZIP_STORED, no comments, extras or ZIP64: file data + local/central indexes.
    return 22 + sum(76 + 2 * len(name.encode()) + len(data) for name, data in archive_items(manifest, blobs).items())


def make_manifest(group_id, representative, fidelity, entries):
    from PIL import __version__ as pillow_version
    return {"product": "StashBooru-P12-prototype", "version": VERSION, "fidelity": fidelity,
            "algorithm": "xor-zlib-nearest-floor-v1", "group": group_id, "representative": representative,
            "decoder": {"name": "Pillow", "version": pillow_version, "usedForReconstruction": fidelity == "pixel-exact"},
            "exporter": "png-filter0-zlib9-v1", "compressor": {"name": "zlib", "level": 9, "version": zlib.ZLIB_RUNTIME_VERSION},
            "entries": sorted(entries, key=lambda item: item["order"])}


def marginal(entry, payload, blobs):
    cost = len(json_bytes(entry))
    if payload is not None and entry["blob"] not in blobs:
        cost += len(payload) + 76 + 2 * len("blobs/" + entry["blob"])
    return cost


def plan_group(group, fidelity="byte-exact", min_bytes=0, min_percent=5.0):
    if fidelity not in ("byte-exact", "pixel-exact"):
        raise ValueError("invalid fidelity")
    integer(min_bytes, 0, MAX_PACK, "minimum bytes")
    if not math.isfinite(min_percent) or not 0 <= min_percent <= 100:
        raise ValueError("minimum percent must be finite and between 0 and 100")
    started = time.monotonic()
    members, representative = prepare(group, fidelity)
    group_id = group.get("id", "stack")
    if not isinstance(group_id, str) or not 1 <= len(group_id) <= 100:
        raise ValueError("invalid stack ID")
    full = [full_entry(member) for member in members]
    independent_blobs = {entry["blob"]: payload for entry, payload in full}
    independent = make_manifest(group_id, representative, fidelity, [entry for entry, _ in full])
    independent_size = archive_size(independent, independent_blobs)
    best, best_blobs, best_base = independent, independent_blobs, None
    best_size, measurements = independent_size, []
    for base_index, base_member in enumerate(members):
        check_deadline(started)
        base_entry, base_payload = full[base_index]
        entries, blobs = [base_entry], {base_entry["blob"]: base_payload}
        for index, member in enumerate(members):
            if index == base_index:
                continue
            check_deadline(started)
            own, own_payload = full[index]
            alternatives = [(own, own_payload)]
            target = member.original if member.pixels is None else member.pixels
            base = base_member.original if base_member.pixels is None else base_member.pixels
            compatible = member.pixels is None or base_member.entry["image"]["plane"] == member.entry["image"]["plane"]
            if compatible:
                geometries = [None] if member.pixels is None else geometry_options(base_member.entry["image"], member.entry["image"], member.hints.get(base_member.entry["id"], []))
                for geometry in geometries:
                    prediction = base if geometry is None else predict(base, base_member.entry["image"], member.entry["image"], geometry)
                    residual = xor_bytes(target, prediction)
                    # Never discard low-amplitude differences, borders or transparent RGB.
                    if xor_bytes(residual, prediction) != target:
                        raise ValueError("internal delta reconstruction failed")
                    record, payload = blob_record(zlib.compress(residual, 9), "zlib")
                    delta = {**member.entry, "kind": "delta", "base": base_entry["id"], **record}
                    if geometry is not None:
                        delta["geometry"] = geometry
                    alternatives.append((delta, payload))
                    if target == prediction:
                        identical = {**member.entry, "kind": "copy", "base": base_entry["id"]}
                        if geometry is not None:
                            identical["geometry"] = geometry
                        alternatives.append((identical, None))
            entry, payload = min(alternatives, key=lambda item: (marginal(item[0], item[1], blobs), item[0]["kind"]))
            entries.append(entry)
            if payload is not None:
                blobs[entry["blob"]] = payload
        manifest = make_manifest(group_id, representative, fidelity, entries)
        size = archive_size(manifest, blobs)
        verified_at = time.monotonic()
        identities, _ = validate_manifest(manifest)
        for member in members:
            check_deadline(started)
            restored = reconstruct(manifest, identities, lambda digest: blobs[digest], member.entry["id"])
            expected = member.original if member.pixels is None else member.pixels
            if restored != expected:
                raise ValueError("candidate base failed full reconstruction")
        measurements.append({"base": base_entry["id"], "archiveBytes": size,
                             "deltaMembers": sum(e["kind"] != "full" for e in entries),
                             "verified": True, "reconstructSeconds": time.monotonic() - verified_at})
        if size < best_size:
            best, best_blobs, best_base, best_size = manifest, blobs, base_entry["id"], size
    original_bytes = sum(len(member.original) for member in members)
    lossless_bytes = sum(member.lossless_bytes for member in members) if fidelity == "pixel-exact" else None
    lossless_sidecar = len(json_bytes({"version": VERSION, "fidelity": fidelity, "complete": True,
                                      "members": [member.lossless_receipt for member in members]})) if fidelity == "pixel-exact" else None
    lossless_total = lossless_bytes + lossless_sidecar if lossless_bytes is not None else None
    baseline = original_bytes if lossless_total is None else min(original_bytes, lossless_total)
    improvement = independent_size - best_size
    net_savings = baseline - best_size
    eligible = improvement > 0 and net_savings > 0 and improvement >= min_bytes and improvement * 100 >= independent_size * min_percent
    reason = "delta meets both minima and saves bytes against independently stored files" if eligible else "keep independent storage: no net savings or delta improvement below minima"
    candidate_size = best_size
    if not eligible:
        best, best_blobs, best_base, best_size = independent, independent_blobs, None, independent_size
    from PIL import __version__ as pillow_version
    import numpy as np
    report = {"group": group_id, "fidelity": fidelity, "members": len(members), "representative": representative,
              "compressionBase": best_base, "layout": "shared-base" if eligible else "independent",
              "reason": reason, "originalFileBytes": original_bytes, "independentLosslessPNGBytes": lossless_bytes,
              "independentLosslessSidecarBytes": lossless_sidecar, "independentLosslessFileBytes": lossless_total,
              "independentArchiveBytes": independent_size, "bestCandidateArchiveBytes": candidate_size,
              "archiveBytes": best_size, "uniqueBlobBytes": sum(map(len, best_blobs.values())),
              "manifestAndIndexBytes": best_size - sum(map(len, best_blobs.values())),
              "savingsVsOriginalBytes": original_bytes - best_size,
              "savingsVsIndependentArchiveBytes": independent_size - best_size,
              "retainedOriginalsPlusArchiveBytes": original_bytes + best_size,
              "cacheBytes": 0, "backupBytes": 0, "sourcesRetained": True, "eligibleDelta": eligible,
              "minimumDeltaBytes": min_bytes, "minimumDeltaPercent": min_percent, "baseCandidates": measurements,
              "encodeSeconds": time.monotonic() - started,
              "runtime": {"python": sys.version.split()[0], "pillow": pillow_version, "numpy": np.__version__, "zlib": zlib.ZLIB_RUNTIME_VERSION}}
    return best, best_blobs, report


def validate_metadata(metadata):
    if not isinstance(metadata, dict) or len(json_bytes(metadata)) > MAX_METADATA:
        raise ValueError("invalid or oversized metadata")
    if not isinstance(metadata.get("chunks"), list) or not isinstance(metadata.get("segments"), list):
        raise ValueError("invalid metadata lists")
    for item in metadata["chunks"]:
        if not isinstance(item, dict) or not re.fullmatch(r"[A-Za-z]{4}", item.get("type", "")) or not isinstance(item.get("embed"), bool):
            raise ValueError("invalid PNG metadata chunk")
        if item["embed"] and item["type"] not in PNG_EMBED:
            raise ValueError("unsafe PNG export metadata")
        unb64(item["data"])
    for item in metadata["segments"]:
        if not isinstance(item, dict) or not isinstance(item.get("type"), str):
            raise ValueError("invalid metadata segment")
        unb64(item["data"])


def validate_manifest(manifest):
    if manifest.get("product") != "StashBooru-P12-prototype" or manifest.get("version") != VERSION or manifest.get("algorithm") != "xor-zlib-nearest-floor-v1":
        raise ValueError("unsupported P12 product/version/algorithm")
    fidelity = manifest.get("fidelity")
    if fidelity not in ("byte-exact", "pixel-exact"):
        raise ValueError("invalid fidelity")
    entries = manifest.get("entries")
    if not isinstance(entries, list) or not 1 <= len(entries) <= MAX_MEMBERS:
        raise ValueError("invalid member count")
    identities, orders, total, blobs = {}, set(), 0, set()
    for entry in entries:
        if not isinstance(entry, dict):
            raise ValueError("invalid member")
        identity = entry.get("id")
        if not isinstance(identity, str) or not 1 <= len(identity) <= 100 or identity in identities:
            raise ValueError("invalid or duplicate member ID")
        if not isinstance(entry.get("name"), str) or not SAFE_NAME.fullmatch(entry["name"]):
            raise ValueError("unsafe export name")
        if not isinstance(entry.get("sourceName"), str) or not 1 <= len(entry["sourceName"]) <= 255:
            raise ValueError("invalid original filename provenance")
        order = integer(entry.get("order"), 0, len(entries) - 1, "member order")
        if order in orders:
            raise ValueError("duplicate member order")
        orders.add(order)
        identities[identity] = entry
        length = integer(entry.get("representationBytes"), 0, MAX_FILE, "representation size")
        integer(entry.get("sourceBytes"), 0, MAX_FILE, "source size")
        total += length
        for key in ("sourceSHA256", "representationSHA256"):
            if not isinstance(entry.get(key), str) or not HASH.fullmatch(entry[key]):
                raise ValueError("invalid member hash")
        if not re.fullmatch(r"[0-9a-f]{32}", entry.get("sourceMD5", "")):
            raise ValueError("invalid source MD5 provenance")
        kind = entry.get("kind")
        if kind not in ("full", "delta", "copy"):
            raise ValueError("invalid member encoding")
        if kind != "copy":
            if not isinstance(entry.get("blob"), str) or not HASH.fullmatch(entry["blob"]):
                raise ValueError("invalid content address")
            if entry.get("encoding") not in ("raw", "zlib", "png") or (kind == "delta" and entry["encoding"] != "zlib"):
                raise ValueError("invalid payload codec")
            if fidelity == "byte-exact" and entry["encoding"] == "png":
                raise ValueError("PNG encoding cannot reconstruct original file bytes")
            integer(entry.get("blobBytes"), 0, MAX_FILE, "encoded payload length")
            blobs.add("blobs/" + entry["blob"])
        if fidelity == "byte-exact":
            if length != entry["sourceBytes"] or entry["representationSHA256"] != entry["sourceSHA256"] or "geometry" in entry:
                raise ValueError("byte-exact source/representation mismatch")
        else:
            image = entry.get("image")
            if not isinstance(image, dict) or image.get("plane") not in ("rgba8", "gray16le"):
                raise ValueError("invalid pixel plane")
            width = integer(image.get("width"), 1, 16384, "image width")
            height = integer(image.get("height"), 1, 16384, "image height")
            gray = image["plane"] == "gray16le"
            if width * height > MAX_PIXELS or length != width * height * (2 if gray else 4):
                raise ValueError("invalid pixel canvas/length")
            if image.get("channels") != (1 if gray else 4) or image.get("bits") != (16 if gray else 8) or image.get("alpha") != ("none" if gray else "straight") or image.get("orientation") != "stored-raster" or image.get("color") != "source-profile-no-conversion":
                raise ValueError("unsupported canonical pixel contract")
            validate_metadata(entry.get("metadata"))
            if sha(json_bytes(entry["metadata"])) != entry.get("metadataSHA256"):
                raise ValueError("metadata checksum mismatch")
            if kind != "full":
                validate_geometry(entry.get("geometry"))
    if total > MAX_GROUP:
        raise ValueError("reconstruction budget exceeds 256 MiB")
    if manifest.get("representative") not in identities:
        raise ValueError("missing representative")
    for entry in entries:
        if entry["kind"] != "full":
            base = identities.get(entry.get("base"))
            if base is None or base["kind"] != "full":
                raise ValueError("base must be an independent member; chains/cycles are forbidden")
            if fidelity == "pixel-exact" and base["image"]["plane"] != entry["image"]["plane"]:
                raise ValueError("incompatible base pixel plane")
    return identities, blobs


def reconstruct(manifest, entries, read_blob, identity):
    if identity not in entries:
        raise ValueError("unknown member ID")
    entry = entries[identity]
    if entry["kind"] == "copy":
        result = reconstruct(manifest, entries, read_blob, entry["base"])
    else:
        payload = read_blob(entry["blob"])
        if len(payload) != entry["blobBytes"] or sha(payload) != entry["blob"]:
            raise ValueError("blob checksum/length mismatch")
        if entry["encoding"] == "zlib":
            result = decompress(payload, entry["representationBytes"])
        elif entry["encoding"] == "png":
            result, descriptor, _ = decode_pixels(payload)
            if descriptor != entry["image"]:
                raise ValueError("independent PNG canvas mismatch")
        else:
            result = payload
    if entry["kind"] != "full":
        base_entry = entries[entry["base"]]
        base = result if entry["kind"] == "copy" else reconstruct(manifest, entries, read_blob, entry["base"])
        prediction = base if manifest["fidelity"] == "byte-exact" else predict(base, base_entry["image"], entry["image"], entry["geometry"])
        result = prediction if entry["kind"] == "copy" else xor_bytes(result, prediction)
    if len(result) != entry["representationBytes"] or sha(result) != entry["representationSHA256"]:
        raise ValueError("reconstructed member checksum/length mismatch")
    return result


class PackReader:
    def __init__(self, path):
        self.path = Path(path)
        self.archive = None
        try:
            # Open once so a replaced path cannot swap the checked archive.
            source = os.fdopen(os.open(self.path, os.O_RDONLY | getattr(os, "O_NONBLOCK", 0)), "rb")
            self.source = source
            import stat
            file_size = os.fstat(source.fileno()).st_size
            if not stat.S_ISREG(os.fstat(source.fileno()).st_mode) or file_size > MAX_PACK:
                raise ValueError("archive is not regular or exceeds 384 MiB")
            # Bound the central directory before zipfile allocates its entry list.
            # Our format deliberately excludes ZIP64, comments and spanning disks.
            if file_size < 22:
                raise ValueError("incomplete P12 archive")
            source.seek(-22, os.SEEK_END)
            end = struct.unpack("<4s4H2IH", source.read(22))
            signature, disk, directory_disk, disk_count, count, directory_size, directory_offset, comment_size = end
            if signature != b"PK\x05\x06" or disk or directory_disk or comment_size or disk_count != count or not 2 <= count <= MAX_MEMBERS + 2 or directory_size > 8192 or directory_offset + directory_size != file_size - 22:
                raise ValueError("invalid or oversized archive directory")
            source.seek(0)
            self.archive = zipfile.ZipFile(source)
            items = self.archive.infolist()
            if len(items) != count or len(items) > MAX_MEMBERS + 2 or len({item.filename for item in items}) != len(items):
                raise ValueError("duplicate or excessive archive entries")
            for item in items:
                if item.compress_type != zipfile.ZIP_STORED or item.flag_bits & 1 or item.extra or item.is_dir():
                    raise ValueError("archive must contain plain stored files only")
                if item.filename not in ("manifest.json", "manifest.sha256") and not re.fullmatch(r"blobs/[0-9a-f]{64}", item.filename):
                    raise ValueError("unsafe archive path")
                limit = MAX_MANIFEST if item.filename == "manifest.json" else (65 if item.filename == "manifest.sha256" else MAX_FILE)
                if item.file_size > limit or item.compress_size != item.file_size:
                    raise ValueError("oversized archive entry")
            raw = self.archive.read("manifest.json")
            if self.archive.read("manifest.sha256") != (sha(raw) + "\n").encode():
                raise ValueError("manifest checksum mismatch")
            self.manifest = json.loads(raw)
            if not isinstance(self.manifest, dict):
                raise ValueError("invalid manifest")
            self.entries, wanted = validate_manifest(self.manifest)
            actual = {item.filename for item in items} - {"manifest.json", "manifest.sha256"}
            if actual - wanted:
                raise ValueError("archive contains unreferenced blobs")
        except BaseException:
            self.close()
            raise

    def close(self):
        if self.archive is not None:
            self.archive.close()
        if hasattr(self, "source"):
            self.source.close()

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        self.close()

    def reconstruct(self, identity):
        return reconstruct(self.manifest, self.entries, lambda digest: self.archive.read("blobs/" + digest), identity)

    def verify(self, identity=None):
        started = time.monotonic()
        if identity is not None and identity not in self.entries:
            raise ValueError("unknown member ID")
        selected = [identity] if identity is not None else list(self.entries)
        members = []
        for member in selected:
            check_deadline(started)
            try:
                result = self.reconstruct(member)
                entry = self.entries[member]
                if self.manifest["fidelity"] == "pixel-exact":
                    exported = encode_png(result, entry["image"], entry["metadata"])
                    pixels, descriptor, metadata = decode_pixels(exported)
                    if pixels != result or descriptor != entry["image"]:
                        raise ValueError("portable PNG export changed pixels")
                    wanted = [c for c in entry["metadata"]["chunks"] if c["embed"]]
                    if metadata["chunks"] != wanted:
                        raise ValueError("portable PNG export changed rendering metadata")
                members.append({"id": member, "verified": True, "bytes": len(result), "dependsOn": self.entries[member].get("base")})
            except (ValueError, OSError, KeyError, zipfile.BadZipFile, zlib.error) as error:
                members.append({"id": member, "verified": False, "error": str(error), "dependsOn": self.entries[member].get("base")})
        return {"fidelity": self.manifest["fidelity"], "verified": all(m["verified"] for m in members),
                "members": members, "verifySeconds": time.monotonic() - started}


def fsync_parent(path):
    if os.name != "nt":
        descriptor = os.open(Path(path).parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)


def publish(staging, output):
    # Hard-link publication is atomic and cannot overwrite another writer's file.
    os.link(staging, output)
    fsync_parent(output)


def write_pack(manifest, blobs, output, checkpoint=lambda _phase, _path: None):
    validate_manifest(manifest)
    output = Path(output)
    if output.exists() or output.is_symlink():
        raise ValueError("output already exists; choose a new archive path")
    expected = archive_size(manifest, blobs)
    if expected > MAX_PACK:
        raise ValueError("archive exceeds 384 MiB")
    fd, name = tempfile.mkstemp(prefix=".p12-", suffix=".staging", dir=output.parent)
    staging = Path(name)
    os.close(fd)
    try:
        checkpoint("created", staging)
        with zipfile.ZipFile(staging, "w", compression=zipfile.ZIP_STORED, allowZip64=False) as archive:
            for name, data in archive_items(manifest, blobs).items():
                info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
                info.external_attr = 0o100600 << 16
                archive.writestr(info, data)
                checkpoint("entry", staging)
        with staging.open("rb") as source:
            os.fsync(source.fileno())
        if staging.stat().st_size != expected:
            raise ValueError("archive overhead accounting disagrees with written size")
        checkpoint("durable", staging)
        with PackReader(staging) as reader:
            verification = reader.verify()
        if not verification["verified"]:
            raise ValueError("candidate verification failed: " + json.dumps(verification["members"]))
        checkpoint("verified", staging)
        publish(staging, output)
        checkpoint("published", staging)
        return {"archiveBytes": expected, **verification}
    finally:
        staging.unlink(missing_ok=True)


def recover(staging, output):
    staging, output = Path(staging), Path(output)
    if staging.resolve() == output.resolve():
        raise ValueError("staging and output must differ")
    with PackReader(staging) as reader:
        verification = reader.verify()
    if not verification["verified"]:
        raise ValueError("staging archive is incomplete/corrupt; originals remain authoritative")
    publish(staging, output)
    # Leave supplied recovery input intact; the caller owns it.
    return verification


def extract(pack, output, identity=None, checkpoint=lambda _phase, _path: None):
    started = time.monotonic()
    output = Path(output)
    with PackReader(pack) as reader:
        if identity is not None and identity not in reader.entries:
            raise ValueError("unknown member ID")
        selected = [reader.entries[identity]] if identity is not None else sorted(reader.entries.values(), key=lambda e: e["order"])
        # Exclusive mkdir refuses existing directories and symlinks. extraction.json
        # is the completion marker; a killed extraction can be rerun into a new dir.
        output.mkdir(mode=0o700)
        exports = []
        try:
            checkpoint("created", output)
            for entry in selected:
                check_deadline(started)
                result = reader.reconstruct(entry["id"])
                filename = export_name(entry, reader.manifest["fidelity"])
                if reader.manifest["fidelity"] == "pixel-exact":
                    result = encode_png(result, entry["image"], entry["metadata"])
                    pixels, descriptor, _ = decode_pixels(result)
                    if sha(pixels) != entry["representationSHA256"] or descriptor != entry["image"]:
                        raise ValueError("export verification failed")
                destination = output / filename
                with destination.open("xb") as file:
                    file.write(result)
                    file.flush()
                    os.fsync(file.fileno())
                exports.append(export_record(entry, result, reader.manifest["fidelity"]))
                checkpoint("member", output)
            receipt = {"version": VERSION, "fidelity": reader.manifest["fidelity"], "complete": True, "members": exports}
            with (output / "extraction.json").open("xb") as file:
                file.write(json_bytes(receipt))
                file.flush()
                os.fsync(file.fileno())
            fsync_parent(output / "extraction.json")
            fsync_parent(output)
            checkpoint("complete", output)
            return receipt
        except BaseException:
            shutil.rmtree(output)
            raise


def read_stacks(path):
    path = Path(path)
    value = json.loads(read_regular(path, MAX_MANIFEST))
    groups = value.get("stacks") if isinstance(value, dict) else None
    if not isinstance(groups, list) or not 1 <= len(groups) <= MAX_GROUPS:
        raise ValueError("input must contain 1–32 stacks")
    identities = set()
    for group in groups:
        if not isinstance(group, dict) or not isinstance(group.get("id"), str) or group["id"] in identities:
            raise ValueError("stack IDs must be unique strings")
        identities.add(group["id"])
        for member in group.get("members", []):
            if not isinstance(member, dict) or not isinstance(member.get("path"), str):
                raise ValueError("member needs an explicit source path")
            source = Path(member["path"])
            if not source.is_absolute():
                member["path"] = str(path.parent / source)
    return groups


def save_json(value, path):
    with Path(path).open("xb") as output:
        output.write(json_bytes(value) + b"\n")


def benchmark(groups, min_bytes=0, min_percent=5.0, verification_dir=None):
    integer(min_bytes, 0, MAX_PACK, "minimum bytes")
    if not math.isfinite(min_percent) or not 0 <= min_percent <= 100:
        raise ValueError("minimum percent must be finite and between 0 and 100")
    results = []
    started = time.monotonic()
    for group in groups:
        for fidelity in ("byte-exact", "pixel-exact"):
            check_deadline(started)
            try:
                manifest, blobs, report = plan_group(group, fidelity, min_bytes, min_percent)
                with tempfile.TemporaryDirectory(prefix="p12-benchmark-", dir=verification_dir) as root:
                    checked = write_pack(manifest, blobs, Path(root) / "candidate.p12")
                report.update({"verified": checked["verified"], "verifySeconds": checked["verifySeconds"]})
                results.append(report)
            except (ValueError, OSError, KeyError, TypeError, zipfile.BadZipFile, zlib.error) as error:
                results.append({"group": group["id"], "fidelity": fidelity, "verified": False, "error": str(error)})
    return {"version": VERSION, "experimental": True, "sourcesRetained": True, "results": results}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    for command in ("benchmark", "pack"):
        current = sub.add_parser(command)
        current.add_argument("--stacks", required=True)
        current.add_argument("--output", required=True)
        current.add_argument("--min-bytes", type=int, default=0)
        current.add_argument("--min-percent", type=float, default=5.0)
        if command == "pack":
            current.add_argument("--group", required=True)
            current.add_argument("--fidelity", choices=("byte-exact", "pixel-exact"), default="byte-exact")
    for command in ("verify", "extract"):
        current = sub.add_parser(command)
        current.add_argument("archive")
        current.add_argument("--entry")
        if command == "extract":
            current.add_argument("--output", required=True)
    current = sub.add_parser("recover")
    current.add_argument("staging")
    current.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        require_dependencies()
        if args.command == "benchmark":
            result = benchmark(read_stacks(args.stacks), args.min_bytes, args.min_percent)
            save_json(result, args.output)
            result["verified"] = all(row.get("verified", False) for row in result["results"])
        elif args.command == "pack":
            groups = read_stacks(args.stacks)
            selected = next((group for group in groups if group["id"] == args.group), None)
            if selected is None:
                raise ValueError("unknown stack ID")
            manifest, blobs, result = plan_group(selected, args.fidelity, args.min_bytes, args.min_percent)
            result.update(write_pack(manifest, blobs, args.output))
        elif args.command == "verify":
            with PackReader(args.archive) as reader:
                result = reader.verify(args.entry)
        elif args.command == "extract":
            result = extract(args.archive, args.output, args.entry)
        else:
            result = recover(args.staging, args.output)
        print(json.dumps(result, sort_keys=True, allow_nan=False))
        return 0 if result.get("verified", True) else 1
    except KeyboardInterrupt:
        print("Cancelled; originals are untouched. Completed archives remain recoverable.", file=sys.stderr)
        return 130
    except (ValueError, OSError, KeyError, TypeError, zipfile.BadZipFile, zlib.error) as error:
        print(str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
