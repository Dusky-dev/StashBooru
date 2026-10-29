import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";

interface SavedCandidate {
  id: string;
  group: string;
  method: string;
  snapshot: string;
}
interface CandidateStatus {
  candidates?: SavedCandidate[];
}
interface ReadinessReport {
  inspected: boolean;
  readOnly: boolean;
  candidateID: string;
  group: string;
  method: string;
  snapshot: string;
  images: number;
  sourceBytes: number;
  archiveBytes: number;
  archiveSHA256Verified: boolean;
  mediaBackupVerified: boolean;
  currentSourcesMatch: boolean;
  candidateHashesMatch: boolean;
  canActivate: boolean;
  canReplaceOriginals: boolean;
  originalEvictionEnabled: boolean;
  blockers: string[];
}
interface ReadinessStatus {
  available: boolean;
  busy: boolean;
  result?: ReadinessReport;
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok)
    throw new Error((await response.text()) || response.statusText);
  return (await response.json()) as T;
}

export const ImageStorageRestorePlanTask: React.FC = () => {
  const [candidates, setCandidates] = useState<SavedCandidate[]>([]);
  const [candidateID, setCandidateID] = useState("");
  const [status, setStatus] = useState<ReadinessStatus>();
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const [saved, plan] = await Promise.all([
      fetch("image/storage-candidate", { signal }).then(
        readJSON<CandidateStatus>
      ),
      fetch("image/storage-restore-plan", { signal }).then(
        readJSON<ReadinessStatus>
      ),
    ]);
    if (signal?.aborted) return;
    const items = saved.candidates ?? [];
    setCandidates(items);
    setStatus(plan);
    setCandidateID((previous) =>
      items.some((item) => item.id === previous)
        ? previous
        : (items[0]?.id ?? "")
    );
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal).catch((reason: unknown) => {
      if (!controller.signal.aborted)
        setError(reason instanceof Error ? reason.message : String(reason));
    });
    return () => controller.abort();
  }, [refresh]);

  const selection = candidates.find((item) => item.id === candidateID);
  const stackID = selection?.group.match(/^stack:([1-9]\d*)$/)?.[1];
  const locked = working || !!status?.busy;

  async function inspect() {
    if (!stackID) return;
    setWorking(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("image/storage-restore-plan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ stackID: Number(stackID), candidateID }),
      });
      const job = await readJSON<{ jobID: number }>(response);
      setMessage(
        "Read-only restore inspection queued as job #" + job.jobID +
          ". Check the job queue and refresh the report."
      );
      await refresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setWorking(false);
    }
  }

  const report = status?.result;
  const matches =
    report?.readOnly &&
    report.inspected &&
    report.candidateID === candidateID &&
    report.archiveSHA256Verified &&
    report.mediaBackupVerified &&
    report.currentSourcesMatch &&
    report.candidateHashesMatch &&
    !report.canActivate &&
    !report.canReplaceOriginals &&
    !report.originalEvictionEnabled;

  return (
    <section
      id="image-storage-restore-plan-task"
      aria-labelledby="image-storage-restore-plan-title"
    >
      <h2 id="image-storage-restore-plan-title">
        Inspect image restore readiness
      </h2>
      <p>
        Check one saved candidate against its verified archive fingerprint,
        current Visual Stack originals and complete media backup. This is a
        point-in-time read-only inspection, not a restore operation.
      </p>
      <Alert variant="warning">
        Even a passing inspection cannot activate packed images, replace
        originals or free disk space. Database recovery, native serving and
        crash-safe rollback must be verified separately.
      </Alert>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      <Form.Group>
        <Form.Label htmlFor="restore-plan-candidate">Saved candidate</Form.Label>
        <Form.Control
          as="select"
          id="restore-plan-candidate"
          value={candidateID}
          disabled={locked}
          onChange={(event) => {
            setCandidateID(event.currentTarget.value);
            setMessage("");
          }}
        >
          {!candidateID && <option value="">Select a candidate</option>}
          {candidates.map((item) => (
            <option key={item.id} value={item.id}>
              {item.group} — {item.method} — {item.snapshot}
            </option>
          ))}
        </Form.Control>
      </Form.Group>
      <Button
        variant="secondary"
        disabled={locked || !status?.available || !stackID}
        onClick={() => void inspect()}
      >
        Run read-only inspection
      </Button>{" "}
      <Button
        variant="outline-secondary"
        disabled={working}
        onClick={() => {
          setError("");
          void refresh().catch((reason: unknown) =>
            setError(reason instanceof Error ? reason.message : String(reason))
          );
        }}
      >
        Refresh report
      </Button>
      {status?.busy && (
        <p role="status" className="small mt-2">
          Checking saved archive and media backup in the job queue…
        </p>
      )}
      {matches && (
        <div className="mt-3">
          <Alert variant="info">
            Read-only hashes match for {report.images} images in {report.group}.
            The archive is still experimental and originals remain active.
          </Alert>
          <p>
            Original bytes: {report.sourceBytes}. Candidate bytes:
            {" "}{report.archiveBytes}.
          </p>
          <strong>Remaining blockers</strong>
          <ul>
            {report.blockers.map((blocker) => (
              <li key={blocker}>{blocker}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
};
