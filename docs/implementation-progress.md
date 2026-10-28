# StashBooru implementation progress

## Read-only P12/P13 image storage candidate comparison — 2026-10-28

- New `feat/readonly-image-storage-estimator-20261028` from `develop`.
- Combine existing offline P12 (base/delta) and P13 (shared blocks/tiles)
  into one per-stack read-only estimate at a selected fidelity.
- Compare complete archive byte totals, net savings versus separate originals,
  and actual physical footprint when original files are retained.
- Never write packs, activate metadata, overwrite originals, or delete data.
  Optional JSON report uses create-only semantics; group errors are explicit.
- Dedicated tests and CI for real algorithm planning with synthetic edited PNGs,
  untouched original checks, threshold validation and no-overwrite reporting.
- **Still offline**; native library selection/Settings estimate and live storage
  activation are later steps. CI and physical library calibration pending.

## Native original-media backup task — 2026-10-28

- Based on the passing #250 offline snapshot branch at
  `70c199bfdb9985e5a46507999dde68fe158f7e55`;
  follow-up branch `feat/native-media-backup-task-20261028`.
- New `GET/POST image/media-backup` task endpoints expose configured media
  roots, a saved destination, recent snapshots, a single active job and
  explicit backup/verify/restore actions. Server derives roots from current
  Stash config; browser paths cannot select arbitrary backup sources.
- Configuration is stored in the StashBooru config directory with restricted
  permissions. Reject nonexistent, relative, same-root, nested and
  symlink-alias destinations. Operations run with cancellable native jobs
  and invoke the existing safe Python snapshot engine without a shell.
- Settings → Tasks → Backup & maintenance now shows destination, snapshot
  list, job IDs, verification and reconstruct-to-separate-directory controls
  with warnings and explicit restore acknowledgement. No original-file
  replacement, storage activation, live catalog mutation or scheduler.
- Dedicated Go path-security regression tests added. CI/PR review and
  real-library/server-mounted validation pending.
- Next: platform/runtime tests and real restore, then non-destructive P12/P13
  storage estimates; do not enable original eviction until recovery and
  native media-consumer integration are verified.

## Original-media backup foundation — 2026-10-28

- Baseline `develop`: `3dc1ade27ea5970c14797308f0973732f2067aaf`;
  branch `feat/media-backup-safe-snapshots-20261028`.
- Add an opt-in, **offline** Python-standard-library media snapshot CLI with
  SHA-256-addressed immutable content, repeated-snapshot reuse, a manifest and
  explicit full or single-item recovery to a separate path.
- Guard accidental source/store nesting, reject symlinks/nonregular files and
  unsafe manifest paths, reject sources changing during copy, publish blobs
  and manifests without overwrite, and verify stored bytes before recovery.
- Dedicated 12-case safety and CLI tests, Makefile target, and scoped CI
  workflow. All 12 passed on implementation commit
  `393eaf93c61c98f13a246965af57b092d49cefea`; broader PR checks and review
  remain pending. No real-library restore has been exercised.
- **Not yet integrated into application Settings or scheduled jobs.** There
  is no native backup destination setting, restore UI, database/config/cache
  coordination, consistent online snapshot, or source eviction. Do not
  activate P12/P13 packed-media storage against live files on this basis.
- Next: native Settings → Tasks → Media backup UX and integrated backup
  destination after reviewing and validating this first recovery foundation.

## pHash-first Image duplicate review — 2026-10-28

- Stacked on PR #246 (`f8e93c3c9883d45bd4a460931fd93ebd3aeef290`);
  branch `feature/duplicate-review-phash-default`.
- `/image-duplicates` defaults to pHash distance 5, with adjustable 0/2/5/8
  Hamming-bit thresholds. Users can switch to EVA02 related-content matching,
  which keeps its existing cosine thresholds and approximate candidate search.
- Reuse SQLite's pHash BK-tree and union-find to group Images by their current
  primary-file hashes. Exclude source_phash, secondary-file hashes and deleted
  Image records. No EVA02 worker is needed in pHash mode.
- Response counts reflect the selected index. UI requests only clusters,
  removing the worker-status call from duplicate review.
- Preserve manual keep/remove, metadata copy, and deletion safeguards.
- Added pHash clustering/database, method parsing and UI request regression
  tests. CI, Codex review, and real-library timing pending.


## Faster, stale-safe duplicate review — 2026-10-28

- Baseline `develop` ee6e6f7a9011c1f4da9477d3ce00ffc215101072;
  branch `fix/fast-image-duplicate-clusters`.
- Replace per-Image vec0 KNN scans with a single bulk read of live embeddings,
  approximate multi-projection candidate hashing and exact full-vector scoring.
  Explain that borderline matches can be missed by approximate discovery.
- Exclude orphan vectors even if migration 102 has not run yet; skip Images
  deleted after the snapshot without failing the whole duplicate search.
- Release the SQLite read transaction during CPU-bound similarity discovery.
  Preserve per-Image top-neighbor limit and cluster preferences.
- Add regression coverage for stale vectors, candidate matching and cancellation.
- CI, real-library timings, mounted-browser verification, Codex review pending.


## Automatic orphaned EVA02 embedding cleanup — 2026-10-28

- Baseline `develop`: `6b5c9b0d05e7d4a858268d90df0123557d256ff3`; branch `fix/visual-embedding-orphan-auto-cleanup`.
- Add migration 102 to delete historical Image embedding vectors and matching
  source metadata when their Image no longer exists. Preserve valid vectors
  and unrelated Video/scene embeddings.
- Add an `AFTER DELETE ON images` trigger for atomic cleanup across all Image
  deletion paths (library scans, manual removal and repository deletes).
- Add real sqlite-vec migration regression tests for historical orphans,
  future deletions, metadata preservation and rolled-back deletion.
- Refresh upgrade fixtures, schema expectations and desktop/mobile browser
  migration validation for schema 102.
- Pending CI and production-library validation; no EVA02 reindex required.


## Fix duplicate-image similarity index query — 2026-10-28

- Baseline `develop` e3be0fb2014f03c33e17526a2da838d34f986c4a; branch `fix/duplicate-image-embedding-rowid`.
- Correct the duplicate review's `ImageEmbeddingIDs` vec0 query to retrieve
  indexed Image IDs from `rowid`, not a nonexistent `image_id` column.
- Add a SQLite vec0 regression test covering empty and populated indexes,
  deterministic ordering and deleted rows.
- Avoid telling users to install EVA02 for unrelated database/query failures
  when embeddings are already indexed.
- Pending CI and physical database/browser validation; no embedding rebuild is
  needed for this query-only change.

## Hide thumbnails globally for general Tags — 2026-10-28

- Add an opt-in **Hide general tag thumbnails** switch under Settings →
  Interface → Tag panel, default off. Persist it in the existing UI config Map.
- When enabled, hide general Tag card, detail header and background images;
  leave Copyrights, Characters and Artists unchanged, and preserve per-tag
  hide-thumbnail overrides plus the image editing controls.
- Add focused visibility-policy tests for both global and per-tag choices.
- Baseline develop: add96183daa9e5b24392ceabd54232ff8e6962d7; new branch `feat/hide-general-tag-thumbnails`.
- UI validation, mounted browser checks, CI and review pending.


## Visual embedding cancellation and per-tag thumbnail visibility — 2026-10-28

- A cancelled visual embedding worker readiness check is no longer reported
  as a failed job; explicit cancellation and normal failure status stay distinct.
- Worker readiness is checked before enumerating a large image library.
- Add a persisted per-tag `hide_thumbnail` flag, off by default, to suppress
  tag card images, detail header and background previews without deleting the
  stored image. Editing and JSON export/import retain it.
- Baseline develop: 144849617dc263b69aaaf4efae843c99b70eb3d1.
- CI, UI and physical local/remote inference verification pending.


Updated: 2026-10-26

## Safe 7z previews and individual downloads — 2026-10-26

- Recognize 7z attachments alongside ZIP/CBZ for Pawchive and Kemono
  in post cards and inspection; no external unarchiving executable needed.
- Add pure-Go 7z parsing with signature/encoded-header bounds, existing
  configurable archive and member size caps, 512-entry/128-media caps,
  restricted file types and paths, and a total selectable decoded-byte cap.
- Reuse the authenticated archive route, media fingerprints, bounded lazy
  inspector, vetted per-entry downloads, conversion workflow and existing
  decompression semaphore for 7z.
- Reject encrypted/split/malformed archives and unsupported member media;
  never extract archive contents to server paths. Add path, format,
  rejection and content-signature tests. CI and live archive tests pending.
- Baseline develop: 055658539dc38a887d0d46093440c5c29b203950.

## Follow-up to #228: native floating Booru pagination — 2026-10-26

- Place the Booru Browser's shared pagination in the same sticky
  `pagination-footer-container` / `pagination-footer` wrapper used by
  native Image lists instead of a plain static row after results.
- Open the page-number popover upward so controls remain usable at the
  bottom edge. Keep unknown-total paging, direct jumps and provider
  `hasMore` behavior from merged PR #228.
- Update browser fixture assertions for sticky footer positioning and
  upward page selector. Browser/CI verification pending.
- Baseline develop: 055658539dc38a887d0d46093440c5c29b203950.

## Pawchive search response resilience — 2026-10-26

- Enforce Pawchive's documented three-character search minimum before making
  a network request; keep recent and creator browsing intact.
- Preserve valid posts when an isolated Pawchive list record has an invalid
  identity or attachment structure. Keep upstream offsets stable, show an
  omission notice, and fail clearly if no valid posts remain.
- Distinguish JSON provider-error envelopes from malformed data without
  reflecting untrusted remote error strings. Preserve size/page safeguards.
- Added focused regression tests for short queries, partial malformed pages,
  all-invalid pages, and JSON API error envelopes. Live Pawchive API testing
  was unavailable from this environment; CI validation pending.
- Baseline develop: 055658539dc38a887d0d46093440c5c29b203950.

## Fill hidden Booru posts and expand page sizes — 2026-10-25

- Offer 60, 120 and 250 posts per displayed page, while making bounded
  provider requests of at most 50 posts for compatibility.
- Compose logical pages from a filtered stream (blacklist, saved Ignore,
  selected media and explicit AI-generated tag filters) on the server.
  Numbered pages have disjoint post IDs and retain the provider's order;
  a final page can be partial when no more eligible posts exist.
- Preserve legacy, non-filled search and favorites API behavior; opt in with
  `fill=1`. A capped scan budget protects providers; fail clearly on expensive
  filtered deep jumps rather than returning duplicated or misleading results.
- Batch review reads and writes at 50 posts; refresh after partial failures.
- Cover offset continuity, 250-post aggregation, exhaustion, validation and
  selected page-size UI behavior in regression tests. CI pending.

## Native Booru pagination with unknown totals — 2026-10-25

- Replace Booru source Previous/Next listing buttons with the shared
  `Pagination` UI and its `.pagination.btn-group` classes.
- Add an opt-in unknown-total mode: show **Page N** instead of fictitious
  **Page N of 750**, retain first/previous/next and direct page jumps.
  Omit Last when the provider has no known last page, disable Next based on
  provider `hasMore`, and cap arbitrary page input at the existing 750 limit.
  Normal library pagination remains unchanged.
- Extend mounted desktop/mobile Booru Browser tests to verify native markup,
  page-two navigation, direct jump to page 50 without a definitive total,
  unavailable Next/Last behavior, first-page return and URL/source isolation.
- CI and code review pending.

## Optional hidden Image filename headings — 2026-10-25

- Add a default-off **Hide image filenames** toggle to Settings → Interface →
  Image titles, backed by the existing persisted `ConfigResult.ui` Map via
  `saveUI` (no schema/database migration required).
- Use one shared predicate for Image card titles and the Image detail header:
  hide only filename fallbacks when the media has no explicit title. Preserve
  user-entered titles, browser document title, file info and thumbnail links.
  Keep the GridCard title action and thumbnail even when the title is hidden.
- Add focused predicate coverage for default, hidden/visible fallback, custom
  titles and missing files. CI and Codex Review pending.

## Remove booru general Tag image popups — 2026-10-25

- Follow-up to the ordinary Tag popup removal: `.booru-tag-section-general`
  uses its own `GeneralTagName` image markup and hover/focus CSS, separate
  from `TagPopover`. Remove that markup and the unused preview rules.
- Keep text links and tag wrapping intact. Do not change Artist/Character/
  Copyright cards or their images.
- Extend the desktop/mobile mounted metadata browser fixture with a general
  Tag carrying `image_path` and an Artist image. Assert no general image or
  preview DOM on initial display, hover or keyboard focus; Artist image stays.
- CI/browser regression and Codex Review pending.

## Bulk metadata review checkbox crash (#224) — 2026-10-25

- Baseline `develop` at `7bfb0a70a27fce6124ea03975bc59afc372069d4`, new focused issue #224 branch.
- Fix React checkbox handlers in `MetadataReviewDialog` for both inference
  models (Camie, EVA02) and all four metadata categories. Read
  `event.currentTarget.checked` synchronously before passing the boolean
  into functional state setters. React clears `currentTarget` after the
  handler, so accessing it later crashes repeated toggles.
- Extend the mounted metadata-review browser fixture on desktop/mobile:
  enable bulk confirmation for a two-Image selection, uncheck/recheck multiple
  model/category options, enforce the no-source/no-category safeguards,
  submit one explicit confirmation, and verify the exact job payload,
  no console errors and no URL changes.
- No bulk tagging defaults or backend semantics changed. Browser fixture
  uses synthetic HTTP; real-worker/model execution remains unverified.
  CI/Codex Review will validate the PR.

## Disable ordinary Tag hover popups — 2026-10-25

- Baseline `develop` at `c03bfec9fe4437037dfa9f0088ed540b05eed23e`, after merging PRs #221 and #222.
- The shared TagPopover component becomes a pass-through: ordinary Tag links,
  search options and selection labels no longer render a hover trigger/card,
  fetch Tag card details, or attach popup DOM. Artist, Character and Copyright
  previews remain independent and unchanged.
- Codex Review identified that Character hover previews share the existing
  `showTagCardOnHover` preference and use the Tag popup CSS. Preserve that
  preference behind a clearly named **Show Character card on hover** switch;
  retain the shared styles for Character and tagger preview cards.
- Regression test renders the Tag wrapper with no extra popup markup, using a
  CommonJS-compatible server renderer to avoid Node ESM directory-resolution
  failures during UI validation.
- CI and browser/device verification remain pending.

## Complete downloadable remote worker bundle — 2026-10-25

- Baseline `develop` at `1702e5f78a065d7417c5c2b060e44b8be3d640e1`,
  separate PR for the remote worker bundle and UI, without changing PR #221.
- Worker Settings links to development ZIP, stable releases and setup/diagnostics.
  Existing server-stored tokens are never exposed in a download URL.
- Bundle includes the complete Python service, processing modules and a read-only
  capability checker, requirements, README and SHA-256 manifest. Development
  and stable releases will publish exactly this same ZIP.
- Optional codecs, GPU runtimes, weights and model installations remain external;
  a reachable worker does not imply every capability is usable.
- Python packaging, endpoint/auth and release asset contract tests run in CI.
  No physical worker/GPU execution has been claimed.

## Larger Image review preview and image-free Tag hover — 2026-10-25

- Baseline develop `1702e5f78a065d7417c5c2b060e44b8be3d640e1` on `ui/review-image-fill-tag-hover-20261025`.
- Expand Review image metadata to an extra-large two-pane layout: Image preview
  fills the available left pane height on desktop; tag controls scroll independently,
  with a vertically stacked, scrollable narrow-screen layout.
- Use the still-image `/image/{id}/image` endpoint for the larger preview, not
  `/preview` (which serves WebM animations); fall back to the thumbnail if the
  original image fails to load. Preserve Image ID on review advance.
- Tag hover popovers still show the Tag's details and count links, but skip
  rendering or loading the Tag card image; ordinary Tag cards remain unchanged.
- Extend the existing metadata browser fixture with desktop/mobile preview-size,
  contain-fit, Image ID and responsive-overflow assertions. CI and manual runtime
  verification remain pending.
- Follow-up to Codex Review on PR #221: switched the image source to the
  still-image endpoint after confirming that /preview returns a generated WebM.
  Corrected the fixture and a noUselessFragments lint finding; browser suite's
  offscreen toast dismissal now uses the native button's click action directly.
  Fresh CI validation on the follow-up commit is pending.

## Booru entity metadata Image/Video counts — 2026-10-25

- Baseline fresh `develop` `7a492ab8d32092d6b8105bedd1e4028156dc4bac`.
  Isolated branch `fix/booru-entity-media-counts-20261025` independent of the
  open bulk-tagging PR #219.
- Root cause: Image detail `effective_associations` requested slim Artists and
  Copyrights without `scene_count`/`image_count`, despite sidebar cards
  rendering these numbers. Video detail requested the direct counts but did
  not fetch hierarchy ancestor counts; `CopyrightGrid` omitted stats entirely.
- Explicitly project media totals for Artists, Copyrights and their breadcrumb
  ancestors on both Image and Video detail fragments, and reuse one shared
  stats display on standard and Copyright hierarchy cards.
- Unknown/missing count fields no longer masquerade as zero; actual zero stays
  visible. Existing native Character card and provider navigation are unchanged.
- Added focused Node tests for count behavior and both GraphQL field contracts;
  full UI GraphQL generation/build, browser verification and CI are pending.

## Bulk Image review thumbnails, advisory and EVA02 source — 2026-10-25

- Follow-up to PR #219 on `feat/bulk-image-tag-confirmation-20261025`: add
  an authenticated thumbnail at left of Image metadata review, tied to each
  selected Image ID across sequential review.
- With bulk auto-tagging disabled, multi-Image review shows a warning with
  **Don't show again**, **OK**, and **Cancel**. The per-browser dismissal
  suppresses only the warning; it never enables auto-apply.
- Automatic bulk mode now offers Camie, EVA02, or both (default both); backend
  uses the existing EVA02 inference client, source conversion and category
  rules, and preserves filename → Camie → EVA02 source priority. Only explicit
  selected Image IDs and one confirmed request may queue the new bulk job.
- Separate user guidance describes the two workflows, model installation
  prerequisites and non-rollback of already-applied changes.
- Pending CI and Codex Review; local full build/browser execution not yet
  verified in the connected GitHub-only workflow.

## Optional bulk Image tagging and single confirmation — 2026-10-25

- Baseline fresh develop `7a492ab8d32092d6b8105bedd1e4028156dc4bac`, branch
  `feat/bulk-image-tag-confirmation-20261025`. The existing Camie multi-image
  job endpoint remained available after its Generate UI action was removed.
- Add server-persisted, default-off **Bulk auto-tagging for multiple selected Images**
  in Settings → Processing → Similarity and tagging → Tagging defaults.
- Reuse the shared Review metadata entry point for Image-only multi-selections
  from Images, All and Generate. When enabled, display one clear warning about
  the number selected, irreversible partial writes, job scope and Camie-only
  source coverage. One explicit Confirm queues the existing cancellable job;
  Cancel writes nothing, and Review individually remains available.
- Preserve the native review flow for single Images, mixed Image/Video lists,
  disabled bulk mode and unreadable settings. Bulk jobs are scoped to explicit
  selected IDs, never the whole library, and do not replace Artists by default.
- Add focused configuration and scope/payload unit tests and the usage contract
  to metadata-and-tagging docs. GitHub CI and review to validate the published PR;
  no local app/bundler/toolchain is available in this GitHub-only environment.

## Copyright draft retention and partial-link retries (E12) — 2026-10-25

