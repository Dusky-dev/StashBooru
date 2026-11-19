import { faGlobe } from "@fortawesome/free-solid-svg-icons";
import { useState } from "react";
import { useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { Icon } from "../Shared/Icon";
import type { ProviderIdentity } from "./identityApi";

export function booruSearchPath(
  row: Pick<ProviderIdentity, "sourceID" | "target">
) {
  const params = new URLSearchParams({ source: row.sourceID, q: row.target });
  return `/booru?${params}`;
}

/** The site's own icon, served through this server; a globe if it has none. */
export function SiteIcon({ sourceID }: { sourceID: string }) {
  const [failed, setFailed] = useState(false);
  if (failed || !sourceID) return <Icon icon={faGlobe} />;
  return (
    <img
      className="site-icon"
      src={`booru-browser/icon?${new URLSearchParams({ source: sourceID })}`}
      alt=""
      width={16}
      height={16}
      loading="lazy"
      onError={() => setFailed(true)}
    />
  );
}

/**
 * The site an identity came from, opening the Booru browser on that site.
 * Tags search the tag; accounts (Pawchive/Kemono creator:service:id, Pixiv
 * user:id) open that creator's posts, which the browser accepts as a query. The icon is fetched by
 * the server from the configured site, never directly by the browser.
 */
export function IdentitySiteLink({
  row,
  link = true,
}: {
  row: Pick<ProviderIdentity, "sourceID" | "sourceName" | "target">;
  /** Off inside dialogs, which can sit on top of the Booru browser. */
  link?: boolean;
}) {
  const intl = useIntl();
  const content = (
    <>
      <SiteIcon sourceID={row.sourceID} />
      <span>{row.sourceName}</span>
    </>
  );
  if (!link || !row.sourceID || !row.target)
    return <span className="identity-site-link">{content}</span>;
  const label = intl.formatMessage(
    {
      id: "identity_sources.open_in_booru",
      defaultMessage: "Search {target} on {source} in the Booru browser",
    },
    { target: row.target, source: row.sourceName }
  );
  return (
    <Link
      className="identity-site-link"
      to={booruSearchPath(row)}
      title={label}
      aria-label={label}
    >
      {content}
    </Link>
  );
}
