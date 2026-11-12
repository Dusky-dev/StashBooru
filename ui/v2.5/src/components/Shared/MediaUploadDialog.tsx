import { useEffect, useState } from "react";
import { Alert, Button, Form, Modal, ProgressBar } from "react-bootstrap";
import { Link } from "react-router-dom";
import { FolderPicker } from "../BooruBrowser/FolderPicker";
import { readBooruResponse } from "../BooruBrowser/browser";
import { loadDownloadPreferences } from "../BooruBrowser/downloadPreferences";
import {
  MediaUploadSettings,
  readMediaUploadSettings,
} from "./mediaUploadSettings";
import { StudioIDSelect } from "../Studios/StudioSelect";
import {
  GalleryIDSelect,
  excludeFileBasedGalleries,
} from "../Galleries/GallerySelect";
import { TagIDSelect } from "../Tags/TagSelect";

type UploadResult = {
  kind: "image" | "scene";
  id: string;
  path: string;
};

type UploadEntry = {
  id: number;
  name: string;
  file?: File;
  url?: string;
  destination: string;
  artistIDs: string[];
  galleryIDs: string[];
  tagIDs: string[];
};

type EntryProgress = {
  phase: "uploading" | "downloading" | "processing" | "complete" | "failed";
  loaded: number;
  total: number;
  message?: string;
  result?: UploadResult;
};

type RemoteEvent = {
  type: "progress" | "processing" | "complete" | "error";
  loaded?: number;
  total?: number;
  message?: string;
  result?: UploadResult;
};

const describeSize = (bytes: number) =>
  bytes >= 1048576
    ? (bytes / 1048576).toFixed(1) + " MiB"
    : (bytes / 1024).toFixed(1) + " KiB";

function uploadLocal(
  entry: UploadEntry,
  progress: (update: EntryProgress) => void
): Promise<UploadResult> {
  return new Promise((resolve, reject) => {
    if (!entry.file) {
      reject(new Error("No local file selected"));
      return;
    }
    const file = entry.file;
    const request = new XMLHttpRequest();
    const form = new FormData();
    // The server processes and streams each file into the selected folder.
    form.append("destination", entry.destination);
    form.append("artistIDs", entry.artistIDs.join(","));
    form.append("galleryIDs", entry.galleryIDs.join(","));
    form.append("tagIDs", entry.tagIDs.join(","));
    form.append("file", file, file.name);
    request.open("POST", "media-upload");
    request.upload.onprogress = (event) => {
      progress({
        phase: "uploading",
        loaded: event.loaded,
        total: event.lengthComputable ? event.total : 0,
      });
    };
    request.upload.onload = () => {
      progress({
        phase: "processing",
        loaded: file.size,
        total: file.size,
        message: "Transfer complete; scanning and applying metadata",
      });
    };
    request.onerror = () => reject(new Error("Upload connection failed"));
    request.onabort = () => reject(new Error("Upload cancelled"));
    request.onload = () => {
      if (request.status < 200 || request.status >= 300) {
        let message = "Upload failed (HTTP " + request.status + ")";
        try {
          const body = JSON.parse(request.responseText) as { error?: string };
          if (typeof body.error === "string") message = body.error;
        } catch {
          // Do not display HTML response bodies.
        }
        reject(new Error(message));
        return;
      }
      try {
        resolve(JSON.parse(request.responseText) as UploadResult);
      } catch {
        reject(new Error("The server did not return a valid import result"));
      }
    };
    progress({ phase: "uploading", loaded: 0, total: file.size });
    request.send(form);
  });
}

