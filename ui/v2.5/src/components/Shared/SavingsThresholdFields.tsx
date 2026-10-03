import React from "react";
import { Col, Form, Row } from "react-bootstrap";
import { SavingsThresholds } from "./mediaConversion";

export function validSavings(value: SavingsThresholds) {
  return (
    Number.isSafeInteger(value.minimumSavedBytes) &&
    value.minimumSavedBytes >= 0 &&
    Number.isFinite(value.minimumSavedPercent) &&
    value.minimumSavedPercent >= 0 &&
    value.minimumSavedPercent <= 100
  );
}

export const SavingsThresholdFields: React.FC<{
  value: SavingsThresholds;
  onChange: (value: SavingsThresholds) => void;
  disabled?: boolean;
  prefix: string;
}> = ({ value, onChange, disabled, prefix }) => (
  <>
    <Row>
      <Form.Group as={Col} xs={12} sm={6} controlId={`${prefix}-saved-bytes`}>
        <Form.Label>Minimum saved bytes</Form.Label>
        <Form.Control
          className="text-input"
          type="number"
          min={0}
          step={1}
          value={value.minimumSavedBytes}
          disabled={disabled}
          onChange={(e) =>
            onChange({ ...value, minimumSavedBytes: Number(e.target.value) })
          }
        />
      </Form.Group>
      <Form.Group as={Col} xs={12} sm={6} controlId={`${prefix}-saved-percent`}>
        <Form.Label>Minimum savings (%)</Form.Label>
        <Form.Control
          className="text-input"
          type="number"
          min={0}
          max={100}
          step={0.1}
          value={value.minimumSavedPercent}
          disabled={disabled}
          onChange={(e) =>
            onChange({ ...value, minimumSavedPercent: Number(e.target.value) })
          }
        />
      </Form.Group>
    </Row>
    <p className="text-muted">
      Both minimums must pass. Zero disables that minimum; compression still
      requires a smaller output. Skipped items keep their source and create no
      original backup. Upscaling uses separate controls.
    </p>
  </>
);
