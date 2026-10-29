import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form, Table } from "react-bootstrap";

interface Snapshot {
  snapshot: string;
  files: number;
}

interface MediaBackupStatus {
  destination: string;
  snapshots: Snapshot[];
  busy: boolean;
  available: boolean;
}

interface PreflightResult {
  readOnly: boolean;
  verified: boolean;
  mediaBackupVerified: boolean;
  sourcesRetained: boolean;
  snapshot: string;
  group: string;
  selectedImagesCovered: number;
  snapshotFilesVerified: number;
  snapshotObjectsVerified: number;
  imageSHA256: Record<string, string>;
  nativePackingEnabled: boolean;
  originalEvictionEnabled: boolean;
}

interface PreflightStatus {
  available: boolean;
  busy: boolean;
  jobID?: number;
  result?: PreflightResult;
}

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok)
    throw new Error((await response.text()) || response.statusText);
  return (await response.json()) as T;
}

export const ImageStorageBackupPreflightTask: React.FC = () => {
  const [backup, setBackup] = useState<MediaBackupStatus>();
  const [status, setStatus] = useState<PreflightStatus>();
  const [stackID, setStackID] = useState("");
  const [snapshot, setSnapshot] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const [preflight, media] = await Promise.all([
      fetch("image/storage-backup-preflight", { signal }).then(
        readResponse<PreflightStatus>
      ),
      fetch("image/media-backup", { signal }).then(
        readResponse<MediaBackupStatus>
      ),
    ]);
    if (signal?.aborted) return;
    if (
      typeof preflight?.available !== "boolean" ||
      !Array.isArray(media?.snapshots)
    ) {
      throw new Error("Native backup preflight API is unavailable.");
    }
    setStatus(preflight);
    setBackup(media);
    setSnapshot((previous) =>
      media.snapshots.some((item) => item.snapshot === previous)
        ? previous
        : (media.snapshots[0]?.snapshot ?? "")
    );
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal).catch((cause: unknown) => {
      if (!controller.signal.aborted)
        setError(cause instanceof Error ? cause.message : String(cause));
    });
    return () => controller.abort();
  }, [refresh]);

  const validID =
    /^[1-9]\d*$/.test(stackID) && Number.isSafeInteger(Number(stackID));
  const disabled = busy || !!status?.busy || !!backup?.busy;
  const available = !!status?.available && !!backup?.available;
  const report = status?.result;
  const reportValid =
    report?.readOnly &&
    report.verified &&
    report.mediaBackupVerified &&
    report.sourcesRetained &&
    !report.nativePackingEnabled &&
    !report.originalEvictionEnabled;

  async function start() {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("image/storage-backup-preflight", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ stackID: Number(stackID), snapshot }),
      });
      const result = await readResponse<{ jobID: number }>(response);
      setMessage(
        `Queued read-only backup coverage check as job #${result.jobID}. Refresh after it finishes.`
      );
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section
      id="image-storage-backup-preflight-task"
      aria-labelledby="image-storage-backup-preflight-heading"
    >
      <h2 id="image-storage-backup-preflight-heading">
        Verify image storage backup coverage
      </h2>
      <p>
        Before experimenting with packing, confirm that each image in an
        existing Visual Stack still matches a verified original-media snapshot.
        This is read-only: it creates no packs, modifies no images and cannot
        activate or remove media.
      </p>
      <Alert variant="warning">
        A verified media snapshot does not back up the database or configuration
        and does not prove native serving, rollback or recovery. Experimental
        image optimization remains disabled.
      </Alert>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      {status && !status.available && (
        <Alert variant="warning">
          Install scripts/image_storage_backup_preflight.py and
          scripts/media_library_backup.py on the server.
        </Alert>
      )}
      <Form.Group>
        <Form.Label htmlFor="storage-preflight-id">Visual Stack ID</Form.Label>
        <Form.Control
          id="storage-preflight-id"
          type="text"
          inputMode="numeric"
          placeholder="17"
          value={stackID}
          disabled={disabled}
          onChange={(event) => setStackID(event.currentTarget.value)}
        />
        <Form.Text className="text-muted">
          One existing image-only Visual Stack containing 2–16 images.
        </Form.Text>
      </Form.Group>
      <Form.Group>
        <Form.Label htmlFor="storage-preflight-snapshot">
          Verified media snapshot to check
        </Form.Label>
        <Form.Control
          id="storage-preflight-snapshot"
          as="select"
          value={snapshot}
          disabled={disabled}
          onChange={(event) => setSnapshot(event.currentTarget.value)}
        >
          {!snapshot && <option value="">Select a snapshot</option>}
          {backup?.snapshots.map((item) => (
            <option value={item.snapshot} key={item.snapshot}>
              {item.snapshot} ({item.files} files)
            </option>
          ))}
        </Form.Control>
        {!backup?.snapshots.length && (
          <Form.Text className="text-muted">
            Create a media snapshot in Backup &amp; maintenance first.
          </Form.Text>
        )}
      </Form.Group>
      <Button
        variant="secondary"
        disabled={disabled || !available || !validID || !snapshot}
        onClick={() => void start()}
      >
        Verify backup coverage (read-only)
      </Button>{" "}
      <Button
        variant="outline-secondary"
        disabled={busy}
        onClick={() => {
          setError("");
          void refresh().catch((cause: unknown) =>
            setError(cause instanceof Error ? cause.message : String(cause))
          );
        }}
      >
        Refresh result
      </Button>
      {status?.busy && (
        <p className="small mt-2" role="status">
          Verifying the complete media snapshot and selected images in the job
          queue…
        </p>
      )}
      {reportValid && (
        <div className="mt-3">
          <h3>Verified backup coverage: {report.group}</h3>
          <p>
            Snapshot {report.snapshot}: {report.selectedImagesCovered} selected
            images matched. Verified {report.snapshotObjectsVerified} backup
            objects across {report.snapshotFilesVerified} snapshot files.
          </p>
          <Table responsive size="sm">
            <thead>
              <tr>
                <th>Image</th>
                <th>Original SHA-256 matching the backup</th>
              </tr>
            </thead>
            <tbody>
              {Object.entries(report.imageSHA256).map(([id, hash]) => (
                <tr key={id}>
                  <td>{id}</td>
                  <td className="text-break">
                    <code>{hash}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
          <p className="small text-muted">
            This is a point-in-time check. All original files remain active and
            no pack has been written. Recheck immediately before any future
            storage experiment.
          </p>
        </div>
      )}
    </section>
  );
};
