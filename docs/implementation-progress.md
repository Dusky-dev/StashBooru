# StashBooru implementation progress

Updated: 2026-10-04

Current baseline: `develop` at `57443b7a8773de267cad81ba80c461fc8363a15e`
(merged PR #133). P01–P09 are complete and merged. Current package: P10 Video
overlap/containment review, implemented and verified on
`feat/p10-video-overlap-20261004`. See [P10 notes](p10-video-overlap.md).

| Package | Status | Notes |
| --- | --- | --- |
| P01 — backup compatibility warning | Complete | Covered by merged PR #91; further backend backup-contract work intentionally skipped per project-owner direction. |
| P02 — image similarity modes | Complete | Merged PR #92 adds explicit pHash and EVA02 modes for image Find similar. |
| P03 — visual comparison / difference highlighting | Complete | Merged PR #93. Scope remains the bounded still-image comparison described below; automatic alignment and video frame selection are not implemented. |
| P04 — copyright sorting / taxonomy | Complete | PRs #94 and #95 are merged. PR #94 added the hierarchy/backend foundation; PR #95 corrected hierarchy UX and media presentation without changing the P04 database schema. |
| P05 — Character variants / disambiguation links | Complete | Merged PR #104 adds native Character variants and typed Copyright/Artist disambiguation links; merged PR #114 refines Character/Copyright editing. |
| P06 — ancestor/profile auto-association | Complete | PRs #115–#118 are merged; the final search consistency follow-up passed Build and Go lint CI. Direct legacy relationships remain explicit; no migration is introduced. |
| P07 — native global All media page | Complete | PRs #119–#129 are merged. The standalone follow-up below removes screenshot posters during automatic Video startup. Equal Copyright/Variant image heights and native preview navigation are retained. |
| P08 — visual stacks / variant filmstrip | Complete; PR #131 merged | Durable catalogue grouping, native controls and reviewed proposals passed backend gates, 30 UI tests, production builds, and desktop/mobile Chromium/Firefox workflows. See [P08 notes](p08-visual-stacks.md). |
| P09 — conversion review / savings thresholds | Complete; PR #133 merged | Estimate, verified saved trials, reviewed apply, both savings minimums, cache/expiry/revalidation and native restore. See [P09 notes](p09-conversion-review.md). |
| P10 — Video overlap / containment | Complete; publication below | Versioned PTS samples, indexed retrieval, exact/near-complete/contained/partial/compilation evidence, durable index checkpoints and synchronized segment review. See [P10 notes](p10-video-overlap.md). |

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
- Character details show a “Variant of” link and a Variants card grid beside Copyrights in the same `detail-group` row. The edit form selects child variants from the parent Character.
- A Character disambiguation selector chooses Copyright, Artist, or Custom. Copyright and Artist choices link the target name; Custom is a plain label. Disambiguation context does not create a media association.
- Migration 95 adds the parent and context foreign keys with `ON DELETE SET NULL`, a check against self-parenting and indexes for lookups.

### P05 verification

- Backend gqlgen and dataloader generation passed.
- UI GraphQL generation, TypeScript check, Biome lint and formatting passed.
- `go test ./pkg/performer ./internal/api` passed with the repository SQLite include flags.
- The focused SQLite integration test `Test_PerformerVariantAndDisambiguationContext` passed, covering persistence, variant lookup, context switching, and cleanup after deleting linked records.

P05 is complete and merged in PRs #104 and #114.

## P04/P05 hierarchy and disambiguation UX follow-up — merged PR #114

- Copyright pages use Copyright/Main/Sub terminology; the editor can select both Main and Sub-Copyrights, empty hierarchy tabs are hidden, and related Copyrights display as cards. Image/Video detail sidebars render Main/Sub hierarchy paths as cards too.
- The optional alphabetical sort name now has an example explaining how it affects ordering.
- Character Copyright associations display as cards before the other detail fields, with singular/plural labeling. Character variants are edited as child selections and displayed as cards.
- Character disambiguation uses one Copyright/Artist/Custom selector with Stash-native styling. The New Sub-Copyright action was removed.
- PR #114 changes only the UI and progress ledger; it adds no schema, migration, or backend changes.

## P06 ancestor/profile auto-association — PRs #115, #116 and #117

- Merged PR #115 supplies Image/Scene effective associations, descendant-aware searches/counts and four independent ancestor defaults. Merged PR #116 adds per-membership provenance and the linked association-source presentation.
- PR #117 shares native direct-association loaders between Image/Video detail reads and the existing-library review, de-duplicates legacy primary Artists and multi-Artist links, and fixes inherited Tagging preview reads to own a SQLite read transaction.
- Native direct relationships remain the editable selections. Ancestors and profile Tags are live projections; shared sources, explicit parents and legacy assignments remain distinguishable. Reparenting changes the next read without rewriting media.
- Settings → Tasks now offers a Review library inheritance preview/apply task through the native Jobs queue. It reviews every existing Image/Video with paginated reads, reports inherited totals and membership deltas, and shows bounded before/after samples with provenance links. Cancellation and per-item errors are visible.
- Apply fingerprints every media item's complete provenance and rechecks the library under a native write transaction before atomically saving the reviewed defaults. Changed associations, profiles, hierarchy or defaults require a new preview. No inherited IDs are copied into direct native tables.
- The reviewed backfill activates the existing library's live projection. System and Tagging share all four enabled-by-default switches, with links from System to the task and Tagging to System.
- Fresh SQLite integration fixtures exercise actual single and bulk GraphQL mutations, reviewed Image/Video Tagging, bulk Video filename review/apply, native Auto Tag, import/repeated import and scan/rescan. They verify direct-only selections, derived membership/provenance, Copyright/Tag diamonds, shared-source detach, explicit-parent retention and native reparenting. Task tests cover pagination, stale previews, invalid-write rollback, errors and cancellation.
- Review state is transient: terminal reports expire after 30 minutes, at most eight reviews are retained, and restarting the server requires another preview. Scan tests stub only thumbnail/cover generation. External model/booru availability and manual browser interaction are outside this follow-up's verification.

The behavior and workflow matrix are documented in [P06 effective associations](p06-effective-associations.md).

### P06 Tag search consistency follow-up — 2026-10-01

- Reproduced a missing acceptance case on merged `develop`: the Image/Video
  detail projection included Artist profile Tags, but the native Tag filter
  returned no media for the same Tag.
- Image/Video Tag `INCLUDES`, `INCLUDES_ALL` and `EXCLUDES` now resolve effective
  memberships through native links, selected profiles, enabled ancestor profiles
  and enabled Tag ancestors. Tag detail media counts use the same native query.
- Searches expand only the requested Tag and supplying profile metadata, then
  join indexed native media relationships. They do not load every media item,
  materialize inherited links, or change IDs, fingerprints or provenance.
- Independent source paths and Copyright/Tag diamonds count each media once.
  The existing server-side sort/pagination and explicit descendant-depth control
  remain available. Exact-set/null criteria, Tag-count maintenance criteria and
  nested `tags_filter` retain their native direct-link semantics.
- New `internal/api/p06_tag_search_integration_test.go` checks the native SQL
  results against the independent Go effective-association projection for both
  media types and all 16 defaults. It also checks Tag detail counts, shared
  sources, exclusions, pagination, profile edits, reparenting, explicit-link
  retention after detach and termination on a malformed legacy Tag cycle.
- Local verification passed: focused native search regressions;
  `make generate-backend`; the full `make it` integration suite;
  `make lint` (zero issues); and `git diff --check`.
  Validated code commit: `c8e7f9c636bd0a1d7ef73103e1a525e89cf565e0`.
- Merged as [PR #118](https://github.com/Dusky-dev/StashBooru/pull/118).
  Remote [Build](https://github.com/Dusky-dev/StashBooru/actions/runs/36844384502)
  and [Go lint](https://github.com/Dusky-dev/StashBooru/actions/runs/36844384515)
  completed successfully.
- No database, GraphQL, UI or writer changes. P07 remains the next roadmap
  package; this branch is limited to the P06 Tag search correction.

### P06 verification

Code head `dfdd33bccae9971af298c666a1d4acbfa535226c` passed the [Build workflow](https://github.com/Dusky-dev/StashBooru/actions/runs/36761342091) and [Go lint workflow](https://github.com/Dusky-dev/StashBooru/actions/runs/36761342087) on 2026-09-30:

- Backend generation and `golangci-lint` passed.
- `make it` (`go test -tags integration ./...`) passed, including the real SQLite API/configuration tests and the new native workflow matrix.
- All seven platform builds passed: Linux, Linux ARM64/ARMv7/ARMv6, Windows, FreeBSD and macOS.
- The unchanged UI passed all 20 tests, JavaScript/CSS lint, TypeScript, Biome formatting and the production build at code head `2a4c1b6d6bbc32ce2b1409c457fcbad9d0b8bd32` in the [UI/build workflow](https://github.com/Dusky-dev/StashBooru/actions/runs/36758817817). Later code commits change only backend integration tests; CI reuses that validated UI build.

No browser interaction or external inference/booru service test is claimed.

## P07 global Media page — 2026-10-01

- `/media`, `/media/images` and `/media/videos` share the native filter toolbar,
  saved shared criteria, Grid/Wall controls, pagination and typed selection.
  Image and Video detail links keep their native IDs.
- `findMedia` applies native shared predicates and one SQL union ordering before
  pagination. Counts use the same predicates and identity-only projections;
  only the requested sort key is projected for ordering, and only the selected
  page is hydrated as native Image/Scene data.
- Typed keys distinguish `image:123` from `scene:123`. Sort ties use kind then ID;
  missing values sort last in both directions. Page sizes are bounded to 500.
  A duration criterion explicitly excludes Images.
- Shared native animation badges label animated Images without reclassifying
  them or duplicating catalogue entries. Existing badge enhancements detect
  native badges and avoid adding another one.
- Mixed metadata edits dispatch native Image and Video mutations separately,
  expose partial failures and retry only the failed type. Conversion passes both
  native target kinds; upscaling selects only Images and reports excluded Videos.
- URL state and eight browser-session snapshots retain list context for back
  navigation. Existing detail-page enhancements remain; P07 adds no recursive
  React tree walker and no database migration.
- Verification passed: backend and UI GraphQL generation, full `make it`,
  `make lint` (zero issues), all 25 UI tests, JavaScript/CSS lint, TypeScript,
  Biome formatting, production UI build, Linux application build and
  `git diff --check`.
- An isolated running application passed an HTTP smoke using the actual Media
  query and native UI fragments: alternating rows across three pages, per-kind
  rating filters/counts, equal-ID native metadata writes and all five new/native
  list routes. The smoke database contains only synthetic records.
- Full Go checks ran sequentially with reduced compiler concurrency after a
  concurrent attempt exceeded workspace memory. The local application build
  used `-buildvcs=false` because this workspace's subprocess Git lookup starts
  outside the checkout. CI retains its standard build commands.
- [PR #119](https://github.com/Dusky-dev/StashBooru/pull/119) was merged on
  2026-10-01. Its Build and Go lint workflows passed. Browser interaction,
  scroll restoration and rendered Grid/Wall previews have not been manually
  verified: Chromium was unavailable and its download failed.

### P07 navigation and Character layout follow-up

- The backend's default menu includes the mixed media page. Its navigation and
  Interface setting labels are “All”; existing custom menu selections still apply.
- Character variants appear beside Copyrights in the same `detail-group` row,
  rather than in a tab. Native card links, disambiguation, loading,
  errors and the existing 100-card limit remain. Empty variant groups are hidden.
- Existing `/performers/:id/variants` links fall back to the Character page.
- Verification passed: configuration package tests (including default/custom
  menu regression coverage), scoped Go lint (zero issues), all 25 UI tests,
  JavaScript/CSS lint, TypeScript, formatting, production UI build and
  `git diff --check`. PR #120 was merged; browser verification was completed
  during the subsequent All list rework below.

### P07 All list rework

- Reproduced React error #185 in Chromium while delaying the mixed query. All
  now mounts Lightbox synchronization only inside the loaded list body, following
  Images. Stable result adapters also avoid creating fresh empty arrays.
- All uses native sidebar filters, toolbar actions, pagination and cards. The
  duplicate All/Images/Videos tab strip is removed. Old type-specific Media URLs
  redirect to the existing Images/Videos pages with their query state.
- Mixed Wall reuses native Video preview source selection and styling, handles
  failed previews and invalid dimensions, bounds item height and opens native
  Video links with their queue. Selection retains typed Image/Video identities.
- Copyrights and Variants share a responsive relation row. Variants still use
  the existing card and edit flows.
- Verification passed: all 28 UI tests, TypeScript, JavaScript/CSS lint,
  formatting, production UI build and `git diff --check`. Chromium exercised
  delayed, empty, Video-only and failed mixed queries; typed mixed selection;
  Wall navigation/back; and legacy native-page redirects. An isolated fixture
  confirmed equal desktop row positions and mobile wrapping without overflow.
  Browser fixtures use synthetic records; real-file playback and external
  conversion/upscaling services were outside this correction.

Query semantics, supported operations and remaining browser checks are recorded
in [P07 global media](p07-global-media.md).

### P07 Character edit pane, relation sizing and Video previews

- The scoped All inline host now clears its active state and unmounts media
  contents when Edit removes the native tabs. Cancel restores the native tabs
  without retaining an active All pane beneath the editor.
- Copyrights and Variants use content widths with a one-rem gap. Images keep
  their full proportions at a bounded height; names wrap and long relation
  lists scroll within the viewport-based height cap.
- Restored the existing Video magnifying-glass bridge, card registry and native
  ScenePlayer preview flow. Selection hides the preview action. Global All
  restores Image lightbox contents when an Image is previewed after a Video.
- Rapid preview navigation exposed a queued marker callback after player
  disposal; that callback now exits safely.
- Verification passed: 28 UI tests, TypeScript, JavaScript/CSS lint, formatting,
  built-in checksum/syntax, production build and `git diff --check`. Chromium
  checked scoped/global All and native Video previews, carousel navigation,
  Video-to-Image preview switching, edit/cancel, short/tall relation images,
  18-item scrolling lists, delayed/empty/failed queries, selection, Wall/back
  navigation, legacy routes and mobile overflow. Browser fixtures are synthetic.

### P07 navbar order, card shapes and preview stability

- All is first and capitalized; Copyrights sit between Artists and Tags in both
  the visual navbar and DOM order. Menu enable/disable preferences remain supported.
- Native Copyright images use 16:9 landscape frames, Artists use square frames,
  and Characters retain portrait frames. Copyright/Variant relation cards use
  the same typed layouts, fit sources without cropping, and bound their width
  to preserve proportions on short viewports. The prior Artist 150-pixel height
  cap and fixed header height no longer constrain its square card image.
- Reproduced a background lightbox caller refreshing its Image contents while
  a Video preview was open, replacing the active viewer. Active caller ownership
  now isolates synchronization; stable open callbacks read the latest caller
  state and reset inherited slideshow/paging callbacks.
- The Video bridge uses request-scoped opener refs, rejects duplicate opens and
  cancels queued opens after dismissal. Video preview slideshow is disabled;
  selected Video playback and explicit next/previous controls remain available.
- Browser regressions cover background refresh/resume-cache updates, idle time
  longer than the default slideshow interval, repeated/double-clicked opens,
  Escape/Back/button dismissals, desktop/mobile navbar order and opposite-shape
  card sources. Synthetic fixtures verify viewer state and mounted player UI;
  they do not verify real-file decoding or playback.
- Validation completed before interruption and recovered on resume: all 28 UI
  unit tests, TypeScript, Biome lint/format, Stylelint and the production UI
  build passed. The final browser runs passed after removal of the Artist
  height cap. Resume checks confirmed the unchanged `develop` baseline,
  reviewed desktop captures, and passed built-in composition/checksum,
  JavaScript syntax and `git diff --check` checks.
- No migration, GraphQL or backend behavior changes. Published as
  [PR #123](https://github.com/Dusky-dev/StashBooru/pull/123), with implementation
  commit `ca852aa2a0a75056cfdb3e44477d9c70649b195c`. Build and Lint CI were running
  at publication; merging and real-file playback remain separate checks.

## P03 validation scope

- Reuses the existing `ReferenceComparison` Lightbox path rather than introducing a second comparison UI.
- Keeps existing side-by-side, wipe, and synchronized pan/zoom behavior.
- Difference mode is deliberately limited to still images. Animated/video media reports that an explicit frame or timestamp is required instead of comparing arbitrary frames.
- Pixel work is bounded to a centered-fit canvas of at most 2048 pixels per dimension and about 1.5 million pixels.
- Different aspect ratios are not stretched; unmatched borders remain visible difference evidence.
- Difference boxes are connected pixel-difference regions after threshold/noise filtering, not object detection.
- Browser decoding supplies orientation/color handling for this first pass; automatic crop/rotation registration is not implemented.
- No database or GraphQL schema changes.

## Repository agent guidance and upstream merge check — 2026-10-01

- `AGENTS.md` records the fork owner's standing authorization for implementation,
  feature-branch publication and PR creation/updates, while keeping merging a
  separate action. It documents product conventions, native UI extension points,
  progress handoffs, repository validation commands and upstream checks.
- Guidance branch: `chore/agent-workflow-20261001`, based on current `develop`
  at `4a0fc01b57c9616c6aae8e0abde65d48a5ee43c7` (merged PR #122).
- Fetched `stashapp/stash` `develop` at
  `e7d33c9bd131f1f5d781b850de30735943fa1195`. Seven upstream commits are absent
  from the fork, touching 14 UI files. They improve scraper search, create-page
  plugin hooks, duration matching, the organized indicator, portrait Video
  wrapper height, VR colors and Safari Character details layout. No incoming
  database, GraphQL schema or dependency-manifest changes were found.
- `git merge-tree --write-tree --name-only` reports zero conflicts against both
  current fork `develop` and pending [PR #123](https://github.com/Dusky-dev/StashBooru/pull/123)
  at `a8800230477b24bb93168f0512cc5325d0ac8f93`. Candidate tree SHAs are
  `fe1fa4e2237a330426a7474e59e0f94139a77f16` and
  `bf55bf1286898ff572f3ca7e59333e82229c7001`, respectively.
- An isolated `git merge --no-commit --no-ff upstream/develop` including #123
  completed without conflicts or unmerged index entries. The temporary result
  passed TypeScript, all 28 existing UI unit tests, Biome lint/format, Stylelint
  and staged/unstaged `git diff --check`. It reused unchanged dependencies and
  generated UI GraphQL artifacts; Node's ESM registration API ran the existing
  ts-node suites in the available Node 24 runtime.
- The guidance change itself is documentation only and passed diff checks.
  No upstream integration was published or merged into the fork. The temporary
  source checks do not establish real Safari rendering, VR playback or full
  production/build-matrix results. Recheck fresh heads before a later merge.

## Video preview cycling and Copyright relation width — 2026-10-01

- Fresh `origin/develop` baseline verified at
  `8420388976f7f2d33cd48681fdf6b4abcaf8cce6`, after PRs #123/#124 and the upstream
  merge. Delivery branch: `fix/p07-preview-cycle-copyright-width-20261002`.
- Reproduced metadata refreshes cycling the actual native ScenePlayer through
  Videos 1, 2, 3 and 4 while the footer remained on Video 1. The previous footer
  assertion missed this mismatch. The new regression observes native player
  identity as well as the footer and player count.
- Video card registrations update in place. An open preview retains captured
  typed identities/order, with metadata reconciled by identity; the player,
  Lightbox and observer share the sequence. New opens use the visible card order
  after sorting, and dismissals release the queue. Both the native Video host
  and existing scoped mixed All path use this behavior.
- Copyright relation cards now use up to 20 rem/70vh landscape width, with a
  larger desktop section allowance. Variant portraits retain their existing
  11 rem/30vh cap. The fixture measures Copyright frames at 278 px versus
  152 px for Variants, with close spacing, preserved ratios, internal scrolling
  for large lists, and mobile wrapping without horizontal overflow.
- All five mounted Chromium suites passed: `preview-queue-stability`,
  `preview-stability`, `all-media`, `character-media`, and
  `navigation-card-shapes`. Coverage includes metadata churn, list reordering,
  idle beyond the slideshow interval, explicit next/previous navigation,
  repeated/double-clicked opens, Escape/Back/button dismissal, source shapes,
  short windows, large relation lists and mobile views.
- A four-second VP8/WebM fixture decoded and played to completion in the native
  ScenePlayer on All, mobile Videos and Character All without advancing.
  Browser-local file/stream metadata and intercepted activity mutations kept
  the suite read-only. This is a codec fixture check, not verification of every
  production file, browser engine, physical device or GPU.
- Validation passed: all 28 existing UI unit tests, TypeScript, Biome lint and
  formatting, Stylelint, built-in composition/checksum and JavaScript syntax,
  production UI bundling and `git diff --check`. Node 24's ESM registration API
  ran the existing ts-node test suites; dependencies and schema artifacts were
  reused unchanged. Before/after relation captures were visually reviewed.
- No backend, GraphQL schema, migration or production data changes.
- Published as [PR #125](https://github.com/Dusky-dev/StashBooru/pull/125).
  Implementation SHA `e9dab826a3347d650cfa161b9a32d03e4d7da026` was verified on
  2026-10-01; the published tree matches the checked local tree exactly. CI is
  pending at publication. Merging and production playback remain separate.

## P07 preview opening and equal relation heights — 2026-10-02

Baseline: `a44f7ac7c55fff9210fd57b3e7218abd5ccc354a` (`develop`, PR #125).
Branch: `fix/p07-preview-open-flash-equal-heights-20261002`.

The owner still reported rapid Video flashes and requested Copyright image
frames match Variant height. This follow-up replaces the observer/placeholder/
portal preview bridge with a declarative native selected-media renderer.
Neighbours never mount a player, and navigation for custom media is immediate.
Native ScenePlayer and explicit navigation remain, with typed selected-ID
queries/links, fit-before-paint geometry, normal-size fitted controls, queue
ownership and metadata-refresh identity preserved. Video completion cannot
advance. Thumbnail requests abort on disposal/source change and ignore stale
responses; expected autoplay cancellation is handled.

Copyright and Variant image frames derive width from one responsive height,
retaining landscape 16:9 and portrait 3:4, borders, bounded relation scrolling
and scrollbar space. The opening regression records native player identity
and frame geometry from before the click, rather than checking the footer only.

Verified implementation published as `a0e8227100e78955a601d8c1ddbd494893730af5` (2026-10-02);
the published Git tree matches the checked local tree:

- All 28 UI unit tests passed; TypeScript, Biome lint/format, Stylelint,
  built-in composition/checksum/syntax and production Vite build passed.
- `preview-opening.mjs`: 12 cold/warm openings on All, mobile Videos and
  Character All rendered/mounted only the selected Video. First visible player
  geometry was stable (desktop 1440×810, mobile 390×219.375), fitted controls
  kept normal size, zoom/reset did not remount, delayed thumbnail setup worked,
  disposal aborted pending thumbnail requests, explicit arrows/footer cleanup
  passed, and no page errors were observed.
- `character-media.mjs`: Copyright/Variant frames both measured 202.65625 px
  high on desktop (widths 360.28125 / 152 px). Opposite-shape sources stayed
  contained; short viewport, 18+18 scrolling relations, mobile equal heights
  and no horizontal page overflow passed. Character All and Edit/Cancel
  cleanup passed. Before/after relation captures were visually inspected.
- `preview-queue-stability.mjs`: native-player identity survived 20–36
  metadata registrations/cache updates, idle, explicit navigation, reordered
  Character All and reopening on all three contexts. The native player decoded
  the four-second VP8 fixture to completion without advancing.
- `preview-stability.mjs`: 74 idle observations retained one selected player
  through another caller's Image refresh and resume-cache writes; completion,
  repeated/double opens, Escape, Back and close-button cleanup passed.
- `all-media.mjs` and `navigation-card-shapes.mjs`: loaded/empty/error/delayed
  queries, native controls, mixed previews, Video/Image cleanup, legacy route
  preservation, desktop/mobile navbar order and native card ratios passed.

These are automated mounted-browser fixture checks, not human manual testing.
VP8 playback was exercised; other production files/codecs, browser engines and
physical devices were not. Running `preview-opening.mjs` against the PR #125 production UI failed its
first-opening assertion for a transient incorrect footer. The exact reported
production multi-Video flash was not established in the fixture; its observer/portal selection path has been
removed. No backend, schema or migration changes. Published as [PR #126](https://github.com/Dusky-dev/StashBooru/pull/126)
from implementation `a0e8227100e78955a601d8c1ddbd494893730af5`. The tree SHA was checked
against the local result before branch publication. Fresh `origin/develop`
remains at the baseline; `git merge-tree --write-tree` reports no conflict.
Merging and production deployment have not been performed.

## P07 Video input and shared playback lifecycle — 2026-10-02

Baseline: `4adba688c14e276c85c21b6f8eeca909cd22ddf1` (merged #126; its
Build and Go lint workflows passed). Branch: `fix/p07-preview-lifecycle-20261002`.

The owner still reported buggy Video previews and supplied Firefox network logs.
The logs alone do not establish a switching trigger. Mounted tests reproduced:

- With Image lightbox `PAN_Y`, a burst of wheel events over the native preview
  mounted fixture IDs `1`, `2`, `3`, `4` instead of retaining `1`.
- An arrow key focused in ScenePlayer both sought within the Video and advanced
  its carousel.
- A preview over a Video detail page produced two `VideoJsPlayer` IDs,
  overwriting the native global registration.

Implemented input isolation, bounded custom-player panning, native-control wheel
handling, shared visible-page preview playback and screenshot fallback until a
decoded hover frame. Background grid/wall clips yield to the native viewer.
Preview players have separate IDs; a covered detail player pauses/resumes without
being disposed. Media Session metadata/actions restore to the remaining player
and clear when all players close. Prior captured queues, caller ownership,
selected-only rendering and equal Copyright/Variant image heights are preserved.

Verified implementation published as `cc9bddbd3d1aee3fa995a2633cb1998da89f8ea7`
on 2026-10-02, with Git tree `32a0d77539cfefdd92af3d20cb15f77bf9211f3a`
matching the checked local tree exactly:

- All 28 UI unit tests, TypeScript, Biome lint/format, Stylelint, built-in
  composition/checksum/syntax and production Vite build passed.
- `preview-input-playback.mjs` passed in Chromium 135 and Firefox 153 on All,
  mobile Videos and Character All: wheel bursts, held native seek keys and
  network refetches retained one selected native player; explicit navigation
  worked. Zoom-mode control sizes, All/Videos wall suspension/resume and
  delayed/failed hover screenshot fallback and pointer-leave pause passed.
- `preview-player-overlap.mjs` passed in both browsers: detail/preview IDs
  were distinct, the covered detail player paused and resumed without disposal,
  and restored Media Session pause/play actions reached the remaining player.
- All five existing mounted Chromium suites passed: `preview-opening`,
  `preview-queue-stability`, `preview-stability`, `all-media` and
  `character-media`. Cold/warm opens, metadata churn, native VP8 completion,
  idle/double opens, Escape/Back/button dismissal, mixed views and relation
  scrolling/mobile wrapping passed. Copyright/Variant frames retain equal
  desktop height (202.65625 px) and equal responsive mobile height.

These are automated mounted-browser fixture checks, not human manual testing.
Firefox used the execution environment's single-process mode with content
sandbox disabled. The fixture contains synthetic VP8 media and intercepts
activity mutations; no production metadata was changed. Production files,
other codecs, extensions and the exact trigger behind the supplied log remain
unverified. No backend/schema/migration change.

Published as [PR #127](https://github.com/Dusky-dev/StashBooru/pull/127).
Merging and production deployment have not been performed; CI is pending at
publication.

## P07 preview arrows and production flash capture — 2026-10-02

Baseline: `5f6e9e593bced10f432b690519914706196d8dff` (merged #127).
Branch: `fix/p07-preview-arrow-debug-20261002`.

The owner prefers preview arrows to navigate previous/next rather than seek,
and still reports flashing. Left/Right are now captured before native VideoJS
seeking, including when player buttons are focused; held repeats are consumed.
A held arrow remains consumed if navigation lands on an Image in the mixed
queue. Form/modified-key behavior and the full Video detail player remain native.

A passive console recorder can be pasted into the currently-installed build
before opening a preview. It records selected/player/node identity, native media
events/state, visibility/geometry, recent input and loaded asset filenames,
and downloads a bounded JSON trace. Titles, addresses and signed URL query
parameters are excluded. Capture instructions are in `p07-global-media.md`.

Verified on 2026-10-02:

- All 28 UI tests, TypeScript, Biome lint/format, Stylelint, built-in
  composition/checksum/syntax, recorder syntax, production Vite build and
  `git diff --check` passed.
- `preview-input-playback.mjs` passed in Chromium 135 and Firefox 153:
  focused player/button arrows moved once without outgoing seeking; held
  repeats stopped across Videos and a scoped mixed Video/Image queue. Full
  Video detail seeking remained native. Wheel/refetch identity, zoom control
  sizes, wall suspension/resume and hover fallback/cleanup passed.
- The same suite verified selected/player/node and input trace observations,
  downloaded JSON contents, recorder restart and no post-stop capture. Signed
  query/API-key values, server addresses and media titles were excluded.
- `preview-opening.mjs` passed all 12 cold/warm desktop/mobile opens with
  selected-only native rendering, stable fit geometry, normal-size controls,
  explicit navigation and disposal.

These are automated fixture checks with synthetic VP8 media. Firefox used
single-process mode with its content sandbox disabled. No human manual testing,
production codecs/files, third-party extensions or physical GPU rendering were
verified. No backend/schema/migration change.
This arrow preference correction does not claim to fix the remaining production
flash; the trace is intended to identify its actual trigger.

Published as [PR #128](https://github.com/Dusky-dev/StashBooru/pull/128), from
verified implementation `0476ae2d1edcb682bef3f00cb7d50093e6e82611` (2026-10-02).
Published tree `3ff61a60b0fc05c48a34eed112494ac13fd82796` matches the checked
local tree exactly. CI is pending at publication; merging and production
deployment have not been performed.

## P07 Video opening state and thumbnail ownership — 2026-10-02

Baseline: `7f96a326e768fde2660ae2ddca7baefcbfa077d5` (merged #128;
matches the owner's reported build `7f96a32`). Branch:
`fix/p07-player-opening-state-20261002`.

The owner clarified that other Videos or thumbnails flash before the requested
Video settles, including on its full detail page. The network log alone does
not identify a decoded-frame cause. Tests against the exact baseline source
reproduced these failures in Chromium:

- A delayed detail transition from Video `1` to `2` left the native stream and
  screenshot for `1` displayed under `/scenes/2`.
- An autostart preference update disposed the existing native player and
  started a replacement, although the plugin already supports in-place sync.
- A late VTT response for source `a` replaced the already-loaded source `b`.

The detail loader now uses only the query result matching its route, and each
Video owns a separate player lifecycle even on cache-hit transitions. Autostart
changes use plugin sync without native-player replacement. Sprite results are
owned by their VTT path, clear immediately on a path change, abort on source
changes/unmount, and handle unsuccessful reads. Scrubber items/width derive
from the current sprite result instead of retaining the previous list. Rapid
player disposal also exposed a persisted-volume storage callback dereferencing
a disposed plugin player; the callback now checks its captured owner.

The passive production recorder now includes route changes and poster/sprite
source revisions, visibility, geometry and sprite positions. Unchanged visual
states are deduplicated. Native stream identity and decoded-frame counters are
still recorded; titles, addresses and signed queries remain excluded.

Verified on 2026-10-02:

- `player-opening-state.mjs` passed in Chromium 135 and Firefox 153: slow and
  cached detail transitions at 1440/390 px, autostart sync in detail and preview
  players without resetting playback, out-of-order/cleared/failed VTT reads,
  and actual detail-scrubber clearing during a same-ID metadata refresh.
  The test serves native synthetic VP8 range requests on a direct `/stream`
  endpoint, with browser-only GraphQL/poster/VTT/activity substitutions.
- `preview-input-playback.mjs` passed in both browsers: arrows, held-key
  suppression, wheel/refetch stability, full-detail seeking, hover/wall
  ownership and fallback. The extended trace captured posters and omitted
  signed screenshot/stream queries, titles and addresses; restart/download
  contents and post-stop cleanup passed.
- `preview-player-overlap.mjs` passed in both browsers: the covered detail
  player paused/resumed without disposal, separate player IDs and Media Session
  ownership remained correct.
- Chromium `preview-opening.mjs` passed all 12 cold/warm All, mobile Videos
  and Character All opens with one selected native player and stable geometry.
  `preview-queue-stability.mjs` passed metadata churn, idle, explicit navigation,
  reopening and native VP8 completion without automatic advance.
- All 28 UI unit tests, TypeScript, Biome lint/format, Stylelint, built-in
  composition/syntax/unchanged checksum, recorder syntax, production Vite build
  and `git diff --check` passed. No dependency changes.

These are automated fixture checks, not human manual testing. Firefox used the
environment's single-process mode with its content sandbox disabled. The actual
production files/codecs, browser extensions and GPU rendering remain unverified.
The reproduced opening-state failures are corrected; this does not establish
that every production flash has the same trigger. If it persists, capture the
extended recorder JSON rather than only network-console messages. No backend,
schema, migration or production metadata/configuration change.

Published as [PR #129](https://github.com/Dusky-dev/StashBooru/pull/129), from
verified implementation `4cc91da2101d7169685b5115c7d76216c923976f` on 2026-10-02. Published Git
tree `e6e60285340be695463578748702cb99b488ec13` exactly matches the checked local
tree. [Build](https://github.com/Dusky-dev/StashBooru/actions/runs/37029906006)
and [Go lint](https://github.com/Dusky-dev/StashBooru/actions/runs/37029905279)
are running at publication. Merging and production deployment have not been
performed.


## Video autoplay/preview poster flash — 2026-10-02

Baseline: `8edd383cd464eac088347bdc9454c598f062bbb8` (merged #129).
Branch: `fix/video-thumbnail-flash-20261002`. The owner confirmed P07 complete
and requested this correction before P08.

The current player still assigned a screenshot poster during automatic startup.
The baseline Firefox fixture observed transient screenshot visibility; the
source assertion also consistently reproduced an assigned poster on an
automatic player while its stream response was held.

The shared native player now computes playback intent and clears its poster
before loading streams. Automatic metadata refreshes retain an empty poster;
manual playback retains its screenshot. A browser `NotAllowedError` restores
only the current, undisposed Video's poster and permits manual playback. Other
playback cancellations/errors retain the existing native behavior. Media
Session artwork, source failover, controls and preview ownership are preserved.
No backend, schema, migration or dependency changes.

Changed files: `ScenePlayer.tsx`, `tests/browser/player-opening-state.mjs`,
`docs/p07-global-media.md` and this ledger.

Verified on 2026-10-02:

- All 28 UI unit tests, Biome lint/format, Stylelint and source TypeScript checks
  passed with `TS_NODE_TRANSPILE_ONLY=true make validate-ui` under Node 24.
  The flag avoids the environment's ts-node loader type-check failures;
  `pnpm exec tsc --noEmit -p tsconfig.test.json` independently passed the test
  sources and their imported modules. The package manager was pnpm 10.33.0.
- `make generate-ui`, built-in composition/checksum/syntax, `make ui-only` and
  `git diff --check` passed. Generated GraphQL/build outputs remain generated.
- `player-opening-state.mjs` passed in Chromium 135 and Firefox 153: manual
  posters, held-stream automatic startup and native previews at 1440/390 px,
  frame samples and empty poster sources, pause/resume, simulated policy refusal,
  poster restoration after cache refresh and manual VP8 playback. Its existing
  slow/cached route changes, in-place autostart configuration and late/cleared/
  failed VTT/scrubber cases also passed. No page errors were observed.
- `preview-opening.mjs` passed in Chromium: 12 cold/warm openings on All,
  mobile Videos and Character All retained selected-only mounting, fitted
  geometry, controls, navigation and cleanup.
- `preview-input-playback.mjs` and `preview-player-overlap.mjs` passed in both
  browsers: wheel/refetch stability, focused/held arrows, mixed Image navigation,
  detail seeking, zoom controls, wall suspension/resume, hover fallbacks and
  distinct detail/preview identities with existing-player pause/resume.

These are automated browser-local GraphQL/stream fixtures with synthetic VP8
media and intercepted activity mutations, not human manual testing. Firefox
used single-process mode, software decoding and disabled content/RDD sandboxing
in the managed execution environment; production browser settings are unchanged.
Production files/codecs, extensions, physical GPU rendering and Safari were not
verified. Autoplay-policy refusal is simulated, not a claim about every browser
policy.

Published as [PR #130](https://github.com/Dusky-dev/StashBooru/pull/130), from
verified implementation `d975761316717be7abcb028a38756df766fbfb12` on 2026-10-02.
Published Git tree `815568abbf42fdc8f2ec533d80b4a2c0c8f4ee86` matches the checked
local tree exactly. The branch starts from the recorded fresh `develop` SHA.
CI is pending at publication; merging and deployment are not performed.
At PR #130 publication, P08 was the next package. Its completed implementation
and current verification are recorded below.

## P08 visual stacks / variant filmstrip — 2026-10-03

Baseline: fresh `develop` at `96309d8ffd66a141e94219a10b2ced7f2113b565`, with
PR #130 merged. Branch: `feat/p08-visual-stacks-20261003`.
The unavailable 2026-10-02 checkout was reconstructed from the package contract
and current source. An intermediate published checkpoint is
`7b8ac0263a6d58a0556c19f3f1452c737a1a03a2`, whose tree
`626b95e438d1257bad9874c8c791e1f891ef1903` matches the local implementation.

- Migration 96 stores ordered native Image/Video memberships, relationship labels,
  one representative and optimistic versions. Creation, add/remove, reorder,
  representative choice, split, merge and unstack all are atomic catalogue edits.
  Native deletion selects a survivor or cleans up an empty stack. File bytes,
  fingerprints, metadata, provenance, activation/restore and gallery order remain
  independent of grouping.
- All's optional Group stacks filter collapses before pagination after applying
  native filters. Any matching member can make a stack appear. Counts and expanded
  cards explain hidden matches and representatives outside filters. Selection
  keeps typed native IDs through representative changes; file actions target
  explicit individual members.
- Native details and previews show the ordered bottom filmstrip, current member,
  labels, representative, variant arrows and native member links. Only the
  selected Video owns a native player, and completion never advances variants.
  The original preview caller and PR #130's automatic empty-poster policy survive.
  Browser checks corrected viewport positioning of expanded members and a cache
  identity collision between equal numeric Image/Video IDs.
- Proposals require review of independent pairs using active MD5, recorded source
  MD5, or pHash plus current configured EVA02 evidence. Stale/missing evidence is
  skipped. There is no automatic/transitive clustering or inference/model download.
  Stacks are bounded at 200 members, proposal selections at 100, and rendered
  proposals at 50 per page.

Verification on the final implementation:

- `make generate-backend` and `make generate-ui` passed.
- Focused P07 compatibility and seven P08 integration tests passed. Coverage
  includes actual bytes/fingerprints, native metadata and gallery order; typed
  identity; stale versions; failed split/merge rollback; representative deletion;
  any-member filtering and pagination; current pair evidence and no transitive
  joining; and fresh schema 96 / upgrade from schema 95.
- Full `make test`, `make it` and `make lint` passed, with zero Go lint issues.
- `TS_NODE_TRANSPILE_ONLY=true make validate-ui` passed all 30 UI tests, Biome,
  Stylelint, source TypeScript and formatting under Node 24 / pnpm 10.33.0.
  `pnpm exec tsc --noEmit -p tsconfig.test.json` independently passed test types.
- `make ui-only`, the Linux server build, built-in composition/checksum/syntax
  and `git diff --check` passed. Generated outputs remain generated; built-in
  source and dependency locks are unchanged.
- The new visual-stack suite passed at 1440×1000 and 390×844 in Chromium 135 and
  Firefox 153: create, collapse, hidden-member selection, ordered Image/Video
  previews, completion staying selected, editor keyboard isolation, stable
  selection after representative changes, native detail URLs, reorder, add,
  split, merge, remove and unstack. Native snapshots were unchanged.
- Existing preview-opening checks passed 12 cold/warm Chromium cases. The
  player-opening-state, preview-input-playback and preview-player-overlap suites
  passed in both browsers, including held-stream poster clearing, delayed/failed
  thumbnails, wheel/refetch stability, focused/held arrows, detail seeking, zoom,
  wall suspension/resume and independent player disposal/pause/resume.

These are automated synthetic PNG/VP8 fixtures with browser-local activity
interception. Firefox used single-process software decoding with disabled
content/RDD sandboxing in the execution environment. Production media/codecs,
Safari, extensions, physical GPU rendering and real EVA02 inference were not
verified. Removing the originally opened member while a preview is showing
another variant falls back to the original native item. Representative changes
preserve the selected variant while the opened member remains grouped.

Published as [PR #131](https://github.com/Dusky-dev/StashBooru/pull/131). Verified
source commit: `3092dfe872345effa80113aa424796228453d28d`, with Git tree
`710ffb0cd39715707a678d73d22234ccfb5ca581`, matching local commit
`9ebe2085195b10c0c17596cc17308813f8c326df` exactly. Publication preserves the
intermediate checkpoint's ancestry. This final ledger update changes docs only.
Remote CI subsequently passed for published head
`04326bbb973cc44b8e4dc5085cf077fbb4329a55`; results were confirmed on 2026-10-04:

- [Build](https://github.com/Dusky-dev/StashBooru/actions/runs/37137377976):
  all ten jobs succeeded, including generation, tests, the seven-platform build
  matrix and the release job.
- [Go lint](https://github.com/Dusky-dev/StashBooru/actions/runs/37137377950):
  the lint job succeeded.

The resumed checkout matches that published head and fresh `origin/develop`
still matches the recorded baseline. PR #131 is open and mergeable, with no
review threads at this check. The source, migration, filmstrip and existing
acceptance coverage were inspected; no implementation changes or additional
runtime tests were needed. This follow-up changes documentation only and was
reviewed with `git diff --check`. Earlier synthetic-fixture limitations remain.
Merging and production deployment are not performed. Next package: P09
conversion trial/threshold workflow. PR #131 was merged before P09 started,
producing the current baseline recorded above.


## P09 conversion review, saved trials and savings thresholds

Starting baseline: merged P08 `2e00454f3b063b729ce98e61afb69916cc44a5d8`.
PR #132 reached develop during publication, producing
`6240a022b74e121174756f8b0a33f6df05e455f9`. Its two native visual-stack editor
UI files merged cleanly; the combined UI is reverified. Work is published in
[PR #133](https://github.com/Dusky-dev/StashBooru/pull/133) on
`feat/p09-conversion-review-20261004`.

- The native converter separates stratified estimate, verified trial and apply.
  Estimates sample up to 24 complete encodes, report coverage/observed uncertainty,
  and delete temporary results. Trials retain measured output separately from
  original restore entries without changing active bytes/fingerprints/metadata.
- Per-file review shows effective format/options, exact bytes and savings, encoder,
  elapsed time, verification/status, skipped reasons, typed links, expiry and
  animation/alpha/audio/profile limitations. Still comparisons reuse P03 controls.
- Global and per-job minima default to zero. Positive savings and both enabled
  inclusive byte/percent minima must pass after verification and before any backup
  or activation. Keep-larger cannot bypass compression policy; intentional upscaling
  remains separate. Potential, applied, skipped/failed and backup usage are labelled.
- Saved apply rechecks primary-file ownership, source identity/SHA-256/measured
  timestamp, normalized options/policy, worker/codecs signature and output checksum.
  It activates the saved bytes through the existing restore/recovery journal.
- Trial cache defaults to 10 GiB / 24 hours, with configurable size/1–168 hour TTL,
  cancellation cleanup, restart recovery and conservative peak-disk planning.
  Remote retained trials need the updated converter script and worker restart.

Verification on 2026-10-04:

- Focused acceptance tests and a native SQLite catalogue-preservation test passed.
  Tags/Characters/Artists/Copyrights, gallery order, visual stack, media/file IDs,
  metadata/provenance and exact restored bytes/fingerprints remain intact.
- Full `make test`, `make it` and `make lint` passed; zero Go lint issues.
- `TS_NODE_TRANSPILE_ONLY=true make validate-ui` passed 30 tests, JS/CSS lint,
  source TypeScript and formatting; test TypeScript independently passed.
- Backend/UI generation and production `make ui-only` passed; built-ins and locked
  dependencies are unchanged. Windows/macOS converter tests cross-compiled.
- Python suite: 27 passed, 5 availability skips (32 total), including actual available
  CPU codec fixtures and worker identity coverage. Physical GPU remains unverified.
- Chromium 143 passed 1440×1000 / 390×844 native component workflows: estimate,
  trial rows, skipped eligibility, comparison/parent isolation, apply, discard,
  global minima/cache/TTL and reopen. Screenshots are in the package notes.
- `git diff --check` passed. Synthetic browser/API/store/database fixtures are
  distinguished from real Python codec execution; no manual owner test is claimed.

See [P09 package notes](p09-conversion-review.md) for API/lifecycle details and
remaining limits. Published implementation commit:
`17b4d4a355b022609faa0b8499187295debbac9e`, Git tree
`5e39f9deba173a8fef2abf5a94dd819c4757a925`, exactly matching locally verified
commit `d88c07690cb373f400287dc2f4ae48980393929f`. The subsequent compatibility
merge preserves both this feature commit and fresh develop ancestry, and updates
these publication notes. GitHub CI results are tracked on the PR; local results
above are confirmed. On the initial published source, [Media converter](https://github.com/Dusky-dev/StashBooru/actions/runs/37159657387)
and [Go lint](https://github.com/Dusky-dev/StashBooru/actions/runs/37159657220)
already passed; Build was in progress at the check. Compatibility UI validation,
production bundling and both browser sizes also passed after incorporating #132.
Next package: P10 robust Video duplicate and interval matching. No merge or
production deployment is performed.

## P10 Video overlap / containment review — 2026-10-04

Implemented on `feat/p10-video-overlap-20261004` from merged P09 `develop`
`57443b7a8773de267cad81ba80c461fc8363a15e` (PR #133). Fresh develop was checked
again before publication and remains at that baseline.

- Versioned actual-PTS frame sampling handles tested VFR, display rotation,
  resolutions and symmetric black bars. Lightweight pHash/dHash/RGB signatures
  downweight blank/static/title frames; SHA-256 remains the exact-file proof.
- An owned, rebuildable SQLite inverted index retrieves candidates before
  bounded monotonic offset alignment, without excluding short-versus-long pairs.
  Exact, near-complete visual, contained, partial and compilation classes retain
  both ranges, offsets, gaps, coverage, tolerance, evidence grades and audio limits.
- Native Settings Tasks and Video detail actions open paged review with typed
  links and configurable keeper suggestions. Explicit synchronized two-stream
  playback supports segment selection/audio audition and cleans up on dismissal.
  Stale results are blocked. Partial/contained/compilation reviews preserve both;
  no delete/merge/convert action is introduced.
- Native Jobs persist atomic per-file checkpoints, failed IDs and captured ID
  bounds. Incremental skip, restart/resume, cancelled failed retries, lost native
  job-history entries and explicit remote-only selection are handled. Source,
  decoder/settings and exact bytes are revalidated. Catalogue metadata, native
  relationships, fingerprints, file activation and conversion backups are intact.
- Full `make test`, `make it`, and `make lint` passed; Go lint reports zero issues.
  The initial concurrent lint attempt exhausted compiler memory; its serial rerun
  passed after resolving actual lint findings. No failing check remains.
- `make test-video-overlap` passed all 14 labelled actual-codec cases and indexed
  short-in-long retrieval. Expected offsets passed the ±1.25 s default tolerance.
  Shared titles, black frames and unsupported 1.5× speed changes produced no
  visual match. The fixture/results table is in [P10 notes](p10-video-overlap.md).
- The combined Python converter/upscaler/sampler suite passed 38 tests with five
  JPEG XL availability skips (43 total). Authenticated remote sampling exercised
  actual FFmpeg decoding; heavyweight model imports were stubbed. No models were
  downloaded and physical GPU inference is not claimed.
- UI validation passed all 33 tests, JS/CSS lint, TypeScript and formatting;
  separate test TypeScript and `make ui-only` production bundling passed. Built-in
  sources/checksums and locked dependencies are unchanged.
- Chromium 143 passed desktop 1440×1000 / mobile 390×844 mounted native-component
  flows with mocked review API and actual synthetic MP4 streams: settings/jobs,
  remote-only payloads, resume/cancel, paging/links/preferences, repeated opens,
  no autoplay, mapped playback/pause/audio, cleanup and stale/empty/error/limits.
  Screenshots were inspected. Native SQLite tests independently cover real
  catalogue/checkpoint behavior; no full production-library or manual owner test
  is claimed. The Windows sampler/process package cross-compiled; Windows runtime
  and SQLite execution are unverified.

Remote workers require the P10 sampler/server update and restart. Results remain
sample-based; static material, arbitrary crops/overlays, very short intervals,
speed normalization, repeated reuse of a range, full audio interval alignment and
real-library recall calibration are outside this first version. Common-posting,
candidate and alignment limits are disclosed. The derived sidecar is rebuilt
after native catalogue restore/import; it is not a new backup contract.

Publication commit/PR verification is recorded below. P11 remains unstarted;
merge and production deployment are not performed.
