# P06 — Effective media associations

P06 is implemented across PRs #115, #116 and #117. Images and Scenes/Videos
expose `effective_associations`: the selected native relationships, their
enabled ancestors, and Tags owned by selected or enabled ancestor profiles.
Detail views show the calculated associations and a collapsible “Association
sources” section with links to the metadata that supplied each membership.

## Relationship and provenance contract

Native `tags`, `performers`, `artists` and `copyrights` remain the direct,
editable relationships. Existing assignments, including legacy rows, are
conservatively explicit. The primary Artist and multi-Artist readers are
de-duplicated before calculation. Disambiguation context does not create an
authorship relationship.

Effective memberships are computed from the current direct selections,
hierarchy edges and profile Tags. Detaching a child removes only memberships
that have lost their supporting sources. An explicit parent survives, and a
shared parent or Tag survives while another source supports it. Reparenting or
changing a profile changes the next read without rewriting media relations.

Each effective membership has its type, ID and a list of origins. Origins
distinguish direct selections, hierarchy ancestors, direct-profile Tags,
ancestor-profile Tags and Tag ancestors. They retain the direct source ID,
profile owner or supplying parent edge, and source Tag ID where applicable.
Copyright/Tag diamonds retain the converging parent edges; independent direct
sources and an explicit parent retain their own origins. Traversal terminates
on repeated nodes, and membership lists are de-duplicated and deterministic.

No database migration or derived-association table is introduced. Calculated
parents and profile Tags are never inserted into the native direct tables.
A direct Tag that is also supplied by a profile remains an intentional direct
assignment.

## Shared Library and Tagging defaults

Library settings independently enable Character, Artist, Copyright and Tag
ancestor inclusion; all four default to enabled. Image/Video detail reads,
Tagging inherited-Tag previews and the existing-library review use these same
defaults. Tagging settings link to the shared Library controls.

A domain switch controls that domain's ancestors and their profile Tags.
Direct entity profile Tags remain available when that entity's ancestor
switch is off. The Tag switch controls Tag ancestors, including ancestors of
profile Tags. Applying reviewed Image/Video Tagging persists only the selected,
resolved native IDs. Its inherited Tags are returned with their origins for
review.

Existing descendant-aware searches/counts from PR #115 are preserved.
Character filters and media tabs include variant media; hierarchy filters keep
their depth selector. Shared media is counted once. Tag/Artist detail pages
offer direct and descendant count modes, and Copyright directory counts use
the subtree totals shown by media tabs.

Image/Video hierarchical Tag searches (`INCLUDES`, `INCLUDES_ALL`, `EXCLUDES`
and their additional exclusions) also match the live profile-derived Tags shown
on detail pages. Tag detail Image/Video counts use that same native search.
Requested Tags are expanded to the profiles and native links that supply them;
the query does not calculate every media/Tag pair in the library. Multiple
sources, hierarchy diamonds and primary/multi-Artist overlap yield one media
result, with stable native sorting and pagination. Profile edits, reparenting
and detach affect the next search without a backfill or sticky copied Tags.

The four defaults control automatic ancestor membership in searches as in
detail reads. The explicit filter depth still lets a user search a whole Tag
branch even when automatic Tag ancestor membership is disabled. Exact-set and
null criteria, Tag-count maintenance criteria and nested `tags_filter` keep
their existing direct-link semantics; Tag-directory count filters/sorts also
retain their native stored-link scope.

## Preview and apply for the existing library

Settings → Tasks → **Review library inheritance** reviews all existing Images
and Videos against four proposed shared defaults. Library settings link to this
task. The native Jobs queue reports progress and supports cancellation.

Preview reads native rows in a SQLite read transaction using ID pagination
with 100 items per page. It reports reviewed Image/Video totals, affected
media, per-domain inherited membership totals and membership additions/removals.
Up to 20 samples show current/proposed memberships and source links; each side
is limited to 32 memberships and indicates truncation. Per-item errors are
reported with a bounded sample and prevent apply.

