# Character identity cleanup and Copyright task auto-tagging

The owner requested automatic cleanup of reversed and shortened Character names
without combining unrelated namesakes, plus the missing Copyright task selector.
This package starts at develop `f5579d46940dcc8ad6a96d69c264908f36be85d6`.

## Use

Open **Settings → Tasks → Merge duplicate Characters**. Preview the proposed
survivors, source Characters and native Copyright links. Groups and skips are
paginated. Applying runs all eligible groups in the native job queue; cancel there
before completion to roll back the batch. A failed job remains available with its
error. Changed catalogues require a fresh preview. No merge runs on scan/startup.

Names and Copyrights are evidence rather than proof of identity: review each group.
To exclude a proposed group, correct its identity/context or use native manual
merging instead. Completed merges remove source Character records; undo is not
provided. Their media files and media IDs remain unchanged.

## Matching contract

- Normalize Unicode NFKC, case, whitespace, underscores and comma separators.
- Full names with two tokens can appear in either order. Longer names must match
  in order. Single-word names are not merged with other single-word names.
- A shortened name must be one complete token of at least three Unicode characters
  and match exactly one full-name identity in the same Copyright scope. Initials,
  nicknames, arbitrary substrings and fuzzy spelling are not inferred. Distinct
  full names cannot become connected through a shared short name.
- Both records need the same nonempty set of directly assigned Copyright IDs.
  Copyright names, shared ancestors and partially overlapping sets are insufficient.
- Character variants and their parents are excluded. Ineligible full names still
  count toward short-name ambiguity. Conflicting typed Copyright/Artist context,
  disambiguation, populated scalar/custom fields, Stash IDs at the same endpoint,
  or different nonempty portraits require manual review.
- Prefer the full name, then lowest ID. Copy missing compatible scalar fields;
  union existing aliases, URLs, Tags and Stash IDs; retain favorite/ignore-auto-tag
  flags and earliest creation time. Transfer Image, Video and Gallery links.
  Direct Copyrights already match. Removed canonical names are stored as
  Copyright-qualified aliases to avoid introducing broad short-name auto-tagging.
- Preview hashes all Character identities/context/profile/related values and portrait
  digests. Apply reloads and revalidates inside one write transaction. An error or
  cancellation before commit rolls back the entire batch. Media link edits need
  no stale rejection because the latest native links transfer at apply time.
- The first version supports up to 10,000 Characters per library and aborts above
  that limit without writing. Portrait digests bound retained image memory. The
  preview request is cancellable by navigating away; applying is a queued job.

## Copyright task option

`AutoTagMetadataInput` and saved `AutoTagMetadataOptions` accept `copyrights`:
`["*"]` for all, an array of native IDs for selected entities, or omitted/empty
for none. IDs are validated before other task passes begin. Copyright names and
aliases are matched using existing path-based tagging with selected-path limits.
Links are additive and native to Images/Videos; no synthetic Tags or Gallery
Copyright associations are created. The Tasks selector persists with other task
options. Existing defaults lacking this field retain previous behavior.

## Verification

Focused matching, real SQLite preservation/rollback, stale-preview/API job,
Copyright task path/alias selection and legacy-default regression tests accompany
the implementation. Mounted browser fixtures exercise desktop/mobile review,
pagination, apply/error recovery and Copyright task input. They contain synthetic
data only; no production catalogue merge or external service is involved.

Browser command (from `ui/v2.5`):

```sh
PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs \
CHROMIUM_EXECUTABLE=/path/to/chromium node tests/browser/character-dedup.mjs
```
