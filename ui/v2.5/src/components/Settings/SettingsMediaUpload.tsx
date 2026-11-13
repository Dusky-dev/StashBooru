import { useEffect, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { FolderPicker } from "../BooruBrowser/FolderPicker";
import {
  MediaUploadSettings,
  readMediaUploadSettings,
  saveMediaUploadSettings,
} from "../Shared/mediaUploadSettings";
import { SettingsDisclosure } from "./SettingsDisclosure";

const defaults: MediaUploadSettings = {
  enabled: false,
  allowURL: true,
  allowFolderCreation: true,
  defaultDirectory: "",
  quickUpload: false,
  quickTagDefaults: false,
  quickConvertDefaults: false,
};

export function SettingsMediaUpload() {
  const [settings, setSettings] = useState<MediaUploadSettings>(defaults);
  const [loaded, setLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [picker, setPicker] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    void readMediaUploadSettings(controller.signal)
      .then((current) => {
        if (controller.signal.aborted) return;
        setSettings(current);
        setLoaded(true);
      })
      .catch((reason: unknown) => {
        if (controller.signal.aborted) return;
        setError(reason instanceof Error ? reason.message : String(reason));
      });
    return () => controller.abort();
  }, []);

  async function save() {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const updated = await saveMediaUploadSettings(settings);
      setSettings(updated);
      setMessage("Experimental media upload settings saved.");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setSaving(false);
    }
  }

  return (
    <SettingsDisclosure
      id="experimental-media-uploads"
      title="Media uploads"
      description="Allow web uploads and optional direct-URL imports. Disabled by default while experimental."
    >
      <p className="text-muted">
        This feature writes directly to your configured Stash libraries. Local
        media is scanned into Stash after upload. Transfer sizes are not capped
        by StashBooru, but server storage and reverse-proxy limits still apply.
      </p>
      {error && <Alert variant="warning">{error}</Alert>}
      {message && <Alert variant="success">{message}</Alert>}
      <Form.Check
        id="experimental-enable-uploads"
        type="switch"
        label="Enable web media uploads"
        checked={settings.enabled}
        disabled={!loaded || saving}
        onChange={(event) => {
          const checked = event.currentTarget.checked;
          setSettings((old) => ({ ...old, enabled: checked }));
          setMessage("");
        }}
      />
      <fieldset disabled={!loaded || !settings.enabled || saving}>
        <Form.Check
          id="experimental-enable-url-imports"
          type="switch"
          label="Allow importing direct image and video URLs"
          checked={settings.allowURL}
          onChange={(event) => {
            const checked = event.currentTarget.checked;
            setSettings((old) => ({ ...old, allowURL: checked }));
          }}
        />
        <small className="d-block text-muted mb-2">
          Only public HTTP/HTTPS links on standard ports. Private network and
          localhost URLs are blocked even when URL imports are enabled.
        </small>
        <Form.Check
          id="experimental-enable-upload-folder-creation"
          type="switch"
          label="Allow creating subfolders during upload"
          checked={settings.allowFolderCreation}
          onChange={(event) => {
            const checked = event.currentTarget.checked;
            setSettings((old) => ({ ...old, allowFolderCreation: checked }));
          }}
        />

        <hr />
        <Form.Check
          id="experimental-enable-quick-upload"
          type="switch"
          label="Quick upload — start importing as soon as files are added"
          checked={settings.quickUpload}
          onChange={(event) => {
            const checked = event.currentTarget.checked;
            setSettings((old) => ({ ...old, quickUpload: checked }));
          }}
        />
        {settings.quickUpload && (
          <div className="ml-3 mt-2 mb-3">
            <Form.Check
              id="experimental-quick-upload-tag-defaults"
              type="switch"
              label="Use tagging defaults"
              checked={settings.quickTagDefaults}
              onChange={(event) => {
                const checked = event.currentTarget.checked;
                setSettings((old) => ({ ...old, quickTagDefaults: checked }));
              }}
            />
            <Form.Check
              id="experimental-quick-upload-conversion-defaults"
              type="switch"
              label="Use conversion defaults"
              checked={settings.quickConvertDefaults}
              onChange={(event) => {
                const checked = event.currentTarget.checked;
                setSettings((old) => ({
                  ...old,
                  quickConvertDefaults: checked,
                }));
              }}
            />
            <small className="d-block text-muted">
              When enabled, tagging uses saved Image Tagging (Camie) or
              unambiguous Video filename metadata; conversion uses saved Media
              converter rules. Quick upload needs a default library folder (or
              your saved Booru download folder).
            </small>
          </div>
        )}
        <Form.Group controlId="experimental-media-upload-default-folder">
          <Form.Label>Default upload folder (optional)</Form.Label>
          <div className="d-flex align-items-center">
            <Form.Control
              value={settings.defaultDirectory}
              readOnly
              placeholder="Choose a Stash library folder"
            />
            <Button
              variant="secondary"
              className="ml-2"
              disabled={saving}
              onClick={() => setPicker(true)}
            >
              Browse…
            </Button>
            {settings.defaultDirectory && (
              <Button
                variant="link"
                disabled={saving}
                onClick={() =>
                  setSettings((old) => ({ ...old, defaultDirectory: "" }))
                }
              >
                Clear
              </Button>
            )}
          </div>
          <Form.Text className="text-muted">
            Leave empty to use your existing Booru download folder preference or
            choose a folder when importing each file.
          </Form.Text>
        </Form.Group>
      </fieldset>
      <Button
        disabled={!loaded || saving}
        onClick={() => void save()}
        variant="primary"
      >
        {saving ? "Saving…" : "Save upload settings"}
      </Button>
      {picker && (
        <FolderPicker
          initial={settings.defaultDirectory}
          title="Select default upload folder"
          onClose={(value) => {
            if (value) {
              setSettings((old) => ({ ...old, defaultDirectory: value }));
            }
            setPicker(false);
          }}
        />
      )}
    </SettingsDisclosure>
  );
}
