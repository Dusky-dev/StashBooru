#!/usr/bin/env python3
"""P13 bounded, offline gallery sharing prototype. Originals are always retained.

Byte-exact archives use fixed or content-defined chunks; pixel-exact archives
use exact tiles. Each member references immutable chunks directly, never pages.
"""
from __future__ import annotations

import argparse
import hashlib
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

import image_delta_storage as base

VERSION = 1
PRODUCT = "StashBooru-P13-prototype"
MAX_CHUNKS = 8192
MAX_REFERENCES = 8192
MAX_INDEX = 4 * 1024 * 1024
MAX_MEMBER_MANIFEST = 2 * 1024 * 1024
MAX_DIRECTORY = 2 * 1024 * 1024
FIXED_SIZES = (4096, 16384, 65536)
CDC_SIZES = (8192, 32768, 65536)
TILE_SIZES = (32, 64, 128)
GEAR = tuple(int.from_bytes(hashlib.sha256(f"p13-gear-v1:{n}".encode()).digest()[:8], "little") for n in range(256))


def deadline(started):
    if time.monotonic() - started > base.MAX_SECONDS:
        raise ValueError("P13 operation exceeded 15 minutes; split the gallery")


def thresholds(min_bytes, min_percent):
    base.integer(min_bytes, 0, base.MAX_PACK, "minimum bytes")
    if isinstance(min_percent, bool) or not isinstance(min_percent, (int, float)) or not math.isfinite(min_percent) or not 0 <= min_percent <= 100:
        raise ValueError("minimum percent must be finite and between 0 and 100")


def algorithms(fidelity):
    if fidelity == "byte-exact":
        return [{"kind": "fixed", "size": size} for size in FIXED_SIZES] + [{"kind": "cdc", "size": size} for size in CDC_SIZES]
    if fidelity == "pixel-exact":
        return [{"kind": "tile", "size": size} for size in TILE_SIZES]
    raise ValueError("invalid fidelity")


def validate_algorithm(algorithm, fidelity):
    if algorithm == {"kind": "independent"}:
        return
    if algorithm not in algorithms(fidelity):
        raise ValueError("unsupported chunk algorithm/size/version")


