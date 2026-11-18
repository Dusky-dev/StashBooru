import { FormattedMessage } from "react-intl";
import { Link } from "react-router-dom";

/** Character profiles link to the library-wide duplicate review. */
export function ProfileLibraryReview() {
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
          defaultMessage="Review library-wide cleanup. Links open in a new tab; nothing runs automatically."
        />
      </p>
      <ul className="pl-3 mb-0">
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
      </ul>
    </details>
  );
}
