# Read-only P12/P13 native media consumer audit

This increment audits a **whole bounded Visual Stack (2–16 images)**,
without enabling any packed-media source or modifying native image paths.

The Settings → Tasks control selects a previously saved P12/P13 candidate
by server-generated ID. The native job resolves the current image members
from the catalogue, verifies the immutable receipt and full archive SHA-256,
verifies the configured original-media backup snapshot, and confirms the
saved hashes match the current originals before proceeding.

An isolated Python process opens the P12/P13 archive **once** and
independently reconstructs **every** member. Each member is byte-compared
with the current original and SHA-256-verified. Invalid members, corrupt
archives, changed original bytes and incomplete source identities fail
closed; the process never extracts archive files to disk.

For each original image, the native job also inspects (via lstat only)
the configured generated thumbnail/clip-preview cache path and returns one
of: missing, cached_regular_unverified, unsafe_or_unreadable.

**A cached file is not proof that a thumbnail or preview is renderable,
correct, freshly generated, or served successfully.** The native
/{imageId}/image, /thumbnail and /preview routes, as well as source
downloads, are **not invoked, intercepted or modified** by this task.
The report always sets nativeConsumersVerified=false,
nativeServingActivated=false, canActivate=false and
originalEvictionEnabled=false and explains the remaining blockers.

The audit runs in the normal job queue; its result is transient and
point-in-time. A future serving or rollback step must rehash all inputs.
Read-only inspection and synthetic tests cannot establish production
media-consumer compatibility or recoverability of database/configuration.

## Verification

- Dedicated synthetic P12/P13 regression tests exercise all-member matching,
  modified originals, wrong source hash, duplicate/missing image IDs,
  corrupted archives, symlink rejection and absence of extraction.
- Go regressions enforce request IDs, output safety flags, complete member
  hashes, no duplicate/missing items and read-only cache state.
- Full Build, Go lint and Settings navigation must pass before merging.
- Browser and owner-library tests are not claimed.

Next implementation milestones are controlled native consumer-path
integration in a **shadow mode**, explicit browser/media format tests,
crash-safe restore of missing originals and verified database/config
recovery. Source eviction remains prohibited until all are independently
demonstrated.
