# Read-only restore readiness inspection for saved image archives

This milestone adds an authenticated **read-only** job under Settings → Tasks
→ Inspect media → Inspect image restore readiness.

## What it does

Choose one previously saved P12/P13 candidate by server-generated identifier.
The browser sends only a numeric Visual Stack ID and that identifier. The
server resolves all primary image paths from the native catalogue, retrieves
the configured original-media backup store and validates the candidate
directory as outside every media root and the backup destination.

The job:

1. Reopens the private immutable candidate receipt and rehashes the complete
   archive, matching its saved SHA-256 and the expected byte count.
2. Validates the candidate's identity, byte-exact mode, member hashes and
   safety flags against the current native Visual Stack.
3. Re-verifies **all objects** in the selected original-media backup snapshot
   and independently fingerprints every selected current original.
4. Compares the current source/backup SHA-256 for each member to the archive
   receipt's recorded source SHA-256.
5. Returns a bounded in-memory report with the source and archive sizes,
   point-in-time agreement, and explicit blockers preventing real activation.

The inspection makes **no** archive copies, writes no media files, changes no
database rows or native media paths, and cannot authorize an optimization.
Only the existing job queue and in-memory report state are updated.

## What a passing check does NOT mean

This is a *point-in-time consistency inspection*. A later operation must
independently revalidate all paths, backups, source hashes and archive data.

It is not a disaster-recovery guarantee: a verified media snapshot is not
a verified backup of the database or application configuration. Packed-image
serving has not been validated across the native image/thumbnail endpoints,
downloads, previews and other media consumers. Atomic replacement, runtime
crash recovery and rollback to the native library have not been proven.
No automatic activation, original-file deletion, space reclamation or
Reconstruct Library modification is implemented.

The report intentionally always returns `canActivate: false`,
`canReplaceOriginals: false`, `originalEvictionEnabled: false`,
`databaseBackupVerified: false`, `nativeServingVerified: false`, and
`rollbackVerified: false`, with the outstanding blockers spelled out.

Only image groups with their original regular files still present are
currently supported by the native Visual Stack source resolver. Missing
originals cause a fail-closed job error rather than guessing restoration
paths; recovering missing media through native catalogue integration is a
separate future milestone.

## Testing

Go regression tests assert safe source/backup/archive hash agreement,
request validation, mismatched members and snapshots, unsafe report flags,
and hard-disabled activation/eviction. Existing full Build, Go lint and
Settings navigation checks must pass before merge. Tests are synthetic and
do not claim production library or mounted-browser validation.