- Started from fresh develop `fb812b71a8f528e91a75169c901bc042b3337bf3`
  on [`fix/copyright-save-retry-20261025`](https://github.com/Dusky-dev/StashBooru/tree/fix/copyright-save-retry-20261025).
  PR #217 was initially waiting on first-library CI. Subsequently all 12 checks
  passed, including the actual Tag desktop/mobile suite and historical upgrade.
  Codex Review completed without findings on
  `b992ba1c6895a9cb6fd2096be50c72fcd5806174`; no submitted reviews, inline
  comments or review threads. Merged #217 at
  `e20b59a8f8370a822028a47b32efeb6afbf1a383` and integrated it cleanly.
  #215's summary still said running without findings; assess eventual viable
  findings, alongside the existing #205/#196 review-follow-up obligations.
- Copyright has its own native form, not TagEditPanel and not Save & New.
  Reproduced a selected Character being deleted in another tab: metadata creation
  succeeds, the separate Character-link request fails a real foreign key, and a
  refetched initial parent resets the draft to blank with Save disabled. No
  rollback of the already-created Copyright occurs. This was an actual server
  failure and form reset, not a mocked response or a duplicate record observed.
- Seed each keyed editor's local draft once, rather than replacing it on cache
  writes for the same Copyright or initial parent. Remember the native ID after
  metadata creation, so correcting a failed Character selection retries an update
  of that record. Keep the native inputs, hierarchy validation, image handling
  and separate transactions. The error explicitly explains that details saved
  while Character links failed, and that the selection can be corrected/retried.
  No new API/schema, dependencies, generic identity layer or rollback contract.
- Tested source `b429eb8dbac4bef0e13147a0baa83098634b8b7e`, tree
  `6156e513f59ae16953d6bc3a4b71b8eb9a768ebb`, was saved remotely.
  After the clean #217 integration and documentation clarification, source
  `f0892b69485b6d526a5c8ee7149d6af10b09ac0c`, tree
  `d69604b390aeddead25018601ca3bff2d5a9f236`, exactly matches the local
  integrated source. Copyright production code and final harness are unchanged;
  combined-branch verification remains CI's gate, not a claimed repeat local build.
- Final copyright-save-retry.mjs passed at 1440 × 1000 and 390 × 844 with
  private databases, owned loopback processes and generated 144 × 81 PNG artwork.
  Blank/whitespace-only names disable Save. Real duplicate-name create rejection
  preserves the draft and all saved Copyrights. A deleted selected Character
  causes a real partial-create failure; remove it and retry the same ID, preserving
  aliases, sort name, description, two parents, a child, valid Character and image.
- Existing-record metadata save followed by another deleted-Character link
  failure also retains both selections and the draft; correcting it retries the
  same record. Choosing a real descendant as a parent rejects the cycle and
  leaves every saved Copyright unchanged. Removing that choice saves only the
  intended name edit. Cancel, canonical list Back, refresh and server restart
  preserve all five Copyright records and saved artwork response bytes. A
  separately queried ordinary Tag with the same name as a Copyright and a native
  Character retain their IDs/fields. No requests/responses are intercepted.
- Validation: make validate-ui passed all 69 UI tests, JS/CSS lint, TypeScript
  and formatting; make ui-only production bundle passed. Final browser-file
  Biome format/lint, JS syntax, workflow YAML/suite placement and git diff --check
  passed. Used #217's Linux CI backend with the freshly built Copyright UI via
  STASH_BROWSER_UI; production Go/API/schema are unchanged, so no new local
  backend build/tests are claimed. Desktop/mobile captures inspected. Successful
  fixtures were removed, no uncaught page errors or final document horizontal
  overflow; no human manual testing is claimed.
- Feature guide documents the partial-save recovery; CI adds the owned suite
  before Image/historical checkout and uploads copyrights logs/captures. Harness
  corrections follow actual native controls: close multi-selects with Escape,
  use the list card title link, and capture Back's URL after Hierarchy sorting
  canonicalizes it. No production list/navigation change was needed for these.
- Important boundary found in the saved-profile captures: Characters displays
  HTTP 400/Error loading items. A second owned server confirms the profile's
  copyrights criterion is rejected as unknown by PerformerFilterType. The save
  fixture verifies native links through GraphQL, not successful Character-list
  loading. Some captured error toasts are clipped at a viewport edge; the cause
  and positioning/animation require separate inspection. Neither is claimed
  fixed here. Network-response loss, refreshing an unsaved/partial draft, bulk
  saves, real-media playback and representative-library acceptance remain open.
- Next: applicable CI and completed Codex Review before merge, addressing viable
  findings and ending when only external gates remain. Then fix native Copyright
  branch Character listing; review toast visibility separately, followed by
  Image/Video bulk-save/partial-failure acceptance. The five broad workstreams in
  the experience audit remain open; this is not completion of P01–P13.

## Tag Save & New validation and hierarchy retries (E12) — 2026-10-25

- Continued the interrupted Tag pass from remotely saved, tested source
  `2200e952c12889b1931ba3acbd7593bcf9ca5d0f`, tree
  `9dd536a24cc173423f933499afffd2813cfdda26`, on
  [`fix/tag-save-new-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/tag-save-new-20261024).
  Baseline was develop `ac17c49490401ff8af6e4f810899960023a34d68`,
  after PR #216 merged with all 13 checks passed and clean completed Codex Review
  on `6457f1e27d8fca7655aa592c378991ef8e0bfde8`.
- PR #215 was found already merged at
  `fb812b71a8f528e91a75169c901bc042b3337bf3`; all 12 checks passed.
  Its review summary still says running on
  `b0c549be5d83e0cdbca2ef8c39c86910b5f9d863`, with no submitted/inline
  findings. This continuation did not perform that merge or treat the delayed
  review as a general waiver. Assess any eventual #215 findings alongside the
  existing #205/#196 eventual-review obligations.
- Reproduced Tag Save & New sending TagCreate for a blank name rather than
  showing the native field error. The server rejected it; no invalid record was
  persisted. Validate through the existing Formik schema and respect custom-field
  errors before submitting. Save and Save & New share existing input formatting;
  normal Formik submission receives the save promise. No new schema or editor.
- On 2026-10-24 the final tag-save-new.mjs passed at 1440 × 1000 and 390 × 844
  against owned fresh servers/databases and local artwork. Initial/post-reset
  blank-name checks send zero create requests. Real duplicate-name rejection
  retains the draft; correcting the name preserves sort name, aliases,
  description, two native parents, a child and a scalar custom field named full.
  Save & New resets metadata, relationships and artwork for a distinct next Tag.
- Existing-Tag editing chooses a real descendant as another parent. The rejected
  cycle retains the edited name and leaves all saved Tag records unchanged.
  Removing that choice and retry changes only the name. Cancel, exact list Back,
  refresh and restart preserve all six Tag records and saved artwork bytes.
  A separate native Copyright with the same name as a Tag and a metadata-only
  Video retain their queried IDs/fields and explicit Tag links throughout.
- No requests/responses are intercepted or mocked. The fixture closes native
  multi-select menus with Escape before clicking the next control, rather than
  allowing a still-open mobile menu to intercept it. No production dropdown
  change is involved. Successful fixtures were removed; no uncaught page errors
  or final profile document horizontal overflow. Desktop/mobile captures were
  inspected in that run. No human manual testing is claimed.
- Completed local validation on that saved source: make validate-ui (69 tests,
  JS/CSS lint, TypeScript and formatting), make ui-only production build, final
  browser-file Biome/JS syntax, workflow YAML/suite-placement checks and
  git diff --check passed. Final harness menu-close refinement had focused checks
  and the full desktop/mobile run. Used the unchanged #213 local backend with
  the new production UI; no new local backend test/build is claimed.
- The transient workspace reset on continuation. Restored the exact published
  tested tree; previous local logs/captures are no longer present. No production
  change or repeat build/test is needed for this documentation follow-up. CI
  adds the suite to first-library before historical checkout and uploads fresh
  tags logs/captures. Feature guide and audit next step reflect the bounded pass.
- Fresh develop is #215's merge above. Read-only merge-tree with it is clean;
  combined runtime acceptance remains CI's gate. Limits: small local profiles,
  multi-parent hierarchy, artwork and one metadata-only Video/native Copyright,
  not representative catalogs, bulk editing, scraping, playback or processing.
  Native Copyright editing uses its own form and is not changed by this fix.
  Prefilled q-query reset behavior is unchanged and outside this fixture.
- Next: applicable CI and completed Codex Review on the final head, addressing
  viable findings before merge; end when only external gates remain. Continue
  native Copyright create/edit rejection and retry acceptance separately.

## Artist Save & New validation and hierarchy retries (E12) — 2026-10-24

- Merged [PR #214](https://github.com/Dusky-dev/StashBooru/pull/214) at
  `efb500ace8d4702f90f46d93bf26b4db698fc930` after all 13 checks passed
  and Codex Review completed without findings on
  `b04538efcfab41b6474b234c5bbcef0a81c66852`; no submitted/inline findings.
  Fresh develop matches this baseline. Work is on
  [`fix/artist-save-new-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/artist-save-new-20261024).
- PR #215 is a separate pending Character pass. Its final head remains
  `b0c549be5d83e0cdbca2ef8c39c86910b5f9d863`; latest checks have builds,
  tests, generation, lint and release passed, first-library still running, and
  Codex Review running without inline findings. A delayed review is not a waiver.
  This branch does not depend on those Character changes.
- Reproduced Artist Save & New sending StudioCreate for a blank name rather than
  displaying the form's field error. The existing server rejects the request;
  this was a validation/feedback gap, not invalid records being persisted.
  Validate with the existing Formik schema and respect custom-field errors before
  submission. Save and Save & New now share their existing input formatting, and
  Formik's normal submission receives the save promise. No new schema or editor.
- Verified source `5abd8e50f08fb3557418210d0bdc7e370fe56990`, tree
  `eb18bf60bbeda1ff450b9f190fd074276666c55f`, exactly matches the tested
  local source and was saved remotely before this ledger/audit follow-up.
- New artist-save-new.mjs passed at 1440 × 1000 and 390 × 844 using private
  servers, fresh databases, a generated square artwork file and native Artist/Tag
  records. Initial and post-reset blank-name checks issue zero create requests.
  An actual duplicate-name create is rejected with the draft retained; correcting
  its name and Save & New preserves aliases, URLs, details, a scalar custom field
  named full, Tag and parent/child IDs. The reset next form's normal Save creates
  a separate Artist with no prior relationships, metadata or artwork.
- Existing-Artist editing chooses a real descendant as its new parent. The server
  rejects the ancestor cycle, retaining the changed name for correction and
  preserving the complete saved hierarchy/metadata snapshot. Selecting the valid
  parent and retry changes only the name. Cancel discards a later name edit;
  refresh, list Back and server restart retain all five Artist records, exact
  parent/child/Tag/custom-field IDs and saved artwork response bytes. Every request
  and response is real: no interception, altered IDs or mocked responses.
- Both viewport runs own their paths/processes and reject existing-library/server URLs;
  successful fixtures are removed. No uncaught page errors or final profile
  document horizontal overflow. Desktop/mobile captures inspected. No human
  manual tests, real-media playback or physical-device acceptance is claimed.
- Validation: make validate-ui passed (69 tests, JS/CSS lint, TypeScript and
  formatting), make ui-only production build passed, focused browser Biome, JS
  syntax, CI YAML/suite-placement validation and git diff --check passed. Reused
  the unchanged #213 local backend with the new production UI via STASH_BROWSER_UI.
  No local backend build/tests are claimed; production Go/API/schema is unchanged.
- CI adds the suite to first-library before the historical checkout with its
  pinned browser/current Linux artifact and artists captures/logs. Feature guide
  and audit next-step text describe the bounded result. Read-only merge-tree with
  pending #215's exact head is clean; this is not combined runtime acceptance.
- Limits: tiny profiles, one Tag, a four-level hierarchy and square artwork, not
  representative catalogs, bulk editing, remote scraping, media processing or
  performance. Existing hierarchy validation/transactions and successful form
  resets are verified, not redesigned. Prefilled q-query reset semantics remain
  unchanged and outside this fixture. Other audit and #205/#196 eventual-review
  obligations remain open.
- Next: applicable CI and completed Codex Review on the final head; assess/fix
  viable findings and end when only external gates remain. Continue native Tag
  Save & New and rejected-save/retry acceptance as a separate bounded pass.

## Image editor reachability and save/retry acceptance (E12) — 2026-10-24

- Merged [PR #213](https://github.com/Dusky-dev/StashBooru/pull/213) at
  `d2a150797b72e17f51700e702d65f108cc335823` after all 12 checks passed
  and Codex Review completed without findings on
  `4ba04e6a3b2cb71257b065ca409221257e7ad6a1`; no submitted/inline findings.
  Fresh develop matches this baseline. Work continues on
  [`fix/image-editor-save-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/image-editor-save-20261024).
- Existing native Image repeated-save behavior passed on desktop/mobile,
  including delayed association writes. The suspected Video custom-field
  transport/reset defect was not reproduced here. Image form/API submission
  code remains unchanged in the final diff.
- Expanding coverage to actual custom-field edits exposed a mobile defect:
  `.image-tabs` had a viewport max-height below the desktop breakpoint, despite
  its children using document scrolling. Overflow extended beyond the panel's
  layout height and body bottom padding, leaving Custom Fields under the fixed
  bottom navbar at 390 × 844. A normal click failed, including after centering
  the target. No forced click or synthetic DOM activation was used.
- Move the existing max-height into the existing >=1200 px scrolling-panel rule.
  Narrow layouts now grow with their content; the desktop width, height and
  internal scrolling contract remain. No schema, backend, dependency, metadata
  transaction, file lifecycle or production form logic changes.
- Verified source `49186a2fb45da35eba27ed4c5387895bdd0de3db`, tree
  `7e76af41ed12703ca5647eb31de754ef6537726d`, exactly matches the saved
  local source before this ledger follow-up.
- New image-editor-save.mjs passed against real private servers at 1440 × 1000
  and 390 × 844. Each instance scans its own generated PNG and assigns one
  native Artist, Character, Copyright, Tag and Gallery. Checks include repeated
  saves in the same editor, delayed real association requests, a custom-field
  edit and scalar user key named full, real failed update, retained title/custom
  draft and successful retry, another save, refresh, list Back and server restart.
  Exact queried metadata/relationship/file IDs and PNG bytes are preserved except
  the explicitly edited title/note. No response is mocked; rejection changes only
  an outgoing fixture Image ID. Existing libraries/server URLs are not accepted.
- Normal clicks reach the Custom Fields disclosure and its lower new-field input.
  The suite waits for the native collapse transition before capture. Computed
  max-height is checked across 575/576/1199/1200 px; full edit flows run at the
  two primary viewports. Successful fixtures are removed and failures retained.
  No uncaught page errors or final list document horizontal overflow. Desktop/
  mobile captures were inspected; the intentional rejected-update toast may
  remain in captures and is not a failed final save.
- Validation: final make validate-ui passed (69 tests, JS/CSS lint, TypeScript
  and formatting), make ui-only production build passed, focused browser-file
  Biome, JavaScript syntax, CI YAML/suite-placement validation and git diff --check
  passed. The final harness ran against that production UI via STASH_BROWSER_UI
  with the exact #213 locally built backend; production Go/API is unchanged.
  No new local backend test/generation/build or human manual testing is claimed.
- CI adds the suite to first-library before the historical checkout, reusing its
  pinned browser and current Linux artifact, with image-editor captures/logs.
  Feature guide and current audit status document the bounded result.
- Limits: one small PNG and one relationship per category, not representative
  catalogs, multiple-file activation, bulk saves, remote scraping or failed
  association-write rollback. Separate Artist/Copyright writes remain separate
  transactions. Other editors/Save & New, real processing/providers and broader
  owner-library acceptance remain open. The audit now groups remaining work into
  five workstreams; these are not five PRs or a percentage-complete claim.
- Next: applicable CI and completed Codex Review on the final head, addressing
  viable findings before merge; end when only external gates remain. Continue
  native Character editor Save & New and failed-save/retry acceptance.
  #205/#196 eventual-review obligations remain separately recorded.

## Plugin task errors and native extension acceptance (E12) — 2026-10-24

- Merged [PR #212](https://github.com/Dusky-dev/StashBooru/pull/212) at
  `a941787fc6a2bdc5d3a1d4bfb9b159fa5bc649ba` after all 12 checks passed
  and clean Codex Review completed on
  `c76772d2e1f7b29afd9007e1e5415ca1f84f63f3`; no submitted/inline findings.
  This is the baseline for
  [`fix/plugin-workflows-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/plugin-workflows-20261024).
- Read the actual first-library job log (run 40116263289, job 120940300502).
  Both historical 99→100 desktop/mobile upgrade runs passed, including rejected
  preflight/draft retention, explicit-path retry, original-schema backup opened
  by the historical app, aliases/metadata/blob bytes, refresh and restart.
  This closes that one pinned path's pending CI gate, not all earlier-version or
  filesystem-blob/automatic-rollback acceptance.
- An actual-server baseline plugin returned Error: Fixture plugin failure but
  its native job was FINISHED with error null. Task completion logged returned
  errors and discarded them. Carry the result through a buffered error channel
  and return it to the existing job manager. A completed waiter can deliver
  after its caller has cancelled without blocking on the new result send.
  Existing progress, Stop handling, successful/no-result paths and APIs remain.
- The rebuilt server reports FAILED with plugin returned error: Fixture plugin
  failure for the same owned probe. No schema, dependency, backup/file lifecycle,
  production UI, plugin protocol or synchronous-operation contract changes.
- Source `5813c2423c66cec67936dc7bb04af4d98e21152a`, tree `944651fb32624539951d829385fd749d78fbcd1a`,
  exactly matches tested local source and is saved remotely before this
  documentation follow-up.
- New plugin-compatibility.mjs passed at 1440 × 1000 and 390 × 844 against real
  private servers. It installs only local JS/UI fixtures with a declared UI
  dependency, CSS and an official PluginApi React/Bootstrap route. Dependency
  execution order and single route mounting, native Video navigation/Back,
  native modal setting edits/Confirm, saved values after refresh, plugin-cache
  reload and UI reload are checked. The harness follows the native setting
  editor and treats the nullable empty jobQueue as empty; neither requires an
  application change.
- Both explicit plugin errors and thrown JS exceptions become Failed jobs with
  full details. Success creates a fixture Tag receipt using the saved setting
  through the plugin's real GraphQL API. Cancellation waits until a VM is Running
  with reported progress, then uses native Stop; a subsequent task succeeds.
  Failed notices survive removal from the live queue, expiry of transient
  completed/cancelled notices, refresh and dismissal of only one failure.
  Their detail text and native status are checked.
- Disabling UI customizations skips the fixture UI while server-side tasks stay
  available. Native disable/Reload UI removes the plugin route/tasks; native
  enable/Reload UI restores them. Settings, enabled state, route dependency
  order and exact queried native Video ID/title/details/custom fields/Tag/file/
  Collection IDs survive a server restart. No API response is mocked, no
  existing library/server URL is accepted, and successful fixtures are removed.
- Validation: focused go test ./pkg/job -count=1, full make it (including unit and
  integration tests), make lint (zero issues), local make build, Go formatting,
  focused browser-file Biome, JavaScript syntax, workflow YAML/step validation
  and git diff --check passed. Desktop/mobile queue and route captures were
  inspected. No uncaught page errors; final route views had no document
  horizontal overflow. No human manual testing is claimed.
- Reused #212's exact generated API/production UI artifact 12569320108 and Linux
  baseline artifact 12570340488 from run 40116263289; API/schema/production UI
  source match this baseline. Only the backend fix was rebuilt locally. Do not
  claim a new local UI build, UI unit run or backend generation. CI adds the owned
  suite to first-library with captures/logs under its plugins artifact directory.
- Limits: two tiny local UI/Goja fixtures and metadata-only Videos, not arbitrary
  community plugins or patch points, package install/update/removal, hooks,
  external Python/raw/RPC runtimes or services, real media/processing,
  physical devices, model/GPU operation or performance. Native plugin-setting
  save/enable rejection flows and broader compatibility remain separate.
- Next: applicable CI and completed Codex Review on the final head; fix viable
  findings before merge and end when only external gates remain. Continue native
  Image editor repeated-save/retry acceptance; broader plugin/owner-library
  acceptance remains open. #205/#196 eventual-review obligations remain recorded
  separately.

## Character Save & New validation and draft retries (E12) — 2026-10-24

- Fresh baseline: develop `d2a150797b72e17f51700e702d65f108cc335823`
  (merged PR #213). The owner explicitly deferred merging PR #214; it remains
  separate, and this branch does not include its Image editor changes. Work is on
  [`fix/character-save-new-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/character-save-new-20261024).
- Reproduced native Save & New with an empty name sending PerformerCreate and
  showing the backend performer-name-must-not-be-blank toast, instead of native
  form validation. The server rejected the record; this was a validation/feedback
  gap, not evidence of invalid rows being created.
- Save & New now validates with the existing Formik schema and respects custom
  field/context-target errors before submitting. Save and Save & New share the
  existing input formatting; ordinary Formik submission returns the save promise.
  No new validation schema, API or parallel editor is introduced.
- The real rejected-create retry then exposed the selected Copyright context
  resetting to Custom. PerformerCreate recreated its draft object whenever the
  mutation hook rerendered its parent, retriggering context initialization.
  Memoize that object from the existing URL query so loading/error rerenders keep
  its identity. Query changes still produce a new draft. Successful Save & New
  keeps its existing explicit form/context/Tag/image reset.
- Verified source `c472765d4baf59847af8d51b6f6466f7b06479c1`, tree
  `8b582540f11210a30147180c5120c97a7a7e143e`, exactly matches the tested
  local source and was saved remotely before this ledger follow-up.
- New character-save-new.mjs passed at 1440 × 1000 and 390 × 844 against owned
  servers, fresh databases and generated portrait fixtures. Desktop chooses a
  native Copyright context; mobile chooses an Artist context. It checks initial
  and post-reset blank-name validation with zero create requests, actual rejected
  create with retained name/details/custom field/context, successful retry,
  persisted aliases/Tag/context/custom-field IDs, a scalar user field named full,
  and a clean next form/ordinary Save creating a distinct second Character without
  previous metadata or artwork. The harness opens the actual image popover and
  accepts the native profile's canonical media-tab URL.
- Existing-Character update rejection leaves the changed name in the editor and
  the saved records unchanged. Retry commits only the name change; Cancel discards
  a later edit. Refresh, exact list Back and restart preserve both native records
  and the saved portrait response bytes. No API response is mocked; rejection
  changes only outgoing fixture IDs. Successful fixtures are removed. No uncaught
  page errors or final profile document horizontal overflow; desktop/mobile
  captures inspected. These are automated checks, not human manual tests.
- Validation: make validate-ui passed (69 tests, JS/CSS lint, TypeScript and
  formatting), make ui-only production build passed, focused final browser-file
  Biome, JS syntax, CI YAML/suite-placement checks and git diff --check passed.
  Initial harness formatting was corrected before the final gate. Reused the
  exact #213 locally built backend with the newly built UI via STASH_BROWSER_UI;
  production Go/API/schema is unchanged. No new local backend test/build is claimed.
- CI adds the owned suite to first-library before historical checkout, using its
  pinned browser/current Linux artifact and characters captures/logs. Feature
  documentation describes validation, retained create context and Save & New.
  The ledger entry is beside the prior E12 checkpoint to avoid overlapping #214's
  separate prepend; the pending PR's merge remains deferred as requested.
- Limits: tiny local profiles/portrait with one context/Tag, not representative
  catalogs, remote scraping, bulk editing, variants/association-write failure
  rollback or migration/file processing. Existing variant/Copyright writes remain
  separate transactions. Prefilled q-query reset semantics are unchanged and not
  part of this fixture. The broader audit and #205/#196 eventual review obligations
  remain open.
- Next: applicable CI and completed Codex Review on the final head; assess/fix
  viable findings and end when only external gates remain. Merge #214 later per
  the owner's instruction. Continue native Artist Save & New and failed-save/retry
  acceptance, keeping that pass separately reviewable.

## Migration backup destination and upgrade coverage (E12) — 2026-10-24

- Merged [PR #211](https://github.com/Dusky-dev/StashBooru/pull/211) at
  `1254abd1cde2a3061079386313eac424b1c3f27c` after all 14 checks passed and
  Codex Review completed without findings on
  `a4c51ef9d3cd9ce36c55d5b77b458eef480749f7`; no submitted/inline findings.
  This is the baseline for
  [`fix/database-upgrade-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/database-upgrade-20261024).
- Migration Execute already preserves explicit backup paths, but PreExecute
  always checked the configured default directory. Reproduced three focused
  baseline failures: usable absolute/relative paths were rejected because the
  unrelated default was blocked; an invalid selected parent passed preflight
  when the default was valid. Match preflight to the explicitly selected parent.
  Bare filenames and blank input keep their existing default-directory behavior.
  The warning names the actual invalid directory.
- No migration SQL, schema version, backup format, Execute/automatic rollback,
  file lifecycle, dependency or production UI changes. Preflight still uses the
  existing directory-creation helper; it does not claim to prove write access or
  reserve disk space. Existing backup/SQL errors remain authoritative.
- Source `5e800ae97a86eba646418d7c4f4f98abd995f63a`, tree `5aa5fa0348d731d37af728e09a8d9cff2ebaf00d`, is saved remotely and exactly matches
  local source before this documentation follow-up. The chat resumed in a reset
  workspace during earlier local generation/build work. Restored and checkpointed
  the implementation/tests; no incomplete historical build or native migration
  run is counted as a pass.
- Restored focused TestMigrateBackupDirectory passed all six cases locally,
  including relative/absolute override, invalid selected parent, blocked defaults
  for filename/blank and creation of a configured destination. Go formatting,
  focused browser-file Biome, JS syntax, workflow YAML/step checks and diff checks
  passed. Full backend/integration gates and historical browser acceptance are
  CI gates, not completed local results at this checkpoint.
- New database-upgrade.mjs owns private configurations, database roots and
  processes. It requires an actual schema-99 application built from fork commit
  `aa1a2b42bf9bfb6b07608350bfbda688e54e91b9`; no current-database version
  label is edited. CI checks out that pinned source separately, generates its
  backend, and builds an API-only historical fixture with a placeholder UI.
  The current Linux artifact supplies the actual migration page. Generator/build
  logs and upgrade captures stay in the first-library artifact.
- The suite is designed to check desktop/mobile 99→100 migration, real rejected
  preflight with the typed path retained, a usable custom path while the default
  remains blocked, native redirect/refresh, SQLite integrity/foreign keys and new
  identity-table presence. It compares exact queried Artist/Character/Copyright
  names and aliases; Artist URLs/custom fields; Video metadata/custom fields/
  native relationships, Collection number and file IDs; and Artist image bytes.
  It checks that the named backup remains schema 99, opens a copied backup in
  the historical app, and restarts the upgraded original in the current app.
  No API response is mocked; the suite cannot target an existing library.
- These historical browser checks are pending CI, so the audit's remaining
  cross-version acceptance is not closed. Limits also include one pinned upgrade
  path, database-stored blobs and metadata-only Videos, not every earlier release,
  filesystem-blob recovery, large owner libraries, actual media/codec/processing
  recovery, interrupted SQL migration/automatic rollback, plugins, physical
  devices, model/GPU operation or performance. No human manual testing claimed.
- Next: applicable CI and completed Codex Review; fix viable failures/findings
  before merging. End when only external gates remain. Continue plugin
  compatibility as a separate bounded pass after this upgrade check is eligible.
  #205/#196 eventual-review obligations remain recorded separately.

## Backup retry and isolated restore acceptance (E12) — 2026-10-24

- Merged [PR #210](https://github.com/Dusky-dev/StashBooru/pull/210) at
  `5d29227ced1405e03989e677c73e59058fe232cb` after all 13 checks passed and
  Codex Review completed without findings on
  `140f828bb2e9a5c32ce8c634f47938df7bf07e12`. No submitted/inline findings.
  This is the baseline for
  [`fix/recovery-workflow-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/recovery-workflow-20261024).
- Baseline acceptance reproduced the backup dialog disappearing as soon as a
  request starts: the parent replaced Tasks with a loading indicator, discarding
  both backup choices and other mounted task state. Keep backup running state
  with DataManagementTasks, use the native modal's running controls and a disabled
  fieldset, guard duplicate requests and close only after successful backup.
  The existing anonymisation loading behavior remains outside this pass.
- A regular file at the configured backup-directory path produces a real server
  failure even when running as root. Rejected backups retain the chosen
  destination/blob values for retry. Mobile captures showed that an open dialog
  can obscure the previous global toast; put this backup error inside a native
  wrapping Alert instead. Clear it on retry or explicit Cancel. Reopening after
  successful retry has no stale error.
- Source `0f1c6069210c228b54cbc0c2694819ee20b7b948`, tree `5fe00c0bc64c61e0ac09263f99d88414c752d9c4`, exactly matches the tested local source
  and was saved remotely before this ledger follow-up.
- New database-backups.mjs passed all four desktop/mobile × database/filesystem
  blob-storage cases against actual private servers. It checks pending choices,
  locked Cancel/action controls, a disabled duplicate activation, actual rejected
  writes with a visible inline cause, retained choices, valid retry and preservation
  of another Tasks section's unsaved inheritance draft/open disclosure.
  No API response is fabricated and no existing library/server URL is accepted.
- Each case saves a directory backup and downloads another via the native
  dialog, retaining the Settings route. Stop the source instance, then restore
  each output into a distinct newly configured instance. SQLite integrity and
  foreign-key checks pass. Fresh GraphQL reads preserve the exact queried
  Artist ID/name/URLs/custom fields and metadata-only Video ID/title/details/
  custom fields/native Artist, Character, Copyright, Tag, Collection/number
  and file IDs. Artist image responses have identical bytes.
- Database-stored blobs survive in SQLite. Filesystem ZIPs pass archive integrity
  and contain only the database plus blobs; filesystem SQLite-only restoration
  deliberately supplies a separately copied blob directory. New media directories
  remain empty, while the source fixture's PNG bytes stay unchanged. This
  demonstrates the existing database/blob-versus-media distinction without
  adding a restore wizard, format, manifest, compatibility detector or file
  lifecycle change. Fresh-origin first-use notes are closed through the native
  dialog before inspecting the restored Artist page.
- Final 69 UI unit tests, JS/CSS lint, TypeScript/formatting, UI GraphQL generation,
  production make ui-only/built-in composition, focused Biome, JavaScript syntax,
  workflow YAML/step validation and git diff --check passed. Existing
  tasks-navigation.mjs also passed desktop/mobile: grouping, pending results/
  drafts, errors, Back, bookmarks, refresh and layout. Final backup and restored
  profile captures were inspected; no uncaught page errors or horizontal document
  overflow in the asserted views. No human manual testing is claimed.
- Runtime is the #210 Linux artifact (run 40099437771, artifact 12563025316)
  with this production UI through STASH_UI. Backend/schema match the merge
  baseline exactly. No backend/schema/dependency changes require new local
  Go gates. CI adds the owned backup suite to first-library using the existing
  artifact/browser tools and its backups artifact subdirectory. The feature
  guide documents the current behavior, invocation and fixture boundaries;
  the audit's progress row distinguishes this same-schema acceptance.
- Limits: configured data roots are created as native setup does. This does not
  verify automatic reconstruction of missing roots, cross-version upgrades,
  representative owner-library recovery, actual media/codec/processing restore,
  physical devices, plugin compatibility, real model/GPU operation or performance.
  Other native editor repeat-save/Save & New acceptance remains separate.
- Next: applicable CI and completed Codex Review on the final head; end when
  only external gates remain. Continue isolated cross-version upgrade/plugin
  acceptance as bounded passes. #205/#196 eventual review obligations remain
  recorded separately.

## Collection Video numbers and repeat-save values (E12) — 2026-10-24

- Continued the owner's “continue” request with Collection Video-number editing.
  #209 initially had clean completed Codex Review with platform builds pending.
  After all 12 checks passed, merged
  [PR #209](https://github.com/Dusky-dev/StashBooru/pull/209) at
  `1f9d40705d41d534194b3dae75b7d6c4e41aaaf3`, reviewed head
  `1f804b0fad9cd497fe76794b2ef10ff7e740d88b`, with no submitted/inline findings.
  Fast-forwarded this worktree to that merge without changing the tested base
  tree. Future PRs retain the ordinary CI/completed-review gates.
- Branch [`fix/collection-video-numbers-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/collection-video-numbers-20261024)
  has verified source `f07fe168f8bbf532c33cf11c15d2f2c2a3084cf9`, tree
  `a92e5937a4c7a73a3561d4f58e0f1a80a28c4807`, matching tested local source and saved
  remotely before this docs follow-up.
- Baseline native-editor acceptance reproduced 1.5 becoming 1. Replace partial
  parseInt parsing with the number input's complete valueAsNumber. The existing
  Yup whole-number validator now receives fractional values and blocks Save;
  complete HTML numeric input 1e2 represents 100 rather than 1. Keep the existing
  signed integer, zero and nullable/blank contracts without new range restrictions.
- A separate actual-server baseline saved zero and found no #0 caption in either
  the Video's Collection card or its Collection's Video card. Treat only undefined
  as unnumbered in those native card renderers. Clearing the number remains
  unnumbered and does not unlink the Collection.
- Repeated native saves also exposed an existing form-state defect: resetting
  Formik with the API's custom_fields full envelope caused later saves to wrap it
  again and receive a SQL unsupported-map-value error. Keep plain custom-field
  maps in form values; apply the existing create/update formatter at onSave's
  request boundary for both submission entry points. No backend/custom-field
  storage contract or shared formatter changes. A seeded user field named full
  remains a normal string, distinguishing it from the API envelope.
- collection-video-numbers.mjs passed at 1440 × 1000 and 390 × 844 with private
  configuration/databases and metadata-only Videos. It checks fraction retention/
  no SceneUpdate, whole numeric parsing, zero captions in both views, blank/null
  membership retention, signed values, real rejected save retaining the draft and
  valid retry, reload and Collection ordering after Back. An independent second
  Collection keeps its number. Exact queried title/code/details/URLs/file IDs,
  Artist/Character/Copyright/Tag/Gallery fields and custom fields stay unchanged,
  and Collection ID sets are preserved after each successful save.
- Rejection changes only the outgoing Video ID to an invalid fixture ID; the
  actual server supplies the error, which is checked for that ID. Every response
  is real. This suite owns fresh loopback processes/directories and cannot use an
  existing library. Successful fixtures are removed. Zero desktop/mobile captures
  were inspected; a transient native success toast can overlap the capture.
- Both existing Collection hierarchy runs also passed against the final UI:
  concurrent cycle rejection/retry, pending fields, native mouse reorder and
  descriptions/counts, Back/reload, rejected unlink selection, valid removal and
  preservation of other-parent metadata. No uncaught page errors or document
  horizontal overflow occurred in either suite's final assertions.
- Validation: UI GraphQL generation; final 69 UI unit tests; JS/CSS lint;
  TypeScript/formatting; production make ui-only/built-in composition; final
  focused Biome; browser/built-in syntax; workflow YAML/step validation; and
  git diff --check passed. Runtime backend/schema were verified identical between
  the reused #205 Linux artifact and this baseline; validation used this production
  UI through STASH_UI. No backend/schema changes require new local Go gates.
- Build CI reuses its existing Linux artifact and pinned browser tools for this
  numbering suite, with captures/logs under the existing artifact's video-numbers
  directory. P07 documents the behavior, commands, fixture and limits; the audit's
  current progress row includes numbering/repeat-save acceptance.
- Limits: metadata-only synthetic Videos and Chromium, not real codec/playback
  or file activation acceptance, physical touch devices, representative owner
  libraries, large-library performance, Save & New interaction, or other entity
  editors' repeated-save behavior. Native relationships/file lifecycle and
  dependency locks are unchanged.
- Next: applicable CI and completed Codex Review on the final head; end when only
  external gates remain. Continue isolated recovery/upgrade/plugin acceptance;
  other native editor repeat-save flows and owner-library/real-model/GPU checks
  remain open. #205/#196 eventual review obligations stay recorded separately.

## Collection membership and hierarchy acceptance (E12) — 2026-10-24

- Merged [PR #205](https://github.com/Dusky-dev/StashBooru/pull/205) at
  `1cc4c4d12ef106222229e30419d46e6cd22d7ec4` after all 15 CI checks passed.
  Codex still reported running on `5eeace8a7aad12b14fa16b5f249b4672b9e64b6b`
  with no submitted/inline findings. The owner explicitly said “merge both and
  continue” after the previous report had identified that pending review. Treat
  this as a #205-specific merge override, not completed review or a general waiver.
  Assess viable eventual #205 findings; the separately recorded #196 obligation
  also remains.
- Merged [PR #208](https://github.com/Dusky-dev/StashBooru/pull/208) at
  `359eec6df3725fcf842d7b7efa822b5b34bc1287` after all 12 CI checks passed and
  clean Codex Review completed on `27b9a835f82ae6673facbf857c2f2c3de2f1f2f7`.
  This is the baseline for
  [`fix/collection-editing-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/collection-editing-20261024).
- Actual-server baseline walkthroughs reproduced three Collection editing defects:
  Add fields remained editable while saving; adding a third child left the
  parent's tab counter at two; and successful unlink left one invisible child
  selected. A separate baseline removal run reached the normal drag/order and
  unlink path before timing out on the stale selected count.
- Reuse the Add dialog's existing running state to disable RelatedGroupTable
  fields and guard duplicate saves. Reuse native cache eviction to invalidate
  FindGroup detail queries as well as lists after Group mutation hooks, refreshing
  parent hierarchy counts/memberships. Pass the existing optional selection-clear
  callback through Group list operations; only the child-removal action uses it
  after its awaited mutation succeeds. Failed removal retains selection. Existing
  three-argument operations and native selection ownership remain.
- Source `b66313f2ab47c365f9d1ce456ea14e2c7db43bd6`, tree
  `b1b8c4a3b1a109a3788e0d3ad94713c16c72c8f2`, exactly matches tested local
  source and is saved remotely before this documentation follow-up.
- New collection-editing.mjs passed at 1440 × 1000 and 390 × 844. It owns fresh
  configuration/database/media directories and loopback servers, and seeds native
  Collections plus an Artist/Tag. A concurrent API edit makes a selected child an
  ancestor after Add opens: the backend rejects the real cycle, the UI retains
  the relationship description, and retry succeeds once the conflict is removed.
  Pending description/select controls and Cancel are disabled.
- Native mouse drag reorders children and preserves relationship descriptions.
  The narrow test scrolls the destination into view while the HTML5 drag remains
  active. Exact child order and parent count are checked before/after refresh and
  detail navigation/Back. Rejected unlink changes only the outgoing parent ID;
  the real error retains selection and all relationships. Successful unlink clears
  selection and preserves the child's exact queried ID/name/synopsis/URLs/Artist/
  Tags/hierarchy fields and the other parent's queried relationships. Add can be
  reopened immediately afterward. Both runs had no uncaught page errors or
  document horizontal overflow; final screenshots were inspected.
- Reused a small isolated-server launcher between this suite and gallery-chapters.
  Both chapter runs passed again: create/update/delete rejection and draft retry,
  pending controls, refresh and native Image membership preservation. Neither suite
  accepts an existing library/server URL or mocks API response data. Build CI adds
  the Collection run using its existing Linux artifact and pinned browser tools,
  with logs/captures in the existing artifact's collections directory.
- Validation: UI GraphQL generation; final 69 UI unit tests; JS/CSS lint;
  TypeScript/formatting; production make ui-only/built-in composition; final
  focused Biome and full format gate; browser/helper/built-in syntax; workflow
  YAML/step validation; and git diff --check passed. No backend/schema, native
  relationship writer, dependency-lock or file-lifecycle changes require new
  local Go gates. Runtime used the #205 Linux artifact with the new production
  UI; its Collection code matches this baseline.
- Limits: tiny synthetic hierarchies and Chromium mouse input at both viewport
  sizes, not physical touch dragging, representative owner libraries, large-tree
  performance, Collection Video-number editing/playback or recovery/upgrade/plugin
  compatibility. The audit's current-status row now reflects chapter and
  Collection hierarchy acceptance without closing those broader gates.
- Next: applicable CI and completed Codex Review on the final head; end rather
  than idle-polling. Continue Collection Video numbering/editing as a bounded
  acceptance pass, then recovery/upgrade/plugin workflows and owner-library/
  real-model/GPU acceptance.

## Booru preview blacklist validation takeover — 2026-10-24

- The owner returned booru-browser maintenance to this stream after the other
  agent completed independent work. Merged [PR #207](https://github.com/Dusky-dev/StashBooru/pull/207)
  at `4e430fb2b1e1daf0f5b5f5999749a0f3c3ac999a`: all 13 checks passed and
  Codex Review completed without findings on `eb4407cd915846315b36dd6d9859de3df4ee6981`.
- Continued the existing [PR #205](https://github.com/Dusky-dev/StashBooru/pull/205)
  and its `feat/booru-preview-tag-blacklist` branch rather than creating a second
  implementation. Integrated the fresh develop baseline above. Published source
  checkpoint `e17ec96def90c3d3f6d91c7ab64b1fdb79285fb9`, tree
  `e65e60632e2761b4bb4679bf3dc68bec0170470c`, exactly matches the local tree.
- Investigated all three failed gates on the original #205 head. Backend lint
  found three ignored JSON encoding errors in tests; check them explicitly,
  including reporting and returning safely from the concurrent append test.
  UI generation/validation found formatting violations; apply the repository
  formatter. The booru browser suite expected an exact confirmation text inside
  a dismissible Alert, whose text also includes its Close control. Reproduced
  the timeout locally; an isolated diagnostic confirmed that the notice and
  refreshed results were present at desktop/mobile. Match the status region's
  content instead of asserting that its Close button has no text.
- Extend the existing mounted booru suite with a rejected blacklist save,
  retained preview/query and unchanged saved blacklist, then successful retry,
  exact source/instance/tag payload, confirmation, originating-source filtering
  and other-source isolation. This suite mocks provider/settings HTTP responses;
  backend handler tests separately exercise actual persisted configuration,
  credentials, deduplication, invalid/stale source input and concurrent appends.
- The first complete mounted run also reproduced an attachment keyboard bug:
  focusing Media 2 can scroll the preview while Media 1 still crosses its top,
  causing the scroll tracker to replace the explicit focused target. Up now
  resolves a focused media entry first, then falls back to the visible entry.
  The suite dispatches the competing scroll event and checks the exact attachment
  download key. Existing native input/video/dropdown key handling remains.
  Source checkpoint `f8e57ca0020a4d774fa2562aaa92038ab4ecd02e`, tree
  `cd57bdbc1e4b93b561b242ecd6a39b61fbbda8b6`, matches the tested local code;
  subsequent changes only organize the guide and record these results.
- Source review: the additive handler holds the existing settings lock, reads
  current configuration, checks the source instance and preserves current
  credentials/other sources. Ordinary settings saves retain revision conflicts;
  blacklist limits and atomic private-file persistence remain in force.
- Codex on the original head explicitly reported review quota exhaustion and
  supplied no inline findings or submitted review. This is the documented quota
  exception, not a completed clean review. Applicable CI still gates merge.
- Validation: all 69 UI unit tests, final JS/CSS lint, TypeScript, formatting,
  production make ui-only/built-in composition, JavaScript syntax and diff checks
  passed. The complete mounted booru suite passed at 1440 × 1000 and 390 × 844,
  including blacklist save rejection/retry and focused-attachment keyboard
  download. Existing selection, preview focus/navigation/playback cleanup,
  downloads/converter defaults/destination, settings/credentials, autocomplete,
  filters/favorites, Back/reload, provider failure/races and overflow checks passed.
- Backend generation and the focused API settings/blacklist/preview tests passed
  locally. An initial direct test command used an incorrect manual SQLite include
  version and did not compile; rerunning with the Makefile's module-resolved
  version passed. On source f8e57ca, CI generate, all three mounted-browser jobs,
  backend lint and Test Backend (make it, including integration tests) passed.
  Their build workflow is run 40051099551, lint is 40051099532. The duplicate
  local full-Go run had not completed when these CI gates passed; it is not used
  as pass evidence. Do not claim local make test, make it or make lint completed.
  Platform builds were still running at this checkpoint. Final docs/ledger head
  still requires applicable CI.
- Request a fresh Codex Review on the final head: #207 completed after #205's
  earlier quota response, so the old exhaustion is not used to bypass a newly
  available review. End when only external gates remain; do not idle-poll.
- Limits: synthetic Chromium desktop/mobile and isolated backend fixtures do
  not establish live-provider reliability or owner-library usability. This pass
  changes no native library data model, file lifecycle or source adapter.
- Next: finish #205 checks and merge only when eligible; resume E12 Gallery
  chapters/Collection hierarchy and ordered editing, then isolated recovery,
  upgrade and plugin compatibility. Live-provider/model/GPU and representative
  owner-library acceptance remain open.

## Gallery action layout and selection (E07/E12) — 2026-10-24

- Merged [PR #206](https://github.com/Dusky-dev/StashBooru/pull/206) at
  `f0ba1ed853a0100e7a4bb466c3adf43b30d5f6e6` after all 12 applicable checks
  passed and Codex Review completed without findings at
  `196ac162c66a60f0556eb327cba407059c37e5a4`. This is the fresh develop
  baseline for branch [`fix/gallery-actions-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/gallery-actions-20261024).
  Verified source `0065e47bfd863e87446ab415684d081f59e60a74`, tree
  `60cc5fcf8ad7362d3c693d32086c32bbb5bbb7eb`, matches the tested local tree.
  The source branch is saved remotely before this ledger follow-up.
- Reproduced Gallery Add mounting the general Images sidebar over the Gallery
  layout at desktop and mobile. A separate baseline action check reproduced a
  successful Add leaving one invisible completed Image selected after refetch.
  The preceding Gallery walkthrough also exposed the same removal behavior.
- Give Gallery Add the existing GalleryImages view, matching the native Images
  tab's inline layout and view preferences. Reuse its normal filter controls;
  no new CSS, route, widget or preference store. Offer an optional selection-clear
  callback in the existing list operation contract and pass it from native Image
  lists. Only Gallery Add/Remove invoke it after their awaited mutation succeeds.
  Existing three-argument operations remain valid; rejected actions keep their
  selection, and the native off-page selection behavior is unchanged.
- Extended the existing optional library-membership suite: assert no general
  sidebar in Gallery Add, no stale selection after successful Add/Remove, and
  selected targets retained for retry after actual server rejection. The failure
  check changes only the outgoing Gallery ID to an invalid fixture value. The
  real error response reaches the native catch/toast path; membership is checked
  unchanged before the normal valid request is retried. No response is mocked.
- All four actual-server Gallery/Collection cases passed at 1440 × 1000 desktop
  and 390 × 844 mobile. Preserve #206's query/scope, ordered Collection paging,
  Image/Video Back, refresh and empty-search/tab-reset checks. Fresh reads confirm
  exact other Gallery members and unchanged native Image ID/title/direct Artist,
  Character, Copyright, Tag and other Gallery IDs after add/remove. The temporary
  membership is removed in finally. Final Add layout and mobile unselected-list
  screenshots were inspected; error toasts in failure captures are intentional.
- Existing selection-ranges.mjs also passed both viewports: All/native cross-page
  ranges, retained off-page selections, typed Image/Video IDs, reload, first Video
  checkbox selection and preview Back/Escape. Browser checks now await populated
  card lists/default-tab resolution explicitly rather than relying only on
  network-idle timing while lazy chunks initialize.
- Validation: UI GraphQL generation; all 69 unit tests; JS/CSS lint; TypeScript;
  final pnpm format-check; focused Biome; production make ui-only with built-in
  composition; browser/built-in JavaScript syntax; and git diff --check passed.
  The initial make validate-ui run reached formatting, which caught a new browser
  wait statement; it was formatted and the format gate then passed. App source
  was unchanged after unit/lint/type checks. No backend/schema changes require
  additional local Go gates. Runtime used the #206 Linux artifact and this UI
  through STASH_UI with a newly generated isolated PNG/H.264 fixture.
- Added a current-progress summary to the experience audit, separating merged
  consolidation passes from remaining live-provider/model, representative-library,
  recovery/upgrade/plugin, accessibility/performance and localization acceptance.
  The dated initial finding rows remain historical rather than completion claims.
- Limits: automated Chromium/synthetic-library checks, not owner usability,
  physical touch devices, all display modes, broad playback/model, chapter/
  sub-Collection reordering or deletion/recovery acceptance. These suites require
  the documented isolated seed and are not ordinary CI browser jobs. Native data
  writers, file lifecycle and the separately assigned booru browser are unchanged.
- Next: E12 Gallery chapters and Collection hierarchy/ordered editing, followed
  by isolated recovery/upgrade/plugin workflows. Real-model/GPU E08 and owner
  representative-library acceptance remain open. Await applicable CI and completed
  Codex Review on this head; end instead of idle polling. The earlier #196 review
  obligation remains recorded below without extending its owner-specific override.

## Gallery/Collection list context (E12) — 2026-10-24

- Merged [PR #203](https://github.com/Dusky-dev/StashBooru/pull/203) at
  `e101ec99ee13b833d57554c0a54aaabe0b68f0ac` after all 12 checks passed and
  Codex Review completed cleanly at `4d61fc8013a95a6ad295de8283f56d5586ad32e6`.
  The following Gallery/Collection implementation passed local checks on October
  23, but its unpublished worktree was lost when the workspace rolled back.
  Recovered the scoped changes on fresh develop
  `45e7553b5a816423e930110547c4515ec21b237a`, which includes the separately
  managed converter PR #204; this pass does not modify that converter work or
  the other agent's open booru-preview work.
- Branch [`fix/library-membership-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/library-membership-20261024)
  has verified source `56c3a2c324d22d03401213a2464fe8da394e4999`, tree
  `b7e2eed526c9ade8a56572048b7bd46a5a3bb59a`, matching the newly tested local
  source. Saved this source branch remotely before the ledger follow-up.
- Actual-server baseline checks reproduced two navigation failures at desktop
  and mobile: Gallery Images → Add replaced title/descending/page-size/page
  options with defaults, and Collection default-tab resolution dropped direction,
  page size and page. A bare-path history.replace discarded the current query.
- Gallery Images/Add now change only the pathname while retaining the current
  location. The shared default-tab resolver does the same when canonicalizing
  the existing tab. Explicit switches between different media types keep their
  previous filter reset. Read history.location at replacement time so child list
  query normalization is retained. No new store, route, preference or filter
  grammar; native Gallery include/exclude and Collection membership remain in
  their existing list hooks.
- Recreated a fresh isolated fixture after recovery: four generated PNG Images,
  two short H.264 Videos, one metadata-only outside Video, a Gallery with two
  members and two candidates, and a Collection with distinct member scene numbers.
  Outside Images have direct Artist/Character/Copyright/Tag relationships for
  preservation checks. No production library or provider/model is involved.
- All four recovered browser cases passed: desktop 1440 × 1000 and mobile
  390 × 844, each Gallery and Collection. Check tab query/scope retention, native
  Image viewer → detail → Back, Video → Back, refresh, Collection descending scene
  order/page, empty search through the default-tab redirect, and reset on an
  explicit Character tab switch. Gallery add/remove uses the actual native
  buttons, checks exact other-member preservation, removal from candidate/member
  lists and unchanged native Image ID/title/direct relationships. The temporary
  membership is removed in finally. No GraphQL responses are mocked.
- New optional library-membership.mjs requires explicit isolated URL and Gallery/
  Collection IDs. It checks visible cards, excluding inactive native tab panes,
  and awaits removal from the rendered list after mutation/refetch. Final desktop
  Gallery and mobile Collection ordering captures were inspected. P07 documents
  fixture requirements, invocation, writes and limits. It is not an ordinary CI
  browser job and does not seed a catalogue itself.
- Fresh recovery validation passed UI GraphQL generation; locked pnpm validate
  (all 69 tests, JS/CSS lint, TypeScript and formatting); production pnpm build
  with built-in composition; focused Biome; browser/built-in Node syntax; and
  git diff --check. Runtime used the PR #203 Linux artifact with the current
  production UI through STASH_UI. Backend/schema code is unchanged, so no new
  local Go gate is required. New CI and completed Codex Review remain required
  on the published head; end rather than idle polling.
- Limits: no production-library, physical touch-device, all display modes,
  chapter/sub-Collection reordering, broad playback/codec, file deletion or
  upgrade/recovery acceptance. Native data writers and viewer/player lifecycle
  behavior are unchanged. Broader E12 and real-model E08 acceptance remain open.
- Next: inspect Gallery Add's sidebar layout and selection retained after
  successful membership actions. Keep intentional native off-page selection;
  assess only the action/scope transitions before changing behavior. The old
  owner-specific #196 review override remains recorded below.

## Honor custom UI command-line paths (E12 follow-up) — 2026-10-23

- Merged [PR #202](https://github.com/Dusky-dev/StashBooru/pull/202) at
  `69df69d75a27bf460d09c43d77dc077b4bc187af` after all 14 applicable checks
  passed and Codex Review completed without findings at
  `ddf52fa882f62bcc5edc2283be9c8e25f72cb730`. This is the fresh develop baseline
  for branch [`fix/custom-ui-flag-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/custom-ui-flag-20261023).
  Verified source `86ac3a98b4e62bcda45e2388672bc1e5237ee96f`, tree
  `c889153b4905ccfad7e8a372c9da26369bb7a14c`, matches the tested local tree.
- Reproduced the previously observed --ui-location failure against the packaged
  Linux server: a custom directory containing a distinctive index and CSS was
  ignored and the embedded frontend was served. Focused configuration regressions
  also failed for the long flag, -u and an explicitly empty override before the
  correction. The loader stored ui-location while GetUILocation reads ui_location.
- Map only this changed command-line flag to the canonical UILocation key. Keep
  unchanged flags out of overrides, preserve other flags, saved/legacy paths and
  existing STASH_UI-over-command-line precedence. Overrides do not rewrite the
  saved configuration. No general flag normalization or precedence change.
- Added configuration cases for both forms, spaced paths, unused flags retaining
  canonical/legacy settings, explicit empty canonical override, embedded default,
  environment precedence, ordinary port flags and unchanged saved values. The
  configuration package passes, including these regressions.
- Added scripts/tests/custom_ui_flags.py with six independent, unconfigured
  loopback server instances and generated HTML/CSS: long flag, short flag,
  relative path (including spaces), environment, environment over flag and
  embedded fallback. Verify the root index, SPA route and exact static asset
  origin. Strip inherited STASH overrides, disable the extra HTTPS listener,
  terminate each process and remove temporary configurations in finally. No
  existing server, library, provider, model or media is used.
- All six real-server cases passed against the newly built fixed backend. Reused
  the unchanged production UI from merged #202, with exact UI-source equality
  confirmed; no additional frontend rebuild or UI gates were needed. CI reuses
  its Linux artifact for these cases in the existing first-library job and
  uploads server logs with first-library-artifacts. Documented invocation,
  configuration precedence and test scope in DEVELOPMENT.md.
- Validation: backend generation, make test, make it and make lint passed
  (zero lint issues). Makefile SQLite include paths/tags were retained. The local
  make stash build passed with -buildvcs=false and the local-test version label;
  all six HTTP serving cases then passed. Focused configuration tests, Go
  formatting, Python syntax, workflow YAML/extracted-shell/artifact assertions
  and git diff --check passed. No dependency lock, schema, database, frontend,
  media relationship, file lifecycle or booru-layout change.
- Limits: HTTP assertions use tiny static fixtures, not browser rendering of an
  arbitrary third-party frontend, authentication/proxy compatibility or physical
  device/production-library acceptance. CI and completed Codex Review remain
  required on the new published head; end rather than wait or repeatedly poll.
  The eventual #196 review was checked once and still reports running with no
  findings; its earlier owner-specific merge override remains separate.
- Next: E12 Gallery/Collection → member → Back acceptance in an isolated library,
  checking native membership, ordering/filter scope and return context. Broader
  owner-library, upgrade/recovery and real-model E08 acceptance remain open.

## Refresh Copyright/default memberships (E07/E12, third pass) — 2026-10-23

- Merged [PR #199](https://github.com/Dusky-dev/StashBooru/pull/199) at
  `eb8301c6ad3f3df612b9394753ad9cbb4994a25b` after all 12 checks, including
  first-library browser acceptance, passed and Codex Review completed cleanly
  at `6651f5ba3c4bdd3eeefafd712784bccaa70f9b37`. The official-mirror/headless
  dependency correction completed remotely; the previous cancellations are
  superseded by this actual acceptance pass.
- Merged [PR #201](https://github.com/Dusky-dev/StashBooru/pull/201) at
  `27261a64e59c3d5d13de2b74444270b7a8ac856c` after all 11 applicable checks
  passed and Codex completed without findings at
  `db5217993dbac2f6c2c59804cec4434a9b5719a0`. This is the fresh develop
  baseline for branch [`fix/copyright-associations-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/copyright-associations-20261023).
  Verified source `d7c192ad9a81ebeccd9306a76a23dcbce7e5cd2e`, tree
  `1317844e029fcb8e5048ec00bf87c288379adbef`, matches the tested local tree.
- Reproduced two remaining actual-server gaps after #201: Copyright forms
  bypassed the shared mutation boundary, so Back retained an old inherited Tag
  after reparenting; saving the Copyright inheritance switch invalidated only
  configuration, so Back still showed inherited memberships after disabling it.
  The native form/save and Library switch changed the server correctly; the
  cached detail snapshots were stale. No mock responses were involved.
- Added Copyright create/update/delete/Character-assignment hooks at the existing
  StashService boundary and connected the Copyright forms. Expire affected
  Copyright/Character query roots, media lists and calculated media associations.
  Inheritance-default saves expire Image/Video lists/calculated associations;
  unrelated default saves keep configuration-only invalidation. Retain direct
  fields, native IDs and relationships. No new store, global network-only policy,
  backend/schema, file lifecycle or booru-browser layout change.
- Optional real-server `copyright-associations.mjs` passed all four cases:
  desktop 1440 × 1000 and mobile 390 × 844, each Image and Video. Toggle Copyright
  inheritance off/on through Library and Back; edit the child Copyright's parent
  through its native form and Back; verify old inherited Tag/parent removal,
  new Tag/parent and provenance, refresh consistency, unchanged direct
  Artist/Character/Copyright/Tag IDs and no page errors. Inspect final desktop
  and mobile captures. The suite requires explicit isolated URL/fixture IDs and
  restores original parent lists and shared inheritance defaults in finally.
  Mobile automation clicks the visible Bootstrap switch track with normal
  hit-target checks, avoiding a forced click on its transparent native input.
- Validation: UI GraphQL generation passed; make validate-ui passed all 69 tests,
  JS/CSS lint, TypeScript and formatting; make ui-only passed and composed the
  built-ins. Focused Biome, new browser-script/built-in Node syntax and diff
  whitespace checks passed. Runtime used the packaged Linux binary with the
  current production UI through STASH_UI. Backend code is unchanged, so no new
  local Go gate is needed. Fresh CI and completed Codex Review remain required
  on the published head; end the turn instead of idle polling.
- Limits: tiny synthetic PNG/H.264 media, Chromium keyboard/mouse automation
  and an optional seeded-server suite do not establish owner-library, physical
  touch-device, deletion/primary-ordering, full playback/codec, external/model,
  upgrade or recovery acceptance. The browser suite is not an ordinary CI job;
  existing cache unit cases remain in UI CI. See P06 for invocation and scope.
- Next: fix the separately observed --ui-location configuration-key mismatch,
  with actual custom-UI loading acceptance. Broader representative-library E12
  and real-model E08 checks remain open. The eventual #196 review obligation
  remains recorded below; no new review override has been inferred.

## Real first-library acceptance (E11) — 2026-10-23

- Merged [PR #198](https://github.com/Dusky-dev/StashBooru/pull/198) at
  `0f02385255d6cea497306bd377a5547e4bc67149` after all 12 CI checks passed
  and Codex Review completed without findings at head
  `6510efa757f3cfe6b305a75f59d61671489974e1`. This is the fresh develop
  baseline; no other PRs were open at publication. The eventual #196 review is
  still outstanding from its specific owner-authorized merge override.
- Branch [`fix/first-library-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/first-library-20261023).
  Verified source checkpoint `bbeae88ccb0d1d17329ca6f7e29f2029670f0460`, tree
  `7fb99d9f1790419f40a316979aaeffaf7152230e`. Local and connector-created source
  trees match; the following ledger-only checkpoint records the result.
- Walked the actual application with fresh configuration, SQLite and media
  folders, rather than extending the existing configuration-response fixtures.
  Added a reproducible suite that owns its loopback server and temporary paths,
  strips inherited STASH configuration overrides and disables the extra HTTPS
  listener. It does not accept an existing instance or mock API/scan/save results.
  Each run draws a neutral geometric PNG and encodes a two-second H.264 MP4.
- Both independent desktop (1440 × 1000) and mobile (390 × 844) libraries passed:
  setup paths → Confirm → Welcome/Finish → Library; note acknowledgement after
  reload; Tasks → Update library → real Scan; one Image and one Video in All,
  with decoded thumbnails; Image viewer → native detail/Edit → saved title;
  refresh and process restart retaining the same native Image ID/title and note
  acknowledgement. Mobile uses the existing Settings selector and navbar menu.
  Editor width and page-error assertions passed; All/editor captures inspected.
- No confirmed gap in this bounded flow justified changing application behavior
  or adding a wizard. Documented the actual path, test invocation and acceptance
  limits in [first-library acceptance](first-library-acceptance.md); updated E11
  in the experience audit. An early exploratory shutdown showed release notes
  again; fresh full runs awaiting the save and checking reload/restart passed on
  both sizes, so no speculative acknowledgement change was made.
- Build CI now downloads its existing Linux artifact for this real-server PR
  check, avoiding a second compile, and uploads screenshots/server logs. This
  adds no production state, dependency version update or release dependency.
  Local production UI/backend builds and backend/UI generation passed at the
  baseline; this sandbox needed Go's `-buildvcs=false` because of worktree VCS
  detection. The Makefile still supplied SQLite flags/includes. No application,
  backend/schema, generated-source or booru-layout change was made.
- Validation: `make validate-ui` passed 61 unit tests, JS/CSS lint, TypeScript and
  formatting. New browser suite passed both complete real-server runs; separate
  Biome check, Node syntax, workflow YAML/artifact/dependency assertions and
  `git diff --check` passed. No additional backend test gate was needed for
  test/documentation/CI-only source edits. CI's downloaded binary execution still
  needs its first remote run; local execution used the same source baseline.
- PR #199 follow-up: Codex completed without findings on initial final head
  `0dfcb0fd7cf7a3ba455aa938d251017fc85ae209`; all 12 ordinary checks passed.
  The additional first-library job was cancelled at its ten-minute limit during
  Ubuntu FFmpeg dependency downloads, before browser installation or execution.
  Logs show no application/test failure. Use `--no-install-recommends` and allow
  thirty minutes for cold dependency installation; YAML/install/budget assertions
  and diff whitespace pass. Do not treat the cancelled run as a pass. New CI and
  review must complete on the updated head before merging; no idle polling.
- Next CI run reached the application and exposed a restart-test race: the
  live page retained Apollo clients when the harness intentionally stopped its
  server, recording expected shutdown fetch failures as page errors. Assert the
  error list while the server is still live, close the page before shutdown, then
  use a fresh browser page/context after restart and assert errors again. Do not
  filter or ignore fetch errors during the live flow. A fresh client also proves
  the saved ID/title and acknowledgement come from the server rather than cache.
  Both complete desktop/mobile runs passed locally with the packaged Linux CI
  binary. Focused Biome, Node syntax and diff whitespace passed; application code
  is unchanged. The new head still needs remote CI and completed Codex Review.
- Merged PR #200 at `efd5c0d93a352bd6e48bf957822dd08d64d2f28f` after all 11
  applicable CI checks and completed clean Codex Review at its final head. Its
  selection fixes and native-library acceptance continuation remain separate.
- PR #199 recheck at `a867c414ff921a54ac0c6fe9f11cfd9c477cdeb7`: Codex
  completed cleanly and all 11 ordinary checks passed. The first-library job
  again exhausted its full thirty-minute budget before any browser execution:
  Azure's Ubuntu mirror took about 23 minutes for FFmpeg, then timed out during
  Playwright's system dependencies. Switch this job's existing Ubuntu mirror
  lists to the official archive/security endpoints and install only the
  headless Chromium shell used by the suite, retaining required dependencies,
  Playwright 1.57.0 and all assertions. YAML, extracted-shell syntax, mirror/
  artifact/install assertions, Playwright CLI option validation and diff checks
  passed. Actual hosted download speed and the complete new CI run remain
  unverified; do not merge until the browser check and review complete.
- Limits: synthetic tiny media and automated browser acceptance are not owner
  usability testing, upgrades, full playback/codec coverage, physical devices,
  non-root hosting, provider/model inference, relationship edits or recovery.
- Next: E07/E12, check local selection, viewer return/Back/refresh and native
  relationships with an isolated library; fix reproduced problems in a bounded
  pass. Representative owner data and real-model E08 acceptance remain open.
  End once only CI/Codex remains; use the ordinary review/CI gate next turn.

## StashBooru first-use introduction (E14, second pass) — 2026-10-23

- Merged [PR #196](https://github.com/Dusky-dev/StashBooru/pull/196) at
  `a496de8aad83b97dff7bad08932b1534348d6eff` after all 13 CI checks passed.
  Codex still reported running with no findings; the owner explicitly requested
  merging both pending PRs after being told that status, overriding the wait for
  this PR. This was not a completed review or quota-exhaustion result. Inspect any
  eventual #196 findings on a later check. Merged [PR #197](https://github.com/Dusky-dev/StashBooru/pull/197)
  at `28017d7736974e869cf5977cbf04e73aec074369` after all 12 checks passed and
  Codex completed without findings. The latter is this fresh develop baseline.
- Branch [`fix/first-use-notes-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/first-use-notes-20261023).
  Verified source checkpoint `b43f37a9265f49e8e2ece3b1bcda46f53281fc13`, tree
  `f4a2944c34d21ce6dca319de9e1c8e7f939ab462`. Connector-created/local source
  trees match; this ledger-only checkpoint is included before opening review.
- Share one dated English StashBooru introduction in Welcome, release notes and
  Changelog. It explains library/All entry points, native metadata/source aliases,
  current Following limits, optional Processing and separate backups, with a
  feature-guide link. Put it before retained upstream notes; keep it accessible
  in Settings → Changelog after acknowledgement. Its date is a note marker, not
  an application version/tag. No new onboarding flag, store, wizard or route.
- Reuse the exact existing unseen-note date predicate in a shared selector used
  by App. Keep App's setup/status/configuration gates and explicit Close save.
  Welcome already marks the newest note on Finish; now it displays that same
  introduction first. Finish still opens Library settings, retains FFmpeg choices
  and uses existing server UI settings. Correct three English setup messages to
  match that destination, preserving IDs/placeholders. Point fork issues to the
  fork repository; upstream help/support links and history stay available.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting. `make ui-only`, GraphQL generation/built-in preparation, generated
  built-in syntax and `git diff --check` passed. Workflow YAML and the three changed
  English messages/ICU placeholders were inspected. No backend/schema/dependency
  changes require Go gates.
- New mounted suite passed desktop/root at 1440 × 1000 and mobile under /stash at
  390 × 844. Check fresh/old acknowledgements, fork-first ordering, retained old
  notes, exact guide link, Escape without acknowledgement, explicit Close saving
  the latest date while retaining other UI preferences, refresh without another
  write, reopening Changelog/upstream history without changing the date, real
  Welcome content/fork issue link and Finish's prefixed Library destination. The
  installed-codec fixture performs no FFmpeg download. Captures were inspected;
  Settings navigation CI now runs this suite and uploads its screenshots.
- Fixtures use the production date selector and real Welcome, release-note dialog
  and Changelog with isolated configuration responses/local persistence. They do
  not mount the entire App bootstrap, contact a production server, perform actual
  setup/scan/codec installation, or establish full first-user acceptance. English
  introduction text follows existing English release-note behavior; complete
  terminology/localization remains separate.
- Next: E11 first-library setup → first scan → first metadata edit in an isolated
  instance. Fix reproduced gaps before adding another wizard or broad UI layer.
  E07/E12 cross-feature acceptance and real-model E08 checks remain open. Future
  PRs keep the ordinary completed-review/quota gate; the #196 override was specific.
  End when only external CI/Codex remains, without waiting or repeated polling.

## Refresh inherited metadata after profile edits (E07/E12, second pass) — 2026-10-23

- Merged [PR #200](https://github.com/Dusky-dev/StashBooru/pull/200) at
  `efd5c0d93a352bd6e48bf957822dd08d64d2f28f` after all 11 applicable checks
  passed and Codex Review completed without findings on its final head
  `a34f0bd64e92f875e5993b61c59829174cebf2f1`. This is the fresh develop baseline.
  Separate branch [`fix/library-acceptance-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/library-acceptance-20261023)
  has verified source checkpoint `fe083299dad8a155412f6f1175a656f509ff5e5f`, tree
  `dcbf0a74167adcbeda2097857e0a0e69a284f143`. Local/connector trees match;
  the following ledger-only checkpoint records verification.
- Reproduced a real native workflow gap: an Image directly selected a child
  Artist and a second Artist. Its old parent supplied an inherited red Tag.
  Reparenting through the native Artist form changed the server to the new
  parent's blue Tag, but browser Back retained red until reload. Evicting list
  roots left complete effective_associations on normalized Image/Scene objects,
  allowing detail reads to reuse their obsolete calculated membership snapshot.
- Reuse the existing shared mutation invalidation boundary: invalidating native
  Image/Video lists now also expires only the effective_associations field on
  cached Images and Scenes. Preserve typed object identity, direct relationships
  and other fields; the next detail read fetches memberships/provenance from the
  server. No new store, global network-only fetch policy, relationship rewriting,
  database/schema change or booru layout change. Existing native Artist, Character
  and Tag mutation helpers already use this boundary. Specialized callbacks
  outside it are not claimed covered without their own acceptance.
- Three real Apollo InMemoryCache unit cases verify equal numeric Image/Video
  IDs, incomplete cached reads after invalidation, unchanged direct fields and
  unrelated records, empty caches and repeat calls. Optional mounted suite uses
  explicit URL and Artist/old-parent/new-parent/Image/Video IDs for an isolated
  fixture, edits the native form, and restores the original parent in finally.
  No catalogue or mutation responses are mocked.
- All four browser runs passed: desktop 1440 × 1000 and mobile 390 × 844, each
  with native Image and Video detail pages. Verify old inherited Tag before edit,
  new Tag/Artist and provenance after Back without reload, disappearance of old
  inherited memberships, reload consistency, no page errors, and fresh GraphQL
  direct Artist/Character/Copyright/Tag IDs unchanged. Two directly selected
  Artists, a Character, Copyright and explicit Tag remain in the fixture. Captures
  inspected. Synthetic PNG/H.264 files and narrow Chromium keyboard/mouse runs
  are not owner data, playback/codec or physical touch-device acceptance.
- Validation: make validate-ui passed all 69 tests, JS/CSS lint, TypeScript and
  formatting. Production make ui-only, UI GraphQL generation/built-in preparation,
  generated built-in syntax, focused Biome, browser-script Node syntax and git
  diff --check passed. Used the packaged Linux CI binary with this production UI
  through STASH_UI; no backend source changed, so no additional Go gate needed.
  Documented reproduction, fix, optional test fixture/invocation and limits in P06.
- PR #199 remains separate and unmerged. Its next CI run reached the app but the
  test kept Apollo alive across deliberate server shutdown, then treated expected
  in-flight fetch failures as page errors. Pushed a close-before-shutdown/fresh-
  page-after-restart correction, with error assertions before and after rather
  than filtering errors, at `a867c414ff921a54ac0c6fe9f11cfd9c477cdeb7`, tree
  `2f353dc58c08c668147a65beddd42747d9f59cd6`. Both full local desktop/mobile
  setup/scan/save/restart runs passed against the packaged binary; Biome, syntax
  and whitespace passed. Codex completed cleanly on that head; fresh CI remains
  pending. Do not bypass its gate. The earlier #196 override review still reports
  running with no findings; inspect any eventual feedback when it arrives.
- Next: browser acceptance for Copyright-specific edit callbacks and inheritance
  defaults, plus the separate observed --ui-location configuration-key mismatch.
  Broader owner-library, upgrade/recovery and real-model E08 checks remain open.
  End once only external CI/Codex remains; assess viable findings before merge.

## Shared media selection fixes (E07/E12, first pass) — 2026-10-23

- Independent branch [`fix/media-interactions-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/media-interactions-20261023)
  starts at develop `0f02385255d6cea497306bd377a5547e4bc67149` while PR #199
  completes checks; it does not depend on that PR's setup suite or CI edits.
  Verified source checkpoint `2120f762da42baee5a76c83908891fcc26e9aef2`, tree
  `50339a1102c369984ac2f21b8487abbded7f6be2`. Connector/local source trees match;
  this separate ledger-only checkpoint records the result. This section is below
  the previous first-use entry to avoid overlapping #199's new ledger section.
- Reproduced the shared selector's off-page anchor bug on actual indexed media:
  select one item on page 1, then Shift-click the second item on page 2. It selected
  only two items overall, omitting the first visible item on page 2. The missing
  anchor became `-1` in Array.slice. Use a small pure range helper: absent anchors
  begin at the first visible item, stale targets add nothing, and both endpoints
  are included in either direction. Keep existing deduplication, off-page items,
  typed keys, anchor retention and ordinary selection actions.
- Reproduced starting selection with a Video being blocked by the invisible
  built-in preview wrapper, which inherited full-card dimensions. Bound the
  existing button at the bottom right, pass pointer events through its wrapper,
  and size/show its icon within the hover/focus affordance. This also avoids
  covering the top-right Artist link. No native Image-preview or booru layout
  change, player lifecycle rewrite, selection store, API or relationship change.
- Added five unit cases and a read-only mounted suite against a seeded isolated
  server. Both 1440 × 1000 and 390 × 844 passed All/native Images page ranges,
  retained off-page items, independent equal numeric Image/Video IDs, card clicks
  during selection, filter URL persistence and selection clearing on reload,
  Image Back/Escape/reopening, starting selection with a Video in All and native
  Videos, compact preview bounds, native Video preview opening and Escape cleanup.
  Browser page-error assertions passed; captures inspected. Narrow viewport tests
  use Chromium keyboard/mouse input, not a physical touch device.
- Actual-library check used four geometric PNGs and two short H.264 Videos.
  A separate one-off title-only mixed edit checked Cancel preserving selection
  and metadata, both native bulk writers on Apply, and fresh reads after reload
  preserving two ordered Artists plus Character, Copyright and general Tag IDs
  on both image:1 and scene:1. This does not establish every relationship mode.
  Existing all-media browser suite passed delayed/empty/Video-only/error queries,
  mixed selection, Wall navigation/Back, Video previews/carousels and legacy routes.
  Its injected response cases remain distinct from unmocked indexing/save checks.
- Validation: `make validate-ui` passed all 66 tests, JS/CSS lint, TypeScript and
  formatting. Production `make ui-only`, UI generation/built-in preparation,
  generated built-in syntax, focused Biome and `git diff --check` passed. The
  composed JavaScript checksum is unchanged; only built-in CSS changed. Runtime
  acceptance passed again using #199's downloaded Linux CI binary with this
  production UI through STASH_UI; no backend/schema source changed. Browser suite
  is optional with documented seeded-server requirements; unit cases run in CI.
- #199 follow-up: initial 12 ordinary checks and Codex passed, but the added
  first-library job exhausted ten minutes while downloading Ubuntu FFmpeg
  packages before running any browser test. Pushed narrower dependency install
  (`--no-install-recommends`) and a thirty-minute cold setup budget at
  `befaa1d8640e01895ef412ea4e182141970d919b`, tree
  `6b587af73a45e2dfa12f01d3b5cb096c3db6385d`. YAML/install/budget assertions
  passed. Its fresh CI/Codex is pending; the cancelled run is not a pass or a
  reason to bypass the merge gate. That fix stays on #199, separate from this PR.
- Next: remaining native hierarchy/inheritance editing and related-list return
  acceptance, then owner data/real-model E08 checks. Separately investigate the
  observed ignored --ui-location flag: command-line loading stores the hyphenated
  name while GetUILocation reads ui_location; STASH_UI worked for these checks.
  Do not fold that backend/configuration correction into this selection pass.
  No owner production/upgrade/recovery/physical GPU acceptance is claimed. End
  once only external CI/Codex remains; inspect viable findings before merging.

## Mask-restoration output guidance (E08, third pass) — 2026-10-23

- Merged [PR #195](https://github.com/Dusky-dev/StashBooru/pull/195) at
  `dfc599bec2a8be4bd54de1da587244ae08ac67cb` after all 12 CI checks passed and
  Codex Review completed without findings at its final head. This fresh develop
  baseline includes the converter and upscaler guidance passes. No other PRs
  were open when checked; unrelated local screenshots were preserved.
- Branch [`fix/restoration-guidance-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/restoration-guidance-20261023).
  Verified source checkpoint `5e4278a91750b69bf13ebc989333c417e359a191`, tree
  `92bf01c43c9a529dfe3d9bace5de08ede160a697`. Connector-created/local source
  trees match; this ledger-only checkpoint is included before opening review.
- State the result before preparation: Save creates a new PNG beside the source,
  copies library metadata and keeps the original unchanged. Generated pixels do
  not recover hidden original detail. Label Prepare source, Paint and describe,
  then Review and save within the existing dialog. The same explicit save action
  now says Save as new Image; optional stacking and the typed native result link
  remain. Repeat the destination/original policy at review and explain that
  closing discards unsaved work; remove that footer note after a successful save.
- Keep mask tools and prompt visible. Group hardware, seed, steps, guidance,
  strength, negative prompt, reference and format details under a native
  Generation options disclosure. Its closed summary shows explicit CPU/GPU and
  active reference state. Children remain mounted; disclosure changes preserve
  settings and a valid preview. Existing field changes still dirty the preview
  and require regeneration before saving. Backend selection/model warnings stay
  visible. Requests, source IDs, PNG verification, normalization, metadata and
  provenance copying, stacking, cleanup and derivative-only save are unchanged.
  No replacement mode, new restore cache or booru-browser layout changes.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting. `make ui-only`, generated built-in syntax and `git diff --check`
  passed. GraphQL generation/built-in preparation passed; generated files are not
  committed. Python restoration contract suite passed all nine tests. Workflow
  YAML parsed with its existing contract job and new mounted browser job. No
  backend/schema source changes require Go gates.
- Extended mounted image-restoration suite passed at 1440 × 1000 and 390 × 844:
  initial output/close guidance, no implicit preparation/save, collapsed options,
  keyboard disclosure, retained GPU/seed/prompt/reference across closing the
  disclosure and regeneration, preview invalidation, explicit session-bound save
  and optional stacking, typed result link, mask/undo/feather/zoom/comparison,
  missing-model feedback, reopening defaults, cancellation and late-response
  cleanup. Page/modal width checks passed. Start, mask and review captures were
  inspected on desktop/mobile. No extra dialog or route was added to the flow.
- The existing browser suite now creates deterministic geometric PNG fixtures
  using browser canvas when file overrides are absent. Image restoration CI is
  wired to the touched UI/test paths and runs the suite with screenshot artifacts.
  Browser HTTP processing is mocked; Python tests use real Pillow/transport with
  declared inference stubs. These are not real diffusion output, saved-file or
  catalogue acceptance, physical GPU/VRAM tests or owner production validation.
- Next: E14, inspect native visible Artist/Video labels and localization, then fix
  a bounded set of confirmed terminology mismatches. Broader E07/E11/E12 isolated
  library/owner acceptance and real-model E08 checks remain open; shared history
  requires a separate design. End when only external CI/Codex remains, inspect
  findings next turn, and do not wait/poll solely for review.

## Converter estimate, trial and apply guidance (E08, second pass) — 2026-10-23

- Merged [PR #194](https://github.com/Dusky-dev/StashBooru/pull/194) at
  `ce1ee76cb77e58982f3803733b1c5668dad58ffe` after all 12 CI checks passed and
  Codex Review completed without findings at its final head. This is the fresh
  develop baseline for this pass; no other PRs were open when checked.
- Branch [`fix/converter-flow-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/converter-flow-20261023).
  Verified source checkpoint `a2c7393a188cd850d56d2ccf81d782cffcb28a94`, tree
  `29197683462b71313b6bd2b6fb9a4af12078d885`. Connector-created/local source
  trees match; this ledger-only checkpoint is included before opening review.
- Explain the result first: estimates measure and delete samples; trials retain
  temporary outputs for comparison; explicit Apply replaces active files beside
  their sources, keeping native entries/metadata and originals in the converter
  restore cache until eviction. Keep the original-backup warning visible both
  before launching a trial and beside the existing reviewed-apply controls.
- Output format, resolved per-input plans and savings limits stay visible before
  actions. Batch plans now wrap as readable text rather than low-contrast
  read-only fields. Group processor, worker checks and encoding overrides under
  a native disclosure, initially closed, with explicit CPU/GPU/custom overrides
  in its summary. Children retain state across tabs and rechecks. Capability
  failures, fallback notices and GPU/decode conflicts remain outside it.
- Open Processing defaults at the exact converter section in a new tab, retaining
  the caller and current draft. Keep review available without another estimate
  or trial. Put estimate sampling and apply validation details in disclosures.
  No change to requests, codec/default resolution, savings gates, typed targets,
  explicit trial selection, comparison, staging, caches, file activation or
  restore/unrestore/recovery contracts. No booru-browser layout work.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting; `make ui-only`, generated built-in syntax and `git diff --check`
  passed on the final application source. GraphQL generation/built-in preparation
  passed; generated files are not committed. Final browser screenshot-wait change
  passed focused Biome checks and the full converter suite. No backend/schema
  changes require Go gates.
- Extended the existing conversion-review browser suite at 1440 × 1000 and
  390 × 844: initially hidden controls, keyboard disclosure, visible wrapping
  plans, limits before actions, CPU/custom overrides retained across tabs and
  failed/successful worker rechecks, visible collapsed errors, disabled invalid
  savings, exact new-tab settings destination with unchanged caller/draft,
  no implicit job, estimate with no applyable output, mixed typed trial payloads,
  eligible-only selection, comparisons and explicit apply. Existing asynchronous
  maintenance-error checks passed. Processing-job-resume also passed all four
  converter/upscaler desktop/mobile cases including refresh, prefix isolation,
  reused IDs, history, cancellation identity and blocked storage.
- Desktop/mobile captures were inspected; the existing Job feedback CI workflow
  already runs this suite and uploads its screenshots. A new failure fixture was
  corrected to return the endpoint's plain-text error body. Review captures wait
  for tab transitions before recording. Settings destination is a stub; these are
  intercepted HTTP/component tests, not full Settings acceptance, actual file
  encoding/activation, physical worker/GPU or production-library validation.
- Next: E08 mask-restoration guidance, preserving its derivative-only contract
  and distinguishing generated pixels from original-file recovery. Broader
  real-data acceptance remains open. End when only external CI/Codex remains;
  inspect findings on the next check rather than waiting/polling.

## Native English terminology (E14, bounded pass) — 2026-10-23

- Fresh develop baseline: `dfc599bec2a8be4bd54de1da587244ae08ac67cb` (merged #195).
  [PR #196](https://github.com/Dusky-dev/StashBooru/pull/196), head
  `1d052e64d891d80dbd0f3e457110bf994941246f`, has all 13 CI checks green but
  Codex Review still reports running, without findings or a quota-exhaustion
  response. Left it unmerged under the standing gate and continued this independent
  pass. Its mask-restoration implementation/checkpoint remains on its own branch.
- Branch [`fix/native-terminology-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/native-terminology-20261023).
  Verified source checkpoint `048f086385b54ba70f760b9e87cb3af5939ee773`, tree
  `a960beec25c0066a4dfe95203f7b07adb52797bd`. Connector-created/local source
  trees match; this ledger-only checkpoint is included before opening review.
- Reassessed the audit against current messages: Tools already says Video tools.
  Correct 22 remaining English display messages in a bounded group: Artist
  parent/child hierarchy and counts, Video cover/sprite/preview/transcode labels,
  and Video/Artist/Character view headings. Preserve message IDs, ICU placeholders,
  hierarchy semantics, native Studio/Performer/Scene IDs, URLs and API fields.
- Use the existing en-GB fallback catalogue rather than introducing another
  translation merge layer. All changed keys are absent from the sparse en-US
  spelling catalogue, so it inherits the corrected values. Non-English message
  files and custom-locale override precedence stay unchanged; this is not a
  claim that every locale or inherited label is now translated to fork vocabulary.
- Remove only parent_id/child_ids from the built-in Advanced DOM mover. Those
  fields already belong to the native Artist editor and now stay visibly there,
  independent of English label matching. Other advanced fields keep their current
  behavior. No new editor, hierarchy store or relationship mutation is added.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting; `make ui-only`, GraphQL generation, built-in preparation and
  generated bundle-extra/unifiedMedia syntax passed. The unified composer checksum
  is unchanged because its source parts were not edited. JSON inspection verified
  the same message keys/placeholders and en-US fallback for all 22 changes.
  `git diff --check` passed; no backend/schema changes require Go gates.
- Mounted profile-library-review suite passed 1440 × 1000 at root and 390 × 844
  under /stash: existing six native profile/review contexts and new real native
  Artist edit cases, with the composed Advanced script loaded. Parent/child fields
  stay visible outside Advanced; editing the name and saving retains parent_id 2
  and child_ids [3]. Page width and no uncaught errors passed. Desktop/mobile
  captures were inspected. Existing Tasks navigation suite also passed both sizes,
  including drafts, pending responses, errors, Back, refresh and bookmarks.
- The editor fixture needed dropdown-create defaults and isolated select-query
  responses; its route/base-prefix handling now supports the edit path. Browser
  changes passed focused Biome checks after the full application validation. The
  Settings CI workflow already runs these suites and now also watches the touched
  built-in/native-editor paths. Fixtures capture form submissions; no production
  catalogue write, backend hierarchy acceptance or full All-sidebar test is claimed.
- Next: E14 fork-aware first-use release notes, retaining upstream history and
  existing acknowledgement/version behavior. Remaining terminology/localization,
  E07/E11/E12 real-library acceptance and real-model E08 checks remain separate.
  Inspect #196's completed review before merging it. End when only external
  CI/Codex remains; do not wait/poll solely for review.

## Upscaling output guidance (E08, first pass) — 2026-10-23

- Merged [PR #192](https://github.com/Dusky-dev/StashBooru/pull/192) at
  `ae16406a2c9a36fa6f7f0a53d703569b60ed793a` after its 12 CI checks passed and
  the owner confirmed Codex Review completed. The connector still showed no
  review summary or findings for #192; owner confirmation supplied that gate.
  Merged [PR #193](https://github.com/Dusky-dev/StashBooru/pull/193) at
  `f2c35ef75d3721b67990ae2e37d98f129ab359e8` after all 13 CI checks passed and
  its Codex summary reported completed/no findings. Fresh develop baseline is
  that #193 merge, containing both passes.
- Branch [`fix/processing-output-guidance-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/processing-output-guidance-20261023).
  Verified source checkpoint `fa8e907420caddd020fab16ce57c24ebc974a331`, tree
  `f9457a18f9aa4c0f7da2090a85ec2647106d0952`. Connector-created/local source
  trees match; this ledger-only checkpoint is included before opening review.
- Start Upscale with the existing new-copy versus replacement choice, before
  model/scale/format. Explain that copy outputs are files beside each source and
  separate Images with inherited metadata/relations; replacement keeps the same
  Image entry and its original in the separate upscaling restore cache, subject
  to eviction. Keep the recommended copy default and second replacement confirm.
- Keep model, scale and format visible; put processor, diagnostics, setup link
  and recheck under native Worker and processing options. Children stay mounted
  across checks, retaining choices and disclosure state. Explicit CPU/GPU choices
  appear in its closed summary; a CPU/GPU-model mismatch explains the blocked
  action. The existing strict CPU-capability guard is retained.
- Disable submission during readiness checks. Failed checks clear stale worker
  capabilities until a successful retry. Setup opens its exact Processing section
  in a new tab, preserving the current dialog draft and caller selection.
  Copy/replacement APIs, encoding defaults, typed targets, confirmation, staging,
  original-file recovery, queue/history state and backend processing stay intact.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting. `make ui-only`, generated built-in syntax and `git diff --check`
  passed. Final browser-test interaction adjustment passed focused Biome checks
  and the full mounted output suite. UI GraphQL generation/built-in preparation
  passed; generated files are not committed. No backend/schema changes require
  Go gates.
- Four output-suite cases passed: 1440 × 1000 and 390 × 844, each with one and
  two selected Images. Check output-first DOM order, initial copy/closed advanced
  state, keyboard disclosure/radio use, exact settings link and retained draft,
  CPU-model incompatibility and recovery, slow/failed/retried readiness checks,
  mounted choices/open state, replacement cancel with no POST, explicit copy and
  confirmed-replacement endpoints/payloads, immutable submitted controls, safe
  default on reopen, unavailable-worker guidance and modal width. Existing
  processing-job-resume suite also passed. Desktop/mobile captures were inspected;
  Job feedback CI now runs the output suite and uploads them with its artifact.
- A cold browser run during concurrent compilation hit its navigation timeout.
  The final isolated run passed; radio selection after cancelling confirmation
  now waits for the native checked state instead of racing focus restoration.
  These are intercepted HTTP/configuration and fixture-dialog checks, not actual
  file output, metadata propagation, physical model/GPU or production acceptance.
- Next: E08 converter estimate/trial/apply presentation, preserving staging,
  reviewed apply, original retention and cache contracts. Mask-restoration
  guidance and broader real-data acceptance remain separate. Browser layout is
  still assigned elsewhere. End when only external CI/Codex Review remains.

## Converter defaults load recovery (E05, bounded pass) — 2026-10-23

- Merged [PR #191](https://github.com/Dusky-dev/StashBooru/pull/191) after all
  13 CI checks passed and Codex Review completed without findings. Fresh develop
  baseline: `4e1aff212cdefb50603eba2d207b2fdd3b9a6235`.
- Branch [`fix/converter-settings-recovery-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/converter-settings-recovery-20261023).
  Verified source checkpoint `b1443b67adb4aba36e67fa6bf95186b1b5b04e8b`, tree
  `8ca4d90d8b330594d4a9077dc2be0abd29f5c0bb`. Connector-created and local source
  trees match. This ledger-only checkpoint is included before opening review.
- Initial converter defaults read failures now show a contextual error and Retry
  loading defaults at the start of the card. Retry has a labeled loading status,
  disables repeat submission while pending and stays available after repeated
  failure. Conversion controls remain disabled until saved defaults load.
- Retry reuses the existing settings GET and decoder, with a shared abortable
  loader for first mount and retry. Successful load removes the error/action and
  restores the server's saved values. Read errors are distinct from save errors;
  no reload action is exposed over an already editable converter draft.
- Keep the page and other Processing drafts intact. Retry performs no POST or
  job submission, and Settings unmount aborts a pending read. Existing converter
  save transactions, codec capability discovery, browser-owned upscaling defaults,
  conversion/original recovery contracts and exact-section URLs are unchanged.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting; `make ui-only`, generated built-in syntax and `git diff --check`
  passed. UI GraphQL generation/built-in preparation passed; generated files are
  not committed. No backend/schema changes require Go gates.
- Mounted Settings suite passed at 1440 × 1000 and 390 × 844, including keyboard
  retry, repeated failed reads, disabled pending retry, delayed successful reads,
  hydration of non-default saved values, GET-only retry, unchanged page URL,
  other-section draft retention, converter draft retention across tabs/Back and
  pending-read cancellation on unmount. Existing settings/navigation/save-scope,
  upscaler save and error cases remain covered. Desktop/mobile error captures
  were inspected and are emitted into the existing Settings navigation CI artifact.
- An initial cold fixture run during concurrent local compilation timed out before
  mounting; subsequent desktop/mobile runs passed. These are intercepted HTTP,
  partial configuration and browser-storage fixtures, not production first-load,
  real-server restart, physical codec/GPU or representative-library acceptance.
- Next: reassess E06 Following entry points and source capability guidance from
  existing native/source contexts. Keep browser layout with its assigned agent.
  Broader E05 consistency and E03 real-job/restart/history decisions remain open.
  CI/Codex Review is a turn-end checkpoint; do not wait/poll solely for those gates.

## Upscaler settings save feedback (E05, bounded pass) — 2026-10-23

- Merged [PR #190](https://github.com/Dusky-dev/StashBooru/pull/190) after all
  13 CI checks passed and Codex Review completed without findings. Fresh develop
  baseline: `54b4322cdc5b64e1f6f0df6a8499780be43fadd4`.
- Branch [`fix/processing-settings-feedback-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/processing-settings-feedback-20261023).
  Verified source checkpoint `df075dc5a03102b40b67270685fbd06d45ff7256`, tree
  `b27077ddeae010a70cdcdbbd50f8ed360df82b4b`. Connector-created and local source
  trees match. This ledger-only follow-up is included before opening review.
- Reassessed E05 against existing Processing controls: scope labels, separate
  Save actions and exact-section navigation already exist. Fixed the upscaler
  save path instead of adding another settings transaction or ownership layer.
- Saving local executable/model paths consumes the normalized server response,
  retains the expanded form and shows inline server-save feedback. Editing a
  path clears that feedback. Follow-up readiness checks do not fetch saved paths
  or overwrite newer drafts; readiness displays checking until the response.
- Keep readiness errors distinct from rejected saves. A successful save remains
  confirmed if readiness fails; a rejected save retains the editable draft.
  Explicit Reload worker setup replaces path drafts and disables path editing
  while loading, with explanatory text beside the actions. Browser job defaults
  retain their independent storage/save action. Correct stale System guidance
  in the upscaling dialog to Processing; its existing exact-section link stays.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting. The final feedback-contrast adjustment subsequently passed
  focused JS lint, repository formatting, `make ui-only`, mounted Settings checks
  and `git diff --check`. UI GraphQL generation/built-in preparation and generated
  built-in syntax check passed. No backend/schema changes require Go gates.
- Mounted Settings maintenance suite passed at 1440 × 1000 and 390 × 844:
  existing navigation, Back/refresh/bookmarks, independent scopes and errors;
  plus normalized save responses, no config re-fetch after save, expanded form
  retention, slow readiness responses with newer edits, checking labels, failed
  readiness after successful save, rejected-save draft retention, and explicit
  reload recovery. Existing processing-job-resume suite also passed. Final
  desktop/mobile save-feedback captures were inspected; the existing Settings
  navigation CI uploads them with its screenshots artifact.
- Fixture HTTP/configuration and browser storage only: no real model paths were
  changed, no physical GPU/worker inference or production-library acceptance is
  claimed. No processing/recovery contract or API changes; broader E05 consistency
  and E03 real-job/restart/history decisions remain open.
- Next: inspect converter defaults recovery after initial settings-read failures
  as a separate E05 pass, preserving drafts and independent saves. Browser layout
  remains separately assigned. CI/Codex Review is a turn-end checkpoint; do not
  wait or poll solely for external gates.

## Contextual Following setup (E06, first pass) — 2026-10-23

- Fresh develop baseline: `4e1aff212cdefb50603eba2d207b2fdd3b9a6235` (merged #191).
  [PR #192](https://github.com/Dusky-dev/StashBooru/pull/192), head
  `8b9a30a12fa24415ab6153fcbec4fa3e0c687534`, has all 12 CI checks green but no
  Codex Review result or explicit quota response. It remains open under the
  standing review gate; this pass proceeds independently instead of waiting.
- Branch [`fix/following-context-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/following-context-20261023).
  Verified source checkpoint `635d6830e6ac009971a9f10eb66c026d560f2087`, tree
  `2744c86a507daa370effb9bff995bc09613ccf8c`. Local/connector source trees match;
  this ledger-only checkpoint is included before opening review.
- Native Artist, Character and Copyright Sources & aliases now offers Follow
  beside saved Danbooru tag connections. Unsupported provider/account connections
  state that feed checks are unavailable. No canonical name or account ID is
  guessed into a tag, and opening the profile still only reads its connections.
- The link opens the existing Following form in a new tab using a stored identity
  ID. It reads that one connection, prefills exact target/category and a readable
  name, and selects a current enabled source for the same provider instance.
  Renamed source settings work; a reused ID for a different instance cannot
  silently retarget a follow. No new form implementation, API or adapter is added.
- Save remains explicit. Context/source errors block it and offer an explicit
  blank start; exact duplicate definitions link to their saved follow, including
  inactive records. Successful saving clears the context URL. Reload saved feed
  retains edits, while refresh reconstructs the explicitly linked connection.
  Existing edit/version/cursor, check/download and remote favorite contracts stay.
- Reuse native dark input/select classes for focused and disabled fields; focus
  and scroll the prefilled manager without losing the original profile tab.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting. Final native input-class changes passed focused Biome checks,
  `make ui-only`, generated built-in syntax, mounted context checks and
  `git diff --check`. UI GraphQL generation/built-in preparation passed; generated
  files are not committed. No backend/schema changes require Go gates.
- New mounted suite passed at 1440 × 1000/root and 390 × 844/under /stash for all
  three categories: raw target/name/type, original-tab fixture input retention,
  source ID reuse and rename, explicit creates/no automatic sync, duplicates,
  refresh, source reload with drafts, dark fields, no horizontal overflow,
  missing/unsupported/invalid/failed identities, no available source, and abort
  on leaving. Existing Following and identities suites passed both viewports.
  Following CI runs the new suite and uploads draft/profile-link captures;
  final desktop/mobile captures were inspected.
- These are real IdentitySources/Following components with fixture configuration
  and intercepted APIs. The retained profile input is fixture-local, not a full
  native edit-form acceptance claim. No live provider/account/GPU/production
  acceptance, additional engines or account follows are claimed. Text merge with
  pending #192 was checked clean; it is not runtime acceptance of a combined tree.
- Next: reassess E08 processing dialogs' output/destination/original-file guidance.
  Broader E06 action reuse and additional adapters remain separate; browser layout
  stays with its assigned agent. End when only external CI/review remains.

## Contextual library review (E04, profile entry points) — 2026-10-23

- Started against PR #189's reviewed head while its CI completed, as requested.
  All 13 checks subsequently passed; merged #189 with no Codex findings at
  `5256fe4e750967c81b4095de43cfca744ef4a9ee`. Fast-forwarded the isolated branch
  onto that fresh develop baseline before committing this pass.
- Branch [`fix/contextual-library-review-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/contextual-library-review-20261023).
  Verified source checkpoint `e3b971c75cc7b41718e2850bc706a3c68751ed57`, tree
  `f75d58284e46c18831794a53abe442f9bbf09ab0`. This ledger-only follow-up is
  published before opening the PR for review.
- Add a small Library review disclosure beside the existing profile details for
  native Characters, Artists and Copyrights. Inspect this name opens the existing
  alias collision inspector with a draft; inherited-association review links to
  the existing task for all three types. Only Characters get duplicate review.
  Both cleanup actions explicitly say whole library. All links open new tabs.
- Supply the profile name through an encoded alias query parameter; initialize
  the inspector draft only on first mount, retaining user edits on ordinary
  Settings rerenders. Lookup, ambiguity decisions, preview/apply and merges stay
  explicit. No profile ID is reinterpreted, no profile-scoped merge is promised,
  and no new task engine, schema, API or persistent store is introduced.
- New review sections occupy their own flex row beneath Sources & aliases. The
  first screenshots exposed adjacent desktop sections; the final layout fixes
  that and a mounted geometry check protects the row boundary.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting. After the final row-layout adjustment, CSS lint, formatting,
  the mounted profile suite, `make ui-only` and `git diff --check` passed on the
  final source. UI GraphQL generation/built-in preparation passed; generated
  files are not committed. No backend/schema change requires Go gates.
- Six native detail-panel cases passed: Character/Artist/Copyright at
  1440 × 1000 with root URLs and 390 × 844 under /stash. Check keyboard opening,
  type-specific link counts, UTF-8/reserved-character name round trips, first
  explicit lookup with ambiguous results, prefixed REST URLs, native task
  anchors, no automatic preview or POST, original tab URL/fixture-field retention,
  separate profile rows and no horizontal overflow or uncaught page errors.
- Existing Tools, Tasks, Settings maintenance and Character merge suites passed
  both viewports with the extended native-profile fixture. Settings navigation
  CI now runs all five suites. Desktop/mobile captures were inspected.
- These are native detail panels against partial fixture configuration and
  intercepted reads; the retained field is fixture-local, not acceptance of a
  complete native profile edit form. No real library mutation, provider/model,
  GPU, catalogue-scale or production restart acceptance is claimed.
- Next: reassess E05 save-scope feedback and exact settings links against current
  code before adding behavior. Broader review-result handoffs and E03 real-job
  restart/history-store decisions remain open; browser layout is separately
  assigned. CI/Codex Review is a turn-end checkpoint, not a reason to wait/poll.

## Tools organization (E04, second pass) — 2026-10-23

- Merged [PR #188](https://github.com/Dusky-dev/StashBooru/pull/188) after all
  CI passed and Codex Review completed without findings. Fresh baseline:
  `develop` at `187d518b500d752240c63f01614ee208e6673633`.
- Branch [`fix/tools-navigation-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/tools-navigation-20261023).
  Verified source checkpoint `23bd9bf52475e18f3f668d4df0ced80869dca096`, tree
  `8bc8e4674cc4b2cd5bfb2425126704fe2c4bcfea`. This ledger-only follow-up is
  included before the PR is opened for review.
- Put Metadata Health, Visual Similarity Triage, Alias Collision Inspector and
  Developer tools in native expandable sections. GraphQL remains in Developer
  tools; the two native Video utility routes and new-tab playground link remain.
  Update the three English Video utility labels using their existing locale IDs.
- Extract Tasks' existing disclosure into SettingsDisclosure and share its
  presentation with Tools. Preserve the two existing SettingsToolsSection plugin
  extension points. Most Tools body changes are indentation under the sections;
  the existing request, matching, ranking and copy functions retain their behavior.
- Collapsing retains pending responses, drafts, selections, errors and results.
  Keep Tools mounted across Settings tab changes so Back retains the current
  review. Leaving Settings or refreshing still ends that temporary React state;
  no persistent store, engine, API or automatic work is introduced. Dark native
  input styling now applies to the touched alias and threshold controls.
- Validation: final `make validate-ui` passed all 61 unit tests, JS/CSS lint,
  TypeScript and formatting; `make ui-only` passed. UI GraphQL generation and
  built-in preparation passed, with no generated files committed.
  `git diff --check` passed. No backend/schema changes require Go gates.
- Native Tools suite passed at 1440 × 1000 and 390 × 844: no automatic scans on
  entry/open, keyboard disclosure, failed/read-only metadata checks and explicit
  retry, pending responses while collapsed, ambiguous exact-name results and
  drafts, Settings tab/Back retention, similarity filters and selected targets,
  rejected/accepted metadata-copy confirmations with exact request identities,
  typed Image links, Video utility/playground routes, bookmarks, refresh and
  horizontal layout. Desktop/mobile captures were inspected.
- Existing Tasks, Settings maintenance and Character merge suites passed both
  viewports after sharing the disclosure. Their coverage retains preview/apply,
  pending responses, errors, drafts, section navigation, save scopes, backup
  cancellation and native Character merge error recovery. Settings navigation
  CI now runs all four suites and uploads captures.
- Configuration, HTTP responses, media thumbnails and metadata-copy writes are
  fixtures. No live provider/model/GPU, real catalogue writes, real plugin
  extension execution, large-library performance or production restart acceptance
  is claimed. Existing panel teardown outside Settings is unchanged.
- Next bounded E04 follow-up: contextual cleanup links from relevant native
  profiles/review pages, using the task anchors already delivered. E03 real-job
  restart acceptance/history-store decision and browser layout remain separate.
  End the turn when only external CI/Codex Review remains; do not wait or poll.

## Tasks grouped by goal (E04, first pass) — 2026-10-23

- Merged PR #187 after all CI passed and its only Codex Review finding was
  verified as already addressed by the ledger commit, then resolved. Fresh
  baseline: `develop` at `3b2c76311fe150f69b597326bc62d72099976300`.
- Branch [`fix/tasks-navigation-20261023`](https://github.com/Dusky-dev/StashBooru/tree/fix/tasks-navigation-20261023).
  Verified source checkpoint `148da94cfff9db50c834e437f81255195da0f106`, tree
  `9c0a8c82b66be11cc99fa24fd454bf0d261bfefa`. This ledger follow-up is
  documentation only; it is included before opening the PR for review.
- Keep the native queue first. Group the existing launchers into Update library,
  Review profiles & associations, Inspect media, Backup & maintenance and Plugin
  tasks. Native expandable sections start closed and retain their mounted task
  components, in-flight responses, drafts and review results when closed.
- Keep each tool's implementation, defaults, explicit preview/apply/confirmation
  and queue ownership. The plugin section explains when no tasks are available.
  No backend, schema, job engine, recovery, metadata or save-scope changes.
- Existing target IDs remain valid. The native Settings anchor handler reveals
  all enclosing native details before positioning a bookmark; advanced mode
  reruns alignment when it mounts an advanced target inside a closed group.
  Preserve unrelated URL parameters, Back and refresh. The queue scrolls in
  normal flow to keep lower bookmarked actions visible rather than covering them.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting; `make ui-only` passed. UI GraphQL generation and built-in
  preparation passed; no generated files are committed. Final formatting and
  `git diff --check` passed. No Go gates apply to these UI/documentation changes.
- Mounted native Settings, Tasks and Character merge suites all passed at
  1440 × 1000 and 390 × 844. Checks cover initial closed groups/queue placement,
  Enter/Space, response completion while closed, retained review/draft state,
  Back, failed animation submission retention without resubmission, backup
  dialog cancellation, plugin empty guidance, old task/queue/advanced bookmarks,
  refresh, anchor visibility and no horizontal overflow or uncaught/unmounted
  update errors. Existing settings save scopes, errors/retries and Character
  preview/pagination/apply/error recovery also passed. Captures were inspected;
  Settings navigation CI runs all three suites and uploads its captures.
- HTTP/GraphQL responses and writes are isolated fixtures. These checks do not
  establish populated-library, enabled-plugin execution, real conversion/GPU,
  real database backup or production restart acceptance. Existing native queue
  behavior is reused; no new durable job history is claimed.
- Remaining E04 work: organize Tools and link cleanup actions from relevant
  profile/review contexts, each as a bounded pass. Next: Tools organization.
  E03 representative real-job/restart acceptance and the history-store decision
  remain open. Browser layout remains separately assigned. CI/Codex Review is a
  turn-end checkpoint; do not wait or poll solely for those external gates.

## Compact settings navigation (E13, navigation pass) — 2026-10-22

- Merged [PR #186](https://github.com/Dusky-dev/StashBooru/pull/186) following
  the owner's request. All CI passed; Codex Review completed on its source
  checkpoint with no findings. Its final follow-up was ledger-only. Merge
  `83f784ee98f3a3d9d5ae4dd94ac5a704f3a78d05` is this pass's fresh baseline.
- Branch `fix/mobile-settings-navigation-20261022`,
  [PR #187](https://github.com/Dusky-dev/StashBooru/pull/187). Verified source
  checkpoint `bba09a014417e8d29c803396fe29c487d55deb66`, tree
  `0ff84675583c4fc00456040365418aa514a370a9`. This subsequent ledger update
  changes documentation only.
- Replace the thirteen stacked settings links with a labeled, dark native
  section chooser below 768 px. Reuse the same section order/localized labels
  in the wider-screen sidebar. Keep advanced mode and troubleshooting beside
  navigation and one existing implementation/state owner per settings panel.
- Mounted checks reproduced every sidebar link being highlighted because the
  router matched only the common pathname. Explicitly match the selected tab.
  A narrow tablet sidebar also crowded the form; keep the compact/full-width
  layout through 767 px. Only one navigation control is exposed at each width.
- Both controls retain section URLs and unrelated query parameters, drop an old
  section's hash on explicit section changes, and preserve Back/refresh and
  existing moved-section bookmarks. Resizing does not remount panels or discard
  unsaved drafts. Existing saves and automatic advanced-mode persistence stay
  with their original owners; no combined save or new persistence is added.
- Validation: final `make validate-ui` passed all 61 unit tests, JS/CSS lint,
  TypeScript and formatting. Final `make ui-only` passed. GraphQL UI generation,
  built-in preparation and `git diff --check` passed. No backend/schema changes
  require backend generation or Go gates.
- The actual Settings fixture passed at 1440 × 1000 and 390 × 844, including
  keyboard selection, advanced mode, Back/refresh, moved bookmarks, anchor
  alignment, retained URL parameters and drafts, existing independent save
  scopes, worker/configuration errors/retries and unmount cleanup. It also
  checks 575/576/767/768 px, the single active sidebar section, dark chooser,
  mobile content beginning within the first 300 px, and absence of document
  horizontal overflow and uncaught/unmounted-update errors. Desktop/mobile
  captures were inspected; the new Settings navigation CI workflow uploads them.
- Configuration, HTTP/GraphQL responses and writes are fixture-local. No real
  provider/worker/GPU, populated-library or fresh-server acceptance is claimed.
  E03 representative real-job/restart acceptance and the durable-history decision
  remain open; current evidence does not justify another history store yet.
- Next bounded cleanup: E04 Tasks launcher grouping by goal, keeping the queue
  first, existing bookmarks and active review/job state. Broader settings/save
  scope changes and separately assigned browser layout remain separate work.
  CI/Codex Review are external gates: end the turn without polling solely for them.

## Processing job resume identity (E03 recovery pass) — 2026-10-22

- Owner explicitly requested merge of PR #185. All CI passed and its viable P2
  finding was fixed/resolved; merge commit
  `96973ea0d291b84ae73700a8da8eb414c27a3eba` is this pass's baseline. Its latest
  follow-up review was still Running at merge; the direct owner instruction
  governed that merge, without changing the default review gate for later PRs.
- Branch `fix/processing-job-resume-20261022`,
  [PR #186](https://github.com/Dusky-dev/StashBooru/pull/186).
  Verified source checkpoint `5c1a00d0c3c0e895eaf0dd66aeac3e18f6a6f1aa`, tree
  `a9b3bf1eb62295ba80901a34139e9a7591d2ea01`. This subsequent ledger update is
  documentation only.
- Converter and upscaling-restore status/cancellation now require the remembered
  numeric job ID to match the existing per-job batch identity. Shared registry
  checks happen before cancellation reaches the native queue. Wrong, missing or
  expired identities cannot expose or cancel a replacement job with a reused ID.
- Job submissions return ID and batch. Positive-ID status requests without a
  matching batch return no live job plus jobUnavailable while retaining saved
  history/configuration; invalid cancellation returns HTTP 409. Numeric-only
  custom API clients must retain the batch and update their requests; ordinary
  history/config reads and native new-copy upscaling handoff remain available.
- A shared browser helper remembers the pair under its exact API address,
  including a server prefix. Ignore old unscoped numeric-only references. Resume
  after refresh/reopen only when identity matches; otherwise clear live status
  and explain that saved records/available original recovery remain accessible.
  Storage failure keeps the accepted job in memory and never resubmits it.
- No durable job-history store, schema, native IDs/relationships, worker protocol,
  automatic retry, media activation or original-file recovery contract changes.
- Validation: focused TestProcessingJob registry checks passed under the Makefile
  SQLite include flags/tags. Full `make test`, `make it` and `make lint` passed
  (final lint: 0 issues). Initial parallel unit/lint compilation exceeded available
  memory; sequential low-memory reruns passed. Lint's local variable-name finding
  was corrected. Backend generation passed; no generated files are committed.
- `make validate-ui` passed 61 unit tests, JS/CSS lint, TypeScript and formatting.
  `make ui-only`, documentation target checks and `git diff --check` passed.
- Native processing-job-resume fixtures passed at 1440 × 1000 and 390 × 844 for
  both converter and upscaling restore: legacy pointers, refresh/reopen, distinct
  server prefixes, reused numeric IDs, retained history, cancellation identity
  and blocked browser storage without duplicate submission. The existing native
  converter workflow, including asynchronous configure/restore/recover failures,
  passed both viewports after updating its batch-qualified responses. Captures
  were inspected; CI uploads them with the job-feedback screenshots.
- Native registry tests reject stale/expired/wrong-feature references, protect
  independent result snapshots and cancel a bounded actual native queue task only
  with a matching identity. Browser HTTP responses simulate restart/ID reuse;
  this is not an end-to-end production server restart, GPU inference or actual
  media-file recovery acceptance.
- Remaining E03 work: representative reconnect/restart acceptance for other job
  producers and remaining context handoffs, then assess whether durable server
  job history is needed. External CI/Codex Review is a turn-end checkpoint; do not
  wait or repeatedly poll solely for it.

## Predictable metadata review (E02) — 2026-10-22

- Baseline: `develop` at `d6e8c9c49efc332ef510dfdf264dc0392b15ea22`,
  after owner merge of PR #183. Branch
  `fix/metadata-review-entrypoints-20261022`,
  [PR #184](https://github.com/Dusky-dev/StashBooru/pull/184).
- Verified implementation commit:
  `17199c725581764c24dd8f59df79dedae6c69831` (tree
  `5abb0a9a7c54313f9a49a5db103641a107aa0fe0`). This ledger update changes
  documentation only after that verified implementation.
- Add the same Review metadata action to Image/Video details and selections in
  All, Images and Videos. Reuse existing native dialogs and review/apply plans,
  one item at a time. Display the typed native item ID, selection progress, Skip
  and a setup link opening existing Processing settings in a new tab.
- Snapshot typed identities/order on opening. A list prop reorder cannot redirect
  the active review or a write, including equal Image/Video numeric IDs. Apply
  advances only after success; a failed save stays on the identifiable current
  item. Skip writes nothing. Close ends review and retains the list selection,
  filters and URL. Dismissal is blocked during an apply. Previously applied items
  remain applied when the review ends.
- Local filename metadata remains the only source loaded on opening. Provider
  lookup, inference and library suggestions remain explicit. Rename the provider
  button to Look up by MD5, clarify native Auto Tag as paths/filenames, and make
  detailed source-merging help expandable. Preserve the Generate review shortcut
  and existing advanced batch-job defaults; label the Video action Batch tagging
  job to distinguish it from per-item review.
- The image dialog now owns its native styling class. Remove the obsolete
  English-title-based DOM class adapter from the built-in extra script. The main
  composed unifiedMedia checksum remains unchanged.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting. Final source also passed targeted Biome checks. `make ui-only`
  passed on the final UI source. Built-in preparation and generated unifiedMedia
  and bundle-extra JavaScript syntax checks passed; `git diff --check` passed.
- The new mounted metadata-review suite passed at 1440 × 1000 and 390 × 844:
  native action visibility, mixed typed identities, in-progress input reorder,
  explicit plans, failed/successful apply, Skip, Close, selection/URL retention,
  legacy Generate shortcut, Image/Video-only selections, provider error retention
  and repeated single-item opens. No uncaught page errors, React unmounted/update
  errors or horizontal overflow were observed. Steady-state desktop/mobile
  screenshots were inspected; CI uploads them as metadata-review-screenshots.
- Fixture HTTP metadata and writes are intercepted. These checks are not live
  provider, populated-library, physical-device or GPU/model acceptance. No backend
  schema, tagging engine, native relationship/identity, file activation or original
  recovery behavior changes. Browser layout remains with its separately assigned
  owner.
- At publication, CI and Codex Review remain external gates. End the turn rather
  than waiting/polling solely for those gates. Next bounded objective: audit pass
  E03, inventory native job retention and unify completion/error/recovery feedback
  before considering additional persistence.

## Contextual source connections (E01) — 2026-10-22

- Baseline: `develop` at `015336533de2f4823818866da5205b8c766b5921`,
  after owner merge of audit PR #182. Branch
  `fix/contextual-source-connections-20261022`,
  [PR #183](https://github.com/Dusky-dev/StashBooru/pull/183).
- Connect Artist/Character/Copyright opens a dialog over the existing provider
  preview. Native profile Sources & aliases manages and adds exact source names
  in a dialog, with an explicit decision to connect to the current profile.
- Extract the existing resolution controls into one shared component for
  dialogs and advanced bulk reconciliation. Keep the original typed identity API,
  native IDs, source targets, optimistic versions and media assignments.
- Conflict/error replies retain the draft and require an explicit version reload
  before retry; reload never reapplies the mutation. Close is blocked during a
  write. Idle cancellation creates no native entity or connection. Observing a
  name can retain local unresolved metadata, as the existing browse flow does.
- Automatic merge authority now follows the owner's instruction: applicable CI
  must pass, Codex Review must finish (or explicitly exhaust quota), viable
  findings must be addressed, and merges must be reported. Pending external
  review/CI is a checkpoint to end the turn, not a reason to poll repeatedly.
- Validation on this dated baseline: all 61 UI unit tests passed; JavaScript/CSS
  lint, TypeScript and formatting passed. The mounted identities suite passed at
  desktop 1440 × 1000 and mobile 390 × 844 (including a `/stash` source prefix),
  explicit Artist connection/rejection/restore, Character create/disambiguation,
  Copyright linking, stale-write draft retention and explicit reload, profile
  addition, submitted-write blocking, native scan cancellation and cleanup.
- The full mounted booru suite passed both viewports, including nested connection
  dialog cancellation, first-open keys and focus return to the same preview URL,
  plus existing downloads, settings, search, selection, WebM/GIF and cleanup checks.
  Final overlay stacking, viewport geometry, Escape and keyboard return were also
  checked with the scoped backdrop styles on both viewports. Production bundling
  passed. These are isolated fixtures, not live
  provider, production-library or physical-device acceptance. No backend schema,
  identity rules, native media activation or file recovery behavior changed.
- Codex Review's viable P2 finding is addressed: Advanced reconciliation retains
  an identity resolved by observation, including after a connection decision clears
  the dialog's current row. Desktop/mobile Character and Copyright regressions
  cover missing initial IDs and the resulting focused link.
- Browser layout and later audit passes remain separate work. Pending CI/review
  will be recorded on the PR; do not wait/poll solely for those external gates.

## Job feedback and retention inventory (E03, first pass) — 2026-10-22

### Review follow-up

- Owner explicitly authorized merging PR #184 despite its stalled review. All
  applicable CI was green; merge commit
  `67a054567fd4c796be82303fd9928ccec9ef5dc6` is now included in this branch.
  This exception is specific to #184 and does not change the normal review gate.
- Assessed Codex Review's P2 finding on #185 as valid: asynchronous configure,
  restore and recovery failures were hidden on Statistics & restore. Render one
  shared conversion feedback panel above all tabs, preserving full error copying,
  typed target links and cancellation. Remove the two tab-specific duplicates.
- Regression checks launch each of configure, restore and recovery from Statistics
  & restore, return only an accepted job ID initially, then fail via a later poll.
  At desktop 1440 × 1000 and mobile 390 × 844, verify the launching tab remains
  selected, the full multiline error is visible and copied, the actual restore
  record ID is submitted, and one feedback panel remains available across all tabs.
- On the integrated source, `make validate-ui` passed 61 unit tests, JS/CSS lint,
  TypeScript and formatting; `make ui-only` and `git diff --check` passed. The
  extended mounted converter workflow passed both viewports. These are intercepted
  browser fixtures, not live original-file restoration or production acceptance.
  New CI is required after publication; end the turn if only external gates remain.

### Original implementation checkpoint

- Baseline: `develop` at `d6e8c9c49efc332ef510dfdf264dc0392b15ea22`.
  Branch `fix/job-feedback-20261022`,
  [PR #185](https://github.com/Dusky-dev/StashBooru/pull/185). This pass is
  independent of metadata-review PR #184: all its CI checks passed, but Codex
  Review still reported Running with no posted findings during this session.
  PR #184 was not merged; delayed review is not the quota-exhaustion exception.
- Verified implementation commit:
  `d21dc6430afa1418965b2b38309689bee7a1a1ea` (tree
  `0acd084e92cc8e4ad9ba27b6b689c033d9d9b31e`). This ledger update changes
  documentation only after the verified implementation.
- Reuse shared readable status/error actions in the native queue and conversion
  feedback, including processing footers. Completed conversion responses with
  item errors show Completed with errors. Preserve unknown state names rather
  than implying success. Queue retention guidance sits outside the scroll area.
- Conversion tabs share progress, cancellation, full multiline error copying and
  typed links to failed native Images/Videos. Display at most 50 error rows but
  copy all returned errors. Trials without actual native targets receive no
  fabricated media link. Leaving through a target closes the processing dialog;
  View job queue opens its existing Tasks anchor in a new tab.
- Upscaling acknowledges a submitted request and links to the queue; it does not
  imply that acceptance is completion. Restore footers use the same status labels.
  Pending cancellation disables resubmission. Native stop requests acknowledge
  submission and wait for server events; failed requests remain retryable.
- [Job feedback guide](job-feedback.md) inventories process-memory queue/results,
  tab-scoped failed notices, on-disk conversion/trial/original records, separate
  upscaling restoration, download receipts and identity observations. No new
  persistence, schema, worker protocol, native target parsing, automatic retry,
  media activation or original-file recovery behavior is introduced.
- Validation: `make validate-ui` passed all 61 unit tests, JS/CSS lint, TypeScript
  and formatting. Final footer/layout adjustments also passed lint, TypeScript
  and formatting. Final `make ui-only` passed. Documentation local-link targets
  and `git diff --check` passed; dependencies remain locked and unchanged.
- Mounted job-feedback, jobs-stacks and conversion-review browser suites all
  passed at 1440 × 1000 and 390 × 844. Coverage includes partial completion,
  multiline/full hidden-row error copying, typed Image/Video links and Back,
  queue links, unknown states, pending cancellation, native expiry/dismissal,
  failed-notice refresh retention, clipboard rejection/fallback/focus, trial
  review/apply/discard and saved settings. The older converter test now expands
  its collapsed trial-storage settings and uses the current save-button label.
  Desktop/mobile captures were inspected; CI uploads job-feedback-screenshots.
- Browser-local HTTP/GraphQL fixtures do not perform production library writes,
  actual conversion/GPU inference, server-restart or file-recovery acceptance.
  No complete E03 or production-ready durable job history claim is made.
- Next E03 slice: validate representative reconnect/restart flows, address custom
  job-ID resume semantics and remaining producer handoffs, then decide whether
  durable server job history is warranted. External CI/Codex Review is a turn-end
  checkpoint; do not wait or repeatedly poll solely for it.

## Whole-project experience audit — 2026-10-22

- Baseline: fresh `develop` at `b36daf9f0d548df25facc73fc98857764c4b61ae`,
  including merged PR #171. Branch `docs/experience-audit-20261022`.
- Owner asked to evaluate inherited and added behavior before more expansion.
  [Experience audit](experience-audit-2026-10-22.md) maps daily workflows,
  prioritizes confirmed fragmentation separately from risks/validation gaps,
  and defines bounded consolidation passes and cross-feature acceptance.
- First proposed implementation: contextual source-name connection using the
  existing identity backend, preserving native IDs and reviewed decisions.
  Browser layout implementation remains with the separately assigned agent.
- Correct README, documentation index, feature/browser guides and roadmap claims
  that Following or source-name bindings are still unmerged/planned. Historical
  dated entries below remain checkpoints, not current deployment claims.
- This PR changes documentation only. Validation and runtime-inspection boundaries
  are recorded in the audit; no production media, accounts or model inference
  are involved. A full-app isolated empty-library probe visited 18 routes at
  desktop 1440 × 1000 and mobile 390 × 844: no uncaught page errors or horizontal
  overflow after closing the initial release notes. Native server build passed;
  reused production UI assets match the baseline's tracked UI source. This is
  rendering inspection, not populated-library/provider/playback/GPU acceptance.
- Documentation review, audit local-link target checks and `git diff --check`
  passed on the dated baseline above. Next: implement the first bounded
  consolidation pass; no broad runtime refactor is included here.

## Large archive media-page fix — 2026-10-21

- Remove the browser API's unconditional HTTP 502 rejection when more than
  256 media files occur across a provider search page. A Pawchive/Kemono post
  may contain many attachments and the inspector already displays files
  incrementally; this was blocking valid post searches and downloads.
- Preserve bounded provider-side parsing (maximum 50 posts per request and
  up to 200 attachments per archive post or 200 Pixiv pages), existing media
  host/proxy validation and per-file byte limits.
- Increase server-owned media snapshot capacity from 4,096 to 16,384 to
  retain keys for the largest supported provider page. Replace repeated
  full-map sweeps with a bounded oldest-first FIFO queue, evicting expired
  and overflow keys and compacting queue metadata periodically.
- Add route regression fixture for 375 assets in one page, ensuring
  every post and attachment has a media key, plus a 4,300-key retention
  regression. Verify in CI; no change to provider API rate limits.


## Pawchive 429 rate limit — reduce request pressure — 2026-10-21

- Baseline: `develop` at `5f8f98ea7d14d6c430b72e92a01521cd37ff87fa`;
  branch `fix/pawchive-rate-limit-request-efficiency-20261021`.
- Owner reported `Provider rate limit reached. Wait before retrying.` on
  Pawchive post search after merged PR #177.
- Code audit: for every Pawchive keyword result page, StashBooru
  could issue an additional `/api/v1/posts?o=...` request to check
  whether the API ignored `q=`. This could double request volume;
  on queries matching artist metadata (not post titles), it often did.
- Remove the active probe. A single Pawchive `q=` request per required
  50-post provider page is the normal path. Since some older Pawchive
  APIs may ignore `q=`, flag pages with any result lacking a visible
  title/body match instead of claiming that their filtering was verified.
- Brief in-memory cache of successful public Pawchive keyword pages,
  coalescing concurrent duplicate reads, with 90s TTL, 32-entry limit
  and 1 MiB/entry cap. Do not cache errors or user-specific creator data.
- For a Pawchive 429 without `Retry-After`, honor a 60-second host
  cooldown (not a bypass/retry). The API response passes `retryAfter`
  to the browser. Link the same search on Pawchive's own site from the
  error card so users can continue when API access is blocked.
- Regression tests cover single outbound request per query block,
  ambiguous-index notice, cache hit, concurrent coalescing, distinct
  offset pages, 429 with no retry header, failure non-caching and
  no caching of unfiltered browsing.
- An upstream 429 on the first request is controlled by Pawchive and
  cannot be removed by this patch; respect any provider Retry-After.
  Live provider acceptance remains user-side. CI pending.




## Booru blacklist toolbar toggle — 2026-10-21

- Based on `develop` at `cfba345f70770cd9ea8b7e74140deb5872c98d1b`. Add a blacklist toggle beside the AI-generated
  filter in the booru browser toolbar. Blacklists stay active by default.
  Explicit `blacklist=show` on search and account-favorites API calls reveals
  blocked posts without modifying per-source server settings.
- The on/off state travels in the shareable browser search URL and joins result
  signatures, fetch dependencies and source result component keys, avoiding stale
  card/tag/page state on changes or reload. Input validation rejects malformed
  blacklist filter values.
- Add backend tests for both search/favorites filtering modes, invalid query
  flags and a mounted browser fixture for the toggle, reload and favorites.
  No database migrations or global configuration changes.
- Validation status: PR CI must verify UI/browser and backend tests.

## Booru blacklist input format — 2026-10-21

- Baseline `develop` at `38dcf44039f13b358e4a110decd053fcd478169e`. Follow-up to merged PR #176.
- The source-settings blacklist editor now accepts whitespace-separated
  booru tags (`tag1 tag2 tag3`) instead of one tag per line. Newlines are also
  accepted. Existing stored tags containing spaces are displayed with underscores
  so editing and saving preserves their matching behavior.
- Browser acceptance fixture now covers space-separated entry, reload and
  mixed newline/space entry. Documentation reflects the booru-style format.
- No backend filtering, storage schema or per-source matching changes.
  Validate UI formatting and mounted browser checks in PR CI.


## Pawchive full post search correction — 2026-10-21

- Baseline: `develop` at `38dcf44039f13b358e4a110decd053fcd478169e`;
  branch `fix/pawchive-full-post-search-20261021`.
- Owner confirmed that Pawchive's own Posts search (`https://pawchive.pw/posts?q=meili`)
  returns historical posts across the archive, whereas the first implementation
  of StashBooru PR #175 only inspected 250 recent posts and incorrectly showed
  no matching posts.
- Remove `searchPawchiveRecentPosts` entirely. Normal keyword queries now
  use `/api/v1/posts?q=...` with offset-aligned provider page slices and
  ordinary `HasMore`, like the website's search. Blank/`*` recent browsing
  and direct creator browsing are unaffected.
- Guard compatibility with older Pawchive API versions which silently ignored
  `q`: when no result titles/content contain the query, compare the returned
  identities to the equivalent unfiltered offset page; if identical, return
  a clear unsupported-query error instead of claiming unrelated recent
  posts are hits. Results indexing other provider fields remain permitted
  if their identities differ from the unfiltered feed.
- Add regression tests for a `meili` search spanning 122 provider results
  (including pages past the old 250-post scan), exact offset pagination,
  ignored-query detection, provider-indexed non-title matches and
  `*`/creator browsing. Update source help and user documentation.
- CI and Codex review still to be checked on the new PR. Live Pawchive API
  behavior (site vs API compatibility) should be verified by user after merge.


## Booru browser: per-source tag blacklist — 2026-10-21

- Baseline `develop` at `fee5744b72124fe65bebe8e55ab4650a541001f9`,
  subsequently merged with `develop` at `e7b68a5ba102133360cbd2b26d2a01213ea589ca`.
  The focused PR adds `blacklistTags` per source in server configuration and
  the settings editor, with exact-tag filtering (case-insensitive,
  spaces/underscores equivalent) for normal searches and account favorites.
- Posts whose own or attachment tags are blocked are omitted before
  category lookup, preview media-key projection and browser display.
  The provider's pagination and totals remain unchanged. A page-local
  `blacklistedCount` explains hidden results.
- Settings validation rejects duplicate, malformed and oversized tag lists.
  Regression test sources cover validation, attachment matching, cross-source
  isolation, settings round-trips, provider pagination and favorites.
- CI: source tests, lint and desktop/mobile fixture acceptance should be
  verified on this merged branch; production provider acceptance remains open.
  Codex's missing-fixture-blacklist finding was addressed and resolved.


## Pawchive server-side post searching — 2026-10-21

- Branch `feat/pawchive-server-post-search-20261021`, based on `develop`
  at `fee5744b72124fe65bebe8e55ab4650a541001f9`.
- The Pawchive post adapter formerly rejected text searches. An initial
  implementation forwarded `q=` to `/api/v1/posts`, based on an upstream
  frontend comment. Codex review identified the upstream README explicitly
  states global `q=` is ignored; the first integration run confirmed the
  request returned unrelated real posts. That approach was removed.
- Keyword mode now scans at most **250 recent posts** in five provider
  requests (`o=0..200`), comparing case-insensitive text in titles and
  content. It does NOT send `q=` to the unreliable API or suggest results
  cover the entire archive. Pages and `HasMore` are computed over real
  matches inside that bounded window. A `searchNotice` field and browser
  banner explain the scope. An empty result is not a guarantee the archive
  contains no older matches. Cancellations/errors stop the scan.
- Preserve the existing `*`/blank recent-post mode, `creator:service:id`
  browsing and artist directory chooser, fixed 50-record provider block
  pagination, record/media parsing and bounded request handling.
  Pawchive queries do not inherit Kemono's arbitrary three-byte minimum.
- Source-selection UI no longer clears ordinary text solely because
  Pawchive was chosen; it still clears creator-specific expressions
  when switching sources. Source help and user docs have been updated.
- Regression tests cover mixed matching/unrelated provider rows,
  case-insensitive title/content matches, single-character terms,
  recent-post mode and creator mode, post identity/media parsing,
  malformed creator expressions, correct filtered paging across 250
  recent records and refusal to scan past the cap. An obsolete API
  integration assertion and mounted browser expectation were updated.
- Provider evidence: `dragonjay-lyj/pawchive` frontend source
  calls `/api/v1/posts?q=`, but its README specifically notes the global
  q filter is ignored on the upstream API. CI integration confirmed a
  real `q=landscape` request returned unrelated posts. The new bounded
  scan avoids depending on this unsupported filtering behavior.
  Live UI acceptance after merge remains necessary.
- No merge/deployment is implied; verify CI and reviews first.




Historical B03a checkpoint: shared provider identities, [PR #171](https://github.com/Dusky-dev/StashBooru/pull/171),
branch `feat/provider-identities-20261020`, against develop
`a312a80063418ca68bc70da4821d37aa194140cf`. The owner-authorized #163/#164
Following merges were complete at this checkpoint. PR #171 has since merged into
the 2026-10-22 baseline above; deployment is not established by merge status.
Verification and review history are recorded on #171.

P01–P13 retain the delivered scopes in the table below. P11 real model/GPU
acceptance and P12/P13 production activation remain open gates; they are not
silently promoted to verified production features by this documentation update.

## B03a — shared provider identities — 2026-10-20

- Owner prioritized Artist/Character/Copyright identity unification after merging
  #163 and #164. General tags remain unchanged. Baseline develop:
  `aa1a2b42bf9bfb6b07608350bfbda688e54e91b9`; branch
  `feat/provider-identities-20261020`. No other PR was open at initial inspection.
- Reuse native Artist (Studio), Character (Performer) and Copyright IDs; add
  provider-scoped raw tag/account bindings, not synthetic root tags or global
  aliases. Store exact provider targets and independent source identity.
  Explicit connections are reversible, renamed native records resolve dynamically,
  rejected candidates are retained, and decision versions reject stale writes.
- Migration 100 preserves native records, relationships, general tags and follows.
  New metadata observations never create authoritative native entities or
  relationships. Name normalization is suggestion evidence only.
- First usable slice: linking from post identifiers, creating/linking
  native records, suggestions and bulk review, Sources & aliases on native records,
  plus bounded scanning of saved download receipts and cached feed metadata.
  Remote artist-profile crawling and automatic catalogue reassignment/merges are
  outside this slice. Unknown/missing source metadata is not guessed.
- Resumed on 2026-10-21 with a working execution environment. Integrated fresh
  develop `fee5744b72124fe65bebe8e55ab4650a541001f9`, preserving current Rule34
  live-category priority and Paheal/browser corrections. PR #171 remains draft.
- Linking UI, native profile Sources & aliases, post actions, batch decisions,
  creator directory labels and cancellable native scan jobs are implemented.
  Canonical display names remain separate from original provider labels/targets.
  Cache recovery retains explicit decisions and capacity never hides existing links.
- Published implementation `8f7dbfb0dce69e8e5e4fefa19b5bd941cb12eb79` has tree
  `9620e3182053c9baad50f6dd5f4bdff7373d89b9`, identical to locally tested
  `3b6f75a54299d2632218b13a0b61a1d74200079f`. Fetch and empty tree diff
  verified publication. Both original feature and fresh develop ancestry remain.
- Local checks: `make generate-backend`, `make test`, `make it`, `make lint`
  (**0 issues**), locked-pnpm `validate` (**61 unit tests**, lint, TypeScript,
  formatting), production UI bundle, and mounted Identity, Booru and Following
  desktop/mobile workflows pass. Final hardening was rechecked with focused
  identity/SQLite tests and lint; UI event/label changes reran validation,
  production bundling and the mounted identity suite. No generated-code edits.
- Identity coverage includes transactional rollback after a later target fails,
  native deletion/rename, category/source/service isolation, label priority,
  stale decisions, scan receipt bounds/symlinks, cached feed provenance under
  broken settings, repeated scan/cancel, cache capacity/recovery and native
  job-queue cancellation. Mounted checks cover linking/batch/create/reject/
  restore/unlink, source aliases, native profiles, refresh/back, conflict
  recovery, scan retry and unmount cleanup at 1440px and 390px (`/stash/`).
- First expanded-head CI passed Identity and Following browser workflows; its
  generation job found two fixable test-script lint declarations, now corrected.
  Earlier backend test failure was an old Copyright fixture lacking migration
  100, now migrated and checked locally. The inherited late-autocomplete browser
  failure did not reproduce in the full current desktop/mobile suite.
- Hardened publication `05d3ae8efbc13558daf6b81601b30f73c1d610f7` has tree
  `b63ecc177ca981c230416dec281c3e31a6c6fb3e`, matching locally verified
  `29da2a1438b4849df3e2405277e3edb310572263`. CI generation/UI validation,
  lint and all three mounted browser workflows passed on that revision.
- Codex Review of `8f7dbfb` completed with four viable P2 findings. Addressed
  them in local `c1c957d` (see Git history for the full commit): native scan
  cancellation now returns cleanly; timeout recovery uses a fresh parent-bounded
  read context; feed scans use configured/default source metadata and preserve
  custom-source names; Artist/Character queries evaluate terms with correlated
  EXISTS across all aliases/connections. Also match qualified Character names.
  Regression tests exercise actual native job cancellation, forced write timeout,
  unavailable custom-source provenance, negative/AND/OR searches for both types,
  and Character disambiguation. Focused API/SQLite regressions pass; full Go
  gates subsequently passed on the published revision below. UI source is unchanged.
- All CI checks passed on `4083d0014e1107c952251f8d65eb0c04e6a97a9e`:
  generation (including UI validation/bundle), Go tests/integration, lint,
  Identity/Booru/Following mounted workflows, all seven platform builds and
  release packaging. The four assessed review threads are resolved with fixes
  and regression evidence. A fresh review of the combined head is still required.
- Refreshed develop to `a312a80063418ca68bc70da4821d37aa194140cf` on
  2026-10-21 and merged it into the feature. Preserve newer Pawchive post search,
  source blacklists/toggle and large media page handling. Only this progress
  document needed conflict resolution; all source files merged without conflicts.
  Revalidate the combined tree; authoritative latest results remain on #171.
- Synthetic captures are committed under `docs/screenshots/provider-identities-*`.
  They were visually inspected. Live provider/production acceptance remains
  unverified. Check current-head CI and Codex Review on #171 before merging.
- See [Provider identities](provider-identities.md) for the contract and limits.

## Following merge preparation — 2026-10-20

- Merge current develop into `feat/booru-follow-foundation-20261020`,
  preserving the existing branch head
  `22c2cfdc2b3529bbec57092bde599fb57dbd1a52` and develop
  `52724d798d52ed4f1a4443466f8394942bf33ffa` as parents.
- The only overlapping file is this progress document. Retain both the B06a
  records and all Rule34/browser records. Runtime source changes are disjoint;
  migration 98 remains the next schema after develop's 97.
- The execution environment is unavailable. Verify the combined tree through
  GitHub Actions and current Codex Review before merging #163; then integrate
  the resulting develop into #164 and verify the feed against that base.
- Owner authorization covers these two PR merges, not production deployment.
- Also prepare `feat/following-feed-20261020` against the updated foundation
  while #163 is verified. Preserve both the feed branch's
  `dc2d46af827460986c659e0486e44806c7e888ea` and the updated foundation
  `baa8574b6f4e6f35ee51942eff30335c86b9fd75` as parents.
  Only this ledger overlaps the intervening develop changes. Feed and browser
  source blobs are retained exactly. Verify both branches in CI; merge #163
  first and only then retarget/merge #164. Existing acceptance is synthetic;
  live provider authentication/custom-instance acceptance remains unverified.

## B06b — on-demand Following feed and management page — 2026-10-20

- Branch `feat/following-feed-20261020`, based on B06a's published
  `22c2cfdc2b3529bbec57092bde599fb57dbd1a52` / local
  `bcd1029` (identical tree `7d856e206d3d8ce2906a7bdef2f1e46ffc76a7cb`).
  B06a PR #163 is still open, so this phase is stacked on its feature branch.
  Initial open-PR inspection found no competing follow-feed work. Browser UI is
  assigned to another agent; this phase adds a separate Following page.
- Migration 99 adds provider-independent opaque cursors/payloads, bounded saved
  post snapshots, per-follow/revision match provenance and explicit seen decisions.
  Same instance/post identity merges multiple follows. Seen tombstones survive
  pruning; native media and ignored/downloaded history remain independent.
- Initial fetch adapter: Danbooru-compatible sequential ID pages, using the exact
  predicate. First success is an explicit recent-50 baseline; later runs freeze
  an upper ID and exhaust newer IDs one page/request, resuming after failures or
  restart. Query changes start a new recent window and keep older matches in
  History. No timestamp-only cursor, moving page offsets or automatic polling.
- Snapshot/match/cursor writes commit together, guarded by follow and checkpoint
  versions. Failed pages do not advance the resume cursor; guarded sanitized
  diagnostics persist across reload. Source/credential changes and stale requests
  discard fetched results. Cancellation remains bounded by the existing client.
- Cache: 5000 posts, 64 KiB metadata/post and 50 displayed match explanations with
  truncation indicated. Explicit seen/unseen decisions are versioned. Removing
  seen snapshots retains decisions, cursors, native files and review history.
- Adds `/following/follows` aliases, feed/sync/seen/prune endpoints, a utility-nav
  Following entry and independent themed management/feed page. Filters/paging
  are URL state; preview grants are re-created from current source policy.
  Mobile shows the feed first. Browse/Download reuses the existing post browser.
- Supported sync is deliberately Danbooru-only in this phase. Other stored booru
  definitions remain visible with checks disabled. Account/profile targets for
  Twitter and similar sites retain their broader planned contract; native
  mappings, intersection selection, polling and edit/deletion resync are deferred.
- Focused provider/API/storage/migration regressions passed, including current
  95/96/97 upgrade fixtures and upgrade from 98. Mounted fixture workflows passed
  1440×1000 and 390×844 (also `/stash/` base prefix), including refresh/back,
  failure/retry, explicit seen state, pruning/rediscovery, query revisions,
  pagination, preview failure, cancellation and repeated leave/return cleanup.
  Layout screenshots were visually inspected. Full `make test` and `make it`
  passed on implementation `8ca9c63d4fdbb6cfc11dd5f6eb479e38d8fd0ce7`
  (local `da3ddef`, identical tree
  `ca6b566973528f542e97afe0bab50cd2edb07500`). Remote lint passed; CI generation
  reached 61 passing UI tests but failed on three intentional hook-dependency
  triggers. Backend integration/build jobs were skipped because generation failed.
- Codex Review identified signed media URLs in snapshots. This is valid: transient
  media allowlisting is not a durable credential policy. Following now omits
  every query-bearing media URL, user information, fragments and ephemeral
  grants (including nested previews), retaining query-free HTTPS locations.
  It does not create broken URLs by stripping signatures. Added signed URL unit
  cases and restart/persisted-payload integration coverage; provider metadata and
  browser links remain available when cached previews are omitted.
- CI correction: failed-preview state is keyed to its thumbnail URL; intentional
  reload epochs retain their dependencies with explicit Biome explanations.
  A dedicated Following mounted-browser workflow now runs both viewport/prefix
  fixtures rather than relying on the separate booru browser suite.
- Follow-up verification SHA:
  `0ac8c161538b79ad38af91796eff6aee8e55a933`, tree
  `4f9cefdfb7f53d6a135de1db5f3ffe741f0647dc`.
  GitHub Actions passed full `make it` (including the signed-payload/restart and
  URL-policy regressions), backend lint, backend/UI generation, `make validate-ui`
  (61 tests, JS/CSS lint, TypeScript, formatting), production `make ui`, the
  dedicated Following desktop/mobile/prefix workflow and the existing booru
  browser workflow. [Build/test run](https://github.com/Dusky-dev/StashBooru/actions/runs/39456302183) ·
  [Following fixture run](https://github.com/Dusky-dev/StashBooru/actions/runs/39456302374).
  Platform builds were still running at this checkpoint. The valid original
  signed-URL thread was resolved after integration success.
- Fresh Codex Review on `0ac8c16` identified two further valid recovery issues.
  Feed projection now prefers the server-stamped snapshot source alias so an
  anonymous built-in alias cannot replace the credentialed alias used for sync.
  It still falls back to the same instance if the original alias is removed or
  rebound, and uses current rotated credentials. Integration coverage exercises
  preview grants, browser links, credential rotation and unavailable/rebound aliases.
  Refreshed definitions now detect a stale editor, preserve its draft and pause
  Save until the user explicitly reloads the current definition. Mounted coverage
  exercises the conflict and successful subsequent edit at both viewports.
  Verification SHA `aaf52554c8e9ef05f65b0b4f543d3ca4d3cf213c`,
  tree `6263167a5efac5db44912b74e97943f1dfa7b3ab`: full `make it`,
  backend lint, backend/UI generation, UI validation and production bundling,
  and both browser workflows passed in GitHub Actions. The new alias regression
  and draft-preserving conflict workflow passed. Codex re-review completed with
  no findings on this SHA; all three earlier threads are resolved.
  [Build/test run](https://github.com/Dusky-dev/StashBooru/actions/runs/39457673229) ·
  [Following fixture run](https://github.com/Dusky-dev/StashBooru/actions/runs/39457673281).
  Platform builds remain in progress at this verification checkpoint; no claim
  of verified cross-platform binaries is made. A subsequent documentation-only
  handoff commit may supersede that CI run without changing implementation files.
- The execution environment became unavailable before the final local UI
  validate/build/lint pipeline could be retrieved; those local results are
  unverified. Follow-up verification above is from GitHub Actions. Changes are
  published through the connector with a branch-head lease, without touching
  develop. Changed text has no trailing whitespace; all 19 relative links in
  this ledger resolve in the published tree.
  Screenshots below are the previously inspected synthetic implementation fixture,
  not evidence of live-provider operation.
  [Desktop](screenshots/following-desktop.png) ·
  [Mobile](screenshots/following-mobile.png).
- Live smoke: one bounded public testbooru request on 2026-10-20 could not reach
  the endpoint before its deadline. No live authentication/provider acceptance is
  claimed; mocked fixture behavior is separate from real provider operation.

## B06a — durable follow definitions and API — 2026-10-20

- Baseline: clean `develop` at
  `7856430423374f0462fb25e82a206ffbae07ee9c`; feature branch
  `feat/booru-follow-foundation-20261020`. Initial open PR inspection found only
  #162 (Rule34 tag DAPI response repair), with no follow/migration overlap.
- Scope: library-wide Artist, Character, Copyright/series and exact native-query
  follow definitions for tag-based booru engines. This is the independent backend
  phase of B06; it neither changes browser UI nor claims a usable Following feed.
- Migration 98 adds `booru_follows`. IDs and engine/URL instance identity are
  immutable. Query targets retain exact case, punctuation and whitespace.
  Versions reject stale concurrent edits; category/target changes increment
  a separate query revision. Rename/unfollow/refollow retain that revision.
- GET/POST `/booru-browser/follows` use existing authentication, JSON bounds,
  same-origin protection and no-store responses. Duplicate instance/category/
  target definitions are rejected across source aliases. Current availability
  resolves an enabled alias without copying credentials or rewriting history.
  Reusing a source ID for another site cannot retarget follows. Unfollow works
  even with broken/disabled source settings and retains independent review marks.
- Bounded to 500 definitions including inactive records. No deletion API,
  native entity/category verification, query execution/decomposition, remote
  requests, jobs, automatic downloads, polling, feed cursors or seen-state claims.
  Pixiv/Pawchive/Kemono creator follows remain deferred to a separate contract.
- Codex Review on published `45f2755ce5797bdff6adc59c1d67c02eb39ad97e`
  found a valid recovery gap: the list endpoint also needs to work with broken
  source settings so a reloaded client can recover IDs/versions for unfollow.
  It now reads stored definitions first and reports `configuration-error`
  availability; malformed/unreadable settings have integration regressions.
- Implementation verification: local
  `a871adddcbfd89c8ae4cdb147458b10ea02cc601` / published
  `ecb8130a30743e6c10e76f03a73be90bc4b38c35`, identical tree
  `d9f81c305e201ee84653771c48dd4759564e2514`.
  [PR #163](https://github.com/Dusky-dev/StashBooru/pull/163).
- Passed `make generate-backend`, focused follow/model/API/schema tests (including
  95/96/97 upgrades), full `make test`, full `make it`, and `make lint` (**0
  issues**). The fresh checkout required `make touch-ui` / backend generation
  before API compilation; no generated files were edited by hand. Also passed
  40 relative documentation links and `git diff --check`. UI source is unchanged;
  no mounted UI/feed or live provider acceptance is claimed by this API phase.
- Codex Review's recovery finding is fixed and resolved. Re-review completed on
  `ecb8130a30743e6c10e76f03a73be90bc4b38c35` with no findings/unresolved
  findings. CI on that implementation passed generation, lint, tests and six
  platform builds at recording time; macOS was still running, with no failures.
  This following documentation-only checkpoint records the verified implementation;
  its new CI run is separate. No merge/deployment.
- Next B06 phase: bounded on-demand feed synchronization, durable snapshots and
  revision-specific checkpoint/seen semantics, followed by management/feed UI.
  Native Artist × Copyright selection remains B03 and must preserve constraints.
- Owner scope clarification after the B06a checkpoint: Following extends to
  Twitter and similar creator/account sites. Updated the guide/roadmap to require
  a shared feed contract with provider adapters, stable account IDs, a distinct
  future account target kind, optional tag metadata and preserving migrations.
  Those site adapters remain later work; no runtime/API capability was added by
  this documentation-only clarification. Reviewed docs and `git diff --check`;
  backend verification above remains the implementation checkpoint.

## Paheal original CDN and inspector quality — 2026-10-20

- Baseline: `develop` at `aa1a2b42bf9bfb6b07608350bfbda688e54e91b9`;
  feature branch `fix/paheal-originals-and-samples-20261020`.
- Owner reports Paheal previews available only at thumbnail quality and
  original downloads unavailable. Shimmie's upstream Danbooru API source
  publishes `file_url` and `preview_url` but no sample URL by default.
  A reproduced Grabber issue documents Paheal originals moving from
  `rule34.paheal.net/_images/` to `r34i.paheal-cdn.net/_images/`.
- Codex review #170 identified three valid issues, addressed on the same
  branch: the special CDN host must match **exactly** (including for old
  persisted overrides), old `/_images/` paths need the sharded
  `/<2hex>/<2hex>/<32hex>` CDN layout, and promoted originals must
  not become public direct-preview fallbacks if the server proxy rejects them.
- Normalize legacy website/relative original links only when a full MD5
  is verified in post metadata or an exact 32-hex filename; preserve
  unresolvable originals instead of inventing paths. Keep original format
  from the legacy URL or filename when CDN paths have no extension.
  Do not rewrite thumbnails or other providers; retain media/redirect bounds.
- For Paheal posts lacking explicit sample data, use the original as the
  inspector's sample within the current 24 MiB preview limit. Keep
  ordinary grid thumbnails and honor explicit samples when present.
- Added adapter + mocked binary download regressions, including exact
  host rejection, sharded-path construction, image format retention, and
  server-only high-resolution preview fallback. CI verifies code paths;
  live Paheal CDN downloads still require owner-side acceptance.
  No merge or deployment performed.

## Rule34 live category priority and quiet fallback — 2026-10-20

- Baseline: `develop` at `ad91eb1843dbe38686848bd74e538cd90b35dc50`,
  branch `fix/rule34-live-category-priority-and-hide-warning-20261020`.
- Owner observed the "Automatic Rule34 tag categories could not be
  installed" warning every search and requested live classification even
  when a manually installed Rule34 CSV succeeds.
- Correct a real precedence bug: `categorizeBooruPosts` formerly applied
  installed CSV before `CategorizeRule34Tags`. Live categorization only
  examines `unknown` tags, so the CSV silently suppressed all lookups
  for tags it covered. The handler now calls live categorization first,
  fills unknown tags from the installed CSV second, and invokes the
  historical auto-catalog only for remaining unknown tags.
- Remove the repeated automatic catalog failure alert from the browser.
  The optional installation continues as before, but a blocked mirror
  no longer interrupts the UI. A first-install loading notice remains.
- Codex review caught the 100-name live lookup budget starvation case:
  if the CSV covers more than 100 early tags, a later CSV-missing tag
  could be excluded before lookup. The API now supplies an exact-name
  CSV-missing priority predicate to the Rule34 client; it reserves live
  request slots for CSV-missing tags first, then tries CSV-covered tags.
  Calls without CSV priorities retain the original behavior.
- Add API and booru-layer regression tests showing a live category
  beats a conflicting CSV category, CSV backfills unavailable live
  entries, CSV-missing tags stay ahead of 100+ covered tags, pretyped
  provider categories remain stable, live calls are attempted on
  repeat searches, and unrelated providers do not make Rule34
  lookups. No schema changes.
- Verify through CI; live Rule34 API traffic requires user-side testing
  after merge. Do not merge or deploy without owner approval.

## Rule34 CSV malformed-name tolerance — 2026-10-20

- Baseline: `develop` at `ec26085c629ed3b8c9e4f712effa118c6fee45da`;
  branch `fix/rule34-skip-malformed-tag-csv-rows-20261020`.
- Owner reports manually uploading a category database fails on CSV row
  78,268 with "invalid tag name ... expected 1–256 UTF-8 bytes without
  control characters". Actual file/row contents were not provided; the
  message only proves the validation failed.
- For Rule34/Gelbooru CSV exports, skip at most 2,048 rows containing
  invalid names while preserving all other valid categories. For headerless
  positional CSV, at least one prior record must validate successfully
  before skipping. This does not change other booru engines, strict
  CSV structure/category/count parsing, or the
  atomic write contract. Reject catalogs with too many invalid names or
  no valid tags. Skip counts are persisted in the parsed index and exposed
  by both upload and settings endpoints.
- Show skipped-row totals on import success and in tag settings. Update
  the outdated one-million-tag label for Gelbooru's existing 2M limit.
- Unit tests cover empty, control-character, and overlong names; strict
  rejection of malformed files; a malformed record at row 78,268;
  bounds; category results after bad rows; and API reload after upload.
- A real owner CSV and live StashBooru Rule34 browsing are still required
  to confirm the exact file works. The PR is not merged/deployed.

## Rule34 automatic category catalog fallback — 2026-10-20

- Base: `develop` at `469ce563d6dc23770120f24ef1422d9a3419575a`;
  branch `fix/rule34-auto-categories-fallback-20261020`.
- Reported issue: previous live Rule34 category fixes (#168/#169)
  still leave tags under "category not supplied". The public website
  may challenge HTML sidebar requests; `s=tag&q=index&name=...` is
  undocumented by Rule34's published API and can return no exact matches.
- Fallback: only when unresolved categories remain, start one bounded
  installation of the public Rule34-specific `deepghs/site_tags` CSV
  snapshot (46.2 MB, last Rule34 update 2025-08-02). Cache it in
  `booru-tags/rule34-auto.csv`, never overwriting a custom CSV.
  Downloads are limited to 64 MiB with fixed HTTPS source, restricted
  redirects, timeouts, atomic publish and full CSV validation. Failed
  installs retry after one hour. The current search does not wait for
  46 MB; a repeat search uses loaded categories.
- Priority: installed source CSV, then live confirmed classifications,
  then historical Rule34 catalog for tags still unknown. No category
  is guessed from spelling, and other sites are unaffected.
- Public search responses include a small `categoryStatus` indicator
  (`loading`, `failed`, or `ready`) so the browser can explain the
  first install or network failure.
- Added simulated download, safety, corrupted-file, persisted-catalog
  and category-priority tests. Actual server access to Hugging Face and
  real Rule34 account categorization remain acceptance tests; the
  snapshot predates new tags. No merge or deployment authorized.

## Rule34 public sidebar wiki-link classification — 2026-10-20

- Baseline: merged `develop` at `fc2651bc4a39022adca438f7524d1ab6008bcdf9`
  (including PR #168); feature branch
  `fix/rule34-live-sidebar-wiki-tags-20261020`.
- Site evidence: the public post page displays five typed tag groups.
  Third-party Rule34 sidebar integration reads canonical wiki `search=` links
  as well as post-list `tags=` links. The previous StashBooru parser only
  accepted post-list links, potentially discarding usable live typed tags.
- Accept canonical wiki links only inside typed `#tag-sidebar` rows, with
  exact matching against the requested post tags and no off-host links.
  Fetch a bounded public HTML prefix to retain early sidebars when the
  full post page exceeds 512 KiB due to comments. JSON API bounds, host
  throttles, redirects and credential isolation are unchanged.
- Added provider-shaped parser and search-path fixtures for wiki-only
  links, oversized HTML pages and preservation of JSON size checks.
  These fixtures are not captured live provider responses.
- Network in the development runtime could not query `api.rule34.xxx`
  or retrieve raw public HTML; no live account/end-to-end verification.
  Rule34's official tag help only documents `id` and `limit`, so the
  alternative exact `name` endpoint is not a verified guarantee.
  Follow-up acceptance needs anonymized provider status/category counts
  from the user's running server. No merge or deployment.

## Rule34 live exact-name tag classification — 2026-10-20

- Baseline: `develop` at `cd6ad046630991d30d30063a7ed58748135d8b14`;
  branch `fix/rule34-live-exact-tag-types-20261020`.
- Owner requested live category classification rather than requiring a CSV.
  Rule34's `autocomplete.php` returns tags without category metadata, while
  a third-party integration documents a singular `s=tag&name=<tag>` DAPI
  lookup returning a type; earlier StashBooru attempts relied on the
  undocumented Gelbooru-style `names` bulk parameter.
- Keep typed public sidebar first for efficiency, then resolve remaining
  unknown tags by exact, authenticated `name` requests (max 6 per search,
  eight-second total lookup budget). Handle both `<tags>` and `<tag>`
  XML responses, preserve exact name allowlists and 24-hour positive cache.
  Provider failures never break the search or override installed CSV types.
- Regression tests cover request shape, exact tag identity, XML variants,
  rate-budget cap, sidebar-first precedence, 403 fallback and caching.
- No live Rule34 account verification available; GitHub CI is authoritative.
  Partial classification can occur due to the hard request/time limits.
  No merge or deployment performed.

## Booru AI filter crossed-out icon — 2026-10-20

- Baseline: `develop` at `b14eee5f20df7e724e765c8d04dc6449e4f8db7f`;
  branch `fix/booru-ai-filter-crossed-icon-20261020`.
- AI-generated filter stays a two-state robot control, but the **hide AI**
  state draws a slash over the robot rather than adding an active/highlighted
  button appearance. The unfiltered state displays the normal robot.
- Existing `aria-pressed`, URL persistence (`ai=hide`), loaded-page
  filtering and lack of provider refetch are unchanged.
- Added Playwright assertions for the slash icon and absence of selected
  styling across activation, refresh, and deactivation.
- Verification: GitHub CI on the PR is authoritative; no merge/deployment.

## Rule34 offline category import compatibility — 2026-10-20

- Baseline: `develop` at `6a5127d4928ab80b7c68c65eebb1b22e253680ca`;
  branch `fix/rule34-csv-import-categories-20261020`.
- Rule34 category lookups still report all tags unknown in the user's setup
  despite merged PRs #161/#162/#166. Provider responses have not been
  inspected from the user's Stash server, so no claim of a verified live fix.
- Confirmed community Rule34 tag CSV data has header
  `type,count,name,ambiguous,id`. Our CSV importer previously only detected
  a header if `name`, `id` or `tag` was the **first** column; the Rule34
  export would be rejected, even though typed labels are available offline.
- Accept name/type CSV headers in any column order. Preserve Gelbooru-family
  deprecated type 2 as `unknown`; do not interpret it as copyright. Do not
  change Danbooru or e621 category mappings.
- Increased the optional Gelbooru-family CSV ceiling to two million unique
  tags and three million total names/aliases, while retaining the 64 MiB
  input-size boundary and existing per-tag alias limit.
- Added input regression tests and installation guidance for the 2025 community
  Rule34 snapshot. This makes per-source **manual** CSV installation usable
  without relying on provider tag metadata, but does not silently download or
  install an external database.
- CI and any provider/live validation must be verified on the PR. Not merged.

## Rule34 typed post-sidebar fallback — 2026-10-20

- Baseline: `develop` at `dbd65145e94d5316d9933dfa7f5058fa963d09fa`;
  branch `fix/rule34-post-sidebar-categories-20261020`.
- User still reports "Tags — category not supplied" after PRs #161 and #162.
  Rule34 officially documents tag lookup by ID and limit but not our existing
  bulk `names` query, so a response can be valid but unrelated to requested
  tags. Previous code spent its entire eight-second window retrying.
- Stop DAPI category batches after an empty/mismatched batch. For remaining
  unknown tags, fetch at most three Rule34 public post pages (canonical
  website host and validated numeric post ID) and read exact tags/categories
  from `#tag-sidebar li.tag-type-*`. Limit response bytes and CPU tokens,
  never send API credentials to website pages, and only classify tags
  already present in a result. Browser search is not blocked on site errors
  or challenges. Positive results use the existing 24h per-site cache.
- Bounded fallback may not cover tags appearing only in additional posts or
  when Cloudflare refuses the page. Installed Rule34 CSV remains the complete
  offline classification path.
- Added parser/provider tests for artist/character/copyright/general/meta
  classes, invalid links/unrelated text, blocked pages, exact matching,
  cache reuse, and respecting the API batch limits. GitHub CI is authoritative.
  No merge/deployment performed.

## Rule34 tag DAPI XML format repair — 2026-10-20

- Baseline: `develop` at `7856430423374f0462fb25e82a206ffbae07ee9c`.
  Branch: `fix/rule34-tag-dapi-xml-20261020`; owner has merged PR #161.
- Follow-up to user report that Rule34 tags still appeared under
  "Tags — category not supplied." Rule34's documented tag DAPI provides XML
  responses; the preceding implementation requested `json=1` and only parsed
  JSON, silently discarding the XML and retaining `unknown` tag types.
- Request standard tag-DAPI XML (no `json=1`), parse bounded XML `tags/tag`
  attributes and exact allowlisted tag names, retaining JSON compatibility.
  Reject HTML/malformed XML/DTD. Honor the established Gelbooru tag
  category mapping: type 3 represents copyright; deprecated type 2 remains
  uncategorized (Codex review correction on PR #162), including in cached
  results. Existing optional CSV priority, per-instance cache, timeouts,
  batching and search-failure fallback remain unchanged.
- Regression tests now exercise realistic XML, actual HTTP Accept/query shape,
  exact matching, unsupported structures, and backward-compatible JSON.
- The provider's authenticated tag API cannot be fully verified without
  live account credentials. Code validation is tracked on GitHub Actions;
  this is not a deployment and does not authorize merge.

## Download queue reclamation — 2026-10-20

- Baseline: `develop` at `dff85613459f83adfb926d8e5013a751196a6f7a`.
  Branch: `fix/booru-download-queue-completed-reclaim-20261020`.
- Fixes the reported 429 "Download queue is full. Completed entries expire after
  one hour" caused by keeping up to 100 already-finished jobs in the
  in-memory Booru download status registry.
- When the registry reaches its 100-slot cap, evict the oldest terminal
  (complete, failed or cancelled) status before admitting a new download;
  preserve active queued/downloading/importing jobs, their job cancellation and
  original file/import receipts. Full active queues still return 429 with a
  specific explanation. Terminal statuses remain visible until memory pressure
  or the previous one-hour expiry.
- Tracks terminal timestamps under the existing registry lock. Adds regression
  tests for oldest-finished eviction, cancelled jobs, active backpressure,
  a hundred completed downloads, and retention below capacity.
- Validation: GitHub CI on the PR is authoritative; no production deployment
  or merge performed in this change.

## Rule34.xxx automatic tag hierarchy — 2026-10-20

- Baseline: `develop` at `b98d93aaf6cba560806ceb22daa5c7d19c7f7fbc`.
  Branch: `fix/rule34-tag-categories-20261020`. No merge or deployment.
- Cause: Rule34 post DAPI returns a flat `tags` string despite the site having
  artist/copyright/character/meta/general categories. Previously the browser
  required a manually installed CSV to group these tags.
- The official authenticated Rule34 tag DAPI is now queried for **exact tag
  names only** after local CSV categorization, in at most five 100-name
  requests per search page. Unknown, unrequested or invalid types are ignored.
  The client caches only verified categories for 24 hours per site instance
  with a bounded 8192-item cache. Lookup failure does not break post searching.
- Added unit tests for type mappings, provider/auth request shape, strict
  exact-name matching, cache reuse, API failure, and request bounds.
  Search has not been tested live against a Rule34 account's credentials.
- Validation: GitHub CI for the PR; this update does not authorize merge
  or deployment.

## Paheal XML API endpoint repair — 2026-10-19

- Baseline: current `develop` after #155, feature branch
  `fix/paheal-shimmie-find-posts-20261019`; no merge or deployment.
- Replaced the obsolete Shimmie/Paheal Danbooru XML search URL
  `/api/danbooru/find_posts/index.xml` with the supported endpoint
  `/api/danbooru/find_posts`, preserving search tags, 1-based paging,
  1–50 result limit, server-side throttling and anonymous access.
- The current Shimmie upstream implementation and maintained booru library
  document the unsuffixed route; the old route may return HTTP 200 HTML,
  causing StashBooru's correct XML validation to reject it.
- Mock-transport regressions verify Paheal's exact API route and parameters,
  realistic posts XML with count/media/tags, and rejection of HTML, negative
  counts and provider records beyond the requested limit. Live Paheal
  acceptance remains unverified; browser/build CI passed on the initial PR head,
  while gofmt was corrected after lint reported its sole source finding.

## Multi-file post inspector stream — 2026-10-19

- Baseline: latest `develop`. Branch
  `feat/booru-inspector-infinite-media-20261019`; no merge or deployment.
- Replaces `Media file` `btn-secondary form-control` select with a vertical
  gallery inside a scrollable post inspector. Shows each media's own preview,
  metadata, download and hash without selecting a dropdown option.
- Progressive IntersectionObserver loading mounts three assets per batch
  near the scroll boundary. Animated/video playback stays on demand;
  post navigation resets scroll-gallery state and download shortcut target.
  The stream is bounded by provider-returned supported attachments.
- Desktop/mobile browser regressions test first/secondary file download,
  per-file metadata, next/previous post navigation and a 12-file gallery
  reaching its end without rendering all 12 immediately. GitHub CI verification
  will be recorded on the new PR.

## Booru AI-generated tag filter — 2026-10-19

- Baseline: current `develop` following Rule34 API updates, feature branch
  `feat/booru-ai-generated-toggle-20261019`; no merge or deployment.
- Added a pressed/unpressed **Hide AI-generated posts on this page** icon in
  `.booru-filter-icons`. It recognizes explicit `ai_generated`, `ai-generated`
  and spacing/case variants without inferring origin from an image. Assisted
  artwork and AI-generated backgrounds remain distinct.
- Saved URL and filters preserve `ai=hide`; tag aggregation, previews, selection
  and media filters use the visible loaded-page posts. Native provider searches,
  pagination and totals are unchanged.
- Mounted browser regression fixture checks toggling, no refetch, combination
  with media filters and persistence after reload. Verification is pending CI;
  provider-tag accuracy and untagged AI are not detectable by this filter.

## Rule34 authenticated API repair — 2026-10-19

- Baseline: `develop` after merged #153. Feature branch
  `fix/rule34-auth-api-host-20261019`; no merge or deployment.
- Exact Rule34.xxx hosts (not arbitrary Gelbooru/DAPI clones) now route
  post searches and live autocomplete to `api.rule34.xxx` with the configured
  account API key and user ID. Missing credentials fail locally with useful
  Settings instructions. Human-facing post URLs and configured source identity
  stay on `rule34.xxx`; server-side secrets remain redacted.
- Rule34 401/403 errors explain account requirements and possible Cloudflare
  enforcement without bypassing access controls. Source Settings and the
  browser guide point to the official account Options page.
- Regression coverage uses mocked HTTP requests for legacy/default/API hosts,
  anonymous request rejection, authenticated paging, canonical post URLs,
  autocomplete, third-party DAPI isolation, and 403 credential redaction.
  Live authenticated Rule34 access is not validated without user credentials.
  GitHub CI verification is pending on the PR branch.

## Browser controls and account favorites — 2026-10-19

- Baseline: merged #152 at `e45586347603f2c58ae40d68787cc71a0173a48f`.
  Branch `feat/booru-controls-favorites-20261019`; no merge or deployment.
- Source dropdown gets persistent up/down ordering. Redundant source badges,
  pagination index and always-visible review toolbar are removed. Native sort menu
  restores Recent/Oldest/Popular/directional score/Updated for supported engines.
- Tags submit immediately by default; a browser-local Settings switch retains the
  draft-only workflow. Preview tag actions own navigation to avoid stale close
  callbacks reverting the new query. Dropdown active highlights track open state;
  keyboard focus is retained. Blank submissions are explicit `q=` URLs.
- Card selection captures the whole card while a selection is active, including
  action buttons; Clear selection restores normal preview/download/tag actions.
  Rating moves to the opposite side of the caption. Missing provider file_ext is
  filled from the original URL extension for loaded-page media filtering.
- Account favorites replace local card favorites in the UI. Danbooru native
  ordered favorites, e621 favorites and Gelbooru index/detail APIs use server-owned
  credentials and bounded existing transport. Account/source changes reject stale
  responses. Legacy local favorites remain stored. No remote favorite writes.
- Endpoint evidence: maintained gallery-dl Gelbooru extractor, Danbooru/e621
  controllers and PixivPy ranking client. Authenticated provider acceptance remains
  separate from synthetic tests.
- Runtime checkpoint local `cf5bee2a0` / published `119472d705c8da6f74e8490c009de1c1bee0babf`,
  identical tree `51f5e715cca2b72faa5ed3acc1f28585b6252296`.
  Review: [PR #153](https://github.com/Dusky-dev/StashBooru/pull/153).
  Provider regressions, all 61 UI tests, JS/CSS lint, TypeScript/formatting and
  production bundling passed. Mounted 1440×1000 and 390×844 passed, and synthetic
  desktop/mobile screenshots were inspected. All 40 relative doc links passed.
- Follow-up handles explicit zero-count deleted favorite details without losing
  provider pagination, and rejects Recent for blank Pixiv daily rankings. Provider
  regressions passed; types/lint/production were repeated. CI exposed a mobile
  fixture race issuing Up before the next post's download settings were ready;
  the test now waits for the enabled Download/Retry action after remembered job
  recovery before exercising the shortcut. Pawchive retains recent-post startup
  on source changes instead of inheriting unsupported post-text queries.
- Final local runtime `db3d80da8ff716e3b1edaddc8a0558959fdf7f42`, tree
  `391c98b2e8f456beb2d93bea641ab8fe80e65f7a`, passed provider regressions, full
  `make test`, `make it` and `make lint` (**0 issues**). All 61 UI tests, JS/CSS
  lint, TypeScript/formatting and production bundling passed; final types/scoped
  lint/production and mounted 1440×1000 + 390×844 passed after the provider guards.
  Final mounted coverage includes persistent source ordering, both tag-click
  modes, preview add/replace redirects, blank submission, media filtering without
  a provider refetch, whole-card selection/action suppression, caption ratings,
  account favorites refresh and existing download/player/review/race workflows.
- Codex Review completed with no findings on published `119472d` and `0773f35`.
  GitHub generation/browser passed; the second lint runner hit an external schema
  fetch timeout before code analysis (first runner and local full lint passed).
  Fresh final-head CI and Codex Review outcomes are attached to PR #153. This
  checkpoint changes documentation only. Live authenticated provider acceptance
  remains unverified; no merge or deployment is performed.

- Codex Review on `0d1d662` identified two valid P2 regressions. Gelbooru's full
  favorite-page signal now takes precedence over advisory totals, matching the
  maintained client's forward pagination. A regression covers under-reported
  totals and the page-750 cap. Wall view now shows the number/rating caption above
  its action rows; mounted checks assert ratings are visible and screenshots were
  inspected. Both changes preserve the requested caption placement.
- Saved filters retain explicit blank submissions instead of converting them to
  `*` (important for Pixiv rankings). Favorites filters omit an unsubmitted draft;
  API/legacy normalization and mounted save/apply/delete/reload regressions pass.
- Verified correction runtime `42512e336a2578becacf937b8822c2f12d46924d`, tree
  `9c0c099e565b658c166bd823cd36729f0f411503`: full `make test`, `make it`,
  `make lint` (**0 issues**), all 61 UI tests, JS/CSS lint, TypeScript/formatting,
  production bundling and mounted 1440×1000 / 390×844 passed again. This ledger
  checkpoint is documentation only; current-head CI and re-review are attached to
  PR #153. Authenticated real-provider acceptance remains separate.


## Single-site browsing and preview refinement — 2026-10-19

- Fresh merged-#151 baseline `9c059fe95d240e234337cd14b5b46b6177861b65`.
  Work is isolated from the preserved unfinished checkout. The owner's policy to
  independently assess Codex Review and implement sound findings is recorded in
  `AGENTS.md`; no merge or deployment is authorized by publication.
- One source with a favicon/initial fallback replaces the multi-select switches.
  Legacy URLs/filters use their first source; source changes reset incompatible
  rating/order and archive creator expressions. The selected URL source also
  governs startup, including old saved browser state.
- Pawchive/Kemono artist lookup searches their public directories by name and
  service and opens a selected creator's posts. Directories use the existing
  protected transport, cancellation/rate gates and source-identity rechecks;
  bounded payload/row/cache limits and concurrent-load coalescing apply. Pawchive
  creator posts use its native endpoint; unsupported post text queries still reject.
- Enter submits typed text; Down+Enter accepts a suggestion. Exact/submitted
  terms and late responses do not reopen completion. Editing can complete the
  same text again. Settings uses `/booru/settings` with refresh and Back support.
- Grid zoom changes preview height and column width; Wall keeps native row zoom.
  Sensitive has a distinct engine-aware filter/color, including current Danbooru
  and Gelbooru versus legacy three-rating sources. Core taxonomy remains icon
  actions with a chooser for multiple tags; exact installed-CSV categories can
  fill unknown provider categories without treating aliases/prefixes as evidence.
- Preview content receives focus on entry and preserves consecutive arrows across
  posts. Player/input/dropdown keys stay local to their controls; a new regression
  caught and fixed Down navigating a tag menu also issuing an ignore write.
  Category-colored tags offer add/replace query actions. Native resolution/size/
  uppercase format replace verbose metadata; MD5/evidence share the bottom row.
  Download controls retain the default-converter switch without settings prose.
- Runtime checkpoint: local `f1c84a28aeb5befceb2d457443bdf17573961117`.
  Full `make test` / `make it`, backend/UI generation, 61 UI tests, JS/CSS lint,
  TypeScript/formatting and production bundling passed. Lint/type/production were
  repeated after the final preview keyboard guard. Mounted 1440×1000 and 390×844
  passed the new source/artist/settings/completion/height/metadata/menu-key flows
  and the existing jobs/download/converter/review/player/race suite. Synthetic
  preview/desktop/mobile screenshots were inspected; live credentials/network
  acceptance remains separate. Go lint requested a test-only switch rewrite;
  final package tests/lint and publication details are recorded below.
- Test-style checkpoint `336b311b5da4e5708a9263253094e779bc10dee3` passed final
  provider tests and full `make lint` with **0 issues**. Published runtime
  `0092c2172b289b61cb04b6a5b72c9f85d2a8e07c` has the identical verified tree
  `6431a5c4a9f4a1141d38b11b7dbcd05f0d2823ff`. Fresh develop is still the baseline
  above. This final checkpoint updates the ledger only. Current head CI and
  independently assessed Codex Review outcomes are attached to PR #152.
- Codex Review identified Kemono's retired `/creators.txt` path. Verified against
  the maintained [gallery-dl Kemono client](https://github.com/mikf/gallery-dl/blob/master/gallery_dl/extractor/kemono.py):
  its public index uses `/api/v1/creators` with `Accept: text/css`. The adapter now
  uses that endpoint; provider regressions assert the live path and retain the
  engine-specific Accept header. Other findings are assessed against current
  behavior rather than accepted automatically.

## PR #151 review corrections — 2026-10-19

- Reviewed Codex findings on `85c8e77c3de39df5bf626cfb3d656a9d82398405`;
  fresh develop remains `d29d8767db06f1b14abb2928a7edf1c911100779`.
  Continue the existing PR/branch; no merge or deployment.
- Post navigation resets the selected gallery attachment to the primary file.
  Selection is scoped to source-instance/post identity before actions render;
  the modal retains focus for consecutive arrow keys. Regression coverage checks
  secondary-file download, next-post primary download, previous-post reset and
  the existing uninterrupted keyboard/animated-preview workflow.
- Blank saved searches outside Favorites persist as `q=*`. Previously accepted
  blank filters are normalized when loaded. Duplicate query parameters, control
  lines and query/URL bounds still reject; Favorites retains its own mode.
- Pawchive text queries now return an explicit unsupported-query input error
  before any network request. Recent-post and creator browsing remain supported;
  Kemono retains text search. The [Pawchive frontend notes](https://github.com/dragonjay-lyj/pawchive#notes)
  document that its upstream posts endpoint ignores `q`; bounded local search is separate.
  Query help and the browser guide describe the supported behavior.
- Runtime checkpoint: local `0daf10e8d4dc5334215a3b4891ef9116694aa1b1`.
  Focused provider/API regressions passed. All 60 UI tests, full JS/CSS lint,
  TypeScript and formatting passed before the modal-focus refinement; focused
  lint and TypeScript passed again after that refinement.
- Full `make test` and `make it` passed, including the final API regressions.
  Production bundling and mounted 1440×1000 / 390×844 passed on the final runtime;
  both gallery-reset/download identity and consecutive arrow-key navigation passed.
  All 24 relative links in the edited guides/ledger and `git diff --check` passed.
  Full `make lint` passed with no issues. GitHub checks rerun on publication;
  their current status is attached to PR #151.
- On reviewed head `85c8e77`, GitHub generation, native tests, lint, browser and
  six platform builds passed. FreeBSD alone failed while downloading
  `github.com/klauspost/compress` with a TLS handshake timeout, before compilation.
  No workflow change is needed for that transient dependency-fetch failure;
  publication reruns the checks on the corrected head.

## Providers, filenames and list toolbar — 2026-10-18 UTC

- Fresh develop `d29d8767db06f1b14abb2928a7edf1c911100779`, no open PRs and
  existing task branch confirmed when resuming. Paheal/Shimmie, Pixiv App API,
  Pawchive and Kemono adapters added with explicit authentication/query limits.
- Fixed upstream pagination is sliced without dropping records. Galleries retain
  separate file identities and an inspector selector; unsupported archive files
  and unavailable originals are labelled. Existing native Jobs, scanner and saved
  converter defaults handle each explicit attachment download.
- Common Grabber filename tokens support safe literal subfolders, collision
  suffixes and asset/template-bound receipt reuse. Empty settings preserve legacy
  names. No arbitrary scripting or overwrites are enabled.
- Search/source selector move into native list toolbar below selected-source
  badges. Native page size, pagination index, Grid/Wall and zoom replace the
  redundant results header/sort selector. Core taxonomy icons fill Search or open
  a chooser. Display switches move into list operations. Folder picker uses a
  header Root icon and automatic typed-path browsing.
- Shared server Favorites preserve bounded post/gallery snapshots separately from
  named search filters. Writes are atomic; disabled/changed sources are rechecked.
- Local full `make test` and `make it` passed for the implementation. All 60 UI
  tests, JS/CSS lint, TypeScript/formatting and final production bundling passed.
  Mounted 1440×1000 and 390×844 passed on local runtime
  `18eccd2f6a09caa7e24bf05854dd30a512c9a103`: Favorites add/remove/reload,
  saved filters apply/delete, single/multiple core tags, native toolbar/page size,
  Grid/Wall zoom persistence, decoded thumbnail geometry and top-corner selection,
  selected attachment download identity, Root folder navigation, filename setting
  persistence, decoded WebM/GIF, keyboard controls and prior race/cancellation flows.
- Published runtime `0d873405c53e8e2efeb2b9986d07c7d8cb154f33` matched that tree
  (`a38c61926d79be54128f703d9f2a0b7589ab05ff`); GitHub generation/full UI/production
  and browser checks passed. CI found one unchecked JSON marshal result in Shimmie.
  The explicit error check is local `f389b5bea94d7854aae6bdd0780b9fe9caa4852a`,
  published `058d671006601f93fecdef8cebb0d95b1f5b7bb8`, matching tree
  `3a8bbd5ca0663d88b8e4a9d9d65f8c2fd3958f66`. Provider tests and package lint
  pass after that change. GitHub full Go lint and mounted desktop/mobile checks
  passed on `058d671006601f93fecdef8cebb0d95b1f5b7bb8`; UI is unchanged by the
  correction. Full integration/platform jobs and generation results are linked
  from PR #151. Final local production bundling used a separate output directory
  to avoid changing embedded assets during Go checks. Synthetic desktop/mobile
  screenshots were reviewed separately from real-provider/account acceptance.
  All 40 relative links in the updated README/guides/ledger and `git diff --check`
  passed. This documentation checkpoint changes no runtime code.
- Limits: provider network/account acceptance remains unverified. Pixiv requires
  a user-provided access token (no refresh/login flow); Ugoira ZIPs, archive
  documents, remote account favorites/follows and bulk gallery downloads are not
  implemented. New engines can use CSV completion. No merge/deploy is performed.

## Native browser controls and animated inspection — 2026-10-18 UTC

- Fresh develop `ce2ae8df1f4a900f5a76ba7185f1b247cdb786a2`, merged #149,
  no open PRs and a clean tree checked before starting this owner-requested slice.
- Dark `text-input` controls, native custom switches, card-check selection,
  selected-items counters and the shared bounded PageSizeSelector replace the
  standalone browser controls. Bulk actions appear after selection. Source/API,
  download/converter and optional CSV tools share one Settings entry point.
- Sort, rating, media and autocomplete use labelled icon menus. Thumbnail/ignored
  switches stay together. Cards remove tags/dimensions/verbose status; download,
  folder, ignore/manual mark and native links use named icon actions. Inspection
  retains full evidence, tags, errors and job progress.
- Left/Right navigate the loaded source page, respecting hidden ignored posts;
  Up downloads using saved defaults, Down marks ignored through existing versioned
  review handlers. Native player/input controls and nested dialogs keep their keys.
  Repeated keydown events cannot repeat download/ignore writes.
- Explicit MP4/WebM/GIF/APNG playback uses a 24 MiB bounded, authenticated snapshot
  route with existing source/CDN/media validation and byte-range seeking. Browsing
  does not preload animated originals. Stop/navigation/close release playback.
  Distinct media/job keys fix a stale-player issue caught by the mounted test.
- Local checks: 59 UI tests, JS/CSS lint, TypeScript/formatting and production
  bundling passed. Mounted 1440×1000 and 390×844 passed real decoded synthetic WebM,
  animated GIF, player cleanup, keyboard actions, page size persistence, native
  switches, compact selection and existing download/review/settings/race flows.
  Synthetic desktop/mobile screenshots were reviewed. The final fixture uses
  distinct per-post media keys and checks actual playback/download/ignore targets.
- Full local `make test` and `make it` passed, including motion bounds, format
  rejection and HTTP byte-range tests. Backend generation, all 59 UI tests,
  JS/CSS lint, TypeScript, formatting and final `make ui-only` passed locally.
  GitHub generation/UI/production, Go lint and desktop/mobile checks passed on
  published runtime `ffe9b0cadea576f2bbb05acb5ef7c663f9f6849d`, matching local
  `5a9f52734f240571aa43702b9b1a4287b34c622a` at tree
  `d1179d95c5c2718bf13a3288215bd96e079177b7`. Downstream CI integration/platform
  jobs are linked from PR #150. All 92 relative links in the updated guides/indexes
  and `git diff --check` passed. The final checkpoint strengthens fixture identity
  assertions and updates documentation; runtime source is unchanged.
- Limits: navigation stays within the current loaded source page; animated
  previews need a supported browser codec and an available original up to 24 MiB.
  Deployed provider credentials/network and real provider media remain separate
  acceptance. Metadata import, follows, pHash grouping and batch automation remain
  separate packages. No merge or deployment is performed by this change.

## Integrated live browser workflow — 2026-10-18 UTC

- Owner approved adapting Codename-11/Stash-Downloader's UI/live completion into
  the existing browser. Reference commit `842621160cddf9b7b12c7c2b51ef489a98a191b4`;
  MIT copyright/license retained in `docs/third-party/stash-browser-MIT.txt`.
- Started on the published #148 head while it was pending; fresh develop now
  merges #148 at `471f304f3456f01b47badbfb9ab578f05e51335c`. Fast-forwarded the
  feature branch to it; its baseline file tree is identical to the reviewed head.
- Native sidebar includes removable chips, editable native queries, sort/rating
  controls, loaded-page media filtering, thumbnail visibility and a live/CSV-only
  autocomplete selector. Mobile filters collapse independently from search.
- Live completion supports Danbooru, Gelbooru/Rule34, Moebooru, e621 and Philomena
  endpoints. Shared host/global throttles, redacted credentials, bounded payloads,
  cancellation and an isolated 512-entry/five-minute cache retain native transport
  rules. Failed live requests can use the existing installed CSV without modifying it.
- Card Download/Folder actions use existing native Jobs, default/custom library
  destinations and saved converter defaults. Card/inspector job state and saved
  preferences synchronize; an obsolete settings response cannot restore old defaults.
  Editors wait for saved settings before accepting changes, including server CSV
  paths. The source editor no longer mounts an editable form that its initial
  configuration load immediately replaces.
- Default source/query/paging/preview/review behavior remains integrated. No new
  entity import, inference, bulk download automation, model or encoder is added.
- Verified local runtime `c6a5dbf5990538d9d9802381b93033bc7ed3d792` and published
  `0f8ae749b5d1f914af074c477caea0ac18287849` have identical tree
  `46852c82fb1c54b526e880691f5d739a77369da2`. Full `make test` and `make it`
  passed locally. All 57 UI tests, JS/CSS lint, TypeScript, formatting and
  production `make ui-only` passed on the final UI source.
- Mounted Chrome Headless Shell passed at 1440×1000 and 390×844, including live
  suggestions before CSV installation, CSV-only completion, native query chips,
  sort/rating URL state, loaded-page filtering without a search request, thumbnail
  persistence, card downloads and shared jobs, delayed editor loading and a late
  obsolete preferences response. The actual outgoing download payload retains
  the newly saved folder and conversion flag. Existing search, settings, review,
  import links, folder, preview fallback, cancellation and cleanup checks passed.
  Synthetic screenshots were visually reviewed at both viewport sizes.
- GitHub full integration tests, generation/UI, Go lint and seven platform builds
  passed on backend-identical `e92f7d1fb645ef4b5cac0246f6fb641dc5c3c2c7`.
  Its browser check exposed the settings race fixed and verified above. The local
  concurrent lint retry exhausted compiler memory; isolated GitHub lint passed.
  Final check runs and synthetic screenshot artifacts are attached to PR #149.
  All 91 relative links in the updated guides/indexes and `git diff --check`
  passed. This final checkpoint updates documentation only.
- Live unauthenticated endpoint smoke attempts from this environment returned
  non-JSON responses. Fixture coverage is separate from deployed/authenticated
  provider acceptance; CSV fallback remains available. No broad live-site,
  real-codec or physical GPU acceptance is claimed.
- Next: test this browser with deployed provider credentials and library/converter
  settings. Continue reviewed metadata import as a separate roadmap package;
  follows, pHash comparison and batch automation remain outside this UI slice.

## Gelbooru CSV duplicate compatibility — 2026-10-18 UTC

- Started from fresh develop `a6c4a94109de0be7a78a36cc6a6ff84fbc166309`
  after #147, with a clean tree. No browsing/UI/converter scope changes.
- Reproduced the owner's row-7130 error with the recommended community CSV:
  `gelbooru_2026-06-10_pt20.csv`, 5,366,510 bytes, SHA-256
  `54c4eda3d7a8b8f58ab4db4fb530c9c241840388ba099e628c27dd17f7174a17`.
  It has 234,090 records but 234,010 distinct trimmed tag names. The first repeat
  is `final_fantasy_v` at records 7129/7130; several others disagree on category
  or count. The reported one-million limit was not the cause.
- Import now merges names, taking the highest-count record's category/count
  (first record on ties) and a bounded, deduplicated alias union. Counts are not
  summed. Every row is still validated before merging. Invalid-name and unique
  tag-limit failures now have separate error messages.
- Both original and cleaned full snapshots pass the actual patched Go importer
  with 234,010 tags. The prior importer separately reproduced the exact reported
  record-7130 failure and accepted the cleaned copy with 234,010 tags, verifying
  compatibility for the existing build.
- Local `go test ./pkg/booru` passed, including category/count selection, stable
  ties, alias preservation/deduplication, invalid duplicate rejection and alias
  union limits. Upload/reload tests now include duplicate names and retain
  revision-conflict and invalid-replacement preservation checks.
- GitHub lint, generation/UI validation and mounted-browser checks passed on
  `c4dede2d69716031f1b40b4f491252bc1e4a389e` (local implementation
  `5465ba6d3aa518e93f8deab682832ccbc295cb15`, identical tree
  `f93ae9b5ec2ccdcf72ac9bb0bbef838e0b5023e9`). Full `make it` also passed
  in GitHub CI, including upload/reload/atomic-replacement regression coverage.
  Redundant cold-cache local broad tests/lint were stopped after CI passed;
  only the focused parser tests/full-snapshot checks are claimed locally.
  All 73 relative guide/index links and `git diff --check` passed. This final
  checkpoint changes documentation only; no UI source changed.
- Next: load the original snapshot after updating to the focused fix, or use
  the cleaned compatibility CSV with the prior build. Deduplication does not
  make stale snapshot categories authoritative or refresh the database.

## Booru library downloads and workflow corrections — 2026-10-18 UTC

- Fresh develop, clean tree and closed/merged PRs checked. B02 is in develop;
  B01 media/autocomplete #146 landed on the previous feature branch. Its source
  is included on this feature branch without modifying develop.
- Download to Stash imports a verified original through native Scan handlers,
  with a configurable default folder and per-request Download to… picker.
  Library containment/symlink checks, no-overwrite publication, receipts and
  explicit retry retain files on failed registration/conversion.
- Optional conversion calls the existing converter with saved format/encoding/
  worker/savings defaults and its native activation/recovery path. No hard-coded
  JPEG XL/quality/effort/CPU override remains. Existing metadata/IDs are retained.
- A dedicated Downloads & autocomplete entry exposes upload and server CSV path
  loading, including persisted paths and atomic/revision-checked replacement.
- Local browser memory separates submitted pages/queries from unfinished drafts,
  preserving refresh/bare-route navigation. HTTPS upgrades, public CDN fallback,
  detailed preview errors and Refresh previews provide recovery.
- Verification on 2026-10-18 UTC passed: full `make test`; focused API/manager
  `TestBooru` tests with integration tags; `make lint` with golangci-lint 2.11.4
  (zero issues); all 54 UI tests plus JS/CSS lint, TypeScript and formatting;
  production `make ui-only`; 73 relative documentation links; `git diff --check`.
- The native integration test used real SQLite, PNG, FFmpeg and FFprobe: two
  scans inside native Jobs produced one Image with its actual primary File ID.
  The converter-defaults integration test checks a saved non-default format,
  quality/effort/lossless/allow-larger options and remote backend selection.
- Mounted Chrome Headless Shell 143 passed at 1440×1000 and 390×844: native
  download/default/custom folder payloads, converter preference, server CSV path,
  upload/autocomplete, typed native links, failure/cancellation/reopen cleanup,
  page-two refresh/bare-route restoration with an unfinished query/source draft,
  and direct public-CDN fallback after failed same-origin media delivery. Existing
  search/settings/review/bulk/undo/conflict/race/overflow cases also passed.
  Synthetic screenshots were reviewed at both viewport sizes.
- Checked implementation: local `6f1f933b0605bb8e91f44b4ab1fc8a3f6f9812ad`
  and remote `5a7ce570665a6651ed8c65453562a62629c5f2f5` have identical tree
  `62fe8789d947533025870f32a182544bd08a9f71`. UI source is unchanged from
  the validated production/mounted-browser follow-up. This checkpoint is
  documentation only. GitHub full `make it`, browser, generation/UI, lint and all
  seven platform builds passed on that remote implementation. The redundant local
  full-integration compile was stopped after CI confirmed the same source; local
  focused integration tests had already passed, with no skips.
- A live unauthenticated Safebooru metadata/preview/sample smoke request returned
  successful JPEG responses through the environment's proxy. It does not verify
  the application's native transport, all providers, authenticated accounts,
  real JPEG XL codec output or physical GPU execution.
- Limits: one post per explicit Job, 128 MiB original, existing configured library
  destinations and hard-link support. Metadata entity/tag review and batch import
  remain later work. Next: test native transport/downloads with the deployed
  provider credentials and configured converter worker, then review metadata
  import as a separate package. No live-account or real JPEG XL acceptance claim.

## Browser media and autocomplete — 2026-10-17 UTC

- Fresh develop and open PRs checked; clean starting tree; preserved #145's
  published ancestry. This is a separate owner-requested usability slice.
- Same-origin thumbnail/sample delivery uses opaque, expiring server-issued
  post snapshots, source-instance checks, public DNS/CDN validation, bounded
  transfers and safe redirects. Danbooru asset variants/dimensions are parsed.
- Native cancellable Jobs prepare an explicit post's original for download to
  the user's computer. Original byte counts and provider MD5 are checked when
  known. Optional JPEG → JPEG XL uses the existing local/remote converter and
  JPEG reconstruction settings; converted output has its own checksum.
- Native generated-download storage expires unused exports, waits for active
  readers and guards timers against reused keys. Ordinary failure/cancel cleans
  staging. Session storage recovers the last job in the same tab. No automatic
  catalogue import, native entity changes, false match links or review marks.
- Per-instance CSV databases support tagcomplete and header-based Danbooru tag
  exports. Atomic validated replacement and revision conflicts retain the last
  installed database on failure. Names/aliases/categories/counts drive debounced
  prefix completion, keyboard/mouse selection and cursor-token replacement.
- Verification on 2026-10-17 UTC passed: focused transport/CSV/API/expiry tests;
  full `make test`; full `make it` with SQLite integration tags; `make lint`
  (golangci-lint 2.11.4, zero issues); all 53 UI tests plus JS/CSS lint,
  TypeScript and formatting on Node 20.20.1 / locked pnpm 10.33.0; production
  `make ui-only`; 73 relative guide/index links; and `git diff --check`.
- Mounted Chrome Headless Shell 143 passed at 1440×1000 and 390×844: same-origin
  PNG thumbnails, database upload, partial database failures, alias completion
  at the caret with keyboard/exclusion preservation, conversion preference,
  prepared download success/failure, queued cancellation, reopen recovery and
  closing during a delayed preparation request. Existing search/paging/provider
  failure/settings/review/bulk/undo/conflict/native-link/race/cleanup cases passed.
  Desktop/mobile screenshots of autocomplete, downloads and settings were reviewed.
- Backend gates ran on local implementation `5594d3b4572b8cb78cf7cb5bedf06572e7fc7918`
  (remote `8ad6ff6e9dde499b10b90af532aedeec07354a9f`, identical tree
  `88d8c3321d35068ebd1abca9d811c308187f4ee6`). Final UI lifecycle/fixture
  follow-up `f6e5a485d4ac8900e0edcd25786bbb7929d7aa7b` and remote
  `5f2dded114255c8658cc607fe1bb0b1ee45f38e9` share verified tree
  `edac871170ae836462f7c8183b3157593e9b1480`; backend source is unchanged.
  All UI checks, mounted fixtures and production bundling passed on that follow-up.
  This final checkpoint changes documentation only. GitHub browser CI passed on
  the prior implementation; final-head CI refreshes after publication.
- Tests use synthetic bytes/artwork and a mock remote converter. They verify
  native Jobs/converter-client/download-store integration, not real codec output,
  live-account access, human manual testing or GPU acceptance. No code/model
  dependencies or encoder binaries were installed by this package.
- Known scope: explicit single-post computer exports, no durable restart/resume
  or library import; no live-account/CDN or real JPEG XL codec claim. Provider
  connectivity and configured worker capabilities remain deployment checks.

## B02 — durable review state — 2026-10-17 UTC

- Fresh develop baseline `598d6849668303369fb2a2c5783160a00172ba59` after #144;
  clean working tree, no relevant open PR. Continues the staged roadmap without
  redesigning browsing or implementing B03–B06.
- Migration 97 adds versioned, library-wide ignored/manual-download marks, public
  source provenance and timestamps. Stable normalized engine/URL instance keys
  isolate changed sites; equal provider IDs remain distinct. Credential/display
  edits do not reset history. Cleared tombstones retain versions.
- Atomic bounded single/bulk changes, separate mark dimensions, last-change undo,
  conflict recovery and show-ignored URL state keep provider paging/totals intact.
  Manual history never fabricates native media links or starts a download.
- Local links require rehashing indexed active-primary-file bytes against the
  remote MD5, plus filesystem stability and native-link revalidation. Typed Image/
  Video IDs remain distinct. Checks are bounded to 50 posts, 100 candidates,
  64 MiB/file, 256 MiB/request, five seconds and two concurrent checks. Missing
  hashes, ZIP members, large/changed/unavailable files remain unknown/incomplete.
  No checksum cache or review flag falsely proves current local presence.
- Existing native IDs, files, metadata, relationships and workers are preserved;
  there are no remote originals, metadata imports, jobs or model downloads. Native
  deletion/replacement is re-evaluated on status refresh without deleting marks.
- Verification on 2026-10-17 UTC passed: focused HTTP/integration tests; full
  `make test`; full `make it`; `make lint` with golangci-lint v2.11.4 (zero issues);
  all 52 UI tests plus JS/CSS lint, TypeScript and formatting with Node 20.20.1 /
  locked pnpm 10.33.0; production `make ui-only`; 83 relative documentation file
  links; and `git diff --check`. Final conflict/ID follow-ups were checked with
  the full integration suite, backend lint, TypeScript and mounted browser fixture.
- Migration tests cover fresh schema 97, upgrade 96→97 and the existing P08
  95→96→97 path without losing native records. Tests cover database close/reopen,
  independent marks, instance isolation, exact uint64 remote IDs, atomic stale
  bulk rollback, versioned undo/tombstones and concurrent writers. Current-file
  tests rehash actual bytes, reject same-size/mtime replacements, and refresh
  after file loss, primary-file removal and native Video deletion while retaining
  manual history. Regular-file limits, cancellation and HTTP validation are tested.
- Mounted Chrome Headless Shell 143 passed at 1440×1000 and 390×844 for single/bulk
  marks, undo, conflict recovery/write disabling, hidden-selection clearing,
  show-ignored URL state and provider paging, reload persistence, source isolation,
  typed native links and manual marks without invented local links. Existing
  settings/search/inspection/refresh/back/failure/race/cancel/cleanup/overflow cases
  passed too. Desktop/mobile screenshots were visually reviewed. Fixtures use
  mocked providers and synthetic local bytes/artwork; no human manual, live-account,
  real-media playback or GPU testing is claimed.
- Published as [PR #145](https://github.com/Dusky-dev/StashBooru/pull/145). Local
  implementation `ed91d16dd5d164ae7031b3889868acc0caa24c5d` and remote
  `735d422c1d1b0e0718321d3129d3efb1c4224d06` share verified tree
  `8eb4b114575a0aca5953d3c9fa9a6763cd2190b3`. The final checkpoint changes only
  documentation and screenshot-readiness waits in the fixture. GitHub generation,
  Go lint and browser checks passed on that implementation; final-head CI refreshes
  after the final push. No agent merge/deployment; next is B03 after owner review.

## B01 follow-up — source settings and compact browser — 2026-10-17

- Started clean from the owner's merged #143 on develop
  `8eed7af01eb31a63ed611aca09e23f96d17103c7`, on the branch named above.
- Adds settings in the browser and Settings → Metadata Providers → Booru sources:
  supported engine credentials, compatible custom sites, CDN hosts, enabled state,
  custom removal and preset reset, persisted in the existing server JSON file.
- Saved keys are never returned. Blank keys retain credentials; explicit clearing
  removes them. URL/engine changes require key replacement/removal. Strict bounded
  JSON, normal authentication, same-origin browser writes, revision conflict
  detection and atomic owner-only file writes apply. Keys remain plaintext server
  secrets; no new engine/login compatibility is claimed.
- Replaces the large form/cards with a search/source sidebar, categorized page tags
  and compact thumbnails. Mobile has stacked controls, two-column previews and
  expandable tags. Tag counts cover one loaded source page; clicking a tag prepares
  a draft without searching. Per-source paging/errors, inspection URLs, cancellation
  and remote IDs remain independent.
- No database/schema/migration/dependency-lock changes, catalogue writes, model
  downloads, develop push, agent merge or deployment. B02–B06 remain outside scope.
- Verification on 2026-10-17 UTC: fresh backend/UI generation; full `make test`;
  full `make it`; `make lint` (golangci-lint v2.11.4, zero issues); and adapter
  `go test -race ./pkg/booru` passed. Full UI validation passed all 51 tests,
  JS/CSS lint, TypeScript and formatting with Node 20.20.1 / pnpm 10.33.0.
  Final duplicate-ID protection was additionally checked with component lint,
  TypeScript, formatting and the mounted fixture. Production `make ui-only`,
  49 relative documentation file links and `git diff --check` passed.
- Mounted Chrome Headless Shell 143 passed at 1440×1000 and 390×844: credential
  entry/blank retention/clearing, stale-edit recovery, duplicate-ID rejection,
  custom creation/reload/disable/removal, tag drafts, independent paging/errors,
  inspection/back/refresh, failed previews, cancellation, cleanup and overflow.
  Desktop/mobile screenshots were visually reviewed, including the mobile editor.
  The fixture uses mocked APIs and synthetic artwork. No human manual testing,
  live-account authentication, real-media playback or deployment is claimed.
- Published in [PR #144](https://github.com/Dusky-dev/StashBooru/pull/144).
  Verified local implementation `80d24f1c9ccbbe2bc04d492abac6dbbb2d424dd7` and remote
  `3fdeecadfa93aeafffc0c07d99fb7aa7d8691a4b` share tree
  `a96233d1941ae39dc6713db39559cab6d7d9ecdc`. The final ledger checkpoint changes
  documentation only. GitHub generation, all seven platform builds, integration
  tests, Go lint and the browser workflow passed on preceding implementation
  `0b16fe37525e03181ecd44a299fcfd54d9c4f681`; final-head CI refreshes after publication.
  No agent merge/deployment. Next is B02, after owner review of this follow-up.

## B01 — multi-source read-only browser — 2026-10-17

- Started from the verified B00 tree on develop `8fc869d879`; fresh develop after
  the owner's merge of #142 is `2af939ff5b2a4d90336f0afbda8b8a5e2f514d76`, tree
  `072920c77d23f6d70a7843d51739b5c350f323c6`, identical to that local B00 tree.
  The browser branch publishes on this merged baseline, without merging itself.
- New `pkg/booru` adapters cover Danbooru, Gelbooru JSON DAPI, Moebooru, current/
  legacy e621 and Philomena. Ten presets plus server-owned compatible definitions
  in `booru-browser.json`; ID-only disabled entries remove presets. Credentials
  are server-side and redacted from source listings and transport/config errors.
- Native `/booru` page and navbar globe use normal React/Bootstrap components.
  Up to eight selected sources search independently, with separate errors,
  cancel/retry and paging. URL state retains query/source/page/inspection;
  source-tag categories and missing metadata remain explicit. No original-file
  download, metadata/entity import, database write or model inference occurs.
- Bounded public-HTTPS transport validates/pins DNS destinations, blocks private/
  reserved/mixed DNS results and redirects, limits concurrency and per-host rate,
  honors bounded Retry-After, and cancels obsolete work. Raw queries retain native
  source syntax. The existing local-file MD5 lookup path is unchanged.
- Validation passed: backend/UI generation; full `make test`; full `make it`;
  `go test -race ./pkg/booru`; focused route tests with Makefile SQLite include
  flags; `make lint` with CI's golangci-lint v2.11.4; all 51 UI tests plus JS/CSS
  lint, TypeScript and formatting (`make validate-ui` equivalent, Node 20.20.1 /
  locked pnpm 10.33.0); production `make ui-only`; diff whitespace checks.
- Mounted Chrome Headless Shell 143 passed desktop 1440×1000 and mobile 390×844
  search, independent paging, inspection, refresh/back, partial failure/retry,
  stale-response rejection, cancellation, cleanup and overflow checks. The fixture
  uses native theme styles, mocked APIs and synthetic artwork; no human manual
  testing or real-media playback is claimed. A dedicated Booru browser PR workflow
  runs these checks with isolated pinned Playwright tooling.
- Live check limits: a small metadata-only probe obtained usable Safebooru JSON
  through the environment's outbound proxy; the other sampled providers returned
  non-JSON intermediary pages. The native public-DNS/direct transport could not
  reach Safebooru here. This does not verify live account access or every preset;
  fixture compatibility is not advertised as universal live availability.
- Remaining limits: five API families, not Grabber-wide engine/login parity;
  custom sites require server-file configuration; private/local URLs and environment
  proxies are not supported; Moebooru password login is absent; flat tag categories
  stay unknown. Listings are live per-source numbered pages, without cross-page/
  cross-source deduplication, snapshot totals, ignores, follows or downloads.
- Migration/API/dependency impact: no migration, GraphQL/schema or dependency-lock
  change. Adds authenticated read-only source/search HTTP routes; existing media
  IDs, metadata, files and workers remain untouched.
- Published as [PR #143](https://github.com/Dusky-dev/StashBooru/pull/143), head
  `d55d4c713aeef6f508d3e7ea5f703309f51c2d5f`. Its tree
  `993cc934e7b8d6b681561e8f62b5efcc9e5e41aa` exactly matches locally reviewed
  `e0eef7daa8842653395d462561b688a4b08b2f1f`. GitHub Build, Go lint and the new
  browser workflow remain PR gates. No agent merge/deployment; next is B02.

## B00 — fork documentation and booru-browser plan — 2026-10-17

- Read the owner's handoff, root AGENTS.md, current progress/package guides and
  open PR state (none at the start). Working tree was clean at the baseline.
- Audited the fork-only history and changed paths against the upstream parent
  `bdee3c83d33d802765332967f09db756e2877bcc` of the current merge. Read current
  native metadata/tagging, similarity/triage, settings, artwork, backup, HTTPS
  and worker sources rather than treating historical PR descriptions as live UI.
- Added a documentation index and metadata/tagging guide; expanded the feature
  guide and README to cover pre-P01 features as well as P01–P13 and later QoL.
  Coverage includes native Copyrights/multiple Artists, variants/context,
  hierarchy/inheritance, mixed browsing/preferences/format filters, all explicit
  Image/Video tagging sources and bulk workflows, artwork editing/fallbacks,
  Metadata Health/alias diagnostics, EVA02 triage/add-only metadata copying,
  pHash/EVA02 search, bounded comparison, stacks, conversion/trials/restore,
  upscaling, animation inspection/timing, Video overlap, mask restoration,
  storage prototypes, jobs, local HTTPS and independent versions/releases.
- Corrected worker setup's old System navigation, missing companion-module list,
  separate upscaling UI/cache and new overlap/restoration endpoints. Corrected
  converter documentation's old inspection location and historical schema-91
  wording. Existing implementation/API behavior is unchanged.
- Added B01–B06 with explicit outcomes, dependencies and acceptance checks:
  read-only browser; durable ignored/manual-download/current-local evidence;
  metadata and Artist × Copyright intersections; recoverable downloads; pHash
  review/Compare/red accessible tag warnings; follows/compound-search feed.
  Existing MD5 enrichment, favorites and EVA02 clusters are identified separately.
- Important status details retained: P01's further backup contract was deferred
  by the owner; P02 has two modes without Combined; P03 lacks automatic alignment
  and whole-animation equality checks. Dormant taxonomy controls are not described
  as active features. New browser behavior remains explicitly planned.
- Migration/API/dependency impact: none. Documentation only; no media writes,
  model downloads, production changes, direct develop push or automatic merge.
- Verification: `git diff --cached --check` passed. A one-off documentation check
  passed for all eight changed Markdown files, 80 relative file/heading links
  and fenced-code pairs. Source review checked current UI labels/source priority,
  model/config defaults, backup contents, HTTPS settings and worker routes/imports.
  Historical package records and benchmark files were preserved. No runtime build,
  model inference or live-provider tests were run for this documentation-only diff.
- Publication: [PR #142](https://github.com/Dusky-dev/StashBooru/pull/142), branch
  `docs/fork-guide-booru-roadmap-20261017`. Published implementation commit
  `242b1c1bf74f6012b338aba09c4702f8e78c0dc3` has tree
  `4acb5e74c3e7fdb7b66486c5cad745a176b8810e`, matching locally reviewed
  `cda00063941c995dcfea6c021f5080489b1b1d77`. Next is B01, with official Gelbooru API verification
  before implementation. No runtime/browser feature is claimed by B00.

## Previous checkpoint — 2026-10-06

Baseline at that checkpoint: `develop` at `f5579d46940dcc8ad6a96d69c264908f36be85d6`
(PR #140 merged; StashBooru 1.0.0 released). Owner-requested package:
conservative Character duplicate merging and native Copyright task auto-tagging,
on `feat/character-dedup-copyright-autotag-20261006`. Published as [PR #141](https://github.com/Dusky-dev/StashBooru/pull/141)
at remote head `8cbbef390f481f863915482f10978572efb0b10d`.
See [Character matching notes](character-dedup.md) and [feature guide](features.md).
P11 model/GPU acceptance and P12/P13 production activation gates remain open.

## Character identity cleanup — 2026-10-06

- Adds a reviewed automatic task for full-name duplicates, reversed two-part names
  and uniquely matched whole-token shortened names within identical direct native
  Copyright sets. Ambiguous, context-free and variant identities stay separate.
- Preserves compatible profiles and media links; conflicting values or portraits
  require manual resolution. Preview fingerprints are rechecked inside an atomic,
  cancellable job before any source Character is removed.
- Copyright task auto-tagging is wired through GraphQL input/defaults and Tasks UI,
  with wildcard/selected Copyrights, alias matching and selected-path filtering for
  Images/Videos. Omitted options preserve previous behavior.
- Copyright directory sorting now defaults to a hierarchy-aware order: each root
  precedes its descendants, multi-parent nodes use a deterministic path, and saved
  sibling order is respected. Existing flat sort options are unchanged.
- Restores the historical progress text accidentally truncated during PR #140
  publication from the intact local verification commit `207b7f2`.
- Verification: focused and full unit tests passed; backend and UI GraphQL
  generation passed; TypeScript, Biome lint/format, production UI build, and
  mounted Chromium desktop/mobile fixtures passed. SQLite hierarchy sorting,
  Character preservation/rollback, stale review/API, Copyright task selection
  and legacy-default tests passed. The standalone stylelint rerun was interrupted
  when pnpm attempted to recreate the local node_modules directory without a TTY;
  the earlier `make validate-ui` run passed before this final browser-only rerun.
  No production data has been merged.

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

## Gallery chapter retry and pending actions (E12) — 2026-10-24

- Continued the owner's “merge and continue” request on fresh develop
  `4e430fb2b1e1daf0f5b5f5999749a0f3c3ac999a` in
  [`fix/library-editing-20261024`](https://github.com/Dusky-dev/StashBooru/tree/fix/library-editing-20261024).
  At the initial check, #205 had all 15 CI checks passing but Codex Review was
  still running on `5eeace8a7aad12b14fa16b5f249b4672b9e64b6b`, without findings.
  Its older quota response does not override the fresh running review; no merge
  was performed at that checkpoint.
- An actual-server baseline check reproduced a rejected chapter create closing
  the editor and discarding its draft. The server rejected Image 3 in a Gallery
  containing only two Images. Source inspection found the same unconditional
  finally-close path in create/update and delete.
- Close the native chapter editor only after successful mutation. Rejected
  create/update/delete retain the current title and Image number and the existing
  error toast, allowing correction/retry. Reuse Apollo mutation loading and
  Formik submission state to disable the native fieldset and actions while
  pending, expose aria-busy and guard overlapping mutation handlers. No new
  editor, route, CSS, store or persistence/API contract.
- Verified source `1f1d4f7b2e72b0f2909d4a3509f99d2c96fb2d80`, tree `5d2eed08cbe75fd0f97d995b99dd198abcb63f73`,
  exactly matches the tested local tree. The implementation branch was saved
  remotely before this ledger-only follow-up.
- Added gallery-chapters.mjs, which owns its configuration, database, generated
  PNGs and loopback server independently at 1440 × 1000 and 390 × 844. Both runs
  passed rejected create/update, exact retained drafts, disabled pending fields/
  Save/Cancel/Delete, successful retry, saved chapter identity/title/index after
  refresh, rejected delete retaining an unsaved draft, valid delete and its
  persistence after refresh. Exact native Image records/Gallery membership,
  Gallery title and two-image count remain unchanged.
- Requests are held briefly for pending-control assertions. Create/update use
  a naturally out-of-range index; delete changes only the outgoing chapter ID
  to an invalid fixture value. The real server supplies every response and
  persists successful writes. No response is mocked. Baseline and final desktop/
  mobile screenshots were inspected. The native dark layout remains usable
  without document/form horizontal overflow in these viewports.
- Validation: UI GraphQL generation, all 69 UI unit tests, JS/CSS lint,
  TypeScript, formatting, production make ui-only/built-in composition passed.
  After refining the test's request synchronization, focused Biome check and the
  full format gate passed, as did browser/built-in JavaScript syntax, workflow
  YAML/step validation and git diff --check. No backend/schema changes require
  local Go generation/test gates.
- Add the real-server chapter check to the existing first-library CI job,
  reusing its Linux artifact and pinned browser tools rather than another
  compile/install job. Its captures and logs use the existing artifact's chapters
  directory. P07 describes the current retry/pending flow and runnable acceptance.
- Limits: synthetic Chromium/SQLite fixtures, not owner-library, physical-device,
  chapter playback or Collection editing acceptance. Baseline runtime reused
  #205's packaged Linux artifact; its chapter code matches fresh develop. Final
  validation served this production UI through STASH_UI. Successful temporary
  libraries were removed; no owner media/configuration was used.
- Next: applicable CI and completed Codex Review on the final head; end when only
  external gates remain. Continue Collection hierarchy/ordered editing as its own
  bounded acceptance pass, followed by recovery/upgrade/plugin workflows.
