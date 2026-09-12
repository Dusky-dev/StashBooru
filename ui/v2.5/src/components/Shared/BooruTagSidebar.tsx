import { faImage } from "@fortawesome/free-solid-svg-icons";
import React, { useMemo, useState } from "react";
import { Link } from "react-router-dom";
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
  entity: BooruEntity;
  general?: boolean;
}> = ({ entity, general = false }) => {
  const [active, setActive] = useState(false);

  if (!entity.image_path) return null;

  return (
    <button
      type="button"
      className={`booru-tag-preview-trigger minimal${
        general ? " booru-general-preview-trigger" : ""
      }`}
      onMouseEnter={() => setActive(true)}
      onMouseLeave={() => setActive(false)}
      onFocus={() => setActive(true)}
      onBlur={() => setActive(false)}
      aria-label={`Preview ${entity.name}`}
    >
      <Icon icon={faImage} />
      {active ? (
        <span className="booru-tag-preview">
          <img src={entity.image_path} alt={entity.name} loading="lazy" />
        </span>
      ) : null}
    </button>
  );
};

const BooruSection: React.FC<{
  kind: EntityKind;
  title: string;
  items: BooruEntity[];
  indexRoute: string;
  route: (id: string) => string;
}> = ({ kind, title, items, indexRoute, route }) => {
  if (items.length === 0) return null;

  return (
    <section className={`booru-tag-section booru-tag-section-${kind}`}>
      <h6 className="booru-tag-section-title">
        <Link to={indexRoute}>{title}</Link>
        <span className="booru-tag-count">{items.length}</span>
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
            <Preview entity={item} general={kind === "general"} />
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
        indexRoute="/copyrights"
        route={(id) => `/copyrights/${id}`}
      />
      <BooruSection
        kind="character"
        title="Characters"
        items={sortedCharacters}
        indexRoute="/performers"
        route={(id) => `/performers/${id}`}
      />
      <BooruSection
        kind="artist"
        title="Artists"
        items={sortedArtists}
        indexRoute="/studios"
        route={(id) => `/studios/${id}`}
      />
      <BooruSection
        kind="general"
        title="Tags"
        items={sortedTags}
        indexRoute="/tags"
        route={(id) => `/tags/${id}`}
      />
    </aside>
  );
};
