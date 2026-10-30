# Disposable complete-media snapshot reconstruction (offline)

This experimental tool restores **every file in one saved original-media
snapshot**, together with a saved SQLite database copy and config YAML,
into a new private scratch directory. This is broader than the prior
2–16-image P12/P13 candidate recovery: it includes other media files in
the snapshot that are not attached to a candidate.

Prerequisite: an immutable verified backup-set receipt; see
image-storage-backup-set.md. The saved SQLite must pass the full
primary-image-path coverage audit. Inputs do **not** require access to
live original media or an active StashBooru database.

Example:

    python scripts/image_storage_disposable_snapshot_restore.py rehearse \
      --backup-set-receipt /private/saved-backup-set.json \
      --output /private-0700/new-disposable-copy \
      --max-files 20000 --max-bytes 107374182400

    python scripts/image_storage_disposable_snapshot_restore.py audit \
      --backup-set-receipt /private/saved-backup-set.json \
      --output /private-0700/new-disposable-copy

The output parent must already exist and be private, and may not overlap
any original-media root, backup store or source backup artifact.
The destination must not exist; repeated attempts never overwrite
previous output. Defaults are deliberately conservative (128 files /
512 MiB). Operators may raise limits explicitly, up to one million
files and 2 TiB. A conservative disk-free preflight is mandatory.

The script verifies the complete backup set and exact saved
image-ID/primary-file-path coverage, copies SQLite/config into scratch,
reconstructs **all** saved media snapshot members from backed-up
SHA-256 objects, and rehashes each. Completion is written last.
Interrupted or partial restores remain unmarked and later audit
reports `incomplete`. Independent audit checks every expected file,
rejects unexpected files/symlinks and revalidates all saved backup
evidence. The original source media may have been deleted without
affecting this *offline* recovery test.

**Important limitation:** `allSavedMediaSnapshotMembersRestored=true`
is not equivalent to `fullLibraryRecovered=true`. This tool does not
rebase SQLite catalogue paths to scratch, restore original database
ZIP blob attachments, configure or boot an isolated StashBooru
instance, prove that the DB/config/media snapshots represent the
same moment, or validate normal browser/media consumer functionality.
It never writes to, reads from or removes live original media paths.
Packed serving, activation, original deletion and rollback remain
strictly disabled.

The next milestone is a trusted catalogue path-rebasing plan and
isolated StashBooru startup test, without any production instance
modification.
