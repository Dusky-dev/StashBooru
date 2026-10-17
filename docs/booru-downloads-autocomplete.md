# Booru thumbnails, downloads and autocomplete

Open **Booru → Source settings → Downloads and autocomplete** to set the default
JPEG conversion preference and install a tag database. These preferences and
databases are shared by everyone using this server.

## Thumbnails

Thumbnails and inspection samples use authenticated, same-origin StashBooru
requests. The server sends the site's Referer and its StashBooru User-Agent,
validates public DNS and configured CDN hosts on every connection/redirect, and
never forwards API credentials to a separate CDN host. Danbooru media-asset
variants and `image_width`/`image_height` are supported alongside older responses.

Only server-issued, unpredictable keys from successful search results can fetch
media. A browser cannot provide an arbitrary proxy URL. Keys expire after 30
minutes, after eviction from the bounded 4096-entry cache, or when the source is
disabled/its URL or engine changes. Refresh the search after expiration. Four
concurrent media requests, two-minute deadlines and 8 MiB thumbnail/24 MiB sample
limits bound retrieval. Only recognized image bytes are displayed; HTML, SVG,
unsafe redirects and unavailable assets produce a fallback. Originals never
silently replace missing thumbnails. Custom sources may need their image/CDN
hosts added in Source settings. Provider access policies still apply.

## Download an original, optionally as JPEG XL

1. Inspect a post to open its **Download** section. Review its known size, or the
   explicit unknown-size notice. The original must be available from an allowed
   host and be at most **128 MiB**.
2. Optionally enable **Convert JPEG to JPEG XL before downloading**. Set its
   default in Source settings. Other input formats remain original.
3. Select **Prepare download**. The native Jobs queue shows download/conversion
   progress; **Cancel download** cancels queued or running work.
4. When preparation completes, use **Save original** or **Save JPEG XL** to save
   to your computer. Failed or cancelled jobs do not publish a Save link.

Original transfers check actual byte length, supported media signatures and the
provider MD5 when supplied. Missing MD5 remains unknown. A separate checksum is
calculated for the completed output; the original JPEG MD5 is never represented
as the JXL file's current checksum. Partial files are removed on ordinary failure
or cancellation. Downloads do not modify review marks or imply a catalogue match.

JPEG XL uses the existing Media converter worker selection, including explicit
local/remote requirements. Configure it in **Settings → Media converter** first.
The worker must advertise JPEG XL. The existing converter performs encoding and
its codec checks; this path requests CPU, quality 100, distance 0, effort 7 and
allows larger output, using `cjxl`'s JPEG reconstruction path. No new encoder or
model is installed automatically. A failed conversion does not silently publish
the unconverted original. The output must have JPEG XL bytes, valid worker
metadata and size at most 256 MiB. Browser JPEG XL playback support is separate
from saving the file.

The original stays staged until conversion and download registration finish.
Completed files use the native generated-download store and expire after one
hour, or 30 seconds after the first download finishes. At most 100 preparation
entries are retained. Preparation times out after 30 minutes of execution;
entries still queued after an hour are cancelled. Opaque job tokens prevent reused Job IDs after restart from resuming another
export. Reopening the same post or
refreshing in the same tab recovers its last Job ID using session storage. If
storage is disabled, keep the inspection open until saving. Server restarts do
not resume these temporary exports: prepare again. A process crash can leave
unregistered files in the operating system's temporary directory; its normal
cleanup applies. This is not a durable download-to-library queue.

**Catalogue import remains B03/B04:** this explicit file-export action does not
scan into the library, create Image/Video/Artist/Character/Copyright records,
import provider tags or create automatic download marks. Those operations need
the separate metadata and import review workflow.

## Install a booru autocomplete database

Choose **Tag database source**, select a CSV, then **Upload tag database**.
Uploading replaces that source instance's previous database atomically after
validation. A stale upload is rejected; reload settings before retrying. A failed
upload leaves the installed database intact. No database is fetched from the
internet automatically, and suggestions make no provider API calls.

Supported formats:

| Format | Columns |
| --- | --- |
| [tagcomplete CSV](https://github.com/DominikDoom/a1111-sd-webui-tagcomplete#csv-tag-data) | `name,type,count,"alias_one,alias_two"`, without a header. Count and aliases are optional. |
| Danbooru-style CSV export | Header containing `name` and `category`/`type`; optional `post_count`/`count`, `aliases` and other columns such as `id`. |

Numeric categories follow the selected engine: Danbooru-compatible CSV uses
0 general, 1 Artist, 3 Copyright, 4 Character and 5 meta. e621 uses
[its own mapping](https://github.com/e621ng/e621ng/blob/master/app/logical/tag_category.rb):
2 contributor, 5 species, 6 invalid, 7 meta and 8 lore, with 0/1/3/4 unchanged.
Category names (including `unknown`, `circle` and `faults`) are also accepted;
use names for custom engine category schemes. Duplicate/invalid names, negative counts, malformed CSV and unknown
categories reject the upload. Limits: 64 MiB, one million tags, two million
names/aliases, 32 aliases per tag and 256 UTF-8 bytes per name. Files must be
uncompressed CSV; SQLite/JSON/Grabber database files are not accepted. Prefix
matching uses names and aliases, with the most-used 20 canonical tags first.

Type at least two characters in Search. Suggestions show the category, post
count, source and matching alias. Use Up/Down, Enter or Tab to choose; Escape
closes the list. Clicking also works. Completion replaces the token at the
cursor and preserves surrounding tags and exclusion operators. When only
Philomena sources are selected, commas delimit tags with spaces. Other/mixed
sources use whitespace delimiters. Provider search syntax is otherwise unchanged;
metatag expressions containing a colon are not completed or rewritten.

Databases live beside config.yml in `booru-tags/<source-instance>.csv` and are
loaded lazily, with at most four immutable indexes cached in memory. Uploads and
configuration survive restart; memory cache does not. Same engine/normalized URL
aliases share a database, credential edits retain it, and URL/engine changes
isolate it. Include `booru-tags/` and `booru-downloads.json` in server configuration
backups. Source API keys stay only in the existing `booru-browser.json`.

## Verification scope

Automated tests cover parsers, media transport/redirect/checksum/type boundaries,
CSV ranking/aliases and atomic replacement, native queued cancellation, export
publication and original/output checksum separation. A mock remote converter
exercises the real converter client and generated-download store. Mounted browser
fixtures cover desktop/mobile PNG thumbnails, uploads, keyboard completion,
conversion preferences, download completion/failure, and existing review/search
behavior. They do not establish live-account/provider availability or real
JPEG XL encoder acceptance on the deployment machine.
