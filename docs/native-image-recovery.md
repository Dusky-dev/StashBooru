# Native isolated image candidate recovery

A strictly opt-in native job can verify and reconstruct the last successfully
published experimental P12/P13 candidate **in the current server session**.
It is a recovery **copy**, not native library rollback or media activation.

## What the browser can request

Settings → Tasks → Inspect media → Reconstruct candidate into a separate
folder requires acknowledging extra disk usage. The browser supplies a numeric
Visual Stack ID and acknowledgement only: it cannot specify an archive,
backup store, source path, output directory, file name or cleanup action.
The server reads its last verified in-memory candidate receipt and rejects
a stack mismatch, stale archive, unexpected path or safety flag.

The job validates the candidate path as a regular non-symlink file within
the native private candidate root, re-checks the candidate proof report
against the current selected Visual Stack and an existing archive file,
and passes the receipt to the offline isolated P12/P13 reconstruction reader.
A private temporary evidence file is removed on completion. All outputs
are new random-named directories inside the protected candidate root.

The reader independently verifies the complete P12/P13 byte-exact archive
and source SHA-256, extracts into an exclusively created directory, and then
reads every restored image and compares it to the recorded original SHA-256.
The resulting completion receipt and all member digests are checked again by
the native job. A failed post-extraction proof removes **only the new output
directory**.

## Scope and safety limitations

- No original media is changed, deactivated, replaced, or deleted.
- The reconstructed directory is disconnected from StashBooru's database and
  any live image-serving routes.
- Existing media files remain authoritative; the proof copies consume disk.
- Only the most recent in-memory candidate is available. A server restart
  loses the candidate selection and successful recovery report; files persist.
- No native rollback into the catalogue, persistent candidate registry,
  database/config recovery, native resumable job or final Reconstruct Library
  workflow is implemented.
- Real owner-media, crash recovery, cross-device filesystem behavior and
  concurrent filesystem manipulation remain unverified. Original eviction is
  strictly disabled.

The dedicated offline P12/P13 reconstruction CI from #261, Go API request,
private-path and report validation tests, Go lint and production frontend
checks are required before merging this increment.
