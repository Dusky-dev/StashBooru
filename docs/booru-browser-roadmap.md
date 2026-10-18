# Booru content browser: delivery plan

Status: **B01–B02 implemented on the current task branch; validation is recorded in the progress ledger. B03–B06 planned.** Baseline: `develop`
`598d6849668303369fb2a2c5783160a00172ba59`, reviewed 2026-10-17 UTC.
This is the next roadmap after the delivered P01–P13 scopes. The owner broadened
B01 on 2026-10-17 to an engine-based browser inspired by Grabber. B01 searches
several remote sources; B02 adds durable review marks and verified current-file
matches. Follows and downloads remain later packages. See
[browser operation and source configuration](booru-browser.md).

The owner requested a content browser, ignored/already-downloaded marks, pHash
groups with Compare, tag differences with disfavored tags in red, following
Artists/series, and metadata/download selection across Artists and Copyrights.
Implement one package at a time, complete its acceptance checks and update the
[progress ledger](implementation-progress.md) before the next package begins.

## Existing foundations to extend

| Foundation | Current source | Reuse and boundary |
| --- | --- | --- |
| MD5 lookup/provider parsing | `internal/api/routes_image_booru.go`, `routes_scene_tagging.go` | Existing local-media metadata lookup. Search, paging, credentials, remote media URLs and downloads need new contracts; do not change existing lookup behavior incidentally. |
| Reviewed metadata application | `internal/api/tagging_change_plan.go`, `routes_image_tagging_priority.go`, `ui/v2.5/src/components/Tagging/TaggingChangePlanModal.tsx` | Reuse ambiguity resolution, native entity IDs and review conventions; remote snapshots are a distinct metadata source. |
| Image matching | `pkg/sqlite/phash_similarity.go`, `image_embedding_similarity.go` | Separate current-file pHash evidence, embedding evidence and historical source fingerprints. |
| Still-image comparison | `ui/v2.5/src/hooks/Lightbox/ReferenceComparison.tsx`, `imageDifference.ts` | Extend the shared viewer with a metadata panel; keep current still/animation/preview limitations. |
| Durable grouping | `docs/p08-visual-stacks.md` | Catalogue stacks are explicit user edits. Remote review groups must not silently create stacks. |
| Artist/Copyright favorites | Native Studio/Copyright records and their favorite controls | Local favorites exist; provider follows, feed checkpoints and provider-tag mappings do not. |
| Jobs and file ownership | Native manager Jobs, scan/import, conversion staging/recovery | StashBooru owns activation/database writes. Inference workers do not become download managers. |
| Diagnostics/triage | `internal/api/metadata_health.go`, `similarity_clusters.go`, Settings Tools | Existing EVA02 triage is distinct from the requested pHash review groups. |

## Delivery packages

| Package | Usable outcome | Dependency |
| --- | --- | --- |
| B00 — documentation and scope | Current feature guides and this staged contract. | Current branch; documentation only. |
| B01 — read-only browser | Search selected sources through five API families, page each independently, inspect tags/previews and open remote posts. Ten presets, editable credentials/custom instances and a compact sidebar/grid. | B00. Broadened by owner direction; see [B01 guide](booru-browser.md) for scope and limits. |
| B02 — persistent review marks | Ignore, undo, show ignored, manually mark already downloaded, and display verified local matches separately. See [B02 guide](booru-review-state.md). | B01 and its stable source/post identity. |
| B03 — metadata and selection review | Import supported metadata and select Artist × Copyright intersections before downloading. | B01–B02; reuse native metadata review. |
| B04 — reviewed downloads | Download the reviewed bounded selection into the library, with cancellation, retries and verified local links. | B03; native scan/import and recoverable staging. |
| B05 — pHash groups and tag comparison | Review similar images, Compare, inspect shared/unique tags and red disfavored-tag warnings. | B02–B04 for stable remote/local identity; reuse P02/P03/P08. |
| B06 — follows and feed | Follow Artists and Copyrights/series, including compound searches; view new posts and resume feeds. | B01–B04; each additional engine needs B01 adapter acceptance checks. |

Do not implement all rows in one PR. A package can have a backend/API PR followed
by its UI PR, but remains in progress until its user workflow and acceptance
checks are complete. Preserve P01–P13 status and open validation/production gates.