async function uploadURL(
  entry: UploadEntry,
  progress: (update: EntryProgress) => void
): Promise<UploadResult> {
  const response = await fetch("media-upload/url", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      url: entry.url,
      destination: entry.destination,
      artistIDs: entry.artistIDs.join(","),
      galleryIDs: entry.galleryIDs.join(","),
      tagIDs: entry.tagIDs.join(","),
    }),
  });
  if (!response.ok) {
    await readBooruResponse(response);
    throw new Error("Could not download remote media");
  }
  if (!response.body) {
    throw new Error("Streaming URL progress is unavailable in this browser");
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let remainder = "";
  let imported: UploadResult | undefined;
  const handleLine = (line: string) => {
    if (!line.trim()) return;
    const event = JSON.parse(line) as RemoteEvent;
    if (event.type === "error") {
      throw new Error(event.message ?? "Remote download failed");
    }
    if (event.type === "progress") {
      progress({
        phase: "downloading",
        loaded: event.loaded ?? 0,
        total: Math.max(0, event.total ?? 0),
      });
    } else if (event.type === "processing") {
      progress({
        phase: "processing",
        loaded: 0,
        total: 0,
        message: event.message ?? "Importing into Stash",
      });
    } else if (event.type === "complete" && event.result) {
      imported = event.result;
    }
  };
  try {
    for (;;) {
      const { done, value } = await reader.read();
      remainder += decoder.decode(value ?? new Uint8Array(), { stream: !done });
      let nextLine = remainder.indexOf("\n");
      while (nextLine >= 0) {
        handleLine(remainder.slice(0, nextLine));
        remainder = remainder.slice(nextLine + 1);
        nextLine = remainder.indexOf("\n");
      }
      if (done) break;
    }
    if (remainder.trim()) handleLine(remainder);
    if (!imported) throw new Error("Remote import ended without a result");
    return imported;
  } finally {
    reader.releaseLock();
  }
}

let nextUploadId = 0;

