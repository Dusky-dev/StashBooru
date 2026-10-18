import { useEffect, useRef, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import { readBooruResponse } from "./browser";
import "./styles.scss";
import { BrowserToolsSettings } from "./BrowserToolsSettings";

interface SourceSetting {
  id: string;
  name: string;
  engine: string;
  url: string;
  mediaHosts: string[];
  login: string;
  userID: string;
  disabled: boolean;
  preset: boolean;
  apiKeyConfigured: boolean;
}
interface SettingsResponse {
  revision: string;
  sources: SourceSetting[];
}
const newSource = (): SourceSetting => ({
  id: "",
  name: "",
  engine: "danbooru",
  url: "",
  mediaHosts: [],
  login: "",
  userID: "",
  disabled: false,
  preset: false,
  apiKeyConfigured: false,
});

export function SourceSettings({ onSaved }: { onSaved?: () => void }) {
  const [settings, setSettings] = useState<SettingsResponse>();
  const [draft, setDraft] = useState<SourceSetting>();
  const [hosts, setHosts] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const [clearKey, setClearKey] = useState(false);
  const [creating, setCreating] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loadAttempt, setLoadAttempt] = useState(0);
  const request = useRef<AbortController>();
  // biome-ignore lint/correctness/useExhaustiveDependencies: Explicit reload retries the same saved configuration read.
  useEffect(() => {
    const controller = new AbortController();
    request.current = controller;
    setBusy(true);
    setError("");
    void fetch("booru-browser/settings", { signal: controller.signal })
      .then(readBooruResponse<SettingsResponse>)
      .then((value) => {
        if (!controller.signal.aborted) setSettings(value);
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted)
          setError(cause instanceof Error ? cause.message : String(cause));
      })
      .finally(() => {
        if (!controller.signal.aborted) setBusy(false);
      });
    return () => request.current?.abort();
  }, [loadAttempt]);
  function edit(source: SourceSetting, isNew = false) {
    setDraft({ ...source });
    setHosts(source.mediaHosts.join("\n"));
    setAPIKey("");
    setClearKey(false);
    setCreating(isNew);
    setError("");
    setNotice("");
  }
  async function save(action: "save" | "remove" | "reset") {
    if (!settings || !draft) return;
    if (
      creating &&
      settings.sources.some((source) => source.id === draft.id.trim())
    ) {
      setError(
        "Source ID is already in use. Edit that source or choose a new ID."
      );
      return;
    }
    const controller = new AbortController();
    request.current = controller;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const source = {
        id: draft.id.trim(),
        name: draft.name.trim(),
        engine: draft.engine,
        url: draft.url.trim().replace(/\/+$/, ""),
        mediaHosts: hosts.split(/[\s,]+/).filter(Boolean),
        login: clearKey ? "" : draft.login.trim(),
        userID: clearKey ? "" : draft.userID.trim(),
        disabled: draft.disabled,
        apiKey: apiKey.trim(),
      };
      const response = await fetch("booru-browser/settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        signal: controller.signal,
        body: JSON.stringify({
          revision: settings.revision,
          action,
          source,
          clearAPIKey: clearKey,
        }),
      });
      const value = await readBooruResponse<SettingsResponse>(response);
      if (controller.signal.aborted) return;
      setSettings(value);
      setDraft(undefined);
      setAPIKey("");
      setClearKey(false);
      setNotice(
        action === "save"
          ? "Source saved. Changes apply to new searches."
          : action === "reset"
            ? "Preset restored. Saved credentials removed."
            : "Custom source removed."
      );
      onSaved?.();
    } catch (cause: unknown) {
      if (!controller.signal.aborted)
        setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  function close() {
    setDraft(undefined);
    setAPIKey("");
    setError("");
  }
  return (
    <div className="booru-source-settings">
      <p className="text-muted">
        Add API credentials or a custom site that uses a supported engine.
        Settings are shared by everyone using this server.
      </p>
      {notice && (
        <Alert variant="success" role="status">
          {notice}
        </Alert>
      )}
      {!draft && error && <Alert variant="danger">{error}</Alert>}
      {!draft && settings && <BrowserToolsSettings key={settings.revision} />}
      <div className="booru-settings-toolbar">
        <Button
          size="sm"
          disabled={busy || !settings}
          onClick={() => edit(newSource(), true)}
        >
          Add custom booru
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={busy}
          onClick={() => {
            close();
            setNotice("");
            setLoadAttempt((n) => n + 1);
          }}
        >
          Reload saved sources
        </Button>
        {busy && <span role="status">Loading…</span>}
      </div>
      <ul className="booru-settings-list">
        {settings?.sources.map((source) => (
          <li key={source.id}>
            <div>
              <strong>{source.name || source.id}</strong>
              <small className="d-block text-muted">
                {source.engine} · {source.disabled ? "Disabled" : "Enabled"} ·{" "}
                {source.apiKeyConfigured ? "API key saved" : "No API key"}
                {!source.preset && " · Custom"}
              </small>
            </div>
            <Button
              size="sm"
              variant="secondary"
              disabled={busy}
              onClick={() => edit(source)}
              aria-label={`Edit ${source.name || source.id}`}
            >
              Edit
            </Button>
          </li>
        ))}
      </ul>
      {draft && (
        <Modal
          show
          onHide={() => {
            if (!busy) close();
          }}
          size="lg"
          className="booru-settings-editor"
          backdrop="static"
        >
          <Modal.Header closeButton={!busy}>
            <Modal.Title>
              {creating ? "Add custom booru" : `Edit ${draft.name || draft.id}`}
            </Modal.Title>
          </Modal.Header>
          <Form
            onSubmit={(event) => {
              event.preventDefault();
              void save("save");
            }}
          >
            <Modal.Body>
              {error && (
                <Alert variant="danger">
                  {error}
                  <Button
                    size="sm"
                    variant="link"
                    disabled={busy}
                    onClick={() => {
                      close();
                      setLoadAttempt((n) => n + 1);
                    }}
                  >
                    Reload saved sources
                  </Button>
                </Alert>
              )}
              <fieldset disabled={busy}>
                <div className="booru-settings-fields">
                  <Form.Group controlId="booru-setting-name">
                    <Form.Label>Name</Form.Label>
                    <Form.Control
                      required
                      maxLength={120}
                      value={draft.name}
                      onChange={(e) =>
                        setDraft({ ...draft, name: e.currentTarget.value })
                      }
                    />
                  </Form.Group>
                  <Form.Group controlId="booru-setting-id">
                    <Form.Label>Source ID</Form.Label>
                    <Form.Control
                      required
                      pattern="[a-z0-9][a-z0-9_-]{0,63}"
                      disabled={!creating}
                      value={draft.id}
                      onChange={(e) => {
                        setDraft({ ...draft, id: e.currentTarget.value });
                        setError("");
                      }}
                    />
                    <Form.Text muted>
                      Unique lowercase name used in search links.
                    </Form.Text>
                  </Form.Group>
                  <Form.Group controlId="booru-setting-engine">
                    <Form.Label>API engine</Form.Label>
                    <Form.Control
                      as="select"
                      value={draft.engine}
                      onChange={(e) => {
                        setDraft({
                          ...draft,
                          engine: e.currentTarget.value,
                          login: "",
                          userID: "",
                        });
                        setAPIKey("");
                        setClearKey(draft.apiKeyConfigured);
                      }}
                    >
                      <option value="danbooru">Danbooru</option>
                      <option value="gelbooru">Gelbooru / JSON DAPI</option>
                      <option value="moebooru">Moebooru</option>
                      <option value="e621">e621 / e926</option>
                      <option value="philomena">Philomena</option>
                    </Form.Control>
                  </Form.Group>
                  <Form.Group controlId="booru-setting-url">
                    <Form.Label>Site URL</Form.Label>
                    <Form.Control
                      required
                      type="url"
                      placeholder="https://booru.example.org"
                      value={draft.url}
                      onChange={(e) =>
                        setDraft({ ...draft, url: e.currentTarget.value })
                      }
                    />
                    <Form.Text muted>
                      Public HTTPS only. Include any installation path.
                    </Form.Text>
                  </Form.Group>
                </div>
                <Form.Group controlId="booru-setting-hosts">
                  <Form.Label>Additional image hosts</Form.Label>
                  <Form.Control
                    as="textarea"
                    rows={2}
                    value={hosts}
                    onChange={(e) => setHosts(e.currentTarget.value)}
                    placeholder="cdn.example.org"
                  />
                  <Form.Text muted>
                    Optional CDN hostnames, separated by commas or newlines. The
                    site host and its subdomains are already allowed.
                  </Form.Text>
                </Form.Group>
                {draft.engine === "moebooru" ? (
                  <p>
                    Moebooru supports anonymous browsing. Password login is not
                    supported.
                  </p>
                ) : (
                  <>
                    <div className="booru-settings-fields">
                      {(draft.engine === "danbooru" ||
                        draft.engine === "e621") && (
                        <Form.Group controlId="booru-setting-login">
                          <Form.Label>Username</Form.Label>
                          <Form.Control
                            value={draft.login}
                            autoComplete="off"
                            onChange={(e) =>
                              setDraft({
                                ...draft,
                                login: e.currentTarget.value,
                              })
                            }
                            disabled={clearKey}
                          />
                        </Form.Group>
                      )}
                      {draft.engine === "gelbooru" && (
                        <Form.Group controlId="booru-setting-userid">
                          <Form.Label>User ID</Form.Label>
                          <Form.Control
                            value={draft.userID}
                            autoComplete="off"
                            onChange={(e) =>
                              setDraft({
                                ...draft,
                                userID: e.currentTarget.value,
                              })
                            }
                            disabled={clearKey}
                          />
                        </Form.Group>
                      )}
                      <Form.Group controlId="booru-setting-key">
                        <Form.Label>API key</Form.Label>
                        <Form.Control
                          type="password"
                          autoComplete="new-password"
                          value={apiKey}
                          disabled={clearKey}
                          onChange={(e) => setAPIKey(e.currentTarget.value)}
                          placeholder={
                            draft.apiKeyConfigured
                              ? "Saved — leave blank to keep"
                              : "Optional API key"
                          }
                        />
                        <Form.Text muted>
                          Saved keys are never returned. Changing the URL
                          requires re-entering or removing the key.
                        </Form.Text>
                      </Form.Group>
                    </div>
                    <Form.Check
                      id="booru-setting-clear"
                      label="Remove saved credentials on save"
                      checked={clearKey}
                      onChange={(e) => {
                        setClearKey(e.currentTarget.checked);
                        setAPIKey("");
                      }}
                    />
                  </>
                )}
                <Form.Check
                  id="booru-setting-enabled"
                  className="mt-3"
                  label="Enable this source"
                  checked={!draft.disabled}
                  onChange={(e) =>
                    setDraft({ ...draft, disabled: !e.currentTarget.checked })
                  }
                />
              </fieldset>
            </Modal.Body>
            <Modal.Footer>
              {!creating && (
                <Button
                  variant="outline-danger"
                  disabled={busy}
                  onClick={() => void save(draft.preset ? "reset" : "remove")}
                >
                  {draft.preset
                    ? "Reset preset and credentials"
                    : "Remove custom source"}
                </Button>
              )}
              <Button variant="secondary" disabled={busy} onClick={close}>
                Cancel
              </Button>
              <Button type="submit" disabled={busy}>
                {busy ? "Saving…" : "Save source"}
              </Button>
            </Modal.Footer>
          </Form>
        </Modal>
      )}
    </div>
  );
}
