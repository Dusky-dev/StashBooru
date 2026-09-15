import React, { useMemo } from "react";
import { Badge, Button, Modal, Spinner } from "react-bootstrap";

export interface TaggingPlanPrediction {
  name: string;
  category: string;
  score: number;
  source?: string;
}

export type TaggingPlanAction = "reuse" | "create" | "review" | "suppressed";
export type TaggingPlanRelationshipMode = "add" | "replace";

export interface TaggingChangePlanItem {
  prediction: TaggingPlanPrediction;
  action: TaggingPlanAction;
  relationshipMode: TaggingPlanRelationshipMode;
  reason?: string;
}

export interface TaggingChangePlan {
  items: TaggingChangePlanItem[];
  canApply: boolean;
  reviewCount: number;
  suppressedCount: number;
}

interface IProps {
  show: boolean;
  title: string;
  plan?: TaggingChangePlan;
  busy?: boolean;
  onBack: () => void;
  onApply: () => void;
  onHide: () => void;
}

function actionVariant(action: TaggingPlanAction) {
  switch (action) {
    case "reuse":
      return "primary";
    case "create":
      return "success";
    case "review":
      return "warning";
    case "suppressed":
      return "secondary";
  }
}

function actionLabel(action: TaggingPlanAction) {
  switch (action) {
    case "reuse":
      return "reuse existing";
    case "create":
      return "create new";
    case "review":
      return "needs review";
    case "suppressed":
      return "suppressed";
  }
}

export const TaggingChangePlanModal: React.FC<IProps> = ({
  show,
  title,
  plan,
  busy = false,
  onBack,
  onApply,
  onHide,
}) => {
  const activeCount = useMemo(
    () =>
      plan?.items.filter(
        (item) => item.action !== "suppressed" && item.action !== "review"
      ).length ?? 0,
    [plan]
  );

  return (
    <Modal show={show} onHide={busy ? undefined : onHide} size="lg" centered>
      <Modal.Header closeButton={!busy}>
        <Modal.Title>{title}</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        {!plan ? (
          <div className="text-center py-4">
            <Spinner animation="border" role="status" />
          </div>
        ) : (
          <>
            <div className="alert alert-info py-2">
              {activeCount} metadata item{activeCount === 1 ? "" : "s"} will be
              applied. {plan.suppressedCount} suppressed; {plan.reviewCount}{" "}
              need review.
            </div>

            {!plan.canApply ? (
              <div className="alert alert-warning py-2">
                This plan still contains unresolved metadata. Go back and
                resolve those items before applying.
              </div>
            ) : null}

            <div style={{ maxHeight: "55vh", overflowY: "auto" }}>
              {plan.items.map((item, index) => (
                <div
                  className="d-flex flex-wrap align-items-center py-2 border-bottom"
                  key={`${item.prediction.category}-${item.prediction.name}-${index}`}
                >
                  <div className="mr-2">
                    <strong>{item.prediction.name}</strong>
                    <span className="ml-2 text-muted small">
                      {item.prediction.category}
                    </span>
                  </div>
                  <Badge className="mr-2" variant={actionVariant(item.action)}>
                    {actionLabel(item.action)}
                  </Badge>
                  {item.prediction.category === "artist" ? (
                    <Badge className="mr-2" variant="secondary">
                      {item.relationshipMode}
                    </Badge>
                  ) : null}
                  {item.prediction.source ? (
                    <span className="small text-muted mr-2">
                      {item.prediction.source}
                    </span>
                  ) : null}
                  {item.reason ? (
                    <span className="small text-muted ml-auto">
                      {item.reason}
                    </span>
                  ) : null}
                </div>
              ))}
            </div>
          </>
        )}
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" disabled={busy} onClick={onBack}>
          Back
        </Button>
        <Button
          variant="primary"
          disabled={busy || !plan?.canApply || activeCount === 0}
          onClick={onApply}
        >
          {busy ? "Applying…" : "Apply this plan"}
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
