"""A disposable SQLite copy may point to saved scratch media, never live paths."""
from pathlib import Path
import json
import sqlite3
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import image_storage_disposable_path_rebase as rebase
import image_storage_disposable_snapshot_restore as full
import image_storage_backup_set as backup_set
import media_library_backup as media


class DisposablePathRebaseTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        self.media = root / "originals"
        self.store = root / "backup"
        self.private = root / "private"
        self.media.mkdir()
        self.store.mkdir()
        self.private.mkdir(mode=0o700)
        for name in ("first.png", "second.png", "unused.txt"):
            (self.media / name).write_bytes(("synthetic media " + name).encode())
        self.snapshot = media.backup(self.store, {"images": self.media})["snapshot"]
        self.db = root / "saved.sqlite"
        conn = sqlite3.connect(self.db)
        # Exercise real StashBooru SQLite: its journal_mode is WAL, not DELETE.
        self.assertEqual(conn.execute("PRAGMA journal_mode=WAL").fetchone(), ("wal",))
        conn.executescript("""
            CREATE TABLE images (id INTEGER PRIMARY KEY);
            CREATE TABLE folders (id INTEGER PRIMARY KEY, path TEXT, zip_file_id INTEGER);
            CREATE TABLE files (id INTEGER PRIMARY KEY, basename TEXT, size INTEGER,
                                parent_folder_id INTEGER, zip_file_id INTEGER);
            CREATE TABLE images_files (image_id INTEGER, file_id INTEGER, "primary" INTEGER);
        """)
        conn.execute("INSERT INTO folders VALUES (1, ?, NULL)", (str(self.media),))
        conn.executemany("INSERT INTO images VALUES (?)", [(101,), (102,)])
        for pos, name in enumerate(("first.png", "second.png")):
            conn.execute("INSERT INTO files VALUES (?, ?, ?, 1, NULL)",
                         (pos + 11, name, (self.media / name).stat().st_size))
            conn.execute('INSERT INTO images_files VALUES (?, ?, 1)', (pos + 101, pos + 11))
        conn.commit()
        conn.close()
        self.config = root / "saved.yml"
        self.config.write_text("database: saved.sqlite\n")
        self.receipt = self.private / "backup-set.json"
        backup_set.create_receipt(self.db, self.config, self.store,
                                  self.snapshot, self.receipt)
        self.recovered = self.private / "recovered"
        self.rebased = self.private / "rebased"
        full.rehearse(self.receipt, self.recovered)

    def do_rebase(self):
        return rebase.rebase(self.receipt, self.recovered, self.rebased)

    def do_audit(self):
        return rebase.audit(self.receipt, self.recovered, self.rebased)

    def test_wal_snapshot_is_rebased_into_a_self_contained_sqlite_file(self):
        self.assertEqual(sqlite3.connect(self.db).execute(
            "PRAGMA journal_mode").fetchone(), ("wal",))
        self.do_rebase()
        self.assertEqual(self.do_audit()["status"], "complete_verified")
        clone = self.rebased / "image-paths.sqlite"
        with sqlite3.connect(clone) as connection:
            self.assertEqual(connection.execute("PRAGMA journal_mode").fetchone(), ("delete",))
            self.assertEqual(connection.execute("SELECT path FROM folders WHERE id=1").fetchone(),
                             (str(self.recovered / "media" / "images"),))
        self.assertFalse((self.rebased / "image-paths.sqlite-wal").exists())
        self.assertFalse((self.rebased / "image-paths.sqlite-shm").exists())

    def test_disposable_sqlite_points_to_verified_scratch_not_live_roots(self):
        proof = self.do_rebase()
        self.assertTrue(proof["imagePrimaryFoldersRebased"])
        self.assertTrue(proof["sourceSavedSQLitePreserved"])
        for key in rebase.FALSE_FLAGS:
            self.assertFalse(proof[key])
        self.assertEqual(self.do_audit()["status"], "complete_verified")
        with sqlite3.connect(self.rebased / "image-paths.sqlite") as db:
            new_path = db.execute("SELECT path FROM folders WHERE id=1").fetchone()[0]
        self.assertEqual(new_path, str(self.recovered / "media" / "images"))
        with sqlite3.connect(self.recovered / "database" / "recovered.sqlite") as db:
            old_path = db.execute("SELECT path FROM folders WHERE id=1").fetchone()[0]
        self.assertEqual(old_path, str(self.media))
        with sqlite3.connect(self.db) as db:
            live_path = db.execute("SELECT path FROM folders WHERE id=1").fetchone()[0]
        self.assertEqual(live_path, str(self.media))
        self.assertEqual((self.media / "first.png").read_bytes(),
                         (self.recovered / "media" / "images" / "first.png").read_bytes())

    def test_success_after_originals_deleted(self):
        for file in self.media.iterdir():
            file.unlink()
        self.media.rmdir()
        self.assertTrue(self.do_rebase()["imagePrimaryFoldersRebased"])
        self.assertTrue(self.do_audit()["verified"])
        self.assertFalse(self.media.exists())

    def test_no_overwrite_or_rebase_into_protected_locations(self):
        self.do_rebase()
        with self.assertRaises(ValueError):
            self.do_rebase()
        with self.assertRaises(ValueError):
            rebase.rebase(self.receipt, self.recovered, self.media / "scratch")
        with self.assertRaises(ValueError):
            rebase.rebase(self.receipt, self.recovered, self.recovered / "nested")

    def test_tampered_rebased_database_or_marker_fails_closed(self):
        self.do_rebase()
        with sqlite3.connect(self.rebased / "image-paths.sqlite") as db:
            db.execute("UPDATE folders SET path='/different' WHERE id=1")
        with self.assertRaises(ValueError):
            self.do_audit()
        with sqlite3.connect(self.rebased / "image-paths.sqlite") as db:
            db.execute("UPDATE folders SET path=? WHERE id=1",
                       (str(self.recovered / "media" / "images"),))
        # Reverting every altered byte may legitimately restore the original
        # verified state. Any further mutation must still fail independent audit.
        with sqlite3.connect(self.rebased / "image-paths.sqlite") as db:
            db.execute("UPDATE folders SET path='/changed-again' WHERE id=1")
        with self.assertRaises(ValueError):
            self.do_audit()

    def test_forged_activation_and_unknown_scratch_file_refused(self):
        self.do_rebase()
        marker = self.rebased / rebase.MARKER
        record = json.loads(marker.read_text())
        record["canActivate"] = True
        marker.write_text(json.dumps(record))
        with self.assertRaises(ValueError):
            self.do_audit()
        record["canActivate"] = False
        marker.write_text(json.dumps(record))
        self.assertTrue(self.do_audit()["verified"])
        (self.rebased / "rogue").write_text("unexpected")
        with self.assertRaises(ValueError):
            self.do_audit()

    def test_source_media_snapshot_corruption_fails_audit(self):
        self.do_rebase()
        item = media._load_manifest(self.store, self.snapshot)["entries"][0]
        media._object_path(self.store, item["sha256"]).write_bytes(b"bad")
        with self.assertRaises((ValueError, media.BackupError)):
            self.do_audit()


if __name__ == "__main__":
    unittest.main()
