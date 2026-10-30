# Isolated complete regular-file catalogue path rebase (experimental)

This is an offline, **additional** SQLite scratch copy that extends the
previous image-primary path rebase (#286) to **all saved regular-file records**
in StashBooru's `files` and `folders` tables, including other media types.

The prerequisites must already have succeeded:

1. Verified original-media/database/config backup-set receipt.
2. Verified whole-snapshot scratch recovery (#285).
3. Verified image-primary path rebase (#286).

Example:

    python scripts/image_storage_disposable_all_file_rebase.py rebase \
      --backup-set-receipt /private/backup-set.json \
      --recovered-output /private/verified-full-restore \
      --image-rebase-output /private/verified-image-rebase \
      --output /private/new-all-files-rebase

    python scripts/image_storage_disposable_all_file_rebase.py audit \
      --backup-set-receipt /private/backup-set.json \
      --recovered-output /private/verified-full-restore \
      --image-rebase-output /private/verified-image-rebase \
      --output /private/new-all-files-rebase

The tool matches **each saved regular file** by its historical full pathname
and size to an unambiguous saved original-media root and snapshot entry. It
rejects ZIP-backed files/folders, unsupported or orphaned folders, duplicate
file path references, missing files, inconsistent saved primary mappings,
nonregular recovered media, oversized catalogues, and outputs nested within
any trusted recovery or live-media input. No original media are opened.

On success, it copies the already verified image-rebased SQLite to a
**new, private** directory. It updates only the copied `folders.path`
columns for all saved regular files, validates row counts, verifies SQLite
integrity, rechecks the saved input evidence, and writes a final fsynced
completion marker. The independent audit rejects marker tampering, unknown
files, symlinks or altered paths.

**This does not prove an isolated StashBooru server can boot.** Other
model-specific absolute paths may exist outside `files`/`folders`.
Configuration references, ZIP blob payloads, plugin/script paths, generated
media, schema migration behavior, credentials, logging and startup writes
are not yet isolated or validated. No server is started. The audit explicitly
reports `restoredServerBootVerified=false`, `fullLibraryRecovered=false`,
`canActivate=false` and `originalEvictionEnabled=false`.

Next: inspect all remaining potentially dangerous persisted paths, construct
an isolated configuration and launch harness without production access,
then actually test startup and media consumers on a disposable instance.
