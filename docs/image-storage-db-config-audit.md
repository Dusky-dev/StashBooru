# Experimental read-only database and config backup readiness

StashBooru's native database backup produces a standalone SQLite file, or a ZIP containing a root-level SQLite and `blobs/` entries when filesystem blobs are included. This tool checks **previously saved backups only** and requires an independently saved YAML configuration backup. It never opens or changes the active library.

```sh
python scripts/image_storage_db_config_audit.py \\
  --database /backups/stash-backup.sqlite \\
  --config /backups/config-backup.yml
```

Use a completed SQLite backup created through StashBooru's backup task; do **not** point it at the live WAL-mode database. The command validates canonical, regular, bounded inputs, hashes them before and after the audit, and runs SQLite's complete `integrity_check` via a read-only immutable connection. For ZIP backups, it checks member paths and size bounds and inspects a privately extracted temporary SQLite copy, which is deleted afterward. It reports configuration-file bytes and SHA-256 only, not YAML schema validity. No installation or recovery output is produced.

A passing SQLite integrity check is **not** a proof that the database and configuration are a coordinated point-in-time backup. It does not independently validate every blob object, restore SQLite/metadata/configuration, prove StashBooru schema compatibility, bind backup contents to image IDs, recover active library paths, enable packed media serving, establish crash-safe rollback or authorize original-file deletion. The JSON output explicitly keeps those claims false.

This is an **offline evidence foundation**, not a native Settings workflow. Future milestones need a server-created paired backup manifest, independent artifact and blob verification, restored scratch-clone boot tests and transactional activation/rollback proofs before any storage reclamation can be enabled.
