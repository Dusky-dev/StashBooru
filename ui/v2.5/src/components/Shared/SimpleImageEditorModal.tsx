import React, { useEffect, useRef, useState } from "react";
import { Button, ButtonGroup, Form } from "react-bootstrap";
import { useIntl } from "react-intl";
import { useToast } from "src/hooks/Toast";
import { ModalComponent } from "./Modal";

type CropAspect = "original" | "square" | "landscape" | "portrait";

interface ISimpleImageEditorModalProps {
  show: boolean;
  source: string;
  title?: string;
  onBack: () => void;
  onApply: (imageData: string) => void;
}

interface ICropGeometry {
  x: number;
  y: number;
  width: number;
  height: number;
  offsetX: number;
  offsetY: number;
  maxOffsetX: number;
  maxOffsetY: number;
}

interface IDragState {
  pointerID: number;
  startX: number;
  startY: number;
  offsetX: number;
  offsetY: number;
}

const MIN_ZOOM = 1;
const MAX_ZOOM = 5;
const PREVIEW_MAX_WIDTH = 760;
const PREVIEW_MAX_HEIGHT = 500;

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function normalizeRotation(rotation: number) {
  return ((rotation % 360) + 360) % 360;
}

function rotatedDimensions(image: HTMLImageElement, rotation: number) {
  const normalized = normalizeRotation(rotation);
  const swap = normalized === 90 || normalized === 270;
  return {
    width: swap ? image.naturalHeight : image.naturalWidth,
    height: swap ? image.naturalWidth : image.naturalHeight,
  };
}

function cropRatio(
  aspect: CropAspect,
  dimensions: { width: number; height: number }
) {
  switch (aspect) {
    case "square":
      return 1;
    case "landscape":
      return 4 / 3;
    case "portrait":
      return 3 / 4;
    default:
      return dimensions.width / dimensions.height;
  }
}

function cropGeometry(
  image: HTMLImageElement,
  rotation: number,
  aspect: CropAspect,
  zoom: number,
  requestedOffsetX: number,
  requestedOffsetY: number
): ICropGeometry {
  const dimensions = rotatedDimensions(image, rotation);
  const ratio = cropRatio(aspect, dimensions);

  let baseWidth = dimensions.width;
  let baseHeight = baseWidth / ratio;
  if (baseHeight > dimensions.height) {
    baseHeight = dimensions.height;
    baseWidth = baseHeight * ratio;
  }

  const safeZoom = clamp(zoom, MIN_ZOOM, MAX_ZOOM);
  const width = baseWidth / safeZoom;
  const height = baseHeight / safeZoom;
  const maxOffsetX = Math.max(0, (dimensions.width - width) / 2);
  const maxOffsetY = Math.max(0, (dimensions.height - height) / 2);
  const offsetX = clamp(requestedOffsetX, -maxOffsetX, maxOffsetX);
  const offsetY = clamp(requestedOffsetY, -maxOffsetY, maxOffsetY);

  return {
    x: (dimensions.width - width) / 2 + offsetX,
    y: (dimensions.height - height) / 2 + offsetY,
    width,
    height,
    offsetX,
    offsetY,
    maxOffsetX,
    maxOffsetY,
  };
}

function previewDimensions(crop: ICropGeometry) {
  const ratio = crop.width / crop.height;
  let width = PREVIEW_MAX_WIDTH;
  let height = width / ratio;

  if (height > PREVIEW_MAX_HEIGHT) {
    height = PREVIEW_MAX_HEIGHT;
    width = height * ratio;
  }

  return {
    width: Math.max(1, Math.round(width)),
    height: Math.max(1, Math.round(height)),
  };
}

