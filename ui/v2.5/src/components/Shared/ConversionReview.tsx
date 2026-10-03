import React, { useEffect, useState } from "react";
import { Alert, Button, Form, Modal, Table } from "react-bootstrap";
import { Link } from "react-router-dom";
import {
  ReferenceComparison,
  ReferenceComparisonMode,
} from "src/hooks/Lightbox/ReferenceComparison";
import { ILightboxImage } from "src/hooks/Lightbox/types";
import "./conversionReview.scss";
import {
  ConversionDiskReservation,
  ConversionEstimate,
  ConversionTrial,
  conversionEndpoint,
  TrialStats,
} from "./mediaConversion";

export function reviewBytes(value: number) {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const index = Math.min(
    units.length - 1,
    Math.max(0, Math.floor(Math.log2(Math.max(1, Math.abs(value))) / 10))
  );
  return `${(value / 1024 ** index).toLocaleString(undefined, { maximumFractionDigits: 2 })} ${units[index]}`;
}

function trialURL(trial: ConversionTrial, file: "source" | "output") {
  return `${conversionEndpoint}?trialID=${encodeURIComponent(trial.id)}&file=${file}`;
}

function TrialComparison({
  trial,
  onHide,
}: {
  trial: ConversionTrial;
  onHide: () => void;
}) {
  const [mode, setMode] =
    useState<Exclude<ReferenceComparisonMode, "selected">>("both");
  const [zoom, setZoom] = useState(1);
  const before = trial.before.image ?? trial.before.video;
  const reference: ILightboxImage = {
    title: "Source",
    paths: { image: trialURL(trial, "source") },
    visual_files: [
      {
        path: before?.path ?? "",
        size: before?.size,
        width: before?.width,
        height: before?.height,
      },
    ],
  };
  const output: ILightboxImage = {
    title: "Trial output",
    paths: { image: trialURL(trial, "output") },
    visual_files: [
      {
        path: `trial.${trial.format.extension}`,
        size: trial.result.size,
        width: trial.result.width,
        height: trial.result.height,
      },
    ],
  };
  const still =
    trial.target.kind === "image" &&
    trial.result.frames === 1 &&
    trial.format.family !== "video";
  return (
    <Modal show onHide={onHide} size="xl" scrollable>
      <Modal.Header closeButton>
        <Modal.Title>Compare verified trial</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <p>
          Comparison uses browser decoding. Unsupported formats need an external
          viewer. Timing checks and notices remain separate from visual
          differences.
        </p>
        {still ? (
          <>
            <Form.Group controlId="trial-comparison-mode">
              <Form.Label>Comparison view</Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={mode}
                onChange={(e) =>
                  setMode(
                    e.target.value as Exclude<
                      ReferenceComparisonMode,
                      "selected"
                    >
                  )
                }
              >
                <option value="both">Side by side</option>
                <option value="slider">Wipe</option>
                <option value="blink">Blink</option>
                <option value="difference">Difference map and boxes</option>
              </Form.Control>
            </Form.Group>
            <Button
              size="sm"
              variant="secondary"
              className="mb-2"
              onClick={() => setZoom(1)}
            >
              Reset zoom
            </Button>
            <div
              className="conversion-trial-comparison"
              style={{ height: "60vh", minHeight: "20rem" }}
            >
              <ReferenceComparison
                referenceImage={reference}
                selectedImage={output}
                mode={mode}
                direction="left"
                animateSelected={false}
                zoom={zoom}
                resetPosition
                setZoom={setZoom}
              />
            </div>
          </>
        ) : (
          <Alert variant="info">
            Animation/Video comparison uses the complete source and trial files.
            This view does not claim synchronized playback or whole-animation
            equality from a still preview.
          </Alert>
        )}
        <div className="d-flex flex-wrap mt-3">
          <a
            className="btn btn-secondary mr-2 mb-2"
            href={trialURL(trial, "source")}
            target="_blank"
            rel="noreferrer"
          >
            Open source file
          </a>
          <a
            className="btn btn-secondary mb-2"
            href={trialURL(trial, "output")}
            target="_blank"
            rel="noreferrer"
          >
            Open trial file
          </a>
        </div>
      </Modal.Body>
    </Modal>
  );
}

