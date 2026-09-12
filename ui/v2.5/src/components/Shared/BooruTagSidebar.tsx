import { faImage } from "@fortawesome/free-solid-svg-icons";
import React, { useMemo, useState } from "react";
import { Spinner } from "react-bootstrap";
import { Link } from "react-router-dom";
import { queryFindTag } from "src/core/StashService";
import { Icon } from "./Icon";

interface BooruEntity {
  id: string;
  name: string;
  image_path?: string | null;
}

interface BooruTagSidebarProps {
  tags?: BooruEntity[] | null;
  artists?: BooruEntity[] | null;
  characters?: BooruEntity[] | null;
  copyrights?: BooruEntity[] | null;
}

type EntityKind = "copyright" | "character" | "artist" | "general";

function sortByName<T extends BooruEntity>(items?: T[] | null): T[] {
  return [...(items ?? [])].sort((a, b) =>
    a.name.localeCompare(b.name, undefined, { sensitivity: "base" })
  );
}

const Preview: React.FC<{
  imagePath?: string | null;
  className?: string;
}> = ({ imagePath, className }) => {
  const [active, setActive] = useState(false);

  if (!imagePath) return null;

  return (
    <span
      className={`booru-tag-preview-trigger${className ? ` ${className}` : ""}`}
      onMouseEnter={() => setActive(true)}
      onMouseLeave={() => setActive(false)}
      tabIndex={0}
      onFocus={() => setActive(true)}
      onBlur={() => setActive(false)}
      aria-label="Preview image"
    >
      <Icon icon={faImage} />
      {active ? (
        <span className="booru-tag-preview">
          <img src={imagePath} alt="" loading="lazy" />
        </span>
      ) : null}
    </span>
  );
};

const LazyGeneralTagPreview: React.FC<{ tag: BooruEntity }> = ({ tag }) => {
  const [active, setActive] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [imagePath, setImagePath] = useState<string | null>(null);

  async function loadPreview() {
    setActive(true);
    if (loaded || loading) return;

    setLoading(true);
    try {
      const result = await queryFindTag(tag.id);
      setImagePath(result.data.findTag?.image_path ?? null);
    } finally {
      setLoading(false);
      setLoaded(true);
    }
  }

  return (
    <span
      className="booru-tag-preview-trigger booru-general-preview-trigger"
      onMouseEnter={() => void loadPreview()}
      onMouseLeave={() => setActive(false)}
      tabIndex={0}
      onFocus={() => void loadPreview()}
      onBlur={() => setActive(false)}
      aria-label={`Preview ${tag.name}`}
    >
      <Icon icon={faImage} />
      {active ? (
        <span className="booru-tag-preview">
          {loading ? (
            <span className="booru-tag-preview-loading">
              <Spinner animation="border" size="sm" />
            </span>
          ) : imagePath ? (
            <img src={imagePath} alt={tag.name} loading="lazy" />
          ) : (
            <span className="booru-tag-preview-empty">No image</span>
          )}
        </span>
      ) : null}
    </span>
  );
};

const BooruSection: React.FC<{
  kind: EntityKind;
  title: string;
  items: BooruEntity[];
  route: (id: string) => string;
}> = ({ kind, title, items, route }) => {
  if (items.length === 0) return null;

  return (
    <section className={`booru-tag-section booru-tag-section-${kind}`}>
      <h6 className="booru-tag-section-title">
        {title} <span className="booru-tag-count">{items.length}</span>
      </h6>
      <div className="booru-tag-list">
        {items.map((item) => (
          <div
            className={`booru-tag-row booru-tag-row-${kind}`}
            key={`${kind}-${item.id}`}
          >
            <Link className="booru-tag-name" to={route(item.id)}>
              {item.name}
            </Link>
            {kind === "general" ? (
              <LazyGeneralTagPreview tag={item} />
            ) : (
              <Preview imagePath={item.image_path} />
            )}
          </div>
        ))}
      </div>
    </section>
  );
};

export const BooruTagSidebar: React.FC<BooruTagSidebarProps> = ({
  tags,
  artists,
  characters,
  copyrights,
}) => {
  const sortedTags = useMemo(() => sortByName(tags), [tags]);
  const sortedArtists = useMemo(() => sortByName(artists), [artists]);
  const sortedCharacters = useMemo(
    () => sortByName(characters),
    [characters]
  );
  const sortedCopyrights = useMemo(
    () => sortByName(copyrights),
    [copyrights]
  );

  if (
    sortedTags.length === 0 &&
    sortedArtists.length === 0 &&
    sortedCharacters.length === 0 &&
    sortedCopyrights.length === 0
  ) {
    return null;
  }

  return (
    <aside className="booru-metadata-sidebar" aria-label="Tags">
      <BooruSection
        kind="copyright"
        title="Copyrights"
        items={sortedCopyrights}
        route={(id) => `/copyrights/${id}`}
      />
      <BooruSection
        kind="character"
        title="Characters"
        items={sortedCharacters}
        route={(id) => `/performers/${id}`}
      />
      <BooruSection
        kind="artist"
        title="Artists"
        items={sortedArtists}
        route={(id) => `/studios/${id}`}
      />
      <BooruSection
        kind="general"
        title="Tags"
        items={sortedTags}
        route={(id) => `/tags/${id}`}
      />
    </aside>
  );
};