function renderCrop(
  canvas: HTMLCanvasElement,
  image: HTMLImageElement,
  rotation: number,
  crop: ICropGeometry,
  outputWidth: number,
  outputHeight: number,
  showGuides = false
) {
  canvas.width = Math.max(1, Math.round(outputWidth));
  canvas.height = Math.max(1, Math.round(outputHeight));

  const context = canvas.getContext("2d");
  if (!context) throw new Error("Canvas rendering is unavailable.");

  const dimensions = rotatedDimensions(image, rotation);
  context.clearRect(0, 0, canvas.width, canvas.height);
  context.save();
  context.scale(canvas.width / crop.width, canvas.height / crop.height);
  context.translate(-crop.x, -crop.y);
  context.translate(dimensions.width / 2, dimensions.height / 2);
  context.rotate((normalizeRotation(rotation) * Math.PI) / 180);
  context.drawImage(image, -image.naturalWidth / 2, -image.naturalHeight / 2);
  context.restore();

  if (!showGuides) return;

  context.save();
  context.strokeStyle = "rgba(255, 255, 255, 0.55)";
  context.lineWidth = 1;
  context.setLineDash([5, 5]);
  for (const fraction of [1 / 3, 2 / 3]) {
    context.beginPath();
    context.moveTo(canvas.width * fraction, 0);
    context.lineTo(canvas.width * fraction, canvas.height);
    context.stroke();
    context.beginPath();
    context.moveTo(0, canvas.height * fraction);
    context.lineTo(canvas.width, canvas.height * fraction);
    context.stroke();
  }
  context.restore();
}

