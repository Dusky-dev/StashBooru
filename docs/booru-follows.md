# Following: durable definitions (B06a)

The first Following phase adds a library-wide database/API foundation. It stores
follow intent for provider Artists, Characters, Copyrights/series and exact
provider-native queries. It does **not** add a Following screen or fetch a feed
yet. Browser UI refinements are a separate workstream. B06 remains in progress.

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
a compound query does not drop its series or exclusion terms. This phase does
not execute or validate provider query grammar, compile tag names into queries,
verify the provider's tag category, expand series descendants or map names to
native catalogue entities. A stored Artist target is user intent, not proof of
authorship. A Copyright follow is separate from a local Copyright Favorite.

The same exact `(instance, category, target)` cannot be created twice, including
inactive follows and source aliases. Equal names on different instances and
equal targets in different categories remain independent. At most 500 definitions
can be stored, including inactive records; refollow/edit existing records at the
limit. This initial API retains inactive records rather than exposing deletion.

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

`GET /booru-browser/follows` returns all active and inactive definitions in stable
creation order, with source availability. An empty library returns `[]`.

`POST /booru-browser/follows` accepts one action per request. For example, after
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
Favorites. No action here makes a provider request or starts a download/job.

Invalid input returns 400; stale versions, duplicate targets, unavailable source
bindings and capacity limits return 409 with distinct error codes. Reload the
list after a conflict. Storage failure returns 500 without applying the mutation.

## Migration and next phase

Migration **98** creates only `booru_follows`. Fresh databases and upgrades from
97 are tested, including preservation of native media and existing review marks.
Existing upgrade fixtures from 95/96 also run through the new migration.

Next is B06b: the on-demand Following feed and management UI. It must add durable
post snapshots, revision-specific checkpoints/tie-break IDs, explicit seen state,
duplicate merging with matched-follow explanations, and partial-page failure
recovery before claiming a usable feed. No feed cursor or seen state is invented
by this storage-only phase. Background polling and automatic downloads remain
outside the initial feed scope.
