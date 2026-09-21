import React, { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Button,
  Col,
  Form,
  Modal,
  Row,
  Spinner,
  Tab,
  Table,
  Tabs,
} from "react-bootstrap";
import { Link } from "react-router-dom";

import {
  conversionEndpoint,
  conversionResponse,
  ConversionFormat,
  ConversionUpscaler,
  loadUpscalingDefaults,
  UpscalingDefaults,
} from "./mediaConversion";

interface Capabilities {
  formats: ConversionFormat[];
  upscalers?: ConversionUpscaler[];
  backend: string;
  notice?: string;
}

interface FileSnapshot {
  id: number;
  basename: string;
  path: string;
  size: number;
  fingerprints: { Type: string; Fingerprint: string | number }[];
}

interface RestoreRecord {
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

interface RestoreStats {
  converted: number;
  savedBytes: number;
  averageSavedBytes: number;
  savedPercent: number;
  cacheBytes: number;
  netSavedBytes: number;
  largerFiles: number;
}

interface RestoreJob {
  jobID: number;
  status: string;
  total: number;
  error?: string;
  items: { error?: string }[];
}

interface RestoreState {
  config: { cacheLimitBytes: number };
  stats: RestoreStats;
  batchStats: RestoreStats;
  latestStats: RestoreStats;
  history: RestoreRecord[];
  historyTotal: number;
  job?: RestoreJob;
}

const upscaleEndpoint = "image/upscale";
const upscaleRestoreEndpoint = "image/upscale-restore";
const GiB = 1024 ** 3;

type QueuedMode = "copy" | "replace";

function bytes(value: number) {
  const unit = Math.abs(value) >= GiB ? GiB : 1024 ** 2;
  return `${(value / unit).toLocaleString(undefined, {
    maximumFractionDigits: 2,
  })} ${unit === GiB ? "GiB" : "MiB"}`;
}

function StatsView({ value }: { value: RestoreStats }) {
  return (
    <Row className="mb-3">
      <Col xs={12} sm={6} lg={3} className="mb-2">
        <strong>{value.converted}</strong>
        <div>Replacement upscales</div>
      </Col>
      <Col xs={12} sm={6} lg={3} className="mb-2">
        <strong>{bytes(value.cacheBytes)}</strong>
        <div>Restorable originals</div>
      </Col>
      <Col xs={12} sm={6} lg={3} className="mb-2">
        <strong>{bytes(value.savedBytes)}</strong>
        <div>Media size change</div>
      </Col>
      <Col xs={12} sm={6} lg={3} className="mb-2">
        <strong>{value.largerFiles}</strong>
        <div>Larger upscaled outputs</div>
      </Col>
    </Row>
  );
}

export const MediaUpscalingDialog: React.FC<{
  selectedIds: string[];
  onHide: () => void;
}> = ({ selectedIds, onHide }) => {
  const [options, setOptions] = useState<UpscalingDefaults>(() =>
    loadUpscalingDefaults()
  );
  const [capabilities, setCapabilities] = useState<Capabilities>();
  const [loading, setLoading] = useState(true);
  const [attempt, setAttempt] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [replaceOriginal, setReplaceOriginal] = useState(false);
  const [confirmReplace, setConfirmReplace] = useState(false);
  const [jobID, setJobID] = useState<number>();
  const [restoreJobID, setRestoreJobID] = useState<number>(
    () => Number(localStorage.getItem("media-upscaling-restore-job")) || 0
  );
  const [queuedMode, setQueuedMode] = useState<QueuedMode>();
  const [error, setError] = useState("");
  const [restoreError, setRestoreError] = useState("");
  const [restoreState, setRestoreState] = useState<RestoreState>();
  const [historyOffset, setHistoryOffset] = useState(0);
  const [cacheGiB, setCacheGiB] = useState<number>();

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    void fetch(`${conversionEndpoint}?capabilities=1&refresh=${attempt}`, {
      signal: controller.signal,
    })
      .then(conversionResponse<Capabilities>)
      .then((value) => {
        if (!controller.signal.aborted) setCapabilities(value);
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [attempt]);

  const refreshRestore = useCallback(
    async (signal?: AbortSignal) => {
      const value = await fetch(
        `${upscaleRestoreEndpoint}?jobID=${restoreJobID}&offset=${historyOffset}`,
        { signal }
      ).then(conversionResponse<RestoreState>);
      setRestoreState(value);
      setCacheGiB(
        (current) => current ?? value.config.cacheLimitBytes / GiB
      );
    },
    [historyOffset, restoreJobID]
  );

  useEffect(() => {
    const controller = new AbortController();
    const poll = () => {
      void refreshRestore(controller.signal).catch((e: Error) => {
        if (!controller.signal.aborted) setRestoreError(e.message);
      });
    };
    poll();
    const timer = window.setInterval(poll, 2500);
    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [refreshRestore]);

  const upscalers = capabilities?.upscalers ?? [];
  const selectedUpscaler = upscalers.find((u) => u.id === options.upscaler);
  const imageFormats = (capabilities?.formats ?? []).filter(
    (format) => format.family !== "video" && format.available
  );
  const selectedFormat =
    options.format === "auto"
      ? undefined
      : imageFormats.find((format) => format.id === options.format);
  const ready =
    selectedIds.length > 0 &&
    !!selectedUpscaler?.available &&
    (options.hardware !== "cpu" || selectedUpscaler.cpu) &&
    (options.format === "auto" || !!selectedFormat);
  const restoreRunning =
    restoreState?.job?.status === "queued" ||
    restoreState?.job?.status === "running";
  const restoreBusy = submitting || restoreRunning;

  const commonRequest = {
    useEncodingDefaults: true,
    targets: selectedIds.map((id) => ({ kind: "image", id: Number(id) })),
    options: {
      upscaler: options.upscaler,
      upscaleScale: options.scale,
      format: options.format,
      hardware: options.hardware,
      quality: 90,
      effort: 7,
      allowLarger: true,
      lossless: false,
      dropAudio: false,
      allowAlphaLoss: false,
    },
  };

  async function queueUpscale(replace: boolean) {
    setSubmitting(true);
    setError("");
    try {
      const result = await fetch(
        replace ? upscaleRestoreEndpoint : upscaleEndpoint,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(
            replace ? { action: "replace", ...commonRequest } : commonRequest
          ),
        }
      ).then(conversionResponse<{ jobID: number }>);
      setQueuedMode(replace ? "replace" : "copy");
      setJobID(result.jobID);
      if (replace) {
        localStorage.setItem("media-upscaling-restore-job", String(result.jobID));
        setRestoreJobID(result.jobID);
        setHistoryOffset(0);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  async function restoreAction(payload: object) {
    setSubmitting(true);
    setRestoreError("");
    try {
      const result = await fetch(upscaleRestoreEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      }).then(conversionResponse<{ jobID: number }>);
      localStorage.setItem("media-upscaling-restore-job", String(result.jobID));
      setRestoreJobID(result.jobID);
      setHistoryOffset(0);
    } catch (e) {
      setRestoreError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  function start() {
    if (replaceOriginal) {
      setConfirmReplace(true);
      return;
    }
    void queueUpscale(false);
  }

  function confirmDestructiveUpscale() {
    setConfirmReplace(false);
    void queueUpscale(true);
  }

  const oneImage = selectedIds.length === 1;

  return (
    <>
      <Modal show={!confirmReplace} onHide={onHide} size="xl" scrollable>
        <Modal.Header closeButton>
          <Modal.Title>Upscale images</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Tabs id="media-upscaling-tabs" defaultActiveKey="upscale" className="mb-3">
            <Tab eventKey="upscale" title="Upscale">
              <p>
                Upscale {selectedIds.length} selected image
                {oneImage ? "" : "s"}. By default StashBooru creates a new Image
                and leaves the original Image and original file untouched.
              </p>
              {error && <Alert variant="danger">{error}</Alert>}
              {jobID && (
                <Alert variant="success">
                  {queuedMode === "copy"
                    ? `Non-destructive upscaling job #${jobID} queued. The original${
                        oneImage ? " stays" : "s stay"
                      } untouched.`
                    : `Replacement upscaling job #${jobID} queued in the dedicated upscaling restore cache.`}
                </Alert>
              )}
              {loading ? (
                <p>
                  <Spinner animation="border" size="sm" /> Checking worker…
                </p>
              ) : (
                <>
                  <Row>
                    <Form.Group
                      as={Col}
                      xs={12}
                      md={6}
                      controlId="upscaler-model"
                    >
                      <Form.Label>Upscaler</Form.Label>
                      <Form.Control
                        as="select"
                        className="input-control"
                        value={options.upscaler}
                        disabled={submitting || !!jobID}
                        onChange={(event) =>
                          setOptions({ ...options, upscaler: event.target.value })
                        }
                      >
                        {upscalers.length === 0 && (
                          <option value={options.upscaler}>
                            No upscalers detected
                          </option>
                        )}
                        {upscalers.map((upscaler) => (
                          <option
                            key={upscaler.id}
                            value={upscaler.id}
                            disabled={!upscaler.available}
                          >
                            {upscaler.label}
                            {!upscaler.available ? " — not installed" : ""}
                          </option>
                        ))}
                      </Form.Control>
                    </Form.Group>
                    <Form.Group
                      as={Col}
                      xs={12}
                      md={6}
                      controlId="upscaler-scale"
                    >
                      <Form.Label>Scale</Form.Label>
                      <Form.Control
                        as="select"
                        className="input-control"
                        value={options.scale}
                        disabled={submitting || !!jobID}
                        onChange={(event) =>
                          setOptions({
                            ...options,
                            scale: Number(event.target.value) === 4 ? 4 : 2,
                          })
                        }
                      >
                        <option value={2}>2×</option>
                        <option value={4}>4×</option>
                      </Form.Control>
                    </Form.Group>
                  </Row>
                  <Row>
                    <Form.Group
                      as={Col}
                      xs={12}
                      md={6}
                      controlId="upscaler-processor"
                    >
                      <Form.Label>Processor</Form.Label>
                      <Form.Control
                        as="select"
                        className="input-control"
                        value={options.hardware}
                        disabled={submitting || !!jobID}
                        onChange={(event) =>
                          setOptions({
                            ...options,
                            hardware: event.target
                              .value as UpscalingDefaults["hardware"],
                          })
                        }
                      >
                        <option value="auto">Prefer GPU, otherwise CPU</option>
                        <option value="gpu">GPU</option>
                        <option
                          value="cpu"
                          disabled={selectedUpscaler?.cpu === false}
                        >
                          CPU
                        </option>
                      </Form.Control>
                    </Form.Group>
                    <Form.Group
                      as={Col}
                      xs={12}
                      md={6}
                      controlId="upscaler-format"
                    >
                      <Form.Label>Output format</Form.Label>
                      <Form.Control
                        as="select"
                        className="input-control"
                        value={options.format}
                        disabled={submitting || !!jobID}
                        onChange={(event) =>
                          setOptions({ ...options, format: event.target.value })
                        }
                      >
                        <option value="auto">Use conversion format default</option>
                        {imageFormats.map((format) => (
                          <option key={format.id} value={format.id}>
                            {format.label}
                          </option>
                        ))}
                      </Form.Control>
                    </Form.Group>
                  </Row>

                  <Form.Group className="mt-3" controlId="upscaler-save-behavior">
                    <Form.Label>Save behavior</Form.Label>
                    <Form.Check
                      type="radio"
                      id="upscaler-create-copy"
                      name="upscaler-save-behavior"
                      label="Create a new upscaled Image (recommended)"
                      checked={!replaceOriginal}
                      disabled={submitting || !!jobID}
                      onChange={() => setReplaceOriginal(false)}
                    />
                    <Form.Text className="text-muted d-block mb-2">
                      Keeps the original Image and file untouched. The new Image
                      inherits the source metadata and relations.
                    </Form.Text>
                    <Form.Check
                      type="radio"
                      id="upscaler-replace-original"
                      name="upscaler-save-behavior"
                      label={
                        oneImage
                          ? "Replace current image"
                          : "Replace selected images"
                      }
                      checked={replaceOriginal}
                      disabled={submitting || !!jobID}
                      onChange={() => setReplaceOriginal(true)}
                    />
                    <Form.Text className="text-muted d-block">
                      Replaces the active file and stores the original in the
                      dedicated upscaling restore cache. You will be asked to
                      confirm again before it starts.
                    </Form.Text>
                  </Form.Group>

                  {replaceOriginal && (
                    <Alert variant="warning" className="mt-3">
                      Replacement changes the active library file. The original
                      is kept only in the upscaling restore cache and may later be
                      evicted according to its independent cache limit.
                    </Alert>
                  )}

                  <p className="text-muted">
                    Worker: {capabilities?.backend ?? "unknown"}.{" "}
                    {selectedUpscaler?.notice}
                  </p>
                  {capabilities?.notice && (
                    <Alert variant="info">{capabilities.notice}</Alert>
                  )}
                  {!selectedUpscaler?.available && (
                    <Alert variant="secondary">
                      Configure the local executable/model paths in System →
                      Image upscaling, or configure the equivalent
                      STASH_WAIFU2X_* / STASH_SEEDVR2_* environment variables on
                      a remote worker.
                    </Alert>
                  )}
                  <p>
                    <Link
                      to="/settings?tab=system#media-upscaling"
                      onClick={onHide}
                    >
                      Edit upscaler defaults and model paths in System settings
                    </Link>
                  </p>
                  <Button
                    size="sm"
                    variant="secondary"
                    className="mr-2"
                    disabled={submitting || !!jobID}
                    onClick={() => setAttempt((value) => value + 1)}
                  >
                    Recheck worker
                  </Button>
                  <Button
                    disabled={submitting || !!jobID || !ready}
                    onClick={start}
                  >
                    {submitting
                      ? "Queuing…"
                      : replaceOriginal
                        ? oneImage
                          ? "Upscale and replace image…"
                          : `Upscale and replace ${selectedIds.length} images…`
                        : oneImage
                          ? "Create upscaled image"
                          : `Create ${selectedIds.length} upscaled images`}
                  </Button>
                </>
              )}
            </Tab>

            <Tab eventKey="statistics" title="Statistics & restore">
              {restoreError && <Alert variant="danger">{restoreError}</Alert>}
              {restoreState?.job?.error && (
                <Alert variant="danger">{restoreState.job.error}</Alert>
              )}
              <p className="text-muted">
                This tab is independent from Media converter history. It contains
                only destructive upscaling replacements and their restorable
                originals. Non-destructive upscaled copies do not consume restore
                cache space.
              </p>
              {restoreState && (
                <>
                  <h5>Upscaling replacements</h5>
                  <StatsView value={restoreState.stats} />
                  {restoreState.latestStats.converted > 0 && (
                    <p className="text-muted">
                      Latest replacement batch: {restoreState.latestStats.converted}{" "}
                      file{restoreState.latestStats.converted === 1 ? "" : "s"} ·{" "}
                      {bytes(restoreState.latestStats.cacheBytes)} of originals
                      retained.
                    </p>
                  )}
                </>
              )}

              <h5>Originals and restoration</h5>
              <p>
                Originals are retained until the upscaling cache exceeds this
                limit. Oldest originals are then permanently deleted. Zero
                disables retention. This setting does not affect the converter
                restore cache.
              </p>
              <Row className="align-items-end">
                <Form.Group as={Col} xs={6} controlId="upscaler-restore-cache">
                  <Form.Label>Upscaling restore cache limit (GiB)</Form.Label>
                  <Form.Control
                    className="text-input"
                    type="number"
                    min={0}
                    step={0.25}
                    value={cacheGiB ?? ""}
                    disabled={restoreBusy}
                    onChange={(event) => setCacheGiB(Number(event.target.value))}
                  />
                </Form.Group>
                <Col className="mb-3">
                  <Button
                    variant="secondary"
                    disabled={
                      restoreBusy ||
                      cacheGiB === undefined ||
                      !Number.isFinite(cacheGiB) ||
                      cacheGiB < 0
                    }
                    onClick={() =>
                      void restoreAction({
                        action: "configure",
                        cacheLimitBytes: Math.round((cacheGiB ?? 20) * GiB),
                      })
                    }
                  >
                    Save limit and trim cache
                  </Button>
                </Col>
              </Row>

              <h5>Upscaling replacement history</h5>
              <div className="mb-2">
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={historyOffset === 0}
                  onClick={() =>
                    setHistoryOffset(Math.max(0, historyOffset - 200))
                  }
                >
                  Newer
                </Button>{" "}
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={
                    !restoreState ||
                    historyOffset + 200 >= restoreState.historyTotal
                  }
                  onClick={() => setHistoryOffset(historyOffset + 200)}
                >
                  Older
                </Button>{" "}
                {restoreState?.historyTotal ?? 0} records
              </div>
              <Button
                size="sm"
                variant="secondary"
                className="mb-2"
                disabled={restoreBusy}
                onClick={() => void restoreAction({ action: "recover" })}
              >
                Recover interrupted upscales
              </Button>
              <Table responsive size="sm">
                <thead>
                  <tr>
                    <th>File / source fingerprints</th>
                    <th>Result</th>
                    <th>Size change</th>
                    <th>Original</th>
                  </tr>
                </thead>
                <tbody>
                  {restoreState?.history.map((record) => {
                    const before = record.before.image ?? record.before.video;
                    const after = record.after.image ?? record.after.video;
                    return (
                      <tr key={record.id}>
                        <td>
                          <div className="text-break">
                            {before?.id ? (
                              <a href={`/image/file/${before.id}/open`}>
                                {before.basename}
                              </a>
                            ) : (
                              before?.basename
                            )}
                          </div>
                          <small>
                            {new Date(record.createdAt).toLocaleString()}
                          </small>
                          <details>
                            <summary>Fingerprints</summary>
                            {before?.fingerprints.map((fingerprint) => (
                              <div
                                className="text-break"
                                key={fingerprint.Type}
                              >
                                <small>
                                  {fingerprint.Type}: {String(fingerprint.Fingerprint)}
                                </small>
                              </div>
                            ))}
                          </details>
                        </td>
                        <td>
                          {record.status}
                          <small className="d-block">
                            {record.backend} · {record.result.encoder}{" "}
                            {record.result.seconds
                              ? `· ${record.result.seconds.toFixed(1)}s`
                              : ""}
                          </small>
                          {record.error && (
                            <small className="text-warning">{record.error}</small>
                          )}
                        </td>
                        <td>
                          {before && after ? (
                            <>
                              {bytes(after.size - before.size)}
                              <small className="d-block">
                                {bytes(before.size)} → {bytes(after.size)}
                              </small>
                            </>
                          ) : (
                            "—"
                          )}
                        </td>
                        <td>
                          {record.status === "complete" && record.cached ? (
                            <Button
                              size="sm"
                              variant="secondary"
                              disabled={restoreBusy}
                              onClick={() =>
                                void restoreAction({
                                  action: "restore",
                                  recordID: record.id,
                                })
                              }
                            >
                              Restore
                            </Button>
                          ) : record.status === "complete" ? (
                            "Evicted"
                          ) : record.status === "restored" ? (
                            "Restored"
                          ) : record.status === "prepared" ||
                            record.status === "committed" ||
                            record.status === "restoring" ? (
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
            </Tab>
          </Tabs>
        </Modal.Body>
        <Modal.Footer>
          <span className="mr-auto text-muted">
            {restoreState?.job &&
              `Restore job #${restoreState.job.jobID}: ${restoreState.job.status}. `}
            Jobs continue when this window is closed.
          </span>
          <Button variant="secondary" onClick={onHide}>
            Close
          </Button>
        </Modal.Footer>
      </Modal>

      <Modal
        show={confirmReplace}
        onHide={() => setConfirmReplace(false)}
        centered
      >
        <Modal.Header closeButton>
          <Modal.Title>
            {oneImage ? "Replace current image?" : "Replace selected images?"}
          </Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Alert variant="danger">
            <strong>Are you sure?</strong>
            <br />
            {oneImage
              ? "The current image file will be replaced by the upscaled result."
              : `The active files for ${selectedIds.length} images will be replaced by their upscaled results.`}
          </Alert>
          <p className="mb-0">
            This uses the destructive upscaling path. Originals are put in the
            dedicated upscaling restore cache rather than the converter cache,
            and cached originals can be evicted when its configured limit is
            reached.
          </p>
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={submitting}
            onClick={() => setConfirmReplace(false)}
          >
            Cancel
          </Button>
          <Button
            variant="danger"
            disabled={submitting}
            onClick={confirmDestructiveUpscale}
          >
            {submitting ? "Queuing…" : "Replace and upscale"}
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
};
