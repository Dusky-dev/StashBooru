import React from "react";
import { faQuestionCircle } from "@fortawesome/free-solid-svg-icons";
import { ManualLink } from "src/components/Help/context";
import { Icon } from "src/components/Shared/Icon";

// Tasks live inside Settings disclosures, so use a compact heading and help link.
export const TaskHelpHeading: React.FC<{ id?: string; title: string }> = ({
  id,
  title,
}) => (
  <h3 id={id} className="h4">
    {title}{" "}
    <ManualLink tab="Tasks">
      <Icon icon={faQuestionCircle} size="xs" aria-hidden="true" />
      <span className="sr-only">Help with {title}</span>
    </ManualLink>
  </h3>
);
