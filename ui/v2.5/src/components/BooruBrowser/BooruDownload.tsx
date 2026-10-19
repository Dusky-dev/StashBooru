import { type Ref, useEffect, useRef, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { Link } from "react-router-dom";
import { type BooruPost, readBooruResponse } from "./browser";
import {
  faDownload,
  faFolderOpen,
  faRotateRight,
  faXmark,
  faArrowUpRightFromSquare,
} from "@fortawesome/free-solid-svg-icons";
import { Icon } from "../Shared/Icon";
import { FolderPicker } from "./FolderPicker";
import { loadDownloadPreferences } from "./downloadPreferences";
interface DownloadState {
  jobID: number;
  token: string;
  status: string;
  error?: string;
  savedPath?: string;
  library?: {
    kind: string;
    id: string;
    path: string;
    converted: boolean;
    notice?: string;
  };
}
function rememberedJob(identity: string): DownloadState | undefined {
  try {
    const value = JSON.parse(sessionStorage.getItem(identity) ?? "null") as {
      jobID: number;
      token: string;
      created: number;
    } | null;
    if (
      value &&
      Number.isSafeInteger(value.jobID) &&
      value.jobID > 0 &&
      /^[a-f0-9]{32}$/.test(value.token) &&
      Date.now() - value.created < 3600000
    )
      return { ...value, status: "queued" };
  } catch {
    /* Optional UI recovery. */
  }
  return undefined;
}
export function BooruDownload({
  post,
  instance,
  compact = false,
  downloadRef,
  archiveFile,
}: {
  post: BooruPost;
  instance: string;
  compact?: boolean;
  downloadRef?: Ref<HTMLButtonElement>;
  archiveFile?: number;
}) {
  const identity = `booru-download:${instance}:${post.id}${archiveFile === undefined ? "" : `:archive:${archiveFile}`}`;
  const [state, setState] = useState<DownloadState | undefined>(() =>
    rememberedJob(identity)
  );
  const [convert, setConvert] = useState(false);
  const [directory, setDirectory] = useState("");
  const [maxFileSizeMiB, setMaxFileSizeMiB] = useState(256);
  const [picker, setPicker] = useState(false);
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  useEffect(() => {
    let active = true;
    let generation = 0;
    const load = () => {
      const requested = ++generation;
      setReady(false);
      void loadDownloadPreferences()
        .then((settings) => {
          if (active && requested === generation) {
            setConvert(settings.convert);
            setDirectory(settings.defaultDirectory);
            setMaxFileSizeMiB(settings.maxFileSizeMiB ?? 256);
            setReady(true);
            setError("");
          }
        })
        .catch((cause: Error) => {
          if (active && requested === generation) {
            setError(cause.message);
            setReady(true);
          }
        });
    };
    load();
    window.addEventListener("booru-download-settings", load);
    return () => {
      active = false;
      window.removeEventListener("booru-download-settings", load);
    };
  }, []);

  useEffect(() => {
    const changed = (event: Event) => {
      const detail = (
        event as CustomEvent<{ identity: string; state: DownloadState }>
      ).detail;
      if (detail?.identity === identity) setState(detail.state);
    };
    window.addEventListener("booru-download-job", changed);
    return () => window.removeEventListener("booru-download-job", changed);
  }, [identity]);
  const jobID = state?.jobID,
    token = state?.token;
  const terminal =
    !!state && ["complete", "failed", "cancelled"].includes(state.status);
  const running = !!state && !terminal;
  useEffect(() => {
    if (!jobID || !token || terminal) return;
    const controller = new AbortController();
    let timer: number;
    const poll = async () => {
      try {
        const response = await fetch(
          `booru-browser/download?${new URLSearchParams({ jobID: String(jobID), token })}`,
          { signal: controller.signal }
        );
        if (response.status === 404 || response.status === 410) {
          if (!controller.signal.aborted)
            setState({
              jobID,
              token,
              status: "failed",
              error:
                "Job expired or the server restarted. Retry; saved files are retained.",
            });
          return;
        }
        const value = await readBooruResponse<DownloadState>(response);
        if (!controller.signal.aborted) {
          setState(value);
          setError("");
          timer = window.setTimeout(() => void poll(), 1000);
        }
      } catch (cause) {
        if (!controller.signal.aborted) {
          setError(cause instanceof Error ? cause.message : String(cause));
          timer = window.setTimeout(() => void poll(), 3000);
        }
      }
    };
    void poll();
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [jobID, token, terminal]);
  async function start(destination: string) {
    setBusy(true);
    setError("");
    try {
      const result = await fetch("booru-browser/download", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          key: post.mediaKey,
          convert,
          destination,
          archiveFile,
        }),
      }).then(readBooruResponse<{ jobID: number; token: string }>);
      if (mounted.current) setState({ ...result, status: "queued" });
      try {
        sessionStorage.setItem(
          identity,
          JSON.stringify({ ...result, created: Date.now() })
        );
      } catch {
        /* Job can still finish in Stash. */
      }
      window.dispatchEvent(
        new CustomEvent("booru-download-job", {
          detail: { identity, state: result },
        })
      );
    } catch (cause) {
      if (mounted.current)
        setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  async function cancel() {
    setBusy(true);
    try {
      await fetch("booru-browser/download", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ cancel: jobID, token }),
      }).then(readBooruResponse);
    } catch (cause) {
      if (mounted.current)
        setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  const available =
    !!post.mediaKey &&
    (archiveFile !== undefined || !!post.downloadAvailable) &&
    (post.bytes ?? 0) <= maxFileSizeMiB * 1048576;
  const libraryURL =
    state?.library && /^\d+$/.test(state.library.id)
      ? state.library.kind === "image"
        ? `/images/${state.library.id}`
        : state.library.kind === "scene"
          ? `/scenes/${state.library.id}`
          : undefined
      : undefined;
  return (
    <section
      className={`booru-download ${compact ? "booru-download-compact" : "mb-3"}`}
      aria-label={compact ? `Download post ${post.id}` : "Download this post"}
    >
      {!compact && (
        <Form.Check
          type="switch"
          id={`booru-convert-${post.sourceID}-${post.id}${archiveFile === undefined ? "" : `-${archiveFile}`}`}
          label="Convert using Media converter defaults"
          checked={convert}
          disabled={busy || running || !ready}
          onChange={(event) => setConvert(event.currentTarget.checked)}
        />
      )}
      {error && <Alert variant="warning">{error}</Alert>}
      {state?.error && <Alert variant="warning">{state.error}</Alert>}
      <Button
        ref={downloadRef}
        className={compact ? "minimal booru-icon-button" : undefined}
        size="sm"
        disabled={busy || running || !ready || !available || !directory}
        aria-label={
          compact ? `Download ${post.sourceID} post ${post.id}` : undefined
        }
        title={
          compact
            ? `Save to ${directory || "configured default folder"}${convert ? " using converter defaults" : ""}`
            : undefined
        }
        onClick={() => void start(directory)}
      >
        {compact ? (
          <Icon icon={terminal ? faRotateRight : faDownload} />
        ) : terminal ? (
          "Retry download to Stash"
        ) : (
          "Download to Stash"
        )}
      </Button>
      <Button
        size="sm"
        className={compact ? "minimal booru-icon-button" : "ml-2"}
        variant="secondary"
        disabled={busy || running || !ready || !available}
        aria-label={
          compact
            ? `Choose download folder for ${post.sourceID} post ${post.id}`
            : undefined
        }
        onClick={() => setPicker(true)}
      >
        {compact ? <Icon icon={faFolderOpen} /> : "Download to…"}
      </Button>
      {running && (
        <Button
          size="sm"
          className="ml-2"
          variant="secondary"
          disabled={busy}
          aria-label="Cancel download"
          title="Cancel download"
          onClick={() => void cancel()}
        >
          {compact ? <Icon icon={faXmark} /> : "Cancel download"}
        </Button>
      )}
      {state && !compact && (
        <span className="d-block mt-2" role="status">
          Job #{state.jobID}: {state.status}
        </span>
      )}
      {libraryURL && (
        <Link
          className={
            compact
              ? "btn btn-success btn-sm minimal booru-icon-button"
              : "btn btn-success btn-sm mt-2"
          }
          to={libraryURL}
          aria-label="Open in Stash"
          title="Open in Stash"
        >
          {compact ? <Icon icon={faArrowUpRightFromSquare} /> : "Open in Stash"}
        </Link>
      )}
      {!compact && (state?.library?.path || state?.savedPath) && (
        <small className="d-block">
          Saved: {state.library?.path || state.savedPath}
        </small>
      )}
      {!compact && state?.library?.notice && <p>{state.library.notice}</p>}
      {compact && running && (
        <span className="sr-only" role="status">
          Download {state?.status ?? "queued"}
        </span>
      )}
      {!available && !compact && (
        <small className="d-block">
          Original unavailable, larger than the configured {maxFileSizeMiB} MiB
          limit, or its source needs a CDN configuration update.
        </small>
      )}
      {picker && (
        <FolderPicker
          initial={directory}
          onClose={(value) => {
            setPicker(false);
            if (value) void start(value);
          }}
        />
      )}
    </section>
  );
}
