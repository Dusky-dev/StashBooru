# P07 native global media

The top-level All navigation entry contains All, Images and Videos tabs. Their
routes are `/media`, `/media/images` and `/media/videos`; existing `/images`,
`/scenes` and native detail links remain usable.

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
The route supplies the media-type constraint. Any duration criterion, including
`IS_NULL`, excludes Images; the page shows that rule while the criterion is active.

Shared criteria, sort, page size, Grid/Wall mode and zoom are preserved in the
URL/native filter state. Saved Media filters store shared criteria; the current
tab supplies the type constraint when they are applied. Selection carries
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
its Build and Go lint workflows passed. Browser rendering/interaction remains
unverified because Chromium's download failed. Grid/Wall previews, mixed
selection/dialogs, filters across tabs and detail-page back navigation with
scroll/selection still need browser verification. No migration is added.
