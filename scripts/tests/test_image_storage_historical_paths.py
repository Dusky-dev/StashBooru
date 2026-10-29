"""Historical SQLite image ID -> exact media backup path provenance tests."""
from pathlib import Path
import hashlib
import json
import sqlite3
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_historical_paths as mapping

A = "a" * 64
B = "b" * 64


class HistoricalMappingTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        root = Path(tmp.name)
        self.database = root / "readonly-source.sqlite"
        db = sqlite3.connect(self.database)
        db.executescript("""
            CREATE TABLE folders (id INTEGER PRIMARY KEY, path TEXT, zip_file_id INTEGER);
            CREATE TABLE files (
                id INTEGER PRIMARY KEY, parent_folder_id INTEGER, basename TEXT,
                size INTEGER, zip_file_id INTEGER
            );
            CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" BOOLEAN);
            INSERT INTO folders VALUES (1, '/old/library/images', NULL);
            INSERT INTO files VALUES (11, 1, 'first.png', 6, NULL);
            INSERT INTO files VALUES (12, 1, 'second.png', 7, NULL);
            INSERT INTO images_files VALUES (201, 11, 1);
            INSERT INTO images_files VALUES (202, 12, 1);
        """)
        db.commit()
        db.close()
        self.expected = {"image:201": A, "image:202": B}
        self.roots = {"images": "/old/library"}
        self.entries = [
            {"root": "images", "path": "images/first.png", "size": 6, "sha256": A},
            {"root": "images", "path": "images/second.png", "size": 7, "sha256": B},
        ]

    def check(self):
        return mapping.map_historical_paths(
            self.database, self.expected, self.roots, self.entries
        )

    def sql(self, statement):
        conn = sqlite3.connect(self.database)
        conn.execute(statement)
        conn.commit()
        conn.close()

    def test_two_images_with_historical_missing_root(self):
        mapped = self.check()
        self.assertEqual([x["imageID"] for x in mapped], [201, 202])
        self.assertEqual(mapped[0]["backupMember"], "images:images/first.png")
        self.assertTrue(mapped[0]["historicalFileSizeVerified"])
        self.assertEqual(mapped[1]["sha256"], B)

    def test_wrong_path_with_same_hash_elsewhere_fails(self):
        self.entries[0]["path"] = "renamed/first.png"
        with self.assertRaises(ValueError):
            self.check()

    def test_wrong_hash_for_exact_historical_path_fails(self):
        self.entries[0]["sha256"] = B
        with self.assertRaises(ValueError):
            self.check()

    def test_ambiguous_overlapping_snapshot_roots_fail(self):
        self.roots["nested"] = "/old/library/images"
        self.entries.append({
            "root": "nested", "path": "first.png", "size": 6, "sha256": A
        })
        with self.assertRaises(ValueError):
            self.check()

    def test_nonunique_primary_and_zip_member_fail(self):
        self.sql('INSERT INTO images_files VALUES (201, 12, 1)')
        with self.assertRaises(ValueError):
            self.check()
        self.sql('DELETE FROM images_files WHERE image_id=201 AND file_id=12')
        self.sql('UPDATE files SET zip_file_id=55 WHERE id=11')
        with self.assertRaises(ValueError):
            self.check()

    def test_missing_primary_and_stale_backup_database_fail(self):
        self.sql('DELETE FROM images_files WHERE image_id=202')
        with self.assertRaises(ValueError):
            self.check()

    def test_bad_basename_and_path_traversal_fail(self):
        self.sql("UPDATE files SET basename='../outside.png' WHERE id=11")
        with self.assertRaises(ValueError):
            self.check()
        self.sql("UPDATE files SET basename='first.png' WHERE id=11")
        self.roots["images"] = "/old/library/../library"
        with self.assertRaises(ValueError):
            self.check()

    def test_size_disagreement_and_unscanned_file(self):
        self.sql("UPDATE files SET size=900 WHERE id=11")
        with self.assertRaises(ValueError):
            self.check()
        self.sql("UPDATE files SET size=-1 WHERE id=11")
        result = self.check()
        self.assertFalse(result[0]["historicalFileSizeVerified"])

    def test_invalid_identity_and_duplicate_manifest_paths(self):
        self.expected["image:002"] = self.expected.pop("image:202")
        with self.assertRaises(ValueError):
            self.check()
        self.expected = {"image:201": A, "image:202": B}
        self.entries.append(dict(self.entries[0]))
        with self.assertRaises(ValueError):
            self.check()