Apply rechecks the whole library under a native write transaction. A SHA-256
fingerprint includes every typed media ID and its complete before/after
provenance, including records outside the displayed samples. Changes to
selections, profiles, hierarchy or defaults that affect that snapshot require
a new preview. Cancellation before activation leaves defaults unchanged.
Successful activation saves all four settings under one configuration lock
with an atomic file replacement; a late cancellation reports the completed
activation. Repeating an already completed apply returns its completed state.

This is the P06 reviewed backfill/activation task: it reviews existing records
and activates the reviewed live projection. Apply saves the shared defaults;
it does not copy ancestor IDs into native relationship tables. Existing media
uses the activated projection on its next read.

Review state is in memory, limited to eight retained reviews. Terminal reviews
expire after 30 minutes; a server restart requires another preview. The browser
retains the current review ID for the session and can resume polling, cancel or
discard it. Discarding a running job requires cancellation to finish first.

## Native workflow acceptance

The integration suite uses fresh SQLite databases and native repository writers.
It reads the result through Image/Scene effective-association resolvers and
checks that native direct selections remain distinct from inherited results.

| Workflow | Verification |
| --- | --- |
| Manual single Image/Video edits | Real GraphQL update mutations and native Copyright update mutations; selected children remain direct while ancestors/profile Tags appear on read. |
| Manual bulk Image/Video edits | Real bulk GraphQL mutations over two media of each type; the same direct/effective contract is checked for both records. |
| Reviewed Image/Video Tagging | Shared change-plan preview and native apply functions, applied twice; inherited Tags stay out of direct media Tags. |
| Bulk Video review | Real filename parsing, native target enrichment, review and repeated batch-item apply against a native primary Video file. |
| Native Auto Tag | Native Character/Artist/Tag filename match writers for Images and Videos; calculated ancestors remain derived. Native Copyright links are retained. |
| Import and repeated import | Native Image/Scene importers match existing file IDs and apply twice. The legacy JSON schema lacks Copyright fields; existing native Copyright links are preserved. |
| Scan and rescan | Native Image/Scene scan handlers preserve selections and recalculate inheritance; animated-image scan adds only its native explicit animated Tag. |
| Shared sources and explicit parents | Copyright/Tag diamonds, independent source origins, detach of one source and detach of all sources; an explicitly selected parent Tag survives. |
| Hierarchy changes | Native Character, Artist, Copyright and Tag reparenting changes the next read without altering direct media links. Invalid Copyright cycle/name edits roll back. |
| Tag searches and detail media counts | Native queries agree with effective-association reads for both media types and all 16 default combinations; shared sources, exclusions, pagination, profile edits, reparenting, detach and malformed legacy Tag cycles are covered. |
| Existing-library task | Real paginated Image/Video scan, preview/apply, stale selections/profiles/reparenting, per-item errors, cancellation and native HTTP Jobs integration. |
| Configuration activation | Atomic snapshot/save, persisted defaults, stale-default and override rejection, existing permission/unrelated-setting preservation, rollback on save failure. |

Tests are in `internal/api/p06_sources_integration_test.go`,
`p06_native_workflows_integration_test.go`,
`p06_native_writers_integration_test.go` and
`association_inheritance_review_integration_test.go`, plus the configuration
tests and earlier provenance/settings/resolver regressions.
Tag search regressions are in `internal/api/p06_tag_search_integration_test.go`.

The scan tests use real native File rows and stub only derived thumbnail/cover
generation; they do not decode media or invoke external encoders. Tagging
tests use local existing targets and filename metadata; external model/booru
availability is outside this acceptance scope. Browser interaction has not
been manually exercised in this follow-up.

## Verification

CI validation is recorded in `docs/implementation-progress.md`. It includes
backend generation/tests, UI tests/lint/TypeScript/format/build, Go lint and
the seven-platform build matrix. The new native workflow tests require the
`integration` build tag and run in the repository's backend CI test target.
