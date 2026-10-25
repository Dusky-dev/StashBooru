# StashBooru feature guide

This guide covers the fork's user-facing additions to upstream Stash, including
features predating P01–P13. See the [documentation index](README.md) for detailed
setup and acceptance notes. It describes current `develop`; individual stable
releases can lag behind it. The [booru browser roadmap](booru-browser-roadmap.md)
separates delivered browsing, review, download, identity and first-feed slices
from later metadata selection and additional provider adapters.

## First use and release notes

Welcome and the release-note dialog start with a short StashBooru introduction:
library setup, All, source connections, optional processing and backups. Finish
still opens Library settings and records the latest note as seen. Existing users
see the new dated introduction when it is newer than their saved acknowledgement;
closing the note keeps the existing server-owned `lastNoteSeen` behavior.

Revisit the introduction in **Settings → Changelog**, above the preserved upstream
version history. The shared introduction is English, like the existing release
notes; other UI translations and custom locale overrides retain their behavior.
Fork issues link to the StashBooru repository; upstream help/support links remain.

## Where things live

| Location | Use it for |
| --- | --- |
| **All** | Browse Images and Videos together, with shared filters and typed selections. Individual Images and Videos pages remain available. |
| **Booru** (navbar globe) | Browse sources, preserve page/draft state, use live/CSV autocomplete and sidebar filters, inspect previews/tags, keep review marks and download a post into Stash with a default/custom folder and optional converter defaults. |
| **Following** | Follow a saved Danbooru tag connection from a profile’s **Sources & aliases**, or manage saved follows and explicitly check new posts. Other feed adapters remain planned. |
| **Identities** / profile **Sources & aliases** | Review connections between exact provider identifiers and existing native Artists, Characters and Copyrights. |
| **Settings → Library** | Media folders, scan rules and automatic ancestor/profile associations. Inheritance switches save automatically. |
| **Settings → Processing** | Shared worker connection, similarity/tagging defaults, conversion rules and upscaling setup. Use each section's Save button. |
| **Settings → Tasks** | Jobs, generated files/hashes, animation inspection, Character merging, association review, Video overlap indexing and database management. |
| **Settings → Tools** | Metadata Health, Alias Collision Inspector and EVA02 Visual Similarity Triage, including explicit add-only metadata copying. |
| **Settings → System** | Application paths, FFmpeg, transcodes and other server settings. |

On phones, use **Settings section** above the controls to change sections. On
larger screens the section list stays beside the controls. Both use the same
section URLs; Back, refresh and existing section bookmarks keep their destination.
Advanced mode and troubleshooting remain available beside the navigation.

Tasks starts with the job queue, followed by **Update library**, **Review profiles
& associations**, **Inspect media**, **Backup & maintenance** and **Plugin tasks**.
Open the group for the work you need. Closing it keeps its current draft and
review results; jobs continue normally. Existing task bookmarks open their group
automatically. The queue scrolls with the page so it does not cover a bookmarked
action. Save scopes and confirmation steps remain with each tool.

Tools groups **Metadata Health**, **Visual Similarity Triage** and **Alias
Collision Inspector** into expandable reviews. **Developer tools** contains the
GraphQL playground; **Video tools** keeps the filename parser and duplicate
checker. Opening a group does not start a scan or copy metadata. Inputs, results
and selections remain when a group is closed or another Settings section is
visited and you return. Leaving Settings or refreshing starts a new review;
these temporary results are not durable history. Metadata copy still requires
explicit selection and confirmation.

Expanded Character, Artist and Copyright profiles include **Library review**.
**Inspect this name** prefills the name collision inspector; press **Inspect
value** to check matches explicitly. Association review and the Character merge
review are labeled **whole library**. These links open the existing tools in a
new tab, keeping the profile page in place. Opening a link does not run a scan,
merge profiles, apply defaults or change metadata.

Processing defaults apply to new jobs. Conversion/tagging defaults and model paths
are saved on the server. Upscaling job defaults are saved in the current browser;
their Save button does not change server paths. Old System bookmarks for moved
sections redirect to the new location.

## Image editing

