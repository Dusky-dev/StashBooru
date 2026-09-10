import React, { useCallback, useEffect, useState } from "react";
import { Badge, Button, Card } from "react-bootstrap";
import { useHistory } from "react-router-dom";

import { useToast } from "src/hooks/Toast";
import { Setting } from "./Inputs";

interface VisualSimilarityStatus {
  installed: boolean;
  loaded: boolean;
  workerOK: boolean;
  workerError?: string;
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

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const response = await fetch("image/visual-similarity/status");
      setStatus(await readResponse<VisualSimilarityStatus>(response));
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

  return (
    <div className="setting-section" id="visual-similarity">
      <h1>Visual Similarity</h1>
      <div className="sub-heading">
        Anime/cartoon-aware EVA02 image embeddings with cosine nearest-neighbour
        search. The model is downloaded only when you explicitly request it.
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
            <Badge className="mr-2" variant={status?.workerOK ? "success" : "secondary"}>
              {loading ? "Checking" : status?.workerOK ? "Worker ready" : "Worker unavailable"}
            </Badge>
            <Badge className="mr-2" variant={status?.installed ? "success" : "secondary"}>
              {status?.installed ? "Model installed" : "Model not installed"}
            </Badge>
            <Button
              variant="secondary"
              disabled={submitting || !status?.workerOK}
              onClick={() => void startJob("download")}
            >
              {status?.installed ? "Re-download model" : "Download model"}
            </Button>
          </div>
        </Setting>

        <Setting
          heading="Image similarity index"
          subHeading={`${indexSummary}. Embeddings are regenerated only when the source image changes.`}
        >
          <div className="d-flex align-items-center flex-wrap justify-content-end">
            {status ? (
              <Badge className="mr-2" variant={status.indexedImages === status.totalImages && status.totalImages > 0 ? "success" : "secondary"}>
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
          <Button variant="secondary" disabled={loading} onClick={() => void refresh()}>
            Refresh status
          </Button>
        </Setting>
      </Card>
    </div>
  );
};
