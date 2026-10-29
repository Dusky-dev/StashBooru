# Verified media-backup coverage for selected Visual Stacks (offline preflight)

This opt-in, read-only check closes a gap between a generic successful media
snapshot verification and the **particular source images** proposed for
experimental packing. It adds no native packing endpoint, no database changes,
no backup mutation, and no original-file replacement.

## What it verifies

The preflight:

1. Runs the existing complete media-snapshot verification, validating every
   content-addressed backup object, including objects belonging to images
   **outside** the selected Visual Stack.
2. Reads the snapshot manifest and maps each selected image path to exactly
   one media root and manifest member, without following source symlink aliases.
3. Hashes each current image and requires exact SHA-256 and length matches
   against its previously stored, verified backup object.
4. Checks that source metadata did not change during the read and rejects
   missing originals, stale snapshots, duplicated members or invalid paths.

It accepts **one existing image-only group containing 2–16 members**.
The existing P12 stacks JSON format supplies the group IDs, representative
and source paths. Operator-supplied paths are allowed only in this **offline**
command, never from a browser or native API request.

## Usage

First create and verify a media snapshot using Settings → Tasks → Media
backup, or the established offline `scripts/media_library_backup.py` CLI.
Then check one group (read-only):

```sh
python3 scripts/image_storage_backup_preflight.py \
  --stacks /path/to/stacks.json \
  --group stack:17 \
  --store /path/to/separate-backup \
  --snapshot 20261029T120000Z-123456abcdef
```

Use your actual snapshot ID, not the illustrative value above. The command
prints a JSON report to stdout and returns nonzero on any failed condition.
A passed report includes `mediaBackupVerified: true` and the currently
matching SHA-256 for each selected Image.

This can become a prerequisite for the future native experimental pack stage,
alongside catalogue-derived sources and a separately validated output
destination. It is **not** a lasting authorization token: the native packing
job must recheck the sources, snapshot, destination and hashes immediately
before publication.

## Explicit limitations

- Media-file backup verification does **not** establish that the application
  database, configuration, derived metadata or production serving paths are
  backed up and restorable.
- This does not prove recovery after native job cancellation, native media
  serving, restoration to the StashBooru catalogue, or rollback.
- A current-source match can become stale as soon as the command finishes.
  Native code must validate again before any storage operation.
- Source files are never modified or evicted. No storage optimization setting
  is enabled. The eventual Reconstruct Library action must remain accessible
  even when experimental optimization is disabled.
