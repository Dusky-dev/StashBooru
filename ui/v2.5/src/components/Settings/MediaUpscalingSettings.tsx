import React, { useEffect, useState } from "react";
import { Alert, Button, Card, Col, Form, Row, Spinner } from "react-bootstrap";

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

interface LocalUpscalerConfig {
  waifu2xExecutable: string;
  waifu2xModels: string;
  seedVR2CLI: string;
  seedVR2Models: string;
  seedVR2Model: string;
  seedVR2Python: string;
  seedVR2BlocksToSwap: number;
}

const upscalerConfigEndpoint = "image/upscaler-config";

const emptyLocalConfig: LocalUpscalerConfig = {
  waifu2xExecutable: "",
  waifu2xModels: "",
  seedVR2CLI: "",
  seedVR2Models: "",
  seedVR2Model: "",
  seedVR2Python: "",
  seedVR2BlocksToSwap: 0,
};

export const MediaUpscalingSettings: React.FC = () => {
  const Toast = useToast();
  const [defaults, setDefaults] = useState<UpscalingDefaults>(() =>
    loadUpscalingDefaults()
  );
  const [capabilities, setCapabilities] = useState<Capabilities>();
  const [localCapabilities, setLocalCapabilities] = useState<Capabilities>();
  const [localConfig, setLocalConfig] =
    useState<LocalUpscalerConfig>(emptyLocalConfig);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    void Promise.all([
      fetch(`${conversionEndpoint}?capabilities=1&refresh=${attempt}`, {
        signal: controller.signal,
      }).then(conversionResponse<Capabilities>),
      fetch(
        `${conversionEndpoint}?capabilities=1&backend=local&refresh=${attempt}`,
        { signal: controller.signal }
      ).then(conversionResponse<Capabilities>),
      fetch(upscalerConfigEndpoint, { signal: controller.signal }).then(
        conversionResponse<LocalUpscalerConfig>
      ),
    ])
      .then(([selected, local, config]) => {
        if (controller.signal.aborted) return;
        setCapabilities(selected);
        setLocalCapabilities(local);
        setLocalConfig({ ...emptyLocalConfig, ...config });
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
  const localUpscalers = localCapabilities?.upscalers ?? [];
  const imageFormats = (capabilities?.formats ?? []).filter(
    (format) => format.family !== "video" && format.available
  );
  const selectedUpscaler = upscalers.find(
    (upscaler) => upscaler.id === defaults.upscaler
  );
  const localWaifu2x = localUpscalers.find((u) => u.id === "waifu2x");
  const localSeedVR2 = localUpscalers.find((u) => u.id === "seedvr2");

  function updateLocal<K extends keyof LocalUpscalerConfig>(
    key: K,
    value: LocalUpscalerConfig[K]
  ) {
    setLocalConfig((current) => ({ ...current, [key]: value }));
  }

  async function save() {
    setSaving(true);
    setError("");
    try {
      await fetch(upscalerConfigEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(localConfig),
      }).then(conversionResponse<LocalUpscalerConfig>);
      saveUpscalingDefaults(defaults);
      Toast.success("Saved image upscaling settings.");
      setAttempt((value) => value + 1);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="setting-section" id="media-upscaling">
      <h1>Image upscaling</h1>
      <div className="sub-heading">
        Dedicated image-upscaling defaults and local worker model paths.
        Upscaling remains separate from normal media conversion.
      </div>

      {error && <Alert variant="danger">{error}</Alert>}
      {loading && <Spinner animation="border" role="status" />}
      {!loading && (
        <>
          <Card className="p-3 mb-3">
            <h4>Job defaults</h4>
            <Form.Group controlId="upscaling-default-model">
              <Form.Label>Upscaler</Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={defaults.upscaler}
                disabled={saving}
                onChange={(event) =>
                  setDefaults({ ...defaults, upscaler: event.target.value })
                }
              >
                {upscalers.length === 0 && (
                  <option value={defaults.upscaler}>
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
              <Form.Text className="text-muted">
                {selectedUpscaler?.notice ||
                  "Choose an upscaler available on the selected worker."}
              </Form.Text>
            </Form.Group>

            <Row>
              <Form.Group
                as={Col}
                xs={12}
                md={4}
                controlId="upscaling-default-scale"
              >
                <Form.Label>Scale</Form.Label>
                <Form.Control
                  as="select"
                  className="input-control"
                  value={defaults.scale}
                  disabled={saving}
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

              <Form.Group
                as={Col}
                xs={12}
                md={4}
                controlId="upscaling-default-processor"
              >
                <Form.Label>Processor</Form.Label>
                <Form.Control
                  as="select"
                  className="input-control"
                  value={defaults.hardware}
                  disabled={saving}
                  onChange={(event) =>
                    setDefaults({
                      ...defaults,
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

              <Form.Group
                as={Col}
                xs={12}
                md={4}
                controlId="upscaling-default-format"
              >
                <Form.Label>Output format</Form.Label>
                <Form.Control
                  as="select"
                  className="input-control"
                  value={defaults.format}
                  disabled={saving}
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
            </Row>

            <p className="text-muted mb-0">
              Selected worker: {capabilities?.backend ?? "unknown"}. Upscaling
              keeps larger outputs and retains the original through the existing
              restore-cache flow.
            </p>
            {capabilities?.notice && (
              <Alert variant="info" className="mt-2 mb-0">
                {capabilities.notice}
              </Alert>
            )}
          </Card>

          <Card className="p-3">
            <h4>Local worker paths</h4>
            <p className="text-muted">
              These are filesystem paths on this StashBooru server. They are
              saved server-side and are never sent in an upscaling request. Leave
              a field blank to use the process environment or built-in worker
              default.
            </p>
            {capabilities?.backend === "remote" && (
              <Alert variant="info">
                Automatic mode is currently using the remote inference worker.
                The paths below configure the local fallback only. Configure the
                equivalent STASH_WAIFU2X_* / STASH_SEEDVR2_* variables on that
                worker.
              </Alert>
            )}

            <h5>waifu2x</h5>
            <Row>
              <Form.Group
                as={Col}
                xs={12}
                md={6}
                controlId="upscaling-waifu2x-executable"
              >
                <Form.Label>Executable path</Form.Label>
                <Form.Control
                  type="text"
                  className="text-input"
                  value={localConfig.waifu2xExecutable}
                  disabled={saving}
                  placeholder="/usr/bin/waifu2x-ncnn-vulkan"
                  onChange={(event) =>
                    updateLocal("waifu2xExecutable", event.target.value)
                  }
                />
                <Form.Text className="text-muted">
                  STASH_WAIFU2X. Blank uses waifu2x-ncnn-vulkan from PATH.
                </Form.Text>
              </Form.Group>
              <Form.Group
                as={Col}
                xs={12}
                md={6}
                controlId="upscaling-waifu2x-models"
              >
                <Form.Label>Model directory path</Form.Label>
                <Form.Control
                  type="text"
                  className="text-input"
                  value={localConfig.waifu2xModels}
                  disabled={saving}
                  placeholder="/path/to/models-cunet"
                  onChange={(event) =>
                    updateLocal("waifu2xModels", event.target.value)
                  }
                />
                <Form.Text className="text-muted">
                  STASH_WAIFU2X_MODELS. Must contain the waifu2x .param and .bin
                  model files.
                </Form.Text>
              </Form.Group>
            </Row>
            <Alert variant={localWaifu2x?.available ? "success" : "secondary"}>
              Local waifu2x: {localWaifu2x?.available ? "ready" : "not ready"}.{" "}
              {localWaifu2x?.notice}
            </Alert>

            <h5>SeedVR2</h5>
            <Row>
              <Form.Group
                as={Col}
                xs={12}
                md={6}
                controlId="upscaling-seedvr2-cli"
              >
                <Form.Label>CLI script path</Form.Label>
                <Form.Control
                  type="text"
                  className="text-input"
                  value={localConfig.seedVR2CLI}
                  disabled={saving}
                  placeholder="/path/to/SeedVR2/inference_cli.py"
                  onChange={(event) =>
                    updateLocal("seedVR2CLI", event.target.value)
                  }
                />
                <Form.Text className="text-muted">
                  STASH_SEEDVR2_CLI.
                </Form.Text>
              </Form.Group>
              <Form.Group
                as={Col}
                xs={12}
                md={6}
                controlId="upscaling-seedvr2-python"
              >
                <Form.Label>Python executable path</Form.Label>
                <Form.Control
                  type="text"
                  className="text-input"
                  value={localConfig.seedVR2Python}
                  disabled={saving}
                  placeholder="/path/to/venv/bin/python"
                  onChange={(event) =>
                    updateLocal("seedVR2Python", event.target.value)
                  }
                />
                <Form.Text className="text-muted">
                  STASH_SEEDVR2_PYTHON. Blank uses the converter worker Python.
                </Form.Text>
              </Form.Group>
            </Row>
            <Row>
              <Form.Group
                as={Col}
                xs={12}
                md={6}
                controlId="upscaling-seedvr2-models"
              >
                <Form.Label>Model directory path</Form.Label>
                <Form.Control
                  type="text"
                  className="text-input"
                  value={localConfig.seedVR2Models}
                  disabled={saving}
                  placeholder="/path/to/SeedVR2/models"
                  onChange={(event) =>
                    updateLocal("seedVR2Models", event.target.value)
                  }
                />
                <Form.Text className="text-muted">
                  STASH_SEEDVR2_MODELS. The directory must also contain
                  ema_vae_fp16.safetensors.
                </Form.Text>
              </Form.Group>
              <Form.Group
                as={Col}
                xs={12}
                md={6}
                controlId="upscaling-seedvr2-model"
              >
                <Form.Label>DiT model filename</Form.Label>
                <Form.Control
                  type="text"
                  className="text-input"
                  value={localConfig.seedVR2Model}
                  disabled={saving}
                  placeholder="seedvr2_ema_3b_fp8_e4m3fn.safetensors"
                  onChange={(event) =>
                    updateLocal("seedVR2Model", event.target.value)
                  }
                />
                <Form.Text className="text-muted">
                  STASH_SEEDVR2_MODEL. Blank uses the worker default model name.
                </Form.Text>
              </Form.Group>
            </Row>
            <Form.Group controlId="upscaling-seedvr2-blocks">
              <Form.Label>Blocks to swap</Form.Label>
              <Form.Control
                type="number"
                className="text-input"
                min={0}
                max={128}
                value={localConfig.seedVR2BlocksToSwap}
                disabled={saving}
                onChange={(event) =>
                  updateLocal(
                    "seedVR2BlocksToSwap",
                    Math.min(128, Math.max(0, Number(event.target.value) || 0))
                  )
                }
              />
              <Form.Text className="text-muted">
                STASH_SEEDVR2_BLOCKS_TO_SWAP. 0 uses the worker default (32).
              </Form.Text>
            </Form.Group>
            <Alert variant={localSeedVR2?.available ? "success" : "secondary"}>
              Local SeedVR2: {localSeedVR2?.available ? "ready" : "not ready"}.{" "}
              {localSeedVR2?.notice}
            </Alert>

            <div className="d-flex flex-wrap justify-content-end">
              <Button
                variant="secondary"
                className="mr-2 mb-2"
                disabled={saving}
                onClick={() => setAttempt((value) => value + 1)}
              >
                Recheck workers
              </Button>
              <Button
                className="mb-2"
                disabled={saving}
                onClick={() => void save()}
              >
                {saving ? "Saving…" : "Save upscaling settings"}
              </Button>
            </div>
          </Card>
        </>
      )}
    </div>
  );
};
