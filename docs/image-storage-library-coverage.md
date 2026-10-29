# Saved image-library backup coverage (read-only)

The offline coverage audit inspects **every saved image ID** in one
previously verified SQLite backup, including image records without a
primary file, and compares its old primary file path and byte length with
an **exact root/path** entry in the verified original-media snapshot.

    python scripts/image_storage_library_coverage.py \
      --backup-set-receipt /private/recovery/backup-set.json \
      --max-examples 10

Reports totals and bounded *image-ID-only* problem examples. Problem types
include missing or ambiguous primary files, zipped sources, unscanned sizes,
unsafe historical paths, missing backup files, size mismatches, ambiguous
backup path matches, and multiple image IDs sharing one saved file.

The script validates original-media backup objects and the saved SQLite
integrity before and after reading. It does not need access to the
original media roots (which may have been deleted), and never restores,
opens or changes current originals. It uses read-only immutable SQLite
access or a private temporary SQLite extraction for zipped backups.
The audit limits joined image rows to 250,000, snapshot entries to
1,000,000, and error examples to 20.

**Scope limitation:** even full image-primary-path coverage does not
demonstrate that the database/config/media snapshots were created at
the same moment. Paths and saved sizes do not authenticate the live
catalogue, original metadata, video files, thumbnail consumers,
configuration semantics, ZIP blob contents, or a runnable recovery.
The report never claims complete library recovery, live packed serving,
rollback or permission to remove any original. All activation/eviction
flags remain false. This is an inventory gate, not a migration plan.
