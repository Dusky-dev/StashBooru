# Experience audit and consolidation plan

Reviewed 2026-10-22 against `develop` at
`b36daf9f0d548df25facc73fc98857764c4b61ae` (including merged PR #171).

The project has useful capabilities, but everyday tasks expose too many of their
implementation boundaries. The next work should reduce context switches and make
existing behavior predictable before extending the feature list. This is a
whole-project assessment, not a proposal to rebuild Stash or discard its safety
contracts. The booru agent’s independent pass is complete. The owner returned booru-browser
maintenance to this stream on 2026-10-24; it follows the same bounded-fix and
acceptance process.

## Current progress — 2026-10-24

The first consolidation passes are merged. Work is now concentrated on actual
workflow acceptance and the defects it exposes, while retaining the existing
native data model and safety contracts.

| Area | Delivered so far | Remaining acceptance/scope |
| --- | --- | --- |
| Connections, metadata, jobs and settings (E01–E06) | Contextual source connections and metadata review; shared job feedback/retention guidance; grouped Tasks/Tools and profile entry points; processing save feedback; contextual Following setup. | Live-provider and broader Following adapter workflows, representative bulk/partial-failure and restart/recovery acceptance. Contextual setup does not add unsupported source adapters. |
| Everyday library use (E07, E11, E12) | Real setup → scan → first edit/restart checks; native selection/viewer fixes; refreshed inherited metadata; Gallery/Collection context; chapter retry, Collection hierarchy/order/unlink, and Video numbering/repeat-save and Image custom-field save/retry acceptance; reachable mobile Image editor controls; Artist and Tag creation/save and hierarchy-retry acceptance; backup rejection/retry and saved/downloaded same-schema restore with both blob-storage modes; a pinned 99→100 upgrade/old-app restore and local plugin task/UI-extension acceptance. | Other editor/Save & New and representative-library acceptance; other historical upgrades, broader plugin/package/runtime compatibility, processing recovery, physical-device/accessibility and performance checks. |
| Processing and recovery (E08) | Clearer conversion, upscaling and mask-restoration output/original-file guidance. Existing validation, activation and recovery contracts remain. | Real model/GPU/worker combinations and representative processing/restore/restart workflows. Synthetic UI checks do not close these gates. |
| Documentation, mobile settings and terminology (E10, E13, E14) | Current feature guides, compact mobile settings navigation, bounded native English label fixes and fork-aware first-use guidance. | Wider terminology/localization and owner usability testing. |
| Native UI ownership (E09) | Touched workflows reuse native components and supported extension points. | Remaining DOM adaptations are addressed when their workflow is fixed; a wholesale rewrite is deferred. |

Remaining work falls into five broad workstreams, each with multiple acceptance
cases and possible fixes. These are not five remaining PRs or a percentage of the
original feature roadmap:

1. **Native library journeys:** other editors and Save & New, bulk/partial-failure
   paths, and representative search, playback, similarity and relationship use.
2. **Providers and Following:** real supported-account preview/download/receipt
   journeys, credentials and partial failures. Additional adapters remain feature
   work; contextual setup does not implement them.
3. **Processing and recovery:** real-file conversion/restoration, cancellation and
   restart, plus representative models, physical GPU and remote-worker operation.
4. **Upgrade and extension compatibility:** historical versions beyond the pinned
   99→100 path, other blob/restore combinations and broader plugin packages,
   runtimes and rejected settings/enable operations.
5. **Owner usability and scale:** a representative existing library, physical
   mobile/accessibility checks, performance and remaining terminology/localization.

Native Character Save & New and save/retry are covered by merged
[PR #215](https://github.com/Dusky-dev/StashBooru/pull/215); its eventual-review
follow-up remains in the ledger. Artist and Tag passes validate Save & New before
a request and cover duplicate-name/hierarchy-cycle rejection and retry, clean next
creation, Cancel, Back and restart. The Tag fixture preserves multiple parents and
separate native Copyright/Video records. The native Copyright save/retry pass is
described below. The Image suite validates
existing repeat saves and fixes narrow-layout control reachability; it does not
change the Image form or combine its separate association writes into a transaction.

The findings below are the original audit snapshot, followed by dated delivery
notes. [Implementation progress](implementation-progress.md) records exact PRs,
checks and outstanding gates; the feature guides describe current user flows.

The Copyright pass covers its separate native create/edit form: rejected
duplicate names and hierarchy cycles retain the draft. If Character-link saving
fails after metadata creation, correction retries the same native Copyright ID.
Its owned desktop/mobile fixture also covers existing-record partial-link retry,
Cancel, Back, refresh, restart and unchanged saved artwork. The two native writes
remain separate transactions. Its saved-profile captures exposed a separate
Characters-tab failure: the profile sends a Copyright criterion unsupported by
PerformerFilterType, and the server returns HTTP 400/unknown field. Fix that
native branch-list path next; Image/Video bulk-save and partial-failure acceptance
remain afterward. Some captured error toasts are clipped at the viewport edge;
their positioning and animation need a separate narrow/wide review.
The broader workstreams above remain open.

## Scope and evidence

This audit traces current routes, components, state ownership, backend contracts,
user guides and browser-test coverage. **Confirmed** below means a behavior or
placement is present in source; it does not imply a live usability study.
**Risk** means a maintenance or consistency concern requiring reproduction before
being treated as a defect. **Validate** means the workflow needs hands-on checks
with representative media, settings or provider accounts.

Coverage includes inherited setup, library lists, filters, profiles, playback,
scraping, tasks, settings and plugins, plus fork metadata, identities, Following,
remote browsing/downloads, comparison, stacks and processing/recovery. Offline
storage prototypes are considered separately from daily UI. Large-library
performance, live provider reliability, real codec coverage, accessibility with
assistive technology and physical GPU/model operation are not established by
source inspection or synthetic browser fixtures.

A full application instance was also built from this baseline and inspected with
an isolated, empty SQLite library. Its production UI assets came from `9bacf6f`
(the PR #171 head), whose tracked UI source matches this baseline. Eighteen routes
were opened at each of 1440 × 1000 and 390 × 844. After dismissing the first-use
release notes, these 36 route visits produced no uncaught page errors or document
horizontal overflow. This checks initial rendering and visible controls only;
it does not establish interaction, playback, populated-list or processing success.
External browser requests were blocked; no provider credentials or model downloads
were used. Optional local inference reported missing `onnxruntime`, so no inference
acceptance is claimed.

| Runtime observation | Implication |
| --- | --- |
| Fresh UI opens the inherited release-note history headed v0.29.0, rather than a fork-specific introduction. | First-use guidance needs to distinguish upstream history from current fork behavior. |
| At 390 × 844, Processing begins below the stacked settings menu, around 540 px down the initial viewport. | Mobile settings need a compact section chooser or a direct jump to content. |
| All's Artist filter still displays “Subsidiary Studios”; Tasks/Tools expose labels such as “Scene covers” and “Scene Tools”. | The fork's vocabulary is only partly applied to inherited UI. |
| Tasks shows queue, generation, cleanup, overlap and database operations in one long panel; Tools exposes GraphQL beside routine metadata diagnostics. | Group by user goal and keep developer tools discoverable in advanced sections. |

## The experience to aim for

| User goal | Main place | Advanced work |
| --- | --- | --- |
| Find and view saved content | All, Images, Videos and existing profile media lists | Saved filters, exact provider identifiers and specialist similarity modes |
| Find new content | Existing remote browser and Following | Provider query syntax and supported-source configuration |
| Organize content | Existing media/profile edit and reviewed metadata actions | Bulk reconciliation, collision diagnostics and inheritance review |
| Process content | Actions on the current item/selection | Worker/model defaults, recovery history and technical diagnostics |
| See what happened | Consistent job feedback linked to Tasks and affected media | Detailed errors, receipts, recovery and logs |
| Configure the server | Settings, with contextual links from blocked actions | Server paths, plugin/developer tools and experimental facilities |

Artists, Characters and Copyrights remain native library records. A source alias
connects to one of those records; it should not feel like another kind of artist
the user has to maintain. Keep ordinary tags simple. Keep Galleries, Collections
and stacks distinct where their actual behavior differs, and explain that
difference at the action that creates them rather than adding another umbrella
entity.

E14's first pass corrects a bounded set of confirmed English labels: Parent/Child
Artists and related hierarchy counts, generated Video covers/sprites/previews,
and Video/Artist/Character view headings. The current Tools heading was already
Video tools when reassessed. The English fallback serves both English variants;
non-English translations and custom override precedence are preserved. Native
message IDs, relationship fields and API names stay unchanged. The built-in
Advanced mover no longer relocates native Artist hierarchy fields by matching
English labels. Mounted native-editor checks verify visibility with that script
loaded and parent/child IDs retained on save. Remaining inherited terminology
remains separate E14 work. A second pass now shares a short dated StashBooru
introduction between Welcome, first-unseen release notes and Changelog. Existing
lastNoteSeen filtering and explicit Close/Finish acknowledgement remain; upstream
history stays available. Welcome's English next-step text matches its Library
settings destination, and fork issues use the fork repository. The introduction
is English; this does not claim complete localization or first-scan acceptance.

## Prioritized findings

Priorities below concern consolidation value, not security severity. Origins
describe the current surface, rather than assigning blame to upstream or a PR.

| ID / priority | Finding and evidence | Origin / confidence | Bounded change |
| --- | --- | --- | --- |
| E01 / first | Connecting a provider name leaves the preview for `/identities`; a native profile's Sources & aliases also sends users to Manage connections. See [identity actions](../ui/v2.5/src/components/Identities/identity.tsx) and [current workflow](provider-identities.md#using-identities). | New / confirmed | Reuse the existing resolver in a contextual dialog with Choose existing, Create and Disconnect. Keep bulk reconciliation available as an advanced entry point. |
| E02 / first | Related metadata actions use Tagger, Image/Video Tagging, Generate → Review Image Metadata, and Auto Tag. Auto Tag matches filenames/paths; it is not model inference. See [GenerateDialog](../ui/v2.5/src/components/Dialogs/GenerateDialog.tsx), [LibraryTasks](../ui/v2.5/src/components/Settings/Tasks/LibraryTasks.tsx), and [tagging guide](metadata-and-tagging.md). | Inherited + new / confirmed | Put metadata review on the selected item/selection and label methods by what they do. Preserve native scrape/tagger workflows and reviewed apply. Generate should primarily describe generated assets/hashes. |
| E03 / first | Tasks has the native queue; download and identity workflows have additional status panels. Failed-job notices persist only in the same browser tab, while download receipts and original-file recovery have different durable stores. See [JobQueue](../ui/v2.5/src/hooks/JobQueue.tsx), [download status](../ui/v2.5/src/components/BooruBrowser/BooruDownload.tsx), and [job guide](qol-jobs-stacks.md). | Inherited + new / confirmed | Share job state labels, cancel/error presentation and links back to affected items. Explain which history survives refresh/restart; do not imply durable job history exists until implemented. Keep feature-specific result detail. |
| E04 / next | Settings → Tasks mixes queue status, library generation, Character cleanup, animation inspection, association review, Video overlap, data management and plugins. Settings → Tools mixes Metadata Health, Similarity Triage, Alias Collision inspection and developer utilities. See [Tasks composition](../ui/v2.5/src/components/Settings/Tasks/SettingsTasksPanel.tsx) and [Tools](../ui/v2.5/src/components/Settings/SettingsToolsPanel.tsx). | Inherited + new / confirmed | Lead Tasks with running/recent work; group launchers by goal. Link cleanup tools from the corresponding profiles/review flows. Keep one implementation per tool and preserve bookmarked URLs. |
| E05 / next | Processing correctly groups defaults now, but save scopes differ: conversion/tagging defaults are server-owned, upscaling job defaults are browser-owned, and upscaler model paths are server-owned. See [Processing](../ui/v2.5/src/components/Settings/SettingsProcessingPanel.tsx), [upscaling settings](../ui/v2.5/src/components/Settings/MediaUpscalingSettings.tsx), and [feature guide](features.md#where-things-live). | New / confirmed | Use consistent scope and save feedback beside controls. Add contextual links to exact settings sections; retain separate Save actions unless a combined transaction is actually supported. |
| E06 / next | Following asks for source, category and exact provider expression; its first feed adapter supports Danbooru-compatible sources. Browse / Download then opens the browser. Local profile favorites, remote account favorites and follows are different operations. See [Following](../ui/v2.5/src/components/Following/Following.tsx) and [follow contract](booru-follows.md). | New / confirmed | Offer Follow from an existing supported source/profile context with the exact target prefilled. Present source capability limits before setup. Reuse browser preview/download actions where practical, without promising Twitter or other adapters already exist. |
| E07 / in progress | All, Images, Videos and related media lists share native infrastructure, while remote browsing and Following have distinct lists/actions. Existing viewer regressions required caller ownership and queue restoration fixes. See [MediaList](../ui/v2.5/src/components/Media/MediaList.tsx) and [viewer acceptance notes](p07-global-media.md). | Inherited + new / real-library range-selection and Video checkbox bugs reproduced and fixed; broader acceptance remains | Adopt common interaction rules for selection, Back, refresh, Escape and preview focus. Keep local structured filters separate from provider-native query grammar. Do not force one query parser or identical capability menus everywhere. |
| E08 / partial | Conversion, upscaling and restoration intentionally have different output and recovery contracts. Their controls can expose technical cache/worker concepts before the user understands the result. See [processing comparison](features.md#conversion-upscaling-and-restoration), [conversion store](../pkg/mediaconvert/store.go) and [upscaling settings](../ui/v2.5/src/components/Settings/MediaUpscalingSettings.tsx). | New / confirmed contracts; presentation needs validation | Start with output choice, destination and original-file policy; reveal advanced codec/model controls as needed. Share a History & restore presentation only after mapping each tool's valid operations. Preserve staging, validation, restore caches and backups. |
| E09 / later | Native React is supplemented by composed built-ins; some use supported PluginApi patches, while others move DOM nodes with selectors and MutationObserver. See [loader](../ui/v2.5/src/index.tsx), [file-info adaptation](../ui/v2.5/builtin-source/universal-file-info.js) and [extra presentation changes](../ui/v2.5/builtin-source/bundle-extra.js). | Earlier fork + new / architectural risk | Move a touched DOM adaptation into its owned native component when fixing that workflow. Keep supported plugin extension points. Do not rewrite the whole built-in bundle or mistake native All for the retired synthetic implementation. |
| E10 / now, docs | README and docs index still call follows planned. The browser roadmap still describes feature-branch delivery and says provider mappings do not exist, despite merged Following and identities. | Documentation / confirmed | Correct current status and link operation guides. Keep dated implementation history as history rather than asking users to interpret the long progress ledger. |
| E11 / owner acceptance next | Setup collects paths and the home page shows configurable recommendations; source/model setup and first-use discovery are separate flows. See [Setup](../ui/v2.5/src/components/Setup/Setup.tsx), [FrontPage](../ui/v2.5/src/components/FrontPage/FrontPage.tsx) and [first-library acceptance](first-library-acceptance.md). | Inherited foundation + new requirements / isolated real-server desktop/mobile walkthrough added 2026-10-23; owner usability testing remains | Keep the tested setup → scan → All → native Image edit path. Add empty-state links only where an observed next step is missing; do not introduce another setup wizard. |
| E12 / validate | Search, relationship editing, Galleries/Collections, playback, mobile density, plugins and advanced settings have substantial inherited behavior. The new-feature tests alone cannot establish their usability or compatibility. | Whole project / validation needed | Use the cross-feature acceptance matrix below with an isolated library, then owner testing with representative data. Fix reproduced regressions before speculative layout changes. |
| E13 / next | The inherited settings navigation stacks above the content on mobile; Processing starts roughly 540 px down at 390 × 844. See [Settings layout](../ui/v2.5/src/components/Settings/Settings.tsx). | Inherited layout + expanded sections / runtime confirmed | Use a compact mobile section chooser or Jump to settings content, retaining URL tabs, anchors and keyboard navigation. |
| E14 / partial | Visible terminology mixes Artists with Subsidiary Studios, and Videos with Scene covers / Scene Tools. First-use release notes lead with upstream v0.29.0. See [release-note selection](../ui/v2.5/src/App.tsx) and [inherited notes](../ui/v2.5/src/docs/en/ReleaseNotes/index.ts). | Inherited UI + fork vocabulary / runtime confirmed | Audit visible labels and localization consistently; provide concise fork-aware first-use guidance while retaining upstream history. Preserve internal Studio/Scene identifiers and compatibility APIs. |

### Consolidation boundaries

- Keep native IDs, typed links, source provenance, ambiguity review, optimistic
  conflict checks and direct-versus-inherited associations. A shorter flow must
  not guess identity from a matching display name.
- Keep All plus the specialist Images/Videos pages. Reusing their selection and
  viewer contracts is more valuable than removing useful views to reduce counts.
- Booru settings already combine downloads, autocomplete and sources on their
  own back-aware route. Main Metadata Providers reuses the same source editor;
  that is shared configuration with two entry points, not two independent stores.
- Do not replace every REST endpoint with GraphQL or combine all durable and
  transient state into one table. Share request/error/job adapters where that
  removes real duplication, while retaining each feature's ownership contracts.
- No automatic provider polling, downloads, linking, merges, deletion or model
  installation should be introduced as a convenience shortcut.
- P12/P13 are offline storage prototypes, not competing daily storage controls.
  P11 still needs real model/GPU acceptance. Neither counts as production-ready
  merely because the P01–P13 implementation packages were delivered.

## Delivery order

Each pass should be one coherent PR or a small, independently reviewable pair.
Do not start all passes together. Stop feature expansion during the pass and
avoid touching the separately assigned browser layout.

### 1. Connect source names without leaving the task

Use the existing identities backend and native entity picker in a reusable
Connect dialog. Open it from Sources & aliases on native profiles and from the
existing preview action; retain the original search expression and preview
position. Keep bulk Identities available under an advanced maintenance entry
point, with existing deep links working. Coordinate the preview action contract
with the browser owner before editing their files.

Accept when the owner's `waligner`, `waligner_(artist)` and
`creator:patreon:49481727` example can connect to one native Artist without opening
a separate management page. A matching name is a suggestion, never proof. Cancel
creates no native entity or binding, create is explicit, disconnect affects only the chosen binding,
stale changes retain the user's draft and canonical search still finds the same
Artist. This pass adds no schema, provider engine or automated scan.

### 2. Make metadata actions predictable

Expose the current reviewed metadata workflow directly for one item and selected
items. Clarify filename matching versus provider lookup versus model suggestions.
Share review/apply feedback while retaining method-specific evidence and
availability. Keep existing entry points/deep links during transition. Do not
rewrite scraping engines or silently flatten general tags and entity categories.

Accept when a user can tell before starting what will be read, whether a provider
or model is needed, what will change, and how to cancel. Single and bulk actions
must preserve selection and resume context; failed items must remain identifiable.

### 3. Follow work through completion and recovery

Standardize status labels, cancellation, Copy error, affected-item links and
return navigation around native jobs. Inventory current retention first. Present
durable receipts and restore entries as their own result records, rather than
claiming the browser-tab notice store is a server audit log. Decide whether a
durable job history is needed before adding new persistence.

Accept on success, partial failure, cancel, refresh, reconnect and server restart.
Show retained files/results accurately. Conversion/upscaling original activation
and restore must keep their existing validation and recovery behavior.

### 4. Consolidate navigation and settings using observed friction

Review top-level destinations and contextual actions after the earlier flows
work. Respect enabled-menu preferences and core catalog order. Keep configuration
in Settings, operations near their items, and technical diagnostics in advanced
sections. Reuse existing source configuration and section bookmarks. Do not hide
features behind a new abstract dashboard merely to shorten the navbar.

Accept with a fresh profile and an existing configured profile at desktop and
mobile widths, including keyboard-only navigation and old bookmarks. Follow the
same capability and return-context rules in Following and the browser.

## Cross-feature acceptance matrix

This is required follow-up coverage, **not a claim that these journeys all passed
during this source audit**. Existing isolated browser fixtures are useful building
blocks; they do not cover every handoff or real provider/model/file behavior.

| Journey | Verify before declaring streamlined |
| --- | --- |
| New server → folder → scan → view | Useful empty state, clear root paths, visible job result, first Image and Video reachable, no optional model prerequisite |
| All / Images / Videos → profile → Back | Filters, sort, page, scroll and typed selection retained; Grid/Wall/List where supported; no invisible enabled-menu entries |
| Select → preview → close/reopen | First-open keyboard focus, Escape and Back, mixed-media caller ownership, repeated opens, mobile controls, no background playback |
| Profile → relationship edit → search | Artist/Character/Copyright/variant identity preserved; direct and inherited relationships understandable; ordinary tags stay ordinary tags |
| Provider name → connect → search | Existing/new/disconnect/cancel/conflict; raw provider target retained; same numeric IDs on different sources remain separate |
| One item / selection → metadata review | Method names, unavailable provider/model help, ambiguous matches, partial failures, cancellation and return context |
| Following → source preview → download → local item | Only supported capabilities offered, exact target retained, saved source credentials reused, no automatic network check/download, receipt versus verified local match distinguished |
| Convert / upscale → result → restore | Scope, defaults ownership, explicit replacement/copy, worker error/cancel, refresh/restart, preserved original and metadata, valid restore/unrestore choices |
| Similarity → compare → stack / cleanup | pHash versus embedding evidence clear; local stacks distinct from remote review groups; no merge/delete from similarity alone |
| Gallery / Collection → member → Back | Clear membership semantics, ordering, filtering and batch-action scope; empty states and inherited editing still work |
| Settings / Tasks → result → return | Section links, per-section save/scope, basic/advanced mode, meaningful errors, persistence limits and retry semantics |
| Backup / upgrade / plugins | Isolated database upgrade/restore, media backup distinction, upstream-compatible extension points, disabled-feature behavior; no production data used for inspection |

Record each result with the exact SHA, viewport, fixture/live-data boundary and
reproduction steps. Track observed defects separately from design preferences.
Performance checks need representative catalog sizes; screenshots alone do not
establish performance or accessibility.

## Mobile settings navigation follow-up — 2026-10-22

E13 now uses a labeled native section chooser below 768 px, with the existing
sidebar at wider sizes. The same section list powers both controls; only the
current sidebar section is highlighted. This also avoids the crowded sidebar
layout at narrow tablet widths. Section URLs, moved-section bookmarks, advanced
mode and the existing panel save/state ownership remain in place.

The mounted Settings fixture checks desktop/mobile navigation, keyboard use,
Back, refresh, unrelated URL parameters, section anchors, draft retention and the
575/576/767/768 px boundaries. Its configuration, HTTP responses and writes are
local fixtures; it is not a fresh-server or production-library acceptance claim.
Tasks launcher grouping (E04) and the remaining E03 real-job/restart acceptance
remain open. See the dated verification in the [progress ledger](implementation-progress.md).

## Tasks grouping follow-up — 2026-10-23

E04's first pass keeps the native queue first and groups existing task launchers
by goal in native expandable sections. Existing components remain mounted when
closed, retaining drafts and review results. Bookmarks reveal the containing
section, including advanced sections after advanced mode is enabled. The queue
now scrolls in normal page flow to keep bookmark targets visible.

No task engine, API, confirmation, save scope or result store changes. Tools
organization, contextual profile cleanup links, and real-job/restart acceptance
remain separate follow-ups. See the dated [progress ledger](implementation-progress.md)
for verification and fixture boundaries.

## Tools organization follow-up — 2026-10-23

E04 now groups the existing library diagnostics and similarity review in native
expandable sections, with GraphQL under Developer tools and the native Video
utility routes retained. Tasks and Tools share the same disclosure component.
Tools stays mounted while visiting other Settings sections so returning retains
its current inputs and results; leaving Settings still ends that temporary view.
No API, scan, copy confirmation or metadata mutation contract changes.

Contextual cleanup links from relevant profile/review pages remain the next
bounded E04 follow-up. The [progress ledger](implementation-progress.md) records
exact checks and fixture limitations; this does not close the broader real-data
or production-restart acceptance gaps.

## Contextual library review follow-up — 2026-10-23

E04 links the existing collision inspector and inherited-association review from
native Character, Artist and Copyright details. Character details also links the
existing duplicate review. Library-wide scope is explicit, and each link opens
in a new tab to retain the profile. A supplied profile name fills the inspector's
draft only; lookup, ambiguity handling and all changes remain explicit.

These links reuse the current tools rather than creating profile-specific scans
or merges. The [progress ledger](implementation-progress.md) records the exact
source/checks and limitations. The next pass should reassess E05's save-scope
feedback and settings links against the current code before adding behavior.

## Contextual Following setup follow-up — 2026-10-23

E06's first pass links Follow from saved Danbooru tag connections in native
Artist, Character and Copyright Sources & aliases. It opens the existing
Following form in a new tab, prefilled from a single stored provider identity;
no native-name guess, second form implementation or automatic follow/check is
added. Unsupported source/account rows state that feed checks are unavailable.

The form validates the current source by provider instance, allowing a renamed
settings entry while rejecting a reused source ID for a different site. Missing,
unsupported or failed connections leave saving blocked with an explicit blank
start. Exact existing follows link back to their saved definition. The form uses
native dark input/select styles, including focused and disabled controls.

Mounted desktop/mobile checks cover all three categories, original profile-tab
fixture state, exact raw targets, renamed/reused source IDs, duplicates, refresh,
source reload with drafts, missing/unsupported inputs and cancelled reads.
See the [progress ledger](implementation-progress.md) for source/checks and limits.
This does not add more feed adapters, remote account follows or browser layout
changes; broader Following action reuse remains a separate follow-up.

## Processing save feedback follow-up — 2026-10-23

E05's existing labels and independent Save actions already describe the different
owners. This bounded pass fixes the upscaler setup's disruptive save/reload path:
the form stays open, the normalized save response becomes the saved value, and
readiness checks do not reload paths or overwrite newer edits. Inline server-save
feedback clears when a path is edited. Readiness failures and rejected saves have
separate outcomes; only explicit Reload replaces a local path draft. Browser job
defaults keep their independent save action. The upscaling dialog now names the
correct Processing section in its setup guidance.

Desktop/mobile fixture checks cover slow readiness responses, edits while checking,
normalized responses, failed checks after successful saves, rejected saves and
explicit reload. See the [progress ledger](implementation-progress.md) for exact
checks and limitations. Broader E05 consistency and real-worker acceptance remain
open; this does not add a combined settings transaction or close the whole audit.

## Converter defaults recovery follow-up — 2026-10-23

E05's converter panel previously stayed disabled after its first settings read
failed, with no retry. The load error now appears at the start of the card with
**Retry loading defaults**. A pending retry has a labeled loading status and
cannot be submitted again. Repeated failure remains retryable; success restores
the saved controls and removes the retry action. Save errors remain separate.

Retry only reads converter defaults. It keeps the page and other Processing
drafts, and aborts its read when Settings unmounts. The existing browser fixture
checks these paths on desktop/mobile, including keyboard retry, delayed reads,
repeated failure, non-default saved values and tab/Back draft retention. Exact
verification and limitations are in the [progress ledger](implementation-progress.md).
This does not add a reload action over an already editable converter draft or
change save, worker capability, conversion or recovery contracts.

## Upscaling output guidance follow-up — 2026-10-23

E08's first pass starts the Upscale dialog with the existing new-copy versus
replacement choice and the corresponding original-file policy. New-copy guidance
now states that output files are saved beside the sources; replacement explains
the same Image entry, its separate restore cache and eviction risk. The second
replacement confirmation is retained.

Model, scale and format stay visible. Processor choice, worker diagnostics and
setup links move into a native disclosure whose children stay mounted during
readiness checks. Explicit CPU/GPU choices are visible in the closed summary,
and an incompatible CPU/model choice explains the blocked action. Setup opens in
a new tab to preserve the current draft. During checks, submission is disabled; a failed recheck clears stale capabilities until
a successful manual retry. No new processing mode, API, recovery store or backend
behavior is added.

Desktop/mobile fixture coverage checks the form order, hidden/retained advanced
controls, setup-link context, slow/failed readiness checks and one/multiple Image
copy and confirmed-replacement request routing. See the exact verification in the
[progress ledger](implementation-progress.md).

E08's converter pass keeps output format and savings minimums before the launch
actions. A native disclosure groups encoding and worker controls, retaining their
mounted state and displaying explicit processor choices/custom overrides in its
summary. Resolved per-input plans, worker failures and fallback notices stay
visible; batch plans use wrapping text instead of low-contrast read-only fields.
The settings link opens a separate tab without dismissing the draft.
The initial dialog and review explain that estimates delete sample outputs,
trials retain temporary outputs, and explicit Apply replaces active files beside
the sources while retaining the same library entries and metadata. Original-file
recovery still depends on the converter restore cache and its eviction policy.
Estimate and apply verification details remain available in disclosures. No new
backend mode, cache or apply bypass is introduced. Desktop/mobile fixture checks
cover draft retention, failed worker checks, savings validation, typed trial
requests and explicit selection/apply alongside the existing comparisons and
maintenance errors.

E08's mask-restoration pass labels prepare, paint/describe and review/save in the
existing dialog. It states the new adjacent PNG/native Image result and unchanged
source before preparation and beside Save as new Image. Prompt/mask controls stay
visible; a native generation-options disclosure retains hardware, seed, reference
and other inference choices. Its summary shows explicit CPU/GPU and active
reference state; toggling it does not invalidate the preview. Editing generation
settings still blocks saving until a new preview. Existing comparison, optional
stacking, cancellation and unsaved-session cleanup remain in place. Closing now
explains the loss of unsaved work. It introduces no replacement mode or shared
restore cache. All three E08 tool presentations now have bounded passes; broader
real-file/model acceptance and any shared history design remain separate.

## Progress measures

For each pass, record before/after routes visited, dialogs opened and manual
identifier entry for the affected task. Target zero page changes for one source
connection and one consistent way to reach a job's result/error. Require refresh,
Back and cancellation checks alongside the happy path. Count preserved safety
contracts and unresolved acceptance gaps as explicitly as reduced clicks.

The first implementation objective is E01's contextual connection flow. The
remaining findings are a bounded backlog, not authorization to refactor the whole
application in one change.
