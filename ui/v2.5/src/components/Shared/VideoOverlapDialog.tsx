import { ProcessingBackendOptions } from "./ProcessingBackendOptions";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Col,
  Form,
  Modal,
  Row,
  Table,
} from "react-bootstrap";
import { Link } from "react-router-dom";
import { conversionResponse } from "./mediaConversion";
import {
  KeeperCriterion,
  keeperCriteria,
  KeeperPreferences,
  mappedVideoTime,
  normalizeKeeperPreferences,
  overlapClassLabels,
  suggestVideoKeeper,
  videoOverlapActive,
  VideoOverlapConfig,
  videoOverlapEndpoint,
  VideoOverlapResponse,
  VideoOverlapRow,
  videoRanges,
  videoTime,
} from "./videoOverlap";

const preferenceKey = "stash.video-overlap.keeper.v1";

function storedPreferences() {
  try {
    return normalizeKeeperPreferences(
      JSON.parse(localStorage.getItem(preferenceKey) ?? "null")
    );
  } catch {
    return normalizeKeeperPreferences(null);
  }
}

function MediaDetails({ info }: { info: VideoOverlapRow["a"] }) {
  return (
    <>
      <Link to={`/scenes/${info.id}`} target="_blank" rel="noreferrer">
        {info.title || `Video #${info.id}`}
      </Link>
      <div className="small">
        {info.media.width}×{info.media.height} · {info.media.codec} ·{" "}
        {(info.media.bitRate / 1000).toFixed(0)} kb/s ·{" "}
        {videoTime(info.media.duration)}
      </div>
      <div className="small">
        {info.media.audioTracks} audio / {info.media.subtitleTracks} subtitle
        tracks · {info.metadataFields}/8 metadata fields
      </div>
    </>
  );
}

