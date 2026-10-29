# Bounded mapped P12/P13 multi-image scratch recovery

This experimental **offline-only** proof restores one already-verified P12/P13
candidate's 2–16 images plus a saved SQLite database and configuration
into a **new** private scratch directory. It requires a verified historical
image-ID-to-exact-backup-path mapping and a verified backup-set receipt.

Example:

    python scripts/image_storage_bounded_recovery.py rehearse \
      --candidate-receipt /private/candidates/stack-17-delta-EXAMPLE.receipt.json \
      --backup-set-receipt /private/backup-set.json \
      --output /private/new-recovery

    python scripts/image_storage_bounded_recovery.py audit \
      --candidate-receipt /private/candidates/stack-17-delta-EXAMPLE.receipt.json \
      --backup-set-receipt /private/backup-set.json \
      --output /private/new-recovery

The output parent must already exist and have private 0700 permissions;
output must not yet exist. The verifier independently checks candidate
archive contents, all original-media backup objects, the saved SQLite
primary-file paths, original SHA-256 values and original sizes. Each selected
image is bounded to 64 MiB and the full selected set to 256 MiB. Originals
may already be missing: only verified backup objects are read.

Recovery writes database, configuration, and selected media into isolated
scratch. No output paths can overlap configured originals or backed-up source
files. A completion marker is published last, after file hashes, SQLite
integrity and reinspection of saved backup evidence. Interrupted attempts
remain unmarked and a later read-only audit reports incomplete rather
than claiming recovery. The audit rejects unexpected files, symlinks,
changed image bytes, changed receipts and forged activation claims.

This is NOT a full-library recovery, coordinated cross-system snapshot,
configuration-semantics proof, boot of a restored StashBooru instance,
production packed-media route, live rollback or original-file eviction.
The database/config/media backup artifacts may represent different times.
No originals or active catalogue entries are modified or removed.