export const SimpleImageEditorModal: React.FC<ISimpleImageEditorModalProps> = ({
  show,
  source,
  title,
  onBack,
  onApply,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const previewCanvas = useRef<HTMLCanvasElement>(null);
  const dragState = useRef<IDragState | undefined>(undefined);
  const [image, setImage] = useState<HTMLImageElement>();
  const [rotation, setRotation] = useState(0);
  const [aspect, setAspect] = useState<CropAspect>("original");
  const [zoom, setZoom] = useState(MIN_ZOOM);
  const [offsetX, setOffsetX] = useState(0);
  const [offsetY, setOffsetY] = useState(0);
  const [dragging, setDragging] = useState(false);
  const [applying, setApplying] = useState(false);

  useEffect(() => {
    const nextImage = new Image();
    nextImage.onload = () => setImage(nextImage);
    nextImage.onerror = () => {
      setImage(undefined);
      Toast.error(
        intl.formatMessage({
          id: "toast.image_source_unavailable",
          defaultMessage: "The selected image is not available.",
        })
      );
    };
    nextImage.src = source;

    setRotation(0);
    setAspect("original");
    setZoom(MIN_ZOOM);
    setOffsetX(0);
    setOffsetY(0);

    return () => {
      nextImage.onload = null;
      nextImage.onerror = null;
    };
  }, [source, Toast, intl]);

  const crop = image
    ? cropGeometry(image, rotation, aspect, zoom, offsetX, offsetY)
    : undefined;

  useEffect(() => {
    if (!image || !previewCanvas.current) return;

    try {
      const currentCrop = cropGeometry(
        image,
        rotation,
        aspect,
        zoom,
        offsetX,
        offsetY
      );
      const dimensions = previewDimensions(currentCrop);
      renderCrop(
        previewCanvas.current,
        image,
        rotation,
        currentCrop,
        dimensions.width,
        dimensions.height,
        true
      );
    } catch (error) {
      Toast.error(error);
    }
  }, [image, rotation, aspect, zoom, offsetX, offsetY, Toast]);

  function reset() {
    setRotation(0);
    setAspect("original");
    setZoom(MIN_ZOOM);
    setOffsetX(0);
    setOffsetY(0);
  }

  function centerCrop() {
    setOffsetX(0);
    setOffsetY(0);
  }

  function rotate(amount: number) {
    setRotation((current) => normalizeRotation(current + amount));
    setOffsetX(0);
    setOffsetY(0);
  }

  function changeAspect(nextAspect: CropAspect) {
    setAspect(nextAspect);
    if (!image) return;

    const nextCrop = cropGeometry(
      image,
      rotation,
      nextAspect,
      zoom,
      offsetX,
      offsetY
    );
    setOffsetX(nextCrop.offsetX);
    setOffsetY(nextCrop.offsetY);
  }

  function changeZoom(nextZoom: number, anchorX = 0.5, anchorY = 0.5) {
    const clampedZoom = clamp(nextZoom, MIN_ZOOM, MAX_ZOOM);
    if (!image) {
      setZoom(clampedZoom);
      return;
    }

    const currentCrop = cropGeometry(
      image,
      rotation,
      aspect,
      zoom,
      offsetX,
      offsetY
    );
    const resizedCrop = cropGeometry(
      image,
      rotation,
      aspect,
      clampedZoom,
      currentCrop.offsetX,
      currentCrop.offsetY
    );
    const anchoredOffsetX =
      currentCrop.offsetX +
      (anchorX - 0.5) * (currentCrop.width - resizedCrop.width);
    const anchoredOffsetY =
      currentCrop.offsetY +
      (anchorY - 0.5) * (currentCrop.height - resizedCrop.height);
    const nextCrop = cropGeometry(
      image,
      rotation,
      aspect,
      clampedZoom,
      anchoredOffsetX,
      anchoredOffsetY
    );

    setZoom(clampedZoom);
    setOffsetX(nextCrop.offsetX);
    setOffsetY(nextCrop.offsetY);
  }

  function beginDrag(event: React.PointerEvent<HTMLCanvasElement>) {
    if (!crop) return;

    event.currentTarget.setPointerCapture(event.pointerId);
    dragState.current = {
      pointerID: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      offsetX: crop.offsetX,
      offsetY: crop.offsetY,
    };
    setDragging(true);
  }

  function moveDrag(event: React.PointerEvent<HTMLCanvasElement>) {
    const state = dragState.current;
    const canvas = previewCanvas.current;
    if (!state || !crop || !canvas || state.pointerID !== event.pointerId)
      return;

    const rect = canvas.getBoundingClientRect();
    if (!rect.width || !rect.height) return;

    const deltaX = ((event.clientX - state.startX) * crop.width) / rect.width;
    const deltaY = ((event.clientY - state.startY) * crop.height) / rect.height;
    setOffsetX(
      clamp(state.offsetX - deltaX, -crop.maxOffsetX, crop.maxOffsetX)
    );
    setOffsetY(
      clamp(state.offsetY - deltaY, -crop.maxOffsetY, crop.maxOffsetY)
    );
  }

  function endDrag(event: React.PointerEvent<HTMLCanvasElement>) {
    if (dragState.current?.pointerID !== event.pointerId) return;

    dragState.current = undefined;
    setDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }

  function wheelZoom(event: React.WheelEvent<HTMLCanvasElement>) {
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    const anchorX = rect.width
      ? clamp((event.clientX - rect.left) / rect.width, 0, 1)
      : 0.5;
    const anchorY = rect.height
      ? clamp((event.clientY - rect.top) / rect.height, 0, 1)
      : 0.5;
    changeZoom(zoom + (event.deltaY < 0 ? 0.15 : -0.15), anchorX, anchorY);
  }

  function apply() {
    if (!image || !crop) return;

    setApplying(true);
    try {
      if (
        normalizeRotation(rotation) === 0 &&
        aspect === "original" &&
        zoom === MIN_ZOOM &&
        crop.offsetX === 0 &&
        crop.offsetY === 0
      ) {
        onApply(source);
        return;
      }

      const canvas = document.createElement("canvas");
      renderCrop(
        canvas,
        image,
        rotation,
        crop,
        Math.round(crop.width),
        Math.round(crop.height)
      );
      onApply(canvas.toDataURL("image/png"));
    } catch (error) {
      Toast.error(error);
    } finally {
      setApplying(false);
    }
  }

  const cropOptions: Array<{ value: CropAspect; label: string }> = [
    { value: "original", label: "Original" },
    { value: "square", label: "1:1" },
    { value: "landscape", label: "4:3" },
    { value: "portrait", label: "3:4" },
  ];
  const outputSize = crop
    ? `${Math.round(crop.width)} × ${Math.round(crop.height)} px`
    : "";

  return (
    <ModalComponent
      show={show}
      onHide={onBack}
      header={intl.formatMessage(
        {
          id: "dialogs.edit_set_image",
          defaultMessage: "Edit image{title}",
        },
        { title: title ? ` — ${title}` : "" }
      )}
      cancel={{
        onClick: onBack,
        text: intl.formatMessage({
          id: "actions.back",
          defaultMessage: "Back",
        }),
      }}
      accept={{
        onClick: apply,
        text: intl.formatMessage({
          id: "actions.apply",
          defaultMessage: "Apply",
        }),
      }}
      disabled={!image}
      isRunning={applying}
      modalProps={{ size: "xl" }}
    >
      <div
        className="d-flex justify-content-center align-items-center p-3 mb-3 rounded"
        style={{
          minHeight: "300px",
          background: "rgba(0, 0, 0, 0.35)",
          overflow: "hidden",
        }}
      >
        <canvas
          ref={previewCanvas}
          onPointerDown={beginDrag}
          onPointerMove={moveDrag}
          onPointerUp={endDrag}
          onPointerCancel={endDrag}
          onWheel={wheelZoom}
          onDoubleClick={centerCrop}
          title={intl.formatMessage({
            id: "image_editor_drag_hint",
            defaultMessage: "Drag to reposition, scroll to zoom",
          })}
          style={{
            maxWidth: "100%",
            maxHeight: "58vh",
            border: "1px solid rgba(255, 255, 255, 0.25)",
            boxShadow: "0 0.5rem 1.5rem rgba(0, 0, 0, 0.35)",
            cursor: dragging ? "grabbing" : "grab",
            touchAction: "none",
            userSelect: "none",
          }}
        />
      </div>

      <div className="text-center text-muted small mb-3">
        {intl.formatMessage({
          id: "image_editor_interaction_hint",
          defaultMessage:
            "Drag the image to reposition • Scroll to zoom • Double-click to center",
        })}
        {outputSize ? ` • ${outputSize}` : ""}
      </div>

      <div className="d-flex flex-wrap align-items-center justify-content-center mb-3">
        <Button
          variant="secondary"
          className="mr-2 mb-2"
          onClick={() => rotate(-90)}
        >
          ↺{" "}
          {intl.formatMessage({
            id: "actions.rotate_left",
            defaultMessage: "Rotate left",
          })}
        </Button>
        <Button
          variant="secondary"
          className="mr-2 mb-2"
          onClick={() => rotate(90)}
        >
          ↻{" "}
          {intl.formatMessage({
            id: "actions.rotate_right",
            defaultMessage: "Rotate right",
          })}
        </Button>
        <Button variant="secondary" className="mr-2 mb-2" onClick={centerCrop}>
          {intl.formatMessage({
            id: "actions.center",
            defaultMessage: "Center",
          })}
        </Button>
        <Button variant="secondary" className="mb-2" onClick={reset}>
          {intl.formatMessage({ id: "actions.reset", defaultMessage: "Reset" })}
        </Button>
      </div>

      <Form.Group className="mb-3 text-center">
        <Form.Label className="d-block">
          {intl.formatMessage({
            id: "crop_aspect",
            defaultMessage: "Crop aspect",
          })}
        </Form.Label>
        <ButtonGroup>
          {cropOptions.map((option) => (
            <Button
              key={option.value}
              variant={aspect === option.value ? "primary" : "secondary"}
              onClick={() => changeAspect(option.value)}
            >
              {option.label}
            </Button>
          ))}
        </ButtonGroup>
      </Form.Group>

      <Form.Group className="mb-0">
        <div className="d-flex justify-content-between align-items-center mb-2">
          <Form.Label className="mb-0">
            {intl.formatMessage({ id: "zoom", defaultMessage: "Zoom" })}
          </Form.Label>
          <span>{zoom.toFixed(2)}×</span>
        </div>
        <div className="d-flex align-items-center">
          <Button
            type="button"
            variant="secondary"
            className="mr-2"
            disabled={zoom <= MIN_ZOOM}
            onClick={() => changeZoom(zoom - 0.25)}
          >
            −
          </Button>
          <Form.Control
            type="range"
            min={MIN_ZOOM}
            max={MAX_ZOOM}
            step={0.05}
            value={zoom}
            onChange={(event) => changeZoom(Number(event.currentTarget.value))}
          />
          <Button
            type="button"
            variant="secondary"
            className="ml-2"
            disabled={zoom >= MAX_ZOOM}
            onClick={() => changeZoom(zoom + 0.25)}
          >
            +
          </Button>
        </div>
      </Form.Group>
    </ModalComponent>
  );
};
