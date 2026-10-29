# Side-by-side original versus packed-shadow browser comparison

This is an **opt-in, disconnected visual check** for a saved P12/P13
candidate. It is a step toward native media compatibility testing, not
activation of compressed library storage.

In Settings → Tasks → Compare packed shadow image, choose a saved candidate
and enter the numeric ID of one image in its Visual Stack. Click
**Compare original and packed reconstruction**.

Two separate browser elements then load on demand:

- The existing original-media route: `image/{imageID}/image`.
- The isolated verified archive route:
  `image/storage-shadow?stackID=...&imageID=...&candidateID=...`.

The shadow route still re-hashes the immutable archive, verifies membership
and full source-file SHA-256, decodes the byte-exact P12/P13 member and
compares it against the **currently existing original** before returning
any image bytes. The browser renders the images side by side and independently
reports whether each load succeeded. Direct original and reconstructed-byte
links remain available for unsupported browser formats.

### Additional automated HTTP contract checks

The test-only Go helper exercises the isolated verified-byte HTTP response
with a small synthetic PNG, comparing complete GET bytes and Range response
bytes/headers against a regular original-file HTTP response. It also checks
HEAD behavior and the required no-store, nosniff and restrictive CSP headers.
HTML, SVG and arbitrary unsupported data are forced to binary downloads,
rather than being presented as active inline content.

**These are synthetic HTTP-handler tests, not an end-to-end test of all
existing StashBooru image consumers.** Thumbnail generation, cached
clip-preview serving, large-file/format compatibility, browser decoding,
streaming/backpressure, crash recovery and actual owner-library comparison
still require separate validation. A successful side-by-side display is not
permission to remove the originals.

The native image, thumbnail, preview and download routes are not changed;
existing original images, database rows, archives and backup store are never
mutated by the comparison.