function SegmentComparison({
  row,
  onHide,
}: {
  row: VideoOverlapRow;
  onHide: () => void;
}) {
  const left = useRef<HTMLVideoElement>(null),
    right = useRef<HTMLVideoElement>(null);
  const [segment, setSegment] = useState(0);
  const [error, setError] = useState("");
  const [audio, setAudio] = useState("mute");
  const [ready, setReady] = useState({ a: false, b: false });
  const interval = row.match.intervals[segment];
  const stop = useCallback(() => {
    left.current?.pause();
    right.current?.pause();
  }, []);
  const seek = useCallback(() => {
    stop();
    if (left.current?.readyState) left.current.currentTime = interval.a.start;
    if (right.current?.readyState)
      right.current.currentTime = mappedVideoTime(interval.a.start, interval);
  }, [interval, stop]);
  useEffect(() => {
    seek();
  }, [seek]);
  useEffect(() => {
    const a = left.current,
      b = right.current;
    return () => {
      for (const v of [a, b]) {
        if (v) {
          v.pause();
          v.removeAttribute("src");
          v.load();
        }
      }
    };
  }, []);
  function sync() {
    const a = left.current,
      b = right.current;
    if (!a || !b) return;
    if (a.currentTime >= interval.a.end || a.ended) {
      stop();
      return;
    }
    if (a.currentTime < interval.a.start) a.currentTime = interval.a.start;
    const time = mappedVideoTime(a.currentTime, interval);
    if (Math.abs(b.currentTime - time) > 0.25) b.currentTime = time;
    b.playbackRate = a.playbackRate;
  }
  async function play() {
    seek();
    setError("");
    try {
      await Promise.all([left.current?.play(), right.current?.play()]);
    } catch {
      stop();
      setError(
        "Playback failed. Open the native Video links to inspect its supported streams."
      );
    }
  }
  return (
    <Modal show onHide={onHide} size="xl" className="video-overlap-comparison">
      <Modal.Header closeButton>
        <Modal.Title>Compare video segments</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        {error && <Alert variant="danger">{error}</Alert>}
        <Form.Group controlId="overlap-segment">
          <Form.Label>Matched interval</Form.Label>
          <Form.Control
            as="select"
            value={segment}
            onChange={(e) => setSegment(Number(e.currentTarget.value))}
          >
            {row.match.intervals.map((m, i) => (
              <option key={i} value={i}>
                A {videoRanges([m.a])} ↔ B {videoRanges([m.b])}
              </option>
            ))}
          </Form.Control>
        </Form.Group>
        <Row>
          <Col sm={6}>
            <MediaDetails info={row.a} />
            <video
              ref={left}
              src={`scene/${row.a.id}/stream.mp4`}
              controls
              playsInline
              preload="metadata"
              muted={audio !== "a"}
              onLoadedMetadata={() => {
                setReady((r) => ({ ...r, a: true }));
                seek();
              }}
              onTimeUpdate={sync}
              onSeeking={sync}
              onPause={() => right.current?.pause()}
              onPlay={() => {
                void right.current?.play().catch(() => {
                  stop();
                  setError("The second stream could not play.");
                });
              }}
              onRateChange={sync}
              onError={() => {
                stop();
                setError("The first video stream could not load.");
              }}
            />
          </Col>
          <Col sm={6}>
            <MediaDetails info={row.b} />
            <video
              ref={right}
              src={`scene/${row.b.id}/stream.mp4`}
              playsInline
              preload="metadata"
              muted={audio !== "b"}
              onLoadedMetadata={() => {
                setReady((r) => ({ ...r, b: true }));
                seek();
              }}
              onError={() => {
                stop();
                setError("The second video stream could not load.");
              }}
            />
          </Col>
        </Row>
        <div className="d-flex flex-wrap mb-3">
          <Button
            disabled={!ready.a || !ready.b}
            onClick={() => void play()}
            className="mr-2 mb-2"
          >
            Play matched segment
          </Button>
          <Button variant="secondary" onClick={stop} className="mr-2 mb-2">
            Pause both
          </Button>
          <Form.Control
            as="select"
            className="video-overlap-audio"
            aria-label="Listen to audio"
            value={audio}
            onChange={(e) => setAudio(e.currentTarget.value)}
          >
            <option value="mute">Mute both</option>
            <option value="a">Listen to A</option>
            <option value="b">Listen to B</option>
          </Form.Control>
        </div>
        <p className="small">
          {row.match.confidence}. This is an evidence grade, not a duplicate
          probability.
        </p>
        <p className="small">
          {interval.samples} matched samples · {interval.distinctFrames}{" "}
          distinct frames · pHash {interval.meanDistance.toFixed(1)} / dHash{" "}
          {interval.meanDHashDistance.toFixed(1)} mean distance ·{" "}
          {(100 * interval.support).toFixed(0)}% sample support. Boundary
          tolerance ±{row.match.tolerance.toFixed(2)} s.
        </p>
        <p className="small">
          Gaps inside this mapping: A {videoRanges(interval.gapsA)}; B{" "}
          {videoRanges(interval.gapsB)}.
        </p>
        <p>{row.match.audio}</p>
        <p className="small">
          Unmatched A: {videoRanges(row.match.unmatchedA)}
          <br />
          Unmatched B: {videoRanges(row.match.unmatchedB)}
        </p>
        <p className="small">
          Sampling does not establish whole-video equality. Playback stops at
          the selected segment; native Video links open the full files and their
          tracks.
        </p>
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" onClick={onHide}>
          Close comparison
        </Button>
      </Modal.Footer>
    </Modal>
  );
}

