import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";

interface BackupStatus {
  available: boolean;
  busy: boolean;
  snapshots: { snapshot: string; files: number }[];
}
interface CandidateReport {
  verified: boolean;
  backupVerified: boolean;
  sourcesRetained: boolean;
  nativePackingCandidateOnly: boolean;
  archiveActivated: boolean;
  originalEvictionEnabled: boolean;
  nativeServingVerified: boolean;
  databaseBackupVerified: boolean;
  group: string;
  method: string;
  mediaBackupSnapshot: string;
  archivePath: string;
  sourceFiles: number;
  sourceBytes: number;
  archiveBytes: number;
}
interface CandidateStatus {
  available: boolean;
  busy: boolean;
  jobID?: number;
  result?: CandidateReport;
}

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok)
    throw new Error((await response.text()) || response.statusText);
  return (await response.json()) as T;
}

const formatSize = (size: number) => (size / 1024 ** 2).toFixed(2) + " MiB";

export const ImageStorageCandidateTask: React.FC = () => {
  const [backup, setBackup] = useState<BackupStatus>();
  const [status, setStatus] = useState<CandidateStatus>();
  const [stackID, setStackID] = useState("");
  const [snapshot, setSnapshot] = useState("");
  const [method, setMethod] = useState("delta");
  const [acknowledged, setAcknowledged] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const [pack, media] = await Promise.all([
      fetch("image/storage-candidate", { signal }).then(
        readResponse<CandidateStatus>
      ),
      fetch("image/media-backup", { signal }).then(
        readResponse<BackupStatus>
      ),
    ]);
    if (signal?.aborted) return;
    if (typeof pack?.available !== "boolean" || !Array.isArray(media?.snapshots))
      throw new Error("Native candidate packing API is unavailable.");
    setStatus(pack);
    setBackup(media);
    setSnapshot((previous) =>
      media.snapshots.some((entry) => entry.snapshot === previous)
        ? previous
        : (media.snapshots[0]?.snapshot ?? "")
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

  async function start() {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("image/storage-candidate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          stackID: Number(stackID),
          snapshot,
          method,
          acknowledged,
        }),
      });
      const result = await readResponse<{ jobID: number }>(response);
      setMessage("Queued experimental candidate job #" + result.jobID +
        ". Follow the job queue, then refresh this report.");
      await refresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(false);
    }
  }

  const validID =
    /^[1-9]\d*$/.test(stackID) && Number.isSafeInteger(Number(stackID));
  const disabled = busy || !!status?.busy || !!backup?.busy;
  const ready = !!status?.available && !!backup?.available;
  const report = status?.result;
  const verified =
    report?.verified &&
    report.backupVerified &&
    report.sourcesRetained &&
    report.nativePackingCandidateOnly &&
    !report.archiveActivated &&
    !report.originalEvictionEnabled &&
    !report.nativeServingVerified &&
    !report.databaseBackupVerified;

  return (
    <section
      id="image-storage-candidate-task"
      aria-labelledby="image-storage-candidate-title"
    >
      <h2 id="image-storage-candidate-title">
        Experimental image candidate packing
      </h2>
      <p>
        Publish a byte-exact P12 delta or P13 shared-tiles candidate for one
        image-only Visual Stack after verifying its media backup. The snapshot
        and original image hashes are checked again before publication.
      </p>
      <Alert variant="warning">
        <strong>No disk savings yet.</strong> All original images stay active.
        This creates an additional archive under the server configuration
        directory, using more disk space. It cannot serve, replace, or delete
        images. Back up the database and configuration separately.
      </Alert>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      {status && !status.available && (
        <Alert variant="warning">
          Native candidate scripts are missing from the server.
        </Alert>
      )}
      <Form.Group>
        <Form.Label htmlFor="candidate-stack-id">Visual Stack ID</Form.Label>
        <Form.Control
          id="candidate-stack-id"
          inputMode="numeric"
          value={stackID}
          placeholder="17"
          disabled={disabled}
          onChange={(event) => {
            setStackID(event.currentTarget.value);
            setAcknowledged(false);
          }}
        />
        <Form.Text className="text-muted">
          An existing Visual Stack of 2–16 images.
        </Form.Text>
      </Form.Group>
      <Form.Group>
        <Form.Label htmlFor="candidate-backup-snapshot">
          Original-media backup snapshot
        </Form.Label>
        <Form.Control
          id="candidate-backup-snapshot"
          as="select"
          value={snapshot}
          disabled={disabled}
          onChange={(event) => {
            setSnapshot(event.currentTarget.value);
            setAcknowledged(false);
          }}
        >
          {!snapshot && <option value="">Select a backup</option>}
          {backup?.snapshots.map((entry) => (
            <option value={entry.snapshot} key={entry.snapshot}>
              {entry.snapshot} ({entry.files} files)
            </option>
          ))}
        </Form.Control>
        {!backup?.snapshots.length && (
          <Form.Text className="text-muted">
            Create a snapshot in Backup &amp; maintenance first.
          </Form.Text>
        )}
      </Form.Group>
      <Form.Group>
        <Form.Label htmlFor="candidate-method">Method</Form.Label>
        <Form.Control
          id="candidate-method"
          as="select"
          value={method}
          disabled={disabled}
          onChange={(event) => {
            setMethod(event.currentTarget.value);
            setAcknowledged(false);
          }}
        >
          <option value="delta">P12 — delta</option>
          <option value="shared-tiles">P13 — shared tiles</option>
        </Form.Control>
      </Form.Group>
      <Form.Check
        id="candidate-opt-in"
        type="checkbox"
        checked={acknowledged}
        disabled={disabled}
        onChange={(event) => setAcknowledged(event.currentTarget.checked)}
        label="I understand this creates additional experimental files, not actual disk savings or a full backup."
      />
      <Button
        variant="secondary"
        disabled={disabled || !ready || !validID || !snapshot || !acknowledged}
        onClick={() => void start()}
      >
        Create verified candidate (opt-in)
      </Button>{" "}
      <Button
        variant="outline-secondary"
        disabled={busy}
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
          Creating the candidate in the job queue…
        </p>
      )}
      {verified && (
        <div className="mt-3">
          <h3>Verified candidate: {report.group}</h3>
          <p>
            {report.sourceFiles} originals ({formatSize(report.sourceBytes)})
            plus one {report.method} archive ({formatSize(report.archiveBytes)}).
          </p>
          <p className="small text-muted">
            Media snapshot: {report.mediaBackupSnapshot}. Candidate archive:
            <code className="text-break d-block">{report.archivePath}</code>
          </p>
          <Alert variant="info">
            The candidate is not activated. Offline proof readers can
            reconstruct a separate copy, but native serving and deletion
            remain disabled.
          </Alert>
        </div>
      )}
    </section>
  );
};
