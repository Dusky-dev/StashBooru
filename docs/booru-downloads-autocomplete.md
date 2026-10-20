# Booru downloads, previews and autocomplete

Open **Booru → Settings** for sources, credentials, downloads and autocomplete.
The same controls are available under **Settings → Metadata Providers → Booru sources**.
Download preferences and installed CSV databases are shared by this Stash server.
Autocomplete mode and thumbnail visibility are remembered in this browser.

## Download into Stash

Set **Default Stash download folder** and save. **Choose default folder** browses
existing directories inside configured Stash libraries. You can also enter an
absolute server directory. Add a library under Stash's normal configuration first
if the desired directory is outside the existing libraries.
Wait for saved settings to load before editing. Saving updates the defaults used
by cards already open in this browser.

Each thumbnail card has **Download** and **Choose download folder** icon actions. These use the saved
conversion switch and default folder; the folder action overrides only that download.
Inspect a post for full progress, conversion controls and destination details:

- **Download to Stash** to use the saved default directory.
- **Download to…** to browse/select a directory for this download only. This does
  not change the default directory.
- **Convert using Media converter defaults** to run the existing converter after
  native import. Its default checkbox state comes from the browser settings.

The native Jobs queue downloads the original, checks its actual size and provider
MD5 when supplied, saves it without overwriting an existing file, and runs the
native scanner on that file. **Open in Stash** links the registered Image or Video.
Source-post URLs are added without replacing existing URLs or curated metadata.
Artist/Character/Copyright/tag imports still require the separate metadata review
workflow; saving a file does not silently create those entities or review marks.

Downloads are explicitly bounded to one reviewed post and a 128 MiB original.
A native scan can reject a file because of configured exclusions or extensions;
the error is shown and the saved file remains available for retry. The filesystem
must support hard links for atomic publication without overwriting files.
Destination paths are checked against actual configured library roots, including
symlink escape checks. Existing files with different bytes are never overwritten.

A receipt in `booru-download-history/` records public source/post identity, original
MD5, destination and imported result. Retrying reuses an unchanged existing file
and scans/registers it again rather than creating another copy in that directory.
A completed receipt is checked against current file bytes before reuse. A changed,
missing or unregistered native entry is not assumed to be in-library just because
a previous Job succeeded. Failed imports and cancelled Jobs retain already-saved
files; partial transfers and unpublished staging files are removed. Receipts and
native scans permit explicit retry after restart; automatic resume is not added.
Reopening a post in the same tab recovers the last live Job ID using session
storage. Expired/restarted Jobs can be retried. Current Job status expires after
one hour; persistent library files and receipts are retained.

## Filename templates

**Download filename template** in Settings accepts Grabber's common `%token%`
vocabulary for native Stash downloads. Leave it blank to retain the existing
`booru-<instance>-<post>-<MD5>.<extension>` scheme. Example:

```text
%artist%/%copyright%/%character%/%id%-%position%-%md5%.%ext%
```

Supported tokens: `%artist%`, `%copyright%`, `%character%`, `%id%` (provider post
ID), `%asset_id%` (stable attachment identity), `%position%` (one-based file index),
`%md5%`, `%ext%`, `%filename%` (remote basename without extension), `%rating%`,
`%website%` (API hostname), `%websitename%`, `%width%`, `%height%`, `%author%`,
`%authorid%`, `%score%`, `%date%`, `%current_date%`, `%name%`, `%title%`, `%num%`.
Dates use UTC `MM-DD-YYYY HH.mm` when supplied as RFC3339. Missing text/category
metadata becomes `unknown`; dimensions/scores can be zero. Categories use supplied
core tags, not inferred classifications. Multiple core tags join with `, `.

