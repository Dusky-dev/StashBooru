# Opt-in packed shadow inside the native image detail viewer

Settings → Tasks → Compare packed shadow image now offers
**Test packed preview in image details** for a selected, verified candidate
image. The link opens the normal `/images/:id` detail page with
`storageShadowCandidate` and `storageShadowStack` query arguments.

The viewer now invokes the **normal** `/image/{id}/image` endpoint with
explicit `storageShadowCandidate` and `storageShadowStack` query arguments.
For these requests only, the normal route delegates to the existing verified
shadow reader. The backend, not the browser, performs the authoritative
saved candidate archive SHA-256 verification and compares the reconstructed
bytes against the current native original. Normal image requests without
these query arguments remain unchanged. A failed shadow
fetch falls back to the unchanged normal image URL. Users can click
**Return to original image** to remove the opt-in query.

There is **no persistent toggle** and no rewrite of image catalogue paths,
normal image/media endpoints, generated preview cache files or stored
candidate receipts. Videos and unsupported preview formats do not use the
shadow route. Navigating to images without the opt-in query uses the
originals. Nothing in the UI enables compressed-image serving for other
users or permits deletion of the original files.

The experiment measures visual and compatibility behavior *inside the real
image detail viewer*. It is not production packed serving, crash-safe
activation, rollback validation or disk-space reclamation.
