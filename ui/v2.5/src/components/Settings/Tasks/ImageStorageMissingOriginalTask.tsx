import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";

interface SavedCandidate {
  id: string;
  group: string;
  method: string;
  snapshot: string;
}
interface SavedCandidates {
  candidates?: SavedCandidate[];
}
interface RecoveryProof {
  candidateID: string;
  snapshot: string;
  verified: boolean;
  readOnlyAgainstLibrary: boolean;
  originalSourcesNotRead: boolean;
  archiveSHA256Verified: boolean;
  mediaBackupObjectsVerified: boolean;
  backupContentDigestsCovered: boolean;
  sourceToBackupPathMappingVerified: boolean;
  databaseRecoveryVerified: boolean;
  nativeMediaServingVerified: boolean;
  crashSafeLiveRollbackVerified: boolean;
  canActivate: boolean;
  originalEvictionEnabled: boolean;
  restoredFiles: number;
  outputPath: string;
}
interface RecoveryJournal {
  candidateID: string;
  outputPath: string;
  state: string;
  updatedUnix: number;
  restoredFiles: number;
  requiresReaudit: boolean;
  canActivate: boolean;
  evictionEnabled: boolean;
}
interface RecoveryStatus {
  available: boolean;
  busy: boolean;
  result?: RecoveryProof;
  journal?: RecoveryJournal;
  journalError?: string;
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return (await response.json()) as T;
}

export const ImageStorageMissingOriginalTask: React.FC = () => {
  const [candidates, setCandidates] = useState<SavedCandidate[]>([]);
  const [candidateID, setCandidateID] = useState("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [status, setStatus] = useState<RecoveryStatus>();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const [saved, recovery] = await Promise.all([
      fetch("image/storage-candidate", { signal }).then(
        readJSON<SavedCandidates>
      ),
      fetch("image/storage-missing-original-rehearsal", { signal }).then(
        readJSON<RecoveryStatus>
      ),
    ]);
    if (signal?.aborted) return;
    const items = saved.candidates ?? [];
    setCandidates(items);
    setStatus(recovery);
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

  const selection = candidates.find((item) => item.id === candidateID);
  const busy = submitting || !!status?.busy;
  const result = status?.result;
  const verified =
    result?.candidateID === candidateID &&
    result.verified &&
    result.readOnlyAgainstLibrary &&
    result.originalSourcesNotRead &&
    result.archiveSHA256Verified &&
    result.mediaBackupObjectsVerified &&
    result.backupContentDigestsCovered &&
    !result.sourceToBackupPathMappingVerified &&
    !result.databaseRecoveryVerified &&
    !result.nativeMediaServingVerified &&
    !result.crashSafeLiveRollbackVerified &&
    !result.canActivate &&
    !result.originalEvictionEnabled;

  async function start() {
    if (!selection || !acknowledged) return;
    setSubmitting(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("image/storage-missing-original-rehearsal", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ candidateID, acknowledged }),
      });
      const job = await readJSON<{ jobID: number }>(response);
      setMessage(
        "Private missing-original rehearsal queued as job #" +
          job.jobID +
          ". Refresh after completion."
      );
      setAcknowledged(false);
      await refresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section aria-labelledby="image-missing-original-title">
      <h2 id="image-missing-original-title">
        Rehearse recovery when originals are missing
      </h2>
      <p>
        Verify one saved P12/P13 archive and its media backup without opening
        any active original images. Recover copies into a new private scratch
        directory, even if the original source files no longer exist.
      </p>
      <Alert variant="warning">
        Experimental scratch recovery only. It consumes additional storage, does
        not restore the database or original file paths, and does not permit
        packed serving or original deletion. Interrupted outputs may require
        manual cleanup.
      </Alert>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      <Form.Group>
        <Form.Label htmlFor="missing-original-candidate">
          Saved candidate
        </Form.Label>
        <Form.Control
          as="select"
          id="missing-original-candidate"
          value={candidateID}
          disabled={busy}
          onChange={(event) => {
            setCandidateID(event.currentTarget.value);
            setAcknowledged(false);
          }}
        >
          {!candidateID && <option value="">Select a saved candidate</option>}
          {candidates.map((candidate) => (
            <option key={candidate.id} value={candidate.id}>
              {candidate.group} — {candidate.method} — {candidate.snapshot}
            </option>
          ))}
        </Form.Control>
      </Form.Group>
      <Form.Check
        id="missing-original-acknowledged"
        type="checkbox"
        checked={acknowledged}
        disabled={busy}
        onChange={(value) => setAcknowledged(value.currentTarget.checked)}
        label="I understand this only creates private recovery copies, with no database or live-library restore."
      />
      <Button
        variant="secondary"
        disabled={busy || !selection || !acknowledged || !status?.available}
        onClick={() => void start()}
      >
        Rehearse into private scratch (opt-in)
      </Button>{" "}
      <Button
        variant="outline-secondary"
        disabled={submitting}
        onClick={() => {
          setError("");
          void refresh().catch((reason: unknown) =>
            setError(reason instanceof Error ? reason.message : String(reason))
          );
        }}
      >
        Refresh recovery
      </Button>
      {status?.busy && (
        <p role="status" className="small mt-2">
          Reconstructing from saved archive and verifying backup objects…
        </p>
      )}
      {status?.journalError && (
        <Alert variant="danger" role="alert" className="mt-3">
          {status.journalError}. No saved recovery state is trusted.
        </Alert>
      )}
      {status?.journal && (
        <Alert variant="warning" className="mt-3" role="status">
          Latest persisted recovery: <strong>{status.journal.state}</strong> for{" "}
          {status.journal.candidateID}.{" "}
          {status.journal.state === "interrupted_or_unconfirmed"
            ? "The previous job may have stopped mid-extraction. Inspect scratch output before any manual action."
            : status.journal.state === "scratch_complete"
              ? "A previous run reported successful scratch reconstruction; the saved result has not been independently re-verified after restart."
              : "This is a scratch-only job record, not live restoration."}
          <code className="d-block text-break">
            {status.journal.outputPath}
          </code>
          Reaudit is always required. Live activation and original-file deletion
          remain disabled.
        </Alert>
      )}
      {verified && (
        <Alert variant="info" className="mt-3">
          Reconstructed {result.restoredFiles} images in isolated scratch.
          <code className="d-block text-break">{result.outputPath}</code>
          Original paths, database entries and native serving are unchanged.
        </Alert>
      )}
    </section>
  );
};
