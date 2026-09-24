import React, { useEffect, useRef, useState } from "react";
import cx from "classnames";

import { isVideo } from "src/utils/visualFile";
import { ILightboxImage } from "./types";
import {
  computeImageDifference,
  createComparisonGeometry,
  type ComparisonRect,
  type ImageDifferenceResult,
} from "./imageDifference";

const CLASSNAME = "Lightbox-reference-comparison";
const ZOOM_STEP = 1.1;
const SPLIT_KEY_STEP = 1;
const SPLIT_KEY_LARGE_STEP = 10;
const BLINK_INTERVAL_MS = 450;
const DEFAULT_DIFFERENCE_THRESHOLD = 16;
const DEFAULT_DIFFERENCE_NOISE = 8;

export type ReferenceComparisonMode =
  | "selected"
  | "both"
  | "slider"
  | "blink"
  | "difference";
export type ReferenceComparisonDirection = "left" | "right";

interface IProps {
  referenceImage: ILightboxImage;
  selectedImage: ILightboxImage;
  mode: Exclude<ReferenceComparisonMode, "selected">;
  direction: ReferenceComparisonDirection;
  animateSelected: boolean;
  zoom: number;
  resetPosition: boolean;
  setZoom: (zoom: number) => void;
}

interface IDragState {
  pointerID: number;
  x: number;
  y: number;
  panX: number;
  panY: number;
}

interface IComparisonMetric {
  label: string;
  referenceText: string;
  selectedText: string;
  referenceValue?: number;
  selectedValue?: number;
}

interface INormalizedPixels {
  width: number;
  height: number;
  reference: Uint8ClampedArray;
  selected: Uint8ClampedArray;
}

function comparisonSource(image: ILightboxImage) {
  return image.paths.image ?? image.paths.preview ?? image.paths.thumbnail ?? "";
}

const ComparisonMedia: React.FC<{ image: ILightboxImage }> = ({ image }) => {
  const source = comparisonSource(image);
  const video = isVideo(image.visual_files?.[0] ?? {});

  if (video) {
    return (
      <video
        className={`${CLASSNAME}-media`}
        src={source}
        loop
        autoPlay
        muted
        playsInline
        draggable={false}
      />
    );
  }

  return (
    <img
      className={`${CLASSNAME}-media`}
      src={source}
      alt=""
      draggable={false}
    />
  );
};

const SelectedComparisonMedia: React.FC<{
  image: ILightboxImage;
  direction: ReferenceComparisonDirection;
  animate: boolean;
}> = ({ image, direction, animate }) => {
  const source = comparisonSource(image);
  const key = image.id ?? source;
  return (
    <div
      key={key}
      className={cx(`${CLASSNAME}-selected-media`, {
        [`${CLASSNAME}-selected-enter-left`]: animate && direction === "left",
        [`${CLASSNAME}-selected-enter-right`]: animate && direction === "right",
      })}
    >
      <ComparisonMedia image={image} />
    </div>
  );
};

function comparisonFilename(image: ILightboxImage) {
  const path = image.visual_files?.[0]?.path;
  if (!path) return "—";
  return path.split(/[\\/]/).pop() || path;
}

function comparisonFormat(image: ILightboxImage) {
  const file = image.visual_files?.[0];
  const path = file?.path ?? "";
  const filename = path.split(/[\\/]/).pop() ?? "";
  const extension = filename.includes(".")
    ? filename.split(".").pop()?.toUpperCase()
    : undefined;
  const codec = file?.video_codec?.toUpperCase();
  if (extension && codec && extension !== codec) return `${extension} · ${codec}`;
  return extension ?? codec ?? "—";
}

