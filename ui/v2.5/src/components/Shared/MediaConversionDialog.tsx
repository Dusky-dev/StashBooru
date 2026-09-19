import React, { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Button,
  Col,
  Form,
  Modal,
  ProgressBar,
  Row,
  Spinner,
  Table,
} from "react-bootstrap";

interface Target {
  kind: "image" | "scene";
  id: number;
}
interface Format {
  id: string;
  label: string;
  extension: string;
  family: string;
  cpu: string[];
  gpu: string[];
  controls: string[];
  available: boolean;
}
interface Options {
  format: string;
  hardware: string;
  quality: number;
  effort: number;
  distance: number;
  lossless: boolean;
  allowLarger: boolean;
  dropAudio: boolean;
  allowAlphaLoss: boolean;
}
interface FileSnapshot {
  basename: string;
  path: string;
  size: number;
  fingerprints: { Type: string; Fingerprint: string | number }[];
}
interface Conversion {
  id: string;
  status: string;
  createdAt: string;
  cached: boolean;
  error?: string;
  before: { image?: FileSnapshot; video?: FileSnapshot };
  after: { image?: FileSnapshot; video?: FileSnapshot };
  result: { encoder: string; seconds: number };
  backend: string;
}
interface Stats {
  converted: number;
  savedBytes: number;
  averageSavedBytes: number;
  savedPercent: number;
  cacheBytes: number;
  netSavedBytes: number;
  largerFiles: number;
}
interface Job {
  jobID: number;
  status: string;
  total: number;
  error?: string;
  items: { target: Target; recordID?: string; error?: string }[];
}
interface State {
  config: { cacheLimitBytes: number };
  stats: Stats;
  batchStats: Stats;
  history: Conversion[];
  historyTotal: number;
  job?: Job;
}
const endpoint = "image/converter";
const GiB = 1024 ** 3;

async function response<T>(r: Response): Promise<T> {
  if (!r.ok) throw new Error((await r.text()) || r.statusText);
  return r.json() as Promise<T>;
}
function bytes(n: number) {
  const unit = Math.abs(n) >= GiB ? GiB : 1024 ** 2;
  return `${(n / unit).toLocaleString(undefined, { maximumFractionDigits: 2 })} ${unit === GiB ? "GiB" : "MiB"}`;
}
function StatsView({ value }: { value: Stats }) {
  return (
    <Row className="mb-3">
      <Col>
        <strong>{bytes(value.savedBytes)}</strong>
        <div>Conversion savings ({value.savedPercent.toFixed(1)}%)</div>
      </Col>
      <Col>
        <strong>{bytes(value.averageSavedBytes)}</strong>
        <div>Average per file</div>
      </Col>
      <Col>
        <strong>{bytes(value.cacheBytes)}</strong>
        <div>Restorable originals</div>
      </Col>
      <Col>
        <strong>{bytes(value.netSavedBytes)}</strong>
        <div>Net media storage savings</div>
      </Col>
    </Row>
  );
}

