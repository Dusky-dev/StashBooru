import React, { useState } from "react";
import { Alert, Button } from "react-bootstrap";

import { useToast } from "src/hooks/Toast";
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
      <h2>Animation inspection</h2>
      <p>
        Scans inspect image frame counts and automatically add the native
        <strong> animated</strong> tag without replacing existing metadata. Use
        this task to backfill images that were scanned before animation
        detection was enabled.
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
