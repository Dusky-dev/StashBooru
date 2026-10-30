# Operator-quiesced storage backup capture — experimental offline prototype

This is the next non-destructive step after verifying independent database,
configuration and original-media backup artifacts.

The operator **must stop scanning, writing and other application activity**
before use, using an externally enforced maintenance process. This script
does **not** pause StashBooru, acquire an application-wide exclusive lock,
block other processes or prove an atomic cross-resource snapshot.

Example: from the repository root, on a suitably isolated/quiet server:

    python scripts/image_storage_quiescent_capture.py capture \
      --database /data/live/stash.sqlite \
      --config /data/live/config.yml \
      --store /separate/media-backup-store \
      --root images=/data/media/images \
      --output /separate/private-0700/new-capture \
      --acknowledge-quiesced

    python scripts/image_storage_quiescent_capture.py audit \
      --output /separate/private-0700/new-capture

Inputs must be canonical absolute non-symlink paths; output parent must
already be private 0700, and the output itself must NOT exist. Original
media roots and the backup store must be separate. The online read-only
SQLite backup API, rather than blindly copying SQLite/WAL files, creates
a first committed database image. The process copies config, builds an
immutable verified original-media snapshot, then captures a second
database/config copy. Both saved database images must be byte-identical,
and config copies must match. The media roots are then fully rescanned,
comparing every source file path, size, mtime, permissions and SHA-256 with
the newly saved media snapshot. The saved SQLite's entire image-primary
catalogue is checked against the exact backup manifest paths and sizes,
and an existing backup-set receipt is created and verified.

A final SQLite snapshot and repeated media checks reduce the risk of
changes during the capture window. A completion record appears **only
after** independent saved-artifact re-auditing and fsync. On failure, any
private scratch files and already published append-only media snapshot
are left incomplete for explicit administrator inspection; no original
file is modified, restored or removed.

Re-audit reads only the saved snapshot artifacts, so original image roots
may be missing. It detects missing/altered files and forged safety flags.

**Critical restrictions:** Matching two/three sampled database snapshots,
two config reads and full source-media scans is **observed stability**, not
proof that writers were prevented or that a transaction atomically
captured database, configuration and media. A writer could change and
revert data between observations. `coherentSnapshotVerified` and
`applicationWriteBarrierVerified` therefore remain **false**, even for
successful captures. Neither packed-media activation, live rollback nor
source-file eviction is permitted.

Also: only direct, regular primary image files mapped to saved media roots
are supported by this conservative image-coverage gate. ZIP images,
unknown sizes, shared-primary image references, unsupported database
schemas, oversized SQLite backups and incomplete media backups refuse
capture rather than silently passing. Other StashBooru media types and
binary dependencies are not yet full-restoration proof.
