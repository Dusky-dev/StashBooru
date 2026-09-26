import React from "react";
import { Link } from "react-router-dom";

export interface IPerformerDisambiguationContext {
  copyright?: { id: string; name: string } | null;
  artist?: { id: string; name: string } | null;
}

interface IProps {
  disambiguation?: string | null;
  context?: IPerformerDisambiguationContext | null;
  linkContext?: boolean;
}

export const PerformerDisambiguationValue: React.FC<IProps> = ({
  disambiguation,
  context,
  linkContext = true,
}) => {
  const target = context?.copyright
    ? {
        name: context.copyright.name,
        url: `/copyrights/${context.copyright.id}`,
      }
    : context?.artist
      ? {
          name: context.artist.name,
          url: `/studios/${context.artist.id}`,
        }
      : undefined;
  const label = disambiguation?.trim();

  if (!target && !label) return null;

  const contextLabel = target ? (
    linkContext ? (
      <Link to={target.url}>{target.name}</Link>
    ) : (
      target.name
    )
  ) : (
    label
  );
  const showLegacyLabel =
    target &&
    label &&
    label.toLocaleLowerCase() !== target.name.trim().toLocaleLowerCase();

  return (
    <span className="performer-disambiguation">
      {" ("}
      {contextLabel}
      {showLegacyLabel ? ` — ${label}` : ""}
      {")"}
    </span>
  );
};
