import React from "react";
import { Button } from "react-bootstrap";
import { faCopyright } from "@fortawesome/free-solid-svg-icons";
import { HoverPopover } from "./HoverPopover";
import { Icon } from "./Icon";
import { CopyrightLink, ImpliedLinks } from "./ImpliedLinks";

/** Card button for an Image's or Video's direct and implied Copyrights. */
export const CopyrightPopoverButton: React.FC<{
  type: "image" | "scene";
  id: string;
  copyrights: { id: string; name: string }[];
}> = ({ type, id, copyrights }) => {
  if (copyrights.length === 0) return null;
  const content = [
    ...copyrights.map((copyright) => (
      <CopyrightLink key={copyright.id} copyright={copyright} />
    )),
    <ImpliedLinks
      key="implied"
      type={type}
      id={id}
      kind="copyrights"
      directIDs={copyrights.map((copyright) => copyright.id)}
    />,
  ];
  return (
    <HoverPopover
      className="copyright-count"
      placement="bottom"
      content={content}
    >
      <Button className="minimal">
        <Icon icon={faCopyright} />
        <span>{copyrights.length}</span>
      </Button>
    </HoverPopover>
  );
};
