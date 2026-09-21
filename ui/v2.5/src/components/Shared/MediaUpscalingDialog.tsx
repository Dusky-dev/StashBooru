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

const upscaleEndpoint = "image/upscale";

type QueuedMode = "copy" | "replace";

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
  const [replaceOriginal, setReplaceOriginal] = useState(false);
  const [confirmReplace, setConfirmReplace] = useState(false);
  const [jobID, setJobID] = useState<number>();
  const [queuedMode, setQueuedMode] = useState<QueuedMode>();
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

  async function queueUpscale(replace: boolean) {
    setSubmitting(true);
    setError("");
    try {
      const common = {
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
      };
      const result = await fetch(replace ? conversionEndpoint : upscaleEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(
          replace
            ? { action: "start", useQuality: true, ...common }
            : common
        ),
      }).then(conversionResponse<{ jobID: number }>);
      setQueuedMode(replace ? "replace" : "copy");
      setJobID(result.jobID);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  function start() {
    if (replaceOriginal) {
      setConfirmReplace(true);
      return;
    }
    void queueUpscale(false);
  }

  function confirmDestructiveUpscale() {
    setConfirmReplace(false);
    void queueUpscale(true);
  }

  const oneImage = selectedIds.length === 1;

  return (
    <>
      <Modal show={!confirmReplace} onHide={onHide} size="lg">
        <Modal.Header closeButton>
          <Modal.Title>Upscale images</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p>
            Upscale {selectedIds.length} selected image
            {oneImage ? "" : "s"}. By default StashBooru creates a new Image
            and leaves the original Image and original file untouched.
          </p>
          {error && <Alert variant="danger">{error}</Alert>}
          {jobID && (
            <Alert variant="success">
              {queuedMode === "copy"
                ? `Non-destructive upscaling job #${jobID} queued. The original${
                    oneImage ? " stays" : "s stay"
                  } untouched.`
                : `Replacement upscaling job #${jobID} queued.`}
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

              <Form.Group className="mt-3" controlId="upscaler-save-behavior">
                <Form.Label>Save behavior</Form.Label>
                <Form.Check
                  type="radio"
                  id="upscaler-create-copy"
                  name="upscaler-save-behavior"
                  label="Create a new upscaled Image (recommended)"
                  checked={!replaceOriginal}
                  disabled={submitting || !!jobID}
                  onChange={() => setReplaceOriginal(false)}
                />
                <Form.Text className="text-muted d-block mb-2">
                  Keeps the original Image and file untouched. The new Image
                  inherits the source metadata and relations.
                </Form.Text>
                <Form.Check
                  type="radio"
                  id="upscaler-replace-original"
                  name="upscaler-save-behavior"
                  label={
                    oneImage
                      ? "Replace current image"
                      : "Replace selected images"
                  }
                  checked={replaceOriginal}
                  disabled={submitting || !!jobID}
                  onChange={() => setReplaceOriginal(true)}
                />
                <Form.Text className="text-muted d-block">
                  Replaces the active file using the converter restore-cache
                  workflow. You will be asked to confirm again before it starts.
                </Form.Text>
              </Form.Group>

              {replaceOriginal && (
                <Alert variant="warning" className="mt-3">
                  Replacement changes the active library file. The original is
                  kept only in the converter restore cache and may later be
                  evicted according to that cache limit.
                </Alert>
              )}

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
                onClick={start}
              >
                {submitting
                  ? "Queuing…"
                  : replaceOriginal
                    ? oneImage
                      ? "Upscale and replace image…"
                      : `Upscale and replace ${selectedIds.length} images…`
                    : oneImage
                      ? "Create upscaled image"
                      : `Create ${selectedIds.length} upscaled images`}
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

      <Modal
        show={confirmReplace}
        onHide={() => setConfirmReplace(false)}
        centered
      >
        <Modal.Header closeButton>
          <Modal.Title>
            {oneImage ? "Replace current image?" : "Replace selected images?"}
          </Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Alert variant="danger">
            <strong>Are you sure?</strong>
            <br />
            {oneImage
              ? "The current image file will be replaced by the upscaled result."
              : `The active files for ${selectedIds.length} images will be replaced by their upscaled results.`}
          </Alert>
          <p className="mb-0">
            This uses the destructive converter path. Originals are put in the
            restore cache rather than kept as normal library files, and cached
            originals can be evicted when the configured cache limit is reached.
          </p>
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={submitting}
            onClick={() => setConfirmReplace(false)}
          >
            Cancel
          </Button>
          <Button
            variant="danger"
            disabled={submitting}
            onClick={confirmDestructiveUpscale}
          >
            {submitting ? "Queuing…" : "Replace and upscale"}
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
};
