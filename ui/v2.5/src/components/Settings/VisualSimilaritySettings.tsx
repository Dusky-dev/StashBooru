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

interface VisualSimilarityJobResponse {
  jobID: number;
}

interface VisualSimilarityRemoteConfig {
  url: string;
  tokenConfigured: boolean;
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
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [savingWorker, setSavingWorker] = useState(false);
  const [remoteURL, setRemoteURL] = useState("");
  const [remoteToken, setRemoteToken] = useState("");
  const [tokenConfigured, setTokenConfigured] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [statusResponse, configResponse] = await Promise.all([
        fetch("image/visual-similarity/status"),
        fetch("image/visual-similarity/remote-config"),
      ]);
      const nextStatus =
        await readResponse<VisualSimilarityStatus>(statusResponse);
      const remoteConfig =
        await readResponse<VisualSimilarityRemoteConfig>(configResponse);
      setStatus(nextStatus);
      setRemoteURL(remoteConfig.url);
      setTokenConfigured(remoteConfig.tokenConfigured);
      setStatusError(undefined);
    } catch (error) {
      setStatusError(error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const saveRemoteWorker = useCallback(
    async (clearToken = false) => {
      setSavingWorker(true);
      try {
        const payload: {
          url: string;
          token?: string;
          clearToken?: boolean;
        } = { url: remoteURL.trim() };
        if (clearToken) {
          payload.clearToken = true;
        } else if (remoteToken.trim()) {
          payload.token = remoteToken.trim();
        }

        const response = await fetch("image/visual-similarity/remote-config", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        const saved =
          await readResponse<VisualSimilarityRemoteConfig>(response);
        setRemoteURL(saved.url);
        setTokenConfigured(saved.tokenConfigured);
        setRemoteToken("");
        await refresh();
      } catch (error) {
        Toast.error(error);
      } finally {
        setSavingWorker(false);
      }
    },
    [Toast, refresh, remoteToken, remoteURL]
  );

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

  return (
    <div className="setting-section" id="visual-similarity">
      <h1>Visual Similarity</h1>
      <div className="sub-heading">
        Anime/cartoon-aware EVA02 image embeddings with cosine nearest-neighbour
        search. Stash keeps the index locally; inference can run here or on a
        remote GPU worker.
      </div>
      <Card>
        <Setting
          className="flex-column align-items-stretch"
          heading="Inference worker"
          subHeading="Leave the URL empty to use the local worker. Set a remote URL to stream images to another machine for inference while keeping all metadata and embeddings in StashBooru."
        >
          <div className="mt-3 w-100">
            <Form.Control
              className="mb-2"
              type="url"
              value={remoteURL}
              placeholder="http://gpu-pc:8000"
              onChange={(event) => setRemoteURL(event.currentTarget.value)}
            />
            <Form.Control
              className="mb-2"
              type="password"
              value={remoteToken}
              placeholder={
                tokenConfigured
                  ? "Bearer token saved — leave blank to keep it"
                  : "Optional bearer token"
              }
              onChange={(event) => setRemoteToken(event.currentTarget.value)}
            />
            <div className="d-flex flex-wrap justify-content-end">
              {tokenConfigured ? (
                <Button
                  variant="outline-secondary"
                  disabled={savingWorker}
                  onClick={() => void saveRemoteWorker(true)}
                >
                  Clear token
                </Button>
              ) : null}
              <Button
                variant="secondary"
                disabled={savingWorker}
                onClick={() => void saveRemoteWorker(false)}
              >
                {savingWorker ? "Saving..." : "Save worker"}
              </Button>
            </div>
          </div>
        </Setting>

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
