import React from "react";
import { FormattedMessage } from "react-intl";
import { Link } from "react-router-dom";

import * as GQL from "src/core/generated-graphql";
import { ErrorMessage } from "src/components/Shared/ErrorMessage";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { PerformerDisambiguationValue } from "../PerformerDisambiguationValue";

interface IProps {
  active: boolean;
  performer: GQL.PerformerDataFragment;
}

export const PerformerVariantsPanel: React.FC<IProps> = ({
  active,
  performer,
}) => {
  const parentID = Number(performer.id);
  const { data, loading, error } = GQL.useFindPerformerVariantsQuery({
    skip: !active || !Number.isSafeInteger(parentID),
    variables: {
      parent_id: parentID,
      filter: {
        page: 1,
        per_page: 100,
        sort: "name",
        direction: GQL.SortDirectionEnum.Asc,
      },
    },
  });

  if (!active) return null;
  if (loading) return <LoadingIndicator />;
  if (error) return <ErrorMessage error={error.message} />;

  const result = data?.findPerformers;
  const variants = result?.performers ?? [];

  return (
    <section className="performer-variants-panel">
      {variants.length === 0 ? (
        <p>
          <FormattedMessage
            id="no_character_variants"
            defaultMessage="No direct variants are linked to this Character."
          />
        </p>
      ) : (
        <ul className="performer-variants-list">
          {variants.map((variant) => (
            <li key={variant.id}>
              <Link to={`/performers/${variant.id}`}>{variant.name}</Link>
              <PerformerDisambiguationValue
                disambiguation={variant.disambiguation}
                context={variant.disambiguation_context}
              />
            </li>
          ))}
        </ul>
      )}
      {(result?.count ?? 0) > variants.length && (
        <p className="text-muted">
          <FormattedMessage
            id="character_variants_page_limit"
            defaultMessage="Showing the first {shown} of {total} direct variants."
            values={{ shown: variants.length, total: result?.count }}
          />
        </p>
      )}
    </section>
  );
};
