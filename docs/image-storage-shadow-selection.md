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
The Settings → Tasks → Compare packed shadow image panel now includes an
explicit **Prepare candidate (no activation)** button and a **Disarm saved
trial** button. Both require acknowledgement and display the saved intent,
current evidence status, and the continuing original-source-only policy.

A mode-0600 temporary JSON record is fsynced, atomically renamed over the
prior record, then the parent directory is fsynced. Revision numbers advance
monotonically *inside one running StashBooru process*. Multiple server
processes or external writers are NOT covered by the process-local lock.
A damaged/symlinked/oversized or non-private record cannot be silently overwritten;
administrator inspection is required. On POSIX, a record made accessible to
group or other users fails closed. Status and action responses are non-cacheable.

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

### Read-only prepared-byte preflight

Settings → Tasks → Inspect media → Compare packed shadow image includes an
explicit **Verify prepared candidate bytes (read-only)** action. It calls
`POST /image/storage-shadow-preflight` with only
`{"Acknowledged":true}`. No request-provided image IDs, local media paths or
archive paths are accepted. The prepared server-side candidate is the sole
authority for selecting members.

This bounded, serialized operation revalidates the saved selection and Visual
Stack, the full packed archive SHA-256, the original-media backup snapshot,
and all 2–16 candidate members by reconstructing each one byte-exactly while
comparing it to the currently retained original file. It re-reads the durable
selection and candidate evidence at the end to reject a disarmed, replaced,
stale or changed trial.

The result is **transient** and point-in-time; it is not a durable verification
receipt, is not a coherent database/configuration restore, and does not prove
crash-safe rollback or safe operation if originals are missing. Successful
preflight does **not** turn on native routing and never grants permission to
delete originals. If any check fails, the endpoint returns an error and normal
original-file serving remains untouched. A maximum four-minute request budget
is enforced, with a shared slot limiting concurrent expensive shadow checks.

### Explicit prepared-media canary (originals always retained)

After a successful read-only preflight, Settings → Tasks → Compare packed shadow
image offers an **Open prepared canary** link for a member of the selected
candidate. This calls `GET /image/{numericImageId}/storage-canary` with no
query parameters. It is a **separate explicit URL**: ordinary image, thumbnail,
preview, download and image-detail routes are never redirected to packed
storage by this feature.

Only one privately saved `prepared` selection is read on the server. The
selected image must belong to its current Visual Stack. The server revalidates
the private intent and candidate receipt/archive/catalogue evidence, checks
that the reconstruction script is a regular file, and regenerates the exact
original bytes from the P12/P13 archive while independently comparing them
with the retained original. The response header
`X-Stash-Shadow-Source: packed-verified` indicates a successful canary read.

If the candidate cannot be reconstructed or verified, or the shared shadow
verification slot is busy, the canary route serves the **same retained original
image** as the default image route and marks the response
`X-Stash-Shadow-Source: original-fallback`. An absent/disarmed/unsafe intent,
invalid ID, non-member, unknown query parameter or unavailable original does
**not** authorize packed data, new sources or eviction and may fail closed
with a 400, 404, 409 or native file-serving error. Results use
`Cache-Control: no-store`, and the normal image route is unaffected.

The canary is a point-in-time probe only. The saved `prepared` intent persists
over an ordinary application restart but never causes automatic serving. Any
`disarm` revokes new explicit canary requests (409). The isolated end-to-end
test boots, verifies packed bytes, range requests and an unchanged normal
route, changes a synthetic retained original to force source fallback, then
restores it, restarts the disposable server and confirms disarm refusal.
No source-file eviction, database rewrites, archive activation or successful
coherent rollback are implied. Cross-process concurrent writers are not
covered by the process-local selection lock.
