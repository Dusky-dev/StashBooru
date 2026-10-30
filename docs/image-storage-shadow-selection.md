# Durable, reversible P12/P13 shadow trial selection (not activation)

This experimental native endpoint records a **non-activating intent** to
investigate one verified packed candidate. It is the first step towards a
controlled future activation state machine, not the state machine itself.

The server keeps one private `shadow-serving-selection.json` in its existing
`image-storage-candidates` directory, outside media roots and backups.
Only explicit `prepare` and `disarm` actions can change this intent.

- `GET /image/storage-shadow-selection` reads the saved intent and, when
  prepared, freshly checks the saved candidate receipt, full archive SHA-256,
  current Visual Stack mapping, source-file presence and proof invariants.
  A stale or corrupt record fails closed. The `EvidenceVerifiedNow` field
  means *metadata/provenance checks only*, not byte-exact reconstruction.
- `POST /image/storage-shadow-selection` with
  `{"Action":"prepare","CandidateID":"stack-17-delta-0123456789abcdef01234567","StackID":17,"Acknowledged":true}`
  requires a valid saved candidate and its current Visual Stack.
  It persists the specific receipt/archive fingerprints and never changes
  the active image source.
- `POST` with `{"Action":"disarm","Acknowledged":true}`
  keeps the prior candidate and evidence for inspection while marking it
  disarmed. Disarm does not require the original archive still to be valid,
  so an obsolete trial can always be disabled when the record is intact.

These are illustrative IDs; use an actual candidate from Settings → Tasks.
There is no UI control for this development-stage endpoint yet.

A mode-0600 temporary JSON record is fsynced, atomically renamed over the
prior record, then the parent directory is fsynced. Revision numbers advance
monotonically *inside one running StashBooru process*. Multiple server
processes or external writers are NOT covered by the process-local lock.
A damaged/symlinked/oversized record cannot be silently overwritten;
administrator inspection is required.

### Safety boundary

**This endpoint has no bearing on /image/{id}/image, thumbnails or downloads.**
No normal requests consult the persisted intent, even in `prepared` state.
The original files always remain authoritative. The separately exposed
per-request P12/P13 native preview remains the only way to request archived
bytes through a regular media URL.

The status always reports `NativeRoutingEnabled=false`, `CanActivate=false`,
`OriginalEvictionEnabled=false` and
`TransactionalRollbackVerified=false`. No original file, catalogue row,
media backup or candidate archive is modified. A prepared intent must be
rechecked and byte-reconstructed before any future serving integration.

The next milestone is a safe, staged serving selection with deterministic
fallback, a read-only backup/rollback preflight, and crash/restart tests
against an **isolated** application copy. Original-file reclamation remains
blocked.
