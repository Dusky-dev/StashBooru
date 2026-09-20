import React, { useCallback, useEffect, useState } from "react";
import { Button, Card, Form } from "react-bootstrap";

import { useToast } from "src/hooks/Toast";
import { Setting } from "./Inputs";

interface InferenceWorkerConfig {
  url: string;
  tokenConfigured: boolean;
}

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return response.json() as Promise<T>;
}

export const InferenceWorkerSettings: React.FC = () => {
  const Toast = useToast();
  const [remoteURL, setRemoteURL] = useState("");
  const [remoteToken, setRemoteToken] = useState("");
  const [tokenConfigured, setTokenConfigured] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const response = await fetch("image/visual-similarity/remote-config");
      const config = await readResponse<InferenceWorkerConfig>(response);
      setRemoteURL(config.url);
      setTokenConfigured(config.tokenConfigured);
      setError(undefined);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const save = useCallback(
    async (clearToken = false) => {
      setSaving(true);
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
        const config = await readResponse<InferenceWorkerConfig>(response);
        setRemoteURL(config.url);
        setTokenConfigured(config.tokenConfigured);
        setRemoteToken("");
        setError(undefined);
        Toast.success("Saved inference worker settings.");
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
        Toast.error(e);
      } finally {
        setSaving(false);
      }
    },
    [Toast, remoteToken, remoteURL]
  );

  return (
    <div className="setting-section" id="inference-worker">
      <h1>Inference worker</h1>
      <div className="sub-heading">
        Shared GPU/CPU worker used by Visual Similarity, EVA02/Image Tagging,
        optional Camie inference, media conversion and image upscaling. Leave
        the URL empty to run supported work on this StashBooru server.
      </div>
      <Card>
        <Setting
          className="flex-column align-items-stretch"
          heading="Remote worker"
          subHeading={
            error ??
            "Set one remote worker here instead of configuring it separately for each inference feature. Existing worker deployments remain compatible."
          }
        >
          <div className="mt-3 w-100">
            <Form.Control
              className="mb-2"
              type="url"
              value={remoteURL}
              disabled={loading || saving}
              placeholder="http://gpu-pc:8000"
              onChange={(event) => setRemoteURL(event.currentTarget.value)}
            />
            <Form.Control
              className="mb-2"
              type="password"
              value={remoteToken}
              disabled={loading || saving}
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
                  className="mr-2 mb-2"
                  variant="outline-secondary"
                  disabled={loading || saving}
                  onClick={() => void save(true)}
                >
                  Clear token
                </Button>
              ) : null}
              <Button
                className="mr-2 mb-2"
                variant="outline-secondary"
                disabled={loading || saving}
                onClick={() => void refresh()}
              >
                Refresh
              </Button>
              <Button
                className="mb-2"
                variant="secondary"
                disabled={loading || saving}
                onClick={() => void save(false)}
              >
                {saving ? "Saving..." : "Save worker"}
              </Button>
            </div>
          </div>
        </Setting>
      </Card>
    </div>
  );
};
