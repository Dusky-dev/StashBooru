import { faImage, faPlayCircle } from "@fortawesome/free-solid-svg-icons";
import React, { useMemo } from "react";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { CopyrightGrid } from "src/components/Copyrights/CopyrightGrid";
import { PerformerCard } from "../Performers/PerformerCard";
import { Icon } from "./Icon";

interface BooruEntity {
  id: string;
  name: string;
  image_path?: string | null;
  scene_count?: number | null;
  image_count?: number | null;
  breadcrumb?: BooruEntity[] | null;
}

interface BooruTagSidebarProps {
  tags?: BooruEntity[] | null;
  artists?: BooruEntity[] | null;
  characters?: GQL.PerformerDataFragment[] | null;
  copyrights?: BooruEntity[] | null;
  provenance?: GQL.MediaAssociationProvenance[] | null;
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

function countLabel(singular: string, plural: string, count: number) {
  return count === 1 ? singular : plural;
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

const SectionTitle: React.FC<{ title: string; count: number }> = ({
  title,
  count,
}) => (
  <h6 className="booru-tag-section-title">
    <span>{title}</span>
    <span className="booru-tag-count">{count}</span>
  </h6>
);

const CardSection: React.FC<{
  kind: CardKind;
  singularTitle: string;
  pluralTitle: string;
  items: BooruEntity[];
  route: (id: string) => string;
}> = ({ kind, singularTitle, pluralTitle, items, route }) => {
  if (items.length === 0) return null;

  return (
    <section className={`booru-tag-section booru-tag-section-${kind}`}>
      <SectionTitle
        title={countLabel(singularTitle, pluralTitle, items.length)}
        count={items.length}
      />
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

  const cardWidth =
    items.length === 1
      ? 320
      : items.length === 2
        ? 240
        : items.length <= 4
          ? 180
          : 140;

  return (
    <section className="booru-tag-section booru-tag-section-character">
      <SectionTitle
        title={countLabel("Character", "Characters", items.length)}
        count={items.length}
      />
      <div className="booru-entity-card-grid booru-character-card-grid">
        {items.map((performer) => (
          <div
            className="booru-character-card-shell"
            key={performer.id}
            style={{ maxWidth: `${cardWidth}px`, width: "100%" }}
          >
            <PerformerCard performer={performer} cardWidth={cardWidth} />
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
      <SectionTitle
        title={countLabel("Tag", "Tags", items.length)}
        count={items.length}
      />
      <div className="booru-general-tags">
        {items.map((item) => (
          <GeneralTagName key={`general-${item.id}`} entity={item} />
        ))}
      </div>
    </section>
  );
};

function associationRoute(type: string, id: number): string | undefined {
  switch (type) {
    case "tag":
      return `/tags/${id}`;
    case "artist":
      return `/studios/${id}`;
    case "character":
      return `/performers/${id}`;
    case "copyright":
      return `/copyrights/${id}`;
    default:
      return undefined;
  }
}

function associationName(
  type: string,
  id: number,
  names: Map<string, string>
): string {
  return names.get(`${type}:${id}`) ?? `${type} #${id}`;
}

const AssociationSources: React.FC<{
  provenance: GQL.MediaAssociationProvenance[];
  names: Map<string, string>;
}> = ({ provenance, names }) => {
  if (provenance.length === 0) return null;

  const entityLink = (
    type: string | null | undefined,
    id: number | null | undefined
  ) => {
    if (!type || !id) return null;
    const route = associationRoute(type, id);
    const name = associationName(type, id, names);
    return route ? <Link to={route}>{name}</Link> : name;
  };

  return (
    <details className="booru-association-sources mt-2">
      <summary>Association sources ({provenance.length})</summary>
      <ul className="small mb-0 pl-3">
        {provenance.map((association) => (
          <li
            key={`${association.association_type}:${association.association_id}`}
          >
            <strong>
              {entityLink(
                association.association_type,
                association.association_id
              )}
            </strong>
            <ul className="pl-3">
              {association.origins.map((origin, index) => (
                <li
                  key={`${origin.kind}:${origin.source_type}:${origin.source_id}:${origin.via_id ?? 0}:${index}`}
                >
                  {origin.kind.replaceAll("_", " ")}:{" "}
                  {entityLink(origin.source_type, origin.source_id)}
                  {origin.via_type && origin.via_id ? (
                    <> via {entityLink(origin.via_type, origin.via_id)}</>
                  ) : null}
                  {origin.source_tag_id && origin.kind === "tag_ancestor" ? (
                    <> from Tag #{origin.source_tag_id}</>
                  ) : null}
                </li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
    </details>
  );
};

interface CopyrightBranch {
  key: string;
  path: BooruEntity[];
  items: BooruEntity[];
}

function commonPath(left: BooruEntity[], right: BooruEntity[]): BooruEntity[] {
  const length = Math.min(left.length, right.length);
  let commonLength = 0;
  while (
    commonLength < length &&
    left[commonLength].id === right[commonLength].id
  ) {
    commonLength += 1;
  }
  return left.slice(0, commonLength);
}

function groupCopyrightBranches(items: BooruEntity[]): CopyrightBranch[] {
  const branches = new Map<string, CopyrightBranch>();

  for (const item of items) {
    const path = item.breadcrumb?.length ? item.breadcrumb : [item];
    const root = path[0] ?? item;
    const existing = branches.get(root.id);
    if (existing) {
      existing.items.push(item);
      existing.path = commonPath(existing.path, path);
      if (existing.path.length === 0) existing.path = [root];
    } else {
      branches.set(root.id, { key: root.id, path, items: [item] });
    }
  }

  return [...branches.values()]
    .map((branch) => ({
      ...branch,
      items: sortByName(uniqueByID(branch.items)),
    }))
    .sort((a, b) =>
      (a.path[0]?.name ?? "").localeCompare(b.path[0]?.name ?? "", undefined, {
        sensitivity: "base",
      })
    );
}

const CopyrightBranchSection: React.FC<{ items: BooruEntity[] }> = ({
  items,
}) => {
  if (items.length === 0) return null;
  const branches = groupCopyrightBranches(items);

  return (
    <section className="booru-tag-section booru-tag-section-copyright">
      <SectionTitle
        title={countLabel("Copyright", "Copyrights", items.length)}
        count={items.length}
      />
      <div className="copyright-branch-groups">
        {branches.map((branch) => {
          const assignedIDs = new Set(branch.items.map((item) => item.id));
          const hierarchyItems = branch.path.filter(
            (node) => !assignedIDs.has(node.id)
          );

          return (
            <div className="copyright-branch-group mb-3" key={branch.key}>
              {hierarchyItems.length > 0 ? (
                <div className="mb-2">
                  <CopyrightGrid
                    items={hierarchyItems}
                    label="Main and Sub-Copyrights"
                  />
                </div>
              ) : null}
              <div className="booru-entity-card-grid booru-entity-card-grid-copyright">
                {branch.items.map((item) => (
                  <EntityCard
                    key={`copyright-${item.id}`}
                    entity={item}
                    kind="copyright"
                    route={`/copyrights/${item.id}`}
                  />
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
};

export const BooruTagSidebar: React.FC<BooruTagSidebarProps> = ({
  tags,
  artists,
  characters,
  copyrights,
  provenance,
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

  const names = useMemo(() => {
    const result = new Map<string, string>();
    for (const entity of tags ?? [])
      result.set(`tag:${entity.id}`, entity.name);
    for (const entity of artists ?? [])
      result.set(`artist:${entity.id}`, entity.name);
    for (const entity of characters ?? [])
      result.set(`character:${entity.id}`, entity.name);
    for (const entity of copyrights ?? [])
      result.set(`copyright:${entity.id}`, entity.name);
    return result;
  }, [artists, characters, copyrights, tags]);

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
            singularTitle="Artist"
            pluralTitle="Artists"
            items={sortedArtists}
            route={(id) => `/studios/${id}`}
          />
          <CharacterSection items={sortedCharacters} />
          <CopyrightBranchSection items={sortedCopyrights} />
        </div>
      ) : null}
      {sortedTags.length > 0 ? (
        <aside className="booru-metadata-sidebar">
          <GeneralTags items={sortedTags} />
        </aside>
      ) : null}
      <AssociationSources provenance={provenance ?? []} names={names} />
    </div>
  );
};
