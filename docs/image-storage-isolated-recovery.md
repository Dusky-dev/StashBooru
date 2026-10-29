# Isolated database, configuration and sample-media recovery rehearsal

This offline, explicitly invoked proof consumes an existing verified **backup-set
receipt** from PR #277. It restores one selected saved media member, a copy of
the backed-up SQLite database, and the saved config YAML into a new private
scratch directory. No operation runs against the live StashBooru database or
media paths.

```sh
python scripts/image_storage_isolated_recovery.py rehearse \
  --receipt /private/backup-set.json \
  --output /private/fresh-recovery \
  --member 'images:example file.png'

python scripts/image_storage_isolated_recovery.py audit \
  --receipt /private/backup-set.json \
  --output /private/fresh-recovery
```

The output parent must already exist and be `0700` on Unix. The target must
not exist. Outputs inside any backup store or known original media path are
refused. It validates the full source receipt and media objects, then creates
private scratch files with SHA-256 checking and SQLite `integrity_check`.
The completion marker is written last; a power outage leaves output
unmarked and a subsequent audit returns `incomplete`, not success.
A later audit separately verifies the exact source receipt and all recovered
bytes, rejecting added files, symlinks, or forged activation flags.

It restores **one chosen media file**, not the full media library; it does not
validate the YAML's meaning, restore a StashBooru application process,
independently authenticate filesystem blobs within ZIP database backups,
prove coordinated database/media/config snapshot timing, map image IDs to
current catalogue paths, update any live routes, or prove transactional
rollback. No original-file eviction or disk-space reclamation is possible.
Scratch failures intentionally leave the private incomplete output in place
for inspection. Do not mistake the completed marker for a rollback proof.