export const ConversionReview: React.FC<{
  trials: ConversionTrial[];
  stats?: TrialStats;
  estimate?: ConversionEstimate;
  reservation?: ConversionDiskReservation;
  total: number;
  offset: number;
  onPage: (offset: number) => void;
  busy: boolean;
  onApply: (ids: string[]) => void;
  onDiscard: (ids: string[]) => void;
  onComparisonChange: (comparing: boolean) => void;
}> = ({
  trials,
  stats,
  estimate,
  reservation,
  total,
  offset,
  onPage,
  busy,
  onApply,
  onDiscard,
  onComparisonChange,
}) => {
  const [selected, setSelected] = useState<string[]>([]);
  const [comparison, setComparison] = useState<ConversionTrial>();
  const eligible = (trial: ConversionTrial) =>
    trial.status === "verified" &&
    trial.retained &&
    new Date(trial.expiresAt).getTime() > Date.now();
  useEffect(() => {
    setSelected((old) =>
      old.filter((id) => {
        const trial = trials.find((t) => t.id === id);
        return (
          !trial ||
          (trial.status === "verified" &&
            trial.retained &&
            new Date(trial.expiresAt).getTime() > Date.now())
        );
      })
    );
  }, [trials]);
  return (
    <>
      <h5>Estimate</h5>
      <p>
        Up to 24 complete sample conversions cover input format, animation
        status (including uninspected files) and size bands, using size
        quantiles within each band. Samples are deleted after measurement. No
        files are activated.
      </p>
      {estimate && (
        <>
          <Alert variant={estimate.covered ? "info" : "warning"}>
            {estimate.sampled}/{estimate.total} files sampled.{" "}
            {estimate.covered ? (
              <>
                Estimated eligible savings:{" "}
                {reviewBytes(estimate.estimatedSavedBytes)}. Observed sample
                envelope: {reviewBytes(estimate.observedLowBytes)}–
                {reviewBytes(estimate.observedHighBytes)}.
              </>
            ) : (
              "Some strata/targets failed or were not covered; no whole-selection savings estimate is available."
            )}{" "}
            The observed envelope is not a confidence interval or a guarantee;
            verify every output before applying.
          </Alert>
          <details>
            <summary>Sample coverage</summary>
            <Table responsive size="sm">
              <thead>
                <tr>
                  <th>Stratum</th>
                  <th>Files</th>
                  <th>Sampled</th>
                  <th>Failed</th>
                </tr>
              </thead>
              <tbody>
                {estimate.strata.map((s) => (
                  <tr key={s.name}>
                    <td>{s.name}</td>
                    <td>{s.count}</td>
                    <td>{s.sampled}</td>
                    <td>{s.failed}</td>
                  </tr>
                ))}
              </tbody>
            </Table>
          </details>
        </>
      )}
      {reservation && (
        <p className="text-muted">
          Disk reservation: {reviewBytes(reservation.requiredBytes)} (
          {reviewBytes(reservation.outputBytes)} proposed output,{" "}
          {reviewBytes(reservation.stagingBytes)} staging,{" "}
          {reviewBytes(reservation.backupBytes)} original backups).
          Output/staging reservations use twice the source bytes each. This is a
          conservative planning estimate; codec temporary usage and actual
          output sizes can vary.
        </p>
      )}
      <h5>Verified trials</h5>
      <p>
        Trials retain the measured output without changing active fingerprints,
        metadata or restore caches. Apply rechecks source identity/content,
        options, savings minimums, converter version and output checksum.
        Changes require a new trial.
      </p>
      {stats && (
        <p>
          {stats.verified} eligible · {stats.skipped} skipped · {stats.failed}{" "}
          failed/cancelled · {stats.applied} applied. Potential savings:{" "}
          {reviewBytes(stats.potentialSavedBytes)} (
          {stats.weightedSavedPercent.toFixed(1)}%, weighted by source bytes).
          Trial cache: {reviewBytes(stats.cacheBytes)}. Potential savings are
          realized only after Apply; original backups consume additional space
          until evicted.
        </p>
      )}
      <div className="d-flex flex-wrap mb-3">
        <Button
          variant="secondary"
          className="mr-2 mb-2"
          disabled={busy || !trials.some(eligible)}
          onClick={() =>
            setSelected((old) => [
              ...new Set([...old, ...trials.filter(eligible).map((t) => t.id)]),
            ])
          }
        >
          Select eligible trials on this page
        </Button>
        <Button
          className="mr-2 mb-2"
          disabled={busy || !selected.length}
          onClick={() => onApply(selected)}
        >
          Apply {selected.length} selected trials
        </Button>
        <Button
          variant="secondary"
          className="mr-2 mb-2"
          disabled={busy || !selected.length}
          onClick={() => onDiscard(selected)}
        >
          Discard selected trials
        </Button>
        <Button
          variant="link"
          className="mb-2"
          disabled={busy || !selected.length}
          onClick={() => setSelected([])}
        >
          Clear selection
        </Button>
      </div>
      <Table responsive size="sm" className="conversion-trial-review">
        <thead>
          <tr>
            <th>Select / media</th>
            <th>Output / options</th>
            <th>Measured bytes / savings</th>
            <th>Verification / status</th>
            <th>Review</th>
          </tr>
        </thead>
        <tbody>
          {trials.map((trial) => {
            const before = trial.before.image ?? trial.before.video;
            const saved = (before?.size ?? 0) - trial.result.size;
            return (
              <tr key={trial.id} data-trial-id={trial.id}>
                <td>
                  <Form.Check
                    type="checkbox"
                    aria-label={`Select trial ${trial.id}`}
                    checked={selected.includes(trial.id)}
                    disabled={busy || !eligible(trial)}
                    onChange={(e) =>
                      setSelected((old) =>
                        e.target.checked
                          ? [...old, trial.id]
                          : old.filter((id) => id !== trial.id)
                      )
                    }
                  />
                  <Link
                    to={`/${trial.target.kind === "image" ? "images" : "scenes"}/${trial.target.id}`}
                  >
                    {before?.basename ??
                      `${trial.target.kind} #${trial.target.id}`}
                  </Link>
                  <small className="d-block">
                    Expires {new Date(trial.expiresAt).toLocaleString()}
                  </small>
                </td>
                <td>
                  {trial.format.label}
                  <small className="d-block">
                    Quality {trial.options.quality}, effort{" "}
                    {trial.options.effort}, {trial.options.hardware}, decode
                    speed {trial.options.decodingSpeed ?? 0}
                    {trial.options.lossless ? ", lossless" : ""}
                  </small>
                  <small>
                    {trial.backend} · {trial.result.encoder} ·{" "}
                    {trial.result.seconds.toFixed(1)}s
                  </small>
                </td>
                <td>
                  {trial.verified ? (
                    <>
                      {(before?.size ?? 0).toLocaleString()} →{" "}
                      {trial.result.size.toLocaleString()} bytes
                      <strong className="d-block">
                        {saved.toLocaleString()} bytes (
                        {before?.size
                          ? ((100 * saved) / before.size).toFixed(1)
                          : "0"}
                        %)
                      </strong>
                    </>
                  ) : (
                    "—"
                  )}
                </td>
                <td>
                  {trial.verified ? "Verified" : "Unverified"} · {trial.status}
                  {!trial.retained && trial.status !== "applied"
                    ? " · output unavailable"
                    : ""}
                  {trial.error && (
                    <small className="d-block text-warning">
                      {trial.error}
                    </small>
                  )}
                  <details>
                    <summary>Options and limitations</summary>
                    {trial.notices.map((n) => (
                      <p key={n}>{n}</p>
                    ))}
                    <small>
                      Minimums: {trial.savings.minimumSavedBytes} bytes and{" "}
                      {trial.savings.minimumSavedPercent}%.
                    </small>
                  </details>
                </td>
                <td>
                  <Button
                    size="sm"
                    variant="secondary"
                    className="mb-1"
                    disabled={
                      busy ||
                      !trial.retained ||
                      !trial.verified ||
                      new Date(trial.expiresAt).getTime() <= Date.now()
                    }
                    onClick={() => {
                      setComparison(trial);
                      onComparisonChange(true);
                    }}
                  >
                    Compare
                  </Button>
                  <Button
                    size="sm"
                    variant="link"
                    disabled={
                      busy ||
                      trial.status === "encoding" ||
                      trial.status === "applying"
                    }
                    onClick={() => onDiscard([trial.id])}
                  >
                    Discard
                  </Button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </Table>
      {!trials.length && (
        <p className="text-muted">
          No saved trials. Run a verified trial from the conversion options.
        </p>
      )}
      <div className="mb-2">
        <Button
          size="sm"
          variant="secondary"
          disabled={busy || offset === 0}
          onClick={() => onPage(Math.max(0, offset - 50))}
        >
          Previous trials
        </Button>
        <span className="mx-2">
          {total
            ? `${offset + 1}–${Math.min(offset + trials.length, total)} of ${total}`
            : "0 trials"}
        </span>
        <Button
          size="sm"
          variant="secondary"
          disabled={busy || offset + trials.length >= total}
          onClick={() => onPage(offset + 50)}
        >
          Next trials
        </Button>
      </div>
      {comparison && (
        <TrialComparison
          trial={comparison}
          onHide={() => {
            setComparison(undefined);
            onComparisonChange(false);
          }}
        />
      )}
    </>
  );
};
