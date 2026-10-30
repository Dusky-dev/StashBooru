# Explicit opt-in P12/P13 reads on native image and thumbnail routes

This experiment exercises the **normal StashBooru image and thumbnail URLs**
without changing their default behavior. It uses the existing byte-exact
`/image/storage-shadow` proof implementation, rather than adding an
independent archive reader.

In **Settings → Tasks → Compare packed shadow image**, choose a verified
saved P12/P13 candidate and member. The comparison now links to two trial URLs:

- `/image/511/image?storageShadowCandidate=<saved-id>&storageShadowStack=51`
- `/image/511/thumbnail?storageShadowCandidate=<saved-id>&storageShadowStack=51`

Numbers are examples, not authoritative candidate identities. Browser
requests **cannot** supply arbitrary original/archive paths, media types,
or image IDs separate from the actual numeric URL path. The two opt-in
parameters are required exactly once; unexpected extra query parameters,
duplicate values, invalid IDs or unsafe candidate IDs return 400.

For a selected request, native ImageCtx first resolves the image normally.
The existing shadow path then reopens the saved candidate receipt, verifies
the entire published archive hash, checks the trusted current Visual Stack
and member identity, independently hashes/compares the current original
against the reconstructed member, and only then serves byte-exact bytes.
Thumbnail requests use the native in-memory JPEG thumbnail encoder on the
verified bytes. The normal generated thumbnail cache is not accessed or
written in this opt-in mode.

**Crucial behavior:**

- Without BOTH opt-in parameters, normal image and thumbnail requests are
  unchanged, including existing cache and fallback behavior.
- Every opt-in request is authenticated against its own candidate and
  current original. Source originals must still exist and match.
- An invalid opt-in request fails closed rather than silently switching to
  unrelated archive data.
- This has no persistent activation, server-wide toggle, filesystem
  mutation, database migration, background conversion, preview generation,
  catalogue rewrite, source eviction, or space reclamation.
- Other native media consumers and codecs are **not** proven compatible.
- A successful synthetic Go test or a successful trial of one original
  does NOT prove full native serving, restart recovery, transactional
  rollback, or the ability to delete originals.

Continue with browser compatibility testing using a disposable library and
normal route regressions before announcing an experimental packed-serving
beta. Do **not** enable original-file deletion on the basis of this opt-in.
