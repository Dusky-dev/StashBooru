# Disposable image path rebasing of a restored SQLite copy

This offline experimental step prepares **a separate SQLite working copy**
for future isolated StashBooru startup tests. Requires a successful,
independently audited complete-media scratch restoration from
image-storage-disposable-snapshot-restore.md.

Example:

    python scripts/image_storage_disposable_path_rebase.py rebase \
      --backup-set-receipt /private/backup-set.json \
      --recovered-output /private/verified-scratch-restoration \
      --output /private/new-sqlite-rebase

    python scripts/image_storage_disposable_path_rebase.py audit \
      --backup-set-receipt /private/backup-set.json \
      --recovered-output /private/verified-scratch-restoration \
      --output /private/new-sqlite-rebase

The script validates the original receipt and entire verified scratch
recovery, resolves every saved image's primary file to an exact
snapshot-root/path entry, then **copies** the recovered SQLite to a new
private output directory. Only in that copy, it rewrites the relevant
`folders.path` entries so the native `images_files` → `files` →
`folders` relationships refer to restored files at
`verified-scratch-restoration/media/LABEL/...`. The verified source
SQLite and all production original-media paths are never modified.

It independently verifies the rewritten database, all image folder
targets, source recovery, proof hashes, and non-activation safety flags.
No overwrite is allowed; incomplete runs have no success marker.
After a successful run, `audit` independently checks current saved
artifacts and the rewritten SQLite.

**Do not start a production StashBooru instance using this database.**
This does not rewrite other media paths, external configuration,
generated-file directories or ZIP database blobs, install a separate
server, secure a read-only media mount, prove application startup,
or establish consistent cross-system backup timing. It may not be
compatible with every future schema and intentionally fails closed.

The next step is a disposable StashBooru startup harness with
independent configuration, isolated media and generated-file
directories, and explicit native browser/media compatibility tests.
All activation, rollback, optimization and deletion flags remain false.
