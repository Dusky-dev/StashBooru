# Historical image path provenance for saved P12/P13 candidates

An **offline read-only inspection**, for a previously saved P12/P13 candidate
receipt and independently verified database/config/media **backup-set receipt**:

```sh
python scripts/image_storage_historical_paths.py \
  --candidate-receipt /private/image-storage-candidates/stack-1-delta-<id>.receipt.json \
  --backup-set-receipt /private/backup-set.json
```

The inspector rehashes the candidate archive, checks its byte-exact entries,
verifies all media snapshot backup objects, verifies the SQLite database backup
(including SQLite-in-ZIP), and queries the saved StashBooru database's
`images_files`, `files`, and `folders` tables for each image ID's unique
non-ZIP primary file. It **compares the exact old primary pathname**, expressed
relative to a saved original-media root, against the snapshot manifest entry
at the same root and path, and compares the archived source SHA-256 against
the saved media object's digest. It rejects duplicate/ambiguous roots,
missing/renamed historical paths, different bytes, unsupported schema,
unscanned sizes when a size cannot be proved (marked individually), and
zip-contained historical sources.

The script does not open the current media files or require the old media
directories to still exist. An optional private temporary SQLite extraction
may occur only for ZIP-backup inspection; it is removed afterward. No
catalogue paths are rewritten and no backups are modified.

**Critical limitation:** Matching a saved database's image IDs, old paths and
a media snapshot's SHA-256 is **historical provenance**, NOT proof those
database and media backups were captured atomically or still correspond to
the live catalogue. Other images outside the selected 2–16 candidate members
are not checked. It does not restore the full library, validate native image
serving, implement rollback, activate packed serving or permit original-file
eviction. The result explicitly sets all those capabilities to false.

Run only with your own saved backup artifacts; the resulting JSON includes
historical relative paths and must not be publicly posted.
