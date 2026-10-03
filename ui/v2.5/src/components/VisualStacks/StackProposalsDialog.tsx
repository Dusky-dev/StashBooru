import { useState } from "react";
import { Alert, Button } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "../Shared/Modal";
import { VisualStackDialog } from "./VisualStackDialog";
import { stackKey, stackPath } from "./identity";

export function StackProposalsDialog({
  selection,
  onClose,
}: {
  selection: GQL.MediaReferenceInput[];
  onClose: () => void;
}) {
  const valid = selection.length >= 2 && selection.length <= 100;
  const query = GQL.useVisualStackProposalsQuery({
    variables: { media: selection },
    skip: !valid,
    fetchPolicy: "network-only",
  });
  const [review, setReview] = useState<GQL.MediaReferenceInput[]>();
  const [page, setPage] = useState(0);
  const proposals = query.data?.visualStackProposals ?? [];
  if (review) return <VisualStackDialog selection={review} onClose={onClose} />;
  return (
    <ModalComponent
      show
      header="Review stack proposals"
      onHide={onClose}
      accept={{ text: "Close", onClick: onClose }}
    >
      <p>
        Review each pair before creating a stack. Proposals use active-file
        hashes, recorded derivation, or both pHash and current EVA02 evidence.
        They do not modify files or join pairs transitively.
      </p>
      {!valid && (
        <Alert variant="warning">Select between 2 and 100 members.</Alert>
      )}
      {query.error && <Alert variant="danger">{query.error.message}</Alert>}
      {!query.loading && valid && proposals.length === 0 && (
        <p>No strong pairwise evidence was found for this selection.</p>
      )}
      {proposals.slice(page * 50, (page + 1) * 50).map((p) => (
        <div
          className="visual-stack-proposal"
          key={p.members.map(stackKey).join("+")}
        >
          <div>
            {p.members.map((m) => (
              <a className="mr-2" key={stackKey(m)} href={stackPath(m)}>
                {stackKey(m)}
              </a>
            ))}
          </div>
          <small>
            {p.evidence}
            {p.score != null ? ` · ${(p.score * 100).toFixed(1)}% cosine` : ""}
          </small>
          <Button size="sm" onClick={() => setReview(p.members)}>
            Review pair
          </Button>
        </div>
      ))}
      {proposals.length > 50 && (
        <div className="mt-2">
          {proposals.length} pairs{" "}
          <Button disabled={page === 0} onClick={() => setPage(page - 1)}>
            Previous
          </Button>{" "}
          <Button
            disabled={(page + 1) * 50 >= proposals.length}
            onClick={() => setPage(page + 1)}
          >
            Next
          </Button>
        </div>
      )}
    </ModalComponent>
  );
}
