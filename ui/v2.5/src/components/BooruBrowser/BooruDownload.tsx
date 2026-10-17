import { useEffect, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { type BooruPost, readBooruResponse } from "./browser";

interface DownloadState {
  jobID: number;
  token: string;
  status: string;
  error?: string;
  url?: string;
  filename?: string;
  converted: boolean;
}
function rememberedJob(identity: string): DownloadState | undefined {
  try {
    const saved = JSON.parse(sessionStorage.getItem(identity) ?? "null") as {
      jobID: number;
      token: string;
      created: number;
    } | null;
    if (
      saved &&
      Number.isSafeInteger(saved.jobID) &&
      saved.jobID > 0 &&
      typeof saved.token === "string" &&
      /^[a-f0-9]{32}$/.test(saved.token) &&
      Date.now() - saved.created < 3600000
    )
      return {
        jobID: saved.jobID,
        token: saved.token,
        status: "queued",
        converted: false,
      };
  } catch {
    /* Storage may be disabled. Jobs still work in the current view. */
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
  const [convert, setConvert] = useState(false);
  const [ready, setReady] = useState(false);
  const [state, setState] = useState<DownloadState | undefined>(() =>
    rememberedJob(identity)
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void fetch("booru-browser/download-settings", { signal: controller.signal })
      .then(readBooruResponse<{ jpegToJXL: boolean }>)
      .then((settings) => {
        if (!controller.signal.aborted) {
          setConvert(settings.jpegToJXL);
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
  const jobID = state?.jobID;
  const token = state?.token;
  const terminal =
    state && ["complete", "failed", "cancelled"].includes(state.status);
  useEffect(() => {
    if (!jobID || terminal) return;
    const controller = new AbortController();
    let timer: number;
    const poll = async () => {
      try {
        const response = await fetch(
          `booru-browser/download?${new URLSearchParams({ jobID: String(jobID), token: token ?? "" })}`,
          {
            signal: controller.signal,
          }
        );
        if (response.status === 404 || response.status === 410) {
          if (!controller.signal.aborted)
            setState({
              jobID,
              token: token ?? "",
              status: "failed",
              converted: false,
              error:
                "Prepared download expired or the server restarted. Prepare it again.",
            });
          return;
        }
        const result = await readBooruResponse<DownloadState>(response);
        if (!controller.signal.aborted) {
          setState(result);
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
  const running = !!state && !terminal;
  async function start() {
    setBusy(true);
    setError("");
    try {
      const result = await fetch("booru-browser/download", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: post.mediaKey, convert }),
      }).then(readBooruResponse<{ jobID: number; token: string }>);
      setState({ ...result, status: "queued", converted: false });
      try {
        sessionStorage.setItem(
          identity,
          JSON.stringify({
            jobID: result.jobID,
            token: result.token,
            created: Date.now(),
          })
        );
      } catch {
        /* Optional reload recovery. */
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
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
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="booru-download mb-3" aria-label="Download this post">
      <h6>Download</h6>
      <p className="mb-1">
        Original file
        {post.bytes
          ? ` · ${(post.bytes / 1048576).toFixed(2)} MiB`
          : " · size unknown"}
        . Maximum 128 MiB. Saved to your computer.
      </p>
      <Form.Check
        id={`booru-convert-${post.sourceID}-${post.id}`}
        label="Convert JPEG to JPEG XL before downloading"
        checked={convert}
        disabled={busy || running || !ready}
        onChange={(event) => setConvert(event.currentTarget.checked)}
      />
      <small className="d-block text-muted mb-2">
        Uses Media converter settings and JPEG reconstruction. Other formats
        remain original. No catalogue entry or review mark is created.
      </small>
      {error && <Alert variant="warning">{error}</Alert>}
      {state?.error && <Alert variant="warning">{state.error}</Alert>}
      <Button
        size="sm"
        disabled={
          busy ||
          running ||
          !ready ||
          !post.mediaKey ||
          !post.downloadAvailable ||
          (post.bytes ?? 0) > 128 * 1048576
        }
        onClick={() => void start()}
      >
        {terminal ? "Prepare again" : "Prepare download"}
      </Button>
      {running && (
        <Button
          size="sm"
          variant="secondary"
          className="ml-2"
          disabled={busy}
          onClick={() => void cancel()}
        >
          Cancel download
        </Button>
      )}
      {state && (
        <span className="ml-2" role="status">
          Job #{state.jobID}: {state.status}
        </span>
      )}
      {state?.status === "complete" &&
        state.url &&
        /^downloads\/[a-zA-Z0-9]+\/[a-z0-9_-]+-\d+\.[a-z0-9]+$/.test(
          state.url
        ) && (
          <a
            className="btn btn-success btn-sm ml-2"
            href={state.url}
            download={state.filename}
          >
            Save {state.converted ? "JPEG XL" : "original"}
          </a>
        )}
      {terminal && (
        <small className="d-block mt-1">
          Prepared files expire after one hour, or shortly after saving. Prepare
          again if the link expired.
        </small>
      )}
      {!post.downloadAvailable && (
        <small className="d-block">
          The provider did not supply an allowed original URL. Check the
          source's CDN hosts or open the remote post.
        </small>
      )}
    </section>
  );
}
