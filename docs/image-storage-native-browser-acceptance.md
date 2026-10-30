# Experimental packed-image browser acceptance

This test is a browser-level extension of the non-destructive, real-server
P12/P13 acceptance test from #289. It runs in a fresh **ephemeral** StashBooru
instance and never connects to any configured production library.

Using the temporary, two-image Visual Stack, original-media backup and
verified P12/P13 candidate receipts already created by the acceptance suite,
the browser opens the normal Image Details route with explicit
`storageShadowCandidate` and `storageShadowStack` query parameters.

For **each** P12/P13 method the browser confirms:

- The normal image endpoint returns HTTP 200 for the verified candidate.
- An actual rendered `img.image-image` uses the opt-in native image URL.
- The HTML image finishes decoding with its expected dimensions.
- Existing native byte-exact GET, byte-range, thumbnail and identity tests
  continue to pass unchanged.

The browser also requests a structurally valid but nonexistent candidate,
confirms the native route returns HTTP 409, and independently verifies that
Image Details switches back to the original image without leaving a broken
image or making a persistent source selection.

**Safety boundary:** Native image routing is selected only for an explicit
query-string trial. The original files are still mandatory and retained.
This does not establish that every browser/codec combination is supported,
that packed storage can become the active catalogue source, that a backup
and database snapshot were captured atomically, or that any original can
be deleted. No space is reclaimed.

The existing `packed-native-acceptance` Build job runs the extended suite
after compiling a Linux binary, so a failed rendered-image or fallback test
blocks the PR. No production original images are used in the test.
