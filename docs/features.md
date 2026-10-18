# StashBooru feature guide

This guide covers the fork's user-facing additions to upstream Stash, including
features predating P01–P13. See the [documentation index](README.md) for detailed
setup and acceptance notes. It describes current `develop`; individual stable
releases can lag behind it. The [booru browser roadmap](booru-browser-roadmap.md)
separates delivered browsing/review/import slices from later metadata and follow work.

## Where things live

| Location | Use it for |
| --- | --- |
| **All** | Browse Images and Videos together, with shared filters and typed selections. Individual Images and Videos pages remain available. |
| **Booru** (navbar globe) | Browse sources, preserve page/draft state, use live/CSV autocomplete and sidebar filters, inspect previews/tags, keep review marks and download a post into Stash with a default/custom folder and optional converter defaults. |
| **Settings → Library** | Media folders, scan rules and automatic ancestor/profile associations. Inheritance switches save automatically. |
| **Settings → Processing** | Shared worker connection, similarity/tagging defaults, conversion rules and upscaling setup. Use each section's Save button. |
| **Settings → Tasks** | Jobs, generated files/hashes, animation inspection, Character merging, association review, Video overlap indexing and database management. |
| **Settings → Tools** | Metadata Health, Alias Collision Inspector and EVA02 Visual Similarity Triage, including explicit add-only metadata copying. |
| **Settings → System** | Application paths, FFmpeg, transcodes and other server settings. |

Processing defaults apply to new jobs. Conversion/tagging defaults and model paths
are saved on the server. Upscaling job defaults are saved in the current browser;
their Save button does not change server paths. Old System bookmarks for moved
sections redirect to the new location.

## Browse a mixed library

The navbar's **All** page uses native Image/Video filters, ordering, selections,
cards and players. Equal numeric Image and Video IDs remain different selections.
Images/Videos pages and entity-related media remain available. Grid, List, Wall
and Tagger displays appear where supported; view/zoom and media-tab preferences
are remembered per view. Primary file format is available for sorting Images and
Videos and in bulk-edit filters. File properties appear in Details.

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

Image Tagging reviews **Local → Booru → Camie → EVA02 → Library** sources. Video
Tagging reviews **Local → Booru → Frames**. Only Local loads on opening; network
lookups, inference and library suggestions are explicit. Select predictions and
resolve ambiguous targets before applying the change plan. Filename identity
metadata takes precedence in populated categories. Camie and EVA02 have independent
thresholds; these do not tune Find similar distance. Bulk Image model tagging is
in Generate; native Auto Tag matches names/paths without a model.

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

### Profile artwork and metadata diagnostics

**Set image → From gallery…** offers a context-filtered, paged Image picker and
crop/rotate/pan/zoom editor for profile artwork. Applying artwork keeps the source
media unchanged. Supported entity image routes can fall back to a related Image
when explicit artwork is absent.

**Settings → Tools → Metadata Health** reports duplicate/same-name identities,
legacy Copyright Tags, Artist-link drift, orphaned links and ambiguous migration
candidates. **Alias Collision Inspector** shows conflicting canonical/alias
matches. Both diagnose without automatically repairing or merging records.

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
P01's additional product-manifest/restore-preflight work was deferred by the owner;
the warning is not a guarantee of automated compatibility detection. Restore
caches are bounded and can evict originals.

## Versions and updates

Settings → About shows **StashBooru 1.0.0** separately from the upstream Stash
build version and commit. Opening About checks GitHub; Check for updates retries.
Startup also logs a stable-release check. Only stable `stashbooru-vX.Y.Z` releases
from this fork are considered. An equal or older release is not an update.
Development snapshots (`latest_develop`) are available from GitHub separately.

Download the matching platform release and follow your existing installation
procedure. The check does not install updates. A network/rate-limit failure is
shown as a failed check, not as proof that the installation is up to date.
