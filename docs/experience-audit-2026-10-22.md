# Experience audit and consolidation plan

Reviewed 2026-10-22 against `develop` at
`b36daf9f0d548df25facc73fc98857764c4b61ae` (including merged PR #171).

The project has useful capabilities, but everyday tasks expose too many of their
implementation boundaries. The next work should reduce context switches and make
existing behavior predictable before extending the feature list. This is a
whole-project assessment, not a proposal to rebuild Stash or discard its safety
contracts. Booru UI implementation remains with the agent already assigned to it.

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
| E07 / next | All, Images, Videos and related media lists share native infrastructure, while remote browsing and Following have distinct lists/actions. Existing viewer regressions required caller ownership and queue restoration fixes. See [MediaList](../ui/v2.5/src/components/Media/MediaList.tsx) and [viewer acceptance notes](p07-global-media.md). | Inherited + new / confirmed history; current inconsistency needs validation | Adopt common interaction rules for selection, Back, refresh, Escape and preview focus. Keep local structured filters separate from provider-native query grammar. Do not force one query parser or identical capability menus everywhere. |
| E08 / next | Conversion, upscaling and restoration intentionally have different output and recovery contracts. Their controls can expose technical cache/worker concepts before the user understands the result. See [processing comparison](features.md#conversion-upscaling-and-restoration), [conversion store](../pkg/mediaconvert/store.go) and [upscaling settings](../ui/v2.5/src/components/Settings/MediaUpscalingSettings.tsx). | New / confirmed contracts; presentation needs validation | Start with output choice, destination and original-file policy; reveal advanced codec/model controls as needed. Share a History & restore presentation only after mapping each tool's valid operations. Preserve staging, validation, restore caches and backups. |
| E09 / later | Native React is supplemented by composed built-ins; some use supported PluginApi patches, while others move DOM nodes with selectors and MutationObserver. See [loader](../ui/v2.5/src/index.tsx), [file-info adaptation](../ui/v2.5/builtin-source/universal-file-info.js) and [extra presentation changes](../ui/v2.5/builtin-source/bundle-extra.js). | Earlier fork + new / architectural risk | Move a touched DOM adaptation into its owned native component when fixing that workflow. Keep supported plugin extension points. Do not rewrite the whole built-in bundle or mistake native All for the retired synthetic implementation. |
| E10 / now, docs | README and docs index still call follows planned. The browser roadmap still describes feature-branch delivery and says provider mappings do not exist, despite merged Following and identities. | Documentation / confirmed | Correct current status and link operation guides. Keep dated implementation history as history rather than asking users to interpret the long progress ledger. |
| E11 / validate | Setup collects paths and the home page shows configurable recommendations; source/model setup and first-use discovery are separate flows. See [Setup](../ui/v2.5/src/components/Setup/Setup.tsx) and [FrontPage](../ui/v2.5/src/components/FrontPage/FrontPage.tsx). | Inherited foundation + new requirements / validation needed | Walk a fresh user through first scan, first useful result and first metadata edit. Add empty-state links only where the next step is missing; do not introduce another setup wizard before observing the gap. |
| E12 / validate | Search, relationship editing, Galleries/Collections, playback, mobile density, plugins and advanced settings have substantial inherited behavior. The new-feature tests alone cannot establish their usability or compatibility. | Whole project / validation needed | Use the cross-feature acceptance matrix below with an isolated library, then owner testing with representative data. Fix reproduced regressions before speculative layout changes. |
| E13 / next | The inherited settings navigation stacks above the content on mobile; Processing starts roughly 540 px down at 390 × 844. See [Settings layout](../ui/v2.5/src/components/Settings/Settings.tsx). | Inherited layout + expanded sections / runtime confirmed | Use a compact mobile section chooser or Jump to settings content, retaining URL tabs, anchors and keyboard navigation. |
| E14 / next | Visible terminology mixes Artists with Subsidiary Studios, and Videos with Scene covers / Scene Tools. First-use release notes lead with upstream v0.29.0. See [release-note selection](../ui/v2.5/src/App.tsx) and [inherited notes](../ui/v2.5/src/docs/en/ReleaseNotes/index.ts). | Inherited UI + fork vocabulary / runtime confirmed | Audit visible labels and localization consistently; provide concise fork-aware first-use guidance while retaining upstream history. Preserve internal Studio/Scene identifiers and compatibility APIs. |

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

## Progress measures

For each pass, record before/after routes visited, dialogs opened and manual
identifier entry for the affected task. Target zero page changes for one source
connection and one consistent way to reach a job's result/error. Require refresh,
Back and cancellation checks alongside the happy path. Count preserved safety
contracts and unresolved acceptance gaps as explicitly as reduced clicks.

The first implementation objective is E01's contextual connection flow. The
remaining findings are a bounded backlog, not authorization to refactor the whole
application in one change.
