import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form, Table } from "react-bootstrap";

interface MethodEstimate {
  method: string;
  layout: string;
  eligible: boolean;
  originalBytes: number;
  packedBytes: number;
  potentialSavedBytes: number;
  reason: string;
}
interface GroupEstimate {
  group: string;
  fidelity: string;
  members: number;
  sourceBytes: number;
  recommendedMethod: string;
  potentialPackedBytes: number;
  potentialSavedBytes: number;
  physicalBytesIfOriginalsRetained: number;
  methods: MethodEstimate[];
  methodErrors: { method: string; error: string }[];
}
interface StorageEstimateReport {
  readOnly: boolean;
  sourcesRetained: boolean;
  sourceBytes: number;
  potentialSavedBytes: number;
  hypotheticalBytesAfterFullActivation: number;
  groupsFailed: number;
  results: GroupEstimate[];
}
interface EstimationStatus {
  available: boolean;
  busy: boolean;
  jobID?: number;
  result?: StorageEstimateReport;
}

const formatSize = (size: number) => {
  if (!Number.isFinite(size) || size < 0) return "Unknown";
  return `${(size / 1024 ** 2).toFixed(2)} MiB`;
};

export const ImageStorageEstimateTask: React.FC = () => {
  const [status, setStatus] = useState<EstimationStatus>();
  const [stackID, setStackID] = useState("");
  const [fidelity, setFidelity] = useState("byte-exact");
  const [acknowledged, setAcknowledged] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const response = await fetch("image/storage-estimate", { signal });
    if (!response.ok) throw new Error(await response.text());
    const data = (await response.json()) as EstimationStatus;
    if (signal?.aborted) return;
    if (typeof data?.available !== "boolean") {
      throw new Error("Storage estimation API is unavailable.");
    }
    setStatus(data);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal).catch((cause: unknown) => {
      if (!controller.signal.aborted)
        setError(cause instanceof Error ? cause.message : String(cause));
    });
    return () => controller.abort();
  }, [refresh]);

  async function start() {
    setBusy(true);
    setMessage("");
    setError("");
    try {
      const response = await fetch("image/storage-estimate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ stackID: Number(stackID), fidelity }),
      });
      if (!response.ok) throw new Error(await response.text());
      const result = (await response.json()) as { jobID: number };
      setMessage(
        `Queued read-only storage estimate as task #${result.jobID}. Check the job queue, then refresh the report.`
      );
      await refresh();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }

  const validID =
    /^[1-9]\d*$/.test(stackID) && Number.isSafeInteger(Number(stackID));
  const disabled = busy || !!status?.busy || !status?.available;
  const result = status?.result;
  const group = result?.results?.[0];

  return (
    <section
      id="image-storage-estimate-task"
      aria-labelledby="image-storage-estimate-heading"
    >
      <h2 id="image-storage-estimate-heading">
        Experimental image storage estimate
      </h2>
      <p>
        Compare P12 delta storage against P13 shared image blocks for one
        existing image-only Visual Stack (2–16 images). This performs a
        read-only estimate using the current primary files and does not create
        packs, modify the catalogue or remove originals.
      </p>
      <Alert variant="warning">
        <strong>No real disk savings yet.</strong> Estimated compression only
        describes potential future storage. Until packed-file serving, verified
        backups and library reconstruction are completed, retaining originals
        alongside packs would increase disk usage. Byte-exact mode is
        recommended; pixel-exact may change image container bytes.
      </Alert>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      {status && !status.available && (
        <Alert variant="warning">
          The estimator script is not installed on the server. Install
          scripts/image_storage_estimate.py alongside its P12/P13 scripts or set
          STASH_IMAGE_STORAGE_ESTIMATE_SCRIPT.
        </Alert>
      )}
      <Form.Group>
        <Form.Label htmlFor="image-storage-estimate-id">
          Visual Stack ID
        </Form.Label>
        <Form.Control
          id="image-storage-estimate-id"
          type="number"
          min={1}
          value={stackID}
          onChange={(event) => setStackID(event.currentTarget.value)}
          placeholder="17"
          disabled={disabled}
        />
        <Form.Text className="text-muted">
          Enter the numeric ID of an existing image-only Visual Stack. This
          first version estimates one stack at a time.
        </Form.Text>
      </Form.Group>
      <Form.Group>
        <Form.Label htmlFor="image-storage-estimate-fidelity">
          Fidelity
        </Form.Label>
        <Form.Control
          id="image-storage-estimate-fidelity"
          as="select"
          value={fidelity}
          onChange={(event) => {
            setFidelity(event.currentTarget.value);
            setAcknowledged(false);
          }}
          disabled={disabled}
        >
          <option value="byte-exact">Byte-exact (recommended)</option>
          <option value="pixel-exact">Pixel-exact (experimental)</option>
        </Form.Control>
      </Form.Group>
      <Form.Check
        id="image-storage-estimate-acknowledge"
        type="checkbox"
        checked={acknowledged}
        onChange={(event) => setAcknowledged(event.currentTarget.checked)}
        label="I understand this is an experimental estimate, not storage optimization or a backup."
        disabled={disabled}
      />
      <Button
        variant="secondary"
        disabled={disabled || !validID || !acknowledged}
        onClick={() => void start()}
      >
        Estimate savings (read-only)
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
        Refresh report
      </Button>
      {status?.busy && (
        <p className="small mt-2" role="status">
          Estimating selected Visual Stack in the task queue…
        </p>
      )}
      {group && result?.readOnly && result.sourcesRetained && (
        <div className="mt-3">
          <h3>Most recent Visual Stack estimate: {group.group}</h3>
          <p>
            {group.members} images · {group.fidelity} · Original files:{" "}
            {formatSize(group.sourceBytes)} · Hypothetical savings:{" "}
            {formatSize(group.potentialSavedBytes)} · Best method:{" "}
            {group.recommendedMethod}.
          </p>
          <Table responsive size="sm">
            <thead>
              <tr>
                <th>Method</th>
                <th>Candidate bytes</th>
                <th>Potential saved</th>
                <th>Meets limits?</th>
              </tr>
            </thead>
            <tbody>
              {group.methods.map((method) => (
                <tr key={method.method}>
                  <td>
                    {method.method} ({method.layout})
                  </td>
                  <td>{formatSize(method.packedBytes)}</td>
                  <td>{formatSize(method.potentialSavedBytes)}</td>
                  <td>{method.eligible ? "Yes" : "No"}</td>
                </tr>
              ))}
            </tbody>
          </Table>
          {group.methodErrors?.map((failure) => (
            <Alert key={failure.method} variant="warning">
              {failure.method}: {failure.error}
            </Alert>
          ))}
          <p className="small text-muted">
            Potential figures exclude future caching and storage activation
            overhead. Source files remain unchanged; this task creates no
            compressed archives.
          </p>
        </div>
      )}
    </section>
  );
};
