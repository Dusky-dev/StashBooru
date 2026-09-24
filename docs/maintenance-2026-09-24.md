# Maintenance review — 24 September 2026

Reviewed `develop` at `613b49ce92e7198f8d619faf2144d8fa926e7244`, compared with the previous checkpoint `215ebaa89`. The delta includes merged PRs #73–#75 and #77–#88, plus an upstream merge: 67 changed files, approximately 5,243 additions and 442 deletions. This was a focused maintenance pass, not an exhaustive audit of the inherited Stash codebase.

## Assessment of the changes

| Area | Assessment |
| --- | --- |
| Shared inference settings and standalone upscaling | Settings are separated in System, and upscaling has its own UI. Worker configuration still uses the legacy storage/API names for compatibility. Automatic routing currently checks worker availability, rather than availability of the particular requested upscaler. |
| Conversion and upscaling restores | Separate journals/caches avoid mixing new replacement upscales with conversions. Existing legacy upscale records intentionally stay in the conversion history. File-ID navigation correctly resolves the owning media record. |
| Non-destructive upscale copies | Source bytes are retained. Found missing native Copyright links and loss of the earliest source hashes after an already-converted image is upscaled; fixed in this maintenance change. Earlier successful copies also now generate thumbnails/pHashes when a later batch item fails or is cancelled. |
| waifu2x / Docker | GPU fallback and Vulkan diagnostics have regression coverage. Fixed the separate problem where explicit GPU inference was rejected for PNG/JXL because those formats have CPU encoders. Actual GPU drivers/model execution still need device testing. |
| Profile Tags, rating hierarchy, migration 92 | Native Copyright-to-Tag storage and schema version activation are present. Automatic rating parenting bypassed existing cycle validation; fixed. Profile inheritance currently runs through the shared tagging resolver, rather than every manual/native relationship update. |
| Copyright galleries | Image filtering and pagination now run on the server; eager image-ID lists were removed from the main Copyright fragment. Type generation and TypeScript checks pass. |
| Gallery image editor | Context-scoped picking, crop, rotation and pointer-centred zoom are present. No browser interaction testing was performed during this pass. |
| Native Character Auto Tag | Canonical disambiguated identities and aliases share matching rules; the unloaded-alias panic has dedicated coverage. Matching and Auto Tag unit tests pass. |
| Local HTTPS | Secondary listener preserves HTTP when certificate/listener setup fails. Certificate SAN normalization and key permissions have unit coverage. Browser trust and container GPU/network integration were not exercised here. |
| CI | Latest develop Build, golangci-lint and Docker publishing runs were all successful at review time. No dependency upgrades or large refactors were added in this pass. |

## Fixes in this change

1. Preserve native Copyright links when creating an upscaled Image. Reading those links and writing the new Image's links happen inside the same existing creation transaction; errors abort the operation.
2. Preserve the earliest `source_md5`, `source_phash`, and other source fingerprints through repeated conversion/upscale operations. The active MD5 describes the output bytes; the old active pHash is never reused as the derivative's active pHash.
3. Validate automatic rating parents through the existing native Tag hierarchy validator before updating them. Existing valid parents remain intact.
4. Use CPU image encoding after GPU image upscaling. Explicit GPU selection still applies to the inference model, and plain conversion keeps its existing GPU-only behavior.
5. Finish generated thumbnails and hashes for already-created copies when an upscale batch stops early.

The fixes affect future operations. They do not retroactively reconstruct Copyright links or original provenance already lost by previous derivative creation, and they do not rewrite pre-existing invalid hierarchies.

## Branch cleanup

At the initial snapshot there were 52 remote branches and no open PRs. Full Git history was fetched before ancestry checks. **46 non-default branch tips were direct ancestors of `develop`**; their exact names and SHAs remain recorded in `maintenance-2026-09-24-merged-branches.json` as an audit snapshot.

Branch cleanup was completed after rechecking current tips and PR history. The five branches originally excluded from the direct-ancestry deletion set were also reviewed individually:

| Branch | Resolution |
| --- | --- |
| `feature/video-local-booru-metadata` | Superseded by merged PR #42 and later Video Tagging fixes, including local identity priority. Deleted. |
| `fix/upscale-cache-history-copyright-tags` | Superseded by the merged restore-cache implementation in PR #77; its unmerged legacy-cache migration is not required by the current design. Deleted. |
| `feature/set-image-simple-editor` | PR #84 was merged and its post-merge interactive-editor work was superseded by merged PR #85. Deleted. |
| `fix/unified-media-preserve-native-children` | PR #64 was merged; its later toolbar-boundary implementation was merged separately through PR #65, with matching final code. Deleted. |
| `fix/batch-video-tagging-native-toolbar` | Contained only the temporary migration path that was replaced by merged PR #69's native Videos operation. Deleted. |

After cleanup, the only remote branches are `develop` and this maintenance branch. The maintenance manifest is intentionally retained as a historical record of the ancestry-safe deletion set rather than a live branch inventory.

## Follow-up priorities and feature ideas

1. **Worker diagnostics and capability-aware routing:** show codecs, upscaler models, device availability and the reason for fallback in one place; choose a worker that actually supports the requested operation.
2. **Original/derivative families:** link upscaled copies back to their source, offer a synchronized zoom comparison, and let similarity searches group or exclude derivatives.
3. **Resumable batch processing:** persist per-file outcomes for non-destructive upscaling, continue after individual failures, and retry only failed items. The current copy job stops at its first error and lacks the converter's detailed history.
4. **Complete profile-default tagging:** add a Copyright profile Tag editor (the mutation exists but has no non-generated frontend caller), apply defaults consistently when attaching profiles, and record which Tags were inherited so later changes can be previewed safely.
5. **Conversion sampling:** estimate savings and elapsed time from a small user-selected sample before processing a large library; reuse existing verification and restore behavior.

## Verification

- 27 Python converter/upscaler tests passed, including real JXL/AJXL codecs, animation timing, audio preservation, authenticated remote transport, cancellation and the GPU-inference/CPU-encoding regression. No tests skipped.
- GraphQL UI generation and TypeScript checking passed.
- Go converter, matcher, Auto Tag and Tag unit suites passed.
- API and SQLite unit suites passed, using the repository's documented SQLite include flags. This includes the new metadata/identity/cycle tests and the existing HTTPS certificate and schema-version tests.
- Formatting/diff whitespace checks passed.
- Full hardware inference, Docker deployment and interactive browser UI testing are outside this pass.
