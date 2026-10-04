"""P13 exact fidelity, random access, incremental ownership and crash recovery."""
import copy
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
import zipfile
import zlib

import numpy as np
from PIL import Image, PngImagePlugin

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import gallery_block_storage as storage
import image_delta_storage as base


class GalleryStorageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.rng = np.random.default_rng(1313)
        self.pixels = self.rng.integers(0, 256, (128, 192, 4), dtype=np.uint8)

    def tearDown(self):
        self.temp.cleanup()

    def image(self, pixels, name, **options):
        path = self.root / name
        Image.fromarray(pixels).save(path, **options)
        return path

    def gallery(self, paths):
        return {"id": "gallery:12", "representative": f"image:{len(paths)}",
                "members": [{"id": f"image:{n + 1}", "path": str(path)} for n, path in enumerate(paths)]}

    def edits(self):
        paths = [self.image(self.pixels, "original.png")]
        for index in range(3):
            changed = self.pixels.copy()
            changed[7 + index:12 + index, 13:20, 0] ^= 1 + index
            paths.append(self.image(changed, f"edit{index}.png"))
        return self.gallery(paths)

    def pack(self, group, fidelity="byte-exact", name="gallery.p13", **thresholds):
        index, members, blobs, report = storage.plan_gallery(group, fidelity, **thresholds)
        output = self.root / name
        verified = storage.write_pack(index, members, blobs, output)
        self.assertTrue(verified["verified"])
        self.assertEqual(output.stat().st_size, report["archiveBytes"])
        self.assertEqual(report["uniqueChunkBytes"] + report["manifestBytes"] + report["zipIndexBytes"], output.stat().st_size)
        self.assertAlmostEqual(sum(row["allocatedBytes"] for row in report["allocations"]), output.stat().st_size, places=6)
        return output, index, members, blobs, report

    def forced(self, group, fidelity, algorithm, name="forced.p13"):
        prepared, representative = base.prepare(group, fidelity)
        index, members, blobs, report = storage.candidate(group["id"], prepared, representative, fidelity, algorithm)
        output = self.root / name
        storage.write_pack(index, members, blobs, output)
        return output, index, members, blobs, report

    def mutate(self, archive, change, name="mutated.p13"):
        with zipfile.ZipFile(archive) as source:
            values = {key: source.read(key) for key in source.namelist()}
        change(values)
        output = self.root / name
        with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_STORED) as target:
            for key, value in values.items():
                target.writestr(key, value)
        return output

    @staticmethod
    def change_index(values, transform):
        index = json.loads(values["gallery.json"])
        members = [json.loads(values["members/" + ref["manifest"] + ".json"]) for ref in index["members"]]
        transform(index, members)
        for key in list(values):
            if key.startswith("members/"):
                del values[key]
        for ref, member in zip(index["members"], members):
            raw = base.json_bytes(member)
            ref["manifest"], ref["manifestBytes"] = base.sha(raw), len(raw)
            values["members/" + ref["manifest"] + ".json"] = raw
        raw = base.json_bytes(index)
        values["gallery.json"], values["gallery.sha256"] = raw, (base.sha(raw) + "\n").encode()

    def test_byte_default_preserves_container_names_ids_order_and_offline_restore(self):
        png = self.image(self.pixels, "page.png")
        jpeg = self.image(self.pixels[:, :, :3], "other.jpg", quality=93)
        group = self.gallery([jpeg, png])
        originals = [path.read_bytes() for path in (jpeg, png)]
        archive, index, members, _, report = self.pack(group)
        self.assertEqual(index["fidelity"], "byte-exact")
        self.assertEqual([entry["id"] for entry in members], ["image:1", "image:2"])
        self.assertTrue(report["sourcesRetained"])
        self.assertEqual(len(index["members"]), 2)
        jpeg.unlink()
        png.unlink()
        empty = self.root / "empty-environment"
        empty.mkdir()
        portable = empty / "backup.p13"
        portable.write_bytes(archive.read_bytes())
        receipt = storage.extract(portable, empty / "restored")
        for n, entry in enumerate(receipt["members"]):
            self.assertEqual((empty / "restored" / entry["name"]).read_bytes(), originals[n])
            self.assertEqual(entry["sourceSHA256"], entry["exportSHA256"])
            self.assertEqual(entry["sourceMD5"], entry["exportMD5"])
        self.assertEqual([e["sourceName"] for e in receipt["members"]], ["other.jpg", "page.png"])

    def test_content_defined_chunks_survive_byte_insertion_and_fixed_chunks_do_not(self):
        data = self.rng.bytes(350000)
        first, second = self.root / "one.bin", self.root / "two.bin"
        first.write_bytes(data)
        second.write_bytes(b"inserted-prefix" + data)
        archive, index, members, blobs, report = self.pack(self.gallery([first, second]))
        self.assertTrue(report["sharingSelected"])
        self.assertEqual(index["algorithm"]["kind"], "cdc")
        cdc = [row for row in report["candidates"] if row["algorithm"]["kind"] == "cdc"]
        fixed = [row for row in report["candidates"] if row["algorithm"]["kind"] == "fixed"]
        self.assertLess(min(r["archiveBytes"] for r in cdc), min(r["archiveBytes"] for r in fixed))
        self.assertLess(report["archiveBytes"], report["originalFileBytes"])
        self.assertGreater(len(set(members[0]["blocks"]) & set(members[1]["blocks"])), 0)
        with storage.GalleryReader(archive) as reader:
            self.assertEqual(reader.reconstruct("image:2"), second.read_bytes())
        self.assertEqual(len(blobs), len(index["chunks"]))

    def test_cdc_version_has_fixed_portable_boundary_oracle(self):
        data = bytes(range(256)) * 500 + np.random.default_rng(5).bytes(100000)
        chunks = list(storage.byte_chunks(data, {"kind": "cdc", "size": 8192}))
        self.assertEqual(b"".join(chunks), data)
        # Independent arithmetic oracle with the published gear derivation.
        import hashlib
        gears = [int.from_bytes(hashlib.sha256(("p13-gear-v1:" + str(n)).encode()).digest()[:8], "little") for n in range(256)]
        lengths, current, rolling = [], 0, 0
        for byte in data:
            current += 1
            rolling = ((rolling * 2) + gears[byte]) % (2 ** 64)
            if current == 32768 or (current >= 2048 and rolling % 8192 == 0):
                lengths.append(current)
                current, rolling = 0, 0
        if current:
            lengths.append(current)
        self.assertEqual([len(piece) for piece in chunks], lengths)

    def test_exact_tiles_local_edits_and_transparent_rgb(self):
        group = self.edits()
        archive, index, members, blobs, report = self.pack(group, "pixel-exact")
        self.assertTrue(report["sharingSelected"])
        self.assertEqual(index["algorithm"]["kind"], "tile")
        self.assertLess(len(blobs), sum(len(entry["blocks"]) for entry in members))
        with storage.GalleryReader(archive) as reader:
            for member in group["members"]:
                self.assertEqual(reader.reconstruct(member["id"]), base.decode_pixels(Path(member["path"]).read_bytes())[0])
        hidden = self.pixels.copy()
        hidden[:, :, 3] = 0
        a = self.image(hidden, "hidden-a.png")
        hidden[50, 60, 2] ^= 1
        b = self.image(hidden, "hidden-b.png")
        archive, _, _, _, _ = self.pack(self.gallery([a, b]), "pixel-exact", "hidden.p13")
        with storage.GalleryReader(archive) as reader:
            self.assertEqual(reader.reconstruct("image:2"), hidden.tobytes())

    def test_mixed_dimensions_edge_tiles_and_gray16(self):
        gray = self.rng.integers(0, 65536, (73, 91), dtype=np.uint16)
        a = self.image(self.pixels[:71, :93], "rgba.png")
        b = self.image(gray, "gray16.png")
        c = self.image(self.pixels, "large.png")
        group = self.gallery([a, b, c])
        archive, _, members, _, _ = self.forced(group, "pixel-exact", {"kind": "tile", "size": 64})
        self.assertEqual(members[1]["image"]["bits"], 16)
        self.assertEqual(members[1]["image"]["alpha"], "none")
        with storage.GalleryReader(archive) as reader:
            self.assertEqual(reader.reconstruct("image:2"), gray.astype("<u2").tobytes())
        receipt = storage.extract(archive, self.root / "mixed-export")
        for row in receipt["members"]:
            pixels, image, _ = base.decode_pixels((self.root / "mixed-export" / row["name"]).read_bytes())
            expected = next(entry for entry in members if entry["id"] == row["id"])
            self.assertEqual(base.sha(pixels), expected["representationSHA256"])
            self.assertEqual(image, expected["image"])

    def test_profile_exif_text_sidecar_and_source_export_hashes(self):
        info = PngImagePlugin.PngInfo()
        info.add_text("Description", "P13 exact metadata")
        info.add(b"vpAg", b"opaque ancillary")
        exif = Image.Exif()
        exif[274] = 6
        a = self.image(self.pixels, "metadata.png", pnginfo=info, icc_profile=b"profile acceptance bytes", exif=exif)
        b = self.image(self.pixels, "plain.png")
        archive, _, members, _, report = self.pack(self.gallery([a, b]), "pixel-exact")
        expected = base.decode_pixels(a.read_bytes())[2]
        self.assertEqual(members[0]["metadata"], expected)
        receipt = storage.extract(archive, self.root / "metadata-export")
        row = receipt["members"][0]
        self.assertEqual(row["metadata"], expected)
        self.assertNotEqual(row["sourceSHA256"], row["exportSHA256"])
        self.assertNotEqual(row["sourceMD5"], row["exportMD5"])
        exported = (self.root / "metadata-export" / row["name"]).read_bytes()
        self.assertEqual(base.decode_pixels(exported)[2]["chunks"], [c for c in expected["chunks"] if c["embed"]])
        standalone = self.root / "baseline"
        standalone.mkdir()
        for entry in members:
            original = Path(self.gallery([a, b])["members"][entry["order"]]["path"]).read_bytes()
            pixels, image, metadata = base.decode_pixels(original)
            (standalone / base.export_name(entry, "pixel-exact")).write_bytes(base.encode_png(pixels, image, metadata))
        baseline_receipt = {"version": 1, "fidelity": "pixel-exact", "complete": True, "members": [entry["losslessReceipt"] for entry in members]}
        (standalone / "extraction.json").write_bytes(base.json_bytes(baseline_receipt))
        self.assertEqual(sum(p.stat().st_size for p in standalone.iterdir()), report["independentExportTotalBytes"])

    def test_identical_whole_files_and_chunks_stored_once(self):
        a = self.image(self.pixels, "a.png")
        b = self.root / "b.png"
        b.write_bytes(a.read_bytes())
        group = self.gallery([a, b])
        archive, _, members, blobs, report = self.pack(group)
        self.assertEqual(len(blobs), 1)
        self.assertEqual(members[0]["blocks"], members[1]["blocks"])
        self.assertLess(report["archiveBytes"], report["originalFileBytes"])
        forced, index, _, blobs, _ = self.forced(group, "byte-exact", {"kind": "fixed", "size": 4096}, "chunks.p13")
        with zipfile.ZipFile(forced) as source:
            self.assertEqual(len([key for key in source.namelist() if key.startswith("chunks/")]), len(index["chunks"]))
        self.assertGreater(len(blobs), 1)
        self.assertTrue(archive.exists())

    def test_content_address_distinguishes_raw_bytes_from_the_same_zlib_stream(self):
        # The encoded payloads are identical, but their decoded values differ.
        a, b = self.root / "compressible.bin", self.root / "already-zlib.bin"
        a.write_bytes(b"x" * 200)
        b.write_bytes(zlib.compress(a.read_bytes(), 9))
        archive, _, members, blobs, _ = self.pack(self.gallery([a, b]))
        self.assertNotEqual(members[0]["blocks"], members[1]["blocks"])
        self.assertEqual(len(blobs), 2)
        self.assertEqual(len(set(blobs.values())), 1)
        with storage.GalleryReader(archive) as reader:
            self.assertEqual(reader.reconstruct("image:1"), a.read_bytes())
            self.assertEqual(reader.reconstruct("image:2"), b.read_bytes())

    def test_one_image_only_reads_its_chunks_even_if_other_image_corrupt(self):
        group = self.edits()
        archive, _, members, _, _ = self.pack(group, "pixel-exact")
        target = members[0]
        other_digest = next(d for d in members[1]["blocks"] if d not in target["blocks"])
        def corrupt(values):
            values["chunks/" + other_digest] = b"corrupt"
        damaged = self.mutate(archive, corrupt)
        with storage.GalleryReader(damaged) as reader:
            self.assertEqual(reader.reconstruct(target["id"]), self.pixels.tobytes())
            self.assertEqual(set(reader.chunk_reads), set(target["blocks"]))
            self.assertEqual(len(reader.chunk_reads), len(set(target["blocks"])))
            self.assertTrue(reader.verify(target["id"])["verified"])
        receipt = storage.extract(damaged, self.root / "single", target["id"])
        self.assertEqual(len(receipt["members"]), 1)

    def test_corrupt_and_missing_shared_chunks_list_only_actual_dependants(self):
        archive, _, members, _, _ = self.pack(self.edits(), "pixel-exact")
        shared = next(d for d in members[0]["blocks"] if sum(d in m["blocks"] for m in members) > 1)
        dependants = {m["id"] for m in members if shared in m["blocks"]}
        for missing in (False, True):
            def change(values):
                if missing:
                    del values["chunks/" + shared]
                else:
                    values["chunks/" + shared] = b"bad bytes"
            damaged = self.mutate(archive, change, f"damage-{missing}.p13")
            with storage.GalleryReader(damaged) as reader:
                result = reader.diagnose()
            self.assertFalse(result["verified"])
            self.assertEqual({r["id"] for r in result["members"] if not r["verified"]}, dependants)
            self.assertEqual(len(result["damagedChunks"]), 1)
            self.assertEqual(set(result["damagedChunks"][0]["dependants"]), dependants)

    def test_explicit_matching_source_repairs_corrupt_or_missing_chunks(self):
        group = self.edits()
        archive, _, members, _, _ = self.pack(group, "pixel-exact")
        shared = next(d for d in members[0]["blocks"] if sum(d in m["blocks"] for m in members) > 1)
        def damage(values):
            del values["chunks/" + shared]
        damaged = self.mutate(archive, damage)
        output = self.root / "repaired.p13"
        repaired = storage.repair(damaged, [group["members"][0]], output)
        self.assertTrue(repaired["verified"])
        self.assertEqual(repaired["repairedChunks"], [shared])
        self.assertEqual(output.read_bytes(), archive.read_bytes())
        self.assertTrue(damaged.exists())
        wrong = self.image(self.pixels ^ 1, "wrong.png")
        with self.assertRaisesRegex(ValueError, "does not match"):
            storage.repair(damaged, [{"id": "image:1", "path": str(wrong)}], self.root / "wrong-repair.p13")
        with self.assertRaisesRegex(ValueError, "cover every"):
            storage.repair(damaged, [], self.root / "no-repair.p13")

    def test_remove_keeps_chunks_needed_by_survivors_and_old_revision(self):
        group = self.edits()
        archive, index, members, _, _ = self.pack(group, "pixel-exact")
        original = archive.read_bytes()
        output = self.root / "removed.p13"
        result = storage.revise(archive, output, removals=["image:1", "image:4"])
        self.assertTrue(result["verified"])
        self.assertEqual(result["reencodedExistingChunks"], 0)
        self.assertEqual(archive.read_bytes(), original)
        with storage.GalleryReader(output) as reader:
            self.assertEqual(list(reader.entries), ["image:2", "image:3"])
            self.assertEqual([entry["order"] for entry in reader.members], [0, 1])
            self.assertEqual(reader.index["representative"], "image:2")
            self.assertEqual(reader.index["generation"], 2)
            self.assertEqual(reader.index["parentSHA256"], base.sha(base.json_bytes(index)))
            live = {d for m in members if m["id"] in reader.entries for d in m["blocks"]}
            self.assertEqual(set(reader.index["chunks"]), live)
            for identity in reader.entries:
                self.assertEqual(reader.reconstruct(identity), base.decode_pixels(Path(group["members"][int(identity[-1]) - 1]["path"]).read_bytes())[0])

    def test_append_keeps_old_chunks_without_old_source_paths_and_unrelated_gallery(self):
        group = self.edits()
        archive, _, members, blobs, _ = self.pack(group, "pixel-exact")
        other, _, _, _, _ = self.pack(self.gallery([self.image(self.pixels ^ 3, "other.png")]), name="other.p13")
        other_bytes = other.read_bytes()
        original = archive.read_bytes()
        for member in group["members"]:
            Path(member["path"]).unlink()
        added = self.image(self.pixels ^ 4, "append.png")
        output = self.root / "appended.p13"
        # Candidate preparation happens only for the newly appended member.
        with mock.patch.object(storage.base, "prepare", wraps=base.prepare) as prepare:
            result = storage.revise(archive, output, additions=[{"id": "image:5", "path": str(added)}])
        self.assertEqual(prepare.call_count, 1)
        self.assertEqual(len(prepare.call_args.args[0]["members"]), 1)
        self.assertEqual(result["reencodedExistingChunks"], 0)
        self.assertEqual(archive.read_bytes(), original)
        self.assertEqual(other.read_bytes(), other_bytes)
        with storage.GalleryReader(output) as reader:
            for member in members:
                self.assertEqual(reader.entries[member["id"]]["blocks"], member["blocks"])
            for digest, data in blobs.items():
                self.assertEqual(reader.read_chunk(digest), data)
            self.assertEqual(reader.reconstruct("image:5"), base.decode_pixels(added.read_bytes())[0])

    def test_revision_rejects_unknown_duplicate_empty_and_oversize_members(self):
        archive, _, _, _, _ = self.pack(self.edits(), "pixel-exact")
        with self.assertRaisesRegex(ValueError, "unknown"):
            storage.revise(archive, self.root / "unknown.p13", removals=["absent"])
        with self.assertRaisesRegex(ValueError, "new"):
            storage.revise(archive, self.root / "duplicate.p13", additions=[{"id": "image:1", "path": str(self.image(self.pixels, "again.png"))}])
        with self.assertRaisesRegex(ValueError, "1–16"):
            storage.revise(archive, self.root / "empty.p13", removals=[f"image:{n}" for n in range(1, 5)])
        additions = [{"id": f"new:{n}", "path": str(self.root / "original.png")} for n in range(13)]
        with self.assertRaisesRegex(ValueError, "1–16"):
            storage.revise(archive, self.root / "oversize.p13", additions=additions)

    def test_minimum_savings_gates_and_noise_fallback_are_real_complete_sizes(self):
        group = self.edits()
        _, _, _, _, report = self.pack(group, "pixel-exact")
        self.assertTrue(report["sharingSelected"])
        saving = min(report["savingsVsIndependentArchiveBytes"], report["savingsVsOriginalBytes"])
        index, _, _, at_limit = storage.plan_gallery(group, "pixel-exact", min_bytes=saving, min_percent=0)
        self.assertTrue(at_limit["sharingSelected"])
        _, _, _, rejected = storage.plan_gallery(group, "pixel-exact", min_bytes=saving + 1, min_percent=0)
        self.assertFalse(rejected["sharingSelected"])
        self.assertEqual(rejected["layout"], {"kind": "independent"})
        self.assertEqual(index["algorithm"]["kind"], "tile")
        noise = self.image(self.rng.integers(0, 256, self.pixels.shape, dtype=np.uint8), "noise.png")
        _, _, _, no_sharing = storage.plan_gallery(self.gallery([self.root / "original.png", noise]), "pixel-exact")
        self.assertFalse(no_sharing["sharingSelected"])
        self.assertIsNotNone(no_sharing["fallback"])
        self.assertEqual(no_sharing["physicalBytesWithRetainedSources"], no_sharing["originalFileBytes"] + no_sharing["archiveBytes"])
        for bad in (-1, float("nan"), float("inf"), 101, True):
            with self.assertRaises(ValueError):
                storage.plan_gallery(group, min_percent=bad)

    def test_repeated_tiles_are_read_once_and_allocation_is_not_physical_duplication(self):
        tile = self.rng.integers(0, 256, (64, 64, 4), dtype=np.uint8)
        repeated = np.tile(tile, (2, 3, 1))
        a = self.image(repeated, "repeated.png")
        archive, _, members, blobs, report = self.forced(self.gallery([a]), "pixel-exact", {"kind": "tile", "size": 64})
        self.assertEqual(len(members[0]["blocks"]), 6)
        self.assertEqual(len(blobs), 1)
        with storage.GalleryReader(archive) as reader:
            self.assertEqual(reader.reconstruct("image:1"), repeated.tobytes())
            self.assertEqual(len(reader.chunk_reads), 1)
        self.assertEqual(report["allocations"][0]["encodedChunkBytesNeeded"], sum(map(len, blobs.values())))

    def test_malformed_members_and_references_are_rejected(self):
        archive, _, _, _, _ = self.pack(self.edits(), "pixel-exact")
        changes = [
            lambda i, m: i.update(version=99),
            lambda i, m: i.update(version=True),
            lambda i, m: i.update(exporter="unknown"),
            lambda i, m: i.update(algorithm={"kind": "tile", "size": 7}),
            lambda i, m: i.update(parentSHA256="0" * 64),
            lambda i, m: m[0].update(name="../escape.png"),
            lambda i, m: m[0].update(order=1),
            lambda i, m: m[1].update(id=m[0]["id"]),
            lambda i, m: m[0].update(blocks=["f" * 64]),
            lambda i, m: m[0].update(blocks=m[0]["blocks"][:-1]),
            lambda i, m: m[0]["image"].update(width=20000),
            lambda i, m: m[0]["image"].update(bits=16),
            lambda i, m: m[0].update(metadataSHA256="0" * 64),
            lambda i, m: m[0].update(representationBytes=base.MAX_FILE + 1),
            lambda i, m: i["chunks"][m[0]["blocks"][0]].update(decodedBytes=1),
        ]
        for n, change in enumerate(changes):
            with self.subTest(case=n):
                altered = self.mutate(archive, lambda values: self.change_index(values, change), f"invalid-{n}.p13")
                with self.assertRaises((ValueError, KeyError)):
                    with storage.GalleryReader(altered):
                        pass

    def test_zip_paths_duplicates_checksums_and_directory_bombs(self):
        archive, _, _, _, _ = self.pack(self.edits(), "pixel-exact")
        transforms = [lambda v: v.update({"../escape": b"x"}), lambda v: v.update({"gallery.sha256": b"0" * 65}),
                      lambda v: v.update({"chunks/" + "f" * 64: b"unreferenced"})]
        for n, change in enumerate(transforms):
            altered = self.mutate(archive, change, f"zip-invalid-{n}.p13")
            with self.assertRaises(ValueError):
                with storage.GalleryReader(altered):
                    pass
        duplicate = self.root / "duplicate-zip.p13"
        with zipfile.ZipFile(archive) as original, zipfile.ZipFile(duplicate, "w") as target:
            for info in original.infolist():
                target.writestr(info.filename, original.read(info.filename))
            import warnings
            with warnings.catch_warnings():
                warnings.simplefilter("ignore", UserWarning)
                target.writestr("gallery.json", original.read("gallery.json"))
        with self.assertRaisesRegex(ValueError, "duplicate"):
            storage.GalleryReader(duplicate)
        for n, (offset, value, fmt) in enumerate([(12, storage.MAX_DIRECTORY + 1, "<I"), (10, 65535, "<H")]):
            raw = bytearray(archive.read_bytes())
            struct.pack_into(fmt, raw, len(raw) - 22 + offset, value)
            path = self.root / f"directory-bomb-{n}.p13"
            path.write_bytes(raw)
            with mock.patch.object(storage.zipfile, "ZipFile", side_effect=AssertionError("allocated before preflight")):
                with self.assertRaisesRegex(ValueError, "directory"):
                    storage.GalleryReader(path)

    def test_bounded_zlib_and_reference_bombs(self):
        archive, _, members, _, _ = self.pack(self.edits(), "pixel-exact")
        digest = members[0]["blocks"][0]
        bomb = zlib.compress(b"x" * 1000000)
        record = {"encoding": "zlib", "blobBytes": len(bomb), "decodedBytes": 5, "decodedSHA256": base.sha(b"xxxxx")}
        with self.assertRaisesRegex(ValueError, "oversized"):
            storage.decode_block(record, bomb, storage.chunk_address(record, bomb))
        def inflate(index, members):
            members[0]["blocks"] = [digest] * (storage.MAX_REFERENCES + 1)
        altered = self.mutate(archive, lambda v: self.change_index(v, inflate))
        with self.assertRaisesRegex(ValueError, "reference budget"):
            storage.GalleryReader(altered)
        with mock.patch.object(storage, "MAX_CHUNKS", 1):
            prepared, representative = base.prepare(self.edits(), "pixel-exact")
            with self.assertRaisesRegex(ValueError, "budget"):
                storage.candidate("g", prepared, representative, "pixel-exact", {"kind": "tile", "size": 32})

    def test_empty_byte_stream_and_reject_unsupported_pixel_precision(self):
        empty = self.root / "empty.bin"
        empty.write_bytes(b"")
        archive, _, _, _, _ = self.forced(self.gallery([empty]), "byte-exact", {"kind": "fixed", "size": 4096})
        with storage.GalleryReader(archive) as reader:
            self.assertEqual(reader.reconstruct("image:1"), b"")
        # Valid one-pixel 16-bit RGB PNG; Pillow's 8-bit conversion is forbidden.
        png = base.PNG_SIGNATURE + base.png_chunk("IHDR", struct.pack(">IIBBBBB", 1, 1, 16, 2, 0, 0, 0))
        png += base.png_chunk("IDAT", zlib.compress(b"\0\x12\x34\x56\x78\x9a\xbc")) + base.png_chunk("IEND", b"")
        high = self.root / "rgb16.png"
        high.write_bytes(png)
        with self.assertRaisesRegex(ValueError, "16-bit multichannel"):
            storage.plan_gallery(self.gallery([high]), "pixel-exact")
        self.pack(self.gallery([high]), name="rgb16-bytes.p13")

    def test_cancelled_pack_extract_and_exclusive_outputs(self):
        index, members, blobs, _ = storage.plan_gallery(self.edits(), "pixel-exact")
        output = self.root / "cancelled.p13"
        def interrupt(phase, _path):
            if phase in ("entry", "member"):
                raise KeyboardInterrupt
        with self.assertRaises(KeyboardInterrupt):
            storage.write_pack(index, members, blobs, output, interrupt)
        self.assertFalse(output.exists())
        self.assertEqual(list(self.root.glob(".p13-*.staging")), [])
        storage.write_pack(index, members, blobs, output)
        original = output.read_bytes()
        with self.assertRaisesRegex(ValueError, "already exists"):
            storage.write_pack(index, members, blobs, output)
        self.assertEqual(output.read_bytes(), original)
        destination = self.root / "cancelled-export"
        with self.assertRaises(KeyboardInterrupt):
            storage.extract(output, destination, checkpoint=interrupt)
        self.assertFalse(destination.exists())
        destination.mkdir()
        (destination / "sentinel").write_text("keep")
        with self.assertRaises(FileExistsError):
            storage.extract(output, destination)
        self.assertEqual((destination / "sentinel").read_text(), "keep")

    def test_real_process_death_at_pack_phases_and_recovery(self):
        group = self.edits()
        for phase in ("created", "entry", "durable", "verified", "published"):
            with self.subTest(phase=phase):
                directory = self.root / phase
                directory.mkdir()
                output = directory / "gallery.p13"
                script = "\n".join([
                    "import json, os, sys", f"sys.path.insert(0, {str(Path(storage.__file__).parent)!r})", "import gallery_block_storage as s",
                    f"group = json.loads({json.dumps(group)!r})", "i, m, b, _ = s.plan_gallery(group, 'pixel-exact')",
                    "def kill(phase, path):", f"    if phase == {phase!r}: os._exit(77)",
                    f"s.write_pack(i, m, b, {str(output)!r}, kill)"])
                result = subprocess.run([sys.executable, "-c", script], capture_output=True, timeout=60)
                self.assertEqual(result.returncode, 77, result.stderr.decode())
                staging = list(directory.glob(".p13-*.staging"))
                self.assertEqual(len(staging), 1)
                if phase in ("created", "entry"):
                    with self.assertRaises((ValueError, KeyError, zipfile.BadZipFile)):
                        storage.recover(staging[0], directory / "recovered.p13")
                    self.assertFalse(output.exists())
                elif phase == "published":
                    with storage.GalleryReader(output) as reader:
                        self.assertTrue(reader.verify()["verified"])
                else:
                    self.assertTrue(storage.recover(staging[0], directory / "recovered.p13")["verified"])
                self.assertTrue(staging[0].exists())
                for member in group["members"]:
                    self.assertTrue(Path(member["path"]).exists())

    def test_real_process_death_during_unpack_and_revision(self):
        group = self.edits()
        archive, _, _, _, _ = self.pack(group, "pixel-exact")
        incomplete = self.root / "interrupted-export"
        script = "\n".join(["import os, sys", f"sys.path.insert(0, {str(Path(storage.__file__).parent)!r})", "import gallery_block_storage as s",
                             "def kill(phase, path):", "    if phase == 'member': os._exit(78)",
                             f"s.extract({str(archive)!r}, {str(incomplete)!r}, checkpoint=kill)"])
        killed = subprocess.run([sys.executable, "-c", script], capture_output=True, timeout=30)
        self.assertEqual(killed.returncode, 78, killed.stderr.decode())
        self.assertFalse((incomplete / "extraction.json").exists())
        self.assertTrue(storage.extract(archive, self.root / "restarted-export")["complete"])
        revised = self.root / "interrupted-revision.p13"
        before = archive.read_bytes()
        script = "\n".join(["import os, sys", f"sys.path.insert(0, {str(Path(storage.__file__).parent)!r})", "import gallery_block_storage as s",
                             "def kill(phase, path):", "    if phase == 'verified': os._exit(79)",
                             f"s.revise({str(archive)!r}, {str(revised)!r}, removals=['image:1'], checkpoint=kill)"])
        killed = subprocess.run([sys.executable, "-c", script], capture_output=True, timeout=30)
        self.assertEqual(killed.returncode, 79, killed.stderr.decode())
        self.assertEqual(archive.read_bytes(), before)
        self.assertFalse(revised.exists())
        staging = list(self.root.glob(".p13-*.staging"))
        self.assertEqual(len(staging), 1)
        storage.recover(staging[0], revised)
        with storage.GalleryReader(revised) as reader:
            self.assertEqual(list(reader.entries), ["image:2", "image:3", "image:4"])

    @unittest.skipUnless(hasattr(os, "mkfifo"), "POSIX FIFO")
    def test_fifo_rejected_without_blocking(self):
        fifo = self.root / "fifo"
        os.mkfifo(fifo)
        with self.assertRaisesRegex(ValueError, "regular"):
            storage.GalleryReader(fifo)
        with self.assertRaisesRegex(ValueError, "regular"):
            storage.plan_gallery(self.gallery([fifo]))

    def test_input_relative_paths_per_item_errors_and_real_cli(self):
        group = self.edits()
        for member in group["members"]:
            member["path"] = Path(member["path"]).name
        spec = self.root / "galleries.json"
        spec.write_text(json.dumps({"galleries": [group]}))
        resolved = storage.read_galleries(spec)
        self.assertTrue(Path(resolved[0]["members"][0]["path"]).is_absolute())
        command = [sys.executable, str(Path(storage.__file__))]
        estimate = subprocess.run(command + ["estimate", "--galleries", str(spec), "--gallery", group["id"], "--fidelity", "pixel-exact"], capture_output=True, timeout=60)
        self.assertEqual(estimate.returncode, 0, estimate.stderr.decode())
        self.assertTrue(json.loads(estimate.stdout)["sharingSelected"])
        archive = self.root / "cli.p13"
        packed = subprocess.run(command + ["pack", "--galleries", str(spec), "--gallery", group["id"], "--fidelity", "pixel-exact", "--output", str(archive)], capture_output=True, timeout=60)
        self.assertEqual(packed.returncode, 0, packed.stderr.decode())
        for action in ("verify", "diagnose"):
            result = subprocess.run(command + [action, str(archive)], capture_output=True, timeout=30)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            self.assertTrue(json.loads(result.stdout)["verified"])
        exported = subprocess.run(command + ["extract", str(archive), "--entry", "image:3", "--output", str(self.root / "cli-export")], capture_output=True, timeout=30)
        self.assertEqual(exported.returncode, 0, exported.stderr.decode())
        broken = copy.deepcopy(group)
        broken["id"] = "missing"
        broken["members"][0]["path"] = "missing.png"
        spec.write_text(json.dumps({"galleries": [group, broken]}))
        report = self.root / "partial.json"
        failed = subprocess.run(command + ["benchmark", "--galleries", str(spec), "--output", str(report)], capture_output=True, timeout=60)
        self.assertEqual(failed.returncode, 1)
        results = json.loads(report.read_text())["results"]
        self.assertEqual(sum(row["verified"] for row in results), 2)
        self.assertEqual(sum(not row["verified"] for row in results), 2)


if __name__ == "__main__":
    unittest.main()
