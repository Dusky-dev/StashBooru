import React from "react";
import { Button } from "react-bootstrap";
import { useIntl } from "react-intl";
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

  if (filter.mode === FilterMode.Images && filter.similarityReferenceID) {
    return (
      <div className="similarity-controls d-flex align-items-center flex-wrap">
        <span className="mb-0 mx-2">
          <strong>Visual similarity</strong> · EVA02 embeddings · nearest first
        </span>
        {referenceButton}
      </div>
    );
  }

  return (
    <div className="similarity-controls d-flex align-items-center flex-wrap">
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

      {referenceButton}
    </div>
  );
};
