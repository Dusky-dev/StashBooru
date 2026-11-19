import { faGlobe } from "@fortawesome/free-solid-svg-icons";
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

/**
 * The site a connected identity came from. Tag identities open the Booru
 * browser on that site, searching the connected tag. A generic icon is used
 * so no third-party favicon is requested.
 */
export function IdentitySiteLink({
  row,
  link = true,
}: {
  row: Pick<ProviderIdentity, "sourceID" | "sourceName" | "target" | "form">;
  /** Off inside dialogs, which can sit on top of the Booru browser. */
  link?: boolean;
}) {
  const intl = useIntl();
  const content = (
    <>
      <Icon icon={faGlobe} />
      <span>{row.sourceName}</span>
    </>
  );
  if (!link || row.form !== "tag" || !row.sourceID || !row.target)
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
