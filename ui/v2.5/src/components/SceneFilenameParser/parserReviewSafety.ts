/** Guard legacy bulk updates which replace, rather than append, direct links. */
interface AssociationField {
  isSet: boolean;
  originalValue?: readonly string[];
  value?: readonly string[];
}

interface FilenameReviewRow {
  performers: AssociationField;
  tags: AssociationField;
}

export function countParserAssociationReplacements(
  rows: readonly FilenameReviewRow[]
) {
  let characters = 0;
  let tags = 0;
  for (const row of rows) {
    if (wouldRemoveAssociations(row.performers)) characters++;
    if (wouldRemoveAssociations(row.tags)) tags++;
  }
  return { characters, tags };
}

function wouldRemoveAssociations(field: AssociationField): boolean {
  return (
    field.isSet &&
    (field.originalValue ?? []).some((id) => !(field.value ?? []).includes(id))
  );
}
