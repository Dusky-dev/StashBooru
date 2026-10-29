# Native isolated rehearsal of recovery with missing originals

The existing offline archive-first missing-original proof is exposed as a
separately opt-in native Tasks job. It is deliberately **not** native library
restoration, source-path replacement or a way to activate compressed images.

## Settings → Tasks

Choose an immutable saved P12/P13 candidate ID and explicitly acknowledge
that this creates additional recovered copies only. The browser cannot submit
an arbitrary archive, backup store, original path, output path or database
restore request.

The authenticated routes are:

- GET `image/storage-missing-original-rehearsal` — job status and latest
  in-memory report.
- POST `image/storage-missing-original-rehearsal` — a strict JSON request
  containing only `candidateID` and `acknowledged: true`.

On job execution the server derives its private candidate directory from
the configured application directory, media roots and verified backup store.
It reopens the versioned immutable candidate receipt, hashes the complete
archive, and generates a new random-named private scratch directory under the
candidate root. It passes only derived paths to the offline Python rehearsal.

The offline tool verifies the saved archive and **every object** in the
selected media backup snapshot without opening any current original source
file. It requires backup *content digest coverage*, while explicitly not
claiming the old image-ID-to-backup-path mapping is independently proven.
It then reconstructs all P12/P13 original bytes into the new private output
directory, rehashing the recovered copies and recording the extraction
completion receipt.

The native job checks all positive and negative safety flags by exact JSON
types, selected candidate ID, selected snapshot, output path, member count and
all source SHA-256 values, plus the expected completed private directory and
extraction receipt. Failed jobs do not publish a successful status.

## Safety and recovery limits

This works when individual original images are missing from their previous
native filesystem paths, provided the candidate archive and immutable media
backup remain available and verified. The media root configuration and
candidate/backup storage must still be intact.

It does **not** restore files to old paths, rewrite database records,
activate packed media, recover database/configuration, verify the historical
path mapping, or free disk space. The script cannot overwrite existing
output directories, and it never touches live originals.

A native process interruption may leave an incomplete private scratch
directory, because a true durable job journal, safe cleanup and rollback
transaction are not yet implemented. Do not use the output directly as a
production media replacement. Restore procedures must independently
revalidate all inputs and prove crash-safe original/media/database recovery
before any original eviction.

Synthetic Python CI checks missing-original scratch recovery and corrupt
backups/archives, while Go tests check the opt-in request and unambiguous
non-activation response flags. Full Go lint, Build and Settings navigation
must pass before merging. Real owner-library and browser validation are
still required.