Open an Image's detail page and use **Edit → Save** to update its metadata.
You can save again in the same editor. **Custom fields** expands the native
field editor; on narrow screens the panel grows with the form so its lower
controls stay reachable above the bottom navigation. Its values are saved
alongside the Image's metadata, while Artist and Copyright assignments use
their existing separate writes. A rejected Image
update leaves its draft available for correction and retry. Separate association
writes are not a combined transaction.

The isolated `image-editor-save.mjs` browser suite scans its own generated PNG
and checks repeated saves, a custom-field edit, a failed update and retained-draft
retry, Back, refresh and server restart at desktop/mobile sizes. It verifies
native relationship/file IDs and unchanged PNG bytes, including a user custom
field named `full`. Its delayed association requests and rejected fixture ID use
real server responses. It accepts no existing library/server URL. Run it using
`STASH_BROWSER_BINARY`, optional `STASH_BROWSER_UI`, `PLAYWRIGHT_MODULE`,
`CHROMIUM_EXECUTABLE` and `STASH_BROWSER_SCREENSHOTS`, as for the other native
browser suites. This covers the existing Image editor; it does not establish
all editors, bulk saves, provider scrapes or association-write rollback.

## Browse a mixed library

The navbar's **All** page uses native Image/Video filters, ordering, selections,
cards and players. Equal numeric Image and Video IDs remain different selections.
Images/Videos pages and entity-related media remain available. Grid, List, Wall
and Tagger displays appear where supported; view/zoom and media-tab preferences
are remembered per view. Primary file format is available for sorting Images and
Videos and in bulk-edit filters. File properties appear in Details.

On Image and Video detail pages, the Booru metadata entity cards display
Video and Image totals for their associated Artists and Copyrights, including
Copyright ancestors in the hierarchy. A genuine zero remains visible; a missing
count is not presented as zero.

Character cards use portrait artwork, Artists square artwork and Copyrights
landscape artwork. Character Copyrights and Variants appear together. Native
Video previews own the active player, stop on dismissal and advance through
explicit navigation rather than cycling when playback ends. See [All page notes](p07-global-media.md).

## Find similar and compare

Image-card **Find similar** starts with **Same image / variants (pHash)**. Generate
Image perceptual hashes in Tasks first. Lower distance narrows the matches. A zero
distance is not proof of equal bytes, and crops/rotations can be missed.

Switch **Match by** to **Related content (EVA02)** for similar subjects or
composition. Install its optional model explicitly and generate visual embeddings
in Processing settings. This ranking uses image content, not catalogue Tags. Model
inference can run remotely; the index remains in StashBooru.

The shared Image comparison offers both images, slider, blink and a difference
heatmap with highlighted regions. It compares bounded previews; it is not a
byte-equality check or whole-animation verification. Automatic geometric alignment
and Video-frame comparison are not currently included.

