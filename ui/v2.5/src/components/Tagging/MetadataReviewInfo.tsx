import { Button } from "react-bootstrap";
import { Link } from "react-router-dom";

export interface MetadataReviewContext {
  index: number;
  total: number;
  onSkip: () => void;
}

export function MetadataReviewInfo({
  kind,
  id,
  context,
  busy,
}: {
  kind: "Image" | "Video";
  id: string;
  context?: MetadataReviewContext;
  busy: boolean;
}) {
  return (
    <section
      className="metadata-review-info mb-3"
      aria-label="Metadata review scope"
    >
      <div className="d-flex flex-wrap align-items-center mb-2">
        <strong className="mr-3">
          {kind} #{id}
        </strong>
        {context && (
          <>
            <span className="mr-3" role="status">
              Item {context.index + 1} of {context.total}
            </span>
            <Button
              size="sm"
              variant="secondary"
              disabled={busy}
              onClick={context.onSkip}
            >
              Skip item
            </Button>
          </>
        )}
      </div>
      <p className="small mb-1">
        Loads local metadata first.{" "}
        {kind === "Image"
          ? "Provider lookup, model analysis and library suggestions run only when you choose them."
          : "Provider lookup uses MD5 matches; frame analysis runs only when you choose it."}{" "}
        Review the proposed changes before applying.
      </p>
      {context && (
        <p className="small mb-1">
          Apply continues through your selection. Skip leaves this item
          unchanged. Close ends the review and keeps your selection.
        </p>
      )}
      <Link
        className="small"
        target="_blank"
        rel="noreferrer"
        to="/settings?tab=processing#visual-similarity"
      >
        Tagging setup (new tab)
      </Link>
    </section>
  );
}
