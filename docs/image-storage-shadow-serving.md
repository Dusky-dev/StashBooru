# Experimental packed-image shadow serving (no activation)

This is a **separate, explicit read-only compatibility endpoint**, not a
replacement for any existing StashBooru media route.

In Settings → Tasks → Inspect media, choose a saved byte-exact P12/P13
candidate and enter the native ID of one image in that Visual Stack.
The Open shadow image (read-only) link requests:

GET image/storage-shadow?stackID=(number)&imageID=(number)&candidateID=(saved-ID)

No browser-supplied file paths, output directories, media overrides or
source deletion flags are accepted.

## Read path and validation

1. Resolve the current native Visual Stack and original primary image paths
   from the catalogue; reject missing or non-regular originals.
2. Require the requested image ID to belong to the selected stack.
3. Validate configured media backup and private candidate root locations.
4. Reopen the immutable candidate receipt and **rehash the entire archive**,
   matching its saved archive SHA-256.
5. Revalidate the candidate identity, member SHA-256 evidence and byte-exact
   fidelity, rejecting unsafe receipt flags.
6. Run the isolated P12/P13 Python reader. Re-read the existing original
   and hash it against the saved proof; independently decode the requested
   archive member, hash and byte-compare with the active original.
7. Only return fully verified bytes, bounded to 64 MiB, with no-store,
   nosniff, a restrictive Content-Security-Policy, and a forced attachment
   for unknown/non-raster response types.

The endpoint is serialized to one expensive reconstruction at a time, with
a subprocess timeout. Unknown image IDs, tampered archives, changed original
bytes, symlinks, unexpected paths and invalid requests fail closed.

The normal image/{imageId}/image, preview and thumbnail routes remain
**entirely unchanged**, and original files are never deleted or rewritten.

## Scope and remaining checks

This proves only one selected member while its original is still available.
It does NOT enable packed media for any normal consumer, support serving
missing originals, prove every image codec/browser, perform activation or
rollback, recover the database/config, or reclaim disk space.

JXL/AVIF and other unknown MIME formats are served as binary attachments
until client-specific support can be validated. Synthetic Python CI exercises
P12/P13 and source/archive corruption; Go tests enforce query and MIME
safety. Full Go lint, Build, and Settings tests are required before merge.
Actual owner-library and mounted-browser visual validation remain outstanding.
