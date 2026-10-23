import React, { useCallback, useEffect, useRef, useState } from "react";
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
  const [refreshing, setRefreshing] = useState(false);
  const [configLoading, setConfigLoading] = useState(false);
  const [localSaved, setLocalSaved] = useState(false);
  const [localStatusError, setLocalStatusError] = useState("");
  const [error, setError] = useState("");
  const [localError, setLocalError] = useState("");
  const [configLoaded, setConfigLoaded] = useState(false);
  const mounted = useRef(true);

  const refreshController = useRef<AbortController>();
  const refreshAttempt = useRef(0);

  // Readiness checks never reload the editable server configuration.
  const refresh = useCallback(async (includeConfig = false) => {
    refreshController.current?.abort();
    const controller = new AbortController();
    refreshController.current = controller;
    const attempt = refreshAttempt.current++;
    setRefreshing(true);
    setLocalStatusError("");
    if (includeConfig) {
      setConfigLoading(true);
      setLocalSaved(false);
    }
    const [selected, local, config] = await Promise.allSettled([
      fetch(`${conversionEndpoint}?capabilities=1&refresh=${attempt}`, {
        signal: controller.signal,
      }).then(conversionResponse<Capabilities>),
      fetch(
        `${conversionEndpoint}?capabilities=1&backend=local&refresh=${attempt}`,
        { signal: controller.signal }
      ).then(conversionResponse<Capabilities>),
      includeConfig
        ? fetch(upscalerConfigEndpoint, { signal: controller.signal }).then(
            conversionResponse<LocalUpscalerConfig>
          )
        : Promise.resolve(undefined),
    ]);
    if (controller.signal.aborted || !mounted.current) return;
    setCapabilities(
      selected.status === "fulfilled" ? selected.value : undefined
    );
    setError(selected.status === "rejected" ? String(selected.reason) : "");
    setLocalCapabilities(
      local.status === "fulfilled" ? local.value : undefined
    );
    setLocalStatusError(
      local.status === "rejected" ? String(local.reason) : ""
    );
    if (includeConfig) {
      if (config.status === "fulfilled" && config.value) {
        setLocalConfig({ ...emptyLocalConfig, ...config.value });
        setConfigLoaded(true);
        setLocalError("");
      } else if (config.status === "rejected") {
        setConfigLoaded(false);
        setLocalError(String(config.reason));
      }
    }
    setConfigLoading(false);
    setRefreshing(false);
    setLoading(false);
  }, []);

  useEffect(() => {
    mounted.current = true;
    void refresh(true);
    return () => {
      mounted.current = false;
      refreshController.current?.abort();
    };
  }, [refresh]);

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
    setLocalSaved(false);
    setLocalConfig((current) => ({ ...current, [key]: value }));
  }

  function saveDefaults() {
    try {
      saveUpscalingDefaults(defaults);
      setError("");
      Toast.success("Saved upscaling defaults in this browser.");
    } catch (e) {
      setError(`Could not save browser defaults: ${String(e)}`);
    }
  }

  async function saveLocalSetup() {
    setSaving(true);
    setLocalSaved(false);
    setLocalError("");
    try {
      const saved = await fetch(upscalerConfigEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(localConfig),
      }).then(conversionResponse<LocalUpscalerConfig>);
      if (!mounted.current) return;
      setLocalConfig({ ...emptyLocalConfig, ...saved });
      setLocalSaved(true);
      Toast.success("Saved local upscaler setup.");
      void refresh();
    } catch (e) {
      if (mounted.current)
        setLocalError(e instanceof Error ? e.message : String(e));
    } finally {
      if (mounted.current) setSaving(false);
    }
  }

  return (
    <div className="setting-section" id="media-upscaling">
      <h1>Image upscaling</h1>
      <div className="sub-heading">
        Create larger Images with waifu2x or SeedVR2. New upscaled copies keep
        the source Image; replacing an Image is a separate choice in the job
        dialog.
      </div>

      {error && <Alert variant="danger">{error}</Alert>}
      {loading && <Spinner animation="border" role="status" />}
      {!loading && (
        <>
          <Card className="p-3 mb-3">
            <h2 className="h5">Job defaults</h2>
            <p className="small text-muted">
              Saved in this browser and used by new upscaling jobs. Local model
              setup below is saved on the server.
            </p>
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
              Selected worker:{" "}
              {capabilities
                ? capabilities.backend === "remote"
                  ? "Remote worker"
                  : "This server"
                : "Unavailable"}
              . New copies inherit source metadata. Replacements use the
              separate upscaling restore cache; compression savings limits do
              not apply.
            </p>
            {capabilities?.notice && (
              <Alert variant="info" className="mt-2 mb-0">
                {capabilities.notice}
              </Alert>
            )}
            <div className="settings-actions mt-3">
              <Button onClick={saveDefaults}>Save upscaling defaults</Button>
            </div>
          </Card>

          <Card className="p-3">
            <details className="settings-details">
              <summary>
                Local model setup
                {localError || localStatusError ? " — check setup" : ""}
              </summary>
              <div className="pt-3">
                {localError && <Alert variant="danger">{localError}</Alert>}
                {localSaved && (
                  <p role="status" className="font-weight-bold">
                    Local model setup saved on this server.
                  </p>
                )}
                {localStatusError && (
                  <Alert variant="warning">
                    Could not check local worker readiness: {localStatusError}
                  </Alert>
                )}
                <fieldset disabled={saving || configLoading || !configLoaded}>
                  <p className="text-muted">
                    These are filesystem paths on this StashBooru server. They
                    are saved server-side and are never sent in an upscaling
                    request. Leave a field blank to use the process environment
                    or built-in worker default.
                  </p>
                  {capabilities?.backend === "remote" && (
                    <Alert variant="info">
                      Automatic mode is currently using the remote inference
                      worker. The paths below configure the local fallback only.
                      Configure the equivalent STASH_WAIFU2X_* / STASH_SEEDVR2_*
                      variables on that worker.
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
                        STASH_WAIFU2X_MODELS. Must contain the waifu2x .param
                        and .bin model files.
                      </Form.Text>
                    </Form.Group>
                  </Row>
                  <Alert
                    variant={
                      !refreshing && localWaifu2x?.available
                        ? "success"
                        : "secondary"
                    }
                  >
                    Local waifu2x:{" "}
                    {refreshing
                      ? "checking"
                      : localWaifu2x?.available
                        ? "ready"
                        : "not ready"}
                    . {!refreshing && localWaifu2x?.notice}
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
                        STASH_SEEDVR2_PYTHON. Blank uses the converter worker
                        Python.
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
                        STASH_SEEDVR2_MODEL. Blank uses the worker default model
                        name.
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
                          Math.min(
                            128,
                            Math.max(0, Number(event.target.value) || 0)
                          )
                        )
                      }
                    />
                    <Form.Text className="text-muted">
                      STASH_SEEDVR2_BLOCKS_TO_SWAP. 0 uses the worker default
                      (32).
                    </Form.Text>
                  </Form.Group>
                  <Alert
                    variant={
                      !refreshing && localSeedVR2?.available
                        ? "success"
                        : "secondary"
                    }
                  >
                    Local SeedVR2:{" "}
                    {refreshing
                      ? "checking"
                      : localSeedVR2?.available
                        ? "ready"
                        : "not ready"}
                    . {!refreshing && localSeedVR2?.notice}
                  </Alert>
                </fieldset>
                <p className="small text-muted">
                  Reload replaces unsaved local paths with the saved server
                  setup. Saving paths checks worker readiness without changing
                  your browser job defaults.
                </p>
                <div className="settings-actions">
                  <Button
                    variant="secondary"
                    disabled={saving || refreshing}
                    onClick={() => void refresh(true)}
                  >
                    {refreshing
                      ? "Checking worker setup…"
                      : "Reload worker setup"}
                  </Button>
                  <Button
                    disabled={saving || configLoading || !configLoaded}
                    onClick={() => void saveLocalSetup()}
                  >
                    {saving ? "Saving…" : "Save local model setup"}
                  </Button>
                </div>
              </div>
            </details>
          </Card>
        </>
      )}
    </div>
  );
};
