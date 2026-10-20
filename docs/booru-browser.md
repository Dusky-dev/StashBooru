# Booru browser

Open **Booru** (globe) in the navbar, or `/booru`. This browser searches
remote posts and displays their previews and metadata.
[Downloads and autocomplete](booru-downloads-autocomplete.md) adds explicit file
downloads into Stash, optional saved converter defaults, and live tag completion with optional per-source CSV databases via upload or
server path. Native scan registers media; Artist/Character/
Copyright/tag metadata import remains separate reviewed work.
[B02 review state](booru-review-state.md) adds durable ignored/manual-download
marks and bounded current-file MD5 matches. Metadata entity review, batch downloads,
pHash comparison and follows remain later [roadmap packages](booru-browser-roadmap.md).

## Rule34.xxx API access

Rule34.xxx requires an account **API key** and **User ID** for DAPI searches
(as of August 2025). In **Booru → Settings → Sources → Rule34**, enter the
separate values and save. Obtain them from the
[Rule34 account Options page](https://rule34.xxx/index.php?page=account&s=options)
under **API Access Credentials**. If the page has no API key, select
**Generate New Key** and save the account options first. In the displayed
`&api_key=...&user_id=...` string, paste only the value following each
parameter into its matching StashBooru field. Do not share the credentials.

Keep the source's website URL as `https://rule34.xxx`. StashBooru
routes its API searches and live autocomplete to `https://api.rule34.xxx`
automatically, while post links still open the public site. Existing site
settings and review identities are preserved. Credentials are kept server-side
and excluded from public source metadata and error messages.

Without both fields, StashBooru reports the missing credentials instead of
sending an anonymous request. An HTTP 401/403 **after configuring** credentials
may instead indicate an invalid key, account restriction, rate enforcement or
Cloudflare challenge. StashBooru does not bypass such provider restrictions;
try the official API from your network and check the account before retrying.

## Search and inspect

Choose one site from **Source**, enter a query (or leave it blank) and select **Search**.
Blank submission opens the default listing; Pixiv uses daily rankings. Each site
has an icon, with a local initial fallback if its favicon is unavailable. Changing
sites opens that site's listing for the current query. Archive creator expressions
are cleared when changing sites. Up/down icons reorder the source menu without
changing the selected site; order is saved in this browser. Pawchive opens recent
posts when selected because its API cannot filter post-text queries. Legacy multi-source links and saved filters use their first source.
A fresh browser without saved state does not search automatically. Refresh and
bare `/booru` visits restore the submitted page and unfinished search draft.
Previous/Next, cancellation and retry apply to the selected site. Inspect a
thumbnail to open the larger preview; **Open remote post** opens the source site.

On Pawchive and Kemono, **Find an artist** searches the provider's artist directory
by name. Select the matching artist/service to browse their posts; no numeric
creator expression needs to be typed. Matching names on different services remain
separate choices. Directory errors have a retry action. Post-text search remains
available on Kemono; Pawchive's posts API ignores text queries, so those are rejected.

With the rating control at its default, the same query is sent unchanged
to the selected source. Other rating values append the selected engine's native filter
syntax; Philomena uses its native sort parameter and rating-tag expression. Space-separated tags and
underscores are common to the first four engines; Philomena uses its own syntax,
including tags with spaces and comma/AND combinations. Expand **Search help** in the sidebar for each selected source. Provider ratings, search limits, account filters and metatags
remain provider-specific. Unsupported/rejected queries are errors rather than
being silently simplified to a broader search.

The selected site and icon stay in Source inside Stash's list toolbar.
The toolbar contains Source, the editable native expression and removable term
chips, Search, rating/media/autocomplete icons, display operations, Favorites,
saved search filters, sorting, page size, Grid/Wall and thumbnail zoom.
Dropdown highlights clear when closed; keyboard focus stays visible. The sidebar retains
loaded-page tags and source help. Source is a single-choice menu. Suggestions
carry category/source labels. **Enter** searches the typed expression; use
**Down then Enter** to choose a suggestion. Tab accepts only an explicitly
highlighted suggestion. Exact tags and submitted searches do not leave a popup
open, including when a delayed response arrives. Editing starts a fresh completion.


Cards retain only the preview, number, rating, selection and named icon actions.
Artist, Copyright and Character icons search immediately: a single tag starts a
search; multiple tags open a chooser, then search the chosen tag. Sidebar tags behave
the same way. Disable **Search immediately when clicking tags** in Booru Settings
to only fill the draft. This preference is saved in this browser. Categories come from the provider or an exact canonical-name match in the
installed per-source CSV. Prefixes and aliases never guess categories for flat tags.
For **Rule34.xxx**, StashBooru attempts live post-tag classification
without requiring a CSV. The server first reads typed tag links from up to
three representative public post sidebars, accepting both wiki `search=` links
and post-list `tags=` links inside `#tag-sidebar`. It reads a bounded HTML
prefix so long comment threads after the sidebar do not invalidate otherwise
usable categories. The resulting tag names must exactly match tags on the
search page. No API credentials are sent to public post pages.

For tags still unresolved, StashBooru attempts exact `name` lookups on the
authenticated Rule34 tag DAPI (`s=tag&q=index&name=...`), rather than the
unsupported Gelbooru bulk `names` query. Rule34's published DAPI help
documents tag lookup by `id` and `limit`, **not** `name`; therefore
classification through the `name` parameter is not guaranteed. Parsed
numeric types are used only for exact returned names, and confirmed categories
are cached for 24 hours.

To avoid overwhelming Rule34, the live fallback makes at most six
single-name tag requests per search and shares an eight-second lookup
deadline with sidebar parsing. This is **best effort**: the first search may
leave some tags under **Tags — category not supplied**, and new results can
become classified as the positive cache fills across subsequent searches.
Valid but unmatched tag records are cached as unknown for five minutes,
preventing the same absent tags from consuming every subsequent request.
If Rule34 blocks the website or its HTML changes, and the tag API does
not provide exact typed matches, categories remain unknown rather than
guessed. This change is covered by provider-shaped regression tests; it
has **not** been verified end-to-end against the user's live Rule34 account.
An installed per-source CSV takes precedence and remains an optional
offline fallback.

The inspector groups and colors tags by category. Click a tag for **Add to search**
or **Replace search**; these update and submit the query and close the preview.
With immediate search disabled, they only update the draft and close the preview.
Metadata uses the native resolution label, dimensions, file-size units and uppercase
format. The MD5 and local-match evidence share the bottom row. Download actions
retain the converter switch, without repeating the settings explanatory text.

**Hide AI-generated posts** is a two-state robot-icon button inside
`.booru-filter-icons`. The robot gets a diagonal slash when filtering is
enabled rather than using a highlighted button state. It is off by default,
and when pressed hides posts on
the **currently loaded page** that have an explicit `ai_generated`,
`ai-generated` or equivalent case/spacing variant tag. It does **not**
classify images from pixels, and `ai_assisted` and
`ai_generated_background` alone do not count as fully AI-generated.
The URL preserves the setting as `ai=hide` across refresh, navigation and
saved searches. The button works together with the media filter, without
refetching or changing the native provider query. Provider pagination and totals
still include filtered-out posts; a page may look empty even when later pages
contain untagged posts. Untagged or incorrectly tagged AI art cannot be
automatically recognized.

**Rating** offers All ratings, Safe / General, Sensitive, Questionable and Explicit.
Sensitive is separate on Danbooru and compatible Gelbooru sites; it is disabled on
engines without that rating. The menu entries and selected underline use matching
colors. Safe means General on current Danbooru/Gelbooru;
Philomena's Questionable also includes Suggestive. Leave All ratings selected for
native rating expressions. Pixiv, Shimmie and archive adapters currently require
All ratings and provider ordering. **Sort by** offers Provider default, Recent,
Oldest, Popular, Score high/low and Updated where supported. Popular uses provider
score ordering, not a computed time-window ranking. Use Provider default when
entering native order expressions.

**Grid** lays cards out in columns; **Wall** fits their aspect ratios into justified
rows. The native zoom slider resizes both preview height and column width in Grid,
and row height in Wall, in four steps. View and zoom are
remembered locally across refreshes. Previous/Next remain at the bottom. The
pagination index, redundant results header and always-visible review toolbar are
removed. Rating sits beside the post number in the card caption.

## Favorites and saved filters

The toolbar star opens the selected site's **account favorites**. Save its API key
and login (Danbooru/e621) or user ID (Gelbooru/DAPI) in Source settings, then choose
**Browse account favorites**. Credentials stay on the server. These are read-only
listings in provider order: inspection/download/review work as in search, while
query, rating and sort filters are cleared. Loaded-page media filtering still works.

Danbooru uses `ordfav:username` post search, e621 its authenticated favorites
endpoint, and Gelbooru its favorite-index API followed by bounded post detail
lookups. Gelbooru totals are advisory: a full page keeps Next available even when
its reported count is lower. All use the existing transport, rate gates and response limits. A Gelbooru
page may require one detail request per favorite; cancellation is supported.
Unsupported engines or missing credentials are explained in the menu. Some DAPI
clones do not expose the favorites API and return a provider error. Provider
privacy/content restrictions and removed posts still apply. Changing account or
source during a request rejects its response.

The local card favorite icon is removed. Previously saved StashBooru favorite data
is preserved in `booru-bookmarks.json`; the UI no longer displays or adds local post
favorites. Remote favorite creation/removal is not implemented.

The separate filter icon opens **Saved search filters**. Name and save the current
query/source/filter combination, select a saved name to apply it, or use its trash
icon to delete it. Saving an existing name replaces that filter. Empty queries
are saved as explicit `q=` submissions, preserving the provider's default listing
(including Pixiv daily rankings). Legacy blank filters are repaired when loaded.
Account favorites filters keep an empty query even when the text box has an
unsubmitted draft. Up to 100 named
filters and legacy local favorite snapshots are stored in private `booru-bookmarks.json`, with
an 8 MiB metadata bound and atomic updates. This is separate from ignored/manual
Downloaded history. No remote follow or automated download is created.

## Provider additions

| Source / engine | Search and authentication | Media limitations |
| --- | --- | --- |
| Paheal / Shimmie | Native tags through the anonymous Danbooru XML endpoint (`/api/danbooru/find_posts`) | Category information may be unavailable; no live completion endpoint. The legacy `/api/danbooru/find_posts/index.xml` URL can return HTML rather than posts XML, so older StashBooru versions may show an unsupported-data error. |
| Pixiv | OAuth **access token** in source API key; text search, `user:123` for artist works, `id:123` for a work | Static illustration/manga pages selectable in the inspector. Ugoira frame ZIPs are explicitly unavailable. No login/token refresh is implemented; replace expired tokens in Settings. |
| Pawchive | Artist-name directory search, or `*` for recent posts; anonymous API. Text post search is unsupported because the API ignores it | Optional API key is the `session` cookie value, sent only to `file.pawchive.pw`. Preview-only/deferred/unavailable originals cannot be downloaded. |
| Kemono | Artist-name directory search, `*` for recent posts, or text of at least three bytes; anonymous API | Supported image/video attachments and ZIP/CBZ previews; documents and unsupported archives are omitted. |

Custom sources can select these engines and configure their HTTPS API root/CDN
hosts in Settings. Preset hostnames can change; failures remain visible for retry
or source configuration. Media is validated through existing size, host, public
DNS and format checks. The adapters use the APIs independently; no Grabber source
code or login flow is embedded.

Pixiv/archive APIs use fixed upstream blocks of 30/50 posts. The browser slices
aligned blocks for the selected page size without skipping records. A gallery is
one card: choose **Media file** in inspection to preview/download a particular
attachment. Moving to another post resets the selector to its primary file while
keeping keyboard navigation active. Keyboard Up downloads the selected attachment. Ignore applies
to the post; alternate files do not inherit first-file MD5 evidence. Up to 200
additional attachments per post and 256 media snapshots per result are accepted;
reduce page size if galleries exceed the result bound. Downloads remain one
explicitly chosen file at a time. No text/ZIP import or bulk gallery download is
added. Installed CSV completion remains available on these new engines.

## Display, downloads and inspection

**Media on this page** filters the currently loaded page into Images or Videos.
It does not refetch or repaginate the provider, and totals still include all media.
When `file_ext` is absent, the adapter reads the original URL extension without
downloading it. Missing/unrecognized formats remain unknown; GIF/APNG are images. **Show thumbnails**
controls grid image loading; inspecting a post remains an explicit preview action.
These controls, per-source pages and inspection persist through refresh/back.
Autocomplete mode and thumbnail visibility are local browser preferences.
The filter icons stay available on desktop and mobile. **Show thumbnails** and
**Show ignored posts** are grouped as native Stash switches. Sources and settings
checkboxes use the same switch style; card selection uses Stash's card checkbox.

**Posts per source page** uses Stash's page-size selector, with a maximum of 50.
Changing it resets source pages and closes inspection; its `limit` URL parameter
survives refresh/back. Provider filters and ignore marks may reduce the visible
count without automatically filling the page.

Each card exposes a **Download** icon (saved default folder/converter preference)
and **Choose download folder** icon (choose an existing library folder). These use the same native Jobs,
checksums, import and converter flow as the inspector. Changes saved in download
settings refresh mounted card preferences, including when an older request ends
late. Cards and the inspector share newly queued job identity. Downloading remains
an explicit action, separate from manual review marks.

Inspect a post to use **Previous/Next preview** or the Left/Right arrow keys within
that source's currently loaded page. Ignored posts are skipped unless shown;
navigation stops at page boundaries. Up arrow starts the same guarded default
library download; Down arrow marks the current post ignored. Keys do not repeat
download/ignore writes when held, and text fields, nested folder dialogs and native
video controls keep their normal keyboard behavior. Ignoring does not close the
inspector; the next navigation skips hidden ignored posts.

For MP4/WebM videos and GIF/APNG images, **Play animated preview** explicitly loads
the original through the protected preview route, up to 24 MiB. Video starts muted
with native playback/seek controls. Stop, navigate or close to release playback.
Large or unsupported formats retain their static preview and remote/download
options. Browsing thumbnails does not prefetch animated originals.

Query, selected sources, per-source page and inspected post identity are in the
URL, supporting refresh/back navigation. This is a live listing, not a persistent
snapshot: new/changed remote posts can shift page boundaries. Results are grouped
by source, de-duplicated by post ID within each returned page, and retain distinct
source/post identity across providers. There is no cross-source or cross-page
deduplication yet. Totals are unknown except for provider-reported Philomena
and Shimmie counts; these are not verified snapshot totals. Numbered browsing ends at page
750; narrow the query to continue.

### Archive-contained media (ZIP/CBZ)

Pawchive/Kemono ZIP and CBZ attachments are included in the post inspector.
The server downloads the archive only when its media section is viewed, validates
its ZIP structure and source URL, then exposes supported image and video entries
as a lazy-scrolling gallery. Entries are streamed to the browser only after
media signature validation and are never extracted to the server filesystem.

**Limits:** 256 MiB per archive by default (configurable 16–512 MiB in Downloads and autocomplete), 512 entries examined, 128 eligible
media entries per archive, 256 MiB decompressed per entry by default (same configurable limit), limited compression
ratio and at most four cached archives for five minutes. Unsafe paths, symlinks,
unsupported formats and nested archives are not opened; the inspector reports
skipped entries. If an archive is too large, corrupt or password-protected,
open the remote post instead. RAR and 7z are not supported yet; archive contents
can be saved individually using **Download to Stash** or **Download to…**. Each extracted media file has its own download job, native scan and optional converter settings.

### Posts with multiple media files

The post inspector displays every supported file in a **vertical media stream**,
rather than the old `Media file` dropdown (`btn-secondary form-control`).
Each media entry has its own preview, format, resolution, hash when supplied,
and download controls. As you scroll the modal, more entries are mounted
three at a time; previews stay lazy and animated/video playback remains
on-demand. The stream stops at the supported media files the provider already
returned; it does not crawl additional posts or invent unavailable attachments.

**Up arrow** downloads the media file currently in view or focused; the other
existing post-navigation/ignore shortcuts are unchanged. Switching posts
resets to the first media file and releases preview playback. The media list
uses the modal's scroll area on desktop and mobile, with metadata/tags below
the list. Missing or unsupported previews keep their individual fallback and
download/open options.

Preview bytes use a bounded same-origin media route backed by server-owned
search keys, validated provider/CDN hosts and public DNS. Missing, expired,
blocked or unsupported previews have a fallback message and remote link.
Originals are never substituted for missing thumbnails. See
[thumbnail limits](booru-downloads-autocomplete.md#refresh-and-preview-recovery).
Dimensions, byte size, rating and MD5 are provider metadata; unknown
fields stay unknown. A preview is not proof of full-file identity or quality.
Flat tag APIs are labelled **category not supplied**, rather than classifying
every tag as general or guessing Artist/Character/Copyright identity.

## Built-in sources and engine support

| Engine | Presets | Credentials supported by this browser |
| --- | --- | --- |
| Danbooru | Danbooru, Safebooru (Danbooru) | Optional login + API key using HTTP Basic. |
| Gelbooru JSON DAPI | Gelbooru, Safebooru, Rule34 | API key + user ID in the server's API request. A provider can require these even for searches. |
| Moebooru | Yande.re, Konachan | Anonymous search only; legacy password-hash login is not implemented. |
| e621 | e621, e926 | Optional login + API key using HTTP Basic; current v2 and legacy response layouts are understood. |
| Philomena | Derpibooru | Optional API key; the source's content filters still apply. |
| Shimmie XML | Paheal | Anonymous API. |
| Pixiv App API | Pixiv | OAuth access token; no automatic refresh/login. |
| Pawchive | Pawchive | Anonymous API; optional file-host session cookie value. |
| Kemono | Kemono | Anonymous API. |

These are API adapters and presets, not a claim of complete Grabber site/login
parity. Szurubooru, Sankaku, HTML scraping, executable source plugins and
browser-session login are not implemented. Compatible instances of
the engines above can be added in **Settings**. More engines
need their own documented adapter and the same acceptance checks.

## Source settings

Open **Settings** in the browser header (`/booru/settings`). The browser Back
button returns to the posts/query/page that opened settings, including after a
settings refresh. **Back to posts** also works when settings was opened directly.
The existing application entry is **Settings → Metadata Providers
→ Booru sources**. This settings page combines source/API credentials, downloads, converter preferences
and optional CSV autocomplete databases. Saved server settings apply to everyone
using this server.

- **Edit** a preset to add credentials, change its CDN hosts or disable it. Use the
  username + API key for Danbooru/e621, user ID + API key for Gelbooru JSON DAPI,
  or API key alone for Philomena. Moebooru remains anonymous.
- **Add custom booru**: choose a stable source ID, display name, supported API
  engine and public HTTPS site URL. Include an installation path if needed. Add
  separate image/CDN hostnames when previews live outside the site host. Choosing
  an engine means the site must implement that engine's supported API.
- A blank API key keeps the saved key. **Remove saved credentials on save** clears
  the key and account fields. Saved keys never return to the browser. When the URL
  or engine changes, re-enter or explicitly remove the existing key so it cannot
  silently move to another host.
- **Enable this source** controls availability in searches. Disabled definitions
  remain editable. Custom sources can be removed; presets can be reset, removing
  their overrides and saved credentials. IDs cannot be renamed in the editor.
- **Save source** persists immediately without restarting StashBooru. The browser's
  source list and current searches refresh after a save. **Reload saved sources**
  discards the current editor draft. A stale editor gets a conflict error rather
  than overwriting another tab or an external file edit.

Search and source selection stay in the top toolbar; categorized page tags
and source help remain in the left sidebar, with thumbnails on the right. On narrow screens controls stack above
a two-column grid and page tags expand with **Show page tags**. Category labels
accompany category colors. Counts cover only the selected source's loaded page,
with up to 20 tags per category; these are not provider-wide related-tag counts.
Selecting a tag prepares a query; press **Search** to run it. Independent source
paging, inspection URLs and raw provider syntax retain their previous behavior.
These definitions configure the remote browser; the existing local-media MD5
lookup provider list is unchanged.

## Server configuration file

The optional **`booru-browser.json` beside `config.yml`** contains an array of
source overrides/additions. Missing file means the fourteen presets. A complete entry
with an existing ID replaces that preset; an ID-only disabled entry removes it.
Custom source IDs are stable lowercase letters/digits/underscores/hyphens, up to
64 characters. Keep IDs stable for bookmarks and future review/follow records.
Use a new ID when switching to a different provider instance; do not repurpose
another site's identity.

Example (replace placeholders with your own values):

```json
[
  {
    "id": "gelbooru",
    "name": "Gelbooru",
    "engine": "gelbooru",
    "url": "https://gelbooru.com",
    "apiKey": "YOUR_API_KEY",
    "userID": "YOUR_USER_ID"
  },
  { "id": "rule34", "disabled": true },
  {
    "id": "my-board",
    "name": "My compatible Danbooru",
    "engine": "danbooru",
    "url": "https://board.example.org",
    "mediaHosts": ["cdn.example.org"],
    "login": "YOUR_LOGIN",
    "apiKey": "YOUR_API_KEY"
  }
]
```

The settings UI writes this same file with atomic replacement and owner-only
permissions (`0600` on Unix). Keys remain plaintext server configuration. Protect
its filesystem and backups like other server secrets; do not commit live keys.
Manual file edits remain supported.
Source/configuration errors are redacted in HTTP responses. The source-list API
returns only public fields and whether credentials are configured. API keys are
never returned to the web UI or put into its search/remote-post URLs. Newly
entered keys travel only in the JSON settings request body. Configuration
is re-read for new requests; replace the file atomically to avoid partial reads.
Invalid configuration fails visibly rather than silently enabling a different
source; repair malformed files on the server before editing in the UI. Settings
are limited to 1 MiB and 64 overrides, with up to 64 image hosts per source. A definition is configuration, not arbitrary executable Grabber code.

Sources must use public HTTPS and port 443, without URL credentials, query or
fragment. Optional path prefixes are preserved. Preview hosts are restricted to
the source hostname/subdomains and explicit `mediaHosts`/their subdomains.
Built-ins declare their required shared CDNs. Private/loopback/link-local/reserved
API destinations and mixed public/private DNS responses are rejected; validated
public addresses are pinned when connecting. Redirects are not followed and
credentials are not forwarded to another host. **Local/private booru instances
and environment HTTP proxies are not supported by this first transport.**

Each API request has a 20-second overall budget, a 15-second HTTP timeout and an
8 MiB response bound. Shared limits permit four active requests globally and one
request at a time per host, spaced by at least a second. A valid `Retry-After`
pauses requests to that host for up to an hour. There are no automatic retries;
the UI reports the failure/pause. Responses are limited to 50 posts, with 24 used
by the UI, and bounded tag counts/lengths. A provider may impose stricter limits.
No unbounded catalogue scan, model inference or database migration is introduced.

## Validation and references

Adapter tests cover all five families, numeric/string IDs, current/legacy e621,
unknown tags/metadata, paging, hostile URLs, DNS destinations, credentials,
redirects, throttling, cancellation and response bounds. Route tests cover public
source configuration/redaction, credential retention/removal, preset/custom
lifecycle, concurrent stale edits, atomic write failure and cross-site writes.
The browser fixture covers desktop/mobile settings, custom sources, credential
clearing, conflict recovery, disabling/removal, page tags, search, partial failure/retry, independent paging, URL inspection,
refresh/back, stale requests, cancellation, cleanup and page overflow with
synthetic media and mocked transports.

Run the mounted fixture from `ui/v2.5` with an installed Playwright module:

```sh
PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs \
CHROMIUM_EXECUTABLE=/path/to/chromium \
node tests/browser/booru-browser.mjs
```

The dedicated **Booru browser** PR workflow runs it on Ubuntu with isolated,
pinned Playwright tooling. No provider credentials or live remote images are used
in CI. See the [progress ledger](implementation-progress.md) for actual checks and
live-provider limitations; fixtures alone do not prove current provider access.

Primary references checked for this package:

- [Grabber site definitions](https://www.bionus.org/imgbrd-grabber/docs/sites/index.html)
  and [engine/source contracts](https://www.bionus.org/imgbrd-grabber/docs/sites/source.html):
  the architectural reference; no Grabber engine implementation is bundled.
- [Gelbooru API wiki](https://gelbooru.com/index.php?page=wiki&s=view&id=18780):
  JSON DAPI search, paging and possible authentication/throttling.
- [Danbooru posts controller](https://github.com/danbooru/danbooru/blob/master/app/controllers/posts_controller.rb),
  [Moebooru post controller](https://github.com/moebooru/moebooru/blob/master/app/controllers/post_controller.rb),
  [e621 posts controller](https://github.com/e621ng/e621ng/blob/master/app/controllers/posts_controller.rb)
  and [Philomena OpenAPI](https://github.com/philomena-dev/philomena/blob/master/openapi.yaml).
- [Grabber's maintained API adapter definitions](https://github.com/Bionus/imgbrd-grabber/tree/master/src/sites)
  provide additional current wire-format compatibility evidence where public
  provider documentation is incomplete. Compatibility is independently implemented
  and tested here; it does not establish support for all their login mechanisms.

Settings use the normal application authentication. `GET /booru-browser/settings`
returns redacted definitions and a revision; `POST` to the same route requires
JSON, that revision, `action` (`save`, `remove`, `reset`) and a `source` definition.
`clearAPIKey` deliberately removes credentials. Stale revisions return 409;
cross-origin browser writes are rejected.

## Design reference

The integrated sidebar, search chips, card actions and live completion approach
use [Codename-11/Stash-Downloader's Stash Browser](https://github.com/Codename-11/Stash-Downloader)
as a reference, adapted to native StashBooru React/Go components. Its copyright
and [MIT license notice](third-party/stash-browser-MIT.txt) are retained. A separate
plugin installation or Python proxy is not required for this browser.

### Artist directory API

`GET /booru-browser/creators?source=<id>&instance=<instance>&q=<name>&page=1`
returns up to 50 matching name/service/ID records per page for a configured Pawchive
or Kemono source. It rechecks source identity after the request. The public provider
directory is bounded to 64 MiB / 500,000 rows, with two indexes cached for ten minutes;
concurrent loads of the same source share a request. Normal public-DNS, redirect,
rate-limit and cancellation rules apply. Invalid records are omitted. No account
favorites or automatic downloads are created by artist lookup.
