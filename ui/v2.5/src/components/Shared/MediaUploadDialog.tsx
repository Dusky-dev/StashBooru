import { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Form, Modal, ProgressBar } from "react-bootstrap";
import { Link } from "react-router-dom";
import { faUpload } from "@fortawesome/free-solid-svg-icons";
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
import { runUploadBatch } from "./mediaUploadQueue";
import { Icon } from "./Icon";

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
  phase:
    | "uploading"
    | "downloading"
    | "processing"
    | "complete"
    | "attention"
    | "failed";
  loaded: number;
  total: number;
  message?: string;
  result?: UploadResult;
  savedPath?: string;
};

type RemoteEvent = {
  type: "progress" | "processing" | "complete" | "error";
  loaded?: number;
  total?: number;
  message?: string;
  result?: UploadResult;
  savedPath?: string;
};

class SavedUploadError extends Error {
  readonly savedPath: string;
  readonly result?: UploadResult;

  constructor(message: string, savedPath: string, result?: UploadResult) {
    super(message);
    this.savedPath = savedPath;
    this.result = result;
  }
}

function isUploadResult(value: unknown): value is UploadResult {
  if (!value || typeof value !== "object") return false;
  const { id, kind, path } = value as Partial<UploadResult>;
  return (
    (kind === "image" || kind === "scene") &&
    typeof id === "string" &&
    /^[1-9]\d*$/.test(id) &&
    typeof path === "string" &&
    path.length > 0
  );
}

function uploadError(message: string, savedPath: unknown, result?: unknown) {
  return typeof savedPath === "string" && savedPath
    ? new SavedUploadError(
        message,
        savedPath,
        isUploadResult(result) ? result : undefined
      )
    : new Error(message);
}

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
        const fallback = "Upload failed (HTTP " + request.status + ")";
        let error = new Error(fallback);
        try {
          const body = JSON.parse(request.responseText) as {
            error?: string;
            savedPath?: string;
            result?: UploadResult;
          } | null;
          error = uploadError(
            typeof body?.error === "string" ? body.error : fallback,
            body?.savedPath,
            body?.result
          );
        } catch {
          // Do not display HTML response bodies.
        }
        reject(error);
        return;
      }
      try {
        const result: unknown = JSON.parse(request.responseText);
        if (isUploadResult(result)) {
          resolve(result);
          return;
        }
      } catch {
        // A malformed response is not a successful import.
      }
      reject(new Error("The server did not return a valid import result"));
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
  let savedPath: string | undefined;
  const handleLine = (line: string) => {
    if (!line.trim()) return;
    const event = JSON.parse(line) as RemoteEvent;
    if (typeof event.savedPath === "string" && event.savedPath) {
      savedPath = event.savedPath;
    }
    if (event.type === "error") {
      throw uploadError(
        event.message ?? "Remote download failed",
        savedPath,
        event.result
      );
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
    } else if (event.type === "complete" && isUploadResult(event.result)) {
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
  } catch (reason) {
    if (imported) return imported;
    if (savedPath && !(reason instanceof SavedUploadError)) {
      throw new SavedUploadError(
        reason instanceof Error ? reason.message : String(reason),
        savedPath
      );
    }
    throw reason;
  } finally {
    reader.releaseLock();
  }
}

function uploadIsFinal(state?: EntryProgress) {
  return state?.phase === "complete" || state?.phase === "attention";
}

function UploadResultLink({ result }: { result: UploadResult }) {
  return (
    <Link
      className="ml-2"
      to={(result.kind === "image" ? "/images/" : "/scenes/") + result.id}
      target="_blank"
      rel="noopener noreferrer"
    >
      Open in Stash
    </Link>
  );
}

