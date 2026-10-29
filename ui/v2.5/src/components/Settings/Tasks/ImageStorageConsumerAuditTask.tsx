import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form, Table } from "react-bootstrap";

interface SavedCandidate {
  id: string;
  group: string;
  method: string;
  snapshot: string;
}
interface CandidateStatus {
  candidates?: SavedCandidate[];
}
interface AuditResult {
  verified: boolean;
  readOnly: boolean;
  candidateID: string;
  group: string;
  mediaBackupVerified: boolean;
  archiveSHA256Verified: boolean;
  nativeConsumersVerified: boolean;
  canActivate: boolean;
  originalEvictionEnabled: boolean;
  blockers: string[];
  members: {
    id: string;
    bytes: number;
    byteExact: boolean;
    thumbnailCache: string;
    previewCache: string;
  }[];
}
interface AuditStatus {
  available: boolean;
  busy: boolean;
  result?: AuditResult;
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return (await response.json()) as T;
}

export const ImageStorageConsumerAuditTask: React.FC = () => {
  const [candidates, setCandidates] = useState<SavedCandidate[]>([]);
  const [candidateID, setCandidateID] = useState("");
  const [status, setStatus] = useState<AuditStatus>();
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const [saved, audit] = await Promise.all([
      fetch("image/storage-candidate", { signal }).then(
        readJSON<CandidateStatus>
      ),
      fetch("image/storage-consumer-audit", { signal }).then(
        readJSON<AuditStatus>
      ),
    ]);
    if (signal?.aborted) return;
    const items = saved.candidates ?? [];
    setCandidates(items);
    setStatus(audit);
    setCandidateID((previous) =>
      items.some((item) => item.id === previous)
        ? previous
        : (items[0]?.id ?? "")
    );
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal).catch((reason: unknown) => {
      if (!controller.signal.aborted) {
        setError(reason instanceof Error ? reason.message : String(reason));
      }
    });
    return () => controller.abort();
  }, [refresh]);

  const candidate = candidates.find((item) => item.id === candidateID);
  const stackID = candidate?.group.match(/^stack:([1-9]\d*)$/)?.[1];
  const busy = working || !!status?.busy;

  async function start() {
    if (!stackID) return;
    setWorking(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("image/storage-consumer-audit", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ stackID: Number(stackID), candidateID }),
      });
      const job = await readJSON<{ jobID: number }>(response);
      setMessage(
        "Read-only media audit queued as job #" +
          job.jobID +
          ". Check the job queue and refresh this report."
      );
      await refresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setWorking(false);
    }
  }

  const result = status?.result;
  const matches =
    result?.candidateID === candidateID &&
    result.verified &&
    result.readOnly &&
    result.mediaBackupVerified &&
    result.archiveSHA256Verified &&
    !result.nativeConsumersVerified &&
    !result.canActivate &&
    !result.originalEvictionEnabled;

  return (
    <section aria-labelledby="image-consumer-audit-title">
      <h2 id="image-consumer-audit-title">
        Audit packed-image consumer readiness
      </h2>
      <p>
        Check every archived image against its original and inspect existing
        generated thumbnail and preview cache files, without opening, generating
        or replacing those caches.
      </p>
      <Alert variant="warning">
        A present cache does not prove the thumbnail or preview can be served.
        This audit does not test native HTTP consumers or enable live packed
        serving, rollback or original deletion.
      </Alert>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      <Form.Group>
        <Form.Label htmlFor="consumer-audit-candidate">
          Saved candidate
        </Form.Label>
        <Form.Control
          as="select"
          id="consumer-audit-candidate"
          value={candidateID}
          disabled={busy}
          onChange={(event) => {
            setCandidateID(event.currentTarget.value);
            setMessage("");
          }}
        >
          {!candidateID && <option value="">Select a saved candidate</option>}
          {candidates.map((item) => (
            <option key={item.id} value={item.id}>
              {item.group} — {item.method} — {item.snapshot}
            </option>
          ))}
        </Form.Control>
      </Form.Group>
      <Button
        variant="secondary"
        disabled={busy || !stackID || !status?.available}
        onClick={() => void start()}
      >
        Audit all archived members (read-only)
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
        Refresh audit
      </Button>
      {status?.busy && (
        <p role="status" className="small mt-2">
          Verifying all original images and archive members in the job queue…
        </p>
      )}
      {matches && (
        <div className="mt-3">
          <Alert variant="info">
            Every audited member matches its original, but native media
            consumers remain unverified.
          </Alert>
          <Table size="sm" responsive>
            <thead>
              <tr>
                <th>Image</th>
                <th>Original bytes</th>
                <th>Thumbnail cache</th>
                <th>Preview cache</th>
              </tr>
            </thead>
            <tbody>
              {result.members.map((item) => (
                <tr key={item.id}>
                  <td>{item.id}</td>
                  <td>{item.bytes}</td>
                  <td>{item.thumbnailCache}</td>
                  <td>{item.previewCache}</td>
                </tr>
              ))}
            </tbody>
          </Table>
          <strong>Remaining blockers</strong>
          <ul>
            {result.blockers.map((blocker) => (
              <li key={blocker}>{blocker}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
};
