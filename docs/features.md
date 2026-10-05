# StashBooru feature guide

## Where things live

| Location | Use it for |
| --- | --- |
| **All** | Browse Images and Videos together, with shared filters and typed selections. Individual Images and Videos pages remain available. |
| **Settings → Library** | Media folders, scan rules and automatic ancestor/profile associations. Inheritance switches save automatically. |
| **Settings → Processing** | Shared worker connection, similarity/tagging defaults, conversion rules and upscaling setup. Use each section's Save button. |
| **Settings → Tasks** | Jobs, generated files/hashes, animation inspection, association review, Video overlap indexing and database management. |
| **Settings → System** | Application paths, FFmpeg, transcodes and other server settings. |

Processing defaults apply to new jobs. Conversion/tagging defaults and model paths
are saved on the server. Upscaling job defaults are saved in the current browser;
their Save button does not change server paths. Old System bookmarks for moved
sections redirect to the new location.

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

Image Tagging and Video frame analysis can review filename, lookup and model
results before applying them. Filename metadata works without Camie. Camie is
optional and requires its model and metadata files on the selected worker. Tagging
thresholds control predictions; they do not tune Find similar distance.

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

## Versions and updates

Settings → About shows **StashBooru 1.0.0** separately from the upstream Stash
build version and commit. Opening About checks GitHub; Check for updates retries.
Startup also logs a stable-release check. Only stable `stashbooru-vX.Y.Z` releases
from this fork are considered. An equal or older release is not an update.
Development snapshots (`latest_develop`) are available from GitHub separately.

Download the matching platform release and follow your existing installation
procedure. The check does not install updates. A network/rate-limit failure is
shown as a failed check, not as proof that the installation is up to date.
