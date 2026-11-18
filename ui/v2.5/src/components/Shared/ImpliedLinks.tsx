import React from "react";
import { Badge } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { TagLink } from "./TagLink";

type MediaType = "image" | "scene";

/**
 * Lists what an Image or Video gets from its Tags' and Copyrights' parents
 * and its profiles, below the direct entries in a card popover. Loaded only
 * while the popover is open, and always fresh so a parent edit shows on the
 * next open.
 */
export const ImpliedLinks: React.FC<{
  type: MediaType;
  id: string;
  kind: "tags" | "copyrights";
  directIDs: string[];
}> = ({ type, id, kind, directIDs }) => {
  const options = {
    variables: { id },
    fetchPolicy: "network-only" as const,
  };
  const image = GQL.useFindImageImpliedAssociationsQuery({
    ...options,
    skip: type !== "image",
  });
  const scene = GQL.useFindSceneImpliedAssociationsQuery({
    ...options,
    skip: type !== "scene",
  });
  const effective =
    type === "image"
      ? image.data?.findImage?.effective_associations
      : scene.data?.findScene?.effective_associations;
  const direct = new Set(directIDs);
  const tags = (effective?.tags ?? []).filter((tag) => !direct.has(tag.id));
  const copyrights = (effective?.copyrights ?? []).filter(
    (copyright) => !direct.has(copyright.id)
  );
  if ((kind === "tags" ? tags : copyrights).length === 0) return null;
  return (
    <div className="implied-links">
      <div className="implied-links-heading text-muted small">
        <FormattedMessage id="media_card.implied" />
      </div>
      {kind === "tags"
        ? tags.map((tag) => <TagLink key={tag.id} tag={tag} linkType={type} />)
        : copyrights.map((copyright) => (
            <CopyrightLink key={copyright.id} copyright={copyright} />
          ))}
    </div>
  );
};

export const CopyrightLink: React.FC<{
  copyright: { id: string; name: string };
}> = ({ copyright }) => (
  <Badge className="tag-item tag-link copyright-link" variant="secondary">
    <Link to={`/copyrights/${copyright.id}`}>{copyright.name}</Link>
  </Badge>
);
