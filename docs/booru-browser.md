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

## Search and inspect

Choose up to eight sources, enter a query and select **Search**.
A fresh browser without saved state does not search automatically. Refresh/bare-route
visits restore the saved submitted page. Editing the query and changing source switches
do not submit a search. Each selected source has independent results,
Previous/Next, cancellation and retry. One unavailable/rate-limited source does
not discard another source's results. Inspect a thumbnail or post number to view
the larger remote preview and grouped raw tags; **Open remote post** visits the
source site in a separate tab.

With the rating control at its default, the same query is sent unchanged
to each source. Other rating values append the selected engine's native filter
syntax; Philomena uses its native sort parameter and rating-tag expression. Space-separated tags and
underscores are common to the first four engines; Philomena uses its own syntax,
including tags with spaces and comma/AND combinations. Expand **Search help** in the sidebar for each selected source. Provider ratings, search limits, account filters and metatags
remain provider-specific. Unsupported/rejected queries are errors rather than
being silently simplified to a broader search.

The top search card shows the selected sources above Stash's list toolbar.
The toolbar contains Sources, the editable native expression and removable term
chips, Search, rating/media/autocomplete icons, display operations, Favorites,
saved search filters, page size, Grid/Wall and thumbnail zoom. The sidebar retains
loaded-page tags and source help. Source switches stay open while choosing up to
eight sources. Suggestions carry category/source labels; Enter/Tab completes a
highlighted suggestion and **Ctrl+Enter** submits.

Cards retain only the preview, number, rating, selection and named icon actions.
Artist, Copyright and Character icons prepare a query: a single tag fills Search;
multiple tags open a chooser. Only provider-supplied categories appear; flat tags
are not guessed. The inspector retains all remote metadata and detailed evidence.

**Rating** offers All ratings, Safe / General, Questionable and Explicit on the
original engines. Safe means General on current Danbooru/Gelbooru;
Philomena's Questionable also includes Suggestive. Leave All ratings selected for
native rating expressions. Pixiv, Shimmie and archive adapters currently require
All ratings and provider ordering. The sort selector has been removed; native
order expressions and older supported sort URLs remain available.

**Grid** lays cards out in columns; **Wall** fits their aspect ratios into justified
rows. The native zoom slider resizes cards/rows in four steps. View and zoom are
remembered locally across refreshes. Per-source pagination indexes show the source,
page and available count; unknown totals remain explicitly unknown. The redundant
results header is removed.

## Favorites and saved filters

The star in each card saves/removes that post in **StashBooru Favorites**. The star
in the toolbar opens saved posts from the selected sources. These favorites are
shared by this Stash server; they do not synchronize remote accounts. Each saved
post keeps its approved media references, bounded core taxonomy and gallery attachments.
Snapshots keep up to 64 core tags with names/labels up to 200 bytes; ordinary tags
remain available in live search/inspection rather than the saved projection.
Favorites can be inspected and downloaded after a refresh/restart. Remote files
can disappear or their URLs can expire; return to the provider search to refresh
and re-save changed media. A disabled or changed source is not reopened through
an old favorite. Removing a favorite refreshes that source's favorite listing.

The separate filter icon opens **Saved search filters**. Name and save the current
query/source/filter combination, select a saved name to apply it, or use its trash
icon to delete it. Saving an existing name replaces that filter. Up to 100 named
filters and 500 favorite posts are stored in private `booru-bookmarks.json`, with
an 8 MiB metadata bound and atomic updates. This is separate from ignored/manual
Downloaded history. No remote follow or automated download is created.

## Provider additions

| Source / engine | Search and authentication | Media limitations |
| --- | --- | --- |
| Paheal / Shimmie | Native tags via the Danbooru XML API; anonymous | Categories may be unavailable; no live completion endpoint. |
| Pixiv | OAuth **access token** in source API key; text search, `user:123` for artist works, `id:123` for a work | Static illustration/manga pages selectable in the inspector. Ugoira frame ZIPs are explicitly unavailable. No login/token refresh is implemented; replace expired tokens in Settings. |
| Pawchive | `*` for recent posts, text of at least three bytes, or `creator:service:id`; anonymous API | Optional API key is the `session` cookie value, sent only to `file.pawchive.pw`. Preview-only/deferred/unavailable originals cannot be downloaded. |
| Kemono | Same archive query forms; anonymous API | Supported image/video attachments only; ZIPs/documents are counted as omitted. |

Custom sources can select these engines and configure their HTTPS API root/CDN
hosts in Settings. Preset hostnames can change; failures remain visible for retry
or source configuration. Media is validated through existing size, host, public
DNS and format checks. The adapters use the APIs independently; no Grabber source
code or login flow is embedded.

Pixiv/archive APIs use fixed upstream blocks of 30/50 posts. The browser slices
aligned blocks for the selected page size without skipping records. A gallery is
one card: choose **Media file** in inspection to preview/download a particular
attachment. Keyboard Up downloads the selected attachment. Ignore/Favorite apply
to the post; alternate files do not inherit first-file MD5 evidence. Up to 200
additional attachments per post and 256 media snapshots per result are accepted;
reduce page size if galleries exceed the result bound. Downloads remain one
explicitly chosen file at a time. No text/ZIP import or bulk gallery download is
added. Installed CSV completion remains available on these new engines.

## Display, downloads and inspection

**Media on this page** filters the currently loaded page into Images or Videos.
It does not refetch or repaginate the provider, and totals still include all media.
Missing formats remain unknown rather than being guessed. **Show thumbnails**
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

Open **Settings** in the browser header, or **Settings → Metadata Providers
→ Booru sources**. This single panel combines source/API credentials, downloads, converter preferences
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