export const MediaConversionDialog: React.FC<{
  kind: "image" | "scene";
  selectedIds: string[];
  onHide: () => void;
}> = ({ kind, selectedIds, onHide }) => {
  const [backend, setBackend] = useState("local");
  const [formats, setFormats] = useState<Format[]>([]);
  const [loadingCapabilities, setLoadingCapabilities] = useState(false);
  const [capabilityError, setCapabilityError] = useState("");
  const [state, setState] = useState<State>();
  const [historyOffset, setHistoryOffset] = useState(0);
  const [jobID, setJobID] = useState<number>(
    () => Number(localStorage.getItem("media-converter-job")) || 0
  );
  const [cacheGiB, setCacheGiB] = useState<number>();
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [options, setOptions] = useState<Options>({
    format: kind === "scene" ? "av1-mp4" : "jxl",
    hardware: "cpu",
    quality: 80,
    effort: 7,
    distance: 1,
    lossless: false,
    allowLarger: false,
    dropAudio: false,
    allowAlphaLoss: false,
  });
  const running =
    state?.job?.status === "queued" || state?.job?.status === "running";
  const busy = submitting || running;
  const format = formats.find((f) => f.id === options.format);
  const encoders =
    options.hardware === "gpu"
      ? format?.gpu
      : options.hardware === "auto" && format?.gpu.length
        ? format.gpu
        : format?.cpu;

  const refresh = useCallback(
    async (signal?: AbortSignal) => {
      const result = await fetch(
        `${endpoint}?jobID=${jobID}&offset=${historyOffset}`,
        { signal }
      ).then(response<State>);
      setState(result);
      setCacheGiB(
        (existing) => existing ?? result.config.cacheLimitBytes / GiB
      );
    },
    [jobID, historyOffset]
  );

  useEffect(() => {
    const controller = new AbortController();
    const poll = () =>
      void refresh(controller.signal).catch((e: Error) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    poll();
    const timer = window.setInterval(poll, 2500);
    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [refresh]);

  useEffect(() => {
    const controller = new AbortController();
    setFormats([]);
    setLoadingCapabilities(true);
    setCapabilityError("");
    void fetch(`${endpoint}?capabilities=1&backend=${backend}`, {
      signal: controller.signal,
    })
      .then(response<{ formats: Format[] }>)
      .then((v) => {
        if (!controller.signal.aborted) setFormats(v.formats);
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setCapabilityError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoadingCapabilities(false);
      });
    return () => controller.abort();
  }, [backend]);

  async function action(payload: object) {
    setSubmitting(true);
    setError("");
    try {
      const result = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      }).then(response<{ jobID?: number }>);
      if (result.jobID) {
        localStorage.setItem("media-converter-job", String(result.jobID));
        setJobID(result.jobID);
        setHistoryOffset(0);
        setState((old) =>
          old
            ? {
                ...old,
                job: {
                  jobID: result.jobID!,
                  status: "queued",
                  total: 0,
                  items: [],
                },
              }
            : old
        );
      } else await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal show onHide={onHide} size="xl" scrollable>
      <Modal.Header closeButton>
        <Modal.Title>Media converter</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <p>
          Convert {selectedIds.length} selected{" "}
          {kind === "scene" ? "video" : "image"}
          {selectedIds.length === 1 ? "" : "s"}. Tags, source URLs, characters,
          artists and original fingerprints stay attached to the same entries.
        </p>
        {error && <Alert variant="danger">{error}</Alert>}
        <Row>
          <Form.Group as={Col} controlId="converter-backend">
            <Form.Label>Run on</Form.Label>
            <Form.Control
              as="select"
              value={backend}
              disabled={busy}
              onChange={(e) => setBackend(e.target.value)}
            >
              <option value="local">This StashBooru server</option>
              <option value="remote">Remote tagging worker</option>
            </Form.Control>
            {backend === "remote" && (
              <Form.Text>
                Uses the URL and token in Visual Similarity settings. Media is
                uploaded to that worker.
              </Form.Text>
            )}
          </Form.Group>
          <Form.Group as={Col} controlId="converter-format">
            <Form.Label>Output format</Form.Label>
            <Form.Control
              as="select"
              value={options.format}
              disabled={busy || loadingCapabilities}
              onChange={(e) =>
                setOptions({ ...options, format: e.target.value })
              }
            >
              {!formats.length && (
                <option value={options.format}>
                  {loadingCapabilities
                    ? "Checking encoders…"
                    : "No encoders loaded"}
                </option>
              )}
              {formats
                .filter((f) => kind !== "scene" || f.family === "video")
                .map((f) => (
                  <option key={f.id} value={f.id} disabled={!f.available}>
                    {f.label}
                    {!f.available ? " — unavailable" : ""}
                  </option>
                ))}
            </Form.Control>
          </Form.Group>
          <Form.Group as={Col} controlId="converter-hardware">
            <Form.Label>Processor</Form.Label>
            <Form.Control
              as="select"
              value={options.hardware}
              disabled={busy}
              onChange={(e) =>
                setOptions({ ...options, hardware: e.target.value })
              }
            >
              <option value="cpu">CPU</option>
              <option value="gpu" disabled={!format?.gpu.length}>
                GPU
              </option>
              <option value="auto">Prefer GPU, otherwise CPU</option>
            </Form.Control>
            <Form.Text>
              {encoders?.length
                ? `Encoder: ${encoders[0]}`
                : "No working encoder for this selection"}
            </Form.Text>
          </Form.Group>
        </Row>
        {loadingCapabilities && (
          <p>
            <Spinner animation="border" size="sm" /> Probing codec and hardware
            support…
          </p>
        )}
        {capabilityError && <Alert variant="warning">{capabilityError}</Alert>}
        <Row>
          {(["distance", "quality", "effort"] as const)
            .filter((key) => format?.controls.includes(key))
            .map((key) => (
              <Form.Group as={Col} key={key} controlId={`converter-${key}`}>
                <Form.Label>
                  {key === "distance"
                    ? "JXL distance (0 = lossless)"
                    : key === "quality"
                      ? "Quality (higher = better)"
                      : "Effort (higher = slower)"}
                </Form.Label>
                <Form.Control
                  type="number"
                  min={key === "effort" ? 1 : 0}
                  max={key === "distance" ? 15 : key === "effort" ? 9 : 100}
                  step={key === "distance" ? 0.1 : 1}
                  value={options[key]}
                  disabled={busy}
                  onChange={(e) =>
                    setOptions({ ...options, [key]: Number(e.target.value) })
                  }
                />
              </Form.Group>
            ))}
        </Row>
        {format?.controls.includes("quality") && (
          <p className="text-muted">
            Quality is mapped to this codec’s quantizer/CRF; equal values across
            codecs do not imply equal quality. Effort selects its compression
            speed preset.
          </p>
        )}
        <Form.Check
          id="converter-larger"
          label="Keep outputs even when they are larger"
          checked={options.allowLarger}
          disabled={busy}
          onChange={(e) =>
            setOptions({ ...options, allowLarger: e.target.checked })
          }
        />
        {format?.controls.includes("lossless") && (
          <Form.Check
            id="converter-lossless"
            label="Lossless WebP"
            checked={options.lossless}
            disabled={busy}
            onChange={(e) =>
              setOptions({ ...options, lossless: e.target.checked })
            }
          />
        )}
        <Form.Check
          id="converter-audio"
          label="Allow discarding audio"
          checked={options.dropAudio}
          disabled={busy}
          onChange={(e) =>
            setOptions({ ...options, dropAudio: e.target.checked })
          }
        />
        <Form.Check
          id="converter-alpha"
          label="Allow losing transparency for formats without alpha"
          checked={options.allowAlphaLoss}
          disabled={busy}
          onChange={(e) =>
            setOptions({ ...options, allowAlphaLoss: e.target.checked })
          }
        />
        <Button
          className="my-3"
          disabled={
            busy ||
            !encoders?.length ||
            !selectedIds.length ||
            loadingCapabilities
          }
          onClick={() =>
            void action({
              action: "start",
              backend,
              options,
              targets: selectedIds.map((id) => ({ kind, id: Number(id) })),
            })
          }
        >
          Convert{" "}
          {selectedIds.length === 1 ? "file" : `${selectedIds.length} files`}
        </Button>
        {state?.job && (
          <div role="status" className="mb-3">
            <strong>
              Job #{state.job.jobID}: {state.job.status}
            </strong>{" "}
            — {state.job.items.length}/{state.job.total} processed
            {running && (
              <>
                <ProgressBar
                  animated
                  now={
                    state.job.total
                      ? (100 * state.job.items.length) / state.job.total
                      : 0
                  }
                />
                <Button
                  size="sm"
                  variant="secondary"
                  className="mt-2"
                  onClick={() => void action({ action: "cancel", jobID })}
                >
                  Cancel remaining work
                </Button>
              </>
            )}
            {state.job.error && (
              <Alert variant="danger">{state.job.error}</Alert>
            )}
            {state.job.items
              .filter((i) => i.error)
              .map((i, index) => (
                <div
                  className="text-danger"
                  key={`${i.target.kind}-${i.target.id}-${index}`}
                >
                  {i.target.kind} #{i.target.id}: {i.error}
                </div>
              ))}
            <p className="mt-2">
              This batch: {state.batchStats.converted} converted,{" "}
              {bytes(state.batchStats.savedBytes)} saved,{" "}
              {state.batchStats.largerFiles} larger outputs.
            </p>
          </div>
        )}
        <hr />
        <h5>Originals and restoration</h5>
        <p>
          Originals are retained until this cache exceeds its limit. Oldest
          originals are then permanently deleted. Zero disables retention.
          Source fingerprints and conversion history remain.
        </p>
        <Row className="align-items-end">
          <Form.Group as={Col} xs={6} controlId="converter-cache">
            <Form.Label>Restore cache limit (GiB)</Form.Label>
            <Form.Control
              type="number"
              min={0}
              step={0.25}
              value={cacheGiB ?? ""}
              disabled={busy}
              onChange={(e) => setCacheGiB(Number(e.target.value))}
            />
          </Form.Group>
          <Col className="mb-3">
            <Button
              variant="secondary"
              disabled={
                busy ||
                cacheGiB === undefined ||
                !Number.isFinite(cacheGiB) ||
                cacheGiB < 0
              }
              onClick={() =>
                void action({
                  action: "configure",
                  cacheLimitBytes: Math.round((cacheGiB ?? 20) * GiB),
                })
              }
            >
              Save limit and trim cache
            </Button>
          </Col>
        </Row>
        {state && (
          <>
            <h5>All conversions</h5>
            <StatsView value={state.stats} />
            <p className="text-muted">
              Net savings include retained originals and may be negative until
              cache eviction. Repeated conversions are combined per file. These
              figures cover media files and originals; generated previews,
              temporary files and filesystem compression are excluded.
            </p>
          </>
        )}
        <h5>Conversion history</h5>
        <div className="mb-2">
          <Button
            size="sm"
            variant="secondary"
            disabled={historyOffset === 0}
            onClick={() => setHistoryOffset(Math.max(0, historyOffset - 200))}
          >
            Newer
          </Button>{" "}
          <Button
            size="sm"
            variant="secondary"
            disabled={!state || historyOffset + 200 >= state.historyTotal}
            onClick={() => setHistoryOffset(historyOffset + 200)}
          >
            Older
          </Button>{" "}
          {state?.historyTotal ?? 0} records
        </div>
        <Button
          size="sm"
          variant="secondary"
          className="mb-2"
          disabled={busy}
          onClick={() => void action({ action: "recover" })}
        >
          Recover interrupted operations
        </Button>
        <Table responsive size="sm">
          <thead>
            <tr>
              <th>File / source fingerprints</th>
              <th>Result</th>
              <th>Space saved</th>
              <th>Original</th>
            </tr>
          </thead>
          <tbody>
            {state?.history.map((r) => {
              const before = r.before.image ?? r.before.video;
              const after = r.after.image ?? r.after.video;
              return (
                <tr key={r.id}>
                  <td>
                    <div className="text-break">{before?.basename}</div>
                    <small>{new Date(r.createdAt).toLocaleString()}</small>
                    <details>
                      <summary>Fingerprints</summary>
                      {before?.fingerprints.map((f) => (
                        <div className="text-break" key={f.Type}>
                          <small>
                            {f.Type}: {String(f.Fingerprint)}
                          </small>
                        </div>
                      ))}
                    </details>
                  </td>
                  <td>
                    {r.status}
                    <small className="d-block">
                      {r.backend} · {r.result.encoder}{" "}
                      {r.result.seconds
                        ? `· ${r.result.seconds.toFixed(1)}s`
                        : ""}
                    </small>
                    {r.error && (
                      <small className="text-warning">{r.error}</small>
                    )}
                  </td>
                  <td>
                    {before && after ? (
                      <>
                        {bytes(before.size - after.size)}
                        <small className="d-block">
                          {bytes(before.size)} → {bytes(after.size)}
                        </small>
                      </>
                    ) : (
                      "—"
                    )}
                  </td>
                  <td>
                    {r.status === "complete" && r.cached ? (
                      <Button
                        size="sm"
                        variant="secondary"
                        disabled={busy}
                        onClick={() =>
                          void action({ action: "restore", recordID: r.id })
                        }
                      >
                        Restore
                      </Button>
                    ) : r.status === "complete" ? (
                      "Evicted"
                    ) : r.status === "restored" ? (
                      "Restored"
                    ) : r.status === "prepared" ||
                      r.status === "committed" ||
                      r.status === "restoring" ? (
                      "Recovery pending"
                    ) : (
                      "Source retained"
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </Table>
      </Modal.Body>
      <Modal.Footer>
        <span className="mr-auto text-muted">
          Jobs continue when this window is closed.
        </span>
        <Button variant="secondary" onClick={onHide}>
          Close
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