## Identity, persistence and provider behavior

Use `(provider instance, remote post ID)` as remote identity, separate from native
`image:ID` / `scene:ID`. Store the provider's original post URL and typed raw tags,
plus an explicitly versioned normalization for display/matching. Same names on
different providers are not automatically the same Artist or Copyright. Use
reviewed provider-tag → native-entity mappings; aliases and display names are not
foreign keys. An absent tag category stays unknown until resolved.

Keep a bounded remote snapshot with retrieval time, version/fingerprint, supported
file URLs, media type, dimensions, size/hash when available, and provenance. A
missing field is unknown, not an empty value asserted to be authoritative.
Remote metadata refresh must not overwrite curated local fields without review.

Provider adapters advertise capabilities: authentication, tag categories, aliases,
profile/wiki metadata, ordering, cursor/page semantics, search limits and media
availability. First inspect current official documentation and actual responses.
Do not promise identical features across providers. A rejected multi-tag query,
rate limit or unavailable total must be visible; never broaden the user's query
silently or label a partial page as all results.

Keep credentials server-side and redact them from URLs/logs. Use the application's
authentication boundaries, bounded timeouts, response/download limits, cancellable
requests, per-provider concurrency/rate limits and finite retry with backoff.
Any server-side media fetching must validate provider/media hosts and redirects,
reject private/loopback destinations, and treat filenames and metadata as untrusted.
This is a concrete requirement for a remote fetch feature, not a new arbitrary-URL
proxy. Use a new migration from current source for durable state; verify fresh
databases and upgrades and preserve existing native IDs/relationships.

Initial persisted review/follow state is library-wide, consistent with the
current self-hosted catalogue. Do not imply private per-user feeds without an
actual user-scoping implementation. Source, query and visible filters belong in
URL state; browser session preferences must not be the only copy of review marks.

## B01 — read-only browser

Add a native **Booru browser** entry without altering the All/Images/Videos list
ownership or their established navbar order. Use existing themed cards, filters,
loading/error states and accessible preview controls. Choose a provider, submit
tags, browse previews and inspect source tags plus the remote post link. Clearly
distinguish remote previews from imported local media. Opening search must not
create catalogue entities, download originals or start inference.

The initial adapters are Danbooru, Gelbooru JSON DAPI, Moebooru, e621 and Philomena,
following the owner's broader-source direction. Keep per-source paging/errors
and native query syntax explicit; do not claim Grabber-wide engine/login parity.
Use supported stable ordering and bounded pages,
retain the query when returning from a preview, and cancel obsolete searches.
Record total semantics as exact, provider estimate or unknown. If an underlying
page API shifts while new posts arrive, de-duplicate seen IDs and expose that
limitation rather than claiming snapshot isolation. Unsupported media can still
link to its remote post with an explanation.

**Acceptance:** parser fixtures for ordinary/missing/malformed fields; empty
results; auth/403/429/timeout/retry states; safe URL handling; search changes while
requests are pending; deterministic fixture pagination without duplicate IDs;
URL refresh/back navigation; desktop/mobile keyboard and preview cleanup. Run a
small explicit live provider smoke check where access permits, recording its
date and limitations separately from mocked tests. Keep existing MD5 lookup tests
passing. No public provider credentials are required in CI.

The owner's B01 follow-up adds **Source settings** in the browser and Metadata
Providers settings: write-only keys, custom compatible instances, disabled
sources and conflict-safe atomic saves. The browser uses a Danbooru-inspired
search/tag sidebar and compact thumbnails. These settings still target the same
five adapters; B02–B06 remain separate packages.

## B02 — ignored and already-downloaded state

Persist separate dimensions rather than one overloaded status:

| State | Meaning / action |
| --- | --- |
| Ignored | Explicit review decision for this provider/post. Reversible; hidden only when the filter requests it. Does not delete remote or local media. |
| Marked already downloaded | Manual historical claim, with time/provenance. Does not fabricate a native media link or prove current local bytes exist. |
| In library | Verified link to existing typed native media, with matching evidence and current file identity. |
| Queued / downloading / failed | Job outcome tracked separately; failed/cancelled requests are never marked successfully downloaded. |

