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
  basename: string;
  size: number;
  fingerprints: { Type: string; Fingerprint: string | number }[];
}

interface UpscaleHistoryRecord {
  id: string;
  status: string;
  createdAt: string;
  cached: boolean;
  error?: string;
  mediaKind?: "image" | "scene";
  mediaID?: number;
  before: { image?: FileSnapshot; video?: FileSnapshot };
  after: { image?: FileSnapshot; video?: FileSnapshot };
  options: { upscaler?: string; upscaleScale?: number };
  result: { encoder: string; seconds: number };
  backend: string;
}

interface UpscaleStats {
  converted: number;
  savedBytes: number;
  cacheBytes: number;
}

interface UpscaleHistoryState {
  config: { cacheLimitBytes: number };
  stats: UpscaleStats;
  history: UpscaleHistoryRecord[];
  historyTotal: number;
}

const upscaleEndpoint = "image/upscale";
const GiB = 1024 ** 3;

type QueuedMode = "copy" | "replace" | "restore";

function bytes(value: number) {
  const absolute = Math.abs(value);
  const unit = absolute >= GiB ? GiB : 1024 ** 2;
  return `${(value / unit).toLocaleString(undefined, {
    maximumFractionDigits: 2,
  })} ${unit === GiB ? "GiB" : "MiB"}`;
}

