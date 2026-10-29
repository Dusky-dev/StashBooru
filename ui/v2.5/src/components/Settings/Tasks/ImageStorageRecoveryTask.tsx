import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";

interface CandidateStatus {
  result?: {
    group: string;
    verified: boolean;
    backupVerified: boolean;
    archivePath: string;
  };
  busy: boolean;
}
interface RecoveryStatus {
  available: boolean;
  busy: boolean;
  result?: {
    verified: boolean;
    byteExact: boolean;
    complete: boolean;
    originalEvictionEnabled: boolean;
    archiveActivated: boolean;
    databaseModified: boolean;
    restoredFiles: number;
    outputPath: string;
  };
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok)
    throw new Error((await response.text()) || response.statusText);
  return (await response.json()) as T;
}

export const ImageStorageRecoveryTask: React.FC = () => {
  const [candidate, setCandidate] = useState<CandidateStatus>();
  const [status, setStatus] = useState<RecoveryStatus>();
  const [acknowledged, setAcknowledged] = useState(false);
  const [requesting, setRequesting] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const [candidateValue, recoveryValue] = await Promise.all([
      fetch("image/storage-candidate", { signal }).then(
        readJSON<CandidateStatus>
      ),
      fetch("image/storage-recovery", { signal }).then(
        readJSON<RecoveryStatus>
      ),
    ]);
    if (!signal?.aborted) {
      setCandidate(candidateValue);
      setStatus(recoveryValue);
    }
  }, []);

  useEffect(() => {
    const abort = new AbortController();
    void refresh(abort.signal).catch((reason: unknown) => {
      if (!abort.signal.aborted)
        setError(reason instanceof Error ? reason.message : String(reason));
    });
    return () => abort.abort();
  }, [refresh]);

  const group = candidate?.result?.group;
  const stackID = group?.match(/^stack:([1-9]\d*)$/)?.[1];
  const ready =
    !!candidate?.result?.verified &&
    !!candidate.result.backupVerified &&
    !!status?.available &&
    !!stackID;
  const busy = requesting || !!candidate?.busy || !!status?.busy;

  async function start() {
    if (!stackID) return;
    setRequesting(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("image/storage-recovery", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ stackID: Number(stackID), acknowledged }),
      });
      const job = await readJSON<{ jobID: number }>(response);
      setMessage(
        "Queued isolated reconstruction job #" +
          job.jobID +
          ". Check the job queue and refresh the result."
      );
      await refresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setRequesting(false);
    }
  }

  const result = status?.result;
  const verified =
    result?.verified &&
    result.byteExact &&
    result.complete &&
    !result.originalEvictionEnabled &&
    !result.archiveActivated &&
    !result.databaseModified;

  return (
    <section
      id="image-storage-recovery-task"
      aria-labelledby="image-storage-recovery-title"
    >
      <h2 id="image-storage-recovery-title">
        Reconstruct candidate into a separate folder
      </h2>
      <p>
        Verify the most recently created experimental candidate and recover its
        original file bytes into a new private directory. This does not replace
        the existing library, restore the database or free any space.
      </p>
      <Alert variant="warning">
        This is an isolated recovery proof, not the final Reconstruct Library
        feature. Originals remain active. The recovered copies require
        additional disk space and are never activated automatically.
      </Alert>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      {!ready && (
        <p className="text-muted">
          Create a verified candidate in this server session first. Only the
          latest candidate can be reconstructed here.
        </p>
      )}
      {ready && <p>Selected candidate: {group}</p>}
      <Form.Check
        id="storage-recovery-opt-in"
        type="checkbox"
        checked={acknowledged}
        disabled={busy}
        onChange={(event) => setAcknowledged(event.currentTarget.checked)}
        label="I understand this reconstructs copies only, without replacing original images or modifying the database."
      />
      <Button
        variant="secondary"
        disabled={!ready || !acknowledged || busy}
        onClick={() => void start()}
      >
        Reconstruct into private folder (opt-in)
      </Button>{" "}
      <Button
        variant="outline-secondary"
        disabled={requesting}
        onClick={() => {
          setError("");
          void refresh().catch((reason: unknown) =>
            setError(reason instanceof Error ? reason.message : String(reason))
          );
        }}
      >
        Refresh reconstruction
      </Button>
      {status?.busy && (
        <p role="status" className="small mt-2">
          Reconstructing the candidate in the job queue…
        </p>
      )}
      {verified && (
        <Alert variant="info" className="mt-3">
          Verified isolated recovery: {result.restoredFiles} files.
          <code className="text-break d-block">{result.outputPath}</code>
          Nothing was changed in the live image library.
        </Alert>
      )}
    </section>
  );
};
