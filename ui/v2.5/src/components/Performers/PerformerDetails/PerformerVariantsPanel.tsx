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
        <div
          className="booru-entity-card-grid booru-character-card-grid performer-variant-grid"
          role="list"
          aria-label="Character variants"
        >
          {variants.map((variant) => (
            <article
              key={variant.id}
              className="booru-entity-card booru-entity-card-character"
              role="listitem"
            >
              <Link
                className="booru-entity-card-image-link"
                to={`/performers/${variant.id}`}
                tabIndex={-1}
                aria-hidden="true"
              >
                {variant.image_path ? (
                  <img
                    loading="lazy"
                    className="booru-entity-card-image"
                    alt=""
                    src={variant.image_path}
                  />
                ) : (
                  <span
                    className="booru-entity-card-placeholder"
                    aria-hidden="true"
                  />
                )}
              </Link>
              <Link
                className="booru-entity-card-name"
                to={`/performers/${variant.id}`}
                title={variant.name}
              >
                {variant.name}
                <PerformerDisambiguationValue
                  disambiguation={variant.disambiguation}
                  context={variant.disambiguation_context}
                  linkContext={false}
                />
              </Link>
            </article>
          ))}
        </div>
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