function mediaURL(record: UpscaleHistoryRecord) {
  if (!record.mediaID) return undefined;
  return record.mediaKind === "image"
    ? `/images/${record.mediaID}`
    : record.mediaKind === "scene"
      ? `/scenes/${record.mediaID}`
      : undefined;
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
  const [queuedMode, setQueuedMode] = useState<QueuedMode>();
  const [error, setError] = useState("");
  const [history, setHistory] = useState<UpscaleHistoryState>();
  const [historyOffset, setHistoryOffset] = useState(0);
  const [cacheGiB, setCacheGiB] = useState<number>();
  const [historyError, setHistoryError] = useState("");

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

  const refreshHistory = useCallback(
    async (signal?: AbortSignal) => {
      const value = await fetch(upscaleEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "history", offset: historyOffset }),
        signal,
      }).then(conversionResponse<UpscaleHistoryState>);
      setHistory(value);
      setCacheGiB((current) => current ?? value.config.cacheLimitBytes / GiB);
    },
    [historyOffset]
  );

  useEffect(() => {
    const controller = new AbortController();
    const poll = () =>
      void refreshHistory(controller.signal).catch((e: Error) => {
        if (!controller.signal.aborted) setHistoryError(e.message);
      });
    poll();
    const timer = window.setInterval(poll, 2500);
    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [refreshHistory]);

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

  async function upscaleAction<T>(payload: object) {
    return fetch(upscaleEndpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    }).then(conversionResponse<T>);
  }

  async function queueUpscale(replace: boolean) {
    setSubmitting(true);
    setError("");
    try {
      const common = {
        action: replace ? "replace" : "copy",
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
      const result = await upscaleAction<{ jobID: number }>(common);
      setQueuedMode(replace ? "replace" : "copy");
      setJobID(result.jobID);
      if (replace) setHistoryOffset(0);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  async function historyAction(payload: object, mode?: QueuedMode) {
    setSubmitting(true);
    setHistoryError("");
    try {
      const result = await upscaleAction<{ jobID?: number }>(payload);
      if (result.jobID) {
        setJobID(result.jobID);
        setQueuedMode(mode);
      }
      await refreshHistory();
    } catch (e) {
      setHistoryError(e instanceof Error ? e.message : String(e));
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
  const outputGrowth = Math.max(0, -(history?.stats.savedBytes ?? 0));

  return (
    <>
      <Modal show={!confirmReplace} onHide={onHide} size="xl" scrollable>
        <Modal.Header closeButton>
          <Modal.Title>Upscale images</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          {error && <Alert variant="danger">{error}</Alert>}
          {jobID && (
            <Alert variant="success">
              {queuedMode === "copy"
                ? `Non-destructive upscaling job #${jobID} queued. The original${
                    oneImage ? " stays" : "s stay"
                  } untouched.`
                : queuedMode === "restore"
                  ? `Upscaling restore job #${jobID} queued.`
                  : `Replacement upscaling job #${jobID} queued.`}
            </Alert>
          )}
          <Tabs
            id="image-upscaling-tabs"
            defaultActiveKey="upscale"
            className="mb-3"
          >
            <Tab eventKey="upscale" title="Upscale">
              <p>
                Upscale {selectedIds.length} selected image
                {oneImage ? "" : "s"}. By default StashBooru creates a new Image
                and leaves the original Image and original file untouched.
              </p>
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
                          setOptions({
                            ...options,
                            upscaler: event.target.value,
                          })
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
                        <option value="auto">
                          Use conversion format default
                        </option>
                        {imageFormats.map((format) => (
                          <option key={format.id} value={format.id}>
                            {format.label}
                          </option>
                        ))}
                      </Form.Control>
                    </Form.Group>
                  </Row>

                  <Form.Group
                    className="mt-3"
                    controlId="upscaler-save-behavior"
                  >
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
                      Replaces the active file and keeps the original in the
                      dedicated upscaling restore cache. You will be asked to
                      confirm again before it starts.
                    </Form.Text>
                  </Form.Group>

                  {replaceOriginal && (
                    <Alert variant="warning" className="mt-3">
                      Replacement changes the active library file. The original
                      is kept only in the upscaling restore cache and may later
                      be evicted according to that cache&apos;s limit.
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
              {historyError && <Alert variant="danger">{historyError}</Alert>}
              <p className="text-muted">
                This history covers replacement upscales that need a restore
                point. Non-destructive upscales keep their source as a normal
                library Image and therefore do not consume restore-cache space.
              </p>
              <Row className="mb-3">
                <Col xs={12} sm={4} className="mb-2">
                  <strong>{history?.stats.converted ?? 0}</strong>
                  <div>Replacement upscales</div>
                </Col>
                <Col xs={12} sm={4} className="mb-2">
                  <strong>{bytes(history?.stats.cacheBytes ?? 0)}</strong>
                  <div>Restorable originals</div>
                </Col>
                <Col xs={12} sm={4} className="mb-2">
                  <strong>{bytes(outputGrowth)}</strong>
                  <div>Active output size growth</div>
                </Col>
              </Row>

              <Row className="align-items-end">
                <Form.Group as={Col} xs={6} controlId="upscaling-cache-limit">
                  <Form.Label>Upscaling restore cache limit (GiB)</Form.Label>
                  <Form.Control
                    className="text-input"
                    type="number"
                    min={0}
                    step={0.25}
                    value={cacheGiB ?? ""}
                    disabled={submitting}
                    onChange={(event) =>
                      setCacheGiB(Number(event.target.value))
                    }
                  />
                </Form.Group>
                <Col className="mb-3">
                  <Button
                    variant="secondary"
                    disabled={
                      submitting ||
                      cacheGiB === undefined ||
                      !Number.isFinite(cacheGiB) ||
                      cacheGiB < 0
                    }
                    onClick={() =>
                      void historyAction({
                        action: "configure",
                        cacheLimitBytes: Math.round((cacheGiB ?? 20) * GiB),
                      })
                    }
                  >
                    Save limit and trim cache
                  </Button>
                </Col>
              </Row>

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
                    !history || historyOffset + 200 >= history.historyTotal
                  }
                  onClick={() => setHistoryOffset(historyOffset + 200)}
                >
                  Older
                </Button>{" "}
                {history?.historyTotal ?? 0} records
              </div>
              <Button
                size="sm"
                variant="secondary"
                className="mb-2"
                disabled={submitting}
                onClick={() => void historyAction({ action: "recover" })}
              >
                Recover interrupted upscaling operations
              </Button>

              <Table responsive size="sm">
                <thead>
                  <tr>
                    <th>File / source fingerprints</th>
                    <th>Upscale result</th>
                    <th>Size change</th>
                    <th>Original</th>
                  </tr>
                </thead>
                <tbody>
                  {history?.history.map((record) => {
                    const before = record.before.image ?? record.before.video;
                    const after = record.after.image ?? record.after.video;
                    const url = mediaURL(record);
                    return (
                      <tr key={record.id}>
                        <td>
                          <div className="text-break">
                            {url ? (
                              <Link to={url} onClick={onHide}>
                                {before?.basename}
                              </Link>
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
                                  {fingerprint.Type}:{" "}
                                  {String(fingerprint.Fingerprint)}
                                </small>
                              </div>
                            ))}
                          </details>
                        </td>
                        <td>
                          {record.status}
                          <small className="d-block">
                            {record.options.upscaler ?? "upscaler"}
                            {record.options.upscaleScale
                              ? ` · ${record.options.upscaleScale}×`
                              : ""}{" "}
                            · {record.backend} · {record.result.encoder}
                            {record.result.seconds
                              ? ` · ${record.result.seconds.toFixed(1)}s`
                              : ""}
                          </small>
                          {record.error && (
                            <small className="text-warning">
                              {record.error}
                            </small>
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
                              disabled={submitting}
                              onClick={() =>
                                void historyAction(
                                  {
                                    action: "restore",
                                    recordID: record.id,
                                  },
                                  "restore"
                                )
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
            and cached originals can be evicted when that cache limit is
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
