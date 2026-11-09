import React, { useCallback, useEffect, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import { SettingsDisclosure } from "./SettingsDisclosure";
import { useSettings } from "./context";

type Category = "image" | "clip" | "animated";
type ThumbnailStatus = {
  busy: boolean;
  jobID?: number;
  category?: Category;
  imageEnabled: boolean;
  clipEnabled: boolean;
  animatedEnabled: boolean;
};

const labels: Record<Category, string> = {
  image: "Still-image thumbnails",
  clip: "Video clip previews",
  animated: "Animated-image thumbnails",
};

export const SettingsThumbnailCompression: React.FC = () => {
  const { ui, refetch } = useSettings();
  const [imageFormat, setImageFormat] = useState<"jpg" | "webp" | "jxl">(
    ui.thumbnailImageFormat ?? "jpg"
  );
  const [imageQuality, setImageQuality] = useState(
    ui.thumbnailImageQuality ?? 70
  );
  const [videoFormat, setVideoFormat] = useState<"webm" | "mp4">(
    ui.thumbnailVideoFormat ?? "webm"
  );
  const [videoQuality, setVideoQuality] = useState(
    ui.thumbnailVideoQuality ?? 75
  );
  const [animatedQuality, setAnimatedQuality] = useState(
    ui.thumbnailAnimatedQuality ?? 70
  );
  const [status, setStatus] = useState<ThumbnailStatus>();
  const [pending, setPending] = useState<Category | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    setImageFormat(ui.thumbnailImageFormat ?? "jpg");
    setImageQuality(ui.thumbnailImageQuality ?? 70);
    setVideoFormat(ui.thumbnailVideoFormat ?? "webm");
    setVideoQuality(ui.thumbnailVideoQuality ?? 75);
    setAnimatedQuality(ui.thumbnailAnimatedQuality ?? 70);
  }, [
    ui.thumbnailImageFormat,
    ui.thumbnailImageQuality,
    ui.thumbnailVideoFormat,
    ui.thumbnailVideoQuality,
    ui.thumbnailAnimatedQuality,
  ]);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    const response = await fetch("image/thumbnail-compression", { signal });
    if (!response.ok) throw new Error(await response.text());
    const current = (await response.json()) as ThumbnailStatus;
    if (!signal?.aborted) setStatus(current);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal).catch((cause: unknown) => {
      if (!controller.signal.aborted)
        setError(cause instanceof Error ? cause.message : String(cause));
    });
    return () => controller.abort();
  }, [refresh]);

  useEffect(() => {
    if (!status?.busy) return;
    const handle = setTimeout(() => {
      void refresh().catch((cause: unknown) => {
        setError(cause instanceof Error ? cause.message : String(cause));
      });
    }, 2500);
    return () => clearTimeout(handle);
  }, [refresh, status?.busy]);

  const enabled: Record<Category, boolean> = {
    image: status?.imageEnabled ?? (ui.thumbnailImageEnabled ?? false),
    clip: status?.clipEnabled ?? (ui.thumbnailVideoEnabled ?? false),
    animated: status?.animatedEnabled ?? (ui.thumbnailAnimatedEnabled ?? false),
  };

  const unchanged: Record<Category, boolean> = {
    image:
      imageFormat === (ui.thumbnailImageFormat ?? "jpg") &&
      imageQuality === (ui.thumbnailImageQuality ?? 70),
    clip:
      videoFormat === (ui.thumbnailVideoFormat ?? "webm") &&
      videoQuality === (ui.thumbnailVideoQuality ?? 75),
    animated: animatedQuality === (ui.thumbnailAnimatedQuality ?? 70),
  };

  const disabled = saving || Boolean(status?.busy);

  async function apply(category: Category, turnOn: boolean) {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const format =
        category === "image" ? imageFormat : category === "clip" ? videoFormat : "webp";
      const quality =
        category === "image"
          ? imageQuality
          : category === "clip"
            ? videoQuality
            : animatedQuality;
      const response = await fetch("image/thumbnail-compression", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          category,
          enabled: turnOn,
          format,
          quality,
          acknowledged: turnOn,
        }),
      });
      if (!response.ok) throw new Error(await response.text());
      const result = (await response.json()) as {
        jobID?: number;
        enabled: boolean;
      };
      setMessage(
        turnOn
          ? `Rebuilding ${labels[category].toLowerCase()} as job #${result.jobID}. Follow progress in the Task Queue.`
          : `${labels[category]} experiment disabled. Native previews will be generated on demand if missing.`
      );
      setPending(null);
      await refresh();
      refetch();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSaving(false);
    }
  }

  const changeEnabled = (category: Category, turnOn: boolean) => {
    setError("");
    if (turnOn) {
      setPending(category);
    } else {
      void apply(category, false);
    }
  };

  return (
    <SettingsDisclosure
      id="experimental-thumbnail-compression"
      title="Opt-in thumbnail compression"
      description="Try new generated thumbnail codecs independently. Each experiment is off by default."
    >
      <p className="text-muted">
        Enabling a category, or applying new settings to an enabled category,
        asks for confirmation before deleting that category's existing generated
        cache and automatically rebuilding it. No library scan is required.
        Original media, other categories, and scene previews are not touched.
      </p>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && <Alert variant="info" role="status">{message}</Alert>}
      {status?.busy && (
        <Alert variant="info" role="status">
          Rebuilding {labels[status.category ?? "image"].toLowerCase()}
          {status.jobID ? ` (job #${status.jobID})` : ""}. Settings are locked
          until this task finishes.
        </Alert>
      )}

      <h4>Still-image thumbnails</h4>
      <Form.Check
        id="experimental-image-enabled"
        type="switch"
        label="Enable experimental still-image thumbnails"
        checked={enabled.image}
        disabled={disabled}
        onChange={(e) => changeEnabled("image", e.currentTarget.checked)}
      />
      <Form.Group controlId="experimental-image-thumb-format">
        <Form.Label>Thumbnail format</Form.Label>
        <Form.Control
          as="select"
          value={imageFormat}
          disabled={disabled}
          onChange={(e) =>
            setImageFormat(e.currentTarget.value as "jpg" | "webp" | "jxl")
          }
        >
          <option value="jpg">JPEG (native)</option>
          <option value="webp">WebP</option>
          <option value="jxl">JPEG XL (experimental)</option>
        </Form.Control>
      </Form.Group>
      <Form.Group controlId="experimental-image-thumb-quality">
        <Form.Label>Quality: {imageQuality}%</Form.Label>
        <Form.Control
          type="range"
          min={10}
          max={100}
          step={5}
          disabled={disabled}
          value={imageQuality}
          onChange={(e) => setImageQuality(Number(e.currentTarget.value))}
        />
      </Form.Group>
      {imageFormat === "jxl" && (
        <Alert variant="warning">
          JPEG XL requires an encoder and browser that support JXL. The
          selected format is strictly enforced; there is no WebP fallback.
        </Alert>
      )}
      {enabled.image && !unchanged.image && (
        <Button variant="outline-warning" disabled={disabled}
          onClick={() => setPending("image")}>
          Apply still-image changes and rebuild
        </Button>
      )}
      <p className="small text-muted">
        Static image thumbnails and still poster thumbnails for video-backed
        Images only. Animated GIF/WebP sources are excluded from this category.
      </p>

      <hr />
      <h4>Video clip previews</h4>
      <Form.Check
        id="experimental-video-enabled"
        type="switch"
        label="Enable experimental video clip previews"
        checked={enabled.clip}
        disabled={disabled}
        onChange={(e) => changeEnabled("clip", e.currentTarget.checked)}
      />
      <Form.Group controlId="experimental-video-preview-format">
        <Form.Label>Preview format</Form.Label>
        <Form.Control
          as="select"
          value={videoFormat}
          disabled={disabled}
          onChange={(e) => setVideoFormat(e.currentTarget.value as "webm" | "mp4")}
        >
          <option value="webm">WebM / VP9 (native)</option>
          <option value="mp4">MP4 / H.264</option>
        </Form.Control>
      </Form.Group>
      <Form.Group controlId="experimental-video-preview-quality">
        <Form.Label>Quality: {videoQuality}%</Form.Label>
        <Form.Control
          type="range"
          min={10}
          max={100}
          step={5}
          disabled={disabled}
          value={videoQuality}
          onChange={(e) => setVideoQuality(Number(e.currentTarget.value))}
        />
      </Form.Group>
      {enabled.clip && !unchanged.clip && (
        <Button variant="outline-warning" disabled={disabled}
          onClick={() => setPending("clip")}>
          Apply video preview changes and rebuild
        </Button>
      )}
      <p className="small text-muted">
        Playable previews of video-backed Images only. Does not change
        still-image thumbnails, scene previews, or original videos.
      </p>

      <hr />
      <h4>Animated-image thumbnails</h4>
      <Form.Check
        id="experimental-animated-enabled"
        type="switch"
        label="Enable experimental animated WebP thumbnails"
        checked={enabled.animated}
        disabled={disabled}
        onChange={(e) => changeEnabled("animated", e.currentTarget.checked)}
      />
      <p className="small text-muted">
        Independently replaces native animated GIF/WebP display with resized
        12-fps animated WebP thumbnail caches. Requires FFmpeg with
        libwebp_anim; unsupported decoders or encoders report errors.
      </p>
      <Form.Group controlId="experimental-animated-quality">
        <Form.Label>Animated WebP quality: {animatedQuality}%</Form.Label>
        <Form.Control
          type="range"
          min={10}
          max={100}
          step={5}
          disabled={disabled}
          value={animatedQuality}
          onChange={(e) => setAnimatedQuality(Number(e.currentTarget.value))}
        />
      </Form.Group>
      {enabled.animated && !unchanged.animated && (
        <Button variant="outline-warning" disabled={disabled}
          onClick={() => setPending("animated")}>
          Apply animated-image changes and rebuild
        </Button>
      )}

      <Modal show={pending !== null} onHide={() => !saving && setPending(null)}>
        <Modal.Header closeButton={!saving}>
          <Modal.Title>Delete and rebuild selected thumbnail cache?</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Alert variant="warning">
            This will permanently delete all existing <strong>
              {pending ? labels[pending].toLowerCase() : "selected"}
            </strong> generated cache variants and queue a full regeneration
            using the chosen format and quality.
          </Alert>
          <p>
            The rebuild may be lengthy for a large library. Some thumbnails
            may be temporarily unavailable while it runs. Originals and
            caches for the other two categories remain unchanged.
          </p>
        </Modal.Body>
        <Modal.Footer>
          <Button variant="secondary" disabled={saving}
            onClick={() => setPending(null)}>
            Cancel
          </Button>
          <Button variant="danger" disabled={saving || pending === null}
            onClick={() => pending && void apply(pending, true)}>
            Delete selected cache and rebuild
          </Button>
        </Modal.Footer>
      </Modal>
    </SettingsDisclosure>
  );
};
