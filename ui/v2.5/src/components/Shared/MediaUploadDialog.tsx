import { useCallback, useEffect, useRef, useState } from "react";
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
import { MediaUploadThumbnail } from "./MediaUploadThumbnail";

type UploadResult = {
  kind: "image" | "scene";
  id: string;
  path: string;
  converted?: boolean;
  notice?: string;
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

type UploadProcessingOptions = {
  taggingDefaults: boolean;
  conversionDefaults: boolean;
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
  progress: (update: EntryProgress) => void,
  processing: UploadProcessingOptions
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
    form.append("taggingDefaults", String(processing.taggingDefaults));
    form.append("conversionDefaults", String(processing.conversionDefaults));
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
  progress: (update: EntryProgress) => void,
  processing: UploadProcessingOptions
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
      taggingDefaults: processing.taggingDefaults,
      conversionDefaults: processing.conversionDefaults,
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
  incomingBatch,
  dropActive = false,
  quickMode = false,
}: {
  onHide: () => void;
  onImported: () => void;
  incomingBatch?: { id: number; files: File[] };
  dropActive?: boolean;
  quickMode?: boolean;
}) {
  const [entries, setEntries] = useState<UploadEntry[]>([]);
  const [selected, setSelected] = useState<number | null>(null);
  const [defaultDirectory, setDefaultDirectory] = useState("");
  const [featureSettings, setFeatureSettings] = useState<MediaUploadSettings>();
  // These are queue-wide options. Their values are captured per batch when it
  // begins so later clicks never alter items already being processed.
  const [taggingDefaults, setTaggingDefaults] = useState(false);
  const [conversionDefaults, setConversionDefaults] = useState(false);
  const [url, setURL] = useState("");
  const [picker, setPicker] = useState(false);
  const [showDetails, setShowDetails] = useState(false);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const [ready, setReady] = useState(false);
  const [progress, setProgress] = useState<Record<number, EntryProgress>>({});
  const processedBatch = useRef(0);
  const processingNow = useRef(false);

  useEffect(() => {
    let active = true;
    void Promise.all([
      readMediaUploadSettings(),
      loadDownloadPreferences().catch(() => ({ defaultDirectory: "" })),
    ])
      .then(([feature, preference]) => {
        if (!active) return;
        setFeatureSettings(feature);
        setTaggingDefaults(feature.quickUpload && feature.quickTagDefaults);
        setConversionDefaults(
          feature.quickUpload && feature.quickConvertDefaults
        );
        if (!feature.enabled) {
          setError("Enable media uploads in Settings > Experimental first.");
          setReady(false);
          return;
        }
        const directory =
          feature.defaultDirectory || preference.defaultDirectory || "";
        setDefaultDirectory(directory);
        setEntries((old) =>
          old.map((entry) => ({
            ...entry,
            destination: entry.destination || directory,
          }))
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

  const addFiles = useCallback(
    (files: File[]) => {
      // Accept new drops during a running transfer; the next pass uploads them.
      if (files.length === 0) return;
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
    },
    [defaultDirectory]
  );

  // Append each incoming file drop once without losing edited queue items.
  useEffect(() => {
    if (!incomingBatch || incomingBatch.id === processedBatch.current) return;
    processedBatch.current = incomingBatch.id;
    addFiles(incomingBatch.files);
  }, [incomingBatch, addFiles]);

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

  const updateProgress = useCallback((id: number, next: EntryProgress) => {
    setProgress((old) => ({ ...old, [id]: next }));
  }, []);

  const submit = useCallback(
    async (batch: UploadEntry[]) => {
      if (
        !ready ||
        !featureSettings?.enabled ||
        processingNow.current ||
        batch.length === 0
      )
        return;
      processingNow.current = true;
      setWorking(true);
      setError("");
      const processing = { taggingDefaults, conversionDefaults };
      for (const entry of batch) {
        if (!entry.destination) {
          setSelected(entry.id);
          setError("Choose a library destination for " + entry.name);
          break;
        }
        try {
          const update = (state: EntryProgress) =>
            updateProgress(entry.id, state);
          const result = entry.file
            ? await uploadLocal(entry, update, processing)
            : await uploadURL(entry, update, processing);
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
      processingNow.current = false;
      setWorking(false);
    },
    [
      ready,
      featureSettings?.enabled,
      taggingDefaults,
      conversionDefaults,
      updateProgress,
      onImported,
    ]
  );

  // Quick upload needs no Import click. It only picks fresh entries: failed
  // items remain visible for a deliberate Retry, never an infinite auto-loop.
  useEffect(() => {
    if (
      !ready ||
      !featureSettings?.quickUpload ||
      working ||
      processingNow.current
    )
      return;
    if (entries.some((entry) => progress[entry.id]?.phase === "failed")) return;
    const queued = entries.filter((entry) => !progress[entry.id]);
    if (!queued.length || !queued.every((entry) => !!entry.destination)) return;
    void submit(queued);
  }, [ready, featureSettings?.quickUpload, entries, progress, working, submit]);

  useEffect(() => {
    if (
      quickMode &&
      ready &&
      entries.some((entry) => !entry.destination && !progress[entry.id])
    ) {
      // Folder selection still requires interaction if no default exists.
      setShowDetails(true);
    }
  }, [quickMode, ready, entries, progress]);

  const actionLabel = featureSettings?.quickUpload
    ? "Retry / Import"
    : "Import";
  const buttonLabel = working
    ? "Importing…"
    : `${actionLabel} ${pending.length} item(s)`;

  const completed = entries.filter(
    (entry) => progress[entry.id]?.phase === "complete"
  ).length;
  const failed = entries.filter(
    (entry) => progress[entry.id]?.phase === "failed"
  ).length;
  const activeTransfer = entries.find((entry) => {
    const phase = progress[entry.id]?.phase;
    return (
      phase === "uploading" || phase === "downloading" || phase === "processing"
    );
  });

  return (
    <>
      {quickMode && !showDetails && !picker && (
        <div
          role="status"
          aria-live="polite"
          className="card shadow p-3"
          style={{
            position: "fixed",
            right: "16px",
            bottom: "16px",
            width: "min(360px, calc(100vw - 32px))",
            zIndex: 1050,
            maxHeight: "min(70vh, 450px)",
            overflowY: "auto",
          }}
        >
          <strong>Quick upload</strong>
          <div className="small mt-1 mb-2">
            {working
              ? "Uploading and importing…"
              : failed > 0
                ? `${failed} item(s) failed`
                : completed === entries.length && completed > 0
                  ? "Import complete"
                  : "Preparing uploads…"}
            {" · "}
            {completed}/{entries.length} complete
          </div>
          {entries.length > 0 && (
            <div className="d-flex mb-2" style={{ gap: 6, overflowX: "auto" }}>
              {entries.slice(-6).map((entry) => (
                <div key={entry.id} className="flex-shrink-0">
                  <MediaUploadThumbnail
                    file={entry.file}
                    name={entry.name}
                    result={progress[entry.id]?.result}
                    size={64}
                  />
                </div>
              ))}
            </div>
          )}
          {activeTransfer && (
            <div
              className="small text-muted text-truncate mb-2"
              title={activeTransfer.name}
            >
              {activeTransfer.name}
              {progress[activeTransfer.id]?.message
                ? " — " + progress[activeTransfer.id]?.message
                : ""}
            </div>
          )}
          {error && <Alert variant="warning">{error}</Alert>}
          {failed > 0 &&
            entries
              .filter((entry) => progress[entry.id]?.phase === "failed")
              .map((entry) => (
                <div key={entry.id} className="small text-danger text-truncate">
                  {entry.name}: {progress[entry.id]?.message}
                </div>
              ))}
          <div className="d-flex justify-content-end mt-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => setShowDetails(true)}
            >
              Details / retry
            </Button>
            <Button
              size="sm"
              variant="outline-secondary"
              className="ml-2"
              disabled={working}
              onClick={onHide}
            >
              Dismiss
            </Button>
          </div>
        </div>
      )}
      {!picker && (!quickMode || showDetails) && (
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
            {featureSettings?.quickUpload &&
              ready &&
              entries.some(
                (entry) => !progress[entry.id] && !entry.destination
              ) && (
                <Alert variant="info">
                  Choose a library folder for each queued item or set a default
                  folder in Experimental settings to start quick upload.
                </Alert>
              )}
            <div
              className="rounded p-3 mb-3 text-center"
              style={{
                border: "2px dashed currentColor",
                background: dropActive ? "rgba(128, 128, 128, 0.2)" : undefined,
              }}
            >
              <div className="font-weight-bold mb-2">
                Drag and drop images or videos here
              </div>
              <Form.Group controlId="media-upload-files">
                <Form.Label>Or browse files from your device</Form.Label>
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
            </div>
            {featureSettings?.allowURL && (
              <Form.Group controlId="media-upload-url">
                <Form.Label>
                  Or download from a direct image/video URL
                </Form.Label>
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
                  Import queue — select a thumbnail to edit its metadata
                </strong>
                {featureSettings?.quickUpload && (
                  <small className="d-block text-muted">
                    Quick upload is on: new files start automatically when a
                    destination is configured.
                  </small>
                )}
                <fieldset
                  className="border rounded p-2 mt-2"
                  disabled={working || pending.length === 0}
                >
                  <legend className="h6 mb-1">
                    Processing for the entire queue
                  </legend>
                  <div className="d-flex align-items-center flex-wrap">
                    <Form.Check
                      id="upload-queue-tag-defaults"
                      type="switch"
                      className="mr-3"
                      label="Tag"
                      checked={taggingDefaults}
                      onChange={(event) =>
                        setTaggingDefaults(event.currentTarget.checked)
                      }
                    />
                    <Form.Check
                      id="upload-queue-convert-defaults"
                      type="switch"
                      label="Convert"
                      checked={conversionDefaults}
                      onChange={(event) =>
                        setConversionDefaults(event.currentTarget.checked)
                      }
                    />
                  </div>
                  {(taggingDefaults || conversionDefaults) && (
                    <small className="d-block text-muted">
                      {taggingDefaults && "Tagging uses saved defaults. "}
                      {conversionDefaults && "Conversion uses saved defaults."}
                    </small>
                  )}
                  <small className="d-block text-muted">
                    Applies to all queued uploads when import starts, not to
                    files already being processed.
                  </small>
                </fieldset>
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
                            variant={
                              selected === entry.id
                                ? "primary"
                                : "outline-secondary"
                            }
                            className="p-1 mr-2"
                            title={entry.name}
                            aria-label={"Select " + entry.name}
                            onClick={() => setSelected(entry.id)}
                          >
                            <MediaUploadThumbnail
                              file={entry.file}
                              name={entry.name}
                              result={state?.result}
                            />
                          </Button>
                          <div className="flex-grow-1" />
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
                            {state.result?.notice && (
                              <small className="d-block text-muted">
                                {state.result.notice}
                              </small>
                            )}
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
                <small
                  className="d-block text-muted text-truncate mb-2"
                  title={active.name}
                >
                  {active.name}
                </small>
                <div className="mb-3">
                  <MediaUploadThumbnail
                    file={active.file}
                    name={active.name}
                    result={progress[active.id]?.result}
                    size={220}
                  />
                </div>
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
              onClick={() => void submit(pending)}
            >
              {buttonLabel}
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
