# Isolated recovery rehearsal when original media files are unavailable

This is an **offline, opt-in, non-destructive rehearsal** of recovering a
verified P12/P13 candidate without consulting source files in the live media
library. It does not use the StashBooru database and does not install
reconstructed files into its media catalogue.

The tool accepts only:

- A server-created immutable `stack-...receipt.json` candidate receipt.
- The configured immutable original-media backup store.
- A **new** output directory which must be a direct child of the candidate
  receipt's private directory, outside all configured original-media roots
  and outside the backup store.

Example (replace with actual server-generated paths):

```bash
python scripts/image_storage_missing_original_rehearsal.py \
  --receipt /private/candidates/stack-17-delta-0123456789abcdef01234567.receipt.json \
  --store /media-backups \
  --output /private/candidates/test-recovery-1
```

The script checks the full versioned receipt, exact candidate ID/path,
mandatory non-activation flags, every original image SHA-256, the complete
archive SHA-256 and expected archive size. It then re-verifies **all objects**
of the selected backup snapshot without opening any original source files.

It also establishes that the backup snapshot contains at least the requisite
number of objects by original-content digest (accounting for duplicate
hashes). Without restoring the live database, this is **content coverage
only**, not proof of the historical image-ID-to-backup-file-path mapping.

Only after these checks pass does the preexisting isolated P12/P13
reconstructor verify the complete archive and materialize exact source
bytes in the designated fresh private output directory. Its extracted
files are independently rehashed, with a completion receipt in that
scratch directory. Existing output directories, symlinks, changed
archives, corrupt backup objects and output paths inside media or backup
roots are rejected. The operation never restores originals into live
paths and cannot reclaim storage.

## Limits and remaining work

- If the media backup or archive is lost, this proof fails rather than
  manufacturing images.
- If the database is lost, the user still needs a separate database and
  configuration restore to recover original image IDs and file paths.
- If an operator chooses a scratch directory with little free space,
  the isolated extract can fail; low-space and abrupt-kill recovery are not
  yet exhaustively proven on real filesystems.
- This offline CLI is not a native recovery job, safe activation procedure,
  or guarantee of power-loss atomicity. It does not handle production
  backups, permissions, concurrent media writes, or Windows path behavior.
- It does not enable packed-image rendering after deleting originals and
  never authorizes source eviction or automatic restoration.
- Synthetic tests **only delete temporary test image files**, then
  verify both P12/P13 methods reconstruct identical bytes into scratch,
  while the original media directory stays empty. They also assert
  receipt tampering, archive corruption, backup corruption, symlinked
  receipts and invalid output locations fail closed.

The next step is native integration with explicit private recovery records,
durable interrupted-job state, and independently tested database/config
recovery before any active-library restoration or pack activation.
