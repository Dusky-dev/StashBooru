import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";

interface CandidateStatus {
  candidates?: {
    id: string;
    group: string;
    method: string;
    snapshot: string;
    archiveBytes: number;
  }[];
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
  const [selectedID, setSelectedID] = useState("");
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
      setSelectedID((previous) =>
        candidateValue.candidates?.some((item) => item.id === previous)
          ? previous
          : (candidateValue.candidates?.[0]?.id ?? "")
      );
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

  const selected = candidate?.candidates?.find((item) => item.id === selectedID);
  const stackID = selected?.group.match(/^stack:([1-9]\d*)$/)?.[1];
  const ready = !!selected && !!status?.available && !!stackID;
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
        body: JSON.stringify({
          stackID: Number(stackID),
          candidateID: selectedID,
          acknowledged,
        }),
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
        Select a saved experimental candidate, including those created before a
        server restart, and reconstruct a separate copy of its original bytes.
        No active library files are replaced.
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
          No saved candidate is available. Create and verify one first.
        </p>
      )}
      <Form.Group>
        <Form.Label htmlFor="saved-image-candidate">Saved candidate</Form.Label>
        <Form.Control
          as="select"
          id="saved-image-candidate"
          value={selectedID}
          disabled={busy}
          onChange={(event) => {
            setSelectedID(event.currentTarget.value);
            setAcknowledged(false);
          }}
        >
          {!selectedID && <option value="">Select a saved candidate</option>}
          {candidate?.candidates?.map((item) => (
            <option key={item.id} value={item.id}>
              {item.group} — {item.method} — {item.snapshot}
            </option>
          ))}
        </Form.Control>
        <Form.Text className="text-muted">
          Saved candidates are rechecked against archive SHA-256 before use.
        </Form.Text>
      </Form.Group>
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