export function VideoOverlapDialog({
  referenceId,
  onHide,
}: {
  referenceId?: number;
  onHide: () => void;
}) {
  const [data, setData] = useState<VideoOverlapResponse>();
  const [config, setConfig] = useState<VideoOverlapConfig>({
    backend: "auto",
    sampleSeconds: 1,
    audioDigest: false,
  });
  const [reference, setReference] = useState(
    referenceId ? String(referenceId) : ""
  );
  const [jobId, setJobId] = useState("");
  const [reviewId, setReviewId] = useState("");
  const [page, setPage] = useState(1);
  const [distance, setDistance] = useState(8),
    [minimum, setMinimum] = useState(4),
    [candidates, setCandidates] = useState(200);
  const [submitting, setSubmitting] = useState(false),
    [error, setError] = useState("");
  const [comparison, setComparison] = useState<VideoOverlapRow>();
  const [prefs, setPrefs] = useState<KeeperPreferences>(storedPreferences);
  const [codecText, setCodecText] = useState(() => prefs.codecs.join(", "));
  const configured = useRef(false);
  const active = videoOverlapActive(data?.job);
  const validReference = /^\d+$/.test(reference) && Number(reference) > 0;

  useEffect(() => {
    let alive = true,
      timer: ReturnType<typeof setTimeout>;
    const controller = new AbortController();
    async function refresh() {
      try {
        const params = new URLSearchParams({
          reference,
          id: jobId,
          reviewID: reviewId,
          page: String(page),
        });
        const result: VideoOverlapResponse = await fetch(
          `${videoOverlapEndpoint}?${params}`,
          { signal: controller.signal }
        ).then(conversionResponse<VideoOverlapResponse>);
        if (!alive) return;
        setData(result);
        if (!configured.current) {
          configured.current = true;
          setConfig(result.config);
        }
        timer = setTimeout(
          () => void refresh(),
          videoOverlapActive(result.job) ? 700 : 2500
        );
      } catch (e) {
        if (alive) {
          setError(e instanceof Error ? e.message : String(e));
          timer = setTimeout(() => void refresh(), 5000);
        }
      }
    }
    void refresh();
    return () => {
      alive = false;
      controller.abort();
      clearTimeout(timer);
    };
  }, [reference, jobId, reviewId, page]);

  useEffect(() => {
    if (
      comparison &&
      data?.review?.rows.some(
        (r) => r.match.b === comparison.match.b && r.stale
      )
    ) {
      setComparison(undefined);
      setError("The compared primary file changed; run another index/search.");
    }
  }, [comparison, data]);
  useEffect(() => {
    try {
      localStorage.setItem(preferenceKey, JSON.stringify(prefs));
    } catch {
      /* Browser storage may be disabled. */
    }
  }, [prefs]);

  async function action(body: object) {
    setSubmitting(true);
    setError("");
    try {
      const result = await fetch(videoOverlapEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      }).then(conversionResponse<{ id?: string; jobID?: number }>);
      if (result.id) {
        setJobId(result.id);
        if ((body as { action: string }).action === "search") {
          setReviewId(result.id);
          setPage(1);
        }
      }
      return result;
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }
  const validConfig =
    Number.isFinite(config.sampleSeconds) &&
    config.sampleSeconds >= 0.5 &&
    config.sampleSeconds <= 10;
  const validSearch =
    Number.isInteger(distance) &&
    distance >= 0 &&
    distance <= 8 &&
    minimum >= 2 &&
    minimum <= 120 &&
    Number.isInteger(candidates) &&
    candidates >= 1 &&
    candidates <= 1000;
  const disabled = submitting || active;
  const review = data?.review;
  return (
    <>
      <Modal
        show
        onHide={onHide}
        size="xl"
        className={`video-overlap-review ${comparison ? "d-none" : ""}`}
      >
        <Modal.Header closeButton>
          <Modal.Title>Video overlap review</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p>
            {data?.indexed ?? 0} stored signatures · {data?.totalVideos ?? 0}{" "}
            Videos. Indexing reads active files and saves lightweight
            timestamped fingerprints. Existing metadata and files stay intact.
          </p>
          {error && <Alert variant="danger">{error}</Alert>}
          <Row>
            <Col md={4}>
              <Form.Group controlId="video-overlap-reference">
                <Form.Label>Reference Video ID</Form.Label>
                <Form.Control
                  type="number"
                  min="1"
                  value={reference}
                  onChange={(e) => {
                    setReference(e.currentTarget.value);
                    setReviewId("");
                    setPage(1);
                  }}
                />
              </Form.Group>
            </Col>
            <Col md={4}>
              <Form.Group controlId="video-overlap-backend">
                <Form.Label>Run on</Form.Label>
                <Form.Control
                  as="select"
                  value={config.backend}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      backend: e.currentTarget
                        .value as VideoOverlapConfig["backend"],
                    })
                  }
                >
                  <ProcessingBackendOptions />
                </Form.Control>
              </Form.Group>
            </Col>
            <Col md={4}>
              <Form.Group controlId="video-overlap-step">
                <Form.Label>Sample every (seconds)</Form.Label>
                <Form.Control
                  type="number"
                  min="0.5"
                  max="10"
                  step="0.5"
                  value={config.sampleSeconds}
                  onChange={(e) =>
                    setConfig({
                      ...config,
                      sampleSeconds: Number(e.currentTarget.value),
                    })
                  }
                />
              </Form.Group>
            </Col>
          </Row>
          <Form.Check
            id="video-overlap-audio-digest"
            type="checkbox"
            label="Include a decoded digest of the first whole audio track (optional)"
            checked={config.audioDigest}
            onChange={(e) =>
              setConfig({ ...config, audioDigest: e.currentTarget.checked })
            }
          />
          <div className="d-flex flex-wrap my-3">
            <Button
              variant="secondary"
              className="mr-2 mb-2"
              disabled={disabled || !validConfig}
              onClick={() => void action({ action: "configure", config })}
            >
              Save index settings
            </Button>
            <Button
              variant="secondary"
              className="mr-2 mb-2"
              disabled={disabled || !validReference || !validConfig}
              onClick={() =>
                void action({
                  action: "index",
                  config,
                  ids: [Number(reference)],
                })
              }
            >
              Index reference
            </Button>
            <Button
              variant="secondary"
              className="mr-2 mb-2"
              disabled={disabled || !validConfig}
              onClick={() =>
                void action({ action: "index", config, all: true })
              }
            >
              Index / resume library
            </Button>
            <Button
              variant="secondary"
              className="mr-2 mb-2"
              disabled={disabled || !validReference || !validConfig}
              onClick={() =>
                void action({
                  action: "index",
                  config,
                  ids: [Number(reference)],
                  force: true,
                })
              }
            >
              Force reindex reference
            </Button>
          </div>
          <p className="small">
            Jobs use these settings; Save keeps them as the default. Unchanged
            entries are skipped. Up to 3,600 existing frames per Video; longer
            Videos use a wider grid. Remote workers must support Video frame
            indexing. Changing the decoder, settings or source file requires
            reindexing.
          </p>
          <Row>
            <Col md={4}>
              <Form.Group controlId="video-overlap-distance">
                <Form.Label>Maximum pHash distance</Form.Label>
                <Form.Control
                  type="number"
                  min="0"
                  max="8"
                  value={distance}
                  onChange={(e) => setDistance(Number(e.currentTarget.value))}
                />
              </Form.Group>
            </Col>
            <Col md={4}>
              <Form.Group controlId="video-overlap-minimum">
                <Form.Label>Minimum segment (seconds)</Form.Label>
                <Form.Control
                  type="number"
                  min="2"
                  max="120"
                  value={minimum}
                  onChange={(e) => setMinimum(Number(e.currentTarget.value))}
                />
              </Form.Group>
            </Col>
            <Col md={4}>
              <Form.Group controlId="video-overlap-candidates">
                <Form.Label>Candidate limit</Form.Label>
                <Form.Control
                  type="number"
                  min="1"
                  max="1000"
                  value={candidates}
                  onChange={(e) => setCandidates(Number(e.currentTarget.value))}
                />
              </Form.Group>
            </Col>
          </Row>
          <Button
            disabled={
              disabled || !validReference || !validSearch || !validConfig
            }
            onClick={() =>
              void action({
                action: "search",
                config,
                reference: Number(reference),
                options: {
                  maxDistance: distance,
                  minimumSeconds: minimum,
                  maxCandidates: candidates,
                },
              })
            }
          >
            Find overlapping segments
          </Button>
          {data?.job && (
            <Alert
              variant={data.job.error ? "danger" : "info"}
              className="mt-3"
            >
              {data.job.action}: {data.job.status} · {data.job.processed}/
              {data.job.total} · {data.job.indexed} indexed, {data.job.skipped}{" "}
              current, {data.job.failed} failed. {data.job.backend}{" "}
              {data.job.error}
              {active && (
                <Button
                  variant="secondary"
                  className="ml-2"
                  onClick={() =>
                    void action({ action: "cancel", id: data.job!.id })
                  }
                  disabled={submitting}
                >
                  Cancel job
                </Button>
              )}
            </Alert>
          )}
          <details className="my-3">
            <summary>Recent jobs / saved checkpoints</summary>
            {data?.jobs.map((j) => (
              <div key={j.id} className="my-2">
                <span>
                  {j.action}: {j.status} · {j.processed}/{j.total}
                </span>
                {j.action === "index" &&
                  !videoOverlapActive(j) &&
                  (j.status !== "complete" || j.failed > 0) && (
                    <Button
                      variant="secondary"
                      size="sm"
                      className="ml-2"
                      disabled={disabled}
                      onClick={() =>
                        void action({ action: "resume", id: j.id })
                      }
                    >
                      Resume checkpoint
                    </Button>
                  )}
                {j.error && <div className="text-danger">{j.error}</div>}
                {j.items
                  .filter((i) => i.error)
                  .map((i) => (
                    <div key={i.id} className="small text-danger">
                      <Link to={`/scenes/${i.id}`}>Video #{i.id}</Link>:{" "}
                      {i.error}
                    </div>
                  ))}
              </div>
            ))}
          </details>
          <hr />
          <Row>
            <Col md={6}>
              <Form.Group controlId="video-overlap-keeper">
                <Form.Label>Keeper suggestion: prioritize</Form.Label>
                <Form.Control
                  as="select"
                  value={prefs.first}
                  onChange={(e) =>
                    setPrefs({
                      ...prefs,
                      first: e.currentTarget.value as KeeperCriterion,
                    })
                  }
                >
                  {Object.entries(keeperCriteria).map(([key, label]) => (
                    <option key={key} value={key}>
                      {label}
                    </option>
                  ))}
                </Form.Control>
              </Form.Group>
            </Col>
            <Col md={6}>
              <Form.Group controlId="video-overlap-codecs">
                <Form.Label>Preferred codecs, in order</Form.Label>
                <Form.Control
                  placeholder="av1, hevc, h264"
                  value={codecText}
                  onChange={(e) => {
                    setCodecText(e.currentTarget.value);
                    setPrefs(
                      normalizeKeeperPreferences({
                        ...prefs,
                        codecs: e.currentTarget.value
                          .split(",")
                          .map((v) => v.trim().toLowerCase())
                          .filter(Boolean),
                      })
                    );
                  }}
                />
              </Form.Group>
            </Col>
          </Row>
          <p className="small">
            Suggestions compare your first criterion, then the remaining
            criteria in the displayed order. More bitrate or tracks does not
            prove better quality. Both files may have unique endings,
            soundtracks, subtitles or metadata. This review never deletes or
            merges media.
          </p>
          {review && (
            <>
              <p>
                {review.total} matching pairs · {review.candidates.ids.length}/
                {review.candidates.total} indexed candidates examined ·{" "}
                {review.skipped} stale/unavailable skipped.
              </p>
              {(review.candidates.limited ||
                review.alignmentLimited > 0 ||
                review.rows.some((r) => r.match.limited)) && (
                <Alert variant="warning">
                  Candidate/posting or alignment limits were reached. This
                  result is incomplete; use a higher candidate limit or a more
                  specific reference. {review.candidates.omittedCommonBands}{" "}
                  common fingerprint bands omitted.
                </Alert>
              )}
              {review.errors.length > 0 && (
                <details>
                  <summary>Skipped candidate details</summary>
                  {review.errors.map((e, i) => (
                    <div key={i} className="small">
                      {e}
                    </div>
                  ))}
                </details>
              )}
              {!review.rows.length && (
                <p>
                  No supported intervals on this page. Missing/stale indexes,
                  static/title frames, small segments, large speed changes or
                  crops can prevent a match.
                </p>
              )}
              {review.rows.map((row) => {
                const keeper = suggestVideoKeeper(row, prefs);
                return (
                  <div
                    key={row.match.b}
                    className="video-overlap-pair my-3 p-3"
                    data-overlap-id={row.match.b}
                  >
                    <Badge variant="secondary">
                      {overlapClassLabels[row.match.class] ?? row.match.class}
                    </Badge>
                    <Row className="my-2">
                      <Col sm={6}>
                        <strong>A · </strong>
                        <MediaDetails info={row.a} />
                      </Col>
                      <Col sm={6}>
                        <strong>B · </strong>
                        <MediaDetails info={row.b} />
                      </Col>
                    </Row>
                    <p>
                      Matched coverage A{" "}
                      {(row.match.coverageA * 100).toFixed(1)}% / B{" "}
                      {(row.match.coverageB * 100).toFixed(1)}%. Boundary
                      tolerance ±{row.match.tolerance.toFixed(2)} s.
                    </p>
                    <Table size="sm" responsive>
                      <thead>
                        <tr>
                          <th>A interval</th>
                          <th>B interval</th>
                          <th>Evidence</th>
                        </tr>
                      </thead>
                      <tbody>
                        {row.match.intervals.map((m, i) => (
                          <tr key={i}>
                            <td>{videoRanges([m.a])}</td>
                            <td>{videoRanges([m.b])}</td>
                            <td>
                              {m.samples} samples · pHash{" "}
                              {m.meanDistance.toFixed(1)} ·{" "}
                              {(100 * m.support).toFixed(0)}% support
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </Table>
                    <p className="small">
                      Unmatched A: {videoRanges(row.match.unmatchedA)}
                      <br />
                      Unmatched B: {videoRanges(row.match.unmatchedB)}
                    </p>
                    <p className="small">{row.match.audio}</p>
                    <p className="small">{row.match.evidence}</p>
                    <p className="small">
                      {row.match.confidence}. Evidence grades are not duplicate
                      probabilities.
                    </p>
                    <p>
                      {keeper
                        ? `Preference suggestion: Video #${keeper.id} (${keeper.reason}).`
                        : "Keeper preferences are tied."}{" "}
                      {[
                        "contained-clip",
                        "partial-overlap",
                        "compilation-segments",
                      ].includes(row.match.class) &&
                        " Preserve both while reviewing unmatched ranges and tracks."}
                    </p>
                    {row.stale && <Alert variant="warning">{row.stale}</Alert>}
                    <Button
                      variant="secondary"
                      disabled={!!row.stale || !row.match.intervals.length}
                      onClick={() => setComparison(row)}
                    >
                      Compare matched segments
                    </Button>
                  </div>
                );
              })}
              <div className="d-flex align-items-center">
                <Button
                  variant="secondary"
                  disabled={page <= 1 || submitting}
                  onClick={() => setPage(page - 1)}
                >
                  Previous pairs
                </Button>
                <span className="mx-3">
                  Page {page} / {Math.max(1, Math.ceil(review.total / 10))}
                </span>
                <Button
                  variant="secondary"
                  disabled={page * 10 >= review.total || submitting}
                  onClick={() => setPage(page + 1)}
                >
                  Next pairs
                </Button>
              </div>
            </>
          )}
        </Modal.Body>
        <Modal.Footer>
          <Button variant="secondary" onClick={onHide}>
            Close
          </Button>
        </Modal.Footer>
      </Modal>
      {comparison && (
        <SegmentComparison
          row={comparison}
          onHide={() => setComparison(undefined)}
        />
      )}
    </>
  );
}
