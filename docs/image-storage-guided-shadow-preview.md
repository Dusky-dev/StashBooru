# Guided visual comparison of saved P12/P13 candidates

The existing Settings → Tasks → Compare packed shadow image workflow now
lists the **member image IDs actually contained in a saved candidate**, with
Previous/Next controls. Users no longer have to inspect the database to find
an ID for side-by-side original, reconstructed-image and native thumbnail
comparisons.

A read-only authenticated `GET /image/storage-shadow-members` endpoint accepts
only `stackID` and the opaque, server-generated `candidateID`. It validates
the current native Visual Stack, the configured backup destination, the
private candidate receipt, the complete SHA-256 of its archived bytes, the
archive receipt's explicit disabled capability flags, and the exact
source-member hashes before listing IDs. The response returns **no disk
paths, arbitrary URLs, filesystem location or media source bytes**. It does
not create an archive or start any packing job.

The UI accepts only bounded, unique positive native image IDs from the
read-only response. Changing a saved candidate clears stale comparisons and
cancels pending member requests. The existing shadow image/download/
thumbnail endpoints still perform **independent verification on every
request**; selecting an image does not activate it.

Limitations:

- Only previously saved, independently verified 2–16-image Visual Stack
  candidates with current originals available.
- The preview is on-demand in Settings, not an interception of normal
  StashBooru image/download/thumbnail routes.
- It does not change database paths, metadata, thumbnail caches, backup
  snapshots, archived files, originals, or packed-media flags.
- This is an opt-in **visual testing preview**, not a full-library serving
  or restore test, rollback proof, or permission to evict originals.
