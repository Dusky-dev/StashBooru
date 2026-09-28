# P06 — Effective media associations

This first P06 slice adds `effective_associations` to Image and Scene GraphQL
results. Detail views use it to show inherited parent Characters, Artists,
Copyrights and Tags, plus Tags owned by selected entity profiles.

The associations are derived at read time. Existing `tags`, `performers`,
`artists` and `copyrights` fields continue to represent the direct, editable
links. Removing a child therefore removes its inherited ancestors from the
effective view, while a parent that is also directly linked remains visible.
No database migration or backfill is needed, and disambiguation context does
not create an authorship link.

Because the derivation reads the stored direct links, it applies regardless of
whether those links came from an edit form, import, scan, or tagging workflow.
This slice updates media detail views; inherited-aware search filters, list
counts, and per-domain inheritance settings remain follow-up work.
