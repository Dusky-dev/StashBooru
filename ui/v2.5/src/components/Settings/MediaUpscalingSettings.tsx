import React, { useEffect, useState } from "react";
import { Alert, Button, Card, Form, Spinner } from "react-bootstrap";

import { useToast } from "src/hooks/Toast";
import {
  conversionEndpoint,
  conversionResponse,
  ConversionFormat,
  ConversionUpscaler,
  loadUpscalingDefaults,
  saveUpscalingDefaults,
  UpscalingDefaults,
} from "../Shared/mediaConversion";

interface Capabilities {
  formats: ConversionFormat[];
  upscalers?: ConversionUpscaler[];
  backend: string;
  notice?: string;
}

export const MediaUpscalingSettings: React.FC = () => {
  const Toast = useToast();
  const [defaults, setDefaults] = useState<UpscalingDefaults>(() =>
    loadUpscalingDefaults()
  );
  const [capabilities, setCapabilities] = useState<Capabilities>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    void fetch(`${conversionEndpoint}?capabilities=1`, {
      signal: controller.signal,
    })
      .then(conversionResponse<Capabilities>)
      .then((value) => {
        if (!controller.signal.aborted) {
          setCapabilities(value);
          setError("");
        }
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, []);

  const upscalers = capabilities?.upscalers ?? [];
  const imageFormats = (capabilities?.formats ?? []).filter(
    (format) => format.family !== "video" && format.available
  );
  const selectedUpscaler = upscalers.find(
    (upscaler) => upscaler.id === defaults.upscaler
  );

  return (
    <div className="setting-section" id="media-upscaling">
      <h1>Image upscaling</h1>
      <div className="sub-heading">
        Defaults for the dedicated image upscaler. Upscaling uses the shared
        inference worker and remains separate from normal media conversion.
      </div>
      <Card className="p-3">
        {error && <Alert variant="danger">{error}</Alert>}
        {loading && <Spinner animation="border" role="status" />}
        {!loading && (
          <>
            <Form.Group controlId="upscaling-default-model">
              <Form.Label>Upscaler</Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={defaults.upscaler}
                onChange={(event) =>
                  setDefaults({ ...defaults, upscaler: event.target.value })
                }
              >
                {upscalers.length === 0 && (
                  <option value={defaults.upscaler}>No upscalers detected</option>
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
              <Form.Text className="text-muted">
                {selectedUpscaler?.notice ||
                  "waifu2x and SeedVR2 models are installed on the selected worker, not downloaded by StashBooru."}
              </Form.Text>
            </Form.Group>

            <Form.Group controlId="upscaling-default-scale">
              <Form.Label>Scale</Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={defaults.scale}
                onChange={(event) =>
                  setDefaults({
                    ...defaults,
                    scale: Number(event.target.value) === 4 ? 4 : 2,
                  })
                }
              >
                <option value={2}>2×</option>
                <option value={4}>4×</option>
              </Form.Control>
            </Form.Group>

            <Form.Group controlId="upscaling-default-processor">
              <Form.Label>Processor</Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={defaults.hardware}
                onChange={(event) =>
                  setDefaults({
                    ...defaults,
                    hardware: event.target.value as UpscalingDefaults["hardware"],
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

            <Form.Group controlId="upscaling-default-format">
              <Form.Label>Output format</Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={defaults.format}
                onChange={(event) =>
                  setDefaults({ ...defaults, format: event.target.value })
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

            <p className="text-muted">
              Worker: {capabilities?.backend ?? "unknown"}. Upscaling always
              keeps larger outputs and retains the original through the existing
              restore-cache flow.
            </p>
            {capabilities?.notice && (
              <Alert variant="info">{capabilities.notice}</Alert>
            )}
            <div className="d-flex justify-content-end">
              <Button
                onClick={() => {
                  saveUpscalingDefaults(defaults);
                  Toast.success("Saved image upscaling defaults.");
                }}
              >
                Save upscaling defaults
              </Button>
            </div>
          </>
        )}
      </Card>
    </div>
  );
};
