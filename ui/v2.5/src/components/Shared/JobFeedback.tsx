import { useEffect, useRef, useState, type ReactNode } from "react";
import { Badge, Button, ProgressBar } from "react-bootstrap";
import { Link } from "react-router-dom";
import { copyText } from "src/utils/clipboard";

export function JobStatusBadge({
  status,
  hasErrors = false,
}: {
  status: string;
  hasErrors?: boolean;
}) {
  const states: Record<string, [string, string]> = {
    ready: ["Queued", "secondary"],
    queued: ["Queued", "secondary"],
    running: ["Running", "primary"],
    stopping: ["Stopping", "warning"],
    finished: ["Completed", "success"],
    complete: ["Completed", "success"],
    cancelled: ["Cancelled", "secondary"],
    failed: ["Failed", "danger"],
  };
  const [label, variant] = states[status.toLowerCase()] ?? [
    status,
    "secondary",
  ];
  const partial = label === "Completed" && hasErrors;
  return (
    <Badge variant={partial ? "warning" : variant}>
      {partial ? "Completed with errors" : label}
    </Badge>
  );
}

export function JobQueueLink() {
  return (
    <Link to="/settings?tab=tasks#job-queue" target="_blank" rel="noreferrer">
      View job queue (new tab)
    </Link>
  );
}

// Keep full error text available to copy even when a caller limits visible rows.
export function JobErrorDetails({
  error,
  children,
}: {
  error?: string | null;
  children?: ReactNode;
}) {
  const [message, setMessage] = useState("");
  const mounted = useRef(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  useEffect(() => setMessage(""), [error]);
  async function copy() {
    try {
      await copyText(error ?? "");
      if (mounted.current) setMessage("Error copied.");
    } catch (cause) {
      if (mounted.current) setMessage(String(cause));
    }
  }
  return (
    <div className="job-error-details">
      <div
        className="job-error"
        style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}
      >
        {children ?? (error || "No error details supplied.")}
      </div>
      <Button
        size="sm"
        variant="secondary"
        disabled={!error}
        onClick={() => void copy()}
      >
        Copy error
      </Button>
      <div role="status">{message}</div>
    </div>
  );
}

export interface ProcessingJobItem {
  target: { kind: "image" | "scene"; id: number };
  trialID?: string;
  error?: string;
}
export interface ProcessingJob {
  jobID: number;
  status: string;
  total: number;
  error?: string;
  backend?: string;
  notice?: string;
  action?: string;
  items: ProcessingJobItem[];
}
function itemLabel(item: ProcessingJobItem) {
  return item.target.id > 0
    ? `${item.target.kind === "image" ? "Image" : "Video"} #${item.target.id}`
    : `Trial ${item.trialID ?? "(unknown)"}`;
}

export function ProcessingJobFeedback({
  job,
  onCancel,
  cancelling = false,
  onNavigate,
}: {
  job: ProcessingJob;
  onCancel: () => void;
  cancelling?: boolean;
  onNavigate: () => void;
}) {
  const errors = job.items.filter((item) => item.error);
  const errorText = [
    job.error,
    ...errors.map((item) => `${itemLabel(item)}: ${item.error}`),
  ]
    .filter(Boolean)
    .join("\n");
  const active = ["queued", "running"].includes(job.status.toLowerCase());
  return (
    <section className="mb-3" aria-label="Conversion job feedback">
      <div role="status">
        <strong>
          {job.action ?? "Conversion"} job #{job.jobID}
        </strong>{" "}
        <JobStatusBadge status={job.status} hasErrors={!!errorText} />
        <span>
          {" "}
          · {job.items.length}/{job.total} processed
        </span>
        {job.backend && <span> · {job.backend} worker</span>}
      </div>
      {job.notice && <p className="text-muted">{job.notice}</p>}
      {active && (
        <>
          <ProgressBar
            animated
            now={job.total ? (100 * job.items.length) / job.total : 0}
          />
          <Button
            size="sm"
            variant="secondary"
            className="my-2"
            disabled={cancelling}
            onClick={onCancel}
          >
            {cancelling ? "Requesting cancellation…" : "Cancel remaining work"}
          </Button>
        </>
      )}
      {errorText && (
        <div className="bg-dark border rounded p-2 my-2">
          <JobErrorDetails error={errorText}>
            {job.error && <div>{job.error}</div>}
            <div
              role="log"
              aria-label="Conversion errors"
              style={{ maxHeight: "12rem", overflowY: "auto" }}
            >
              {errors.slice(0, 50).map((item, index) => (
                <div
                  key={`${item.target.kind}-${item.target.id}-${index}`}
                  className="text-danger"
                >
                  {Number.isSafeInteger(item.target.id) &&
                  item.target.id > 0 ? (
                    <Link
                      to={`/${item.target.kind === "image" ? "images" : "scenes"}/${item.target.id}`}
                      onClick={onNavigate}
                    >
                      {itemLabel(item)}
                    </Link>
                  ) : (
                    itemLabel(item)
                  )}
                  : {item.error}
                </div>
              ))}
            </div>
            {errors.length > 50 && (
              <p>
                Showing 50 of {errors.length} item errors. Copy error includes
                all returned errors.
              </p>
            )}
          </JobErrorDetails>
        </div>
      )}
      <p className="small text-muted mb-1">
        Cancellation stops remaining work; completed results are retained. Live
        job details are temporary; saved conversion records and available
        originals are in History.
      </p>
      <JobQueueLink />
    </section>
  );
}
