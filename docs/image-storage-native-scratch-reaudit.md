# Native read-only re-audit of private packed-image scratch recovery

The private P12/P13 missing-original rehearsal can leave scratch files after a
restarted process. A persisted journal (PR #273) is a record of a previous
attempt, **not** fresh evidence that those bytes remain valid.

This milestone makes the isolated restart audit script from PR #272 available
from **Settings → Tasks** through an authenticated, explicitly invoked native
request:

    POST /image/storage-missing-original-audit
    {"candidateID":"<server-generated P12/P13 candidate ID>"}

The browser cannot submit source paths, scratch output paths or filenames.
The server reads the latest validated private journal, reopens the saved
candidate receipt and independently rehashes the candidate archive before
running `image_storage_scratch_audit.py` with strictly server-derived paths.

The script compares saved source IDs and SHA-256 evidence with every recovered
scratch file and its completion marker, returning exactly one of:

- `no_output`: no scratch output exists.
- `incomplete`: partial output without a completion marker.
- `complete_verified`: all saved members' bytes match their source digest
  at the time of inspection.

Malformed/symlinked output and dishonest capability claims fail closed.
The call is bounded by a 90-second subprocess deadline, reads files only and
does not modify StashBooru's database, originals, media serving paths, backed
up objects or scratch files.

**This is not Reconstruct Library, active packed-image serving or crash-safe
rollback.** Successful inspection does not reverify the full backup store,
prove historical image-to-path mapping, restore the database/configuration,
authorize original-file eviction, or reclaim disk space. The journal's
`scratch_complete` state never automatically becomes a current verified
result. Every audit is opt-in, independent and point-in-time.

Go tests validate narrow request parsing, strict JSON safety flags,
incomplete states, member counts and output identity.
