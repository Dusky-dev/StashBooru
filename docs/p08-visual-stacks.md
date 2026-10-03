# P08 visual stacks and variant filmstrip

Visual stacks group native Images and Videos in the catalogue. A stack does not
change file bytes, fingerprints, metadata, native relationships, file activation,
restore records, or gallery membership/order. Every member retains its native
`/images/ID` or `/scenes/ID` URL. A representative is a display choice, never a
compression base.

## Actions and ownership

- Create from 2–200 selected members on All, Images or Videos. Typed keys keep
  `image:1` and `scene:1` separate. Each member can belong to only one stack.
- Manage from a grid badge, member detail or preview filmstrip: add a native
  member URL, remove membership, reorder, edit optional relationship labels,
  choose a representative, split, merge, or unstack all.
- Split selects at least two members and leaves at least one behind. Order and
  labels survive. Merge explicitly reviews all members of the other stack;
  the first stack's identity survives. Save pending edits before split/merge.
- Removing/deleting the representative selects the first surviving member.
  Singleton survivors remain navigable; deleting the last member cleans up
  the empty stack. Unstack all deletes only grouping and never media/files.
- Optimistic versions reject stale editor, split, merge and unstack writes.
  Native media deletion increments the affected version. Reload discards edits
  and obtains a fresh version. API operations run within one write transaction.
- Native file actions target selected individual members. Expanding a collapsed
  card does not implicitly select all members, and delete dialogs say that other
  members/files are kept. Stack edits preserve the list's selected typed IDs.

## Filtering, counts and order

The **Group stacks** toggle on All is stored in native URL/filter state and is
off by default. Native media filters execute first. A stack appears when any
member matches; its representative can be outside the filters, including the
media-type filter. The notice explains this, and expanding the card shows all
members with individual selection, preview and native links.

Grouping occurs before pagination. `count` counts displayed cards;
`image_count` and `video_count` describe representative card kinds;
`matched_count` counts matching native members. `stack_match_count` gives the
number of matches represented by each card. A group's position comes from its
first matching member under the existing global sort and native kind/ID ties,
so changing its representative does not move it within that sort. NULL values
remain last. Images/Videos and gallery queries retain their native ordering.

## Viewer

Native details and the shared preview show an ordered bottom filmstrip with
the selected member, relationship, representative marker, variant arrows,
keyboard navigation and **Open this member**. Preview selection follows typed
member identity through representative changes. Previous/next result controls
remain separate from variant navigation, preserving gallery/list order.

Only the selected Video mounts a native ScenePlayer; switching variants or
closing the preview disposes it. Video completion never advances to another
variant. Automatic playback retains PR #130's empty-poster startup policy.
Editor inputs do not navigate or dismiss the underlying preview. Variant zoom
resets follow the existing reset-on-navigation preference. P03 still-image
comparison remains disabled while a native Video variant is selected.

## Reviewed proposals

Review 2–100 selected members without inference or model downloads. Proposals
are independent pairs supported by one of:

- matching active-file MD5;
- recorded `source_md5` matching the other member's active MD5;
- for Images, pHash distance at most 4 **and** EVA02 cosine at least 0.98.

Embedding evidence must use the configured EVA02 model and a source key matching
the actual current primary file's path, size and modification time. Missing or
stale embeddings are skipped. Existing pairs in the same stack are skipped.
No transitive clustering is performed. Each pair opens an editable creation
review; nothing is grouped automatically. Rendering is paged at 50 proposals.

## Migration and verification

Migration 96 adds only visual stack tables, indexes and a native member-deletion
trigger. Fresh databases and an upgrade from schema 95 are covered by focused
integration tests, along with invariant preservation, rollback, typed identity,
representative deletion, filtering before pagination and current pair evidence.

Baseline: `develop` at `96309d8ffd66a141e94219a10b2ced7f2113b565` (merged #130).
Branch: `feat/p08-visual-stacks-20261003`. On 2026-10-03 the restored source passed
focused P07/P08 integration tests, all 30 UI tests, lint, source/test TypeScript
and formatting. Full backend gates, production builds and browser workflows
are in progress; final verification and publication details follow in the
implementation ledger.

Browser media are isolated synthetic VP8 and PNG fixtures. Production media,
other codecs, Safari, browser extensions and physical GPU rendering require
separate checks. P09 conversion trial/threshold work is a separate package.