export function MediaUploadDialog({
  onHide,
  onImported,
}: {
  onHide: () => void;
  onImported: () => void;
}) {
  const [entries, setEntries] = useState<UploadEntry[]>([]);
  const [selected, setSelected] = useState<number | null>(null);
  const [defaultDirectory, setDefaultDirectory] = useState("");
  const [featureSettings, setFeatureSettings] = useState<MediaUploadSettings>();
  const [url, setURL] = useState("");
  const [picker, setPicker] = useState(false);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const [ready, setReady] = useState(false);
  const [progress, setProgress] = useState<Record<number, EntryProgress>>({});

  useEffect(() => {
    let active = true;
    void Promise.all([readMediaUploadSettings(), loadDownloadPreferences()])
      .then(([feature, preference]) => {
        if (!active) return;
        setFeatureSettings(feature);
        if (!feature.enabled) {
          setError("Enable media uploads in Settings > Experimental first.");
          setReady(false);
          return;
        }
        const directory =
          feature.defaultDirectory || preference.defaultDirectory || "";
        setDefaultDirectory(directory);
        setEntries((old) =>
          old.map((entry) =>
            entry.destination ? entry : { ...entry, destination: directory }
          )
        );
        setReady(true);
      })
      .catch((reason: unknown) => {
        if (active) {
          setError(reason instanceof Error ? reason.message : String(reason));
          setReady(false);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const active = entries.find((entry) => entry.id === selected);
  const pending = entries.filter(
    (entry) => progress[entry.id]?.phase !== "complete"
  );

  function addFiles(files: File[]) {
    if (files.length === 0 || working) return;
    const newEntries = files.map(
      (file): UploadEntry => ({
        id: ++nextUploadId,
        name: file.name,
        file,
        destination: defaultDirectory,
        artistIDs: [],
        galleryIDs: [],
        tagIDs: [],
      })
    );
    setEntries((old) => [...old, ...newEntries]);
    setSelected(newEntries[0].id);
  }

  function addURL() {
    const value = url.trim();
    if (!value || working || !featureSettings?.allowURL) return;
    try {
      const parsed = new URL(value);
      if (!["http:", "https:"].includes(parsed.protocol)) {
        throw new Error("Only HTTP and HTTPS URLs are supported");
      }
    } catch {
      setError("Enter a valid HTTP or HTTPS direct media URL.");
      return;
    }
    const entry: UploadEntry = {
      id: ++nextUploadId,
      name: value,
      url: value,
      destination: defaultDirectory,
      artistIDs: [],
      galleryIDs: [],
      tagIDs: [],
    };
    setEntries((old) => [...old, entry]);
    setSelected(entry.id);
    setURL("");
    setError("");
  }

  function updateEntry(changes: Partial<UploadEntry>) {
    if (!active || working) return;
    setEntries((old) =>
      old.map((entry) =>
        entry.id === active.id ? { ...entry, ...changes } : entry
      )
    );
  }

  function updateProgress(id: number, next: EntryProgress) {
    setProgress((old) => ({ ...old, [id]: next }));
  }

  async function submit() {
    if (!ready || !featureSettings?.enabled || working || pending.length === 0) return;
    setWorking(true);
    setError("");
    for (const entry of pending) {
      if (!entry.destination) {
        setSelected(entry.id);
        setError("Choose a library destination for " + entry.name);
        break;
      }
      try {
        const update = (state: EntryProgress) =>
          updateProgress(entry.id, state);
        const result = entry.file
          ? await uploadLocal(entry, update)
          : await uploadURL(entry, update);
        updateProgress(entry.id, {
          phase: "complete",
          loaded: entry.file?.size ?? 0,
          total: entry.file?.size ?? 0,
          result,
        });
        onImported();
      } catch (reason) {
        const message =
          reason instanceof Error ? reason.message : String(reason);
        updateProgress(entry.id, {
          phase: "failed",
          loaded: 0,
          total: 0,
          message,
        });
        setSelected(entry.id);
        setError(
          entry.name + ": " + message + ". Remaining items were not imported."
        );
        break;
      }
    }
    setWorking(false);
  }

  return (
    <>
      {!picker && (
        <Modal
          show
          size="lg"
          onHide={() => {
            if (!working) onHide();
          }}
        >
          <Modal.Header closeButton={!working}>
            <Modal.Title>Add media to Stash</Modal.Title>
          </Modal.Header>
          <Modal.Body>
            {error && <Alert variant="warning">{error}</Alert>}
            <Form.Group controlId="media-upload-files">
              <Form.Label>Upload from device</Form.Label>
              <Form.Control
                type="file"
                accept="image/*,video/*,.jxl,.heic,.heif,.mkv,.avi,.wmv,.ts"
                multiple
                disabled={working}
                onChange={(event) => {
                  addFiles(
                    Array.from(
                      (event.currentTarget as HTMLInputElement).files ?? []
                    )
                  );
                  event.currentTarget.value = "";
                }}
              />
              <Form.Text className="text-muted">
                Add multiple files, including large videos. No fixed upload
                limit; files stream into the Stash library.
              </Form.Text>
            </Form.Group>
            {featureSettings?.allowURL && (
            <Form.Group controlId="media-upload-url">
              <Form.Label>Or download from a direct image/video URL</Form.Label>
              <div className="d-flex">
                <Form.Control
                  value={url}
                  disabled={working}
                  type="url"
                  placeholder="https://example.org/media/image.jpg"
                  onChange={(event) => setURL(event.currentTarget.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      addURL();
                    }
                  }}
                />
                <Button
                  className="ml-2"
                  variant="secondary"
                  disabled={working || !url.trim()}
                  onClick={addURL}
                >
                  Add URL
                </Button>
              </div>
              <Form.Text className="text-muted">
                Direct public HTTP/HTTPS media links only. Local and private
                network destinations are blocked.
              </Form.Text>
            </Form.Group>
            )}
            {entries.length > 0 && (
              <>
                <strong>
                  Import queue — select an item to edit its metadata
                </strong>
                <div
                  className="border rounded mb-3 mt-2 p-2"
                  style={{ maxHeight: "220px", overflowY: "auto" }}
                >
                  {entries.map((entry) => {
                    const state = progress[entry.id];
                    const complete = state?.phase === "complete";
                    const percent =
                      state?.total && state.total > 0
                        ? Math.min(
                            100,
                            Math.round((state.loaded / state.total) * 100)
                          )
                        : 0;
                    return (
                      <div key={entry.id} className="mb-2">
                        <div className="d-flex align-items-center">
                          <Button
                            size="sm"
                            variant={selected === entry.id ? "primary" : "link"}
                            className="text-truncate"
                            style={{ maxWidth: "85%" }}
                            onClick={() => setSelected(entry.id)}
                          >
                            {entry.file ? "File: " : "URL: "}
                            {entry.name}
                          </Button>
                          {!working && !complete && (
                            <Button
                              size="sm"
                              variant="outline-secondary"
                              className="ml-auto"
                              aria-label={"Remove " + entry.name}
                              onClick={() => {
                                setEntries((old) =>
                                  old.filter((item) => item.id !== entry.id)
                                );
                                if (selected === entry.id) setSelected(null);
                              }}
                            >
                              Remove
                            </Button>
                          )}
                        </div>
                        {state && (
                          <div className="px-2">
                            {state.phase === "uploading" ||
                            state.phase === "downloading" ? (
                              <ProgressBar
                                animated
                                striped={state.total === 0}
                                now={state.total === 0 ? 100 : percent}
                                label={
                                  state.total > 0 ? percent + "%" : undefined
                                }
                                className="mb-1"
                              />
                            ) : state.phase === "processing" ? (
                              <ProgressBar
                                animated
                                striped
                                now={100}
                                className="mb-1"
                              />
                            ) : null}
                            <small className="text-muted">
                              {state.phase === "uploading" ||
                              state.phase === "downloading"
                                ? (state.phase === "uploading"
                                    ? "Uploading: "
                                    : "Downloading: ") +
                                  describeSize(state.loaded) +
                                  (state.total > 0
                                    ? " / " + describeSize(state.total)
                                    : " (size unknown)")
                                : state.phase === "complete"
                                  ? "Imported"
                                  : (state.message ?? state.phase)}
                            </small>
                            {state.result && (
                              <Link
                                className="ml-2"
                                onClick={onHide}
                                to={
                                  state.result.kind === "image"
                                    ? "/images/" + state.result.id
                                    : "/scenes/" + state.result.id
                                }
                              >
                                Open in Stash
                              </Link>
                            )}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </>
            )}
            {active && (
              <fieldset
                disabled={working || progress[active.id]?.phase === "complete"}
              >
                <legend className="h6">
                  Metadata and destination for selected item
                </legend>
                <Form.Group controlId="media-upload-destination">
                  <Form.Label>Save to folder</Form.Label>
                  <div className="d-flex align-items-center">
                    <Form.Control
                      value={active.destination}
                      placeholder="Select or create a library folder"
                      readOnly
                    />
                    <Button
                      variant="secondary"
                      className="ml-2"
                      onClick={() => setPicker(true)}
                    >
                      Browse…
                    </Button>
                  </div>
                </Form.Group>
                <Form.Group>
                  <Form.Label>Artist(s)</Form.Label>
                  <StudioIDSelect
                    ids={active.artistIDs}
                    isMulti
                    isClearable
                    onSelect={(items) =>
                      updateEntry({ artistIDs: items.map((item) => item.id) })
                    }
                    menuPortalTarget={document.body}
                  />
                </Form.Group>
                <Form.Group>
                  <Form.Label>Gallery / galleries</Form.Label>
                  <GalleryIDSelect
                    ids={active.galleryIDs}
                    isMulti
                    isClearable
                    extraCriteria={excludeFileBasedGalleries}
                    onSelect={(items) =>
                      updateEntry({ galleryIDs: items.map((item) => item.id) })
                    }
                    menuPortalTarget={document.body}
                  />
                  <Form.Text className="text-muted">
                    User-created galleries only.
                  </Form.Text>
                </Form.Group>
                <Form.Group>
                  <Form.Label>Tags</Form.Label>
                  <TagIDSelect
                    ids={active.tagIDs}
                    isMulti
                    isClearable
                    onSelect={(items) =>
                      updateEntry({ tagIDs: items.map((item) => item.id) })
                    }
                    menuPortalTarget={document.body}
                  />
                </Form.Group>
              </fieldset>
            )}
          </Modal.Body>
          <Modal.Footer>
            <Button variant="secondary" disabled={working} onClick={onHide}>
              {pending.length === 0 ? "Close" : "Cancel"}
            </Button>
            <Button
              variant="primary"
              disabled={!ready || working || pending.length === 0}
              onClick={() => void submit()}
            >
              {working ? "Importing…" : "Import " + pending.length + " item(s)"}
            </Button>
          </Modal.Footer>
        </Modal>
      )}
      {picker && active && (
        <FolderPicker
          initial={active.destination}
          allowCreate={featureSettings?.allowFolderCreation ?? false}
          title="Choose upload folder"
          onClose={(value) => {
            if (value) updateEntry({ destination: value });
            setPicker(false);
          }}
        />
      )}
    </>
  );
}
