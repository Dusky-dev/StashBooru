# P06 — Effective media associations

P06 adds `effective_associations` to Image and Scene GraphQL results. Detail
views use it to show inherited parent Characters, Artists, Copyrights and
Tags, plus Tags owned by selected entity profiles.

The associations are derived at read time. Existing `tags`, `performers`,
`artists` and `copyrights` fields continue to represent the direct, editable
links. Removing a child therefore removes its inherited ancestors from the
effective view, while a parent that is also directly linked remains visible.
This read-time projection needs no migration. The broader P06 handoff's
previewable backfill requirement remains unresolved because the current design
does not persist inherited links. Disambiguation context does not create an
authorship link.

System settings independently control whether Character, Artist, Copyright
and Tag ancestors are included. Each defaults to enabled. Changing a switch
changes the calculated result; it does not rewrite existing media relationships.

The same read-time resolver consumes the stored direct links regardless of
whether an edit form, import, scan, native Auto Tag, or tagging workflow wrote
them. Manual edits keep their existing direct-only update contract. Image and
Video detail queries use the same effective-association builder, so inheritance
is recalculated after an edit or hierarchy change without a media rewrite.

Media searches expand Character variants, and Tag, Artist and Copyright
hierarchy filters include descendants by default while preserving the existing
depth selector. Character counts and media tabs include variant media across
Images, Videos, Galleries and Groups; shared media is counted once. Tagging
adds profile Tags from Character, Artist and Copyright ancestors, as well as
Tag ancestors, to its additive result. Copyright list counts now use the same
subtree totals as their media tabs.

Tag and Artist detail views continue to offer direct and all-descendant count
modes. Regression tests cover cycle termination, de-duplication, diamond
Copyright/Tag ancestry, domain settings, reparenting, shared parents and explicit
parents. The shared resolver makes the result independent of which supported
write workflow supplied the direct links; end-to-end tests for the full
cross-workflow matrix remain follow-up work.

Image and Video tagging previews now include inherited Tags with their origins:
the direct Character, Artist, or Copyright profile that supplied a Tag, the
ancestor profile that supplied it, or the selected Tag whose parent supplied
it. The same summary is shown in bulk Video review when selected predictions
resolve to existing metadata. Applying predictions persists only the explicitly
selected Tag IDs; profile Tags and hierarchy parents remain calculated.

No derived-association storage has been introduced, so live inheritance itself
recalculates without a data migration. Existing direct associations, including
legacy assignments, are treated as explicit. Do not remove direct media Tags
merely because they also appear through a profile or hierarchy; that could
erase an intentional assignment.

## Acceptance across media workflows

Every supported writer stores native direct media relationships. The effective
resolver reads those relationships from the same Image and Scene repositories,
so behavior does not depend on whether a link came from an edit form, an import,
a scan, native Auto Tag, or reviewed Tagging. There is no second writer-specific
inheritance state to synchronize.

| Workflow | Stored relationship | P06 behavior |
| --- | --- | --- |
| Single or bulk Image/Video edits | Selected direct Character, Artist, Copyright, and Tag IDs | The next detail read recalculates ancestors and profile Tags. |
| Reviewed Image/Video Tagging | Only selected, resolved IDs are applied; inherited Tags are returned with origins for review | Applying a plan does not turn inherited Tags into direct assignments. |
| Native Auto Tag, import, and scan | Existing native media relationship rows | The same effective resolver reads the rows; no writer-specific migration is needed. |
| Existing library | Existing direct rows and current entity hierarchies | Existing media receives the live projection immediately. |

Resolver tests cover the Image and Video GraphQL relationship readers, legacy
primary Artist plus multi-Artist links, multi-parent Copyrights, profile Tags,
Tag ancestors, and de-duplication. The core behavior tests cover settings,
cycles, shared ancestors, explicit parent links, detach/reparent behavior, and
tagging origins/direct-only application.

## Backfill decision

No write-backfill job is needed for this design. Effective associations are
computed from existing direct rows and current hierarchy/profile data, so there
are no previously materialized inherited links to migrate. A job that inserted
ancestors into the native direct relation tables would make them sticky after a
child is detached, and could overwrite the user's distinction between an
explicit link and a calculated one. The reviewed Tagging preview already shows
the calculated additions and their origins without changing media rows.

If a future consumer requires stored inherited rows for performance or export,
that needs a separate provenance-backed materialization design. It must record
direct-versus-derived origins and reconcile shared sources, hierarchy changes,
and user edits before any write-backfill can be safe.
