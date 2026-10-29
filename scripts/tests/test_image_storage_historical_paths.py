"""Historical SQLite image ID -> exact media backup path provenance tests."""
from pathlib import Path
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


if __name__ == "__main__":
    unittest.main()