def byte_chunks(data, algorithm):
    size = algorithm["size"]
    if algorithm["kind"] == "fixed":
        for offset in range(0, len(data), size):
            yield data[offset:offset + size]
        return
    # Gear-64 v1: reset at each boundary; min=target/4, max=target*4.
    # Fixed constants above and a power-of-two mask make boundaries portable.
    start, rolling = 0, 0
    for index, value in enumerate(data):
        rolling = ((rolling << 1) + GEAR[value]) & ((1 << 64) - 1)
        length = index + 1 - start
        if length >= size * 4 or (length >= size // 4 and rolling & (size - 1) == 0):
            yield data[start:index + 1]
            start, rolling = index + 1, 0
    if start < len(data):
        yield data[start:]


def tile_chunks(pixels, image, size):
    import numpy as np
    bpp = 2 if image["plane"] == "gray16le" else 4
    plane = np.frombuffer(pixels, dtype="u1").reshape(image["height"], image["width"], bpp)
    for top in range(0, image["height"], size):
        for left in range(0, image["width"], size):
            yield plane[top:top + size, left:left + size].tobytes()


def encode_block(data):
    compressed = zlib.compress(data, 9)
    payload, encoding = (compressed, "zlib") if len(compressed) < len(data) else (data, "raw")
    record = {"encoding": encoding, "blobBytes": len(payload), "decodedBytes": len(data), "decodedSHA256": base.sha(data)}
    return chunk_address(record, payload), record, payload


def chunk_address(record, payload):
    # Include the decoding contract: raw bytes can themselves be another chunk's
    # zlib stream. Encoded-payload hashes alone cannot distinguish those meanings.
    return base.sha(b"P13-chunk-v1\0" + base.json_bytes(record) + b"\0" + payload)


def add_member(prepared, algorithm, fidelity, chunks, blobs):
    entry = dict(prepared.entry)
    entry["losslessExportBytes"] = prepared.lossless_bytes if fidelity == "pixel-exact" else len(prepared.original)
    # Full receipt bytes are included in the standalone pixel baseline.
    entry["losslessReceipt"] = prepared.lossless_receipt
    if algorithm["kind"] == "independent":
        full, payload = base.full_entry(prepared)
        record = {"encoding": full["encoding"], "blobBytes": len(payload), "decodedBytes": entry["representationBytes"], "decodedSHA256": entry["representationSHA256"]}
        digest = chunk_address(record, payload)
        if digest in chunks and chunks[digest] != record:
            raise ValueError("conflicting content address")
        chunks[digest] = record
        blobs[digest] = payload
        entry["blocks"] = [digest]
    else:
        data = prepared.original if fidelity == "byte-exact" else prepared.pixels
        pieces = byte_chunks(data, algorithm) if fidelity == "byte-exact" else tile_chunks(data, entry["image"], algorithm["size"])
        entry["blocks"] = []
        for piece in pieces:
            digest, record, payload = encode_block(piece)
            if digest in chunks and chunks[digest] != record:
                raise ValueError("conflicting content address")
            chunks[digest], blobs[digest] = record, payload
            entry["blocks"].append(digest)
            if len(chunks) > MAX_CHUNKS or len(entry["blocks"]) > MAX_REFERENCES:
                raise ValueError("chunk/reference budget exceeded; use larger blocks or split the gallery")
    return entry


def make_index(gallery_id, representative, fidelity, algorithm, members, chunks, generation=1, parent=None):
    from PIL import __version__ as pillow_version
    refs = []
    for entry in sorted(members, key=lambda item: item["order"]):
        raw = base.json_bytes(entry)
        if len(raw) > MAX_MEMBER_MANIFEST:
            raise ValueError("member manifest exceeds 2 MiB")
        refs.append({"id": entry["id"], "order": entry["order"], "manifest": base.sha(raw), "manifestBytes": len(raw)})
    index = {"product": PRODUCT, "version": VERSION, "gallery": gallery_id, "representative": representative,
             "fidelity": fidelity, "algorithm": algorithm, "generation": generation, "parentSHA256": parent,
             "decoder": {"name": "Pillow", "version": pillow_version, "usedForReconstruction": fidelity == "pixel-exact"},
             "exporter": "png-filter0-zlib9-v1", "compressor": {"name": "zlib", "level": 9, "version": zlib.ZLIB_RUNTIME_VERSION},
             "members": refs, "chunks": chunks}
    if len(base.json_bytes(index)) > MAX_INDEX:
        raise ValueError("gallery index exceeds 4 MiB")
    return index


def items(index, members, blobs):
    raw = base.json_bytes(index)
    return {"gallery.json": raw, "gallery.sha256": (base.sha(raw) + "\n").encode(),
            **{"members/" + base.sha(base.json_bytes(entry)) + ".json": base.json_bytes(entry) for entry in members},
            **{"chunks/" + digest: payload for digest, payload in sorted(blobs.items())}}


def entry_cost(name, data):
    return len(data) + 76 + 2 * len(name.encode())


def archive_size(index, members, blobs):
    return 22 + sum(entry_cost(name, data) for name, data in items(index, members, blobs).items())


def validate_index(index, members):
    if not isinstance(index, dict) or index.get("product") != PRODUCT or type(index.get("version")) is not int or index["version"] != VERSION:
        raise ValueError("unsupported P13 product/version")
    fidelity = index.get("fidelity")
    validate_algorithm(index.get("algorithm"), fidelity)
    if fidelity not in ("byte-exact", "pixel-exact"):
        raise ValueError("invalid fidelity")
    if not isinstance(index.get("gallery"), str) or not 1 <= len(index["gallery"]) <= 100:
        raise ValueError("invalid gallery ID")
    decoder, compressor = index.get("decoder"), index.get("compressor")
    if (index.get("exporter") != "png-filter0-zlib9-v1" or not isinstance(decoder, dict) or decoder.get("name") != "Pillow"
            or decoder.get("usedForReconstruction") is not (fidelity == "pixel-exact") or not isinstance(decoder.get("version"), str)
            or not 1 <= len(decoder["version"]) <= 64 or not isinstance(compressor, dict) or compressor.get("name") != "zlib"
            or type(compressor.get("level")) is not int or compressor["level"] != 9 or not isinstance(compressor.get("version"), str)
            or not 1 <= len(compressor["version"]) <= 64):
        raise ValueError("unsupported decoder/exporter/compressor contract")
    base.integer(index.get("generation"), 1, 1000000, "generation")
    parent = index.get("parentSHA256")
    if (index["generation"] == 1 and parent is not None) or (index["generation"] > 1 and (not isinstance(parent, str) or not base.HASH.fullmatch(parent))):
        raise ValueError("invalid revision provenance")
    refs = index.get("members")
    if not isinstance(refs, list) or not 1 <= len(refs) <= base.MAX_MEMBERS or len(members) != len(refs):
        raise ValueError("a bounded gallery must have 1–16 members")
    chunks = index.get("chunks")
    if not isinstance(chunks, dict) or len(chunks) > MAX_CHUNKS:
        raise ValueError("invalid chunk index")
    for digest, record in chunks.items():
        if not isinstance(digest, str) or not base.HASH.fullmatch(digest) or not isinstance(record, dict) or record.get("encoding") not in ("raw", "zlib", "png"):
            raise ValueError("invalid chunk address/codec")
        if set(record) != {"encoding", "blobBytes", "decodedBytes", "decodedSHA256"} or not isinstance(record["decodedSHA256"], str) or not base.HASH.fullmatch(record["decodedSHA256"]):
            raise ValueError("invalid chunk record")
        base.integer(record["blobBytes"], 0, base.MAX_FILE, "encoded chunk size")
        base.integer(record["decodedBytes"], 0, base.MAX_FILE, "decoded chunk size")
        if record["encoding"] == "raw" and record["blobBytes"] != record["decodedBytes"]:
            raise ValueError("inconsistent raw chunk")
        if record["encoding"] == "png" and (fidelity != "pixel-exact" or index["algorithm"]["kind"] != "independent"):
            raise ValueError("PNG chunks require independent pixel storage")
    proxies, wanted, seen_manifests, total_refs = [], set(), set(), 0
    for position, (ref, entry) in enumerate(zip(refs, members)):
        if not isinstance(ref, dict) or not isinstance(entry, dict) or ref.get("id") != entry.get("id") or ref.get("order") != position or entry.get("order") != position:
            raise ValueError("member index/order mismatch")
        base.integer(ref["order"], 0, len(refs) - 1, "indexed member order")
        raw = base.json_bytes(entry)
        if len(raw) > MAX_MEMBER_MANIFEST or ref.get("manifestBytes") != len(raw) or ref.get("manifest") != base.sha(raw) or ref["manifest"] in seen_manifests:
            raise ValueError("member manifest checksum/length mismatch")
        seen_manifests.add(ref["manifest"])
        blocks = entry.get("blocks")
        if not isinstance(blocks, list) or any(not isinstance(digest, str) or digest not in chunks for digest in blocks):
            raise ValueError("unknown member chunk")
        total_refs += len(blocks)
        if total_refs > MAX_REFERENCES:
            raise ValueError("gallery reference budget exceeded")
        wanted.update(blocks)
        length = base.integer(entry.get("representationBytes"), 0, base.MAX_FILE, "member bytes")
        if index["algorithm"]["kind"] == "independent":
            if len(blocks) != 1 or chunks[blocks[0]]["decodedBytes"] != length or chunks[blocks[0]]["decodedSHA256"] != entry.get("representationSHA256"):
                raise ValueError("invalid independent member")
        elif fidelity == "byte-exact":
            size = index["algorithm"]["size"]
            if sum(chunks[d]["decodedBytes"] for d in blocks) != length:
                raise ValueError("chunk lengths do not cover member")
            for n, digest in enumerate(blocks):
                decoded = chunks[digest]["decodedBytes"]
                low, high = (size, size) if index["algorithm"]["kind"] == "fixed" else (size // 4, size * 4)
                if decoded < 1 or decoded > high or (n < len(blocks) - 1 and decoded < low):
                    raise ValueError("invalid byte chunk boundary length")
        else:
            image = entry.get("image")
            if not isinstance(image, dict):
                raise ValueError("missing tile canvas")
            width = base.integer(image.get("width"), 1, 16384, "tile canvas width")
            height = base.integer(image.get("height"), 1, 16384, "tile canvas height")
            if width * height > base.MAX_PIXELS:
                raise ValueError("oversized tile canvas")
            size = index["algorithm"]["size"]
            expected_count = ((width + size - 1) // size) * ((height + size - 1) // size)
            if len(blocks) != expected_count:
                raise ValueError("tile grid does not cover canvas")
            bpp = 2 if image.get("plane") == "gray16le" else 4
            n = 0
            for top in range(0, height, size):
                for left in range(0, width, size):
                    if chunks[blocks[n]]["decodedBytes"] != min(size, width - left) * min(size, height - top) * bpp:
                        raise ValueError("invalid edge tile length")
                    n += 1
        base.integer(entry.get("losslessExportBytes"), 0, base.MAX_FILE + base.MAX_METADATA, "standalone export bytes")
        if fidelity == "byte-exact" and (entry["losslessExportBytes"] != entry.get("sourceBytes") or entry.get("losslessReceipt") is not None):
            raise ValueError("invalid byte baseline")
        if fidelity == "pixel-exact":
            receipt = entry.get("losslessReceipt")
            if not isinstance(receipt, dict) or receipt.get("id") != entry.get("id") or receipt.get("representationSHA256") != entry.get("representationSHA256"):
                raise ValueError("invalid pixel baseline receipt")
        # Reuse P12's strict source, metadata and canonical-plane validation.
        # No P12 base relationships enter this format.
        proxies.append({**entry, "kind": "full", "blob": "0" * 64, "blobBytes": length, "encoding": "raw"})
    if wanted != set(chunks):
        raise ValueError("unreferenced chunks in gallery index")
    base.validate_manifest(base.make_manifest(index["gallery"], index.get("representative"), fidelity, proxies))
    return {entry["id"]: entry for entry in members}


def decode_block(record, payload, digest, image=None):
    if len(payload) != record["blobBytes"] or chunk_address(record, payload) != digest:
        raise ValueError("chunk checksum/length mismatch")
    if record["encoding"] == "zlib":
        result = base.decompress(payload, record["decodedBytes"])
    elif record["encoding"] == "png":
        result, descriptor, _ = base.decode_pixels(payload)
        if descriptor != image:
            raise ValueError("independent PNG canvas mismatch")
    else:
        result = payload
    if len(result) != record["decodedBytes"] or base.sha(result) != record["decodedSHA256"]:
        raise ValueError("decoded chunk checksum/length mismatch")
    return result


def reconstruct(index, entry, read_blob):
    started = time.monotonic()
    if index["algorithm"]["kind"] != "tile":
        pieces = []
        for digest in entry["blocks"]:
            deadline(started)
            pieces.append(decode_block(index["chunks"][digest], read_blob(digest), digest, entry.get("image")))
        result = b"".join(pieces)
    else:
        import numpy as np
        image, size = entry["image"], index["algorithm"]["size"]
        bpp = 2 if image["plane"] == "gray16le" else 4
        plane = np.empty((image["height"], image["width"], bpp), dtype="u1")
        n = 0
        for top in range(0, image["height"], size):
            for left in range(0, image["width"], size):
                deadline(started)
                digest = entry["blocks"][n]
                data = decode_block(index["chunks"][digest], read_blob(digest), digest)
                height, width = min(size, image["height"] - top), min(size, image["width"] - left)
                plane[top:top + height, left:left + width] = np.frombuffer(data, dtype="u1").reshape(height, width, bpp)
                n += 1
        result = plane.tobytes()
    if len(result) != entry["representationBytes"] or base.sha(result) != entry["representationSHA256"]:
        raise ValueError("reconstructed member checksum/length mismatch")
    deadline(started)
    return result


def portable(entry, data, fidelity):
    if fidelity == "byte-exact":
        return data
    exported = base.encode_png(data, entry["image"], entry["metadata"])
    pixels, descriptor, metadata = base.decode_pixels(exported)
    if pixels != data or descriptor != entry["image"] or metadata["chunks"] != [c for c in entry["metadata"]["chunks"] if c["embed"]]:
        raise ValueError("portable PNG changed pixels or rendering metadata")
    return exported


def account(index, members, blobs):
    encoded = items(index, members, blobs)
    total = 22 + sum(entry_cost(name, data) for name, data in encoded.items())
    chunk_bytes = sum(len(payload) for payload in blobs.values())
    chunk_indexes = sum(76 + 2 * len("chunks/" + digest) for digest in blobs)
    manifests = sum(len(data) for name, data in encoded.items() if not name.startswith("chunks/"))
    zip_indexes = total - chunk_bytes - manifests
    owners = {digest: [] for digest in blobs}
    for entry in members:
        for digest in set(entry["blocks"]):
            owners[digest].append(entry["id"])
    shared = 22 + sum(entry_cost(name, encoded[name]) for name in ("gallery.json", "gallery.sha256"))
    allocations = []
    for entry in members:
        name = "members/" + base.sha(base.json_bytes(entry)) + ".json"
        allocation = entry_cost(name, encoded[name]) + shared / len(members)
        allocation += sum(entry_cost("chunks/" + digest, blobs[digest]) / len(owners[digest]) for digest in set(entry["blocks"]))
        allocations.append({"id": entry["id"], "allocatedBytes": allocation, "chunksNeeded": len(set(entry["blocks"])),
                            "encodedChunkBytesNeeded": sum(len(blobs[d]) for d in set(entry["blocks"]))})
    original = sum(entry["sourceBytes"] for entry in members)
    lossless = sum(entry["losslessExportBytes"] for entry in members)
    receipt_bytes = 0
    if index["fidelity"] == "pixel-exact":
        receipt_bytes = len(base.json_bytes({"version": VERSION, "fidelity": "pixel-exact", "complete": True, "members": [entry["losslessReceipt"] for entry in members]}))
    initialization = 22 + sum(46 + len(name.encode()) for name in encoded)
    initialization += sum(len(data) + 30 + len(name.encode()) for name, data in encoded.items() if not name.startswith("chunks/"))
    return {"archiveBytes": total, "uniqueChunkBytes": chunk_bytes, "uniqueChunks": len(blobs),
            "logicalRepresentationBytes": sum(entry["representationBytes"] for entry in members),
            "uniqueDecodedChunkBytes": sum(record["decodedBytes"] for record in index["chunks"].values()),
            "randomAccessInitializationBytes": initialization,
            "chunkReferences": sum(len(entry["blocks"]) for entry in members),
            "manifestBytes": manifests, "zipIndexBytes": zip_indexes, "chunkZipIndexBytes": chunk_indexes,
            "paddingBytes": 0, "originalFileBytes": original, "independentExportBytes": lossless,
            "independentExportReceiptBytes": receipt_bytes, "independentExportTotalBytes": lossless + receipt_bytes,
            "savingsVsOriginalBytes": original - total, "sourcesRetained": True,
            "physicalBytesWithRetainedSources": original + total, "cacheBytes": 0, "additionalBackupBytes": 0,
            "allocationPolicy": "allocation only: exclusive member manifests, equal shares of common index and of each chunk among referencing members",
            "allocations": allocations}


def candidate(gallery, prepared, representative, fidelity, algorithm):
    chunks, blobs, members = {}, {}, []
    started = time.monotonic()
    for member in prepared:
        deadline(started)
        members.append(add_member(member, algorithm, fidelity, chunks, blobs))
        if sum(len(entry["blocks"]) for entry in members) > MAX_REFERENCES:
            raise ValueError("gallery reference budget exceeded; use larger blocks or split the gallery")
    index = make_index(gallery, representative, fidelity, algorithm, members, chunks)
    validate_index(index, members)
    total = archive_size(index, members, blobs)
    if total > base.MAX_PACK:
        raise ValueError("archive exceeds 384 MiB; split the gallery")
    timings = []
    for source, entry in zip(prepared, members):
        before = time.monotonic()
        result = reconstruct(index, entry, blobs.__getitem__)
        if result != (source.original if fidelity == "byte-exact" else source.pixels):
            raise ValueError("candidate failed exact reconstruction")
        portable(entry, result, fidelity)
        timings.append({"id": entry["id"], "seconds": time.monotonic() - before, "uniqueChunksRead": len(set(entry["blocks"]))})
    return index, members, blobs, {"algorithm": algorithm, **account(index, members, blobs),
                                  "verified": True, "reconstruction": timings, "planSeconds": time.monotonic() - started}


def plan_gallery(group, fidelity="byte-exact", min_bytes=0, min_percent=5.0):
    import numpy as np
    thresholds(min_bytes, min_percent)
    algorithms(fidelity)
    identity = group.get("id")
    if not isinstance(identity, str) or not 1 <= len(identity) <= 100:
        raise ValueError("invalid gallery ID")
    started = time.monotonic()
    prepared, representative = base.prepare(group, fidelity)
    independent = candidate(identity, prepared, representative, fidelity, {"kind": "independent"})
    chosen = independent
    measurements = [independent[3]]
    baseline = independent[3]["archiveBytes"]
    for algorithm in algorithms(fidelity):
        deadline(started)
        try:
            current = candidate(identity, prepared, representative, fidelity, algorithm)
            measurement = current[3]
            saving = baseline - measurement["archiveBytes"]
            measurement["savingsVsIndependentArchiveBytes"] = saving
            # Include the whole gallery, metadata and ZIP overhead in all gates.
            source_saving = measurement["savingsVsOriginalBytes"]
            eligible = (saving > 0 and saving >= min_bytes and saving * 100 >= baseline * min_percent
                        and source_saving > 0 and source_saving >= min_bytes
                        and source_saving * 100 >= measurement["originalFileBytes"] * min_percent
                        and measurement["archiveBytes"] < measurement["independentExportTotalBytes"])
            measurement["eligible"] = eligible
            if eligible and measurement["archiveBytes"] < chosen[3]["archiveBytes"]:
                chosen = current
            measurements.append(measurement)
        except (ValueError, zlib.error) as error:
            measurements.append({"algorithm": algorithm, "verified": False, "error": str(error), "eligible": False})
    report = {"gallery": identity, "fidelity": fidelity, "layout": chosen[0]["algorithm"],
              **chosen[3], "sharingSelected": chosen is not independent, "minimumBytes": min_bytes, "minimumPercent": min_percent,
              "independentArchiveBytes": baseline, "savingsVsIndependentArchiveBytes": baseline - chosen[3]["archiveBytes"],
              "candidates": measurements, "planSeconds": time.monotonic() - started,
              "runtime": {"python": sys.version.split()[0], "Pillow": chosen[0]["decoder"]["version"], "NumPy": np.__version__, "zlib": zlib.ZLIB_RUNTIME_VERSION},
              "fallback": None if chosen is not independent else "Keep independent sources; no chunk/tile candidate met all complete-size savings gates."}
    return chosen[0], chosen[1], chosen[2], report


class GalleryReader:
    def __init__(self, path):
        self.archive = None
        try:
            self.source = os.fdopen(os.open(path, os.O_RDONLY | getattr(os, "O_NONBLOCK", 0)), "rb")
            import stat
            size = os.fstat(self.source.fileno()).st_size
            if not stat.S_ISREG(os.fstat(self.source.fileno()).st_mode) or size > base.MAX_PACK or size < 22:
                raise ValueError("archive is incomplete, non-regular or exceeds 384 MiB")
            self.source.seek(-22, os.SEEK_END)
            signature, disk, directory_disk, disk_count, count, directory_size, offset, comment = struct.unpack("<4s4H2IH", self.source.read(22))
            if signature != b"PK\x05\x06" or disk or directory_disk or comment or disk_count != count or not 3 <= count <= MAX_CHUNKS + base.MAX_MEMBERS + 2 or directory_size > MAX_DIRECTORY or offset + directory_size != size - 22:
                raise ValueError("invalid or oversized archive directory")
            self.source.seek(0)
            self.archive = zipfile.ZipFile(self.source)
            infos = self.archive.infolist()
            if len(infos) != count or len({info.filename for info in infos}) != count:
                raise ValueError("duplicate archive entries")
            self.actual = {info.filename for info in infos}
            for info in infos:
                if info.compress_type != zipfile.ZIP_STORED or info.flag_bits or info.extra or info.comment or info.is_dir():
                    raise ValueError("archive requires plain stored files")
                if info.filename not in ("gallery.json", "gallery.sha256") and not re.fullmatch(r"(?:members/[0-9a-f]{64}\.json|chunks/[0-9a-f]{64})", info.filename):
                    raise ValueError("unsafe archive path")
                limit = MAX_INDEX if info.filename == "gallery.json" else (65 if info.filename == "gallery.sha256" else (MAX_MEMBER_MANIFEST if info.filename.startswith("members/") else base.MAX_FILE))
                if info.file_size > limit or info.compress_size != info.file_size:
                    raise ValueError("oversized archive entry")
            raw = self.archive.read("gallery.json")
            if self.archive.read("gallery.sha256") != (base.sha(raw) + "\n").encode():
                raise ValueError("gallery checksum mismatch")
            self.index = json.loads(raw)
            if not isinstance(self.index, dict) or not isinstance(self.index.get("members"), list) or not 1 <= len(self.index["members"]) <= base.MAX_MEMBERS:
                raise ValueError("invalid gallery index")
            self.members = []
            for ref in self.index["members"]:
                if not isinstance(ref, dict) or not isinstance(ref.get("manifest"), str) or not base.HASH.fullmatch(ref["manifest"]):
                    raise ValueError("invalid member manifest address")
                member_raw = self.archive.read("members/" + ref["manifest"] + ".json")
                if len(member_raw) != ref.get("manifestBytes") or base.sha(member_raw) != ref["manifest"]:
                    raise ValueError("member manifest checksum mismatch")
                self.members.append(json.loads(member_raw))
            self.entries = validate_index(self.index, self.members)
            wanted = {"gallery.json", "gallery.sha256"} | {"members/" + ref["manifest"] + ".json" for ref in self.index["members"]} | {"chunks/" + digest for digest in self.index["chunks"]}
            if self.actual - wanted:
                raise ValueError("archive contains unreferenced entries")
            # Missing chunks are allowed here so diagnostics can isolate dependants.
            self.chunk_reads = []
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

    def read_chunk(self, digest):
        self.chunk_reads.append(digest)
        return self.archive.read("chunks/" + digest)

    def reconstruct(self, identity):
        if identity not in self.entries:
            raise ValueError("unknown member ID")
        # Cache only this member's compressed chunks, bounded by its 64 MiB plane.
        cache = {}
        def read(digest):
            if digest not in cache:
                cache[digest] = self.read_chunk(digest)
            return cache[digest]
        return reconstruct(self.index, self.entries[identity], read)

    def verify(self, identity=None):
        started = time.monotonic()
        if identity is not None and identity not in self.entries:
            raise ValueError("unknown member ID")
        rows = []
        for member in ([identity] if identity is not None else self.entries):
            deadline(started)
            before = len(self.chunk_reads)
            try:
                data = self.reconstruct(member)
                portable(self.entries[member], data, self.index["fidelity"])
                rows.append({"id": member, "verified": True, "bytes": len(data), "chunksRead": len(self.chunk_reads) - before})
            except (ValueError, OSError, KeyError, zipfile.BadZipFile, zlib.error) as error:
                rows.append({"id": member, "verified": False, "error": str(error), "chunksRead": len(self.chunk_reads) - before})
        return {"gallery": self.index["gallery"], "fidelity": self.index["fidelity"], "verified": all(row["verified"] for row in rows),
                "members": rows, "verifySeconds": time.monotonic() - started}

    def diagnose(self):
        started = time.monotonic()
        owners = {digest: [entry["id"] for entry in self.members if digest in entry["blocks"]] for digest in self.index["chunks"]}
        damaged = []
        for digest, record in self.index["chunks"].items():
            deadline(started)
            try:
                image = self.entries[owners[digest][0]].get("image")
                decode_block(record, self.read_chunk(digest), digest, image)
            except (ValueError, OSError, KeyError, zipfile.BadZipFile, zlib.error) as error:
                damaged.append({"chunk": digest, "error": str(error), "dependants": owners[digest]})
        checked = self.verify()
        return {**checked, "damagedChunks": damaged, "repair": "Supply explicit hash-matching sources; metadata cannot regenerate lost bytes." if damaged else "No damaged chunks detected."}


def write_pack(index, members, blobs, output, checkpoint=lambda _phase, _path: None):
    validate_index(index, members)
    if set(blobs) != set(index["chunks"]):
        raise ValueError("all and only referenced chunks must be supplied")
    expected = archive_size(index, members, blobs)
    if expected > base.MAX_PACK:
        raise ValueError("archive exceeds 384 MiB")
    output = Path(output)
    if output.exists() or output.is_symlink():
        raise ValueError("output already exists; choose a new archive path")
    fd, filename = tempfile.mkstemp(prefix=".p13-", suffix=".staging", dir=output.parent)
    staging = Path(filename)
    os.close(fd)
    try:
        checkpoint("created", staging)
        for_manifest = items(index, members, blobs)
        with zipfile.ZipFile(staging, "w", compression=zipfile.ZIP_STORED, allowZip64=False) as archive:
            for name, data in for_manifest.items():
                info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
                info.external_attr = 0o100600 << 16
                archive.writestr(info, data)
                checkpoint("entry", staging)
        with staging.open("rb") as source:
            os.fsync(source.fileno())
        base.fsync_parent(staging)
        checkpoint("durable", staging)
        if staging.stat().st_size != expected:
            raise ValueError("archive accounting differs from physical size")
        with GalleryReader(staging) as reader:
            result = reader.verify()
        if not result["verified"]:
            raise ValueError("staged archive verification failed: " + json.dumps(result))
        checkpoint("verified", staging)
        base.publish(staging, output)
        checkpoint("published", staging)
        return {**result, "archiveBytes": expected}
    finally:
        staging.unlink(missing_ok=True)


def recover(staging, output):
    if Path(staging).is_symlink():
        raise ValueError("recover a regular staging file, not a symlink")
    with GalleryReader(staging) as reader:
        result = reader.verify()
    if not result["verified"]:
        raise ValueError("staging is incomplete or corrupt; rebuild from retained sources")
    base.publish(staging, output)
    return result


def extract(path, output, identity=None, checkpoint=lambda _phase, _path: None):
    with GalleryReader(path) as reader:
        if identity is not None and identity not in reader.entries:
            raise ValueError("unknown member ID")
        output = Path(output)
        output.mkdir(mode=0o700)
        try:
            exports = []
            for entry in reader.members:
                if identity is not None and entry["id"] != identity:
                    continue
                data = portable(entry, reader.reconstruct(entry["id"]), reader.index["fidelity"])
                destination = output / base.export_name(entry, reader.index["fidelity"])
                with destination.open("xb") as file:
                    file.write(data)
                    file.flush()
                    os.fsync(file.fileno())
                exports.append(base.export_record(entry, data, reader.index["fidelity"]))
                checkpoint("member", output)
            receipt = {"version": VERSION, "gallery": reader.index["gallery"], "fidelity": reader.index["fidelity"], "complete": True, "members": exports}
            with (output / "extraction.json").open("xb") as file:
                file.write(base.json_bytes(receipt))
                file.flush()
                os.fsync(file.fileno())
            base.fsync_parent(output / "extraction.json")
            base.fsync_parent(output)
            checkpoint("complete", output)
            return receipt
        except BaseException:
            shutil.rmtree(output)
            raise


def revise(path, output, additions=None, removals=None, min_bytes=0, min_percent=5.0, checkpoint=lambda _phase, _path: None):
    thresholds(min_bytes, min_percent)
    retained_revision_bytes = Path(path).stat().st_size
    with GalleryReader(path) as reader:
        if not reader.verify()["verified"]:
            raise ValueError("repair the archive before revising it")
        removing = set(removals or [])
        if removing - set(reader.entries):
            raise ValueError("unknown member ID to remove")
        members = [dict(entry) for entry in reader.members if entry["id"] not in removing]
        chunks = dict(reader.index["chunks"])
        blobs = {digest: reader.read_chunk(digest) for digest in chunks}
        if additions:
            prepared, _ = base.prepare({"members": additions}, reader.index["fidelity"])
            known = {entry["id"] for entry in reader.members}
            for item in prepared:
                if item.entry["id"] in known:
                    raise ValueError("append IDs must be new; removal does not permit identity reuse")
                known.add(item.entry["id"])
                item.entry["order"] = len(members)
                members.append(add_member(item, reader.index["algorithm"], reader.index["fidelity"], chunks, blobs))
        if not 1 <= len(members) <= base.MAX_MEMBERS:
            raise ValueError("revision must retain 1–16 members; split a larger gallery")
        for order, entry in enumerate(members):
            entry["order"] = order
            if entry.get("losslessReceipt"):
                entry["losslessReceipt"] = {**entry["losslessReceipt"], "order": order, "name": base.export_name(entry, reader.index["fidelity"])}
        live = {digest for entry in members for digest in entry["blocks"]}
        chunks = {digest: chunks[digest] for digest in sorted(live)}
        blobs = {digest: blobs[digest] for digest in sorted(live)}
        representative = reader.index["representative"]
        if representative in removing:
            representative = members[0]["id"]
        parent = base.sha(base.json_bytes(reader.index))
        index = make_index(reader.index["gallery"], representative, reader.index["fidelity"], reader.index["algorithm"], members, chunks, reader.index["generation"] + 1, parent)
    result = account(index, members, blobs)
    savings = result["savingsVsOriginalBytes"]
    eligible = savings > 0 and savings >= min_bytes and savings * 100 >= result["originalFileBytes"] * min_percent and result["archiveBytes"] < result["independentExportTotalBytes"]
    result.update({"layout": index["algorithm"], "incremental": True, "reencodedExistingChunks": 0, "minimumBytes": min_bytes, "minimumPercent": min_percent,
                   "netSourceSavingsGatePassed": eligible, "fallback": None if eligible else "Keep independent sources; this revision does not pass the complete-size source savings gate.",
                   "oldRevisionRetained": True, "retainedRevisionBytes": retained_revision_bytes,
                   "physicalBytesWithRetainedSourcesAndRevisions": result["physicalBytesWithRetainedSources"] + retained_revision_bytes,
                   "parentSHA256": parent})
    result.update(write_pack(index, members, blobs, output, checkpoint))
    return result


def repair(path, sources, output):
    """Repair only from explicit originals matching the recorded SHA-256/MD5."""
    with GalleryReader(path) as reader:
        diagnosis = reader.diagnose()
        needed = {row["chunk"] for row in diagnosis["damagedChunks"]}
        replacements = {}
        for source in sources:
            identity = source.get("id")
            if identity not in reader.entries:
                raise ValueError("unknown repair member ID")
            original = base.read_regular(source["path"], base.MAX_FILE)
            entry = reader.entries[identity]
            if len(original) != entry["sourceBytes"] or base.sha(original) != entry["sourceSHA256"] or hashlib.md5(original, usedforsecurity=False).hexdigest() != entry["sourceMD5"]:
                raise ValueError("repair source does not match recorded original")
            prepared, _ = base.prepare({"members": [{"id": identity, "path": source["path"], "name": entry["name"]}]}, reader.index["fidelity"])
            chunks, blobs = {}, {}
            regenerated = add_member(prepared[0], reader.index["algorithm"], reader.index["fidelity"], chunks, blobs)
            if regenerated["blocks"] != entry["blocks"]:
                raise ValueError("repair runtime cannot reproduce recorded chunks; use original runtime")
            for digest in needed & set(blobs):
                if chunks[digest] != reader.index["chunks"][digest]:
                    raise ValueError("repair chunk descriptor mismatch")
                replacements[digest] = blobs[digest]
        if needed - set(replacements):
            raise ValueError("matching sources did not cover every damaged chunk")
        blobs = {digest: replacements[digest] if digest in replacements else reader.read_chunk(digest) for digest in reader.index["chunks"]}
        index, members = reader.index, reader.members
    return {**write_pack(index, members, blobs, output), "repairedChunks": sorted(replacements), "damagedInputRetained": True}


def read_galleries(path):
    path = Path(path)
    value = json.loads(base.read_regular(path, base.MAX_MANIFEST))
    groups = value.get("galleries") if isinstance(value, dict) else None
    if not isinstance(groups, list) or not 1 <= len(groups) <= base.MAX_GROUPS:
        raise ValueError("input must contain 1–32 galleries")
    identities = set()
    for group in groups:
        if not isinstance(group, dict) or not isinstance(group.get("id"), str) or not 1 <= len(group["id"]) <= 100 or group["id"] in identities:
            raise ValueError("gallery IDs must be unique strings of 1–100 characters")
        identities.add(group["id"])
        members = group.get("members")
        if not isinstance(members, list) or not 1 <= len(members) <= base.MAX_MEMBERS:
            raise ValueError("split galleries into groups of 1–16 members")
        for member in members:
            if not isinstance(member, dict) or not isinstance(member.get("path"), str):
                raise ValueError("member needs an explicit source path")
            source = Path(member["path"])
            if not source.is_absolute():
                member["path"] = str(path.parent / source)
    return groups


def benchmark(groups, min_bytes=0, min_percent=5.0):
    thresholds(min_bytes, min_percent)
    started, results = time.monotonic(), []
    for group in groups:
        for fidelity in ("byte-exact", "pixel-exact"):
            deadline(started)
            try:
                index, members, blobs, report = plan_gallery(group, fidelity, min_bytes, min_percent)
                with tempfile.TemporaryDirectory(prefix="p13-benchmark-") as root:
                    checked = write_pack(index, members, blobs, Path(root) / "gallery.p13")
                report.update(checked)
                results.append(report)
            except (ValueError, OSError, KeyError, TypeError, zipfile.BadZipFile, zlib.error) as error:
                results.append({"gallery": group["id"], "fidelity": fidelity, "verified": False, "error": str(error)})
    return {"version": VERSION, "experimental": True, "sourcesRetained": True, "results": results}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    for command in ("estimate", "benchmark", "pack"):
        current = sub.add_parser(command)
        current.add_argument("--galleries", required=True)
        current.add_argument("--output", required=command != "estimate")
        current.add_argument("--min-bytes", type=int, default=0)
        current.add_argument("--min-percent", type=float, default=5.0)
        if command != "benchmark":
            current.add_argument("--gallery", required=True)
            current.add_argument("--fidelity", choices=("byte-exact", "pixel-exact"), default="byte-exact")
    for command in ("verify", "diagnose", "extract", "append", "remove", "repair"):
        current = sub.add_parser(command)
        current.add_argument("archive")
        if command in ("verify", "extract"):
            current.add_argument("--entry")
        if command in ("extract", "append", "remove", "repair"):
            current.add_argument("--output", required=True)
        if command in ("append", "repair"):
            current.add_argument("--galleries", required=True)
            current.add_argument("--gallery", required=True)
        if command == "remove":
            current.add_argument("--entry", action="append", required=True)
        if command in ("append", "remove"):
            current.add_argument("--min-bytes", type=int, default=0)
            current.add_argument("--min-percent", type=float, default=5.0)
    current = sub.add_parser("recover")
    current.add_argument("staging")
    current.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        base.require_dependencies()
        if args.command in ("estimate", "benchmark", "pack", "append", "repair"):
            groups = read_galleries(args.galleries)
            if args.command != "benchmark":
                selected = next((group for group in groups if group["id"] == args.gallery), None)
                if selected is None:
                    raise ValueError("unknown gallery ID")
        if args.command == "benchmark":
            result = benchmark(groups, args.min_bytes, args.min_percent)
            base.save_json(result, args.output)
            result["verified"] = all(row.get("verified", False) for row in result["results"])
        elif args.command in ("estimate", "pack"):
            index, members, blobs, result = plan_gallery(selected, args.fidelity, args.min_bytes, args.min_percent)
            if args.command == "pack":
                result.update(write_pack(index, members, blobs, args.output))
            elif args.output:
                base.save_json(result, args.output)
        elif args.command in ("verify", "diagnose"):
            with GalleryReader(args.archive) as reader:
                result = reader.verify(args.entry) if args.command == "verify" else reader.diagnose()
        elif args.command == "extract":
            result = extract(args.archive, args.output, args.entry)
        elif args.command == "append":
            result = revise(args.archive, args.output, additions=selected["members"], min_bytes=args.min_bytes, min_percent=args.min_percent)
        elif args.command == "remove":
            result = revise(args.archive, args.output, removals=args.entry, min_bytes=args.min_bytes, min_percent=args.min_percent)
        elif args.command == "repair":
            result = repair(args.archive, selected["members"], args.output)
        else:
            result = recover(args.staging, args.output)
        print(json.dumps(result, sort_keys=True, allow_nan=False))
        return 0 if result.get("verified", True) else 1
    except KeyboardInterrupt:
        print("Cancelled; sources and old revisions remain recoverable.", file=sys.stderr)
        return 130
    except (ValueError, OSError, KeyError, TypeError, zipfile.BadZipFile, zlib.error) as error:
        print(str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