Show the state reason, source post and any native links. Permit bulk mark/undo
within a reviewed selection. Clearing ignore must not clear download history.
Deleting local media or replacing its file invalidates current match evidence
without discarding historical download records. Exact source-post linkage or
verified current-file checksum can support a local match. A filename MD5,
`source_md5` or pHash alone cannot prove that the current original is present.
Cross-provider/hash-wide ignore is a separate explicit action, not the default.

**Acceptance:** persisted marks survive app/server restart; equal post IDs on two
providers stay distinct; concurrent changes reject stale updates; bulk undo and
show-ignored work; manual marking never invents local media; deletion/file changes
refresh current local evidence; review filters do not break pagination/count labels.

## B03 — metadata import and Artist × Copyright selection

Support both metadata-only import/update and a reviewed media selection that B04
can download. Keep Artist, Character and Copyright categories native. Preview
provider-supported names, aliases, URLs, descriptions and relationships with
source and retrieval time; identify unsupported fields explicitly. A post's tag
list does not imply that a provider exposes complete Artist/Character profiles.
Do not create authoritative hierarchy or authorship relationships merely because
tags co-occur on a post. Reuse ambiguity resolution and preserve curated values.

The selector must express the owner's examples without leaking other series:

| User selection | Required predicate |
| --- | --- |
| Artists A and B, only series S | `(artist A OR artist B) AND copyright S` |
| Artist A, series S and T | `artist A AND (copyright S OR copyright T)` |
| Several Artists and series | `(any selected Artist) AND (any selected Copyright)` by default; a separate explicit control can require all within a dimension. |
| All Artists found for series S | Discover candidate Artists within the bounded S search, review/select them, and retain `copyright S` when fetching posts. |
| All series found for Artists A/B | Discover candidate Copyrights within the bounded A/B search, review/select them, and retain the Artist constraint when fetching posts. |

Also allow optional Character constraints and excluded tags. Display the resolved
predicate in plain language and review the Artist/series pairs, remote IDs and
available/unknown counts before proceeding. An empty selection yields no work;
it must never become all Artists or the entire provider.

Resolve provider limitations by bounded query decomposition and union/intersection
of typed post IDs, retaining every selected dimension and exclusion. De-duplicate
multi-Artist/multi-Copyright posts. Show limits and incomplete discovery; “all
Artists” means all candidates in the declared completed scope, not an unverified
global enumeration. Changed selections/snapshots invalidate the prepared review.

**Acceptance:** fixtures distinguish A/S, A/T, B/S and unrelated C/U; multi-tag
posts occur once; the two example queries return only the intended intersections;
empty selection returns none; exclusions survive query decomposition; unsupported
syntax never broadens scope; ambiguous names stay unbound; repeated metadata import
is idempotent; curated data and native Copyright identities survive.

## Browser usability slice — 2026-10-17

At the owner's request, thumbnails, explicit downloads to the user's computer,
optional JPEG → JPEG XL conversion and per-source CSV database autocomplete were
prioritized before B03. See [the guide](booru-downloads-autocomplete.md). Native
Jobs, the existing converter and generated-download store are reused. These
exports neither import catalogue metadata nor create in-library links/marks;
they do not complete B03/B04's durable catalogue import acceptance criteria.

The 2026-10-18 correction adds explicit single-post native scan/import, default and
custom library folders, persistent receipts/retry, and saved converter defaults.
B03 entity/tag review and B04 bounded batches/restart automation remain planned.

## B04 — downloads and catalogue import

Review original-vs-preview choice, destination, number of files, known bytes and
unknown sizes. Only explicit selected posts or a bounded reviewed query snapshot
may enter the queue. Do not use a moving “everything matching now” query as a
silent substitute. Following a source does not authorize automatic downloads.

Use native cancellable Jobs with per-item status/retry. Validate media response,
size/type, expected checksum where provided and the actual stored checksum.
Stage safely, choose collision-free paths, durably record activation/import state,
then register through native scan/import and apply the reviewed metadata. A retry
must not create a duplicate file/entity. An interrupted file or failed catalogue
registration remains recoverable and cannot be marked in-library prematurely.
Preserve source URLs/post IDs, raw metadata provenance and current-file identity.
Unsupported or unavailable originals report a reason; do not substitute previews
without the explicit choice shown in review.

