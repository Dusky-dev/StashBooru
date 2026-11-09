import React from "react";
import { Form } from "react-bootstrap";
import { SettingsDisclosure } from "./SettingsDisclosure";
import { useSettings } from "./context";

export const SettingsThumbnailCompression: React.FC = () => {
  const { ui, saveUI } = useSettings();
  const imageFormat = ui.thumbnailImageFormat ?? "jpg";
  const imageQuality = ui.thumbnailImageQuality ?? 70;
  const videoFormat = ui.thumbnailVideoFormat ?? "webm";
  const videoQuality = ui.thumbnailVideoQuality ?? 75;

  return (
    <SettingsDisclosure
      id="experimental-thumbnail-compression"
      title="Thumbnail and video preview formats"
      description="Test smaller generated thumbnails and alternative preview codecs. Original media files are never modified."
    >
      <Form.Group controlId="experimental-image-thumb-format">
        <Form.Label>Image thumbnail format</Form.Label>
        <Form.Control
          as="select"
          value={imageFormat}
          onChange={(e) =>
            saveUI({
              thumbnailImageFormat: e.currentTarget.value as "jpg" | "webp" | "jxl",
            })
          }
        >
          <option value="jpg">JPEG (default, widest compatibility)</option>
          <option value="webp">WebP (smaller, broadly supported)</option>
          <option value="jxl">JPEG XL (experimental, needs libvips)</option>
        </Form.Control>
      </Form.Group>
      <Form.Group controlId="experimental-image-thumb-quality">
        <Form.Label>Image thumbnail quality: {imageQuality}%</Form.Label>
        <Form.Control
          type="range"
          min={10}
          max={100}
          step={5}
          value={imageQuality}
          onChange={(e) =>
            saveUI({ thumbnailImageQuality: Number(e.currentTarget.value) })
          }
        />
        <Form.Text className="text-muted">
          Lower quality generally means smaller thumbnails. 70% is the default.
        </Form.Text>
      </Form.Group>
      {imageFormat === "jxl" && (
        <p className="text-warning">
          JPEG XL is not supported by every browser or encoder build. Clients
          without advertised JXL support receive WebP; generation also falls
          back to WebP if JXL encoding fails.
        </p>
      )}
      <hr />
      <Form.Group controlId="experimental-video-preview-format">
        <Form.Label>Image-clip video preview format</Form.Label>
        <Form.Control
          as="select"
          value={videoFormat}
          onChange={(e) =>
            saveUI({
              thumbnailVideoFormat: e.currentTarget.value as "webm" | "mp4",
            })
          }
        >
          <option value="webm">WebM / VP9 (default)</option>
          <option value="mp4">MP4 / H.264 (compatibility)</option>
        </Form.Control>
        <Form.Text className="text-muted">
          Affects playable previews for video-backed Images, not scene previews,
          video originals or downloads.
        </Form.Text>
      </Form.Group>
      <Form.Group controlId="experimental-video-preview-quality">
        <Form.Label>Image-clip preview quality: {videoQuality}%</Form.Label>
        <Form.Control
          type="range"
          min={10}
          max={100}
          step={5}
          value={videoQuality}
          onChange={(e) =>
            saveUI({ thumbnailVideoQuality: Number(e.currentTarget.value) })
          }
        />
      </Form.Group>
      <p className="text-muted">
        These settings apply to newly generated assets. Existing variants stay
        on disk. For previously generated files, run Generate with overwrite
        for image thumbnails or clip previews. Legacy video previews continue
        displaying until their replacement is generated.
      </p>
    </SettingsDisclosure>
  );
};