function formatBytes(bytes?: number) {
  if (bytes === undefined || !Number.isFinite(bytes)) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  const digits = value >= 100 || unit === 0 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(digits)} ${units[unit]}`;
}

function formatDuration(seconds?: number | null) {
  if (seconds === undefined || seconds === null || !Number.isFinite(seconds)) {
    return "—";
  }
  const total = Math.max(0, Math.round(seconds));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  return hours > 0
    ? `${hours}:${minutes.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`
    : `${minutes}:${secs.toString().padStart(2, "0")}`;
}

function formatBitrate(bitRate?: number | null) {
  if (bitRate === undefined || bitRate === null || !Number.isFinite(bitRate)) {
    return "—";
  }
  if (bitRate >= 1_000_000) {
    return `${(bitRate / 1_000_000).toFixed(2)} Mbps`;
  }
  if (bitRate >= 1_000) return `${(bitRate / 1_000).toFixed(0)} kbps`;
  return `${bitRate.toFixed(0)} bps`;
}

function resolutionValue(width?: number, height?: number) {
  if (!width || !height) return undefined;
  return width * height;
}

function formatResolution(width?: number, height?: number) {
  if (!width || !height) return "—";
  return `${width}×${height}`;
}

function comparisonMetrics(
  referenceImage: ILightboxImage,
  selectedImage: ILightboxImage
): IComparisonMetric[] {
  const referenceFile = referenceImage.visual_files?.[0];
  const selectedFile = selectedImage.visual_files?.[0];
  const metrics: IComparisonMetric[] = [
    {
      label: "Format",
      referenceText: comparisonFormat(referenceImage),
      selectedText: comparisonFormat(selectedImage),
    },
    {
      label: "Size",
      referenceText: formatBytes(referenceFile?.size),
      selectedText: formatBytes(selectedFile?.size),
      referenceValue: referenceFile?.size,
      selectedValue: selectedFile?.size,
    },
    {
      label: "Resolution",
      referenceText: formatResolution(
        referenceFile?.width,
        referenceFile?.height
      ),
      selectedText: formatResolution(selectedFile?.width, selectedFile?.height),
      referenceValue: resolutionValue(
        referenceFile?.width,
        referenceFile?.height
      ),
      selectedValue: resolutionValue(selectedFile?.width, selectedFile?.height),
    },
  ];

  const referenceHasDuration = referenceFile?.duration !== undefined;
  const selectedHasDuration = selectedFile?.duration !== undefined;
  if (referenceHasDuration || selectedHasDuration) {
    metrics.push({
      label: "Duration",
      referenceText: formatDuration(referenceFile?.duration),
      selectedText: formatDuration(selectedFile?.duration),
      referenceValue: referenceFile?.duration ?? undefined,
      selectedValue: selectedFile?.duration ?? undefined,
    });
  }

  const referenceHasBitrate = referenceFile?.bit_rate !== undefined;
  const selectedHasBitrate = selectedFile?.bit_rate !== undefined;
  if (referenceHasBitrate || selectedHasBitrate) {
    metrics.push({
      label: "Bitrate",
      referenceText: formatBitrate(referenceFile?.bit_rate),
      selectedText: formatBitrate(selectedFile?.bit_rate),
      referenceValue: referenceFile?.bit_rate ?? undefined,
      selectedValue: selectedFile?.bit_rate ?? undefined,
    });
  }

  return metrics;
}

const ComparisonInfo: React.FC<{
  title: string;
  filename: string;
  side: "reference" | "selected";
  metrics: IComparisonMetric[];
  className?: string;
}> = ({ title, filename, side, metrics, className }) => (
  <div className={cx(`${CLASSNAME}-info`, className)}>
    <strong className={`${CLASSNAME}-info-title`}>{title}</strong>
    <span className={`${CLASSNAME}-info-filename`} title={filename}>
      {filename}
    </span>
    {metrics.map((metric) => {
      const comparable =
        metric.referenceValue !== undefined &&
        metric.selectedValue !== undefined;
      const selectedClass =
        side === "selected" && comparable
          ? metric.selectedValue! >= metric.referenceValue!
            ? `${CLASSNAME}-metric-better`
            : `${CLASSNAME}-metric-worse`
          : undefined;
      return (
        <span className={`${CLASSNAME}-metric`} key={metric.label}>
          <span className={`${CLASSNAME}-metric-label`}>{metric.label}</span>
          <span className={selectedClass}>
            {side === "reference" ? metric.referenceText : metric.selectedText}
          </span>
        </span>
      );
    })}
  </div>
);

function clampSplit(value: number) {
  return Math.max(0, Math.min(100, value));
}

function isTemporalMedia(image: ILightboxImage) {
  const file = image.visual_files?.[0];
  return file?.__typename === "VideoFile" || (file?.duration ?? 0) > 0;
}

function loadImage(source: string) {
  return new Promise<HTMLImageElement>((resolve, reject) => {
    const image = new Image();
    image.decoding = "async";
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error(`failed to load ${source}`));
    image.src = source;
  });
}

function drawNormalizedImage(
  image: HTMLImageElement,
  rect: ComparisonRect,
  width: number,
  height: number
) {
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (!context) throw new Error("2D canvas is unavailable");
  context.clearRect(0, 0, width, height);
  context.imageSmoothingEnabled = true;
  context.imageSmoothingQuality = "high";
  context.drawImage(image, rect.x, rect.y, rect.width, rect.height);
  return context.getImageData(0, 0, width, height).data;
}

const DifferenceComparison: React.FC<{
  referenceImage: ILightboxImage;
  selectedImage: ILightboxImage;
  transform: string;
}> = ({ referenceImage, selectedImage, transform }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [threshold, setThreshold] = useState(DEFAULT_DIFFERENCE_THRESHOLD);
  const [noiseArea, setNoiseArea] = useState(DEFAULT_DIFFERENCE_NOISE);
  const [showBoxes, setShowBoxes] = useState(true);
  const [pixels, setPixels] = useState<INormalizedPixels | null>(null);
  const [result, setResult] = useState<ImageDifferenceResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const referenceSource = comparisonSource(referenceImage);
  const selectedSource = comparisonSource(selectedImage);
  const temporal =
    isTemporalMedia(referenceImage) || isTemporalMedia(selectedImage);

  useEffect(() => {
    let cancelled = false;
    setPixels(null);
    setResult(null);
    setError(null);

    if (temporal) {
      setLoading(false);
      setError(
        "Difference heatmap currently compares still images only. Animated/video media needs an explicit frame or timestamp before a pixel diff is meaningful."
      );
      return () => {
        cancelled = true;
      };
    }
    if (!referenceSource || !selectedSource) {
      setLoading(false);
      setError("A full image source is missing for this comparison.");
      return () => {
        cancelled = true;
      };
    }

    setLoading(true);
    Promise.all([loadImage(referenceSource), loadImage(selectedSource)])
      .then(([reference, selected]) => {
        if (cancelled) return;
        const geometry = createComparisonGeometry(
          reference.naturalWidth,
          reference.naturalHeight,
          selected.naturalWidth,
          selected.naturalHeight
        );
        const referencePixels = drawNormalizedImage(
          reference,
          geometry.reference,
          geometry.width,
          geometry.height
        );
        const selectedPixels = drawNormalizedImage(
          selected,
          geometry.selected,
          geometry.width,
          geometry.height
        );
        if (cancelled) return;
        setPixels({
          width: geometry.width,
          height: geometry.height,
          reference: referencePixels,
          selected: selectedPixels,
        });
        setLoading(false);
      })
      .catch((reason: unknown) => {
        if (cancelled) return;
        const message = reason instanceof Error ? reason.message : String(reason);
        setLoading(false);
        setError(
          `Could not read image pixels for comparison: ${message}. Same-origin image access is required.`
        );
      });

    return () => {
      cancelled = true;
    };
  }, [referenceSource, selectedSource, temporal]);

  useEffect(() => {
    if (!pixels) {
      setResult(null);
      return;
    }
    setResult(
      computeImageDifference(
        pixels.reference,
        pixels.selected,
        pixels.width,
        pixels.height,
        {
          threshold,
          minRegionArea: noiseArea,
          boxPadding: 2,
          mergeGap: 4,
        }
      )
    );
  }, [pixels, threshold, noiseArea]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !pixels || !result) return;

    canvas.width = pixels.width;
    canvas.height = pixels.height;
    const context = canvas.getContext("2d");
    if (!context) return;

    context.putImageData(
      new ImageData(pixels.selected, pixels.width, pixels.height),
      0,
      0
    );
    context.fillStyle = "rgba(0, 0, 0, 0.68)";
    context.fillRect(0, 0, pixels.width, pixels.height);

    const heatmap = document.createElement("canvas");
    heatmap.width = pixels.width;
    heatmap.height = pixels.height;
    const heatmapContext = heatmap.getContext("2d");
    if (heatmapContext) {
      heatmapContext.putImageData(
        new ImageData(result.heatmap, pixels.width, pixels.height),
        0,
        0
      );
      context.drawImage(heatmap, 0, 0);
    }

    if (showBoxes) {
      context.save();
      context.lineWidth = Math.max(2, Math.round(pixels.width / 700));
      context.strokeStyle = "rgba(255, 255, 255, 0.96)";
      context.shadowColor = "rgba(0, 0, 0, 0.9)";
      context.shadowBlur = Math.max(2, Math.round(pixels.width / 900));
      for (const box of result.boxes) {
        context.strokeRect(box.x, box.y, box.width, box.height);
      }
      context.restore();
    }
  }, [pixels, result, showBoxes]);

  const changedPercent =
    result && result.totalPixels > 0
      ? (result.changedPixels / result.totalPixels) * 100
      : 0;

  return (
    <>
      <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
        {loading ? (
          <div className={`${CLASSNAME}-difference-message`}>
            Preparing normalized comparison…
          </div>
        ) : error ? (
          <div className={`${CLASSNAME}-difference-message`}>{error}</div>
        ) : (
          <canvas
            ref={canvasRef}
            className={`${CLASSNAME}-difference-canvas`}
            aria-label="Absolute pixel difference heatmap"
          />
        )}
      </div>
      {!error && (
        <div
          className={`${CLASSNAME}-difference-controls`}
          onPointerDown={(event) => event.stopPropagation()}
          onPointerMove={(event) => event.stopPropagation()}
          onPointerUp={(event) => event.stopPropagation()}
          onWheel={(event) => event.stopPropagation()}
        >
          <label>
            Threshold <strong>{threshold}</strong>
            <input
              type="range"
              min={0}
              max={128}
              step={1}
              value={threshold}
              aria-label="Difference threshold"
              onChange={(event) =>
                setThreshold(Number.parseInt(event.currentTarget.value, 10))
              }
            />
          </label>
          <label>
            Noise <strong>{noiseArea}px</strong>
            <input
              type="range"
              min={1}
              max={128}
              step={1}
              value={noiseArea}
              aria-label="Minimum difference region area"
              onChange={(event) =>
                setNoiseArea(Number.parseInt(event.currentTarget.value, 10))
              }
            />
          </label>
          <label className={`${CLASSNAME}-difference-checkbox`}>
            <input
              type="checkbox"
              checked={showBoxes}
              onChange={(event) => setShowBoxes(event.currentTarget.checked)}
            />{" "}
            Boxes
          </label>
          {pixels && result ? (
            <span className={`${CLASSNAME}-difference-status`}>
              {result.changedPixels.toLocaleString()} changed pixels (
              {changedPercent.toFixed(changedPercent < 0.1 ? 3 : 1)}%) ·{" "}
              {result.boxes.length} regions · {pixels.width}×{pixels.height} canvas
            </span>
          ) : null}
          <small>
            Browser-decoded, centered-fit comparison. Images with the same aspect
            ratio are normalized to the same canvas; unmatched borders remain
            differences. EXIF orientation and color management follow the browser
            decoder.
          </small>
        </div>
      )}
    </>
  );
};

export const ReferenceComparison: React.FC<IProps> = ({
  referenceImage,
  selectedImage,
  mode,
  direction,
  animateSelected,
  zoom,
  resetPosition,
  setZoom,
}) => {
  const [split, setSplit] = useState(50);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [blinkReference, setBlinkReference] = useState(true);
  const dragState = useRef<IDragState | null>(null);
  const sliderRef = useRef<HTMLDivElement>(null);
  const metrics = comparisonMetrics(referenceImage, selectedImage);
  const referenceFilename = comparisonFilename(referenceImage);
  const selectedFilename = comparisonFilename(selectedImage);

  // Keep the reference fixed when only the selected match changes. Reset only
  // for an explicit parent reset, a view-mode change, or a different reference.
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset pan/split only for reference/view resets, not selected-image navigation
  useEffect(() => {
    setPan({ x: 0, y: 0 });
    setSplit(50);
  }, [mode, referenceImage.id, resetPosition]);

  useEffect(() => {
    if (mode !== "blink") {
      setBlinkReference(true);
      return;
    }
    setBlinkReference(true);
    const timer = window.setInterval(
      () => setBlinkReference((value) => !value),
      BLINK_INTERVAL_MS
    );
    return () => window.clearInterval(timer);
  }, [mode]);

  const transform = `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`;

  function resetComparisonView() {
    setPan({ x: 0, y: 0 });
    setSplit(50);
    setZoom(1);
  }

  function onWheel(event: React.WheelEvent<HTMLDivElement>) {
    if (event.deltaY === 0) return;
    event.preventDefault();
    event.stopPropagation();
    setZoom(zoom * (event.deltaY < 0 ? ZOOM_STEP : 1 / ZOOM_STEP));
  }

  function onPointerDown(event: React.PointerEvent<HTMLDivElement>) {
    if (event.button !== 0) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    dragState.current = {
      pointerID: event.pointerId,
      x: event.clientX,
      y: event.clientY,
      panX: pan.x,
      panY: pan.y,
    };
  }

  function onPointerMove(event: React.PointerEvent<HTMLDivElement>) {
    const drag = dragState.current;
    if (!drag || drag.pointerID !== event.pointerId) return;
    setPan({
      x: drag.panX + event.clientX - drag.x,
      y: drag.panY + event.clientY - drag.y,
    });
  }

  function onPointerUp(event: React.PointerEvent<HTMLDivElement>) {
    if (dragState.current?.pointerID !== event.pointerId) return;
    dragState.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }

  function updateSplitFromPointer(clientX: number) {
    const rect = sliderRef.current?.getBoundingClientRect();
    if (!rect || rect.width <= 0) return;
    setSplit(clampSplit(((clientX - rect.left) / rect.width) * 100));
  }

  function onDividerPointerDown(event: React.PointerEvent<HTMLDivElement>) {
    if (event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    updateSplitFromPointer(event.clientX);
  }

  function onDividerPointerMove(event: React.PointerEvent<HTMLDivElement>) {
    if (!event.currentTarget.hasPointerCapture(event.pointerId)) return;
    event.preventDefault();
    event.stopPropagation();
    updateSplitFromPointer(event.clientX);
  }

  function onDividerPointerUp(event: React.PointerEvent<HTMLDivElement>) {
    event.stopPropagation();
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }

  function onDividerKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    const step = event.shiftKey ? SPLIT_KEY_LARGE_STEP : SPLIT_KEY_STEP;
    let next = split;
    switch (event.key) {
      case "ArrowLeft":
      case "ArrowDown":
        next -= step;
        break;
      case "ArrowRight":
      case "ArrowUp":
        next += step;
        break;
      case "Home":
        next = 0;
        break;
      case "End":
        next = 100;
        break;
      default:
        return;
    }
    event.preventDefault();
    event.stopPropagation();
    setSplit(clampSplit(next));
  }

  const interactionProps = {
    onWheel,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onPointerCancel: onPointerUp,
  };

  const resetButton = (
    <button
      type="button"
      className={`btn btn-secondary btn-sm ${CLASSNAME}-reset`}
      onPointerDown={(event) => event.stopPropagation()}
      onWheel={(event) => event.stopPropagation()}
      onClick={(event) => {
        event.stopPropagation();
        resetComparisonView();
      }}
    >
      Reset comparison
    </button>
  );

  if (mode === "both") {
    return (
      <div className={cx(CLASSNAME, `${CLASSNAME}-both`)} {...interactionProps}>
        {resetButton}
        <div className={`${CLASSNAME}-pane`}>
          <ComparisonInfo
            title="Reference"
            filename={referenceFilename}
            side="reference"
            metrics={metrics}
          />
          <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
            <ComparisonMedia image={referenceImage} />
          </div>
        </div>
        <div className={`${CLASSNAME}-pane`}>
          <ComparisonInfo
            title="Selected"
            filename={selectedFilename}
            side="selected"
            metrics={metrics}
          />
          <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
            <SelectedComparisonMedia
              image={selectedImage}
              direction={direction}
              animate={animateSelected}
            />
          </div>
        </div>
      </div>
    );
  }

  if (mode === "blink") {
    return (
      <div className={cx(CLASSNAME, `${CLASSNAME}-blink`)} {...interactionProps}>
        {resetButton}
        <div
          className={`${CLASSNAME}-layer ${CLASSNAME}-blink-layer`}
          style={{ opacity: blinkReference ? 1 : 0 }}
        >
          <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
            <ComparisonMedia image={referenceImage} />
          </div>
        </div>
        <div
          className={`${CLASSNAME}-layer ${CLASSNAME}-blink-layer`}
          style={{ opacity: blinkReference ? 0 : 1 }}
        >
          <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
            <ComparisonMedia image={selectedImage} />
          </div>
        </div>
        <ComparisonInfo
          title="Reference"
          filename={referenceFilename}
          side="reference"
          metrics={metrics}
          className={`${CLASSNAME}-info-left`}
        />
        <ComparisonInfo
          title="Selected"
          filename={selectedFilename}
          side="selected"
          metrics={metrics}
          className={`${CLASSNAME}-info-right`}
        />
        <div className={`${CLASSNAME}-blink-indicator`}>
          {blinkReference ? "Reference" : "Selected"}
        </div>
      </div>
    );
  }

  if (mode === "difference") {
    return (
      <div
        className={cx(CLASSNAME, `${CLASSNAME}-difference`)}
        {...interactionProps}
      >
        {resetButton}
        <DifferenceComparison
          referenceImage={referenceImage}
          selectedImage={selectedImage}
          transform={transform}
        />
      </div>
    );
  }

  return (
    <div
      ref={sliderRef}
      className={cx(CLASSNAME, `${CLASSNAME}-slider`)}
      {...interactionProps}
    >
      {resetButton}
      <div className={`${CLASSNAME}-layer`}>
        <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
          <SelectedComparisonMedia
            image={selectedImage}
            direction={direction}
            animate={animateSelected}
          />
        </div>
      </div>
      <div
        className={`${CLASSNAME}-layer ${CLASSNAME}-reference-layer`}
        style={{ clipPath: `inset(0 ${100 - split}% 0 0)` }}
      >
        <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
          <ComparisonMedia image={referenceImage} />
        </div>
      </div>
      <ComparisonInfo
        title="Reference"
        filename={referenceFilename}
        side="reference"
        metrics={metrics}
        className={`${CLASSNAME}-info-left`}
      />
      <ComparisonInfo
        title="Selected"
        filename={selectedFilename}
        side="selected"
        metrics={metrics}
        className={`${CLASSNAME}-info-right`}
      />
      <div
        className={`${CLASSNAME}-divider`}
        style={{ left: `${split}%` }}
        role="slider"
        tabIndex={0}
        aria-label="Reference comparison split"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(split)}
        aria-valuetext={`${Math.round(split)}% reference`}
        onPointerDown={onDividerPointerDown}
        onPointerMove={onDividerPointerMove}
        onPointerUp={onDividerPointerUp}
        onPointerCancel={onDividerPointerUp}
        onWheel={(event) => event.stopPropagation()}
        onKeyDown={onDividerKeyDown}
      >
        <span className={`${CLASSNAME}-divider-line`} aria-hidden="true" />
        <span className={`${CLASSNAME}-divider-handle`} aria-hidden="true" />
      </div>
    </div>
  );
};
