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
  Tab,
  Table,
  Tabs,
} from "react-bootstrap";
import { Link } from "react-router-dom";
import {
  conversionEndpoint as endpoint,
  conversionResponse as response,
  ConversionConfig,
  ConversionFormat as Format,
  ConversionPlan,
  ConversionUpscaler,
} from "./mediaConversion";

interface Target {
  kind: "image" | "scene";
  id: number;
}
interface Options {
  upscaler?: string;
  upscaleScale?: number;
  format: string;
  hardware: string;
  quality: number;
  effort: number;
  lossless: boolean;
  allowLarger: boolean;
  dropAudio: boolean;
  allowAlphaLoss: boolean;
}
interface FileSnapshot {
  id: number;
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
  backend?: string;
  notice?: string;
  items: { target: Target; recordID?: string; error?: string }[];
}
interface State {
  config: ConversionConfig;
  stats: Stats;
  batchStats: Stats;
  latestStats: Stats;
  history: Conversion[];
  historyTotal: number;
  job?: Job;
}
const GiB = 1024 ** 3;

function bytes(n: number) {
  const unit = Math.abs(n) >= GiB ? GiB : 1024 ** 2;
  return `${(n / unit).toLocaleString(undefined, { maximumFractionDigits: 2 })} ${unit === GiB ? "GiB" : "MiB"}`;
}
function StatsView({ value }: { value: Stats }) {
  return (
    <Row className="mb-3">
      <Col xs={12} sm={6} lg={3} className="mb-2">
        <strong>{bytes(value.savedBytes)}</strong>
        <div>Conversion savings ({value.savedPercent.toFixed(1)}%)</div>
      </Col>
      <Col xs={12} sm={6} lg={3} className="mb-2">
        <strong>{bytes(value.averageSavedBytes)}</strong>
        <div>Average per file</div>
      </Col>
      <Col xs={12} sm={6} lg={3} className="mb-2">
        <strong>{bytes(value.cacheBytes)}</strong>
        <div>Restorable originals</div>
      </Col>
      <Col xs={12} sm={6} lg={3} className="mb-2">
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
  const [useEncodingDefaults, setUseEncodingDefaults] = useState(true);
  const [resolvedBackend, setResolvedBackend] = useState("");
  const [workerNotice, setWorkerNotice] = useState("");
  const [capabilityAttempt, setCapabilityAttempt] = useState(0);
  const [plans, setPlans] = useState<ConversionPlan[]>([]);
  const [loadingPreview, setLoadingPreview] = useState(true);
  const [previewError, setPreviewError] = useState("");
  const [formats, setFormats] = useState<Format[]>([]);
  const [upscalers, setUpscalers] = useState<ConversionUpscaler[]>([]);
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
    format: "auto",
    hardware: "auto",
    quality: kind === "scene" ? 80 : 90,
    effort: 7,
    lossless: false,
    allowLarger: false,
    dropAudio: false,
    allowAlphaLoss: false,
  });
  const running =
    state?.job?.status === "queued" || state?.job?.status === "running";
  const busy = submitting || running;
  const outputIDs =
    options.format === "auto"
      ? [...new Set(plans.filter((p) => !p.error).map((p) => p.output))]
      : [options.format];
  const chosenFormats = formats.filter((f) => outputIDs.includes(f.id));
  const supports = (control: string) =>
    chosenFormats.some(
      (f) =>
        f.controls.includes(control) ||
        (control === "quality" && f.controls.includes("distance"))
    );
  const encodersFor = (f: Format) =>
    options.hardware === "gpu"
      ? f.gpu
      : options.hardware === "auto" && f.gpu.length
        ? f.gpu
        : f.cpu;
  const canEncode =
    outputIDs.length > 0 &&
    chosenFormats.length === outputIDs.length &&
    chosenFormats.every((f) => f.available && encodersFor(f).length > 0);
  const targetsJSON = JSON.stringify(
    selectedIds.map((id) => ({ kind, id: Number(id) }))
  );
  const defaultsJSON = JSON.stringify([
    state?.config.formatDefaults,
    state?.config.encodingDefaults,
  ]);
  const backend = state?.config.backend;

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
    if (!backend) return;
    const controller = new AbortController();
    setFormats([]);
    setUpscalers([]);
    setResolvedBackend("");
    setWorkerNotice("");
    setLoadingCapabilities(true);
    setCapabilityError("");
    void fetch(`${endpoint}?capabilities=1&refresh=${capabilityAttempt}`, {
      signal: controller.signal,
    })
      .then(
        response<{
          formats: Format[];
          upscalers?: ConversionUpscaler[];
          backend: string;
          notice: string;
        }>
      )
      .then((v) => {
        if (!controller.signal.aborted) {
          setFormats(v.formats);
          setUpscalers(v.upscalers ?? []);
          setResolvedBackend(v.backend);
          setWorkerNotice(v.notice);
        }
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setCapabilityError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoadingCapabilities(false);
      });
    return () => controller.abort();
  }, [backend, capabilityAttempt]);

  useEffect(() => {
    if (!backend || !defaultsJSON) return;
    const controller = new AbortController();
    setLoadingPreview(true);
    setPreviewError("");
    void fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        action: "preview",
        targets: JSON.parse(targetsJSON),
      }),
      signal: controller.signal,
    })
      .then(response<{ plans: ConversionPlan[] }>)
      .then((v) => {
        if (!controller.signal.aborted) {
          setPlans(v.plans);
          const first = v.plans.find((p) => !p.error);
          if (first)
            setOptions((old) => ({
              ...old,
              quality: first.quality,
              effort: first.effort,
            }));
        }
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setPreviewError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoadingPreview(false);
      });
    return () => controller.abort();
  }, [targetsJSON, defaultsJSON, backend]);

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
        {error && <Alert variant="danger">{error}</Alert>}
        {state?.job?.error && <Alert variant="danger">{state.job.error}</Alert>}
        <Tabs
          id="media-converter-tabs"
          defaultActiveKey="convert"
          className="mb-3"
        >
          <Tab eventKey="convert" title="Convert">
            <p>
              Convert {selectedIds.length} selected{" "}
              {kind === "scene" ? "video" : "image"}
              {selectedIds.length === 1 ? "" : "s"}. Tags, source URLs,
              characters, artists and original fingerprints stay attached to the
              same entries.
            </p>
            <Row>
              <Form.Group as={Col} xs={12} md={8} controlId="converter-format">
                <Form.Label>Output format</Form.Label>
                <Form.Control
                  className="text-input"
                  readOnly
                  value={
                    loadingPreview
                      ? "Reading input formats…"
                      : outputIDs
                          .map(
                            (id) =>
                              formats.find((f) => f.id === id)?.label ??
                              id.toUpperCase()
                          )
                          .join(" / ") || "Unavailable"
                  }
                />
                <Form.Text>
                  <Link
                    to="/settings?tab=system#media-converter"
                    onClick={onHide}
                  >
                    Edit format defaults in System settings
                  </Link>
                </Form.Text>
              </Form.Group>
              <Form.Group
                as={Col}
                xs={12}
                md={4}
                controlId="converter-hardware"
              >
                <Form.Label>Processor</Form.Label>
                <Form.Control
                  className="input-control"
                  as="select"
                  value={options.hardware}
                  disabled={busy}
                  onChange={(e) =>
                    setOptions({ ...options, hardware: e.target.value })
                  }
                >
                  <option value="auto">Prefer GPU, otherwise CPU</option>
                  <option value="cpu">CPU</option>
                  <option
                    value="gpu"
                    disabled={
                      !chosenFormats.length ||
                      chosenFormats.some((f) => !f.gpu.length)
                    }
                  >
                    GPU
                  </option>
                </Form.Control>
                <Form.Text className="text-muted">
                  {chosenFormats
                    .map(
                      (f) => `${f.label}: ${encodersFor(f)[0] ?? "unavailable"}`
                    )
                    .join(" · ")}
                </Form.Text>
              </Form.Group>
            </Row>
            <p className="text-muted">
              Worker:{" "}
              {resolvedBackend === "remote"
                ? "remote tagging worker"
                : resolvedBackend === "local"
                  ? "this StashBooru server"
                  : "checking availability…"}
            </p>
            {loadingCapabilities && (
              <p>
                <Spinner animation="border" size="sm" /> Probing codec and
                hardware support…
              </p>
            )}
            {capabilityError && (
              <Alert variant="warning">{capabilityError}</Alert>
            )}
            {workerNotice && <Alert variant="info">{workerNotice}</Alert>}
            <Button
              size="sm"
              variant="secondary"
              className="mb-3"
              disabled={busy || loadingCapabilities}
              onClick={() => setCapabilityAttempt((v) => v + 1)}
            >
              Recheck worker
            </Button>
            {options.format === "auto" && (
              <div className="mb-3">
                {loadingPreview ? <span>Reading input formats…</span> : null}
                {previewError && (
                  <Alert variant="warning">{previewError}</Alert>
                )}
                {!loadingPreview &&
                  plans.map((plan, index) => (
                    <div
                      key={`${plan.input}-${plan.output}-${index}`}
                      className={plan.error ? "text-warning" : ""}
                    >
                      {plan.count} {plan.count === 1 ? "file" : "files"}:{" "}
                      {plan.error ||
                        `${plan.input.toUpperCase()} → ${formats.find((f) => f.id === plan.output)?.label ?? plan.output} · quality ${useEncodingDefaults ? plan.quality : options.quality}, effort ${useEncodingDefaults ? plan.effort : options.effort}`}
                      {!plan.error &&
                      !loadingCapabilities &&
                      !formats.find((f) => f.id === plan.output)?.available
                        ? " — unavailable on this worker"
                        : ""}
                    </div>
                  ))}
              </div>
            )}
            <Form.Check
              id="converter-saved-encoding"
              label="Use saved quality and effort for each input format"
              checked={useEncodingDefaults}
              disabled={busy}
              onChange={(e) => setUseEncodingDefaults(e.target.checked)}
            />
            {!useEncodingDefaults && (
              <Row>
                {(["quality", "effort"] as const)
                  .filter((key) => supports(key))
                  .map((key) => (
                    <Form.Group
                      as={Col}
                      key={key}
                      controlId={`converter-${key}`}
                    >
                      <Form.Label>
                        {key === "quality"
                          ? "Quality (higher = better)"
                          : "Effort (higher = slower)"}
                      </Form.Label>
                      <Form.Control
                        className="text-input"
                        type="number"
                        min={key === "effort" ? 1 : 0}
                        max={key === "effort" ? 9 : 100}
                        step={1}
                        value={options[key]}
                        disabled={busy}
                        onChange={(e) =>
                          setOptions({
                            ...options,
                            [key]: Number(e.target.value),
                          })
                        }
                      />
                    </Form.Group>
                  ))}
              </Row>
            )}
            {supports("quality") && (
              <p className="text-muted">
                Quality ranges from 0 to 100. JPEG XL quality 100 is lossless.
                Equal values across codecs do not imply equal quality. Higher
                effort trades encoding time for compression.
              </p>
            )}
            {kind === "image" && upscalers.length > 0 && (
              <Row>
                <Form.Group as={Col} controlId="converter-upscaler">
                  <Form.Label>Optional upscaling (still images)</Form.Label>
                  <Form.Control
                    as="select"
                    className="input-control"
                    value={options.upscaler ?? ""}
                    disabled={busy}
                    onChange={(e) =>
                      setOptions({
                        ...options,
                        upscaler: e.target.value || undefined,
                        upscaleScale: e.target.value ? 2 : undefined,
                        allowLarger: e.target.value
                          ? true
                          : options.allowLarger,
                      })
                    }
                  >
                    <option value="">None</option>
                    {upscalers.map((u) => (
                      <option
                        key={u.id}
                        value={u.id}
                        disabled={
                          !u.available || (options.hardware === "cpu" && !u.cpu)
                        }
                      >
                        {u.label}
                        {!u.available ? " — not installed on this worker" : ""}
                      </option>
                    ))}
                  </Form.Control>
                  <Form.Text className="text-muted">
                    {upscalers.find((u) => u.id === options.upscaler)?.notice ||
                      "Install models on the selected worker to enable waifu2x or SeedVR2."}
                  </Form.Text>
                </Form.Group>
                {options.upscaler && (
                  <Form.Group as={Col} controlId="converter-upscale-scale">
                    <Form.Label>Scale</Form.Label>
                    <Form.Control
                      as="select"
                      className="input-control"
                      value={options.upscaleScale ?? 2}
                      disabled={busy}
                      onChange={(e) =>
                        setOptions({
                          ...options,
                          upscaleScale: Number(e.target.value),
                        })
                      }
                    >
                      <option value={2}>2×</option>
                      <option value={4}>4×</option>
                    </Form.Control>
                    <Form.Text className="text-muted">
                      Upscale before encoding to the displayed output format.
                      Originals remain restorable in the cache.
                    </Form.Text>
                  </Form.Group>
                )}
              </Row>
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
            {supports("lossless") && (
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
                !canEncode ||
                !selectedIds.length ||
                loadingCapabilities ||
                (options.format === "auto" &&
                  (loadingPreview || !!previewError)) ||
                !Number.isFinite(options.quality) ||
                options.quality < 0 ||
                options.quality > 100 ||
                !Number.isInteger(options.effort) ||
                options.effort < 1 ||
                options.effort > 9
              }
              onClick={() =>
                void action({
                  action: "start",
                  options,
                  useQuality: true,
                  useEncodingDefaults,
                  targets: selectedIds.map((id) => ({ kind, id: Number(id) })),
                })
              }
            >
              Convert{" "}
              {selectedIds.length === 1
                ? "file"
                : `${selectedIds.length} files`}
            </Button>
            {state?.job && (
              <div role="status" className="mb-3">
                <strong>
                  Job #{state.job.jobID}: {state.job.status}
                </strong>{" "}
                — {state.job.items.length}/{state.job.total} processed
                {state.job.backend && ` · ${state.job.backend} worker`}
                {state.job.notice && (
                  <p className="text-muted">{state.job.notice}</p>
                )}
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
            {state && state.latestStats.converted > 0 && !running && (
              <p className="text-muted">
                Latest conversion batch: {state.latestStats.converted} converted
                · {bytes(state.latestStats.savedBytes)} saved (
                {state.latestStats.savedPercent.toFixed(1)}%) ·{" "}
                {bytes(state.latestStats.averageSavedBytes)} average per file.
              </p>
            )}
          </Tab>
          <Tab eventKey="statistics" title="Statistics & restore">
            {state && (
              <>
                <h5>All conversions</h5>
                <StatsView value={state.stats} />
                {state.batchStats.converted > 0 && (
                  <>
                    <h5>Latest selected batch</h5>
                    <StatsView value={state.batchStats} />
                  </>
                )}
                <p className="text-muted">
                  Net savings include retained originals and may be negative
                  until cache eviction. Repeated conversions are combined per
                  file. These figures cover media files and originals; generated
                  previews, temporary files and filesystem compression are
                  excluded.
                </p>
              </>
            )}
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
                  className="text-input"
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
            <h5>Conversion history</h5>
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
                        <div className="text-break">
                          {before?.id ? (
                            <a href={`/image/file/${before.id}/open`}>
                              {before.basename}
                            </a>
                          ) : (
                            before?.basename
                          )}
                        </div>
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
          </Tab>
        </Tabs>
      </Modal.Body>
      <Modal.Footer>
        <span className="mr-auto text-muted">
          {state?.job && `Job #${state.job.jobID}: ${state.job.status}. `}
          Jobs continue when this window is closed.
        </span>
        <Button variant="secondary" onClick={onHide}>
          Close
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
