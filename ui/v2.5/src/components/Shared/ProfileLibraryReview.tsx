import { FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";

export function ProfileLibraryReview({
  kind,
  name,
}: {
  kind: "character" | "artist" | "copyright";
  name: string;
}) {
  const intl = useIntl();
  const query = new URLSearchParams({ tab: "tools", alias: name });
  return (
    <details className="profile-library-review my-3">
      <summary>
        <FormattedMessage
          id="profile_library_review.title"
          defaultMessage="Library review"
        />
      </summary>
      <p className="small text-muted mt-2">
        <FormattedMessage
          id="profile_library_review.help"
          defaultMessage="Inspect this name or review library-wide cleanup. Links open in a new tab; nothing runs automatically."
        />
      </p>
      <ul className="pl-3 mb-0">
        <li>
          <Link
            to={{
              pathname: "/settings",
              search: `?${query}`,
              hash: "#tools-aliases",
            }}
            target="_blank"
            rel="noreferrer"
          >
            {intl.formatMessage({
              id: "profile_library_review.inspect_name",
              defaultMessage: "Inspect this name",
            })}
          </Link>
        </li>
        <li>
          <Link
            to="/settings?tab=tasks#association-inheritance-task"
            target="_blank"
            rel="noreferrer"
          >
            <FormattedMessage
              id="profile_library_review.inheritance"
              defaultMessage="Review inherited associations (whole library)"
            />
          </Link>
        </li>
        {kind === "character" && (
          <li>
            <Link
              to="/settings?tab=tasks#character-dedup-task"
              target="_blank"
              rel="noreferrer"
            >
              <FormattedMessage
                id="profile_library_review.duplicates"
                defaultMessage="Review duplicate Characters (whole library)"
              />
            </Link>
          </li>
        )}
      </ul>
    </details>
  );
}
