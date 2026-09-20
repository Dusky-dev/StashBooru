import React, { useCallback, useEffect, useState } from "react";
import { Badge, Button, Card, Form } from "react-bootstrap";
import { useHistory } from "react-router-dom";

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

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const statusResponse = await fetch("image/visual-similarity/status");
      setStatus(await readResponse<VisualSimilarityStatus>(statusResponse));
      setStatusError(undefined);
    } catch (error) {
      setStatus(undefined);
      setStatusError(error instanceof Error ? error.message : String(error));
    }

    try {
      const [camieResponse, camieConfigResponse] = await Promise.all([
        fetch("image/visual-similarity/camie/status"),
        fetch("image/visual-similarity/camie/config"),
      ]);
      setCamieStatus(await readResponse<CamieStatus>(camieResponse));
      setCamieConfig(await readResponse<CamieConfig>(camieConfigResponse));
      setCamieStatusError(undefined);
    } catch (error) {
      setCamieStatus(undefined);
      setCamieStatusError(
        error instanceof Error ? error.message : String(error)
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const saveCamieConfig = useCallback(async () => {
    setSavingCamie(true);
    try {
      const response = await fetch("image/visual-similarity/camie/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(camieConfig),
      });
      setCamieConfig(await readResponse<CamieConfig>(response));
      Toast.success("Saved tagging defaults.");
    } catch (error) {
      Toast.error(error);
    } finally {
      setSavingCamie(false);
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
        history.push("/settings?tab=tasks");
      } catch (error) {
        Toast.error(error);
        setSubmitting(false);
      }
    },
    [Toast, history]
  );

  const indexSummary = status
    ? `${status.indexedImages.toLocaleString()} / ${status.totalImages.toLocaleString()} images indexed`
    : "Index status unavailable";
  const isRemote = status?.backend === "remote";
  const isCamieRemote = camieStatus?.backend === "remote";
  const camieSubHeading =
    camieStatusError ??
    camieStatus?.workerError ??
    "Optional anime knowledge model for character, copyright, artist, general, and meta tag predictions. StashBooru never downloads or bundles Camie; supply the model and metadata files yourself.";

  return (
    <div className="setting-section" id="visual-similarity">
      <h1>Visual Similarity</h1>
      <div className="sub-heading">
        Anime/cartoon-aware EVA02 image embeddings with cosine nearest-neighbour
        search. Inference uses the shared worker configured above while the
        embedding index remains in StashBooru.
      </div>
      <Card>
        <Setting
          heading="Embedding model"
          subHeading={
            statusError ??
            status?.workerError ??
            status?.modelPath ??
            "Checking the visual embedding worker..."
          }
        >
          <div className="d-flex align-items-center flex-wrap justify-content-end">
            <Badge className="mr-2" variant={isRemote ? "info" : "secondary"}>
              {isRemote ? "Remote" : "Local"}
            </Badge>
            <Badge
              className="mr-2"
              variant={status?.workerOK ? "success" : "secondary"}
            >
              {loading
                ? "Checking"
                : status?.workerOK
                  ? "Worker ready"
                  : "Worker unavailable"}
            </Badge>
            <Badge
              className="mr-2"
              variant={status?.installed ? "success" : "secondary"}
            >
              {status?.installed ? "Model installed" : "Model not installed"}
            </Badge>
            {isRemote ? (
              <Badge variant="secondary">Model managed remotely</Badge>
            ) : (
              <Button
                variant="secondary"
                disabled={submitting || !status?.workerOK}
                onClick={() => void startJob("download")}
              >
                {status?.installed ? "Re-download model" : "Download model"}
              </Button>
            )}
          </div>
        </Setting>

        <Setting
          className="flex-column align-items-stretch"
          heading="Tagging defaults"
          subHeading="Defaults used by Image Metadata inference and Video frame analysis. Camie/frame inference and EVA02 use independent thresholds; the per-category limit is shared."
        >
          <div className="mt-3 w-100">
            <div className="d-flex flex-wrap align-items-end mb-2">
              <Form.Group className="mr-3 mb-2">
                <Form.Label>Camie / frame threshold</Form.Label>
                <Form.Control
                  type="number"
                  min="0.001"
                  max="0.999"
                  step="0.01"
                  value={camieConfig.threshold}
                  onChange={(event) =>
                    setCamieConfig((current) => ({
                      ...current,
                      threshold: Number.parseFloat(event.currentTarget.value),
                    }))
                  }
                  style={{ width: "10rem" }}
                />
              </Form.Group>
              <Form.Group className="mr-3 mb-2">
                <Form.Label>EVA02 threshold</Form.Label>
                <Form.Control
                  type="number"
                  min="0.001"
                  max="0.999"
                  step="0.01"
                  value={camieConfig.eva02Threshold}
                  onChange={(event) =>
                    setCamieConfig((current) => ({
                      ...current,
                      eva02Threshold: Number.parseFloat(
                        event.currentTarget.value
                      ),
                    }))
                  }
                  style={{ width: "9rem" }}
                />
              </Form.Group>
              <Form.Group className="mb-2">
                <Form.Label>Per-category limit</Form.Label>
                <Form.Control
                  type="number"
                  min="1"
                  max="200"
                  value={camieConfig.limit}
                  onChange={(event) =>
                    setCamieConfig((current) => ({
                      ...current,
                      limit: Number.parseInt(event.currentTarget.value, 10),
                    }))
                  }
                  style={{ width: "9rem" }}
                />
              </Form.Group>
            </div>
            <div className="d-flex justify-content-end mb-2">
              <Button
                variant="secondary"
                disabled={savingCamie}
                onClick={() => void saveCamieConfig()}
              >
                {savingCamie ? "Saving..." : "Save tagging defaults"}
              </Button>
            </div>
          </div>
        </Setting>

        <Setting
          className="flex-column align-items-stretch"
          heading="Camie Tagger v2 (optional)"
          subHeading={camieSubHeading}
        >
          <div className="mt-3 w-100">
            <div className="d-flex align-items-center flex-wrap mb-2">
              <Badge
                className="mr-2 mb-1"
                variant={isCamieRemote ? "info" : "secondary"}
              >
                {isCamieRemote ? "Remote" : "Local"}
              </Badge>
              <Badge
                className="mr-2 mb-1"
                variant={camieStatus?.workerOK ? "success" : "secondary"}
              >
                {loading
                  ? "Checking"
                  : camieStatus?.workerOK
                    ? "Worker ready"
                    : "Worker unavailable"}
              </Badge>
              <Badge
                className="mr-2 mb-1"
                variant={camieStatus?.installed ? "success" : "secondary"}
              >
                {camieStatus?.installed ? "Model ready" : "Model files missing"}
              </Badge>
              {camieStatus?.tagCount ? (
                <Badge className="mb-1" variant="secondary">
                  {camieStatus.tagCount.toLocaleString()} tags
                </Badge>
              ) : null}
            </div>
            {camieStatus?.modelPath ? (
              <div className="mb-2 text-break">
                <strong>Model:</strong> <code>{camieStatus.modelPath}</code>
              </div>
            ) : null}
            {camieStatus?.metadataPath ? (
              <div className="text-break mb-3">
                <strong>Metadata:</strong>{" "}
                <code>{camieStatus.metadataPath}</code>
              </div>
            ) : null}
            <Form.Check
              className="mb-2"
              type="checkbox"
              id="camie-filename-enabled"
              checked={camieConfig.filenameEnabled}
              onChange={(event) =>
                setCamieConfig((current) => ({
                  ...current,
                  filenameEnabled: event.currentTarget.checked,
                }))
              }
              label="Use filename metadata together with Camie predictions"
            />
            <Form.Group className="mb-2">
              <Form.Label>Filename layout</Form.Label>
              <Form.Control
                type="text"
                disabled={!camieConfig.filenameEnabled}
                value={camieConfig.filenameLayout}
                onChange={(event) =>
                  setCamieConfig((current) => ({
                    ...current,
                    filenameLayout: event.currentTarget.value,
                  }))
                }
              />
              <Form.Text className="text-muted">
                Supported tokens: <code>%artist%</code>,{" "}
                <code>%copyright%</code>, <code>%character%</code>,{" "}
                <code>%md5%</code>, and <code>%ext%</code>. The default matches
                Imgbrd-style names such as{" "}
                <code>[%artist%](%copyright%).%character%_%md5%.%ext%</code>.
              </Form.Text>
            </Form.Group>
            <div className="d-flex justify-content-end mb-2">
              <Button
                variant="secondary"
                disabled={savingCamie}
                onClick={() => void saveCamieConfig()}
              >
                {savingCamie ? "Saving..." : "Save Camie filename settings"}
              </Button>
            </div>
            {!camieStatus?.installed && camieStatus?.workerOK ? (
              <div className="mt-2 text-muted">
                Place <code>camie-tagger-v2.onnx</code> and{" "}
                <code>camie-tagger-v2-metadata.json</code> at the paths above,
                then refresh status. Camie is never downloaded automatically.
              </div>
            ) : null}
          </div>
        </Setting>

        <Setting
          heading="Image similarity index"
          subHeading={`${indexSummary}. Embeddings are regenerated only when the source image changes.`}
        >
          <div className="d-flex align-items-center flex-wrap justify-content-end">
            {status ? (
              <Badge
                className="mr-2"
                variant={
                  status.indexedImages === status.totalImages &&
                  status.totalImages > 0
                    ? "success"
                    : "secondary"
                }
              >
                {status.dimensions}D EVA02
              </Badge>
            ) : null}
            <Button
              variant="primary"
              disabled={submitting || !status?.installed}
              onClick={() => void startJob("index")}
            >
              Generate visual embeddings
            </Button>
          </div>
        </Setting>

        <Setting
          heading="Image Find similar"
          subHeading="After indexing, the Find similar button on image cards uses the EVA02 embedding index and returns nearest matches first. pHash remains separate for perceptual duplicate-style matching."
        >
          <Button
            variant="secondary"
            disabled={loading}
            onClick={() => void refresh()}
          >
            Refresh status
          </Button>
        </Setting>
      </Card>
    </div>
  );
};
