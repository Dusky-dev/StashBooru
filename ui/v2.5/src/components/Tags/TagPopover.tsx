import React from "react";
import { Placement } from "react-bootstrap/esm/Overlay";

interface ITagPopoverProps {
  id: string;
  hide?: boolean;
  placement?: Placement;
  target?: React.RefObject<HTMLElement>;
}

// Ordinary Tag links and selection chips no longer open hover previews.
// Preserve this component's public props so existing callers and UI extensions
// retain their link/selection markup without an extra DOM wrapper or data fetch.
// Artist, Character and Copyright previews use separate components.
export const TagPopover: React.FC<ITagPopoverProps> = ({ children }) => (
  <>{children}</>
);