function UploadAttention({ state }: { state: EntryProgress }) {
  return (
    <div className="small mb-2 media-upload-attention">
      <strong>
        {state.result
          ? "Imported; needs attention"
          : "File saved; check import"}
      </strong>
      <div>{state.message}</div>
      <div className="text-break">{state.savedPath}</div>
      {state.result ? (
        <UploadResultLink result={state.result} />
      ) : (
        <span>Check the library or run Scan for the saved file.</span>
      )}
    </div>
  );
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
  const [picker, setPicker] = useState<{
    initial: string;
    ids: number[];
  }>();
  const [showDetails, setShowDetails] = useState(false);
  const [working, setWorking] = useState(false);
  const [paused, setPaused] = useState(false);
  const [error, setError] = useState("");
  const [ready, setReady] = useState(false);
  const [progress, setProgress] = useState<Record<number, EntryProgress>>({});
  const fileInputRef = useRef<HTMLInputElement>(null);
  const processedBatch = useRef(0);
  const processingNow = useRef(false);
  const pauseRequested = useRef(false);

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
  const pending = entries.filter((entry) => !uploadIsFinal(progress[entry.id]));

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

  function requestPause() {
    if (!processingNow.current) return;
    pauseRequested.current = true;
    setPaused(true);
  }

  function chooseFolder(targets: UploadEntry[]) {
    if (!active || working || !targets.length) return;
    setPicker({
      initial: active.destination,
      ids: targets.map((entry) => entry.id),
    });
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
      pauseRequested.current = false;
      setPaused(false);
      setWorking(true);
      setError("");
      const processing = { taggingDefaults, conversionDefaults };
      try {
        await runUploadBatch(
          batch,
          () => pauseRequested.current,
          async (entry, hasMore) => {
            if (!entry.destination) {
              setSelected(entry.id);
              setError("Choose a library destination for " + entry.name);
              return false;
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
              return true;
            } catch (reason) {
              const message =
                reason instanceof Error ? reason.message : String(reason);
              const saved =
                reason instanceof SavedUploadError ? reason : undefined;
              updateProgress(entry.id, {
                phase: saved ? "attention" : "failed",
                loaded: 0,
                total: 0,
                message,
                savedPath: saved?.savedPath,
                result: saved?.result,
              });
              setSelected(entry.id);
              if (saved) {
                if (saved.result) onImported();
                if (hasMore) {
                  setError(
                    "The other items are still queued. Import to continue."
                  );
                }
              } else {
                setError(
                  entry.name +
                    ": " +
                    message +
                    (hasMore ? ". Remaining items were not imported." : "")
                );
              }
              return false;
            }
          }
        );
      } finally {
        processingNow.current = false;
        setWorking(false);
      }
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
      !quickMode ||
      showDetails ||
      !ready ||
      !featureSettings?.quickUpload ||
      working ||
      paused ||
      pauseRequested.current ||
      processingNow.current
    )
      return;
    if (
      entries.some((entry) => {
        const phase = progress[entry.id]?.phase;
        return phase === "failed" || phase === "attention";
      })
    )
      return;
    const queued = entries.filter((entry) => !progress[entry.id]);
    if (!queued.length || !queued.every((entry) => !!entry.destination)) return;
    void submit(queued);
  }, [
    quickMode,
    showDetails,
    ready,
    featureSettings?.quickUpload,
    entries,
    progress,
    working,
    paused,
    submit,
  ]);

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

  const actionLabel = paused
    ? "Resume"
    : quickMode && featureSettings?.quickUpload
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
  const attention = entries.filter(
    (entry) => progress[entry.id]?.phase === "attention"
  );
  const activeTransfer = entries.find((entry) => {
    const phase = progress[entry.id]?.phase;
    return (
      phase === "uploading" || phase === "downloading" || phase === "processing"
    );
  });
  const activeStage = activeTransfer && progress[activeTransfer.id]?.phase;
  const activeLabel =
    activeStage === "uploading"
      ? "Uploading file…"
      : activeStage === "downloading"
        ? "Downloading file…"
        : activeStage === "processing"
          ? "Importing into Stash…"
          : "Starting imports…";
  const pauseButton = working && !paused && pending.length > 1 && (
    <Button size="sm" variant="secondary" onClick={requestPause}>
      Pause after current
    </Button>
  );
  const pauseNotice = paused && pending.length > 0 && (
    <Alert variant="info">
      {working
        ? "Finishing the current item before pausing."
        : "Upload queue paused. Pending items stay queued."}
    </Alert>
  );

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
              ? activeLabel
              : failed > 0
                ? `${failed} item(s) failed`
                : attention.length > 0
                  ? "Saved files need attention"
                  : completed === entries.length && completed > 0
                    ? "Import complete"
                    : paused
                      ? "Waiting to resume"
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
          {pauseNotice}
          {attention.map((entry) => (
            <div key={entry.id}>
              <div className="small font-weight-bold">{entry.name}</div>
              <UploadAttention state={progress[entry.id]} />
            </div>
          ))}
          {failed > 0 &&
            entries
              .filter((entry) => progress[entry.id]?.phase === "failed")
              .map((entry) => (
                <div key={entry.id} className="small text-danger text-truncate">
                  {entry.name}: {progress[entry.id]?.message}
                </div>
              ))}
          <div className="d-flex flex-wrap justify-content-end mt-2">
            {pauseButton}
            {paused && !working && pending.length > 0 && !failed && (
              <Button
                size="sm"
                className="ml-2"
                disabled={!ready}
                onClick={() => void submit(pending)}
              >
                {buttonLabel}
              </Button>
            )}
            <Button
              size="sm"
              variant="secondary"
              className="ml-2"
              onClick={() => setShowDetails(true)}
            >
              {pending.length ? "Details / retry" : "Details"}
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
            {pauseNotice}
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
              role="region"
              aria-label="Drop images or videos to upload"
              className="rounded p-4 mb-3 text-center"
              style={{
                border: "2px dashed currentColor",
                background: dropActive
                  ? "rgba(0, 0, 0, 0.55)"
                  : "rgba(128, 128, 128, 0.08)",
                color: dropActive ? "#fff" : undefined,
                transition: "background-color 150ms ease",
              }}
            >
              <div
                className="rounded-circle d-inline-flex align-items-center justify-content-center mb-3"
                style={{
                  width: 56,
                  height: 56,
                  fontSize: 24,
                  background: dropActive
                    ? "rgba(255, 255, 255, 0.2)"
                    : "rgba(128, 128, 128, 0.18)",
                }}
              >
                <Icon icon={faUpload} />
              </div>
              <div className="h5 mb-1">
                {dropActive
                  ? "Release to add files"
                  : "Drop images or videos here"}
              </div>
              <p
                className={dropActive ? "small mb-3" : "small text-muted mb-3"}
              >
                Drop files anywhere on the page, or select them from your
                device.
              </p>
              <Button
                type="button"
                size="sm"
                variant={dropActive ? "light" : "outline-primary"}
                disabled={working}
                onClick={() => fileInputRef.current?.click()}
              >
                Browse files
              </Button>
              <input
                ref={fileInputRef}
                id="media-upload-files"
                type="file"
                className="d-none"
                aria-label="Choose images or videos to upload"
                accept="image/*,video/*,.jxl,.heic,.heif,.mkv,.avi,.wmv,.ts"
                multiple
                disabled={working}
                onChange={(event) => {
                  const files = Array.from(event.currentTarget.files ?? []);
                  event.currentTarget.value = "";
                  addFiles(files);
                }}
              />
              <small
                className={
                  dropActive ? "d-block mt-3" : "d-block text-muted mt-3"
                }
              >
                Multiple files supported, including large videos. No fixed
                upload limit.
              </small>
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
                <small className="d-block text-muted">
                  Review the queue and metadata, then click Import to start.
                </small>
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
                    const final = uploadIsFinal(state);
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
                          {!working && !final && (
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
                        {state?.phase === "attention" && (
                          <UploadAttention state={state} />
                        )}
                        {state && state.phase !== "attention" && (
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
                              <UploadResultLink result={state.result} />
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
                disabled={working || uploadIsFinal(progress[active.id])}
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
                      onClick={() => chooseFolder([active])}
                    >
                      Browse…
                    </Button>
                  </div>
                  {pending.length > 1 && (
                    <Button
                      size="sm"
                      variant="link"
                      className="px-0"
                      onClick={() => chooseFolder(pending)}
                    >
                      Choose folder for {pending.length} pending items…
                    </Button>
                  )}
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
            {pauseButton}
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
      {picker && (
        <FolderPicker
          initial={picker.initial}
          allowCreate={featureSettings?.allowFolderCreation ?? false}
          title={
            picker.ids.length > 1
              ? "Choose folder for pending uploads"
              : "Choose upload folder"
          }
          onClose={(value) => {
            if (value) {
              const targets = new Set(picker.ids);
              setEntries((old) =>
                old.map((entry) =>
                  targets.has(entry.id) && !uploadIsFinal(progress[entry.id])
                    ? { ...entry, destination: value }
                    : entry
                )
              );
            }
            setPicker(undefined);
          }}
        />
      )}
    </>
  );
}
