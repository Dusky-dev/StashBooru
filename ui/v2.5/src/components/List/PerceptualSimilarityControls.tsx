import React from "react";
import { Button, Form } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { faTimes } from "@fortawesome/free-solid-svg-icons";

import { FilterMode } from "src/core/generated-graphql";
import {
  ListFilterModel,
  MAX_SIMILARITY_DISTANCE,
} from "src/models/list-filter/filter";
import { Icon } from "../Shared/Icon";

export const PerceptualSimilarityControls: React.FC<{
  filter: ListFilterModel;
  setFilter: (filter: ListFilterModel) => void;
}> = ({ filter, setFilter }) => {
  const intl = useIntl();

  const distanceLabel = intl.formatMessage({
    id: "similarity_distance",
    defaultMessage: "Distance",
  });

  const referenceLabel = intl.formatMessage({
    id: "similarity_reference",
    defaultMessage: "Reference",
  });

  const clearReferenceLabel = intl.formatMessage({
    id: "actions.clear_similarity_reference",
    defaultMessage: "Clear similarity reference",
  });

  const referenceButton = filter.similarityReferenceID ? (
    <Button
      className="ml-2"
      variant="secondary"
      size="sm"
      title={clearReferenceLabel}
      onClick={() => setFilter(filter.setSimilarityReferenceID(undefined))}
    >
      {referenceLabel} #{filter.similarityReferenceID}
      <Icon icon={faTimes} className="ml-2" />
    </Button>
  ) : null;

  const imageReference =
    filter.mode === FilterMode.Images && !!filter.similarityReferenceID;
  const embedding = imageReference && filter.similarityMethod === "embedding";

  return (
    <div className="similarity-controls d-flex align-items-center flex-wrap">
      {imageReference && (
        <>
          <label className="mb-0 mx-2" htmlFor="image-similarity-method">
            <FormattedMessage id="image_similarity.method" />
          </label>
          <Form.Control
            as="select"
            id="image-similarity-method"
            size="sm"
            className="w-auto"
            value={filter.similarityMethod}
            onChange={(event) =>
              setFilter(
                filter.setSimilarityMethod(
                  event.currentTarget.value === "phash" ? "phash" : "embedding"
                )
              )
            }
          >
            <option value="phash">
              {intl.formatMessage({ id: "image_similarity.phash" })}
            </option>
            <option value="embedding">
              {intl.formatMessage({ id: "image_similarity.embedding" })}
            </option>
          </Form.Control>
        </>
      )}
      {!embedding && (
        <>
          <label className="mb-0 mx-2" htmlFor="phash-similarity-distance">
            {distanceLabel}: <strong>{filter.similarityDistance}</strong>
          </label>

          <input
            id="phash-similarity-distance"
            type="range"
            min={0}
            max={MAX_SIMILARITY_DISTANCE}
            step={1}
            value={filter.similarityDistance}
            aria-label={distanceLabel}
            onChange={(event) =>
              setFilter(
                filter.setSimilarityDistance(
                  Number.parseInt(event.currentTarget.value, 10)
                )
              )
            }
          />
        </>
      )}

      {referenceButton}
      {imageReference && (
        <small className="w-100 mt-1 mx-2 text-muted">
          <FormattedMessage
            id={
              embedding
                ? "image_similarity.embedding_help"
                : "image_similarity.phash_help"
            }
          />
        </small>
      )}
    </div>
  );
};
