# P06 — Effective media associations

P06 adds `effective_associations` to Image and Scene GraphQL results. Detail
views use it to show inherited parent Characters, Artists, Copyrights and
Tags, plus Tags owned by selected entity profiles.

The associations are derived at read time. Existing `tags`, `performers`,
`artists` and `copyrights` fields continue to represent the direct, editable
links. Removing a child therefore removes its inherited ancestors from the
effective view, while a parent that is also directly linked remains visible.
No database migration or backfill is needed, and disambiguation context does
not create an authorship link.

Because the derivation reads the stored direct links, it applies regardless of
whether those links came from an edit form, import, scan, or tagging workflow.
Media searches also expand Character variants, and new Tag, Artist and
Copyright hierarchy filters include descendants by default while preserving
the existing depth selector. Tagging now adds profile Tags from Character,
Artist and Copyright ancestors, as well as Tag ancestors, to its additive
result.

Hierarchy-aware list counts and per-domain inheritance settings remain
follow-up work.
