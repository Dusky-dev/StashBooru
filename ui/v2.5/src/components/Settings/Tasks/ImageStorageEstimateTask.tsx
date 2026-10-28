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
  physicalBytesBeforeEvictionWithAllCandidatePacks: number;
  groupsChecked: number;
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
  const [stackIDsText, setStackIDsText] = useState("");
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
        body: JSON.stringify({ stackIDs: parsedIDs, fidelity }),
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

  const tokens = stackIDsText.trim().split(/[\s,;]+/);
  const parsedIDs = tokens.map(Number);
  const validIDs =
    stackIDsText.trim().length > 0 &&
    tokens.length <= 8 &&
    tokens.every((token) => /^[1-9]\d*$/.test(token)) &&
    parsedIDs.every((id) => Number.isSafeInteger(id)) &&
    new Set(parsedIDs).size === parsedIDs.length;
  const disabled = busy || !!status?.busy || !status?.available;
  const result = status?.result;
  const groups = result?.results ?? [];

  return (
    <section
      id="image-storage-estimate-task"
      aria-labelledby="image-storage-estimate-heading"
    >
      <h2 id="image-storage-estimate-heading">
        Experimental image storage estimate
      </h2>
      <p>
        Compare P12 delta storage against P13 shared image blocks for up to
        eight existing image-only Visual Stacks (2–16 images each). This
        performs read-only estimates using current primary files without
        creating packs, modifying the catalogue or removing originals.
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
          Visual Stack IDs
        </Form.Label>
        <Form.Control
          id="image-storage-estimate-id"
          type="text"
          value={stackIDsText}
          onChange={(event) => setStackIDsText(event.currentTarget.value)}
          placeholder="17, 21, 32"
          disabled={disabled}
        />
        <Form.Text className="text-muted">
          Enter up to eight distinct numeric IDs separated by commas or spaces.
          Every selected stack must contain 2–16 image files.
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
        disabled={disabled || !validIDs || !acknowledged}
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
          Estimating selected Visual Stacks in the task queue…
        </p>
      )}
      {result?.readOnly && result.sourcesRetained && groups.length > 0 && (
        <div className="mt-3">
          <h3>Most recent storage estimate: {groups.length} stacks</h3>
          <p>
            Original files: {formatSize(result.sourceBytes)} · Hypothetical
            savings: {formatSize(result.potentialSavedBytes)} · Potential
            footprint after full activation:{" "}
            {formatSize(result.hypotheticalBytesAfterFullActivation)}.
          </p>
          <Alert variant="info">
            Originals were retained. Storing the candidate packs alongside them
            would instead require about{" "}
            {formatSize(
              result.physicalBytesBeforeEvictionWithAllCandidatePacks
            )}
            .
          </Alert>
          <Table responsive size="sm">
            <thead>
              <tr>
                <th>Visual Stack</th>
                <th>Images</th>
                <th>Originals</th>
                <th>Best method</th>
                <th>Potential saved</th>
              </tr>
            </thead>
            <tbody>
              {groups.map((group) => (
                <tr key={group.group}>
                  <td>{group.group}</td>
                  <td>{group.members}</td>
                  <td>{formatSize(group.sourceBytes)}</td>
                  <td>{group.recommendedMethod}</td>
                  <td>{formatSize(group.potentialSavedBytes)}</td>
                </tr>
              ))}
            </tbody>
          </Table>
          {groups.map((group) => (
            <div key={group.group} className="mt-3">
              <h4>
                {group.group} · {group.fidelity}
              </h4>
              <Table responsive size="sm">
                <thead>
                  <tr>
                    <th>Method</th>
                    <th>Candidate size</th>
                    <th>Potential saved</th>
                    <th>Eligible?</th>
                  </tr>
                </thead>
                <tbody>
                  {group.methods?.map((method) => (
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
            </div>
          ))}
          <p className="small text-muted">
            Figures exclude live storage integration and caching overhead. No
            source files were modified and no storage packs were created.
          </p>
        </div>
      )}
    </section>
  );
};
