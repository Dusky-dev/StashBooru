# Media-file backup foundation (experimental storage prerequisite)

StashBooru's existing **Settings → Tasks → Backup** backs up the database and,
optionally, filesystem blobs. It does **not** include original media files. P12
image-delta and P13 gallery block-sharing remain offline, non-destructive
prototypes. Never activate source eviction using only database backups.

This first media-file backup package provides a **manual, opt-in, offline**
file-level snapshot/verify/restore tool using only Python 3's standard library.
It is a safety building block, not a live StashBooru Settings button, automatic
backup scheduler, full application backup, or source-eviction authorization.

## Native Settings → Tasks integration

The follow-up native task appears in **Settings → Tasks → Backup & maintenance**
under **Original media backup**. The server must have Python 3 and the
`scripts/media_library_backup.py` file installed alongside the app, or use
`STASH_MEDIA_BACKUP_SCRIPT` to point to that exact local script. A missing
script displays an unavailable warning; it never attempts a backup.

1. Create a directory for snapshots on the **server**, preferably on a
   different physical drive from the media library.
2. Enter its absolute path and choose **Save backup destination**. This saves
   a private `media-library-backup.json` config in the StashBooru config
   directory. No files are copied just by saving the path.
3. Choose **Back up media files**, then monitor the normal Tasks job queue.
   The server uses current configured media roots, not arbitrary browser paths.
4. Refresh the snapshot list and choose **Verify** to fully hash every
   backup blob, or **Restore to directory…** to reconstruct snapshot files
   to a fresh, separate server directory without touching active source files.

Backup, verification and recovery use cancellable native jobs. The task shows
the job ID and requires the user to inspect its completion/error in the queue.
Restore requires explicit acknowledgement. If the server is missing the Python
script, or a destination overlaps a media root, the operation fails closed.
The server disallows running two media backup operations concurrently.

This is **not** an automatically scheduled backup and does not create a
`Reconstruct Library` operation for future delta-optimized media. Restore
copies original media into a separate directory; the user must coordinate
database/configuration recovery and changes to active filesystem roots.

## Create a snapshot

Choose a destination **outside all backed-up roots**, preferably another
physical drive with sufficient free space. Create it first:

```sh
mkdir -p /mnt/separate-drive/stashbooru-media-backups
python3 scripts/media_library_backup.py backup \
  --store /mnt/separate-drive/stashbooru-media-backups \
  --root images=/path/to/your/images \
  --root videos=/path/to/your/videos
```

Save the `snapshot` identifier printed as JSON. The store contains immutable
content-addressed objects and versioned per-snapshot manifests. Repeated
snapshots reuse identical content, but the tool still reads and hashes every
source on each invocation. It does **not** delete files or reclaim objects.
Include any storage packs in a separate named root **if they already exist**;
none of the prototype P12/P13 packs are installed automatically.

## Verify, recover and restore

```sh
python3 scripts/media_library_backup.py verify \
  --store /mnt/separate-drive/stashbooru-media-backups \
  --snapshot 20261028T160000Z-a1b2c3d4e5f6

# Reconstruct ALL snapshot files in a SEPARATE output directory:
python3 scripts/media_library_backup.py restore \
  --store /mnt/separate-drive/stashbooru-media-backups \
  --snapshot 20261028T160000Z-a1b2c3d4e5f6 \
  --output /mnt/recovery/media

# Or one file; directories are restored under <output>/<root-label>:
python3 scripts/media_library_backup.py restore \
  --store /mnt/separate-drive/stashbooru-media-backups \
  --snapshot 20261028T160000Z-a1b2c3d4e5f6 \
  --output /mnt/recovery/one-file \
  --member images:subfolder/example.png
```

Replace the example snapshot ID with the actual ID reported by backup.
Verification hashes **every** referenced object. Restore checks all selected
objects before writing, then atomically publishes each output file without
overwriting preexisting destination files. If interrupted, a restore may have
completed some files; rerun into a fresh directory. It does not write into your
active library, update database paths, or restore automatically on app startup.

## Safety and operational limits

- Originals and media database entries are never changed by this script.
- No symlinks, FIFO, device files or path traversal; source files changing while
  copied are rejected. Do not run backups against actively changing sources
  when a consistent point-in-time snapshot is required.
- Atomic immutable object publication and manifest publication protect prior
  successful snapshots when a later backup is interrupted. Newly written
  unreferenced blobs may remain after an interrupted backup; there is no garbage
  collection yet.
- Snapshots are not encryption or a defense against a malicious actor who can
  modify both manifests and objects; keep an independent offsite/offline copy.
  A backup on another directory on the same disk is not a substitute for one on
  another device.
- This version does not back up or restore the **database, configuration,
  generated files, metadata files, conversion/upscaling restore caches** or a
  coherent running-app snapshot. Back those up separately using appropriate
  methods and coordinate their restore points. Filesystem root labels are
  operator-specified and do not alter native media IDs.
- No built-in scheduling, retention, quotas, UI, media-specific filtering or
  automatic recovery exists yet. Verify a representative restore on another
  device before trusting backups for original-file removal.
- Backup store and media roots cannot contain one another; media root scans
  reject symlinks rather than silently omitting content. The script never
  deletes backup objects even if sources or old manifests are deleted.

## Next integration milestones

1. Native **Settings → Tasks → Media backup** and a dedicated destination
   setting; safe scheduling, backup status and progress/cancellation.
2. Consistent snapshots coordinated with database/configuration/caches, review
   and selective restoration in the app, and platform-specific path checks.
3. Non-destructive P12/P13 estimates, verified packs, live serving and scanning
   from validated materializations.
4. Only after backup/restore and crash recovery tests pass: an explicitly gated
   experimental image-storage opt-in, separate optimization task, and
   `Reconstruct Library` recovery path that remains available when disabled.

No original-file eviction or experimental delta activation is enabled by this PR.
