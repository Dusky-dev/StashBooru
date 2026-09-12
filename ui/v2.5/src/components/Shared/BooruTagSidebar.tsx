import { faImage, faPlayCircle } from "@fortawesome/free-solid-svg-icons";
import React, { useMemo } from "react";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { PerformerCard } from "../Performers/PerformerCard";
import { Icon } from "./Icon";

interface BooruEntity {
  id: string;
  name: string;
  image_path?: string | null;
  scene_count?: number | null;
  image_count?: number | null;
}

interface BooruTagSidebarProps {
  tags?: BooruEntity[] | null;
  artists?: BooruEntity[] | null;
  characters?: GQL.PerformerDataFragment[] | null;
  copyrights?: BooruEntity[] | null;
}

type CardKind = "artist" | "copyright";

function sortByName<T extends { name: string }>(items?: T[] | null): T[] {
  return [...(items ?? [])].sort((a, b) =>
    a.name.localeCompare(b.name, undefined, { sensitivity: "base" })
  );
}

function uniqueByID<T extends { id: string }>(items: T[]): T[] {
  return [...new Map(items.map((item) => [item.id, item])).values()];
}

const EntityStats: React.FC<{ entity: BooruEntity }> = ({ entity }) => (
  <div className="booru-entity-card-stats">
    <span title="Videos">
      <Icon icon={faPlayCircle} /> {entity.scene_count ?? 0}
    </span>
    <span title="Images">
      <Icon icon={faImage} /> {entity.image_count ?? 0}
    </span>
  </div>
);

const EntityCard: React.FC<{
  entity: BooruEntity;
  kind: CardKind;
  route: string;
}> = ({ entity, kind, route }) => (
  <article className={`booru-entity-card booru-entity-card-${kind}`}>
    <Link className="booru-entity-card-image-link" to={route} tabIndex={-1}>
      {entity.image_path ? (
        <img
          className="booru-entity-card-image"
          src={entity.image_path}
          alt=""
          loading="lazy"
        />
      ) : (
        <span className="booru-entity-card-placeholder" aria-hidden="true" />
      )}
    </Link>
    <Link className="booru-entity-card-name" to={route} title={entity.name}>
      {entity.name}
    </Link>
    <EntityStats entity={entity} />
  </article>
);

const CardSection: React.FC<{
  kind: CardKind;
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
      <div className={`booru-entity-card-grid booru-entity-card-grid-${kind}`}>
        {items.map((item) => (
          <EntityCard
            key={`${kind}-${item.id}`}
            entity={item}
            kind={kind}
            route={route(item.id)}
          />
        ))}
      </div>
    </section>
  );
};

const CharacterSection: React.FC<{
  items: GQL.PerformerDataFragment[];
}> = ({ items }) => {
  if (items.length === 0) return null;

  return (
    <section className="booru-tag-section booru-tag-section-character">
      <h6 className="booru-tag-section-title">
        <Link to="/performers">Characters</Link>
        <span className="booru-tag-count">{items.length}</span>
      </h6>
      <div className="booru-entity-card-grid booru-character-card-grid">
        {items.map((performer) => (
          <div className="booru-character-card-shell" key={performer.id}>
            <PerformerCard performer={performer} cardWidth={140} />
          </div>
        ))}
      </div>
    </section>
  );
};

const GeneralTagName: React.FC<{ entity: BooruEntity }> = ({ entity }) => (
  <span className="booru-general-tag-name-wrap">
    <Link
      className="booru-tag-name"
      to={`/tags/${entity.id}`}
      title={entity.name}
    >
      {entity.name}
    </Link>
    {entity.image_path ? (
      <span className="booru-tag-preview" aria-hidden="true">
        <img src={entity.image_path} alt="" loading="lazy" />
      </span>
    ) : null}
  </span>
);

const GeneralTags: React.FC<{ items: BooruEntity[] }> = ({ items }) => {
  if (items.length === 0) return null;

  return (
    <section className="booru-tag-section booru-tag-section-general">
      <h6 className="booru-tag-section-title">
        <Link to="/tags">Tags</Link>
        <span className="booru-tag-count">{items.length}</span>
      </h6>
      <div className="booru-general-tags">
        {items.map((item) => (
          <GeneralTagName key={`general-${item.id}`} entity={item} />
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
  const sortedCharacters = useMemo(() => sortByName(characters), [characters]);
  const sortedCopyrights = useMemo(() => {
    const inherited = (characters ?? []).flatMap(
      (character) => character.copyrights ?? []
    );
    return sortByName(uniqueByID([...(copyrights ?? []), ...inherited]));
  }, [characters, copyrights]);

  if (
    sortedTags.length === 0 &&
    sortedArtists.length === 0 &&
    sortedCharacters.length === 0 &&
    sortedCopyrights.length === 0
  ) {
    return null;
  }

  const hasEntityCards =
    sortedArtists.length > 0 ||
    sortedCharacters.length > 0 ||
    sortedCopyrights.length > 0;

  return (
    <div className="booru-metadata" aria-label="Booru metadata">
      {hasEntityCards ? (
        <div className="booru-entity-metadata">
          <CardSection
            kind="artist"
            title="Artists"
            items={sortedArtists}
            indexRoute="/studios"
            route={(id) => `/studios/${id}`}
          />
          <CharacterSection items={sortedCharacters} />
          <CardSection
            kind="copyright"
            title="Copyrights"
            items={sortedCopyrights}
            indexRoute="/copyrights"
            route={(id) => `/copyrights/${id}`}
          />
        </div>
      ) : null}
      {sortedTags.length > 0 ? (
        <aside className="booru-metadata-sidebar">
          <GeneralTags items={sortedTags} />
        </aside>
      ) : null}
    </div>
  );
};
