import React, { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Badge, Button, Card, Col, Form, Row } from "react-bootstrap";
import { Link, useHistory } from "react-router-dom";
import { useIntl } from "react-intl";

import { useToast } from "src/hooks/Toast";
import { Setting } from "./Inputs";

interface VisualSimilarityStatus {
  installed: boolean;
  loaded: boolean;
  workerOK: boolean;
  workerError?: string;
  backend: "local" | "remote";
  remoteURL?: string;
  modelPath?: string;
  model: string;
  revision: string;
  dimensions: number;
  indexedImages: number;
  totalImages: number;
}

interface CamieStatus {
  installed: boolean;
  loaded: boolean;
  workerOK: boolean;
  workerError?: string;
  backend: "local" | "remote";
  remoteURL?: string;
  modelExists: boolean;
  metadataExists: boolean;
  modelPath?: string;
  metadataPath?: string;
  model: string;
  tagCount: number;
}

interface CamieConfig {
  threshold: number;
  eva02Threshold: number;
  limit: number;
  filenameEnabled: boolean;
  filenameLayout: string;
}

interface VisualSimilarityJobResponse {
  jobID: number;
}

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return response.json() as Promise<T>;
}

export const VisualSimilaritySettings: React.FC = () => {
  const Toast = useToast();
  const intl = useIntl();
  const history = useHistory();
  const [status, setStatus] = useState<VisualSimilarityStatus>();
  const [statusError, setStatusError] = useState<string>();
  const [camieStatus, setCamieStatus] = useState<CamieStatus>();
  const [camieStatusError, setCamieStatusError] = useState<string>();
  const [camieConfig, setCamieConfig] = useState<CamieConfig>({
    threshold: 0.492,
    eva02Threshold: 0.35,
    limit: 50,
    filenameEnabled: true,
    filenameLayout: "[%artist%](%copyright%).%character%_%md5%.%ext%",
  });
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [savingCamie, setSavingCamie] = useState(false);
  const [configLoaded, setConfigLoaded] = useState(false);
  const [configError, setConfigError] = useState("");
  const mounted = useRef(true);
  const refreshController = useRef<AbortController>();

  const refresh = useCallback(async (includeConfig = false) => {
    refreshController.current?.abort();
    const controller = new AbortController();
    refreshController.current = controller;
    setLoading(true);
    const get = <T,>(path: string) =>
      fetch(`image/visual-similarity/${path}`, {
        signal: controller.signal,
      }).then(readResponse<T>);
    const [embedding, camie, config] = await Promise.allSettled([
      get<VisualSimilarityStatus>("status"),
      get<CamieStatus>("camie/status"),
      includeConfig
        ? get<CamieConfig>("camie/config")
        : Promise.resolve(undefined),
    ]);
    if (controller.signal.aborted || !mounted.current) return;
    setStatus(embedding.status === "fulfilled" ? embedding.value : undefined);
    setStatusError(
      embedding.status === "rejected" ? String(embedding.reason) : undefined
    );
    setCamieStatus(camie.status === "fulfilled" ? camie.value : undefined);
    setCamieStatusError(
      camie.status === "rejected" ? String(camie.reason) : undefined
    );
    if (includeConfig) {
      if (config.status === "fulfilled" && config.value) {
        setCamieConfig(config.value);
        setConfigLoaded(true);
        setConfigError("");
      } else if (config.status === "rejected") {
        setConfigError(String(config.reason));
      }
    }
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

  const saveCamieConfig = useCallback(async () => {
    setSavingCamie(true);
    try {
      const response = await fetch("image/visual-similarity/camie/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(camieConfig),
      });
      const saved = await readResponse<CamieConfig>(response);
      if (!mounted.current) return;
      setCamieConfig(saved);
      Toast.success("Saved tagging defaults.");
    } catch (error) {
      if (mounted.current) Toast.error(error);
    } finally {
      if (mounted.current) setSavingCamie(false);
    }
  }, [Toast, camieConfig]);

  const startJob = useCallback(
    async (endpoint: "download" | "index") => {
      setSubmitting(true);
      try {
        const response = await fetch(`image/visual-similarity/${endpoint}`, {
          method: "POST",
        });
        await readResponse<VisualSimilarityJobResponse>(response);
        if (mounted.current) history.push("/settings?tab=tasks");
      } catch (error) {
        if (mounted.current) Toast.error(error);
      } finally {
        if (mounted.current) setSubmitting(false);
      }
    },
    [Toast, history]
  );

  const indexSummary = status
    ? `${status.indexedImages.toLocaleString()} / ${status.totalImages.toLocaleString()} images indexed`
    : "Index status unavailable";
  const isRemote = status?.backend === "remote";
  const validConfig =
    configLoaded &&
    [camieConfig.threshold, camieConfig.eva02Threshold].every(
      (v) => Number.isFinite(v) && v >= 0.001 && v <= 0.999
    ) &&
    Number.isInteger(camieConfig.limit) &&
    camieConfig.limit >= 1 &&
    camieConfig.limit <= 200;

  function workerStatus(value?: VisualSimilarityStatus | CamieStatus) {
    if (loading) return <Badge variant="secondary">Checking…</Badge>;
    if (!value) return <Badge variant="secondary">Status unavailable</Badge>;
    return (
      <div className="processing-status">
        <Badge variant={value.backend === "remote" ? "info" : "secondary"}>
          {value.backend === "remote" ? "Remote worker" : "This server"}
        </Badge>
        <Badge variant={value.workerOK ? "success" : "secondary"}>
          {value.workerOK ? "Worker ready" : "Worker unavailable"}
        </Badge>
        <Badge variant={value.installed ? "success" : "secondary"}>
          {value.installed ? "Model ready" : "Model files missing"}
        </Badge>
      </div>
    );
  }

  return (
    <div className="setting-section" id="visual-similarity">
      <h1>Similarity and tagging</h1>
      <div className="sub-heading">
        Find similar defaults to <strong>Same image / variants (pHash)</strong>.
        Switch to <strong>Related content (EVA02)</strong> for similar subjects
        or composition. pHash needs generated perceptual hashes; EVA02 needs its
        own model and image index. Neither mode proves two files are identical.
      </div>
      <Card>
        <Setting
          className="flex-column align-items-stretch"
          heading="Related-content index"
          subHeading={`${indexSummary}. Generate embeddings after installing the model. Unchanged images are skipped.`}
        >
          <div className="mt-3 w-100">
            {workerStatus(status)}
            {(statusError || status?.workerError) && (
              <Alert variant="warning" className="mt-2">
                {statusError || status?.workerError}
              </Alert>
            )}
            <div className="settings-actions mt-3">
              {!isRemote && !status?.installed && (
                <Button
                  variant="secondary"
                  disabled={loading || submitting || !status?.workerOK}
                  onClick={() => void startJob("download")}
                >
                  Download EVA02 model
                </Button>
              )}
              <Button
                disabled={
                  loading ||
                  submitting ||
                  !status?.installed ||
                  !status.workerOK
                }
                onClick={() => void startJob("index")}
              >
                Generate visual embeddings
              </Button>
              <Button
                variant="secondary"
                disabled={loading || submitting}
                onClick={() => void refresh(!configLoaded)}
              >
                Refresh status
              </Button>
            </div>
            <p className="small text-muted mt-2 mb-0">
              For pHash matching, generate Image perceptual hashes in{" "}
              <Link to="/settings?tab=tasks">Tasks</Link>. The EVA02 index stays
              on this server when inference runs remotely.
            </p>
            <details className="settings-details mt-3">
              <summary>EVA02 model details</summary>
              <div className="pt-2">
                {status && (
                  <p className="text-break">
                    {status.model} · {status.revision} · {status.dimensions}{" "}
                    dimensions
                  </p>
                )}
                {status?.modelPath && (
                  <p className="text-break">
                    <code>{status.modelPath}</code>
                  </p>
                )}
                {isRemote ? (
                  <p className="mb-0">
                    Install or update the model on the remote worker.
                  </p>
                ) : (
                  <>
                    <p>Model downloads start only when requested.</p>
                    {status?.installed && (
                      <Button
                        variant="secondary"
                        disabled={loading || submitting || !status.workerOK}
                        onClick={() => void startJob("download")}
                      >
                        Re-download EVA02 model
                      </Button>
                    )}
                  </>
                )}
              </div>
            </details>
          </div>
        </Setting>
        <Setting
          className="flex-column align-items-stretch"
          heading="Tagging defaults"
          subHeading="Server-wide defaults for Image Tagging and Video frame analysis. Higher thresholds keep fewer predictions. Filename metadata works without Camie."
        >
          <Form
            className="mt-3 w-100"
            onSubmit={(event) => {
              event.preventDefault();
              if (validConfig && !savingCamie) void saveCamieConfig();
            }}
          >
            {configError && <Alert variant="danger">{configError}</Alert>}
            <fieldset disabled={loading || savingCamie || !configLoaded}>
              <Row>
                <Form.Group
                  as={Col}
                  xs={12}
                  md={4}
                  controlId="tagging-camie-threshold"
                >
                  <Form.Label>Camie / frame threshold</Form.Label>
                  <Form.Control
                    className="text-input"
                    type="number"
                    min="0.001"
                    max="0.999"
                    step="any"
                    value={camieConfig.threshold}
                    required
                    onChange={(event) =>
                      setCamieConfig({
                        ...camieConfig,
                        threshold: Number(event.currentTarget.value),
                      })
                    }
                  />
                </Form.Group>
                <Form.Group
                  as={Col}
                  xs={12}
                  md={4}
                  controlId="tagging-eva02-threshold"
                >
                  <Form.Label>EVA02 tagging threshold</Form.Label>
                  <Form.Control
                    className="text-input"
                    type="number"
                    min="0.001"
                    max="0.999"
                    step="any"
                    value={camieConfig.eva02Threshold}
                    required
                    onChange={(event) =>
                      setCamieConfig({
                        ...camieConfig,
                        eva02Threshold: Number(event.currentTarget.value),
                      })
                    }
                  />
                </Form.Group>
                <Form.Group
                  as={Col}
                  xs={12}
                  md={4}
                  controlId="tagging-category-limit"
                >
                  <Form.Label>Per-category limit</Form.Label>
                  <Form.Control
                    className="text-input"
                    type="number"
                    min="1"
                    max="200"
                    step="1"
                    value={camieConfig.limit}
                    required
                    onChange={(event) =>
                      setCamieConfig({
                        ...camieConfig,
                        limit: Number(event.currentTarget.value),
                      })
                    }
                  />
                </Form.Group>
              </Row>
              <Form.Check
                className="mb-2"
                type="checkbox"
                id="camie-filename-enabled"
                checked={camieConfig.filenameEnabled}
                onChange={(event) =>
                  setCamieConfig({
                    ...camieConfig,
                    filenameEnabled: event.currentTarget.checked,
                  })
                }
                label="Read metadata from filenames"
              />
              <Form.Group controlId="camie-filename-layout">
                <Form.Label>Filename layout</Form.Label>
                <Form.Control
                  className="text-input"
                  type="text"
                  disabled={!camieConfig.filenameEnabled}
                  value={camieConfig.filenameLayout}
                  onChange={(event) =>
                    setCamieConfig({
                      ...camieConfig,
                      filenameLayout: event.currentTarget.value,
                    })
                  }
                />
                <Form.Text className="text-muted">
                  Supported tokens: <code>%artist%</code>,{" "}
                  <code>%copyright%</code>, <code>%character%</code>,{" "}
                  <code>%md5%</code>, <code>%ext%</code>. Matched filename
                  identities take priority over inferred identities.
                </Form.Text>
              </Form.Group>
              <p className="small">
                <Link to="/settings?tab=library#association-inheritance-settings">
                  {intl.formatMessage({
                    id: "config.association_inheritance.review.shared_defaults",
                  })}
                </Link>
              </p>
              <div className="settings-actions">
                <Button type="submit" disabled={!validConfig || savingCamie}>
                  {savingCamie ? "Saving…" : "Save tagging defaults"}
                </Button>
              </div>
            </fieldset>
          </Form>
        </Setting>
        <Setting
          className="flex-column align-items-stretch"
          heading="Camie Tagger v2 (optional)"
          subHeading="Adds Character, Copyright, Artist and general Tag predictions. Supply its model and metadata on the selected worker; Camie is never downloaded automatically."
        >
          <div className="mt-3 w-100">
            {workerStatus(camieStatus)}
            {(camieStatusError || camieStatus?.workerError) && (
              <Alert variant="warning" className="mt-2">
                {camieStatusError || camieStatus?.workerError}
              </Alert>
            )}
            <details className="settings-details mt-3">
              <summary>Camie model setup</summary>
              <div className="pt-2 text-break">
                {camieStatus?.modelPath && (
                  <p>
                    <strong>Model:</strong> <code>{camieStatus.modelPath}</code>
                  </p>
                )}
                {camieStatus?.metadataPath && (
                  <p>
                    <strong>Metadata:</strong>{" "}
                    <code>{camieStatus.metadataPath}</code>
                  </p>
                )}
                <p className="mb-0">
                  Install <code>camie-tagger-v2.onnx</code> and{" "}
                  <code>camie-tagger-v2-metadata.json</code>
                  {camieStatus?.backend === "remote"
                    ? " on the remote worker"
                    : " on this server"}
                  , then refresh status.
                  {camieStatus?.tagCount
                    ? ` ${camieStatus.tagCount.toLocaleString()} model tags detected.`
                    : ""}
                </p>
              </div>
            </details>
          </div>
        </Setting>
      </Card>
    </div>
  );
};
