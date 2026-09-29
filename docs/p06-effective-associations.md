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

The handoff also calls for a previewable, reviewed backfill job for existing
Image and Video associations. It is still open: before such a job can write
anything, the implementation needs a separate provenance model for materialized
inherited links. Writing ancestors into the current direct relation tables would
turn derived memberships into sticky explicit assignments when a child is later
removed. The live read-time projection already covers existing media safely;
the backfill must preserve that distinction and must never silently rewrite the
library.
