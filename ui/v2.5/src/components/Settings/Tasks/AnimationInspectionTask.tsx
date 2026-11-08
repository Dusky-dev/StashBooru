import React, { useState } from "react";
import { Alert, Button } from "react-bootstrap";

import { useToast } from "src/hooks/Toast";
import { TaskHelpHeading } from "./TaskHelpHeading";
import {
  conversionEndpoint,
  conversionResponse,
} from "../../Shared/mediaConversion";

export const AnimationInspectionTask: React.FC = () => {
  const Toast = useToast();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  async function inspectExistingImages() {
    setSubmitting(true);
    setError("");
    try {
      await fetch(conversionEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "inspect-animations" }),
      }).then(conversionResponse);
      Toast.success("Queued animation inspection.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div id="animation-inspection-task">
      <TaskHelpHeading title="Animation inspection" />
      <p className="text-muted">
        Regular scans detect animation in new Images. Run this once to inspect
        existing Images imported before that detection was available.
      </p>
      {error && <Alert variant="danger">{error}</Alert>}
      <Button
        variant="secondary"
        disabled={submitting}
        onClick={() => void inspectExistingImages()}
      >
        {submitting ? "Queuing…" : "Inspect existing images"}
      </Button>
    </div>
  );
};
