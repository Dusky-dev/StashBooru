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

Image and Video detail queries use the same effective-association builder, so
inheritance is recalculated after an edit or hierarchy change without a media
rewrite. This shared read path does not prove each writer workflow's behavior;
end-to-end checks for manual edits, native Auto Tag, import, scan, repeated
import, and batch review remain outstanding.

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
parents. Resolver tests cover Image and Video wiring. They do not yet exercise
the full cross-workflow acceptance matrix.

Image and Video tagging previews now include inherited Tags with their origins:
the direct Character, Artist, or Copyright profile that supplied a Tag, the
ancestor profile that supplied it, or the selected Tag whose parent supplied
it. The same summary is shown in bulk Video review when selected predictions
resolve to existing metadata. Applying predictions persists only the explicitly
selected Tag IDs; profile Tags and hierarchy parents remain calculated.

Effective-association responses also include one provenance entry per effective
membership. Each entry identifies its association type and ID plus all known
origins: direct selection, supported hierarchy child, profile Tag owner, or
source Tag and parent edge. Shared parents retain an origin for each independent
direct source; an explicitly selected parent retains its direct origin beside
derived origins. Image and Video detail views show these sources in a collapsed
“Association sources” section with links to the referenced metadata.

No derived-association storage has been introduced, so live inheritance itself
recalculates without a data migration. Existing direct associations, including
legacy assignments, are treated as explicit. Do not remove direct media Tags
merely because they also appear through a profile or hierarchy; that could
erase an intentional assignment.

## Acceptance across media workflows — incomplete

The implementation relies on native direct media relationships and a shared
read resolver. That is a design argument, not an integration test of every
writer. The P06 handoff's workflow-level acceptance requirements are not yet
verified end to end.

| Workflow | Stored relationship | P06 behavior |
| --- | --- | --- |
| Single or bulk Image/Video edits | Selected direct Character, Artist, Copyright, and Tag IDs | The next detail read recalculates ancestors and profile Tags. |
| Reviewed Image/Video Tagging | Only selected, resolved IDs are applied; inherited Tags are returned with origins for review | Applying a plan does not turn inherited Tags into direct assignments. |
| Native Auto Tag, import, and scan | Existing native media relationship rows | The same effective resolver reads the rows; no writer-specific migration is needed. |
| Existing library | Existing direct rows and current entity hierarchies | Existing media receives the live projection immediately. |

Resolver tests cover the Image and Video GraphQL relationship readers, legacy
primary Artist plus multi-Artist links, multi-parent Copyrights, profile Tags,
Tag ancestors, and de-duplication. Core tests cover settings, cycles, shared
ancestors, explicit parent links, detach/reparent behavior, and tagging
origins/direct-only application. Actual manual single/bulk edits, native Auto
Tag, import, scan, repeated import, and batch-review workflows still need tests
for both Images and Videos.

The current resolver tests verify direct and derived provenance across Image
and Video associations, including multiple Copyright ancestry paths, an
explicit parent that is also inherited, profile Tags and Tag ancestors. These
tests do not yet exercise provenance through every actual media writer
workflow.

## Backfill requirement — unresolved

The P06 handoff explicitly requires a previewable backfill task. This
implementation omits it because effective associations are computed from
existing direct rows and current hierarchy/profile data. That rationale does
not satisfy the stated requirement. Blindly inserting ancestors into native
direct relation tables would make them sticky after a child is detached and
could overwrite the user's distinction between an explicit link and a
calculated one.

Before P06 can be marked complete, either implement a reviewed preview/apply
backfill with provenance that safely reconciles shared sources, hierarchy
changes, and user edits, or explicitly revise the handoff to accept read-time
projection in place of backfill. Current System settings affect Tagging too;
the request for System/Tagging defaults also needs confirmation as to whether a
separate Tagging override is required.
