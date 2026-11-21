import React, { useEffect, useRef, useState } from "react";
import { Alert, Button } from "react-bootstrap";
import { useIntl } from "react-intl";
import { identityRequest } from "src/components/Identities/identityApi";

interface MediaDateStatus {
  running: boolean;
  lastRun?: {
    startedAt: string;
    finishedAt?: string;
    checked: number;
    fromFile: number;
    fromPosted: number;
    failed: number;
    cancelled?: boolean;
    note?: string;
  };
}

/**
 * Tasks → Library: give Images and Videos without a Date one from the
 * photo's capture date (EXIF), else the site's post date from saved Booru
 * downloads. Items with neither keep counting by date added.
 */
export const MediaDatesTask: React.FC = () => {
  const intl = useIntl();
  const m = (id: string, values?: Record<string, string | number>) =>
    intl.formatMessage({ id: `config.tasks.media_dates.${id}` }, values);
  const [status, setStatus] = useState<MediaDateStatus>();
  const [error, setError] = useState("");
  const controller = useRef(new AbortController());

  async function send(body?: unknown) {
    setError("");
    try {
      setStatus(
        await identityRequest<MediaDateStatus>(
          "media-dates",
          controller.current.signal,
          body
        )
      );
    } catch (e) {
      if (!controller.current.signal.aborted)
        setError(e instanceof Error ? e.message : String(e));
    }
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: load once on mount.
  useEffect(() => {
    void send();
    const current = controller.current;
    return () => current.abort();
  }, []);
  // Follow a running fill every few seconds.
  // biome-ignore lint/correctness/useExhaustiveDependencies: each status schedules the next poll.
  useEffect(() => {
    if (!status?.running) return;
    const timer = setTimeout(() => void send(), 3000);
    return () => clearTimeout(timer);
  }, [status]);

  const last = status?.lastRun;
  return (
    <section className="setting-group" aria-label={m("title")}>
      <h3>{m("title")}</h3>
      <p className="text-muted">{m("help")}</p>
      {error && <Alert variant="danger">{error}</Alert>}
      <Button
        variant="secondary"
        disabled={!status || status.running}
        onClick={() => void send({ action: "run" })}
      >
        {m("run")}
      </Button>
      <div role="status" className="mt-2">
        {status?.running && <div>{m("running")}</div>}
        {last?.finishedAt && (
          <div>
            {m(last.cancelled ? "cancelled" : "last", {
              checked: last.checked,
              file: last.fromFile,
              posted: last.fromPosted,
              failed: last.failed,
            })}{" "}
            {last.note}
          </div>
        )}
      </div>
    </section>
  );
};
