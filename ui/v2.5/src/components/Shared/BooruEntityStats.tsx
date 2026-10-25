import { faImage, faPlayCircle } from "@fortawesome/free-solid-svg-icons";
import React from "react";
import {
  booruEntityMediaCounts,
  type BooruMediaCountSource,
} from "./booruEntityCounts";
import { Icon } from "./Icon";

export const BooruEntityStats: React.FC<{ entity: BooruMediaCountSource }> = ({
  entity,
}) => {
  const { videos, images } = booruEntityMediaCounts(entity);
  if (videos === null && images === null) return null;

  return (
    <div className="booru-entity-card-stats">
      {videos !== null ? (
        <span title="Videos" aria-label={`${videos} Videos`}>
          <Icon icon={faPlayCircle} /> {videos}
        </span>
      ) : null}
      {images !== null ? (
        <span title="Images" aria-label={`${images} Images`}>
          <Icon icon={faImage} /> {images}
        </span>
      ) : null}
    </div>
  );
};
