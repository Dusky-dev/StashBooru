import React, { useEffect, useState } from "react";
import { Alert, Button, Col, Form, Modal, Row, Spinner } from "react-bootstrap";
import { Link } from "react-router-dom";

import {
  conversionEndpoint,
  conversionResponse,
  ConversionFormat,
  ConversionUpscaler,
  loadUpscalingDefaults,
  UpscalingDefaults,
} from "./mediaConversion";

interface Capabilities {
  formats: ConversionFormat[];
  upscalers?: ConversionUpscaler[];
  backend: string;
  notice?: string;
}

export const MediaUpscalingDialog: React.FC<{
  selectedIds: string[];
  onHide: () => void;
}> = ({ selectedIds, onHide }) => {
  const [options, setOptions] = useState<UpscalingDefaults>(() =>
    loadUpscalingDefaults()
  );
  const [capabilities, setCapabilities] = useState<Capabilities>();
  const [loading, setLoading] = useState(true);
  const [attempt, setAttempt] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [jobID, setJobID] = useState<number>();
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    void fetch(`${conversionEndpoint}?capabilities=1&refresh=${attempt}`, {
      signal: controller.signal,
    })
      .then(conversionResponse<Capabilities>)
      .then((value) => {
        if (!controller.signal.aborted) setCapabilities(value);
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [attempt]);

  const upscalers = capabilities?.upscalers ?? [];
  const selectedUpscaler = upscalers.find((u) => u.id === options.upscaler);
  const imageFormats = (capabilities?.formats ?? []).filter(
    (format) => format.family !== "video" && format.available
  );
  const selectedFormat =
    options.format === "auto"
      ? undefined
      : imageFormats.find((format) => format.id === options.format);
  const ready =
    selectedIds.length > 0 &&
    !!selectedUpscaler?.available &&
    (options.hardware !== "cpu" || selectedUpscaler.cpu) &&
    (options.format === "auto" || !!selectedFormat);

  async function start() {
    setSubmitting(true);
    setError("");
    try {
      const result = await fetch(conversionEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "start",
          useQuality: true,
          useEncodingDefaults: true,
          targets: selectedIds.map((id) => ({ kind: "image", id: Number(id) })),
          options: {
            upscaler: options.upscaler,
            upscaleScale: options.scale,
            format: options.format,
            hardware: options.hardware,
            quality: 90,
            effort: 7,
            allowLarger: true,
            lossless: false,
            dropAudio: false,
            allowAlphaLoss: false,
          },
        }),
      }).then(conversionResponse<{ jobID: number }>);
      setJobID(result.jobID);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal show onHide={onHide} size="lg">
      <Modal.Header closeButton>
        <Modal.Title>Upscale images</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <p>
          Upscale {selectedIds.length} selected image
          {selectedIds.length === 1 ? "" : "s"} without mixing upscaler controls
          into the normal conversion workflow. Existing metadata stays on the
          same Image entries and originals remain restorable.
        </p>
        {error && <Alert variant="danger">{error}</Alert>}
        {jobID && (
          <Alert variant="success">
            Upscaling job #{jobID} queued. It continues after this dialog is
            closed.
          </Alert>
        )}
        {loading ? (
          <p>
            <Spinner animation="border" size="sm" /> Checking worker…
          </p>
        ) : (
          <>
            <Row>
              <Form.Group as={Col} xs={12} md={6} controlId="upscaler-model">
                <Form.Label>Upscaler</Form.Label>
                <Form.Control
                  as="select"
                  className="input-control"
                  value={options.upscaler}
                  disabled={submitting || !!jobID}
                  onChange={(event) =>
                    setOptions({ ...options, upscaler: event.target.value })
                  }
                >
                  {upscalers.length === 0 && (
                    <option value={options.upscaler}>
                      No upscalers detected
                    </option>
                  )}
                  {upscalers.map((upscaler) => (
                    <option
                      key={upscaler.id}
                      value={upscaler.id}
                      disabled={!upscaler.available}
                    >
                      {upscaler.label}
                      {!upscaler.available ? " — not installed" : ""}
                    </option>
                  ))}
                </Form.Control>
              </Form.Group>
              <Form.Group as={Col} xs={12} md={6} controlId="upscaler-scale">
                <Form.Label>Scale</Form.Label>
                <Form.Control
                  as="select"
                  className="input-control"
                  value={options.scale}
                  disabled={submitting || !!jobID}
                  onChange={(event) =>
                    setOptions({
                      ...options,
                      scale: Number(event.target.value) === 4 ? 4 : 2,
                    })
                  }
                >
                  <option value={2}>2×</option>
                  <option value={4}>4×</option>
                </Form.Control>
              </Form.Group>
            </Row>
            <Row>
              <Form.Group
                as={Col}
                xs={12}
                md={6}
                controlId="upscaler-processor"
              >
                <Form.Label>Processor</Form.Label>
                <Form.Control
                  as="select"
                  className="input-control"
                  value={options.hardware}
                  disabled={submitting || !!jobID}
                  onChange={(event) =>
                    setOptions({
                      ...options,
                      hardware: event.target
                        .value as UpscalingDefaults["hardware"],
                    })
                  }
                >
                  <option value="auto">Prefer GPU, otherwise CPU</option>
                  <option value="gpu">GPU</option>
                  <option
                    value="cpu"
                    disabled={selectedUpscaler?.cpu === false}
                  >
                    CPU
                  </option>
                </Form.Control>
              </Form.Group>
              <Form.Group as={Col} xs={12} md={6} controlId="upscaler-format">
                <Form.Label>Output format</Form.Label>
                <Form.Control
                  as="select"
                  className="input-control"
                  value={options.format}
                  disabled={submitting || !!jobID}
                  onChange={(event) =>
                    setOptions({ ...options, format: event.target.value })
                  }
                >
                  <option value="auto">Use conversion format default</option>
                  {imageFormats.map((format) => (
                    <option key={format.id} value={format.id}>
                      {format.label}
                    </option>
                  ))}
                </Form.Control>
              </Form.Group>
            </Row>
            <p className="text-muted">
              Worker: {capabilities?.backend ?? "unknown"}.{" "}
              {selectedUpscaler?.notice}
            </p>
            {capabilities?.notice && (
              <Alert variant="info">{capabilities.notice}</Alert>
            )}
            {!selectedUpscaler?.available && (
              <Alert variant="secondary">
                Configure the local executable/model paths in System → Image
                upscaling, or configure the equivalent STASH_WAIFU2X_* /
                STASH_SEEDVR2_* environment variables on a remote worker.
              </Alert>
            )}
            <p>
              <Link to="/settings?tab=system#media-upscaling" onClick={onHide}>
                Edit upscaler defaults and model paths in System settings
              </Link>
            </p>
            <Button
              size="sm"
              variant="secondary"
              className="mr-2"
              disabled={submitting || !!jobID}
              onClick={() => setAttempt((value) => value + 1)}
            >
              Recheck worker
            </Button>
            <Button
              disabled={submitting || !!jobID || !ready}
              onClick={() => void start()}
            >
              {submitting
                ? "Queuing…"
                : `Upscale ${selectedIds.length === 1 ? "image" : `${selectedIds.length} images`}`}
            </Button>
          </>
        )}
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" onClick={onHide}>
          Close
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
