# Provider identities

This work connects provider Artist, Character and Copyright identifiers to native
StashBooru records. General tags keep their existing behavior. Work is in progress
on the first usable linking/review slice.

## Identity contract

One native Artist/Character/Copyright ID is the permanent local identity. Each
provider tag/account is a separate, source-scoped identifier with an exact target.
For example, after review, Rule34 `waligner`, Gelbooru `waligner_(artist)` and
Pawchive `creator:patreon:49481727` can point to one native Artist. This example
does not independently verify that those accounts belong to the same person.

Keys include the engine/URL instance, category, tag/account form and exact target.
A matching display name is not identity proof. Source aliases share a key only
when their normalized instance matches. Accounts include their service in the
provider selector, so equal numeric IDs on different services remain distinct.
Neither credentials nor signed media URLs belong in these records.

Bindings point directly to native IDs and read current names. Renaming a native
record does not break a connection. Native deletion clears the link and invalidates
outstanding decisions. Linking does not rename/delete source identifiers, append
unscoped native aliases, merge catalogue records, modify general tags or infer
Character/series hierarchy from co-occurrence.

Explicit link/unlink and rejected/allowed suggestions have version checks. Batch
decisions are all-or-nothing, limited to 100 identifiers in one category. Creating
a native record and linking selected identifiers is one transaction. A failure
leaves no partial binding or orphan newly created entity.

## First usable slice

Browse metadata is cached as unresolved provider observations, bounded to 100,000
identifiers. Refreshing those observations never overwrites a connection or
rejected candidate. Name/underscore/category-suffix normalization helps find
candidates but never automatically joins people or characters.

The planned review UI offers linking from post identifiers, native record
creation/selection, batch decisions, remembered rejections, original identifiers
under Sources & aliases, and a scan of locally saved provider metadata. Native
Artist/Character/Copyright text searches also recognize their connected provider
identifiers while retaining native IDs, relationships and curated aliases.

Only saved download receipts and cached feed metadata can be scanned locally.
Source metadata absent from those records cannot be recovered by guessing
filenames or treating a general tag as an Artist. Live cross-provider profile
discovery, automatic media reassignment, native duplicate consolidation and
Following changes are later work.

## API under development

All routes use normal server authentication, JSON/origin bounds for mutations and
no-store responses.

- `GET /identities/list`: category, unresolved/linked state, text, native entity
  or exact identity filters; stable keyset pages of 50.
- `GET /identities/candidates`: bounded native names/aliases/connected identifiers,
  with Character disambiguation and explicit optional rejected matches.
- `POST /identities/update`: observe, link, unlink, create, reject or restore.
  Decisions include identity IDs and current versions; native targets are typed.

Migration 100 adds observations/bindings and rejected suggestions. It does not
rewrite existing media, native entities, general tags or follow intent.
