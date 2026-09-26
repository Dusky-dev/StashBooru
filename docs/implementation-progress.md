# StashBooru implementation progress

Updated: 2026-09-26

| Package | Status | Notes |
| --- | --- | --- |
| P01 — backup compatibility warning | Complete | Covered by merged PR #91; further backend backup-contract work intentionally skipped per project-owner direction. |
| P02 — image similarity modes | Complete | Merged PR #92 adds explicit pHash and EVA02 modes for image Find similar. |
| P03 — visual comparison / difference highlighting | Complete | Merged PR #93. Scope remains the bounded still-image comparison described below; automatic alignment and video frame selection are not implemented. |
| P04 — copyright sorting / taxonomy | Complete | PRs #94 and #95 are merged. PR #94 added the hierarchy/backend foundation; PR #95 corrected hierarchy UX and media presentation without changing the P04 database schema. |
| P05 — Character variants / disambiguation links | Implementation complete; review pending | Adds a single-parent Character variant relation and typed Copyright/Artist disambiguation links. P06 ancestor auto-association remains separate. |

## P04 completed behavior

Backend and data-model work from merged PR #94 is preserved:

- Native Copyright IDs and parent/child relations remain the hierarchy source of truth.
- Multi-parent Copyrights are supported; hierarchy writes reject self-links, missing targets and cycles.
- Copyright directory sorting supports name/sort name, timestamps, direct media counts, descendant-inclusive counts and stable ID tie-breaking.
- Descendant-aware media filtering/counting counts distinct media across converging hierarchy paths.
- Breadcrumbs and per-parent manual sibling ordering remain available.
- Existing deletion behavior keeps media and child records rather than cascading media deletion.

Corrective UX in merged PR #95 (`fix/p04-main-sub-branch-grouping`, based on `develop` at `ae28561d057ff0e2d75136b9fc6da67c941608fd`):

- Removed the separate Flat/Tree Copyright-directory mode. The directory uses the normal Stash Grid/List display controls only.
- Copyright detail pages expose hierarchy navigation through native Bootstrap/Stash-style **Main** and **Sub** tabs. Main shows parent/main Copyright relationships; Sub shows child/sub-Copyright relationships and manual child ordering. Breadcrumb navigation remains available.
- Removed the visible free-form Structural Role controls from Copyright and Tag details. The merged schema field is retained for compatibility but is not part of the corrected P04 UX because it currently has no functional behavior.
- Removed the Image/Video global Primary Copyright selector. The merged schema/API field is retained for compatibility, but the corrected presentation no longer relies on one globally preferred Copyright merely to put it first.
- Image and Video Copyright presentation now fetches hierarchy breadcrumbs and groups attached Copyrights by independent hierarchy root/branch. When several attached Copyrights share a branch and diverge below it, the group heading uses their common hierarchy prefix. This allows unrelated branches such as Pokémon and Super Mario to remain separate on the same media item.
- New hierarchy UI reuses React-Bootstrap/native Stash controls and existing card/theme classes rather than adding raw always-visible inputs.

### P04 migration / compatibility impact

- PR #95 adds no migration and does not alter the merged P04 hierarchy tables or validation logic.
- `structural_role`, `primary_copyright` and related merged API/storage remain readable for compatibility. Their mistaken/decorative UI is removed rather than destructively migrating existing databases.
- A future compatibility-cleanup package may deprecate those dormant fields if desired; that is not required for the corrected Main/Sub hierarchy behavior.

### P04 verification

Code head `67f72916f09dddb72edb0be439a099bc52725edc` passed the repository GitHub Actions gates before this ledger update:

- backend generation and `golangci-lint` — passed;
- UI generation — passed;
- UI test suite — 17 tests passed;
- JavaScript/CSS lint — passed;
- TypeScript `tsc --noEmit` — passed;
- Biome formatting — passed;
- UI production build — passed;
- backend test job — passed;
- cross-platform build matrix — passed, including Linux, Linux ARM variants, Windows, FreeBSD and macOS.

The first corrective CI attempts exposed formatting-only Biome failures. Those were fixed; no test, lint or TypeScript failure remained on the validated code head above.

## P05 Character variants and disambiguation links

- Characters remain native Performer records with their own IDs, aliases, images, tags and metadata. Each Character can optionally point to one base Character; validation rejects missing parents, self-links and cycles.
- Character details show a link to the base Character and a Variants tab listing its direct variants. The edit form can select or clear a base Character.
- A Character can have one typed disambiguation target: a Copyright or Artist (Studio). The target name is displayed as a link in details, lists, selects and tagger views; a rename is reflected automatically. Deleting a target clears the relation, while existing free-text disambiguation remains as fallback.
- Legacy free-text disambiguation is preserved and remains independently editable. Linking an Artist does not automatically apply that Artist to media; automatic ancestor/profile association is P06.
- Migration 95 adds the parent and context foreign keys with `ON DELETE SET NULL`, a check against self-parenting and indexes for lookups.

### P05 verification

- Backend gqlgen and dataloader generation passed.
- UI GraphQL generation, TypeScript check, Biome lint and formatting passed.
- `go test ./pkg/performer ./internal/api` passed with the repository SQLite include flags.
- The focused SQLite integration test `Test_PerformerVariantAndDisambiguationContext` passed, covering persistence, variant lookup, context switching, and cleanup after deleting linked records.

P05 is implemented and ready for review. P06 ancestor auto-association is the next roadmap package after P05 is reviewed and merged.

## P03 validation scope

- Reuses the existing `ReferenceComparison` Lightbox path rather than introducing a second comparison UI.
- Keeps existing side-by-side, wipe, and synchronized pan/zoom behavior.
- Difference mode is deliberately limited to still images. Animated/video media reports that an explicit frame or timestamp is required instead of comparing arbitrary frames.
- Pixel work is bounded to a centered-fit canvas of at most 2048 pixels per dimension and about 1.5 million pixels.
- Different aspect ratios are not stretched; unmatched borders remain visible difference evidence.
- Difference boxes are connected pixel-difference regions after threshold/noise filtering, not object detection.
- Browser decoding supplies orientation/color handling for this first pass; automatic crop/rotation registration is not implemented.
- No database or GraphQL schema changes.
