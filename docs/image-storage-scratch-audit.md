# Read-only restart audit of private P12/P13 scratch recovery

The native missing-original rehearsal (PR #271) recovers verified candidate
bytes into a **new private scratch directory**, not into StashBooru's media
library. An interrupted or abruptly terminated process may leave partially
written files there. The presence of a directory, individual recovered files,
or even the existence of an extraction marker is **not** sufficient evidence
to enable any kind of live restore.

This milestone adds a separate, **read-only** checker for an existing private
rehearsal output. It does not create, repair, delete, rename, or install files.

Example, using only trusted server-generated absolute paths:

    python scripts/image_storage_scratch_audit.py \
      --receipt /private/candidates/stack-17-delta-aaaaaaaaaaaaaaaaaaaaaaaa.receipt.json \
      --output /private/candidates/stack-17-orphan-rehearsal-bbbbbbbbbbbbbbbbbbbbbbbb

The checker rejects symlinks, noncanonical ancestors, unsafe paths/names,
foreign stack identifiers, excessive members, unexpected files, invalid
candidate receipt structure, duplicate identities, mismatched extraction
members and SHA-256 failures. It reports:

- no_output: Scratch directory has not been created. No verified result.
- incomplete: Directory exists, but extraction.json is missing. The scratch
  output is **not** certified even if some files have been written.
- complete_verified: The marker is complete and byte-exact, the filenames
  and image IDs match the immutable candidate receipt, and every scratch
  member matches its original SHA-256 evidence.

Unsafe or corrupt inputs cause a **nonzero error**, never successful
certification. The audit is point-in-time and does not revalidate the
candidate archive, backup store, source-to-backup-path mapping, catalogue,
database/configuration snapshot or live routing. Every result explicitly
disables native activation, live rollback claims and original eviction.

This is **not** transactional live rollback or Reconstruct Library. Next:
persistent job lifecycle recording across server restarts, database and
configuration backup identity checks, and independently verified end-to-end
live-media rollback before considering automatic serving or file deletion.
