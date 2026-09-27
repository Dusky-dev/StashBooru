import React from "react";
import { Link } from "react-router-dom";

export interface CopyrightGridItem {
  id: string;
  name: string;
  image_path?: string | null;
}

interface CopyrightGridProps {
  items?: CopyrightGridItem[] | null;
  label?: string;
}

export const CopyrightGrid: React.FC<CopyrightGridProps> = ({
  items,
  label,
}) => {
  if (!items?.length) return null;

  return (
    <div
      className="booru-entity-card-grid booru-entity-card-grid-copyright"
      role="list"
      aria-label={label}
    >
      {items.map((copyright) => (
        <article
          key={copyright.id}
          className="booru-entity-card booru-entity-card-copyright"
          role="listitem"
        >
          <Link
            className="booru-entity-card-image-link"
            to={`/copyrights/${copyright.id}`}
            tabIndex={-1}
            aria-hidden="true"
          >
            {copyright.image_path ? (
              <img
                loading="lazy"
                className="booru-entity-card-image"
                alt=""
                src={copyright.image_path}
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
            to={`/copyrights/${copyright.id}`}
            title={copyright.name}
          >
            {copyright.name}
          </Link>
        </article>
      ))}
    </div>
  );
};
