# Isolated candidate reconstruction proof (offline only)

This increment proves that an experimental P12 delta or P13 shared-tiles
candidate archive can be **independently verified and fully reconstructed**
into a **new, separate** private directory. It is not an in-place library
restore, native media serving, or a rollback/eviction mechanism.

The trusted server-side job is intended to supply a private JSON evidence
file with `archivePath`, `method`, `group`,
`mediaBackupSnapshot`, and the `sourceSHA256` map from a previously
verified candidate receipt. This proof CLI itself accepts filesystem paths,
so **do not expose it as a public browser API**.

Example for offline operator-controlled test only:

```sh
python3 scripts/image_storage_candidate_reconstruct.py \
  --evidence /private/candidate-evidence.json \
  --output /private/new-reconstruction-directory
```

Before creating the output directory, the reader verifies archive integrity,
byte-exact fidelity, the precise image identity set, and all reconstructed
SHA-256 digests against retained original-media evidence. Extraction must
create a new directory and a completion receipt. The proof re-reads each
extracted file and verifies the restored source SHA-256 and absence of
unexpected extra files. A failed post-extraction check deletes only the new
recovery directory, **never the archive or original images**.

The output directory is intentionally not activated or linked to StashBooru's
catalogue. A tampered archive, inconsistent member ID/hash, existing output,
or symlinked archive file fails closed. The synthetic test suite covers P12,
P13, corruption, mismatched evidence, missing sources, symlink paths and
no-overwrite behavior.

Remaining work: native catalogue-controlled recovery job, validated server
path isolation, crash recovery, persistent proof receipts, cancellation,
database/config restoration, rollback into native media infrastructure and
real-owner-media verification. No original-file deletion or optimization
activation is authorized by these offline checks.
