# Booru browser — B01

Open **Booru** (globe) in the navbar, or `/booru`. This read-only browser searches
remote posts and displays their previews and metadata. It does not download
originals, import entities, change the catalogue or check existing local copies.
Ignored/downloaded marks, metadata import, reviewed downloads, pHash comparison
and follows remain later [roadmap packages](booru-browser-roadmap.md).

## Search and inspect

Choose up to eight sources, enter a query and select **Search**.
Opening the browser without a query, editing the query and changing checkboxes
do not search automatically. Each selected source has independent results,
Previous/Next, cancellation and retry. One unavailable/rate-limited source does
not discard another source's results. Inspect a thumbnail or post number to view
the larger remote preview and grouped raw tags; **Open remote post** visits the
source site in a separate tab.

The same query is sent unchanged to each source. Space-separated tags and
underscores are common to the first four engines; Philomena uses its own syntax,
including tags with spaces and comma/AND combinations. Expand **Search help** in the sidebar for each selected source. Provider ratings, search limits, account filters and metatags
remain provider-specific. Unsupported/rejected queries are errors rather than
being silently simplified to a broader search.

Query, selected sources, per-source page and inspected post identity are in the
URL, supporting refresh/back navigation. This is a live listing, not a persistent
snapshot: new/changed remote posts can shift page boundaries. Results are grouped
by source, de-duplicated by post ID within each returned page, and retain distinct
source/post identity across providers. There is no cross-source or cross-page
deduplication yet. Totals are unknown except for provider-reported Philomena
totals; these are not verified snapshot totals. Numbered browsing ends at page
750; narrow the query to continue.

Preview bytes come directly from the configured provider/CDN to the browser,
without a StashBooru media proxy. A missing/blocked/unsupported preview has a
fallback message and remote link. Originals are never substituted for missing
thumbnails. Dimensions, byte size, rating and MD5 are provider metadata; unknown
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

These are API adapters and presets, not a claim of complete Grabber site/login
parity. Shimmie, Szurubooru, Sankaku, Pixiv, HTML scraping, executable source
plugins and browser-session login are not implemented. Compatible instances of
the five engines above can be added in **Source settings**. More engines
need their own documented adapter and the same acceptance checks.

## Source settings

Open **Source settings** in the browser header, or **Settings → Metadata Providers
→ Booru sources**. These settings apply to everyone using this server.

- **Edit** a preset to add credentials, change its CDN hosts or disable it. Use the
  username + API key for Danbooru/e621, user ID + API key for Gelbooru JSON DAPI,
  or API key alone for Philomena. Moebooru remains anonymous.
- **Add custom booru**: choose a stable source ID, display name, supported API
  engine and public HTTPS site URL. Include an installation path if needed. Add
  separate image/CDN hostnames when previews live outside the site host. Choosing
  an engine means the site must implement that engine's supported JSON API.
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

The browser keeps search/sources and categorized page tags in a left sidebar,
with compact thumbnail rows on the right. On narrow screens controls stack above
a two-column grid and page tags expand with **Show page tags**. Category labels
accompany category colors. Counts cover only the selected source's loaded page,
with up to 20 tags per category; these are not provider-wide related-tag counts.
Selecting a tag prepares a query; press **Search** to run it. Independent source
paging, inspection URLs and raw provider syntax retain their previous behavior.
These definitions configure the remote browser; the existing local-media MD5
lookup provider list is unchanged.

## Server configuration file

The optional **`booru-browser.json` beside `config.yml`** contains an array of
source overrides/additions. Missing file means the ten presets. A complete entry
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
