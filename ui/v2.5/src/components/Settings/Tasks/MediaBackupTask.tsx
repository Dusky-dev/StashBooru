import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form, Table } from "react-bootstrap";

interface MediaBackupSnapshot {
  snapshot: string;
  files: number;
  logicalBytes: number;
}

interface MediaBackupStatus {
  destination: string;
  roots: string[];
  snapshots: MediaBackupSnapshot[];
  busy: boolean;
  available: boolean;
}

interface BackupJob {
  jobID: number;
}

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return (await response.json()) as T;
}

export const MediaBackupTask: React.FC = () => {
  const [status, setStatus] = useState<MediaBackupStatus>();
  const [destination, setDestination] = useState("");
  const [output, setOutput] = useState("");
  const [selectedSnapshot, setSelectedSnapshot] = useState("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [lastJob, setLastJob] = useState<number>();

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const response = await fetch("image/media-backup", { signal });
    const value = await readResponse<MediaBackupStatus>(response);
    if (signal?.aborted) return;
    setStatus(value);
    setDestination((previous) =>
      previous === "" ? value.destination : previous
    );
    setLoading(false);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal).catch((cause: unknown) => {
      if (controller.signal.aborted) return;
      setError(cause instanceof Error ? cause.message : String(cause));
      setLoading(false);
    });
    return () => controller.abort();
  }, [refresh]);

  async function act(
    action: "config" | "backup" | "verify" | "restore",
    snapshot?: string
  ) {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("image/media-backup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action,
          ...(action === "config" ? { destination } : {}),
          ...(snapshot ? { snapshot } : {}),
          ...(action === "restore" ? { output } : {}),
        }),
      });
      if (action === "config") {
        const result = await readResponse<{ destination: string }>(response);
        setDestination(result.destination);
        setMessage("Backup destination saved. No files were changed.");
      } else {
        const result = await readResponse<BackupJob>(response);
        setLastJob(result.jobID);
        setMessage(
          `Queued ${action} as task #${result.jobID}. Follow progress in the job queue above, then refresh snapshots.`
        );
      }
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }

  const disabled = busy || status?.busy || !status?.available;
  const savedDestination =
    !!status?.destination && destination === status.destination;

  return (
    <section id="media-backup-task" aria-labelledby="media-backup-heading">
      <h2 id="media-backup-heading">Original media backup</h2>
      <p>
        Back up the original files from your configured StashBooru media
        directories, independently of the database and filesystem blobs.
        Backups reuse unchanged file content, but each run rechecks sources.
        No original files are deleted or replaced.
      </p>
      <Alert variant="warning">
        <strong>Experimental backup workflow.</strong> Save backups on a
        separate physical device and test a recovery before using experimental
        image storage. This does not include the database, configuration,
        generated files, or conversion/upscaling restore caches. Back those up
        separately. Media backup is not an automatic schedule.
      </Alert>
      {loading && <p role="status">Loading media backup settings…</p>}
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      {status && !status.available && (
        <Alert variant="warning">
          Media backup script unavailable on this server. Install
          scripts/media_library_backup.py alongside the server, or set
          STASH_MEDIA_BACKUP_SCRIPT to its location.
        </Alert>
      )}
      <Form.Group>
        <Form.Label htmlFor="media-backup-destination">
          Backup destination (absolute server filesystem path)
        </Form.Label>
        <Form.Control
          id="media-backup-destination"
          type="text"
          value={destination}
          disabled={busy || !!status?.busy}
          placeholder="/mnt/backup-drive/stashbooru"
          onChange={(event) => {
            setDestination(event.currentTarget.value);
            setMessage("");
          }}
        />
        <Form.Text className="text-muted">
          Create this directory on the server first. It must not be inside
          your library directories, or contain them.
        </Form.Text>
      </Form.Group>
      <Button
        variant="secondary"
        disabled={busy || !!status?.busy || !destination.trim()}
        onClick={() => void act("config")}
      >
        Save backup destination
      </Button>
      <Button
        variant="primary"
        className="ml-2"
        disabled={disabled || !savedDestination || status?.roots?.length === 0}
        onClick={() => void act("backup")}
      >
        Back up media files
      </Button>
      <Button
        variant="secondary"
        className="ml-2"
        disabled={busy}
        onClick={() => {
          setError("");
          void refresh().catch((cause: unknown) =>
            setError(cause instanceof Error ? cause.message : String(cause))
          );
        }}
      >
        Refresh
      </Button>
      {lastJob !== undefined && (
        <p className="small text-muted mt-2">
          Last queued job: #{lastJob}. Check the Tasks job queue above for its
          result. Refresh to list new snapshots.
        </p>
      )}
      <p className="small text-muted mt-3">
        Configured roots: {status?.roots?.length ?? 0}
        {status?.busy ? " · Backup operation running" : ""}
      </p>
      {!!status?.snapshots?.length && (
        <>
          <h3>Saved media snapshots</h3>
          <Table responsive size="sm">
            <thead>
              <tr>
                <th>Snapshot</th>
                <th>Files</th>
                <th>Logical size</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {status.snapshots.map((snapshot) => (
                <tr key={snapshot.snapshot}>
                  <td className="text-break">{snapshot.snapshot}</td>
                  <td>{snapshot.files.toLocaleString()}</td>
                  <td>
                    {(snapshot.logicalBytes / 1024 ** 3).toFixed(2)} GiB
                  </td>
                  <td>
                    <Button
                      size="sm"
                      variant="secondary"
                      disabled={disabled}
                      onClick={() => void act("verify", snapshot.snapshot)}
                    >
                      Verify
                    </Button>{" "}
                    <Button
                      size="sm"
                      variant="outline-secondary"
                      disabled={disabled}
                      onClick={() => {
                        setSelectedSnapshot(snapshot.snapshot);
                        setAcknowledged(false);
                        setOutput("");
                        setError("");
                      }}
                    >
                      Restore to directory…
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
        </>
      )}
      {selectedSnapshot && (
        <div className="mt-3">
          <Alert variant="warning">
            Recovery writes original files to a <strong>separate</strong>
            directory, never over the live library or existing files. Check
            that there is enough space. Restore the corresponding database and
            other app data separately if needed.
          </Alert>
          <h3>Restore {selectedSnapshot}</h3>
          <Form.Group>
            <Form.Label htmlFor="media-backup-output">
              Restore to directory (absolute server path)
            </Form.Label>
            <Form.Control
              id="media-backup-output"
              value={output}
              disabled={disabled}
              onChange={(event) => {
                setOutput(event.currentTarget.value);
                setAcknowledged(false);
              }}
              placeholder="/mnt/recovery/snapshot"
            />
          </Form.Group>
          <Form.Check
            id="media-backup-confirm"
            type="checkbox"
            checked={acknowledged}
            disabled={disabled}
            onChange={(event) => setAcknowledged(event.currentTarget.checked)}
            label="I understand this creates recovered files in a separate directory."
          />
          <Button
            variant="secondary"
            disabled={disabled || !acknowledged || !output.trim()}
            onClick={() => void act("restore", selectedSnapshot)}
          >
            Reconstruct snapshot files
          </Button>{" "}
          <Button
            variant="outline-secondary"
            disabled={disabled}
            onClick={() => setSelectedSnapshot("")}
          >
            Cancel
          </Button>
        </div>
      )}
    </section>
  );
};
