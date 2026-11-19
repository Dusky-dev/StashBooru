import { useEffect, useRef, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { useIntl } from "react-intl";
import { identityRequest } from "../Identities/identityApi";
import type { FollowSchedule } from "./FollowAction";

const intervals = [0, 1, 3, 6, 12, 24];

/**
 * Following → Manage: turn automatic checks on or off, see the last and next
 * run, or check every follow now. onFinished reloads the follow list after a
 * run so counts and last-check times update.
 */
export function FollowScheduleSettings({
  onFinished,
}: {
  onFinished: () => void;
}) {
  const intl = useIntl();
  const [schedule, setSchedule] = useState<FollowSchedule>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const wasRunning = useRef(false);
  const finished = useRef(onFinished);
  finished.current = onFinished;

  async function send(body?: unknown) {
    const controller = new AbortController();
    setError("");
    try {
      const value = await identityRequest<FollowSchedule>(
        "following/schedule",
        controller.signal,
        body
      );
      setSchedule(value);
      if (wasRunning.current && !value.running) finished.current();
      wasRunning.current = value.running;
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  // biome-ignore lint/correctness/useExhaustiveDependencies: load once on mount.
  useEffect(() => {
    void send();
  }, []);
  // While a run is in progress, follow it every few seconds.
  // biome-ignore lint/correctness/useExhaustiveDependencies: each new status schedules the next poll.
  useEffect(() => {
    if (!schedule?.running) return;
    const timer = setTimeout(() => void send(), 3000);
    return () => clearTimeout(timer);
  }, [schedule]);

  const m = (id: string, values?: Record<string, string | number>) =>
    intl.formatMessage({ id: `following_action.auto.${id}` }, values);
  const time = (value: string) =>
    intl.formatDate(value, { dateStyle: "medium", timeStyle: "short" });
  const last = schedule?.lastRun;
  return (
    <section
      className="card card-body mb-3 following-schedule"
      aria-label={m("title")}
    >
      <h5>{m("title")}</h5>
      <p className="text-muted mb-2">{m("help")}</p>
      {error && <Alert variant="danger">{error}</Alert>}
      <div className="following-actions">
        <Form.Group controlId="following-interval" className="mb-0">
          <Form.Label className="mr-2">{m("interval")}</Form.Label>
          <Form.Control
            as="select"
            className="input-control d-inline-block w-auto"
            value={schedule?.intervalHours ?? 0}
            disabled={!schedule || busy}
            onChange={async (e) => {
              setBusy(true);
              await send({
                action: "set",
                intervalHours: Number(e.currentTarget.value),
              });
              setBusy(false);
            }}
          >
            {intervals.map((hours) => (
              <option key={hours} value={hours}>
                {hours === 0 ? m("off") : m("every", { hours })}
              </option>
            ))}
          </Form.Control>
        </Form.Group>
        <Button
          size="sm"
          variant="secondary"
          disabled={!schedule || schedule.running || busy}
          onClick={async () => {
            setBusy(true);
            wasRunning.current = true;
            await send({ action: "run" });
            setBusy(false);
          }}
        >
          {m("run_now")}
        </Button>
      </div>
      <div role="status" className="mt-2">
        {schedule?.running && <div>{m("running")}</div>}
        {last?.finishedAt && (
          <div>
            {m("last", {
              time: time(last.finishedAt),
              checked: last.checked,
              posts: last.newPosts,
              failed: last.failed,
            })}{" "}
            {last.note}
          </div>
        )}
        {schedule?.nextRun && schedule.intervalHours > 0 && (
          <div className="text-muted">
            {m("next", { time: time(schedule.nextRun) })}
          </div>
        )}
      </div>
    </section>
  );
}
