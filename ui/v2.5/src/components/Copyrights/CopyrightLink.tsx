import React from "react";
import { Badge } from "react-bootstrap";
import { Link } from "react-router-dom";

interface CopyrightLinkProps {
  copyright: {
    id: string;
    name: string;
    sort_name?: string | null;
  };
  className?: string;
}

export const CopyrightLink: React.FC<CopyrightLinkProps> = ({
  copyright,
  className,
}) => (
  <Badge
    data-name={className}
    data-sort-name={copyright.sort_name || copyright.name}
    className={`tag-item tag-link${className ? ` ${className}` : ""}`}
    variant="secondary"
  >
    <Link to={`/copyrights/${copyright.id}`}>{copyright.name}</Link>
  </Badge>
);
