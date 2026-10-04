"""P12 recovery, fidelity, overhead, bounds, fallback and actual crash tests."""
import copy
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import unittest
import zipfile
import zlib

import numpy as np
from PIL import Image, PngImagePlugin

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_delta_storage as storage


class DeltaStorageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.rng = np.random.default_rng(73)
        # High entropy makes small edits genuinely cheaper than independent storage.
        self.pixels = self.rng.integers(0, 256, (128, 192, 4), dtype=np.uint8)
        self.paths = []

    def tearDown(self):
        self.temp.cleanup()

    def image(self, pixels, name, **options):
        path = self.root / name
        Image.fromarray(pixels).save(path, **options)
        self.paths.append(path)
        return path

    def stack(self, paths, representative=None):
        members = [{"id": f"image:{index + 1}", "path": str(path)} for index, path in enumerate(paths)]
        return {"id": "fixture-stack", "representative": representative or members[0]["id"], "members": members}

    def edited_stack(self):
        paths = [self.image(self.pixels, "original.png")]
        for index in range(3):
            changed = self.pixels.copy()
            changed[7 + index:12 + index, 13:20, 0] ^= 1 + index
            paths.append(self.image(changed, f"edit{index}.png"))
        return self.stack(paths, "image:4")

    def pack(self, group, fidelity="byte-exact", name="test.p12", **kwargs):
        manifest, blobs, report = storage.plan_group(group, fidelity, **kwargs)
        output = self.root / name
        result = storage.write_pack(manifest, blobs, output)
        self.assertTrue(result["verified"])
        self.assertEqual(output.stat().st_size, report["archiveBytes"])
        self.assertEqual(report["uniqueBlobBytes"] + report["manifestAndIndexBytes"], output.stat().st_size)
        return output, manifest, blobs, report

    def mutate(self, archive, transform):
        with zipfile.ZipFile(archive) as source:
            values = {name: source.read(name) for name in source.namelist()}
        transform(values)
        output = self.root / (archive.stem + "-mutated.p12")
        with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_STORED) as target:
            for name, value in values.items():
                target.writestr(name, value)
        return output

    @staticmethod
    def edit_manifest(values, change):
        manifest = json.loads(values["manifest.json"])
        change(manifest)
        values["manifest.json"] = storage.json_bytes(manifest)
        values["manifest.sha256"] = (storage.sha(values["manifest.json"]) + "\n").encode()

    def test_byte_exact_cross_format_copies_and_offline_restore(self):
        png = self.image(self.pixels, "sample.png")
        jpeg = self.image(self.pixels[:, :, :3], "sample.jpg", quality=93)
        duplicate = self.root / "copy.png"
        duplicate.write_bytes(png.read_bytes())
        group = self.stack([png, jpeg, duplicate])
        originals = {member["id"]: Path(member["path"]).read_bytes() for member in group["members"]}
        pack, manifest, _, report = self.pack(group)
        self.assertEqual(manifest["fidelity"], "byte-exact")
        self.assertTrue(report["sourcesRetained"])
        for path in (png, jpeg, duplicate):
            path.unlink()
        # No database, original paths or original dependency files remain.
        moved = self.root / "empty-environment"
        moved.mkdir()
        portable = moved / "backup.p12"
        portable.write_bytes(pack.read_bytes())
        receipt = storage.extract(portable, moved / "restored")
        for member in receipt["members"]:
            self.assertEqual((moved / "restored" / member["name"]).read_bytes(), originals[member["id"]])
            self.assertEqual(member["sourceMD5"], member["exportMD5"])
            self.assertEqual(member["sourceSHA256"], member["exportSHA256"])

    def test_exact_files_share_one_immutable_blob(self):
        original = self.image(self.pixels, "a.png")
        paths = [original]
        for index in range(3):
            path = self.root / f"copy{index}.png"
            path.write_bytes(original.read_bytes())
            paths.append(path)
        _, _, blobs, report = self.pack(self.stack(paths))
        self.assertEqual(len(blobs), 1)
        self.assertLess(report["archiveBytes"], report["originalFileBytes"])

    def test_pixel_deltas_include_one_level_changes_and_transparent_rgb(self):
        group = self.edited_stack()
        expected = {member["id"]: storage.decode_pixels(Path(member["path"]).read_bytes())[0] for member in group["members"]}
        pack, manifest, _, report = self.pack(group, "pixel-exact")
        self.assertEqual(report["layout"], "shared-base")
        self.assertEqual(manifest["representative"], "image:4")
        self.assertEqual(len(report["baseCandidates"]), 4)
        with storage.PackReader(pack) as reader:
            for identity, pixels in expected.items():
                self.assertEqual(reader.reconstruct(identity), pixels)
        # A transparent RGB modification must be stored even if invisible in previews.
        changed = self.pixels.copy()
        changed[:, :, 3] = 0
        a = self.image(changed, "transparent-a.png")
        changed[50, 60, 2] ^= 1
        b = self.image(changed, "transparent-b.png")
        second, _, _, _ = self.pack(self.stack([a, b]), "pixel-exact", "alpha.p12")
        with storage.PackReader(second) as reader:
            self.assertEqual(reader.reconstruct("image:2"), changed.tobytes())

    def test_canvas_offset_borders_and_floor_resizing_reconstruct_exactly(self):
        original = self.image(self.pixels, "original.png")
        bordered = np.zeros((146, 216, 4), dtype=np.uint8)
        bordered[9:137, 12:204] = self.pixels
        border_path = self.image(bordered, "border.png")
        descriptor = {"width": 192, "height": 128, "channels": 4, "plane": "rgba8"}
        target = {**descriptor, "width": 91, "height": 60}
        geometry = {"width": 91, "height": 60, "left": 0, "top": 0}
        scaled = storage.predict(self.pixels.tobytes(), descriptor, target, geometry)
        # Independent coordinate oracle, including floor behavior and every channel.
        expected = self.pixels[(np.arange(60) * 128 // 60)[:, None], (np.arange(91) * 192 // 91)[None, :]]
        self.assertEqual(scaled, expected.tobytes())
        resized = self.image(expected, "resized.png")
        group = self.stack([original, border_path, resized])
        pack, _, _, _ = self.pack(group, "pixel-exact")
        with storage.PackReader(pack) as reader:
            self.assertEqual(reader.reconstruct("image:2"), bordered.tobytes())
            self.assertEqual(reader.reconstruct("image:3"), expected.tobytes())

    def test_hint_translation_preserves_full_target_canvas(self):
        base = self.image(self.pixels, "base.png")
        target = np.zeros_like(self.pixels)
        target[3:, 5:] = self.pixels[:-3, :-5]
        target[:3, :, 2] = 151
        target[:, :5, 3] = 201
        path = self.image(target, "translated.png")
        group = self.stack([base, path])
        group["members"][1]["predictions"] = {"image:1": [{"width": 192, "height": 128, "left": 5, "top": 3}]}
        pack, _, _, _ = self.pack(group, "pixel-exact")
        with storage.PackReader(pack) as reader:
            self.assertEqual(reader.reconstruct("image:2"), target.tobytes())

    def test_16_bit_grayscale_preserves_all_values(self):
        values = self.rng.integers(0, 65536, (128, 192), dtype=np.uint16)
        a = self.image(values, "gray.png")
        changed = values.copy()
        changed[20:24, 30:38] ^= 1
        b = self.image(changed, "gray-edit.png")
        pack, manifest, _, _ = self.pack(self.stack([a, b]), "pixel-exact")
        self.assertEqual(manifest["entries"][0]["image"]["bits"], 16)
        receipt = storage.extract(pack, self.root / "gray-export")
        for member, original in zip(receipt["members"], (values, changed)):
            with Image.open(self.root / "gray-export" / member["name"]) as image:
                np.testing.assert_array_equal(np.asarray(image), original)

    def test_color_profile_exif_png_text_and_provenance_survive(self):
        from PIL import ImageCms
        icc = ImageCms.ImageCmsProfile(ImageCms.createProfile("sRGB")).tobytes()
        exif = Image.Exif()
        exif[274] = 6
        exif[315] = "fixture artist"
        info = PngImagePlugin.PngInfo()
        info.add_text("Description", "fixture metadata")
        info.add_itxt("XML:com.adobe.xmp", "<fixture/>")
        a = self.image(self.pixels, "metadata.png", icc_profile=icc, exif=exif, pnginfo=info, dpi=(96, 96))
        jpeg = self.image(self.pixels[:, :, :3], "metadata.jpg", icc_profile=icc, exif=exif, quality=95)
        pack, _, _, report = self.pack(self.stack([a, jpeg]), "pixel-exact")
        receipt = storage.extract(pack, self.root / "meta-export")
        exports = self.root / "meta-export"
        self.assertEqual(report["independentLosslessSidecarBytes"], (exports / "extraction.json").stat().st_size)
        self.assertEqual(report["independentLosslessFileBytes"], sum(path.stat().st_size for path in exports.iterdir()))
        for member in receipt["members"]:
            with Image.open(self.root / "meta-export" / member["name"]) as image:
                self.assertEqual(image.info["icc_profile"], icc)
                self.assertEqual(image.getexif()[274], 6)
                self.assertEqual(image.getexif()[315], "fixture artist")
                if member["id"] == "image:1":
                    self.assertEqual(image.info["Description"], "fixture metadata")
            self.assertNotEqual(member["sourceSHA256"], member["exportSHA256"])
            self.assertEqual(member["sourceMD5"], hashlib.md5((a if member["id"] == "image:1" else jpeg).read_bytes()).hexdigest())

    def test_pixel_mode_rejects_multichannel_16_bit_and_animation_bytes_remain_exact(self):
        # Hand-built genuine 16-bit RGB PNG; Pillow normally silently truncates it.
        path = self.root / "rgb16.png"
        header = struct.pack(">IIBBBBB", 1, 1, 16, 2, 0, 0, 0)
        path.write_bytes(storage.PNG_SIGNATURE + storage.png_chunk("IHDR", header) + storage.png_chunk("IDAT", zlib.compress(b"\0\x00\x01\x12\x34\xff\xfe")) + storage.png_chunk("IEND", b""))
        with self.assertRaisesRegex(ValueError, "16-bit multichannel"):
            storage.plan_group(self.stack([path]), "pixel-exact")
        gif = self.root / "animation.gif"
        Image.new("RGB", (10, 10), "red").save(gif, save_all=True, append_images=[Image.new("RGB", (10, 10), "blue")], duration=[70, 130], loop=2)
        with self.assertRaisesRegex(ValueError, "PNG/JPEG/WebP"):
            storage.plan_group(self.stack([gif]), "pixel-exact")
        pack, _, _, _ = self.pack(self.stack([path, gif]))
        with storage.PackReader(pack) as reader:
            self.assertEqual(reader.reconstruct("image:1"), path.read_bytes())
            self.assertEqual(reader.reconstruct("image:2"), gif.read_bytes())

    def test_no_savings_fallback_and_both_inclusive_minima(self):
        group = self.edited_stack()
        _, _, normal = storage.plan_group(group, "pixel-exact", min_percent=0)
        saving = normal["independentArchiveBytes"] - normal["bestCandidateArchiveBytes"]
        _, _, inclusive = storage.plan_group(group, "pixel-exact", min_bytes=saving, min_percent=0)
        self.assertTrue(inclusive["eligibleDelta"])
        manifest, _, excluded = storage.plan_group(group, "pixel-exact", min_bytes=saving + 1, min_percent=0)
        self.assertEqual(excluded["layout"], "independent")
        self.assertTrue(all(e["kind"] == "full" for e in manifest["entries"]))
        _, _, percent_excluded = storage.plan_group(group, "pixel-exact", min_percent=100)
        self.assertFalse(percent_excluded["eligibleDelta"])
        unrelated = [self.image(self.rng.integers(0, 256, self.pixels.shape, dtype=np.uint8), f"random{i}.png") for i in range(2)]
        _, _, _, report = self.pack(self.stack(unrelated), "pixel-exact")
        self.assertEqual(report["layout"], "independent")

    def test_base_selection_is_independent_of_representative_and_file_size(self):
        group = self.edited_stack()
        # A different metadata-heavy representative is deliberately the largest file.
        info = PngImagePlugin.PngInfo()
        info.add_text("unused", "representative metadata " * 100)
        image = Image.open(group["members"][-1]["path"])
        image.save(group["members"][-1]["path"], pnginfo=info)
        image.close()
        _, _, report = storage.plan_group(group, "pixel-exact", min_percent=0)
        sizes = report["baseCandidates"]
        self.assertEqual(report["bestCandidateArchiveBytes"], min(row["archiveBytes"] for row in sizes))
        self.assertEqual(report["representative"], "image:4")
        self.assertNotEqual(report["compressionBase"], "image:4")

    def test_corrupt_missing_blobs_identify_dependants_and_allow_independent_member(self):
        group = self.edited_stack()
        pack, manifest, _, _ = self.pack(group, "pixel-exact")
        delta = next(e for e in manifest["entries"] if e["kind"] == "delta")
        damaged = self.mutate(pack, lambda values: values.pop("blobs/" + delta["blob"]))
        with storage.PackReader(damaged) as reader:
            result = reader.verify()
            failures = [e for e in result["members"] if not e["verified"]]
            self.assertIn(delta["id"], [e["id"] for e in failures])
            self.assertTrue(reader.verify(delta["base"])["verified"])
            self.assertEqual(next(e for e in failures if e["id"] == delta["id"])["dependsOn"], delta["base"])
        base = next(e for e in manifest["entries"] if e["id"] == delta["base"])
        def corrupt(values):
            values["blobs/" + base["blob"]] = b"broken"
        broken = self.mutate(pack, corrupt)
        with storage.PackReader(broken) as reader:
            self.assertFalse(reader.verify()["verified"])
            self.assertTrue(all(not e["verified"] for e in reader.verify()["members"] if e["dependsOn"] == base["id"]))

    def test_single_member_reads_only_its_payload_and_one_base(self):
        pack, manifest, _, _ = self.pack(self.edited_stack(), "pixel-exact")
        delta = next(e for e in manifest["entries"] if e["kind"] == "delta")
        with storage.PackReader(pack) as reader:
            from unittest.mock import patch
            with patch.object(reader.archive, "read", wraps=reader.archive.read) as read:
                receipt = storage.extract(pack, self.root / "one", delta["id"])
                # Measure the already-open reader separately; extraction likewise selects one ID.
                self.assertTrue(reader.verify(delta["id"])["verified"])
                blob_reads = [call.args[0] for call in read.call_args_list if call.args[0].startswith("blobs/")]
                self.assertEqual(len(blob_reads), 2)
        self.assertEqual(len(receipt["members"]), 1)

    def test_manifest_paths_cycles_versions_and_checksums_rejected(self):
        pack, _, _, _ = self.pack(self.edited_stack(), "pixel-exact")
        for change, message in ((lambda m: m.update(version=999), "version"),
                                (lambda m: m["entries"][0].update(name="../escape.png"), "unsafe export"),
                                (lambda m: m["entries"][0].update(order=1), "duplicate member order"),
                                (lambda m: m["entries"][0].update(kind="copy", base=m["entries"][0]["id"], geometry={"width": 192, "height": 128, "left": 0, "top": 0}), "chains/cycles")):
            mutated = self.mutate(pack, lambda values: self.edit_manifest(values, change))
            with self.assertRaisesRegex(ValueError, message):
                storage.PackReader(mutated)
        mutated = self.mutate(pack, lambda values: values.update({"manifest.json": values["manifest.json"] + b" "}))
        with self.assertRaisesRegex(ValueError, "manifest checksum"):
            storage.PackReader(mutated)

    def test_archive_traversal_duplicate_entries_and_compression_bombs_rejected(self):
        pack, _, _, _ = self.pack(self.edited_stack())
        mutated = self.mutate(pack, lambda values: values.update({"../escape": b"bad"}))
        with self.assertRaisesRegex(ValueError, "unsafe archive path"):
            storage.PackReader(mutated)
        duplicate = self.root / "duplicate.p12"
        with zipfile.ZipFile(pack) as original, zipfile.ZipFile(duplicate, "w") as out:
            for name in original.namelist():
                out.writestr(name, original.read(name))
            import warnings
            with warnings.catch_warnings():
                warnings.simplefilter("ignore")
                out.writestr("manifest.json", original.read("manifest.json"))
        with self.assertRaisesRegex(ValueError, "duplicate"):
            storage.PackReader(duplicate)
        compressed = self.root / "compressed.p12"
        with zipfile.ZipFile(pack) as original, zipfile.ZipFile(compressed, "w", compression=zipfile.ZIP_DEFLATED) as out:
            for name in original.namelist():
                out.writestr(name, original.read(name))
        with self.assertRaisesRegex(ValueError, "stored files"):
            storage.PackReader(compressed)
        with self.assertRaisesRegex(ValueError, "oversized"):
            storage.decompress(zlib.compress(b"x" * 100000), 10)
        with self.assertRaisesRegex(ValueError, "trailing"):
            storage.decompress(zlib.compress(b"abc") + b"trailing", 3)

    def test_directory_allocation_bomb_and_false_payload_lengths_rejected(self):
        pack, _, _, _ = self.pack(self.edited_stack())
        encoded = bytearray(pack.read_bytes())
        # Malicious central-directory size must be rejected before ZipFile opens.
        struct.pack_into("<I", encoded, len(encoded) - 10, 0xFFFFFF00)
        malformed = self.root / "directory-bomb.p12"
        malformed.write_bytes(encoded)
        from unittest.mock import patch
        with patch.object(storage.zipfile, "ZipFile", side_effect=AssertionError("allocated entry list")):
            with self.assertRaisesRegex(ValueError, "archive directory"):
                storage.PackReader(malformed)
        changed = self.mutate(pack, lambda values: self.edit_manifest(values, lambda m: m["entries"][0].update(blobBytes=1)))
        with storage.PackReader(changed) as reader:
            self.assertFalse(reader.verify()["verified"])

    @unittest.skipIf(os.name == "nt", "FIFO requires POSIX")
    def test_nonregular_inputs_rejected_without_blocking(self):
        fifo = self.root / "fifo.png"
        os.mkfifo(fifo)
        with self.assertRaisesRegex(ValueError, "regular"):
            storage.read_regular(fifo, storage.MAX_FILE)
        with self.assertRaisesRegex(ValueError, "regular"):
            storage.PackReader(fifo)

    def test_cancel_cleanup_and_existing_outputs_are_never_overwritten(self):
        group = self.edited_stack()
        manifest, blobs, _ = storage.plan_group(group)
        target = self.root / "cancel.p12"
        before = {p: p.read_bytes() for p in self.paths}
        def cancel(phase, _path):
            if phase == "entry":
                raise KeyboardInterrupt()
        with self.assertRaises(KeyboardInterrupt):
            storage.write_pack(manifest, blobs, target, cancel)
        self.assertFalse(target.exists())
        self.assertEqual(list(self.root.glob(".p12-*.staging")), [])
        target.write_bytes(b"sentinel")
        with self.assertRaisesRegex(ValueError, "already exists"):
            storage.write_pack(manifest, blobs, target)
        self.assertEqual(target.read_bytes(), b"sentinel")
        for path, data in before.items():
            self.assertEqual(path.read_bytes(), data)

    def test_actual_process_crashes_before_and_after_publication_recover(self):
        group = self.edited_stack()
        stacks = self.root / "stacks.json"
        stacks.write_text(json.dumps({"stacks": [group]}))
        for phase in ("created", "entry", "durable", "verified", "published"):
            target = self.root / f"crash-{phase}.p12"
            code = '''import os, sys
from pathlib import Path
sys.path.insert(0, sys.argv[1])
import image_delta_storage as s
groups = s.read_stacks(sys.argv[2])
m, b, _ = s.plan_group(groups[0], "pixel-exact")
def fail(phase, path):
    if phase == sys.argv[4]:
        print(path, flush=True)
        os._exit(77)
s.write_pack(m, b, sys.argv[3], fail)
'''
            result = subprocess.run([sys.executable, "-c", code, str(Path(storage.__file__).parent), str(stacks), str(target), phase], capture_output=True, text=True, timeout=30)
            self.assertEqual(result.returncode, 77, result.stderr)
            staging = Path(result.stdout.strip())
            self.assertTrue(staging.exists())
            if phase in ("created", "entry"):
                self.assertFalse(target.exists())
                with self.assertRaises((ValueError, KeyError, zipfile.BadZipFile)):
                    storage.recover(staging, self.root / f"invalid-{phase}.p12")
            elif phase == "published":
                with storage.PackReader(target) as reader:
                    self.assertTrue(reader.verify()["verified"])
            else:
                self.assertFalse(target.exists())
                recovered = self.root / f"recovered-{phase}.p12"
                self.assertTrue(storage.recover(staging, recovered)["verified"])
                self.assertTrue(staging.exists())
            staging.unlink()
        self.assertTrue(all(path.exists() for path in self.paths))

    def test_extract_cancellation_and_killed_extraction_can_restart(self):
        pack, _, _, _ = self.pack(self.edited_stack(), "pixel-exact")
        target = self.root / "cancel-extract"
        def cancel(phase, _path):
            if phase == "member":
                raise KeyboardInterrupt()
        with self.assertRaises(KeyboardInterrupt):
            storage.extract(pack, target, checkpoint=cancel)
        self.assertFalse(target.exists())
        target.mkdir()
        sentinel = target / "keep"
        sentinel.write_bytes(b"keep")
        with self.assertRaises(FileExistsError):
            storage.extract(pack, target)
        self.assertEqual(sentinel.read_bytes(), b"keep")
        partial = self.root / "partial"
        code = '''import os, sys
sys.path.insert(0, sys.argv[1])
import image_delta_storage as s
def fail(phase, _path):
    if phase == "member": os._exit(77)
s.extract(sys.argv[2], sys.argv[3], checkpoint=fail)
'''
        result = subprocess.run([sys.executable, "-c", code, str(Path(storage.__file__).parent), str(pack), str(partial)], timeout=30)
        self.assertEqual(result.returncode, 77)
        self.assertFalse((partial / "extraction.json").exists())
        receipt = storage.extract(pack, self.root / "restart")
        self.assertTrue(receipt["complete"])

    def test_limits_invalid_minima_relative_paths_and_benchmark_errors(self):
        group = self.edited_stack()
        for value in (float("nan"), float("inf"), -1, 101):
            with self.assertRaises(ValueError):
                storage.plan_group(group, min_percent=value)
        repeated = copy.deepcopy(group)
        repeated["members"][1]["id"] = repeated["members"][0]["id"]
        with self.assertRaisesRegex(ValueError, "unique"):
            storage.plan_group(repeated)
        many = {"id": "too-large", "members": group["members"] * 5}
        with self.assertRaisesRegex(ValueError, "1–16"):
            storage.plan_group(many)
        source = self.root / "missing.json"
        source.write_text(json.dumps({"stacks": [{"id": "missing", "members": [{"id": "image:1", "path": "absent.png"}]}]}))
        groups = storage.read_stacks(source)
        result = storage.benchmark(groups)
        self.assertEqual(len(result["results"]), 2)
        self.assertTrue(all("image:1" in r["error"] for r in result["results"]))
        self.assertEqual(groups[0]["members"][0]["path"], str(self.root / "absent.png"))

    def test_cli_pack_verify_extract_and_missing_setup(self):
        groups = {"stacks": [self.edited_stack()]}
        input_path = self.root / "stacks.json"
        input_path.write_text(json.dumps(groups))
        packed = self.root / "cli.p12"
        base = [sys.executable, storage.__file__]
        result = subprocess.run(base + ["pack", "--stacks", str(input_path), "--group", "fixture-stack", "--output", str(packed)], capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["fidelity"], "byte-exact")
        result = subprocess.run(base + ["verify", str(packed)], capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(json.loads(result.stdout)["verified"])
        result = subprocess.run(base + ["extract", str(packed), "--entry", "image:2", "--output", str(self.root / "cli-export")], capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        receipt = json.loads(result.stdout)
        self.assertEqual(len(receipt["members"]), 1)
        self.assertTrue(receipt["complete"])
        from unittest.mock import patch
        with patch.object(storage.importlib.util, "find_spec", return_value=None):
            with self.assertRaisesRegex(ValueError, "pip install Pillow numpy"):
                storage.require_dependencies()

    def test_cli_benchmark_records_unsupported_items_and_exits_nonzero(self):
        source = self.root / "opaque.bin"
        source.write_bytes(b"an archival container with no image decoder")
        input_path = self.root / "input.json"
        input_path.write_text(json.dumps({"stacks": [self.stack([source])]}))
        result_path = self.root / "results.json"
        result = subprocess.run([sys.executable, storage.__file__, "benchmark", "--stacks", str(input_path), "--output", str(result_path)], capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 1, result.stderr)
        rows = json.loads(result_path.read_text())["results"]
        self.assertTrue(rows[0]["verified"])
        self.assertFalse(rows[1]["verified"])
        self.assertIn("image:1", rows[1]["error"])


if __name__ == "__main__":
    unittest.main()
