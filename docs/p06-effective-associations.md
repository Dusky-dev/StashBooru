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
changes the calculated result and tagging suggestions; it does not rewrite
existing media relationships.

Because the derivation reads the stored direct links, it applies regardless of
whether those links came from an edit form, import, scan, or tagging workflow.
Media searches expand Character variants, and Tag, Artist and Copyright
hierarchy filters include descendants by default while preserving the existing
depth selector. Character counts and media tabs include variant media across
Images, Videos, Galleries and Groups; shared media is counted once. Tagging
adds profile Tags from Character, Artist and Copyright ancestors, as well as
Tag ancestors, to its additive result. Copyright list counts now use the same
subtree totals as their media tabs.

Tag and Artist detail views continue to offer direct and all-descendant count
modes. The shared application service, review provenance and complete
cross-workflow acceptance matrix remain follow-up work.