class HistoricalEndToEndTests(unittest.TestCase):
    def test_real_candidate_backup_set_historical_mapping_with_deleted_originals(self):
        from PIL import Image
        import image_storage_backup_set as backup_set
        import image_storage_pack_proof as packing
        import media_library_backup as backup

        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        media, store, private = root / "old-images", root / "backups", root / "private"
        media.mkdir()
        store.mkdir()
        private.mkdir(mode=0o700)
        first, second = media / "first.png", media / "second.png"
        picture = Image.new("RGB", (128, 128), (120, 135, 150))
        picture.save(first)
        picture.putpixel((64, 64), (2, 4, 6))
        picture.save(second)
        saved = [(path, path.stat().st_size) for path in (first, second)]
        snapshot = backup.backup(store, {"images": media})["snapshot"]

        database = root / "saved.sqlite"
        conn = sqlite3.connect(database)
        conn.executescript("""
            CREATE TABLE folders (id INTEGER PRIMARY KEY, path TEXT, zip_file_id INTEGER);
            CREATE TABLE files (
                id INTEGER PRIMARY KEY, parent_folder_id INTEGER, basename TEXT,
                size INTEGER, zip_file_id INTEGER
            );
            CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" BOOLEAN);
        """)
        conn.execute("INSERT INTO folders VALUES (1, ?, NULL)", (str(media),))
        for idx, (original, size) in enumerate(saved):
            conn.execute("INSERT INTO files VALUES (?, 1, ?, ?, NULL)",
                         (idx + 10, original.name, size))
            conn.execute('INSERT INTO images_files VALUES (?, ?, 1)',
                         (idx + 171, idx + 10))
        conn.commit()
        conn.close()
        config = root / "config.yml"
        config.write_text("database: saved.sqlite\n")
        backup_receipt = private / "backup-set.json"
        backup_set.create_receipt(database, config, store, snapshot, backup_receipt)

        group = {"id": "stack:17", "representative": "image:171",
                 "members": [{"id": "image:171", "path": str(first)},
                             {"id": "image:172", "path": str(second)}]}
        cid = "stack-17-delta-" + "a" * 24
        archive = private / (cid + ".pack")
        packed = packing.pack_proof(group, "delta", archive, 0, 0)
        packed.update({
            "archivePath": str(archive), "group": "stack:17",
            "mediaBackupSnapshot": snapshot, "backupVerified": True,
            "nativePackingCandidateOnly": True, "nativeServingVerified": False,
            "databaseBackupVerified": False, "archiveActivated": False,
        })
        candidate_receipt = private / (cid + ".receipt.json")
        candidate_receipt.write_text(json.dumps({
            "version": 1, "archiveSHA256": hashlib.sha256(archive.read_bytes()).hexdigest(),
            "report": packed,
        }))
        for original in (first, second):
            original.unlink()
        media.rmdir()
        proof = mapping.inspect(candidate_receipt, backup_receipt)
        self.assertTrue(proof["historicalPathMappingVerified"])
        self.assertEqual(len(proof["members"]), 2)
        self.assertFalse(proof["databaseMediaSnapshotCoherent"])
        self.assertFalse(proof["currentCataloguePathsVerified"])
        self.assertFalse(proof["canActivate"])
        self.assertFalse(proof["originalEvictionEnabled"])
        self.assertFalse(media.exists())


if __name__ == "__main__":
    unittest.main()