Literal `/` creates subfolders inside the chosen library directory. Values cannot
introduce folders; forbidden/control characters and Windows reserved names are
sanitized. Templates must be relative, end in `.%ext%`, and stay within eight
components/512 bytes; rendered components are capped at 200 bytes. Parent paths,
unknown tokens, malformed substitutions and symlink template folders are rejected.
This is a common-token compatibility subset of
[Grabber filename settings](https://www.bionus.org/imgbrd-grabber/docs/filename.html),
not its scripting, options or conditional grammar.

`%md5%` and `%ext%` use the original bytes actually downloaded, not guessed URL
metadata. Subsequent native conversion retains the converter's naming/activation
rules and changes the output extension as usual. Different existing bytes get a
numeric suffix (or increment `%num%`); identical bytes at the chosen name can be
reused. Template/destination and the original asset reference are checked before
receipt reuse, including nested folders. The legacy computer-export endpoint
retains its fixed filename; this setting governs **Download to Stash**.

The folder chooser's header **Root** icon returns to configured libraries.
Entering a server path browses it after a short pause (or Enter); the redundant
Open directory button is removed. **Use this folder** waits for a valid directory
listing and remains bounded to configured libraries.

## Conversion uses the converter's saved defaults

The browser no longer forces JPEG XL, quality 100, effort 7 or CPU. It uses the
native Media converter's saved input-format mapping, encoding quality, effort,
lossless/alpha/audio options, decode-speed controls, worker selection and savings
policy. For JPEG → JPEG XL, configure that mapping and its quality in **Media
converter** first, then enable browser conversion.

The original is first imported as a normal native file. The existing converter
then updates that same file/media identity through its activation and recovery
system, retaining metadata and relationships. Its normal original-cache and
restore rules apply. A savings-policy skip keeps the original and reports why.
A conversion failure leaves the imported original available in Stash and reports
the failure. Repeating a download does not automatically re-encode an already
converted result or restore an original over a curated current file.

No new encoder, model or alternate conversion implementation is installed. The
selected worker must support the configured formats. Browser JPEG XL playback
support is separate from saving/importing the converted file.

## Animated previews

Inspection offers explicit playback for MP4/WebM videos and GIF/APNG images up to
24 MiB. Browsing thumbnails loads no animated originals. The authenticated motion
route accepts only an existing server snapshot, rechecks the enabled source/CDN,
checks actual media bytes through the native client and supports bounded byte-range
responses for seeking. Stop/navigation/close release playback. Unsupported codecs
and oversized files retain static previews, download and remote-post options.
This preview never imports a file or marks it downloaded.

## Live autocomplete and optional CSV fallback

Autocomplete defaults to **Live, with CSV fallback** in the **Autocomplete** icon menu.
It queries the selected source through the authenticated server route; a CSV
file is optional. **Installed CSV only** uses the installed per-source database
without contacting providers. A failed live request falls back to the installed
CSV when available and shows a notice. No usable live response or database gives
an explicit unavailable notice. Artist-directory search on Pawchive/Kemono is a
separate provider lookup, available by name in the browser sidebar.

| Engine | Live endpoint |
| --- | --- |
| Danbooru | `/autocomplete.json`, tag-query mode. |
| Gelbooru | `/index.php?page=autocomplete2`; Rule34 uses its public `api.rule34.xxx/autocomplete.php`. |
| Moebooru | `/tag.json`, prefix name and count ordering. |
| e621 | `/tags.json`, prefix name matching and count ordering. |
| Philomena | `/autocomplete/tags`; at least three prefix bytes are needed. |

Credentials stay in the server configuration. Requests share the browser's
public-HTTPS transport, redirect rejection, one-request-per-second host gate,
global concurrency and provider rate-limit cooldown with searches. Completion
waits 350 ms after typing, aborts obsolete requests, limits responses to 512 KiB
and 200 raw records, and returns at most 20 suggestions. Successful results use a
five-minute, 512-entry cache isolated by source instance, credentials and prefix.
Errors are not cached as empty success. Site/API access restrictions still apply;
compatible custom sites must expose the selected engine's endpoint.

Counts and categories come from each provider. Missing categories remain unknown;
Rule34's label may supply a count without a category. This is autocomplete data,
not a native Artist/Character/Copyright mapping. No original media is downloaded
by typing a tag.

## Install an optional autocomplete database

Choose **Tag database source**, then use either:

- **Autocomplete database CSV path on the server → Load database from path**.
  Enter an absolute path, for example `/models/autocomplete/danbooru.csv`. This
  reads a regular CSV file on the Stash host, including mounted model/data folders.
- **Tag database CSV → Upload tag database** to upload a file from your computer.

Both paths validate and atomically install a per-source copy. The server path is
remembered; press **Load database from path** again after updating that source
file. There is no automatic directory scan or periodic reload. A failed load or
stale upload leaves the installed database intact. Reload settings after a revision
conflict. These are search suggestions, separate from native catalogue tags.

Repeated tag names in provider snapshots are combined into one suggestion. The
row with the highest post count supplies its count/category; ties keep the first
row. Aliases are combined without counting duplicates twice. Counts are never
added together, and malformed duplicate rows still reject the replacement.

A ready-made [Gelbooru CSV snapshot](https://github.com/DraconicDragon/dbr-e621-lists-archive/blob/main/tag-lists/gelbooru/gelbooru_2026-06-10_pt20.csv)
is available from DraconicDragon. It is dated June 2026, uses a post threshold of
20 and has no aliases. Select Gelbooru before installing it. This is a community
snapshot, not an automatically refreshed official database.

For **Rule34.xxx tag categories**, a community
[Rule34 tag CSV snapshot](https://huggingface.co/datasets/deepghs/site_tags/blob/main/rule34.xxx/tags.csv)
is available from DeepGHS (`type,count,name,ambiguous,id`, last synced
August 2025). Download the CSV, choose **Rule34.xxx** in Tag database source,
then **Upload tag database** (or load its absolute path on the server).
This imports the actual Rule34 category assignments and bypasses unreliable
live type lookups, including on providers that reject post-page requests.
It is not official or automatically updated; newly introduced tags may
remain under **Tags — category not supplied**. Deprecated type 2 is retained
as unknown, not guessed as copyright. Installation must complete successfully
before categories appear in browser search results.

| Format | Columns |
| --- | --- |
| [tagcomplete CSV](https://github.com/DominikDoom/a1111-sd-webui-tagcomplete#csv-tag-data) | `name,type,count,"alias_one,alias_two"`, without a header; count/aliases optional. |
| Danbooru-style CSV export | Header containing `name` and `category`/`type` in **any column order**; optional `post_count`/`count`, `aliases`, and other columns such as `id`. |

Numeric categories follow the selected engine. Danbooru-compatible data uses
0 general, 1 Artist, 3 Copyright, 4 Character, 5 meta. Gelbooru/Rule34
additionally accepts deprecated type 2 as unknown. e621 uses its own mapping:
2 contributor, 5 species, 6 invalid, 7 meta, 8 lore, with 0/1/3/4 unchanged.
Category names including `unknown`, `circle` and `faults` are accepted for custom
schemes. Limits: 64 MiB, one million tags and two million names/aliases
for most engines; Gelbooru/Rule34 CSVs allow up to two million unique tags
and three million names/aliases. All engines retain 32 aliases per tag and
256 UTF-8 bytes per name. Uncompressed CSV is supported; SQLite/JSON
and Grabber database files are not.

Type at least two characters. Suggestions include categories, counts, sources and
matching aliases. Up/Down selects, Down then Enter / highlighted Tab completes, Escape dismisses; clicking
also works. Completion replaces the token at the cursor and preserves exclusions
and surrounding tags. Philomena-only searches use comma-delimited tags with
spaces; other/mixed sources use whitespace. Metatag expressions containing a colon
are left alone. One unavailable database does not discard other sources' results.
CSV-only mode performs no provider API request. No database is downloaded
automatically; live mode requests only bounded suggestions for the typed prefix.

Databases live beside config.yml in `booru-tags/<instance>.csv`, with saved path
metadata alongside them. Matching engine/normalized URL aliases share a database;
credential changes retain it, URL/engine changes isolate it. At most four indexes
are cached in memory. Include `booru-tags/`, `booru-downloads.json` and
`booru-download-history/` in configuration backups. API keys stay in the existing
`booru-browser.json`.

## Refresh and preview recovery

The browser remembers its submitted query, source pages, inspection, unfinished
search text/source selection, settings view, tag sidebar source and scroll
position in local browser storage. Refreshing or opening the bare `/booru` route
restores that state. An explicit query URL takes precedence. Submitted results
are fetched again, so live provider listings can still shift between requests.
Disabled storage leaves URL-based navigation available.

Preview/sample requests first use Stash's authenticated media route. Only opaque
keys issued for configured source/post snapshots can fetch bytes; arbitrary proxy
URLs are rejected. The server uses public DNS and declared CDN hosts, HTTPS-only
redirects, source Referer and its User-Agent. API credentials never go to another
CDN host. Legacy HTTP media URLs are upgraded to HTTPS before validation.

If server delivery fails, an available public, query-free provider/CDN URL can be
tried directly by the browser. Query-bearing URLs are not exposed as fallbacks; this does not classify every
provider-specific signed URL format. If both paths fail, the inspector shows a
useful server error and **Refresh previews** fetches fresh search keys. Missing thumbnails never trigger
silent original-file downloads. Site credentials, CDN allowlists and provider
access restrictions still apply.

Asset keys last at most 30 minutes in a 4096-entry bounded cache. Server media
retrieval uses four slots, two-minute deadlines and limits of 8 MiB per thumbnail
and 24 MiB per sample or explicit animated original. HTML/SVG are refused by the server media route. Direct CDN
fallback uses the browser's normal image handling and network access.

## Verification scope

Tests cover live engine endpoints, credentials/cache isolation, shared throttling,
cancellation, CSV fallback and source changes, native scan registration/idempotence,
saved converter settings,
destination/symlink boundaries, retention after import failure, retry without
overwrite, CSV path import/revision behavior, preview transport and browser
refresh/fallback/default/custom folder flows. Synthetic fixtures and a local PNG
scan are separate from live-account testing or real JPEG XL codec acceptance.

The browser now uses one source at a time. Enter searches the typed query unless
an autocomplete row was explicitly selected with an arrow key; an exact tag or a
submitted query closes suggestions. Settings has a dedicated `/booru/settings`
route, so Back returns to the browser. Preview downloads keep the default-converter
switch and folder actions; detailed policy and default-folder text stay in Settings.
