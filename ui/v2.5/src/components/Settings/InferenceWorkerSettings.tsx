import React, { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Card, Form } from "react-bootstrap";

import { useToast } from "src/hooks/Toast";
import { Setting } from "./Inputs";
import { ExternalLink } from "src/components/Shared/ExternalLink";

interface InferenceWorkerConfig {
  url: string;
  tokenConfigured: boolean;
  enabled: boolean;
  fallbackLocal: boolean;
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
  const [remoteEnabled, setRemoteEnabled] = useState(false);
  const [fallbackLocal, setFallbackLocal] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();
  const mounted = useRef(true);
  const refreshController = useRef<AbortController>();

  const refresh = useCallback(async () => {
    refreshController.current?.abort();
    const controller = new AbortController();
    refreshController.current = controller;
    setLoading(true);
    try {
      const response = await fetch("image/visual-similarity/remote-config", {
        signal: controller.signal,
      });
      const config = await readResponse<InferenceWorkerConfig>(response);
      if (controller.signal.aborted || !mounted.current) return;
      setRemoteURL(config.url);
      setTokenConfigured(config.tokenConfigured);
      setRemoteEnabled(config.enabled ?? !!config.url);
      setFallbackLocal(config.fallbackLocal ?? false);
      setError(undefined);
    } catch (e) {
      if (!controller.signal.aborted && mounted.current)
        setError(e instanceof Error ? e.message : String(e));
    } finally {
      if (!controller.signal.aborted && mounted.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    void refresh();
    return () => {
      mounted.current = false;
      refreshController.current?.abort();
    };
  }, [refresh]);

  const save = useCallback(
    async (clearToken = false) => {
      setSaving(true);
      try {
        const payload: {
          url: string;
          token?: string;
          clearToken?: boolean;
          enabled: boolean;
          fallbackLocal: boolean;
        } = {
          url: remoteURL.trim(),
          enabled: remoteEnabled,
          fallbackLocal,
        };
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
        if (!mounted.current) return;
        setRemoteURL(config.url);
        setTokenConfigured(config.tokenConfigured);
        setRemoteEnabled(config.enabled ?? !!config.url);
        setFallbackLocal(config.fallbackLocal ?? false);
        setRemoteToken("");
        setError(undefined);
        Toast.success("Saved inference worker settings.");
      } catch (e) {
        if (mounted.current) {
          setError(e instanceof Error ? e.message : String(e));
          Toast.error(e);
        }
      } finally {
        if (mounted.current) setSaving(false);
      }
    },
    [Toast, remoteToken, remoteURL, remoteEnabled, fallbackLocal]
  );

  return (
    <div className="setting-section" id="inference-worker">
      <h1>Processing worker</h1>
      <div className="sub-heading">
        Connect an optional remote processing worker.
      </div>
      <div className="mb-3">
        <ExternalLink href="https://github.com/Dusky-dev/StashBooru/releases/download/latest_develop/stash-remote-worker.zip">
          Download remote worker ZIP (development)
        </ExternalLink>
        {" · "}
        <ExternalLink href="https://github.com/Dusky-dev/StashBooru/releases">
          Version-matched stable releases
        </ExternalLink>
        {" · "}
        <ExternalLink href="https://github.com/Dusky-dev/StashBooru/blob/develop/scripts/visual_embedding_server.md">
          Setup and capability guide
        </ExternalLink>
      </div>
      <Card>
        <Setting
          className="flex-column align-items-stretch"
          heading="Remote worker"
          subHeading="Worker address and access token."
        >
          <div className="mt-3 w-100">
            {error && <Alert variant="danger">{error}</Alert>}
            <Form.Check
              id="inference-worker-enabled"
              type="switch"
              label="Enable remote worker"
              checked={remoteEnabled}
              disabled={loading || saving}
              onChange={(event) => setRemoteEnabled(event.currentTarget.checked)}
            />
            <Form.Check
              id="inference-worker-fallback-local"
              type="switch"
              label="Fall back to local processing when the remote worker is unavailable"
              checked={fallbackLocal}
              disabled={loading || saving}
              onChange={(event) => setFallbackLocal(event.currentTarget.checked)}
            />
            <Form.Text className="d-block text-muted mb-3">
              Disabling the remote worker keeps its URL and token. Local
              fallback requires the matching models and codecs to be installed
              on this StashBooru server. If fallback is off, remote failures
              remain visible instead of silently switching workers.
            </Form.Text>
            <Form.Group controlId="inference-worker-url">
              <Form.Label>Worker URL</Form.Label>
              <Form.Control
                className="text-input"
                type="url"
                value={remoteURL}
                disabled={loading || saving}
                placeholder="http://gpu-pc:8000"
                onChange={(event) => setRemoteURL(event.currentTarget.value)}
              />
            </Form.Group>
            <Form.Group controlId="inference-worker-token">
              <Form.Label>Bearer token</Form.Label>
              <Form.Control
                className="text-input"
                type="password"
                autoComplete="new-password"
                value={remoteToken}
                disabled={loading || saving}
                placeholder={
                  tokenConfigured
                    ? "Bearer token saved — leave blank to keep it"
                    : "Optional bearer token"
                }
                onChange={(event) => setRemoteToken(event.currentTarget.value)}
              />
              <Form.Text className="text-muted">
                Leave blank to keep a saved token; use Clear token to remove it.
              </Form.Text>
            </Form.Group>
            <div className="settings-actions">
              {tokenConfigured ? (
                <Button
                  variant="secondary"
                  disabled={loading || saving}
                  onClick={() => void save(true)}
                >
                  Clear token
                </Button>
              ) : null}
              <Button
                variant="secondary"
                disabled={loading || saving}
                onClick={() => void refresh()}
              >
                Reload saved worker
              </Button>
              <Button
                variant="primary"
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
