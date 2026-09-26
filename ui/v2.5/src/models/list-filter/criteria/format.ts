import { StringCriterion, StringCriterionOption } from "./criterion";
import {
  addFileFilterInput,
  imageFormatFileFilterInput,
  sceneFormatFileFilterInput,
} from "./file-filter";

const formatOptions = [
  { id: "apng", name: "APNG" },
  { id: "avif", name: "AVIF" },
  { id: "avi", name: "AVI" },
  { id: "bmp", name: "BMP" },
  { id: "flv", name: "FLV" },
  { id: "gif", name: "GIF" },
  { id: "jpeg", name: "JPEG (jpeg)" },
  { id: "jpegxl", name: "JXL (JPEG XL)" },
  { id: "m4v", name: "M4V" },
  { id: "matroska", name: "MKV (Matroska)" },
  { id: "mjpeg", name: "JPG / JPEG (mjpeg)" },
  { id: "mov", name: "MOV" },
  { id: "mp4", name: "MP4" },
  { id: "mpeg", name: "MPEG" },
  { id: "mpegts", name: "MPEG-TS" },
  { id: "ogg", name: "OGG / OGV" },
  { id: "png", name: "PNG" },
  { id: "tiff", name: "TIFF" },
  { id: "webm", name: "WebM" },
  { id: "webp", name: "WebP" },
  { id: "wmv", name: "WMV / ASF" },
];

export const ImageFormatCriterionOption = new StringCriterionOption({
  messageID: "format",
  type: "format",
  inputType: "select",
  options: formatOptions,
  makeCriterion: () => new ImageFormatCriterion(),
});

export class ImageFormatCriterion extends StringCriterion {
  constructor() {
    super(ImageFormatCriterionOption);
  }

  public applyToCriterionInput(input: Record<string, unknown>) {
    addFileFilterInput(
      input,
      imageFormatFileFilterInput(this.toCriterionInput())
    );
  }
}

export const SceneFormatCriterionOption = new StringCriterionOption({
  messageID: "format",
  type: "format",
  inputType: "select",
  options: formatOptions,
  makeCriterion: () => new SceneFormatCriterion(),
});

export class SceneFormatCriterion extends StringCriterion {
  constructor() {
    super(SceneFormatCriterionOption);
  }

  public applyToCriterionInput(input: Record<string, unknown>) {
    addFileFilterInput(
      input,
      sceneFormatFileFilterInput(this.toCriterionInput())
    );
  }
}
