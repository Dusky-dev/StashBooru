import { useEffect, useRef, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { Link } from "react-router-dom";
import { type BooruPost, readBooruResponse } from "./browser";
import { FolderPicker } from "./FolderPicker";
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
}: {
  post: BooruPost;
  instance: string;
}) {
  const identity = `booru-download:${instance}:${post.id}`;
  const [state, setState] = useState<DownloadState | undefined>(() =>
    rememberedJob(identity)
  );
  const [convert, setConvert] = useState(false);
  const [directory, setDirectory] = useState("");
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
    const controller = new AbortController();
    void fetch("booru-browser/download-settings", { signal: controller.signal })
      .then(readBooruResponse<{ convert: boolean; defaultDirectory: string }>)
      .then((settings) => {
        if (!controller.signal.aborted) {
          setConvert(settings.convert);
          setDirectory(settings.defaultDirectory);
          setReady(true);
        }
      })
      .catch((cause: Error) => {
        if (!controller.signal.aborted) {
          setError(cause.message);
          setReady(true);
        }
      });
    return () => controller.abort();
  }, []);
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
        body: JSON.stringify({ key: post.mediaKey, convert, destination }),
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
    !!post.downloadAvailable &&
    (post.bytes ?? 0) <= 128 * 1048576;
  const libraryURL =
    state?.library && /^\d+$/.test(state.library.id)
      ? state.library.kind === "image"
        ? `/images/${state.library.id}`
        : state.library.kind === "scene"
          ? `/scenes/${state.library.id}`
          : undefined
      : undefined;
  return (
    <section className="booru-download mb-3" aria-label="Download this post">
      <h6>Download into Stash</h6>
      <p className="mb-1">
        Original
        {post.bytes
          ? ` · ${post.bytes.toLocaleString()} bytes`
          : " · size unknown"}
        . Maximum 128 MiB.
      </p>
      <Form.Check
        id={`booru-convert-${post.sourceID}-${post.id}`}
        label="Convert using Media converter defaults"
        checked={convert}
        disabled={busy || running || !ready}
        onChange={(event) => setConvert(event.currentTarget.checked)}
      />
      <small className="d-block text-muted mb-2">
        Uses the converter's saved format, quality, effort, worker and savings
        policy. Originals follow its normal recovery rules.
      </small>
      <p className="booru-destination">
        Default folder:{" "}
        {directory ||
          "Not configured — set it in Downloads & autocomplete, or use Download to…"}
      </p>
      {error && <Alert variant="warning">{error}</Alert>}
      {state?.error && <Alert variant="warning">{state.error}</Alert>}
      <Button
        size="sm"
        disabled={busy || running || !ready || !available || !directory}
        onClick={() => void start(directory)}
      >
        {terminal ? "Retry download to Stash" : "Download to Stash"}
      </Button>
      <Button
        size="sm"
        className="ml-2"
        variant="secondary"
        disabled={busy || running || !ready || !available}
        onClick={() => setPicker(true)}
      >
        Download to…
      </Button>
      {running && (
        <Button
          size="sm"
          className="ml-2"
          variant="secondary"
          disabled={busy}
          onClick={() => void cancel()}
        >
          Cancel download
        </Button>
      )}
      {state && (
        <span className="d-block mt-2" role="status">
          Job #{state.jobID}: {state.status}
        </span>
      )}
      {libraryURL && (
        <Link className="btn btn-success btn-sm mt-2" to={libraryURL}>
          Open in Stash
        </Link>
      )}
      {(state?.library?.path || state?.savedPath) && (
        <small className="d-block">
          Saved: {state.library?.path || state.savedPath}
        </small>
      )}
      {state?.library?.notice && <p>{state.library.notice}</p>}
      {!available && (
        <small className="d-block">
          Original unavailable, over the size limit, or its source needs a CDN
          configuration update.
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
