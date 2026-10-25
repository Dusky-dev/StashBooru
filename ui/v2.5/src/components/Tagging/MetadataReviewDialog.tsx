import { useEffect, useMemo, useRef, useState } from "react";
import { Alert, Button, Form, Modal, Spinner } from "react-bootstrap";
import { MediaKind } from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";
import { ImageKnowledgeTagDialog } from "../Images/ImageDetails/ImageKnowledgeTagDialog";
import { VideoTaggingDialog } from "../Scenes/SceneDetails/VideoTaggingDialog";
import {
  bulkSelectedImageIDs,
  bulkTaggingRequest,
  defaultBulkTaggingCategories,
  defaultBulkTaggingSources,
  manualBulkWarningDismissed,
  MANUAL_BULK_WARNING_KEY,
  type BulkTaggingCategories,
  type BulkTaggingConfig,
  type BulkTaggingSources,
} from "./bulkTaggingPolicy";

export interface MetadataReviewItem {
  kind: MediaKind;
  id: string;
}

interface BulkTaggingJobResponse {
  jobID: number;
}

// Snapshot typed identities: a refetch/reorder must not redirect a review or write.
export function MetadataReviewDialog({
  items,
  onHide,
  onApplied,
}: {
  items: readonly MetadataReviewItem[];
  onHide: () => void;
  onApplied?: () => Promise<unknown>;
}) {
  const Toast = useToast();
  const toastRef = useRef(Toast);
  toastRef.current = Toast;
  const [selection] = useState(() => [
    ...new Map(
      items.map((item) => [`${item.kind}:${item.id}`, { ...item }])
    ).values(),
  ]);
  const bulkIDs = useMemo(
    () => bulkSelectedImageIDs(selection, MediaKind.Image),
    [selection]
  );
  const [bulkChecking, setBulkChecking] = useState(Boolean(bulkIDs));
  const [bulkConfig, setBulkConfig] = useState<BulkTaggingConfig>();
  const [manualReview, setManualReview] = useState(false);
  const [skipManualWarning, setSkipManualWarning] = useState(() => {
    try {
      return manualBulkWarningDismissed(
        typeof window === "undefined" ? undefined : window.localStorage
      );
    } catch {
      return false;
    }
  });
  const [sources, setSources] = useState<BulkTaggingSources>({
    ...defaultBulkTaggingSources,
  });
  const [categories, setCategories] = useState<BulkTaggingCategories>({
    ...defaultBulkTaggingCategories,
  });
  const [starting, setStarting] = useState(false);
  const [bulkError, setBulkError] = useState<string>();
  const [index, setIndex] = useState(0);
  const applied = useRef(false);
  const item = selection[index];

  useEffect(() => {
    if (!selection.length) onHide();
  }, [selection.length, onHide]);

  useEffect(() => {
    if (!bulkIDs) return;
    const controller = new AbortController();
    void (async () => {
      try {
        const response = await fetch("image/visual-similarity/camie/config", {
          signal: controller.signal,
        });
        if (!response.ok) {
          throw new Error((await response.text()) || response.statusText);
        }
        const config = (await response.json()) as BulkTaggingConfig;
        if (!controller.signal.aborted) setBulkConfig(config);
      } catch (error) {
        if (!controller.signal.aborted) {
          toastRef.current.error(error);
          // Do not enable unconfirmed bulk writes if settings cannot be read.
          setManualReview(true);
        }
      } finally {
        if (!controller.signal.aborted) setBulkChecking(false);
      }
    })();
    return () => controller.abort();
  }, [bulkIDs]);

  async function startBulkTagging() {
    if (!bulkIDs || !bulkConfig || starting) return;
    if (!Object.values(categories).some(Boolean)) {
      setBulkError("Choose at least one metadata category.");
      return;
    }
    if (!Object.values(sources).some(Boolean)) {
      setBulkError("Choose Camie, EVA02 or both.");
      return;
    }

    setStarting(true);
    setBulkError(undefined);
    try {
      const response = await fetch("image/visual-similarity/bulk-tag", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(
          bulkTaggingRequest(bulkIDs, bulkConfig, categories, sources)
        ),
      });
      if (!response.ok) {
        throw new Error((await response.text()) || response.statusText);
      }
      const result = (await response.json()) as BulkTaggingJobResponse;
      Toast.success(
        `Started bulk tagging for ${bulkIDs.length} Images (job #${result.jobID}).`
      );
      onHide();
    } catch (error) {
      setBulkError(error instanceof Error ? error.message : String(error));
    } finally {
      setStarting(false);
    }
  }

  function advance() {
    applied.current = false;
    if (index + 1 < selection.length) setIndex((value) => value + 1);
    else onHide();
  }
  async function didApply() {
    await onApplied?.();
    applied.current = true;
  }
  function hide() {
    if (applied.current) advance();
    else onHide();
  }

  function dismissManualWarning() {
    try {
      window.localStorage.setItem(MANUAL_BULK_WARNING_KEY, "1");
      setSkipManualWarning(true);
    } catch {
      // The current review can still proceed when browser storage is blocked.
    }
    setManualReview(true);
  }

  if (bulkChecking) {
    return (
      <Modal show centered onHide={onHide}>
        <Modal.Body className="text-center">
          <Spinner animation="border" size="sm" role="status" /> Checking
          tagging settings…
        </Modal.Body>
      </Modal>
    );
  }

  // With auto-apply disabled, explain the distinction once. The user can
  // suppress this advisory per browser without ever enabling auto writes.
  if (
    bulkIDs &&
    bulkConfig &&
    !bulkConfig.bulkTaggingEnabled &&
    !manualReview &&
    !skipManualWarning
  ) {
    return (
      <Modal show centered onHide={onHide}>
        <Modal.Header>
          <Modal.Title>Bulk tagging uses individual review</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Alert variant="warning">
            You selected {bulkIDs.length.toLocaleString()} Images, but automatic
            bulk tagging is disabled in Settings → Processing → Similarity and
            tagging. Continuing opens each Image for separate tag approval;
            nothing will be auto-applied.
          </Alert>
          <p className="small text-muted mb-0">
            To process the entire selection with one approval, enable bulk
            auto-tagging in the tagging settings.
          </p>
        </Modal.Body>
        <Modal.Footer>
          <Button variant="secondary" onClick={dismissManualWarning}>
            Don't show again
          </Button>
          <Button variant="secondary" onClick={onHide}>
            Cancel
          </Button>
          <Button variant="primary" onClick={() => setManualReview(true)}>
            OK
          </Button>
        </Modal.Footer>
      </Modal>
    );
  }

  // Multi-image auto-apply is strictly opt-in and still needs one explicit
  // confirmation per selection. Never auto-apply a lone Image or mixed media.
  if (bulkIDs && bulkConfig?.bulkTaggingEnabled && !manualReview) {
    const categoryOptions: {
      key: keyof BulkTaggingCategories;
      label: string;
    }[] = [
      { key: "applyCharacters", label: "Characters" },
      { key: "applyArtist", label: "Artists" },
      { key: "applyCopyright", label: "Copyrights" },
      { key: "applyTags", label: "General and meta Tags" },
    ];
    return (
      <Modal show centered onHide={starting ? () => {} : onHide}>
        <Modal.Header>
          <Modal.Title>Confirm bulk Image tagging</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <Alert variant="warning">
            <strong>
              You are about to tag {bulkIDs.length.toLocaleString()} selected
              Images.
            </strong>{" "}
            One approval starts a background job that applies the selected
            inference models and configured filename predictions to every
            selected Image, with no per-image review. This can create metadata
            and cannot be undone with one action. Cancelling a running job does
            not reverse work already applied.
          </Alert>
          <p className="small text-muted">
            Booru lookup and library suggestions are not part of this bulk job.
            Existing Artist links will not be replaced. Camie threshold:{" "}
            {bulkConfig.threshold}. EVA02 threshold: {bulkConfig.eva02Threshold}
            . Per-category limit: {bulkConfig.limit}.
          </p>
          <p className="mb-2">
            <strong>Inference sources:</strong>
          </p>
          <Form.Check
            type="checkbox"
            id="bulk-tagging-use-camie"
            label="Camie v2 (Characters, Artists, Copyrights and Tags)"
            checked={sources.useCamie}
            disabled={starting}
            onChange={(event) =>
              setSources((current) => ({
                ...current,
                useCamie: event.currentTarget.checked,
              }))
            }
          />
          <Form.Check
            className="mb-3"
            type="checkbox"
            id="bulk-tagging-use-eva02"
            label="EVA02 (general, rating and Character predictions)"
            checked={sources.useEva02}
            disabled={starting}
            onChange={(event) =>
              setSources((current) => ({
                ...current,
                useEva02: event.currentTarget.checked,
              }))
            }
          />
          <p className="mb-2">
            <strong>Apply these categories:</strong>
          </p>
          {categoryOptions.map(({ key, label }) => (
            <Form.Check
              key={key}
              type="checkbox"
              id={`bulk-tagging-${key}`}
              label={label}
              checked={categories[key]}
              disabled={starting}
              onChange={(event) =>
                setCategories((current) => ({
                  ...current,
                  [key]: event.currentTarget.checked,
                }))
              }
            />
          ))}
          {bulkError && (
            <Alert className="mt-3" variant="danger">
              {bulkError}
            </Alert>
          )}
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={starting}
            onClick={() => setManualReview(true)}
          >
            Review individually
          </Button>
          <Button variant="secondary" disabled={starting} onClick={onHide}>
            Cancel
          </Button>
          <Button
            variant="warning"
            disabled={
              starting ||
              !Object.values(categories).some(Boolean) ||
              !Object.values(sources).some(Boolean)
            }
            onClick={() => void startBulkTagging()}
          >
            {starting
              ? "Starting…"
              : `Confirm and tag ${bulkIDs.length.toLocaleString()} Images`}
          </Button>
        </Modal.Footer>
      </Modal>
    );
  }

  if (!item) return null;
  const props = {
    onHide: hide,
    onApplied: didApply,
    reviewContext: { index, total: selection.length, onSkip: advance },
  };
  return item.kind === MediaKind.Image ? (
    <ImageKnowledgeTagDialog
      key={`image:${item.id}`}
      imageId={item.id}
      {...props}
    />
  ) : (
    <VideoTaggingDialog key={`video:${item.id}`} sceneId={item.id} {...props} />
  );
}

export function metadataReviewOperation(
  items: readonly MetadataReviewItem[],
  showModal: (dialog: JSX.Element) => void,
  onHide: () => void
) {
  return {
    text: "Review metadata…",
    isDisplayed: () => items.length > 0,
    onClick: () =>
      showModal(<MetadataReviewDialog items={items} onHide={onHide} />),
  };
}
