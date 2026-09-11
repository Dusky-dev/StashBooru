import React, { useEffect, useRef, useState } from "react";
import cx from "classnames";

import { isVideo } from "src/utils/visualFile";
import { ILightboxImage } from "./types";

const CLASSNAME = "Lightbox-reference-comparison";
const ZOOM_STEP = 1.1;

export type ReferenceComparisonMode = "selected" | "both" | "slider";

interface IProps {
  referenceImage: ILightboxImage;
  selectedImage: ILightboxImage;
  mode: Exclude<ReferenceComparisonMode, "selected">;
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

export const ReferenceComparison: React.FC<IProps> = ({
  referenceImage,
  selectedImage,
  mode,
  zoom,
  resetPosition,
  setZoom,
}) => {
  const [split, setSplit] = useState(50);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const dragState = useRef<IDragState | null>(null);

  useEffect(() => {
    setPan({ x: 0, y: 0 });
  }, [mode, referenceImage.id, resetPosition, selectedImage.id]);

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
          <span className={`${CLASSNAME}-label`}>Reference</span>
          <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
            <ComparisonMedia image={referenceImage} />
          </div>
        </div>
        <div className={`${CLASSNAME}-pane`}>
          <span className={`${CLASSNAME}-label`}>Selected</span>
          <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
            <ComparisonMedia image={selectedImage} />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={cx(CLASSNAME, `${CLASSNAME}-slider`)} {...interactionProps}>
      <div className={`${CLASSNAME}-layer`}>
        <div className={`${CLASSNAME}-viewport`} style={{ transform }}>
          <ComparisonMedia image={selectedImage} />
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
      <span className={`${CLASSNAME}-label ${CLASSNAME}-label-left`}>
        Reference
      </span>
      <span className={`${CLASSNAME}-label ${CLASSNAME}-label-right`}>
        Selected
      </span>
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
