import React, { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Form, Modal } from "react-bootstrap";
import { SettingsDisclosure } from "./SettingsDisclosure";
import { useSettings } from "./context";

type Category = "image" | "clip" | "animated";
type ThumbnailStatus = {
  busy: boolean;
  jobID?: number;
  jobStatus?:
    | "READY"
    | "RUNNING"
    | "STOPPING"
    | "FINISHED"
    | "FAILED"
    | "CANCELLED";
  jobError?: string;
  progress?: number;
  category?: Category;
  imageEnabled: boolean;
  clipEnabled: boolean;
  animatedEnabled: boolean;
  imageKeepOld?: boolean;
  clipKeepOld?: boolean;
  operation?: "rebuild" | "restore" | "deleteOld";
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
  const [imageKeepOld, setImageKeepOld] = useState(
    ui.thumbnailImageKeepOld ?? true
  );
  const [videoKeepOld, setVideoKeepOld] = useState(
    ui.thumbnailVideoKeepOld ?? true
  );
  const [status, setStatus] = useState<ThumbnailStatus>();
  const [pending, setPending] = useState<Category | null>(null);
  const [deletePending, setDeletePending] = useState<Category | null>(null);
  const [saving, setSaving] = useState(false);
  const lastCompletedDeletion = useRef(0);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    setImageFormat(ui.thumbnailImageFormat ?? "jpg");
    setImageQuality(ui.thumbnailImageQuality ?? 70);
    setVideoFormat(ui.thumbnailVideoFormat ?? "webm");
    setVideoQuality(ui.thumbnailVideoQuality ?? 75);
    setAnimatedQuality(ui.thumbnailAnimatedQuality ?? 70);
    setImageKeepOld(ui.thumbnailImageKeepOld ?? true);
    setVideoKeepOld(ui.thumbnailVideoKeepOld ?? true);
  }, [
    ui.thumbnailImageFormat,
    ui.thumbnailImageQuality,
    ui.thumbnailVideoFormat,
    ui.thumbnailVideoQuality,
    ui.thumbnailAnimatedQuality,
    ui.thumbnailImageKeepOld,
    ui.thumbnailVideoKeepOld,
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
    const handle = setInterval(() => {
      void refresh().catch((cause: unknown) => {
        setError(cause instanceof Error ? cause.message : String(cause));
      });
    }, 2500);
    return () => clearInterval(handle);
  }, [refresh, status?.busy]);

  // A delete-old job completes asynchronously. Sync the local Keep old
  // checkbox after completion so subsequent format changes cannot accidentally
  // claim an already-deleted native cache is still retained.
  useEffect(() => {
    if (
      !status?.busy &&
      status?.operation === "deleteOld" &&
      status.jobStatus === "FINISHED" &&
      status.jobID &&
      status.jobID !== lastCompletedDeletion.current
    ) {
      lastCompletedDeletion.current = status.jobID;
      if (status.category === "image") setImageKeepOld(false);
      if (status.category === "clip") setVideoKeepOld(false);
      refetch();
    }
  }, [
    refetch,
    status?.busy,
    status?.category,
    status?.jobID,
    status?.jobStatus,
    status?.operation,
  ]);

  const enabled: Record<Category, boolean> = {
    image: status?.imageEnabled ?? ui.thumbnailImageEnabled ?? false,
    clip: status?.clipEnabled ?? ui.thumbnailVideoEnabled ?? false,
    animated: status?.animatedEnabled ?? ui.thumbnailAnimatedEnabled ?? false,
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
  const retained = {
    image: status?.imageKeepOld ?? ui.thumbnailImageKeepOld ?? false,
    clip: status?.clipKeepOld ?? ui.thumbnailVideoKeepOld ?? false,
    animated: false,
  };

  async function apply(
    category: Category,
    turnOn: boolean,
    action: "apply" | "deleteOld" = "apply"
  ) {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const format =
        category === "image"
          ? imageFormat
          : category === "clip"
            ? videoFormat
            : "webp";
      const quality =
        category === "image"
          ? imageQuality
          : category === "clip"
            ? videoQuality
            : animatedQuality;
      const keepOld =
        category === "image"
          ? imageKeepOld
          : category === "clip"
            ? videoKeepOld
            : false;
      const response = await fetch("image/thumbnail-compression", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action,
          category,
          enabled: turnOn,
          format,
          quality,
          keepOld,
          acknowledged: turnOn || action === "deleteOld",
        }),
      });
      if (!response.ok) throw new Error(await response.text());
      const result = (await response.json()) as {
        jobID?: number;
        enabled: boolean;
      };
      setMessage(
        action === "deleteOld"
          ? `Deleting retained native ${labels[category].toLowerCase()} as job #${result.jobID}. The deleted cache cannot be restored; disable experimental mode to regenerate native previews.`
          : turnOn
            ? `Rebuilding ${labels[category].toLowerCase()} as job #${result.jobID}. Follow progress in the Task Queue.`
            : result.jobID
              ? `${labels[category]} experiment disabled. Missing native previews will be regenerated as job #${result.jobID}.`
              : `${labels[category]} experiment disabled. The retained native cache is active again.`
      );
      setPending(null);
      setDeletePending(null);
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
        Choose whether to retain the native cache before enabling compression.
        With Keep old thumbnails selected, the original JPEG/WebM files remain
        untouched, and disabling the experiment selects them again. You can
        explicitly delete retained native thumbnails to reclaim disk space.
        Changes rebuild only the chosen category; no library scan is required.
        Original media and scene previews are never deleted.
      </p>
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      {status?.jobStatus === "FAILED" && (
        <Alert variant="danger" role="alert">
          Thumbnail cache rebuild failed: {status.jobError ?? "Unknown error"}.
          Check the Task Queue. Original media were not modified.
        </Alert>
      )}
      {status?.jobStatus === "CANCELLED" && (
        <Alert variant="warning" role="status">
          Thumbnail cache rebuild was cancelled. Missing previews can still be
          generated on demand or regenerated from Tasks.
        </Alert>
      )}
      {status?.busy && (
        <Alert variant="info" role="status">
          {status.operation === "deleteOld"
            ? "Deleting retained native"
            : status.operation === "restore"
              ? "Restoring native"
              : "Rebuilding"}{" "}
          {labels[status.category ?? "image"].toLowerCase()}
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
      <Form.Check
        id="experimental-image-keep-old"
        type="checkbox"
        label="Keep old thumbnails (restore instantly when disabled)"
        checked={enabled.image ? retained.image : imageKeepOld}
        disabled={disabled || enabled.image}
        onChange={(event) => setImageKeepOld(event.currentTarget.checked)}
      />
      {enabled.image && (
        <Button
          variant="outline-danger"
          disabled={disabled || !retained.image}
          onClick={() => setDeletePending("image")}
        >
          Delete old thumbnails
        </Button>
      )}
      {enabled.image && !retained.image && (
        <Form.Text className="text-muted d-block">
          No native cache retained. Disabling regenerates missing thumbnails.
        </Form.Text>
      )}
      {imageFormat === "jxl" && (
        <Alert variant="warning">
          JPEG XL requires an encoder and browser that support JXL. The selected
          format is strictly enforced; there is no WebP fallback.
        </Alert>
      )}
      {enabled.image && !unchanged.image && (
        <Button
          variant="outline-warning"
          disabled={disabled}
          onClick={() => setPending("image")}
        >
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
          onChange={(e) =>
            setVideoFormat(e.currentTarget.value as "webm" | "mp4")
          }
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
      <Form.Check
        id="experimental-video-keep-old"
        type="checkbox"
        label="Keep old clip previews (restore instantly when disabled)"
        checked={enabled.clip ? retained.clip : videoKeepOld}
        disabled={disabled || enabled.clip}
        onChange={(event) => setVideoKeepOld(event.currentTarget.checked)}
      />
      {enabled.clip && (
        <Button
          variant="outline-danger"
          disabled={disabled || !retained.clip}
          onClick={() => setDeletePending("clip")}
        >
          Delete old clip previews
        </Button>
      )}
      {enabled.clip && !retained.clip && (
        <Form.Text className="text-muted d-block">
          No native WebM cache retained. Disabling regenerates native previews.
        </Form.Text>
      )}
      {enabled.clip && !unchanged.clip && (
        <Button
          variant="outline-warning"
          disabled={disabled}
          onClick={() => setPending("clip")}
        >
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
        12-fps animated WebP thumbnail caches. Original animated files are never
        deleted, so disabling restores native playback without a backup option.
        Requires FFmpeg with libwebp_anim.
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
        <Button
          variant="outline-warning"
          disabled={disabled}
          onClick={() => setPending("animated")}
        >
          Apply animated-image changes and rebuild
        </Button>
      )}

      <Modal show={pending !== null} onHide={() => !saving && setPending(null)}>
        <Modal.Header closeButton={!saving}>
          <Modal.Title>
            Delete and rebuild selected thumbnail cache?
          </Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Alert variant="warning">
            This will rebuild{" "}
            <strong>
              {pending ? labels[pending].toLowerCase() : "selected"}
            </strong>{" "}
            using your selected codec and quality.
            {pending === "image" && imageKeepOld
              ? " Existing native JPEG thumbnails will be retained for instant restoration."
              : pending === "clip" && videoKeepOld
                ? " Existing native WebM clip previews will be retained for instant restoration."
                : pending === "animated"
                  ? " Native animated image originals remain untouched."
                  : " Existing native cache files in this category will be deleted and require regeneration to restore."}
          </Alert>
          <p>
            The rebuild may be lengthy for a large library. Some thumbnails may
            be temporarily unavailable while it runs. Originals and caches for
            the other two categories remain unchanged.
          </p>
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={saving}
            onClick={() => setPending(null)}
          >
            Cancel
          </Button>
          <Button
            variant="danger"
            disabled={saving || pending === null}
            onClick={() => pending && void apply(pending, true)}
          >
            Confirm and rebuild selected cache
          </Button>
        </Modal.Footer>
      </Modal>
      <Modal
        show={deletePending !== null}
        onHide={() => !saving && setDeletePending(null)}
      >
        <Modal.Header closeButton={!saving}>
          <Modal.Title>Delete retained original thumbnails?</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Alert variant="danger">
            This permanently deletes the retained native{" "}
            <strong>
              {deletePending
                ? labels[deletePending].toLowerCase()
                : "thumbnail"}
            </strong>{" "}
            cache, not the experimental cache. Your original media files remain
            untouched. Disabling the experiment later will require regeneration
            instead of bringing back the old cached files.
          </Alert>
          <p>
            Only the selected category is affected. You can keep using the
            experimental thumbnails while this deletion runs.
          </p>
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={saving}
            onClick={() => setDeletePending(null)}
          >
            Cancel
          </Button>
          <Button
            variant="danger"
            disabled={saving || deletePending === null}
            onClick={() =>
              deletePending && void apply(deletePending, true, "deleteOld")
            }
          >
            Delete old thumbnails permanently
          </Button>
        </Modal.Footer>
      </Modal>
    </SettingsDisclosure>
  );
};
