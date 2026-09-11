import React, { useEffect, useRef, useState } from "react";
import cx from "classnames";

import { isVideo } from "src/utils/visualFile";
import { ILightboxImage } from "./types";

const CLASSNAME = "Lightbox-reference-comparison";
const ZOOM_STEP = 1.1;

export type ReferenceComparisonMode = "selected" | "both" | "slider";
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

const ComparisonMedia: React.FC<{ image: ILightboxImage }> = ({ image }) => {
  const source =
    image.paths.image ?? image.paths.preview ?? image.paths.thumbnail ?? "";
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
  const source =
    image.paths.image ?? image.paths.preview ?? image.paths.thumbnail ?? "";
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
      label: "Size",
      referenceText: formatBytes(referenceFile?.size),
      selectedText: formatBytes(selectedFile?.size),
      referenceValue: referenceFile?.size,
      selectedValue: selectedFile?.size,
    },
    {
      label: "Resolution",
      referenceText: formatResolution(referenceFile?.width, referenceFile?.height),
      selectedText: formatResolution(selectedFile?.width, selectedFile?.height),
      referenceValue: resolutionValue(referenceFile?.width, referenceFile?.height),
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
  side: "reference" | "selected";
  metrics: IComparisonMetric[];
  className?: string;
}> = ({ title, side, metrics, className }) => (
  <div className={cx(`${CLASSNAME}-info`, className)}>
    <strong className={`${CLASSNAME}-info-title`}>{title}</strong>
    {metrics.map((metric) => {
      const comparable =
        metric.referenceValue !== undefined && metric.selectedValue !== undefined;
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
  const dragState = useRef<IDragState | null>(null);
  const metrics = comparisonMetrics(referenceImage, selectedImage);

  // Keep the reference fixed when only the selected match changes. Reset only
  // for an explicit parent reset, a view-mode change, or a different reference.
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset pan only for reference/view resets, not selected-image navigation
  useEffect(() => {
    setPan({ x: 0, y: 0 });
  }, [mode, referenceImage.id, resetPosition]);

  const transform = `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`;

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

  const interactionProps = {
    onWheel,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onPointerCancel: onPointerUp,
  };

  if (mode === "both") {
    return (
      <div className={cx(CLASSNAME, `${CLASSNAME}-both`)} {...interactionProps}>
        <div className={`${CLASSNAME}-pane`}>
          <ComparisonInfo title="Reference" side="reference" metrics={metrics} />
          <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
            <ComparisonMedia image={referenceImage} />
          </div>
        </div>
        <div className={`${CLASSNAME}-pane`}>
          <ComparisonInfo title="Selected" side="selected" metrics={metrics} />
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

  return (
    <div className={cx(CLASSNAME, `${CLASSNAME}-slider`)} {...interactionProps}>
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
        side="reference"
        metrics={metrics}
        className={`${CLASSNAME}-info-left`}
      />
      <ComparisonInfo
        title="Selected"
        side="selected"
        metrics={metrics}
        className={`${CLASSNAME}-info-right`}
      />
      <div
        className={`${CLASSNAME}-divider`}
        style={{ left: `${split}%` }}
        aria-hidden="true"
      />
      <input
        className={`${CLASSNAME}-range`}
        type="range"
        min={0}
        max={100}
        value={split}
        aria-label="Reference comparison slider"
        onPointerDown={(event) => event.stopPropagation()}
        onPointerMove={(event) => event.stopPropagation()}
        onPointerUp={(event) => event.stopPropagation()}
        onWheel={(event) => event.stopPropagation()}
        onChange={(event) => setSplit(Number(event.currentTarget.value))}
      />
    </div>
  );
};
