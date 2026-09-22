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

interface ICropRect {
  x: number;
  y: number;
  width: number;
  height: number;
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

function cropRatio(aspect: CropAspect) {
  switch (aspect) {
    case "square":
      return 1;
    case "landscape":
      return 4 / 3;
    case "portrait":
      return 3 / 4;
    default:
      return undefined;
  }
}

function cropRect(
  width: number,
  height: number,
  aspect: CropAspect,
  positionX: number,
  positionY: number
): ICropRect {
  const ratio = cropRatio(aspect);
  if (!ratio) return { x: 0, y: 0, width, height };

  let cropWidth = width;
  let cropHeight = cropWidth / ratio;
  if (cropHeight > height) {
    cropHeight = height;
    cropWidth = cropHeight * ratio;
  }

  const remainingX = Math.max(0, width - cropWidth);
  const remainingY = Math.max(0, height - cropHeight);
  return {
    x: remainingX * (positionX / 100),
    y: remainingY * (positionY / 100),
    width: cropWidth,
    height: cropHeight,
  };
}

function renderEditedImage(
  canvas: HTMLCanvasElement,
  image: HTMLImageElement,
  rotation: number,
  aspect: CropAspect,
  positionX: number,
  positionY: number,
  maxDimension?: number
) {
  const dimensions = rotatedDimensions(image, rotation);
  const crop = cropRect(
    dimensions.width,
    dimensions.height,
    aspect,
    positionX,
    positionY
  );
  const scale = maxDimension
    ? Math.min(1, maxDimension / Math.max(crop.width, crop.height))
    : 1;

  canvas.width = Math.max(1, Math.round(crop.width * scale));
  canvas.height = Math.max(1, Math.round(crop.height * scale));

  const context = canvas.getContext("2d");
  if (!context) throw new Error("Canvas rendering is unavailable.");

  context.clearRect(0, 0, canvas.width, canvas.height);
  context.save();
  context.scale(scale, scale);
  context.translate(-crop.x, -crop.y);
  context.translate(dimensions.width / 2, dimensions.height / 2);
  context.rotate((normalizeRotation(rotation) * Math.PI) / 180);
  context.drawImage(image, -image.naturalWidth / 2, -image.naturalHeight / 2);
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
  const [image, setImage] = useState<HTMLImageElement>();
  const [rotation, setRotation] = useState(0);
  const [aspect, setAspect] = useState<CropAspect>("original");
  const [positionX, setPositionX] = useState(50);
  const [positionY, setPositionY] = useState(50);
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
    setPositionX(50);
    setPositionY(50);

    return () => {
      nextImage.onload = null;
      nextImage.onerror = null;
    };
  }, [source, Toast, intl]);

  useEffect(() => {
    if (!image || !previewCanvas.current) return;
    try {
      renderEditedImage(
        previewCanvas.current,
        image,
        rotation,
        aspect,
        positionX,
        positionY,
        720
      );
    } catch (error) {
      Toast.error(error);
    }
  }, [image, rotation, aspect, positionX, positionY, Toast]);

  function reset() {
    setRotation(0);
    setAspect("original");
    setPositionX(50);
    setPositionY(50);
  }

  function rotate(amount: number) {
    setRotation((current) => normalizeRotation(current + amount));
  }

  async function apply() {
    if (!image) return;
    setApplying(true);
    try {
      if (
        normalizeRotation(rotation) === 0 &&
        aspect === "original" &&
        positionX === 50 &&
        positionY === 50
      ) {
        onApply(source);
        return;
      }

      const canvas = document.createElement("canvas");
      renderEditedImage(canvas, image, rotation, aspect, positionX, positionY);
      onApply(canvas.toDataURL("image/png"));
    } catch (error) {
      Toast.error(error);
    } finally {
      setApplying(false);
    }
  }

  const cropEnabled = aspect !== "original";
  const cropOptions: Array<{ value: CropAspect; label: string }> = [
    { value: "original", label: "Original" },
    { value: "square", label: "1:1" },
    { value: "landscape", label: "4:3" },
    { value: "portrait", label: "3:4" },
  ];

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
        onClick: () => void apply(),
        text: intl.formatMessage({
          id: "actions.apply",
          defaultMessage: "Apply",
        }),
      }}
      disabled={!image}
      isRunning={applying}
      modalProps={{ size: "xl" }}
    >
      <div className="text-center mb-3">
        <canvas
          ref={previewCanvas}
          style={{
            maxWidth: "100%",
            maxHeight: "55vh",
            background: "var(--card-bg, transparent)",
          }}
        />
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
              onClick={() => setAspect(option.value)}
            >
              {option.label}
            </Button>
          ))}
        </ButtonGroup>
      </Form.Group>

      <Form.Group>
        <Form.Label>
          {intl.formatMessage({
            id: "crop_horizontal_position",
            defaultMessage: "Horizontal crop position",
          })}
        </Form.Label>
        <Form.Control
          type="range"
          min={0}
          max={100}
          value={positionX}
          disabled={!cropEnabled}
          onChange={(event) => setPositionX(Number(event.currentTarget.value))}
        />
      </Form.Group>

      <Form.Group className="mb-0">
        <Form.Label>
          {intl.formatMessage({
            id: "crop_vertical_position",
            defaultMessage: "Vertical crop position",
          })}
        </Form.Label>
        <Form.Control
          type="range"
          min={0}
          max={100}
          value={positionY}
          disabled={!cropEnabled}
          onChange={(event) => setPositionY(Number(event.currentTarget.value))}
        />
      </Form.Group>
    </ModalComponent>
  );
};
