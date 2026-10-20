# Following: definitions and on-demand feed

B06a adds library-wide follow definitions for provider Artists, Characters,
Copyrights/series and exact provider-native queries. B06b adds the independent
**Following** page and an on-demand saved feed, initially for Danbooru-compatible
sources. Browser UI refinements are a separate workstream. Additional engines
and account/profile providers remain later B06 phases.

## Broader Following scope

The owner clarified on 2026-10-20 that Following also covers accounts/profiles on
Twitter and similar sites. The intended product is one Following feed across
content sources, with booru tag/query follows as its first supported target type.
Twitter and other site adapters can be added to the content browser later; this
phase does not implement or advertise those integrations.

The next feed design must separate shared follow management, snapshots, seen
state and checkpoints from each provider's target resolution and pagination.
Creator/account follows need a distinct target type and stable provider account
identity, with handles/display names as changeable presentation data. Add that
contract through a versioned migration when its adapter ships, preserving current
follow IDs and source bindings. Do not encode a social account as a booru Artist
tag or infer that matching names belong to the same person across sites.

Feed items must allow source-specific metadata: Artist/Character/Copyright tags
are optional capabilities, with absent categories remaining unknown. A future
content-browser adapter should reuse the same provider identity and credential
configuration as Following. Each provider needs its own acceptance checks before
being offered; using an engine-compatible custom booru is separate from adding
a new social-site engine. Following remains local library intent, with downloads
and any remote account-follow action requiring their own explicit action.

## What persists

Each follow has an opaque ID, display name, category, exact target, source engine
and URL fingerprint, original source settings ID, active flag, timestamps and
two counters:

- `version` changes on every accepted mutation. Send the current version to
  prevent an older tab from overwriting another change.
- `queryRevision` changes only when the category or target changes. Renaming,
  unfollowing and refollowing retain it. A future feed must partition its history
  by this revision and choose an explicit cursor policy for a changed target.

Targets retain case, punctuation and whitespace exactly. In particular, storing
a compound query does not drop its series or exclusion terms. Definition storage does
not execute or validate provider query grammar, compile tag names into queries,
verify the provider's tag category, expand series descendants or map names to
native catalogue entities. A stored Artist target is user intent, not proof of
authorship. A Copyright follow is separate from a local Copyright Favorite.

The same exact `(instance, category, target)` cannot be created twice, including
inactive follows and source aliases. Equal names on different instances and
equal targets in different categories remain independent. At most 500 definitions
can be stored, including inactive records; refollow/edit existing records at the
limit. This initial API retains inactive records rather than exposing deletion.

## Using the Following page (B06b)

Open **Following** in the utility navigation. Choose an enabled Danbooru-compatible
source, name the follow, select Artist/Character/Copyright or Provider query, and
enter the exact provider tag or query. Source credentials/custom instances use
the existing Source settings page. Existing definitions for other engines remain
visible, with checks disabled until their feed adapter is supported.

Use **Check new posts** on a follow. Its first successful check saves up to **50
recent matching posts**, establishing a new-upload baseline; it does not crawl
the entire historical catalogue. Every later check fetches one page of newer IDs.
If more remain, **Continue check** resumes the same run, including after a server
restart. New uploads during a pending run are picked up by the next check. Each
follow and query revision has its own cursor. Changing category/target starts a
new recent window; previous matches remain accessible through **Include History**.
Unfollow/refollow without a target change retains the existing cursor.

Checks are explicit: opening/reloading the page, filtering, opening a thumbnail
or visiting a post never polls a provider or starts downloads. **Reload saved
feed** reads the local snapshots and refreshes preview grants. **Browse / Download**
opens the existing browser on the selected remote post for its usual reviewed
download workflow. Neither a cached post nor a manual downloaded mark proves that
the current original exists in Stash.

Posts found through multiple follows appear once per provider instance/post ID,
with their matching follow names and exact targets. Explanations show up to 50
matches (the selected/current follow first), with an explicit notice when more
are omitted. Equal IDs on different sites remain independent. Feed order is
**latest discovered**, using stable local sequence IDs rather than provider
timestamps. Filtering and older/newest page navigation persist in the URL,
including back navigation, refresh and servers hosted under a path prefix.

**Mark seen / Mark unseen** is an explicit, versioned, library-wide decision.
**Unseen only**, **Include History**, **Show ignored** and **Hide marked downloaded**
filter saved snapshots. Ignore/download history remains separate. Unfollowing
does not erase it. **Remove seen snapshots** frees the bounded feed cache, retaining
seen decisions, follow cursors, native media and ignore/download history. If a
different follow rediscovers a pruned post, its seen state is retained.

The snapshot cache holds at most **5000 posts**, with at most **64 KiB** of metadata
per post. A page that exceeds capacity or metadata limits fails atomically;
its posts/matches and cursor do not partially commit. Review and mark items seen,
remove seen snapshots, then retry. History can reveal snapshots retained from
inactive or changed follows. No unseen post is automatically evicted.

Failures retain the resume cursor and last successful check time. Sanitized
failure diagnostics are saved separately, guarded against stale requests, and
shown on each follow after reload. Retry is manual; a 429 includes its retry
delay. Cancel or leaving the page aborts the current request. A page already
committed just before cancellation remains saved and is reflected on reload.

### First provider contract and limits

