# StashBooru implementation progress

Updated: 2026-09-25

| Package | Status | Notes |
| --- | --- | --- |
| P01 — backup compatibility warning | Complete | Covered by merged PR #91; further backend backup-contract work intentionally skipped per project-owner direction. |
| P02 — image similarity modes | Complete | Merged PR #92 adds explicit pHash and EVA02 modes for image Find similar. |
| P03 — visual comparison / difference highlighting | Complete | Merged PR #93. Scope remains the bounded still-image comparison described below; automatic alignment and video frame selection are not implemented. |
| P04 — copyright sorting / taxonomy | In progress | First increment: editable Sort name, directory sorting by direct counts, stable pagination, and validated multi-parent hierarchy edits. Remaining taxonomy work is listed below. |

## P04 first increment

- Reuses native Copyright IDs and parent/child relations. No migration or GraphQL change.
- Exposes Sort name in create/edit. Directory Name ordering uses the displayed name; Sort Name uses the optional ordering name, falling back to the displayed name. Sort Name is the default for new directory views.
- Adds direct image/video/character count sorting and an ID tie-breaker to make pagination deterministic. Existing list-filter URL and saved-filter machinery stores these sort choices.
- Validates the complete proposed hierarchy before an update changes metadata or relationships. Rejects self-links, missing targets, malformed IDs, and cycles on both create and update. Multiple parents remain supported. The existing transaction around create/update rolls back a failed operation.
- Allows valid simultaneous parent/child changes, including reversing a relation when the old direction is removed in the same edit.
- Documents the existing deletion behavior: remove Copyright links, keep media and child records, and leave children with their remaining parents (or as roots).
- SQLite regression tests cover cycles, multi-parent edits, moves, creation rollback, deletion safety, search, count sorting, and stable pagination. UI validation covers TypeScript and focused lint/format checks.

### P04 still to implement

- Descendant-inclusive media filters/counts and sorting, with distinct counts across multiple parent paths.
- Tree/flat navigation, breadcrumbs, and manual sibling ordering.
- User-defined structural category roles for Tags and Copyrights, without fixed three-level rules.
- Deterministic media grouping/sorting by Copyright branch and optional primary Copyright selection.

Ancestor auto-association remains P06; Character variant relations remain P05.

## P03 validation scope

- Reuses the existing `ReferenceComparison` Lightbox path rather than introducing a second comparison UI.
- Keeps existing side-by-side, wipe, and synchronized pan/zoom behavior.
- Difference mode is deliberately limited to still images. Animated/video media reports that an explicit frame or timestamp is required instead of comparing arbitrary frames.
- Pixel work is bounded to a centered-fit canvas of at most 2048 pixels per dimension and about 1.5 million pixels.
- Different aspect ratios are not stretched; unmatched borders remain visible difference evidence.
- Difference boxes are connected pixel-difference regions after threshold/noise filtering, not object detection.
- Browser decoding supplies orientation/color handling for this first pass; automatic crop/rotation registration is not implemented.
- No database or GraphQL schema changes.
