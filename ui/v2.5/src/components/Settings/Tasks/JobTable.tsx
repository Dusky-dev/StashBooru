import {
  faBan,
  faCheck,
  faCircle,
  faCircleExclamation,
  faCog,
  faHourglassStart,
  faTimes,
} from "@fortawesome/free-solid-svg-icons";
import moment from "moment/min/moment-with-locales";
import React, { useEffect, useRef, useState } from "react";
import { Alert, Button, Card, ProgressBar } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { Icon } from "src/components/Shared/Icon";
import { mutateStopJob } from "src/core/StashService";
import * as GQL from "src/core/generated-graphql";
import { useJobNotices } from "src/hooks/JobQueue";
import { JobNotice, jobKey } from "src/hooks/jobQueueState";
import { copyText } from "src/utils/clipboard";

interface IJob {
  job: JobNotice;
  expiresAt?: number;
  onDismiss: () => void;
}

const Task: React.FC<IJob> = ({ job, expiresAt, onDismiss }) => {
  const [stopping, setStopping] = useState(false);
  const [className, setClassName] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const mounted = useRef(true);

  useEffect(() => {
    const timer = window.setTimeout(() => setClassName("fade-in"));
    return () => {
      mounted.current = false;
      window.clearTimeout(timer);
    };
  }, []);

  useEffect(() => {
    if (expiresAt === undefined) {
      setClassName("fade-in");
      return;
    }
    const timer = window.setTimeout(
      () => setClassName("fade-out"),
      Math.max(0, expiresAt - Date.now() - 200)
    );
    return () => window.clearTimeout(timer);
  }, [expiresAt]);

  async function stopJob() {
    setStopping(true);
    try {
      await mutateStopJob(job.id);
    } catch (e) {
      if (mounted.current) {
        setActionMessage(String(e));
        setStopping(false);
      }
    }
  }

  function canStop() {
    return (
      !stopping &&
      (job.status === GQL.JobStatus.Ready ||
        job.status === GQL.JobStatus.Running)
    );
  }

  function getStatusClass() {
    switch (job.status) {
      case GQL.JobStatus.Ready:
        return "ready";
      case GQL.JobStatus.Running:
        return "running";
      case GQL.JobStatus.Stopping:
        return "stopping";
      case GQL.JobStatus.Finished:
        return "finished";
      case GQL.JobStatus.Cancelled:
        return "cancelled";
      case GQL.JobStatus.Failed:
        return "failed";
    }
  }

  function getStatusIcon() {
    let icon = faCircle;
    let iconClass = "";
    switch (job.status) {
      case GQL.JobStatus.Ready:
        icon = faHourglassStart;
        break;
      case GQL.JobStatus.Running:
        icon = faCog;
        iconClass = "fa-spin";
        break;
      case GQL.JobStatus.Stopping:
        icon = faCog;
        iconClass = "fa-spin";
        break;
      case GQL.JobStatus.Finished:
        icon = faCheck;
        break;
      case GQL.JobStatus.Cancelled:
        icon = faBan;
        break;
      case GQL.JobStatus.Failed:
        icon = faCircleExclamation;
        break;
    }

    return <Icon icon={icon} className={`fa-fw ${iconClass}`} />;
  }

  function maybeRenderProgress() {
    if (
      job.status === GQL.JobStatus.Running &&
      job.progress !== undefined &&
      job.progress !== null
    ) {
      const progress = job.progress * 100;
      return (
        <ProgressBar
          animated
          now={progress}
          label={`${progress.toFixed(0)}%`}
        />
      );
    }
  }

  function maybeRenderETA() {
    if (
      job.status === GQL.JobStatus.Running &&
      job.startTime !== null &&
      job.startTime !== undefined &&
      job.progress !== null &&
      job.progress !== undefined &&
      job.progress > 0
    ) {
      const now = new Date();
      const start = new Date(job.startTime);
      const nowMS = now.valueOf();
      const startMS = start.valueOf();
      const estimatedLength = (nowMS - startMS) / job.progress;
      const estLenStr = moment.duration(estimatedLength).humanize();
      return (
        <span className="job-eta">
          <FormattedMessage id="eta" />: {estLenStr}
        </span>
      );
    }
  }

  function maybeRenderSubTasks() {
    if (
      job.status === GQL.JobStatus.Running ||
      job.status === GQL.JobStatus.Stopping
    ) {
      return (
        <div>
          {/* XXbiome-ignore-start react/no-array-index-key: intentional */}
          {(job.subTasks ?? []).map((t, i) => (
            <div className="job-subtask" key={i}>
              {t}
            </div>
          ))}
          {/* XXbiome-ignore-end react/no-array-index-key: intentional */}
        </div>
      );
    }

    if (job.status === GQL.JobStatus.Failed) {
      return (
        <>
          <div className="job-error">
            {job.error || "No error details supplied."}
          </div>
          <div className="job-actions">
            <Button
              size="sm"
              variant="secondary"
              disabled={!job.error}
              onClick={async () => {
                try {
                  await copyText(job.error!);
                  if (mounted.current) setActionMessage("Error copied.");
                } catch (e) {
                  if (mounted.current) setActionMessage(String(e));
                }
              }}
            >
              Copy error
            </Button>
            <Button size="sm" variant="secondary" onClick={onDismiss}>
              Dismiss
            </Button>
          </div>
        </>
      );
    }
  }

  return (
    <li className={`job ${className}`} data-job-key={jobKey(job)}>
      <div>
        {job.status !== GQL.JobStatus.Failed && (
          <Button
            className="minimal stop"
            size="sm"
            onClick={() => stopJob()}
            disabled={!canStop()}
            aria-label={`Stop ${job.description}`}
          >
            <Icon icon={faTimes} />
          </Button>
        )}
        <div className={`job-status ${getStatusClass()}`}>
          <div className="job-description">
            <div>
              {getStatusIcon()}
              <span>{job.description}</span>
            </div>
            {maybeRenderETA()}
          </div>
          <div>{maybeRenderProgress()}</div>
          {maybeRenderSubTasks()}
          <div role="status">{actionMessage}</div>
        </div>
      </div>
    </li>
  );
};

export const JobTable: React.FC = () => {
  const intl = useIntl();
  const { entries, error, storageWarning, dismiss, refresh } = useJobNotices();
  const failed = entries.some(({ job }) => job.status === GQL.JobStatus.Failed);

  return (
    <Card className="job-table">
      <div className="job-actions">
        {failed && (
          <>
            <small>
              Failed jobs stay here until dismissed in this browser tab.
            </small>
            <Button size="sm" variant="secondary" onClick={() => dismiss()}>
              Dismiss all failures
            </Button>
          </>
        )}
        <Button size="sm" variant="link" onClick={refresh}>
          Refresh jobs
        </Button>
      </div>
      {error && <Alert variant="warning">{error}</Alert>}
      {storageWarning && (
        <Alert variant="warning">
          Browser storage is unavailable. Failed jobs will be kept until you
          leave or refresh this application.
        </Alert>
      )}
      <ul>
        {!entries.length ? (
          <li className="empty-queue-message fade-in">
            {intl.formatMessage({ id: "config.tasks.empty_queue" })}
          </li>
        ) : undefined}
        {entries.map(({ job, expiresAt }) => (
          <Task
            job={job}
            expiresAt={expiresAt}
            key={jobKey(job)}
            onDismiss={() => dismiss(jobKey(job))}
          />
        ))}
      </ul>
    </Card>
  );
};
