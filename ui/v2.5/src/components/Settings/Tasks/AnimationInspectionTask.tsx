import React, { useState } from "react";
import { Alert, Button } from "react-bootstrap";
import { faQuestionCircle } from "@fortawesome/free-solid-svg-icons";

import { ManualLink } from "src/components/Help/context";
import { Icon } from "src/components/Shared/Icon";
import { useToast } from "src/hooks/Toast";
import { Setting } from "../Inputs";
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
    <Setting
      id="animation-inspection-task"
      heading={
        <>
          Animation inspection{" "}
          <ManualLink tab="Tasks">
            <Icon icon={faQuestionCircle} size="xs" aria-hidden="true" />
            <span className="sr-only">Help with Animation inspection</span>
          </ManualLink>
        </>
      }
      subHeading={
        <>
          Inspect existing Images imported before automatic animation detection
          was available. Regular scans already inspect new Images; this action
          checks the whole library, not just folders selected for a scan.
          {error && (
            <Alert variant="danger" role="alert" className="mt-2 mb-0">
              {error}
            </Alert>
          )}
        </>
      }
    >
      <Button
        variant="secondary"
        disabled={submitting}
        onClick={() => void inspectExistingImages()}
      >
        {submitting ? "Queuing…" : "Inspect existing images"}
      </Button>
    </Setting>
  );
};
