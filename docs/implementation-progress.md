Warning: truncated output (original token count: 20643)
Total output lines: 1174

# StashBooru implementation progress

Updated: 2026-10-05

Current baseline: `develop` at `5162463028e69420f32c7f5ff629bb36b9057008`
(merged PR #139). P01–P13's listed implementation/prototype packages and the
three-item Jobs/stack QoL batch are merged. P11 model/GPU acceptance and P12/P13
production activation gates remain open. Current package: owner-requested
maintenance and independent StashBooru 1.0 release on
`maintenance/features-ui-settings-20261005`.
Published as [PR #140](https://github.com/Dusky-dev/StashBooru/pull/140).
See [maintenance notes](maintenance-2026-10-05.md), [feature guide](features.md)
and [release procedure](releases.md). GitHub publication/check results follow below.

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
| P10 — Video overlap / containment | Complete; PR #134 merged | Versioned PTS samples, indexed retrieval, exact/near-complete/contained/partial/compilation evidence, durable index checkpoints and synchronized segment review. See [P10 notes](p10-video-overlap.md). |
| P11 — mask-based image restoration | Merged; PR #135 | Final CI and nine local restoration tests rechecked. Native mask editor/adapter, reviewed new derivative and provenance; actual model/GPU validation remains. See [P11 notes](p11-image-restoration.md). |
| P12 — image delta storage | Offline prototype merged; PR #136 | Byte/pixel contracts, measured base selection and fallbacks, full accounting, self-contained pack/verify/extract/recover. 22 tests and 20 fixture runs passed locally and in the new CI workflow; production activation and owner-stack calibration remain gated. See [P12 notes](p12-image-delta-storage.md). |
| P13 — Gallery shared blocks | Offline prototype merged; PRs #137/#138 | #137 initially merged into the P12 feature branch; #138 landed the unchanged tested P13 tree on develop. Production activation remains gated. See [P13 notes](p13-gallery-block-storage.md). |
| QoL — Jobs and stack review | Complete; PR #139 merged | Retained failed jobs and Copy error; representative/compare shortcuts; touch/mouse draft reordering with up/down controls. See [QoL notes](qol-jobs-stacks.md). |

## Maintenance and StashBooru 1.0 — 2026-10-05

- Dated implementation verification: GitHub commit
  `9d2c20ade1cb2a7b38566736d712d9e70c381694` has tree
  `ae8e6016d7fc266e6e69762c053c4a8906011018`, identical to locally verified
  `abb87c4be4574c71ca62c56a2d6060c9dd53a89a`. Baseline remains
  `5162463028e69420f32c7f5ff629bb36b9057008`; no agent merge or deploy.
- Processing settings/category, scope-specific saves, draft-preserving refresh,
  old-bookmark redirects, native theme styling and the feature guide are complete.
  Duplicate saves and misleading help were removed. Compatibility-sensitive fields
  remain, with explicit retirement recommendations in the maintenance notes.
- StashBooru 1.0.0 has an independent Go/Vite version source, About display and
  stable fork update checking. Release target is
  [stashbooru-v1.0.0](https://github.com/Dusky-dev/StashBooru/releases/tag/stashbooru-v1.0.0).
  Publication uses the exact tested release-branch commit and the guarded Build
  workflow; it does not require or perform an agent merge into develop.
- Local verification: 47 UI tests plus lint/TypeScript/format checks, production
  bundle, backend generation, focused Go version tests, five release-guard tests,
  standalone fixture typing, and four desktop/mobile mounted browser suites passed.
  [PR #140 checks](https://github.com/Dusky-dev/StashBooru/pull/140/checks) record the
  authoritative full Go/lint/platform verification for each published revision.
- This following documentation-only checkpoint records the published implementation
  SHA/PR. Feature scope is frozen; P11 model/GPU acceptance and P12/P13 production
  activation remain explicit gates. Presets and CSV/JSON exports remain deferred.

## Jobs and stack QoL — 2026-10-04

- User approved the first three QoL suggestions after P01–P13: failed jobs stay
  until dismissed with Copy error, quick representative/comparison actions, and
  drag reordering with existing keyboard controls retained. Presets and conversion
  CSV/JSON exports remain deferred.
- Started from P12 develop `cf2c9fdb38d756797f587f5ddbbba36d8d034281`, then rebased
  onto fresh `a061a22cbbd3c8fee30dd14e1b5c13e45326d984` after the owner merged #138.
  The baseline tree is `538a8e8ae00e192e926139404f898f70a9471cf8`, exactly the
  previously verified final P13 tree. No PR was merged or deployed by the agent.
- Application-wide job monitoring retains failures in this tab through navigation
  and refresh until individual/all-failure dismissal. Job ID plus creation time
  separates server restarts. Stale in-flight queries cannot overwrite events or
  resurrect dismissed/expired jobs. Successful/cancelled jobs still expire after
  ten seconds. Copy preserves full error text and handles clipboard rejection.
  This is tab history, not durable server history while the app is disconnected.
- Stack shortcuts use existing typed, version-checked mutations and native shared
  still-image comparison. Order/labels/active member persist, stale writes reject,
  and caller switches/dismissals dispose cache observers and pending local updates.
  Reordering supports captured mouse/pen/touch handles, edge scrolling, live
  announcements, Escape/outside cancellation and existing up/down controls.
  The draft remains local until Save.
- `pnpm run gqlgen` passed. `make validate-ui` passed **42 tests**, JS/CSS lint,
  TypeScript and formatting with Node 20.20.1/pnpm 10.33.0, matching CI's Node 20
  major and the repository lock. `make ui-only` passed production bundling and
  built-in composition. There is no backend schema, migration or dependency-lock
  change; the subscription requests the already available job `addTime` field.
- Mounted Chromium **143.0.7499.0** passed 1440×1000 desktop and 390×844 mobile,
  including real browser touch events, keyboard reorder, cancellation, draft/save,
  typed equal IDs, retained labels/representative, stale-version rejection, late
  snapshots, expiry, navigation/refresh/dismissal, clipboard fallback/failure,
  active-caller changes and closing during an unresolved mutation. One viewer,
  no unmounted updates and no horizontal page overflow were asserted. The isolated
  Apollo fixture also passed a standalone TypeScript check. These are synthetic
  automated fixtures; no human manual, physical-device or real-file decoding
  validation is claimed.
- Dated verification: final application-code local commit
  `dfa2f4c8b0985408f82ff8bb57d248c5633d4ccd` has tree
  `fda0a7fb4ce6f7a2516f0ab770d40fa4052acad2`. The following commit adds validated
  fixture metadata, screenshots and documentation. Published implementation
  `b7b0cacd63101f7fe0d905dbe486a404951b69af` has tree
  `6048b341f418d8aa744339200504856a872b6216`, exactly matching local documentation
  head `d8c684019bd81b843d2cdc1b299c842affec7ada`. PR #139 targets develop directly;
  GitHub checks are tracked on its exact head. This following publication-note
  commit changes documentation only.
- Next: review and merge this focused QoL PR after checks, then maintenance and
  cleanup. Physical P11 inference and native P12/P13 production/recovery integration
  remain separately scoped acceptance work.

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
- All seven platform builds passed: Linux, Linux ARM64/ARMv7/ARMv6, Windows, FreeBS…10643 tokens truncated… from merged P09 `develop`
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

Published as [PR #134](https://github.com/Dusky-dev/StashBooru/pull/134). Verified
implementation commit `9bcd1a380dd2a70cfce49196224afde9abe82335` has Git tree
`4d737177947ec52a227038b1fd398321a4d426ab`, exactly matching locally checked
commit `7f59d3ee8fddbe65065246437392bb60bc96fd32`. Fetch/diff confirmed that tree
and preserved merged P09 ancestry on 2026-10-04. This subsequent publication-note
commit changes documentation only.

GitHub [Build](https://github.com/Dusky-dev/StashBooru/actions/runs/37200848871),
[Go lint](https://github.com/Dusky-dev/StashBooru/actions/runs/37200848905) and
[Media converter](https://github.com/Dusky-dev/StashBooru/actions/runs/37200848862)
were in progress at the publication check; local results above are confirmed.
That was the P10 publication checkpoint. On 2026-10-04, final head
`c506506f2149c80cb936839b272778d7ab03a4e9` was rechecked: [Build](https://github.com/Dusky-dev/StashBooru/actions/runs/37201043858),
[Go lint](https://github.com/Dusky-dev/StashBooru/actions/runs/37201043781) and
[Media converter](https://github.com/Dusky-dev/StashBooru/actions/runs/37201043962)
all passed. PR #134 is now merged as `c63027cf995304614a2e42c31c41d7fecbcdbc99`.
P10 is complete; its documented sampled-evidence limits remain applicable.

## P11 masked still-image restoration — 2026-10-04

Verified local implementation commit `083704bb779a69237e028fad7b915ab9b8f0e7da`
has tree `8127cf65a4f9e867c3c4e820f09ba0d870d85a53`. The following documentation
update records that checkpoint; no implementation changes follow its checks.

Based on fresh merged P10 `develop` at
`c63027cf995304614a2e42c31c41d7fecbcdbc99`; branch
`feat/p11-image-restoration-20261004`. P10's server endpoints merged cleanly into
this branch before final validation.

- Image Operations opens brush/erase/box/clear, feather, zoom/pan and bounded undo.
  The exact effective mask is visible. Preview/settings changes invalidate Save;
  P03 comparison is reused. Optional reference supplies starting pixels only.
- A separate offline SD1.5 nine-channel inpainting adapter negotiates model/revision,
  hardware and bounds through existing authenticated worker transport and Jobs.
  Model installation is explicit. CPU needs an opt-in; GPU-only/remote-only are
  honored. CUDA work is bounded to a 512×512 crop, not arbitrary source resolution.
- Final compositing retains every unmasked canonical RGBA pixel and all alpha.
  Orientation/ICC/embedded-metadata and crop-resampling behavior is disclosed.
  Generated pixels are labelled as plausible restoration, not recovered originals.
- Save copies exact reviewed PNG bytes to a new native Image/file. Source bytes,
  IDs and primary files remain intact. Direct metadata, all Artists, Characters,
  Copyrights, Tags, URLs, custom fields and Gallery links survive; an optional
  stack append preserves existing members/order/labels/representative. Raw and
  effective masks, reference and provenance remain with the saved journal.
- Checks passed: backend generation; full `make test`, `make it`, `make lint`;
  35 UI tests, JS/CSS lint, source/test TypeScript, formatting and production
  bundling; nine Pillow/authenticated-HTTP restoration tests; Windows processing
  package cross-compilation. The merged worker suite passed 46 tests with five
  JPEG XL skips before adding the separately verified soft-mask edge case.
- Native tests cover exact preview/final agreement, unchanged originals, native
  metadata/relationship retention, new/existing mixed-media stacks, stale inputs,
  corrupt preview and transactional rollback. Go/Python cover bounded archives,
  transport verification, finite feather blending, CPU/GPU rejection, missing
  models, cancellation cleanup and preview pruning.
- Desktop/mobile Chromium 143 (1440×1000 / 390×844) passed mounted native controls,
  reference upload, comparison, dirty preview, save/link, missing-model state,
  repeated opens, cancellation and late-response cleanup. API processing is mocked; screenshots were
  inspected. Real diffusion/PyTorch are stubbed in automated adapter tests.

No native migration, dependency-lock change, model download, actual diffusion or
physical RTX 3060 test is claimed. Target-worker acceptance still needs a bounded
512×512/20-step run with measured VRAM/time and seam inspection. Deployment and
merge are separate owner actions. [P11 notes](p11-image-restoration.md) include
explicit installation, protocol, limits, journal and preservation behavior.

Next package: P12 image delta-storage benchmark and non-destructive prototype.
P12 is not started by this request.

P11 was published as [PR #135](https://github.com/Dusky-dev/StashBooru/pull/135)
at `57eead67c6381e87caa9f04aa4b1a46132a30d2b`, preserving merged P10 ancestry.
Its tree `12f8890e7de76748246887ac95a9168555b8f5b2` matches locally verified
`92eea899ac1290433bc22de6b1b448bc5f243590` exactly. Fetch/diff confirmed the
publication on 2026-10-04; these subsequent notes are documentation only.
GitHub CI status is tracked on the PR separately from confirmed local checks.

## P11 recheck and P12 offline storage prototype — 2026-10-04

Fresh `develop` baseline is `03f3bd88969f8b04414dca7781ddd1c96bf53c93`,
the merge of P11 PR #135. Its final head
`a0273de1d65606426ee1b1846ee0cb3a3cc026ef` was rechecked: Build (generation,
test and all platform builds), Go lint, Media converter and Image restoration
all passed. The nine restoration tests passed locally again. Actual model/GPU
inference, VRAM/time measurement and seam inspection remain open.

P12 is implemented on `feat/p12-image-delta-prototype-20261004`. This is the
bounded first benchmark/recovery prototype, with original files retained:

- Offline `benchmark`, `pack`, `verify`, `extract` and `recover`; byte-exact is
  default and pixel-exact is explicit. No catalogue, native relationships,
  source bytes, file activation or application worker protocol is changed.
- Every bounded base is compared with independently stored files/representations.
  Complete unique blobs, manifests, indexes and required metadata sidecars are
  counted. Both delta minima and actual net savings are enforced; losing
  deltas retain independent storage. Display representative and base are separate.
- Exact full-stream/plane residuals preserve every low-level, alpha, transparent
  RGB, border and target-canvas value. Integer prediction is versioned; no
  heatmap threshold establishes equality. Multichannel16 and unsupported pixel
  decoders reject rather than silently truncate; byte mode retains all bytes.
- Self-contained SHA-256 blobs/manifest/index support one member plus at most
  one independent base. Portable exports keep source and export fingerprints
  separate. ICC/Exif/PNG metadata and opaque JPEG APP/COM data are preserved
  under the documented embedding/sidecar contract.
- Durable verified staging, atomic no-overwrite publication, interrupted-stage
  recovery, independent extraction after originals are removed, per-member
  corrupt/missing-dependant diagnostics and bounded hostile-input validation.
  A hard-killed extraction can restart into a new directory from the intact pack.

Final local verification:

- `python3 -m unittest discover -s scripts/tests -p 'test_image_*.py' -v`:
  **31 passed**, comprising 22 P12 acceptance/recovery/CLI tests and nine P11
  adapter tests. Restoration diffusion/PyTorch remain stubbed; actual Pillow,
  NumPy, authenticated HTTP and process-exit recovery executed.
- `python3 scripts/tests/image_delta_fixtures.py --output <new-report.json>`:
  **20 verified** group/contract runs. The checked report is
  `docs/benchmarks/p12-image-delta-20261004.json`; fixtures use repository artwork
  derivatives plus seeded random/high-bit-depth data, not owner-library stacks.
- Localized-edit fixture: originals 644,143 B; pixel-exact shared-base archive
  255,007 B (about 60% savings); byte-exact independent archive 645,764 B.
  Recompressed JPEG/resolution cases demonstrate real no-delta fallbacks.
  Physical prototype storage includes retained originals as a separate total.
- `make test-image-delta` passed its acceptance and fixture checks before the
  final CLI/provenance accounting refinements; the final combined suite and
  benchmark above cover those refinements. The Makefile's existing Go-env probes
  print missing-Go diagnostics in this container; the Python target succeeds.
- Python syntax/AST and CI YAML checks passed; no unused Python imports were
  found; `git diff --check` passed. No native schema/GraphQL/UI or locked
  dependency changes; local Go/UI gates were not rerun for this offline package.
  The new Image delta storage CI workflow tests recovery and publishes its report.

See [P12 notes](p12-image-delta-storage.md) for commands, archive format,
exact-byte results, consumer inventory and the production gate. Owner-stack
calibration (including JXL), cross-platform/version runtime, native logical reads,
live-base GC, concurrent cache and journalled application activation/backup/restore
remain separate before any original eviction. No production activation occurred.

Next roadmap prototype is P13 bounded Gallery block sharing. P13 is unstarted.

Published as [PR #136](https://github.com/Dusky-dev/StashBooru/pull/136).
Verified implementation commit `e85e1e6e44746e59afe4e95514255dead5ee9ce3`
has Git tree `0af2dec7e3e6c582a171e354cb0a69f510b8656f`, exactly matching
locally verified `89f477a7c2bb643753eec94780c00780cf85ca64`. Fresh develop was
still the merged P11 baseline before publication. The branch was fetched and
diff/ancestry checked; this following update changes publication documentation
only. Merge and deployment remain separate owner actions.

On the implementation head, the new [Image delta storage prototype workflow](https://github.com/Dusky-dev/StashBooru/actions/runs/37219023641)
passed all 22 acceptance tests and the 20 measured fixture contracts, and
published its JSON artifact. Build, Go lint and Media converter were still
running at that check. Final-head CI is tracked on the PR separately from the
confirmed local/implementation-head results.

## P12 completion check and P13 Gallery sharing prototype — 2026-10-04

P12 PR [#136](https://github.com/Dusky-dev/StashBooru/pull/136) final head
`7023c9f812f1ecb9184193b3480a5c8790f61357` was rechecked: all Build jobs,
Go lint, Media converter and Image delta storage passed. P12 is complete as its
specified non-destructive prototype. It remains open/unmerged; fresh `develop`
is still merged P11 `03f3bd88969f8b04414dca7781ddd1c96bf53c93`.

P13 branch `feat/p13-gallery-block-prototype-20261004` starts from that exact
remote P12 head (tree `f8c6899c05da8e7df7c824dbb73227ec53433286`). Review targets
the P12 branch so only P13 is shown. After P12 merges, retarget P13 to develop
before its separate owner-approved merge; no merge or activation was performed.

- Bounded per-Gallery or explicit-group offline `estimate`, `benchmark`, `pack`,
  `verify`, `diagnose`, `extract`, `append`, `remove`, `repair` and `recover`.
  Byte-exact is default; pixel-exact explicitly retains the P12 canonical plane,
  alpha/hidden-RGB, uint16, ICC/Exif/opaque metadata and fingerprint contracts.
- Compares fixed byte chunks 4/16/64 KiB, Gear-64 content-defined target sizes
  8/32/64 KiB, exact tiles 32/64/128 and a full independent competitor. All
  candidates reconstruct exactly; inclusive byte/percentage minima apply to
  complete sizes against independent archives and originals, with ordinary
  lossless export/sidecar costs included. Unprofitable sharing falls back.
- Self-contained ZIP_STORED packs contain independent per-image manifests,
  a versioned Gallery/chunk index and immutable cryptographically addressed
  payloads. Chunk addresses include the codec/decoded contract, avoiding raw
  versus zlib interpretation collisions. IDs, original filenames and reading
  order survive; a member directly references only its own chunks.
- Physical bytes count each chunk once, including every manifest and ZIP index.
  Per-image attribution is labelled allocation and sums to the complete pack.
  Initial metadata access and selected-member chunk costs are measured;
  retained sources/previous revisions, cache and extra backups are separate.
- Append prepares only new sources and copies old compressed chunks unchanged;
  remove traces survivors without losing referenced chunks. Both verify and
  publish an immutable parent-linked revision of only that Gallery, retaining
  old revisions and leaving unrelated Galleries intact. A revision's net-source
  savings gate is disclosed; global reoptimization is a separate whole estimate.
- Durable private staging and atomic exclusive publication, actual process-death
  recovery, independent empty-environment export, per-corrupt-chunk dependant
  diagnostics, and repair from explicit full-hash-matching originals only.
  Hostile paths/indexes, duplicate manifests, canvas/reference bounds, FIFO,
  ZIP directory and zlib bombs reject. No approximate repair or source eviction.

Final local verification:

- `python3 -m unittest discover -s scripts/tests -p 'test_*storage.py' -v`:
  **47 passed** (25 P13 acceptance/recovery/CLI tests + 22 P12 regressions).
  Actual subprocess exits execute at five pack stages, unpack and revision;
  P13 tests also cover independent one-image I/O, ownership after removal,
  append without old sources, metadata/high-bit depth and true threshold limits.
- `python3 scripts/tests/gallery_block_fixtures.py --output <new-report.json>`:
  **26 verified** Gallery/contract runs (13 labelled cases). Checked report:
  `docs/benchmarks/p13-gallery-block-20261004.json`. It records exact overhead,
  runtime versions, all block sizes and reconstruction timings. This is repo
  artwork/seeded fixture data, not owner-library Gallery calibration.
- Localized edits: originals **644,143 B**, byte archive **647,573 B** (independent),
  pixel tiles **288,139 B**, about 55% below originals. Byte-prefix insertion
  control: **590,917 B → 237,305 B** with CDC 32 KiB; it deliberately uses
  uncompressed PNGs. Noise, boundary shifts, resized/JPEG and small flat-page
  cases demonstrate measured independent fallbacks.
- Python syntax/import-use and CI YAML checks passed; exact physical ZIP sizes
  and allocation sums are asserted in acceptance. `git diff --check` passed.
  No Go/UI, schema/GraphQL, model or dependency-lock changes. Local Go is absent;
  application checks are tracked on GitHub. New Gallery block CI runs its
  acceptance/fixtures and uploads the JSON benchmark.

See [P13 notes](p13-gallery-block-storage.md) for runnable commands, format,
recovery, revision semantics, full accounting and production gates. Real Gallery
and JXL codec measurements, supported-platform/version conformance, native
logical-media reads, catalogue activation/rollback, live-reference GC, concurrent
cache and application backup/restore remain open. P13 completes the last listed
roadmap prototype; production replacement of originals is a separate milestone.

Published as [PR #137](https://github.com/Dusky-dev/StashBooru/pull/137), targeting
P12's feature branch. Verified implementation
`262d59d9843dcb9e53163ac88e83f1bd213d64ef` has tree
`8b11972f1571db24772b203d9d7e07df2ea9d0f5`, exactly matching locally tested
`5b600ca1ec15a2598ed078887ec7eb22ff15709b`. Fetch, empty tree diff and explicit
ancestor checks confirmed both P12 and fresh merged-P11 develop ancestry on
2026-10-04. This subsequent publication-note commit changes documentation only.

On the implementation head, [Gallery block storage](https://github.com/Dusky-dev/StashBooru/actions/runs/37222868235)
passed its 25 acceptance tests, 26 measured fixture contracts and benchmark
artifact publication on Ubuntu. Media converter and backend generation also
passed. Native tests, seven platform builds and Go lint were still running at
that check. Final-head checks are tracked on the PR separately from these
confirmed local and implementation-head results. Those were the original
publication statuses. P12 #136 subsequently merged into develop; #137 merged into
P12's branch, and owner-merged #138 landed the same P13 tree on develop. No merge
or deployment was performed by the agent. The current checkpoint above supersedes
these historical CI snapshots.