**Settings → Tools → Visual Similarity Triage** uses EVA02 clusters, not pHash
groups. It suggests a preferred Image and can explicitly add missing metadata to
selected other members. It does not merge/delete Images; a connected cluster is
not proof that every pair is a duplicate. The planned browser's pHash grouping
and red disfavored-tag comparison are [separate work](booru-browser-roadmap.md#b05--phash-groups-compare-and-tag-warnings).

## Names in the interface

The English interface uses **Artists**, **Characters** and **Videos**. Artist
hierarchy fields use **Parent Artist** and **Child Artists**; they stay in the
native profile editor rather than the built-in Advanced group. Video asset and
view labels follow the same vocabulary.

This pass updates the English fallback catalogue, which also supplies missing
US English messages. Other language translations and custom locale overrides
retain their existing behavior. Native Studio/Performer/Scene IDs, URLs, message
IDs and API fields remain compatible; these display names do not rename data.

## Stacks and metadata

Stacks group ordered Image/Video variants while each member keeps its own file,
metadata and ID. Choose a representative from the filmstrip, or compare an Image
with its Image representative. Drag or use up/down buttons in **Manage stack**;
the draft order changes only after **Save stack**. Deleting a stack does not delete
its media. Proposals require review; a suggested relationship is not proof that
either member is safe to delete.

Copyrights, Tags, Artists and Character variants retain native hierarchy links.
Library inheritance switches control effective ancestor/profile associations.
Removing a child does not remove an explicitly assigned parent. Use the Tasks
association preview to inspect the effects of changing shared defaults.
The Copyright list defaults to **Hierarchy** sorting, which keeps each Main
Copyright above its Sub-Copyrights and respects saved sibling order. Name,
Sort Name, count and date sorts remain available when a flat list is wanted.

Copyrights have their own aliases, artwork, favorites and media/Character links;
they remain separate from ordinary Tags. Multiple Artists can be attached to an
Image/Video. Character variants keep their own profiles, and disambiguation can
link to a native Copyright or Artist. The hierarchy's Main/Sub tabs, branch paths
and descendant filters keep these identities separate. Artist context links do
not imply authorship.

**Review metadata…** is available on Image/Video details and selected items in
All, Images and Videos. Mixed selections retain typed native IDs and are reviewed
one item at a time, with Skip and selection progress. The Image review dialog
shows a larger, full-height image preview beside independently scrollable controls
on desktop, with a stacked layout on narrow screens. Tag hover cards retain their
details and media counts without displaying Tag artwork. Images review
**Local → Booru → Camie → EVA02 → Library** sources; Videos review
**Local → Booru → Frames**. Only Local loads on opening; network
lookups, inference and library suggestions are explicit. Select predictions and
resolve ambiguous targets before applying the change plan. Filename identity
metadata takes precedence in populated categories. Camie and EVA02 have independent
thresholds; these do not tune Find similar distance. Bulk Image model tagging is
in Generate; native **Auto Tag (paths and filenames)** matches names/paths without
a model. Videos' separate **Batch tagging job…** retains its advanced job options.

See [metadata and tagging](metadata-and-tagging.md) for filename tokens, provider
lookup behavior, model setup, direct/calculated relationships and ambiguity rules.
Existing MD5 lookup enriches local media. The separate [Booru browser](booru-browser.md)
supports multi-source search through Danbooru, Gelbooru, Moebooru, e621
and Philomena adapters, with ten presets and compatible custom definitions.
**Booru → Settings** (also **Settings → Metadata Providers → Booru sources**) edits
API keys, engine, URL, image hosts and enabled state. Its Danbooru-inspired sidebar
and compact thumbnail grid include categorized loaded-page tag counts.
[B02 review state](booru-review-state.md) adds persistent ignored/manual-download
marks, bulk undo, a show-ignored filter and bounded current-file MD5 matches.
[Downloads and live/CSV autocomplete](booru-downloads-autocomplete.md) support native
Stash import into a default/custom folder, optional converter defaults, upload or
server CSV paths, live tag suggestions with CSV fallback, card downloads, search chips,
sort/rating icon menus, loaded-page media filters and refresh persistence. Native dark
inputs/switches, card selection/counters and page-size controls match the rest of Stash.
Inspection supports keyboard navigation/download/ignore and explicit bounded video/GIF
playback. Follows, metadata entity review and bounded batch import remain later packages.

### Tag creation and saves

On **Add Tag**, **Save & New** validates the form before sending a request and
uses Save's existing metadata formatting. A missing name shows the native field
error. A duplicate name is rejected by the server while retaining the draft;
correct the name and retry. Success resets metadata, parent/child selections and
artwork for the next Tag. Ordinary Save opens the saved Tag. Multiple parents and
existing hierarchy-cycle validation retain their native behavior.

The isolated `tag-save-new.mjs` suite checks validation, duplicate create/retry,
clean next creation, multiple parents, real hierarchy-cycle rejection/correction,
Cancel, Back and refresh/restart at desktop/mobile sizes. It also verifies that a
same-name native Copyright and a metadata-only Video keep their own IDs and
explicit Tag links. All server/database/artwork fixtures are owned by the suite;
responses are never intercepted or mocked. Run it with `STASH_BROWSER_BINARY`,
optional `STASH_BROWSER_UI`, `PLAYWRIGHT_MODULE`, `CHROMIUM_EXECUTABLE` and
`STASH_BROWSER_SCREENSHOTS`. Native Copyright editing and broader bulk/real-library
acceptance remain separate work.

### Artist creation and saves

On **Add Artist**, **Save & New** validates the form before sending a request,
using Save's existing metadata formatting. A missing name shows the native field
error. If a name is already used, the server rejects it and retains the draft;
correct the name and retry. Success resets the form, including parent/child
Artists, Tags and artwork, for a distinct next entry. Ordinary Save opens the
saved Artist. Existing parent/child hierarchy validation still rejects cycles.

The isolated `artist-save-new.mjs` suite checks blank-name validation, duplicate
create/draft retry, clean next creation, an actual rejected descendant-parent
cycle and correction, Cancel, Back, refresh/restart and saved metadata/artwork.
It owns its database, server and local artwork; responses are never intercepted
or mocked. Run it at desktop/mobile sizes using `STASH_BROWSER_BINARY`, optional
`STASH_BROWSER_UI`, `PLAYWRIGHT_MODULE`, `CHROMIUM_EXECUTABLE` and
`STASH_BROWSER_SCREENSHOTS`. Broader bulk editing, scraping and representative
libraries remain separate acceptance work.

### Copyright creation and saves

Copyrights use their own **New Copyright** and **Edit** forms. A blank or
whitespace-only name disables Save. Duplicate names and hierarchy cycles are
rejected by the server; the form keeps the draft for correction and retry.
Main/Sub-Copyrights, aliases and landscape artwork remain native Copyright data,
separate from ordinary Tags.

Metadata and Character links are saved in separate requests. If a selected
Character was deleted elsewhere, the metadata may already have saved when the
link request fails. An error message explains that partial save. Remove the stale
Character selection and retry: the form
updates the same Copyright ID and retains the other draft values. These requests
are not a combined transaction. Keep the form open to correct the failed save;
refreshing discards its unsaved draft.

The isolated `copyright-save-retry.mjs` suite uses real duplicate-name,
deleted-Character and hierarchy-cycle failures. It checks creation and editing,
same-ID retry, metadata/relationship/artwork preservation, Cancel, list Back,
refresh and restart on desktop/mobile. It owns its server, database and landscape
artwork; it neither intercepts responses nor accepts an existing-library URL.
Use `STASH_BROWSER_BINARY`, optional `STASH_BROWSER_UI`, `PLAYWRIGHT_MODULE`,
`CHROMIUM_EXECUTABLE` and `STASH_BROWSER_SCREENSHOTS` as for the other native
browser suites. Network-response loss, bulk editing and representative libraries
remain separate acceptance work.

### Profile artwork and metadata diagnostics

**Set image → From gallery…** offers a context-filtered, paged Image picker and
crop/rotate/pan/zoom editor for profile artwork. Applying artwork keeps the source
media unchanged. Supported entity image routes can fall back to a related Image
when explicit artwork is absent.

**Settings → Tools → Metadata Health** reports duplicate/same-name identities,
legacy Copyright Tags, Artist-link drift, orphaned links and ambiguous migration
candidates. **Alias Collision Inspector** shows conflicting canonical/alias
matches. Both diagnose without automatically repairing or merging records.

### Character creation and saves

On **Create Character**, the Save menu's **Save & New** validates the form before
sending a request, using the same metadata formatting as Save. A missing name
shows the native field error. A rejected create retains the selected Copyright
or Artist context with the rest of the draft for retry. After a successful
**Save & New**, the new-Character form resets for the next entry. Ordinary Save
opens the saved profile. Existing-Character saves and Cancel keep their native
behavior; separate variant/Copyright writes remain separate transactions.

The isolated `character-save-new.mjs` browser suite uses owned servers and local
portrait/metadata fixtures. It checks validation, rejected create and update,
retained-draft retries, clean subsequent creation, Cancel, Back, refresh/restart
and native context/Tag/custom-field identity at desktop/mobile sizes. Run it with
`STASH_BROWSER_BINARY`, optional `STASH_BROWSER_UI`, `PLAYWRIGHT_MODULE`,
`CHROMIUM_EXECUTABLE` and `STASH_BROWSER_SCREENSHOTS`, as for the other native
browser suites. Broader bulk editing, scraping and association-write rollback
remain separate acceptance work.

### Character cleanup and Copyright auto-tagging

In **Settings → Tasks → Merge duplicate Characters**, preview matching full names,
reversed first/last names and unique short names such as “Miku” / “Hatsune Miku”.
Both entries must have identical directly assigned native Copyrights. Missing
context, ambiguous short names, variants and conflicting profiles are skipped with
reasons. Review the surviving names and Copyrights, then apply eligible groups as
a cancellable job. Media links and compatible profile values are retained; removed
names become Copyright-qualified aliases. See [matching rules and limits](character-dedup.md).

In Tasks' **Advanced mode → Auto Tag**, enable **Copyrights** to match Copyright
names/aliases against Image and Video paths. **Selective Auto Tag** limits the
paths searched. This creates native Copyright links; Galleries have no Copyright
relationship. Existing saved task options keep Copyrights off until enabled.

## Conversion, upscaling and restoration

| Tool | What it does | What happens to the original |
| --- | --- | --- |
| Media converter | Changes file format/encoding; offers estimates, verified trials and reviewed apply. | An estimate/trial leaves the source intact. Applying a replacement uses the converter's original-file restore cache. |
| Image upscaling | Enlarges Images with an installed waifu2x or SeedVR2 model. | New-copy mode keeps the source and creates a new Image with its metadata. Explicit replacement uses a separate upscaling restore cache. |
| Mask restoration | Generates new pixels in selected regions of a non-explicit still Image, with mask/preview review. | Saves a new derivative and preserves the source. Generated content is not recovery of hidden original pixels. |

The converter starts with output format and savings minimums. Expand **Encoding
and worker options** for temporary codec/processor overrides. Estimates measure
samples; verified trials retain outputs for comparison; **Review trials & apply**
activates only the eligible trials you select. Its settings link opens a new tab
so the current draft stays available.

Mask restoration shows prepare, paint and review/save stages. Keep the prompt
and mask tools visible; expand **Generation options** for inference controls and
an optional starting-pixel reference. **Save as new Image** creates a PNG beside
the source and copies library metadata. Closing discards unsaved work. It keeps
the original and does not use conversion/upscaling replacement caches.

Both enabled conversion savings minimums must pass. A saved trial is temporary and
is revalidated before apply; changed sources/settings need a new trial. Trial space
and original-file restore space are different. Restore caches can evict old
originals, so they do not replace backups. Upscaling may intentionally produce
larger files and does not use compression-only savings thresholds.

**Automatic** prefers the configured remote worker and uses the supported local
fallback. **This server only** and **Remote worker only** keep that choice. Each
tool still requires the corresponding worker capability and installed codec/model.
GPU-only processing must have GPU support; local model paths do not configure a
remote machine. No large model download starts silently.

The **Upscale images** dialog starts with the choice to create new Images or
replace active files. New files are saved beside their sources. Replacement
requires confirmation and uses its separate restore cache. Model, scale and
format stay visible; **Worker and processing options** reveals processor/setup
controls. Setup opens in a new tab to retain the current job draft.

In **Settings → Processing → Image upscaling**, job defaults are saved in this
browser; local executable/model paths are saved on the server. Saving paths keeps
the setup open and confirms the save separately from worker readiness. A failed
readiness check does not undo the save. **Reload worker setup** explicitly replaces
unsaved paths with the saved server configuration; it does not save browser job
defaults.

If the converter's saved defaults cannot load, use **Retry loading defaults** in
its error message. The retry reads converter settings without refreshing the page
or resetting other Processing drafts. Conversion controls remain disabled until
the saved defaults load; the other sections keep their own Save actions.

See [converter setup and restore](media-converter.md),
[conversion trials](p09-conversion-review.md), and
[restoration setup](p11-image-restoration.md) for operational detail.

## Jobs and Video overlap

Failed jobs stay in Tasks until dismissed and have **Copy error**. This history
survives navigation/refresh in the same browser tab. It ends when that tab closes
and does not recover jobs that finished while the app was closed/disconnected.
Successful/cancelled jobs disappear after ten seconds.

**Video overlap review** in Tasks indexes timestamped frames and reviews duplicate,
contained, partial and compilation matches. Indexing is incremental and resumes
after cancellation/restart. Inspect both matched timelines and unmatched content;
partial overlap is not a reason to automatically delete an entire Video.

## Animation detection and timing

Scanning inspects image frame counts and adds the native `animated` Tag for
multi-frame images without removing existing metadata. Native cards/walls display
animation-format badges beyond GIF, including APNG, AJXL and animated WebP/AVIF.
Missing inspectors leave animation status unknown. Use **Settings → Tasks →
Animation inspection → Inspect existing images** for older records; JXL inspection
needs libjxl tools on the catalogue server even with remote conversion.

Animated Image headers show average FPS across the whole timeline, including long
holds. Conversion verifies timing/frame count separately. Rescan with file
rescanning enabled to refresh missing/stale timing. The converter documents
zero-delay normalization and supported animation/color/audio checks in
[animation timing](media-converter.md#animated-image-frame-rate).

## Local HTTPS

Normal HTTP installations also attempt a secondary HTTPS listener, enabled by
default on port **9443**, using the same application handler/authentication. An
existing explicit TLS configuration keeps its single-listener behavior. Failure
to start the extra listener is logged and leaves normal HTTP available.

Configuration keys in `config.yml`:

```yaml
local_https:
  enabled: true
  port: 9443
  hosts: ["stash.local", "192.168.1.50"]
```

Without custom paths, the server keeps a self-signed `stash-local.crt` and
`stash-local.key` beside its configuration. Clients must trust the certificate to
remove browser warnings. Set additional LAN names/IPs explicitly; Docker bridge
networking cannot discover the host's LAN address. Publish port 9443 separately
when using that listener in a container. Custom `cert_path` and `key_path` must be
supplied together. Set `enabled: false` to disable the secondary listener. This
does not change the separately configured processing worker's HTTP/TLS setup.

## Plugins and task failures

Settings → Plugins manages local plugin settings and enabled state. Settings →
Tasks → Plugin tasks shows tasks from enabled plugins. The queued confirmation
means a task was accepted; check the job queue for its final result. A plugin's
reported error or script exception now produces a **Failed** job with its error
details, rather than being labelled Completed. Existing successful, no-result
and cancellation behavior is retained.

Failed-job notices stay in the current browser tab until dismissed and survive
refresh there; they are not permanent server history. The existing Copy error
and Dismiss controls remain the way to inspect and clear them. Enabling or
disabling a UI plugin offers Reload UI to apply its scripts/styles. Troubleshooting
mode's Disable customizations skips UI customizations; it does not disable the
plugin's server-side tasks.

The isolated suite installs only its own local JS/UI fixtures in disposable
libraries. It exercises dependency order, plugin CSS/React routes, native Video
navigation/Back, settings persistence, plugin-cache reload, reported and thrown
task failures, success, running-task cancellation/retry, retained failure details,
UI customization bypass, disable/re-enable and server restart. Native Video
metadata stays unchanged; a successful fixture task creates its own Tag receipt.

```sh
STASH_BROWSER_BINARY=/absolute/path/to/stash-linux \
PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs \
node ui/v2.5/tests/browser/plugin-compatibility.mjs
```

Use Node 20 and Playwright 1.57 Chromium. Optional `CHROMIUM_EXECUTABLE`,
`STASH_BROWSER_UI` and `STASH_BROWSER_SCREENSHOTS` select the browser, a production
UI build and output directory. This suite owns all paths/processes and cannot
use an existing library. It does not install third-party packages or establish
compatibility with arbitrary community plugins, external runtimes/services,
real media/processing workflows or physical devices.

## Experimental storage and backups

Image delta storage and Gallery block sharing currently run as optional offline
prototypes. They benchmark, pack, verify and extract while retaining originals;
they are not active transparent application storage. Byte-exact archives restore
original file bytes. Pixel-exact archives preserve the documented decoded-pixel
contract, not an identical original compressed file. Real-library savings vary.

See [image delta storage](p12-image-delta-storage.md) and
[Gallery sharing](p13-gallery-block-storage.md) for commands and production gates.
Actual restoration model/GPU acceptance also remains separate from automated
fixture tests.

StashBooru database backups are incompatible with vanilla Stash. Keep media and
required sidecar/packed payloads in your backup plan; a database-only backup does
not contain those bytes.

The warning accompanies the backup section and saved/downloaded database backup
dialog. The current backup action documents whether blobs are included; it
does not include media files, generated assets, original-file restore caches or
offline packed-storage payloads. Keep the appropriate data directories separately.

The backup dialog stays open while the server writes the backup. Its destination,
blob choice and action buttons are locked until the request finishes. A rejected
backup shows its error inside the dialog and keeps the choices for retry;
only a successful request closes it. Other Tasks sections remain mounted.

Database blob storage is included in a SQLite backup automatically. With
filesystem blob storage, including blobs produces a ZIP containing the database
and its `blobs/` assets. A SQLite-only backup of filesystem storage needs those
blobs preserved separately. Restore into a stopped instance with the appropriate
configuration and separately backed-up data directories; there is no new restore
wizard or compatibility detector in this change.

The isolated acceptance suite creates its own servers and temporary databases:

```sh
STASH_BROWSER_BINARY=/absolute/path/to/stash-linux \
PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs \
node ui/v2.5/tests/browser/database-backups.mjs
```

It requires Python 3's standard library and Playwright 1.57 Chromium. Optional
`CHROMIUM_EXECUTABLE`, `STASH_BROWSER_UI` and `STASH_BROWSER_SCREENSHOTS` select
the browser, a production UI build and output directory. It cannot target an
existing library. Desktop/mobile runs cover database and filesystem blob storage,
a real unavailable-directory failure, pending controls and retry, other Tasks
draft retention, directory saves, browser downloads, SQLite integrity/foreign-key
checks and restoration into new
instances. Metadata-only Video relationships and an Artist image are synthetic;
this does not establish cross-version upgrades, owner-library recovery, file
conversion recovery, live plugins, codecs or physical-device behavior.

P01's additional product-manifest/restore-preflight work was deferred by the owner;
the warning is not a guarantee of automated compatibility detection. Restore
caches are bounded and can evict originals.

### Database upgrade backup paths

The migration page accepts a filename in the configured backup folder or a path
to another destination. An absolute path, or a relative path containing a parent
directory, validates its own parent. An unavailable default backup folder no
longer blocks a usable explicit path. A bare filename continues to use the
configured backup folder. Failed preflight keeps the page and typed path so you
can correct it and retry.

An empty value keeps the existing temporary rollback-backup behavior: the backup
is deleted after successful migration. A named backup retains the earlier schema
for its matching earlier application. Migration backups are SQLite-only;
filesystem blobs, configuration, media and processing caches need separate
preservation. StashBooru backups still require a compatible StashBooru version.

The isolated upgrade suite uses a real schema-99 application from fork commit
`aa1a2b42bf9bfb6b07608350bfbda688e54e91b9`. CI generates/builds that backend
with an API-only placeholder UI; the current Linux artifact supplies the upgrade
interface. It never changes a current database's version label to simulate an
older database.

```sh
STASH_BROWSER_BINARY=/absolute/path/to/current-stash-linux \
STASH_UPGRADE_FROM_BINARY=/absolute/path/to/stash-schema99 \
PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs \
node ui/v2.5/tests/browser/database-upgrade.mjs
```

Use the Python 3, Playwright and optional browser/UI/output settings described for
the backup suite above. It owns all paths/processes and cannot target an existing
library. It checks schema 99 → 100 at desktop/mobile sizes: rejected backup
preflight and retained path, custom-path retry despite an unavailable default,
upgrade, integrity/foreign keys, an original-schema backup reopened by the
historical app, native aliases/relationships/custom fields and database blob
bytes, refresh and restart. This is one pinned upgrade path, not acceptance of
all older versions, filesystem-blob upgrade recovery, interrupted SQL migration,
large owner libraries, plugins, codecs or processing recovery.

## Versions and updates

Settings → About shows **StashBooru 1.0.0** separately from the upstream Stash
build version and commit. Opening About checks GitHub; Check for updates retries.
Startup also logs a stable-release check. Only stable `stashbooru-vX.Y.Z` releases
from this fork are considered. An equal or older release is not an update.
Development snapshots (`latest_develop`) are available from GitHub separately.

Download the matching platform release and follow your existing installation
procedure. The check does not install updates. A network/rate-limit failure is
shown as a failed check, not as proof that the installation is up to date.

### Job feedback and retained results

Tasks' native queue and processing feedback use readable status labels. Converter
feedback distinguishes Completed with errors, provides typed links to failed media
and copies all returned errors even when only 50 rows are displayed. Queue links
open in a new tab; upscaling acknowledges submission and links to its live queue.
See [job feedback and recovery](job-feedback.md) for the distinct lifetimes of
queue notices, processing details, saved receipts and original-file records.

Live converter/upscaling-restore job references include the batch identity and are
scoped to the server API address. Refresh can resume the same retained job; a
reused numeric ID after restart cannot display or cancel unrelated work. Expired
live references leave saved history and available original recovery accessible.