**Acceptance:** cancellation/cleanup, disk full, truncated/HTML responses, hash
mismatch, unsafe redirects/paths, repeated jobs, name collision, registration
failure and restart at each activation boundary. Verify metadata/relationships,
exact stored-byte identity, only completed imports linked in-library, and an
existing local exact match avoids a redundant download with its evidence shown.

## B05 — pHash groups, Compare and tag warnings

Add **Group similar (pHash)** to the review workflow, with a distance threshold,
missing/indexing states and a **Compare** action for a selected pair or an anchor
and member. Display exact-file evidence and raw pHash distance separately from
EVA02 related-content evidence. pHash generation is an explicit bounded job;
provider metadata must not be assumed to include compatible hashes.

Remote thumbnail hashes are preview evidence only. Record the actual fetched
asset, checksum, dimensions and hash algorithm/version. A changed asset invalidates
its fingerprint. Compare compatible hashes, never a historical original hash to
an active derivative as if both described current bytes. Do not automatically
download every original merely to calculate search-page hashes.

Group before pagination with stable ordering and explicit group/member totals.
Use anchor/pair evidence; A≈B and B≈C does not imply A≈C. Render per-pair distances
and unverified relationships. Expanding a group retains independent IDs, selection
and local/remote state. Grouping does not merge files, delete variants or create
catalogue stacks without a separate reviewed action.

Extend shared comparison with shared tags, left-only tags and right-only tags,
separated into Artist/Character/Copyright/general/meta categories. Preserve raw
provider names, mapped native IDs and provenance; do not equate entities just
because normalized names match. Show metadata age and unavailable categories.

User-editable disfavored-tag rules can include provider spellings such as
`censored` and `lowres` (displayed as “low resolution”). Highlight matched tags in
red **and** use a warning icon/text and an accessible label. A rule records the
provider/category/alias mapping and reason; no arbitrary hardcoded aesthetic
score decides a keeper. Report actual dimensions/bytes separately. A tag is a
provider assertion or user preference, not independent pixel-quality verification.
Keep filters/exclusions distinct from warning-only rules so highlighting a tag
does not silently hide a candidate. No automatic deletion/download replacement.

**Acceptance:** labelled identical/resized/edited/unrelated fixtures; missing and
stale hashes; non-transitive A/B/C group; filter/page stability; matching hash but
different bytes; shared/unique/conflicting tags; explicit alias rules; color-blind
and keyboard access; long tags; cross-provider equal names; preview/full-image
labels; desktop/mobile comparison and cancellation cleanup. Retain current P03
bounded still-image limits and avoid whole-animation equality claims.

## B06 — follows and favorite searches

Follow a provider Artist, Copyright/series, Character or a saved compound query.
Series follows refer to native Copyright mappings; descendant expansion is an
explicit choice. A compound follow retains the B03 intersection, so following
Artists A/B within S never becomes all posts by A/B. Keep **Follow** separate from
existing local **Favorite** unless the user explicitly opts into a reviewed mapping.

Start with an on-demand Following feed. Persist source-specific checkpoints,
seen/unseen decisions, query revisions and tie-break IDs. Merge and de-duplicate
posts found through several follows, explaining which follows matched. Unfollow
retains download/ignore history. Do not advance a checkpoint past failed or
unfetched pages; changing a query requires a new cursor policy. Provider edits,
deletions, unavailable totals and partial sync errors remain visible. Any later
background polling needs bounded schedules and rate handling; it is not required
for the initial usable feed and does not trigger downloads by default.

**Acceptance:** repeated sync/new-post insertion without losing or duplicating
items; equal timestamps; a post matching several follows; partial failure/retry;
restart; query change; rename/mapping change; unfollow/refollow; ignored/downloaded
filters; explicit mark-seen; feed back navigation and mobile access. Re-run the
B01 adapter contract suite before adding another provider.

## Completion record for every package

Record baseline/head, focused scope, migration/API impact, actual checks and their
results, PR/patch, limitations and the next package. Use the existing unit/UI/build
gates appropriate to the change. Provider fixtures do not prove live auth/search;
synthetic browser fixtures do not prove real-media decoding. Publish a focused
reviewable branch; do not push directly to develop or merge automatically.
