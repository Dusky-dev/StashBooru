# StashBooru implementation progress

Updated: 2026-10-23

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
