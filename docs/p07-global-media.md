# P07 native global media

The top-level All navigation entry opens `/media`. Images and Videos keep their
existing `/images` and `/scenes` pages beside it in the navbar. All has no second
media-type tab row. Legacy `/media/images` and `/media/videos` links redirect to
the native pages and retain their query parameters and history state.

All is included in the backend's default navigation. A customized menu can enable it in
Settings → Interface → Menu items.

## Query and identity

`findMedia(media_filter, filter)` returns a total, Image/Video counts and an
ordered page of `MediaItem` records. Each item has one native Image or Scene and
a typed key such as `image:123` or `scene:123`. Selection retains that key until
an action dispatches the native numeric ID to the appropriate API.

Both native filters select matching IDs in the same read transaction. One SQL
`UNION ALL` applies a shared sort and page boundary. Hydration reads only the
selected native records. The count query projects only identities; path/file
lookups run only when required by the requested sort.

Supported sorts are date, title, rating, path, file size, created and updated
timestamps. The page defaults to date descending; an API request without a sort
uses created ascending, consistent with native `FindFilterType` defaults. All
sorts end with kind ascending (`IMAGE` before `VIDEO`) then numeric ID ascending.
Missing values sort last in both directions; empty titles/dates are missing.
Titles and paths use native natural, case-insensitive collation. File size/path
refer to the native primary file; records without one have missing values.

Page sizes range from 1 to 500 (API default 25). Unbounded pages and unsupported
sort names are rejected. Counts describe all matches, including past-end pages.
Each catalogue entry remains one row, including animated Images. These typed
identities remain suitable for future P08 stack members.

## Filters and view state

Text search, title, details, path, rating, dates/timestamps, organized status,
favorite Characters, Tags, Characters, Artists and Copyrights use their existing
native predicates. Hierarchy depth and P06 inheritance retain native semantics.
Any duration criterion, including
`IS_NULL`, excludes Images; the page shows that rule while the criterion is active.

Shared criteria, sort, page size, Grid/Wall mode and zoom are preserved in the
URL/native filter state. Saved All filters store shared criteria. Selection carries
across pages. Eight in-memory history snapshots retain selection and scroll for
back navigation during the browser session; reloading clears those snapshots.

The page renders at most 500 native cards/wall items at once. Image previews and
Video queues follow native behavior using the current page's respective kinds.
Animation badges are native shared components; they do not classify a preview
video as a separate Scene.

## Selection actions

| Action | Supported selection |
| --- | --- |
| Edit metadata | Images and Videos: title, details, date, rating, organized status, primary Artist, Character links and Tags. |
| Convert | Images and Videos, dispatched as native `image`/`scene` targets. |
| Upscale | Images; selected Videos are excluded, counted in the action explanation and shown inside the dialog. Disabled if no Images are selected. |

Mixed edits use two native transactions with the existing hooks and validation.
Each type's result is reported separately. If one succeeds and the other fails,
the editor locks the submitted fields and retries only the failed type. Closing
after partial success refreshes the list and clears the completed selection.

## Verification

Fresh SQLite integration tests check alternating dates across page boundaries,
both directions, ties/nulls, natural text/path sorting, file-size sorting,
type counts, shared filters, inherited profile Tags/hierarchies, favorites,
duration semantics, typed GraphQL payloads, native bulk writers and query bounds.
UI tests check equal-ID selection, deduplication, conversion target identity,
native mutation dispatch, empty selections and partial failures.

The full backend/UI checks and production builds passed. A running isolated
application also passed the actual UI query/fragments, filtered counts, native
metadata writes and new/existing list-route HTTP smoke checks.

P07 was merged in [PR #119](https://github.com/Dusky-dev/StashBooru/pull/119);
its Build and Go lint workflows passed. Navigation corrections were merged in
[PR #120](https://github.com/Dusky-dev/StashBooru/pull/120).
The native All list rework was merged in
[PR #121](https://github.com/Dusky-dev/StashBooru/pull/121).

The All list now follows the native Images/Videos structure: saved filter sidebar,
toolbar and operations menu, cached pagination, loaded list body and native
cards. Lightbox synchronization mounts inside the loaded body. Previously it
mounted while a query was loading, and fresh empty arrays caused React's update
depth limit to fail. Wall uses the native Video styling and preview source
selection, safe dimensions, bounded row height and native Video queue links.

The optional mounted-browser regression suite covers delayed, empty, Video-only
and failed queries, mixed Grid selection, Wall navigation/back and absence of
duplicate type tabs. Run it against an isolated instance containing Images and
Videos with Playwright available:

```sh
STASH_BROWSER_URL=http://127.0.0.1:9999 node ui/v2.5/tests/browser/all-media.mjs
```

`PLAYWRIGHT_MODULE` can specify an absolute Playwright module path and
`CHROMIUM_EXECUTABLE` a browser executable. This suite injects delayed/error
responses only inside its own browser; it does not modify library records.
Unit regressions also cover missing/failed preview sources and invalid dimensions.

## Character editing, relation cards and Video previews

The existing Character/Artist/Tag All enhancement now deactivates its inline
pane when editing removes the native tab navigation. Its media contents unmount;
cancelling restores the native tabs with All inactive.

Copyrights and Variants sit together at their content width with a one-rem gap.
Their images keep their natural proportions with `object-fit: contain`, capped
at the smaller of 16 rem and 40% of viewport height. Names wrap. Each relation
list grows with its contents up to the smaller of 24 rem and half the viewport
height, then scrolls internally; no records are hidden by line clamping.

Video cards again show the magnifying-glass preview button outside selection
mode. Scoped All uses its existing mixed preview carousel; global All and Videos
use the shared native ScenePlayer preview host and current mounted Video cards.
An Image preview in global All explicitly restores its Image list after a Video
preview closes. The player's queued marker setup exits if navigation already
disposed the player.

The Chromium suite checks Video previews and carousel navigation on global All
and native Videos, then opens an Image after closing a Video. The Character
suite also checks scoped All previews, editing while All is selected, portrait
and landscape card dimensions, 18-item relation lists and mobile overflow:

```sh
STASH_BROWSER_URL=http://127.0.0.1:9999 STASH_BROWSER_CHARACTER_ID=123 \
  node ui/v2.5/tests/browser/character-media.mjs
```

Use an isolated Character linked to an Image, Video, Copyright and direct
Variant. This suite substitutes relation images/counts only in its own browser
and makes no metadata writes. `STASH_BROWSER_SCREENSHOT_PREFIX` optionally saves
desktop and mobile captures. All 28 UI tests, TypeScript, lint, formatting, the
built-in checksum/syntax checks and production build passed with these changes.
Real-file playback and external processing services remain outside these
synthetic-fixture browser checks.
