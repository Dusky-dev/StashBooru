# Private verified backup-set receipt (offline)

Use `scripts/image_storage_backup_set.py` only with **previously completed**
database, configuration, and original-media backups. It never copies or
changes their original files. The supplied private output directory must
already exist, be mode `0700` on Unix, and be separate from the media and
backup-store roots.

```sh
python scripts/image_storage_backup_set.py create \
  --database /saved/stash-backup.sqlite \
  --config /saved/config-backup.yml \
  --store /media-backups \
  --snapshot 20261029T102030Z-012345abcdef \
  --output /private/recovery/backup-set.receipt.json

python scripts/image_storage_backup_set.py verify \
  --receipt /private/recovery/backup-set.receipt.json
```

Creation uses the read-only SQLite/config verifier, verifies every object in
the selected media snapshot, and stores file SHA-256 checksums, expected sizes,
SQLite integrity metadata, media object count, and the media snapshot
manifest fingerprint in a private versioned receipt. It refuses to overwrite
existing files and syncs the new receipt and directory. Verification repeats
those checks against each actual backup, failing on changes, corrupt database,
missing media objects or unsafe paths.

**This receipt only binds the exact backup artifacts inspected at the time.**
It is not a cryptographic signature or an atomic cross-system snapshot:
an attacker with write access to the private directory and backups can alter
both. The database, configuration and media snapshots might have been created
at different times. The receipt does not confirm image-ID-to-backup-path
mapping, authenticate all ZIP blobs, validate configuration semantics, restore
a database, run StashBooru on a scratch clone, or prove transactional
rollback. It cannot enable packed serving or delete originals. Those
capability flags stay explicitly false.

Next: create a *native, coordinated* database/config/media snapshot with
origin and timing evidence, then test isolated restoration before any
live-file switching or eviction is considered.
