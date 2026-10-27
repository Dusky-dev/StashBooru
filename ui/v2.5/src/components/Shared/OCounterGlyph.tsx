import { faHeart, faThumbsUp } from "@fortawesome/free-solid-svg-icons";
import React from "react";
import { useConfigurationContext } from "src/hooks/Config";
import { Icon } from "./Icon";
import { SweatDrops } from "./SweatDrops";

/** One icon source for scene and image O-counters. */
export const OCounterGlyph: React.FC = () => {
  const { configuration } = useConfigurationContext();
  const appearance = configuration?.ui?.oCounterIcon ?? "heart";

  if (appearance === "classic") {
    return configuration.interface.sfwContentMode ? (
      <Icon icon={faThumbsUp} />
    ) : (
      <SweatDrops />
    );
  }
  if (appearance === "thumbs-up") return <Icon icon={faThumbsUp} />;
  if (appearance === "custom") {
    const custom = configuration?.ui?.oCounterCustomIcon?.trim();
    if (custom) return <span aria-hidden="true">{custom}</span>;
  }
  return <Icon icon={faHeart} />;
};