The Danbooru adapter uses the documented
[sequential ID pagination](https://danbooru.donmai.us/wiki_pages/help%3Acommon_url_parameters)
(`page=bID`) with the original tag predicate unchanged. It verifies descending,
progressing IDs and freezes the run's upper ID while paging toward the prior
completed checkpoint. Only a fully fetched interval advances that checkpoint;
empty/full pages and equal creation timestamps do not substitute for ID evidence.
The same secure transport, host throttles, finite deadlines, response bounds and
server-side credentials as the browser are reused.

Exact single-tag follows reject query operators/wildcards; use Provider query
for compound predicates. Feed queries cannot contain sorting, page, random or
limit controls. They retain all Artist/series/exclusion terms. Provider access
restrictions and search limits still apply. The feed detects newly uploaded
matching IDs, not later votes/tag edits on older IDs. Dynamic searches may change
membership between pages; sequential IDs prevent head-insertion page shifts but
do not provide an immutable provider snapshot. Saved metadata is labelled with
its retrieval time and may differ from a current/edited/deleted remote post.

Live provider access is separate from fixture acceptance. The 2026-10-20 public
testbooru smoke attempt could not reach the endpoint before the deadline; live
authentication and custom-instance acceptance remain unverified. Gelbooru,
Moebooru, e621, Philomena, Shimmie and creator/account services need their own
pagination and acceptance checks before feed synchronization is enabled.

## Source changes and credentials

Creation binds to the enabled source's current `instance` from
`GET /booru-browser/sources`. The engine/URL binding is immutable. A different
site configured under the same display ID cannot inherit the follow. Listing
reports `available`, `source-changed` or `source-unavailable`, plus the current
`resolvedSource` when an enabled alias matches the exact instance. Restoring the
original source recovers availability; credentials can rotate without changing
identity. No account name, API key or cookie is copied to the follow table.
If source settings cannot be read/parsed, the stored list still loads with
`configuration-error` availability and its current IDs/versions. A client can
reload and unfollow without repairing settings first.

The first phase accepts Danbooru, Gelbooru-compatible DAPI, Moebooru, e621,
Philomena and Shimmie source engines. Pixiv, Pawchive and Kemono creator-service
follows need their own identity/query contract and remain deferred. Accepting a
definition does not claim that a provider exposes a complete Artist or series
profile, or that a query will pass the provider's search limits.

## API

Routes use the server's normal authentication. Mutations require JSON and the
existing same-origin checks. Responses are `Cache-Control: no-store`.

`GET /following/follows` returns all active and inactive definitions in stable
creation order, with source availability. An empty library returns `[]`.
It also returns `syncSupported` and current-revision `feed` diagnostics/progress.
`/booru-browser/follows` remains a GET/POST compatibility alias.

`POST /following/follows` accepts one action per request. For example, after
reading Danbooru's current instance fingerprint from the sources endpoint:

```json
{
  "action": "create",
  "source": "danbooru",
  "instance": "<current instance fingerprint>",
  "name": "Series S",
  "kind": "copyright",
  "target": "series_s"
}
```

`kind` is `artist`, `character`, `copyright` or `query`. Display names are limited
to 120 UTF-8 bytes and targets to 1024. Blank targets, invalid UTF-8 and control
characters are rejected. Create returns the full definition with `active: true`,
`version: 1` and `queryRevision: 1`.

Update supplies the complete display definition and the returned ID/version:

```json
{
  "action": "update",
  "id": "<returned ID>",
  "version": 1,
  "name": "Renamed series",
  "kind": "copyright",
  "target": "series_s"
}
```

Unfollow/refollow accept only action, ID and current version:

```json
{"action": "unfollow", "id": "<returned ID>", "version": 2}
```

Use `refollow` to reactivate that record. Update/refollow require its original
instance to be available; unfollow works even if the source is disabled or the
source settings file is broken. Unfollow retains identity and version counters.
It never clears ignored/downloaded marks, removes local files or changes native
Favorites. Definition actions never make a provider request or start a download/job.

Invalid input returns 400; stale versions, duplicate targets, unavailable source
bindings and capacity limits return 409 with distinct error codes. Reload the
list after a conflict. Storage failure returns 500 without applying the mutation.

## Migration and next phase

Migration **98** creates only `booru_follows`. Fresh databases and upgrades from
97 are tested, including preservation of native media and existing review marks.
Existing upgrade fixtures from 95/96 also run through the new migration.

Migration **99** adds provider-independent snapshot payloads, match provenance,
opaque adapter cursors and explicit seen decisions. It preserves B06a definitions
and native/review identities. Snapshot pruning does not delete decision tombstones.

The feed endpoints use the same authentication, origin checks, JSON bounds and
no-store responses:

| Endpoint | Contract |
| --- | --- |
| `GET /following/feed` | `limit=1..50` (default 25), `before` local sequence cursor, optional `follow` ID, `unseen=1`, `history=1`, `ignored=1`, `downloaded=hide`. Returns `items` and `next`; no remote requests. |
| `POST /following/sync` | `{ "id": "<follow ID>", "version": 1 }`; saves one bounded page plus its checkpoint atomically. Returns progress, matched count and notice. |
| `POST /following/seen` | `{ "sequence": "<feed sequence>", "version": 0, "seen": true }`; stale decisions return 409. |
| `POST /following/prune` | `{ "action": "remove-seen-snapshots" }`; removes only seen snapshots/matches, retaining decisions, cursors and native media/history. |

Next is adapter coverage beyond Danbooru, then stable account/profile targets for
Twitter and similar sources under the broader contract above. Native entity
mapping and the reviewed Artist × Copyright builder remain B03. Background polling,
automatic downloads and retrospective edit/deletion synchronization are outside
this initial on-demand new-upload feed.
