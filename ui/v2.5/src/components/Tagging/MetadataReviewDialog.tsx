import { useEffect, useMemo, useRef, useState } from "react";
import { Alert, Button, Form, Modal, Spinner } from "react-bootstrap";
import { type IntlShape, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { MediaKind } from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";
import { ImageKnowledgeTagDialog } from "../Images/ImageDetails/ImageKnowledgeTagDialog";
import { VideoTaggingDialog } from "../Scenes/SceneDetails/VideoTaggingDialog";
import {
  allowedBulkTaggingSources,
  availableBulkTaggingSources,
  bulkSelectedImageIDs,
  bulkTaggingRequest,
  defaultBulkTaggingCategories,
  taggingActionKind,
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
  const intl = useIntl();
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
  const [sources, setSources] = useState<BulkTaggingSources>({
    useFilename: false,
    useCamie: false,
    useEva02: false,
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
        if (!controller.signal.aborted) {
          setBulkConfig(config);
          setSources(availableBulkTaggingSources(config));
        }
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
      setBulkError(
        intl.formatMessage({ id: "media_tagging.error_no_category" })
      );
      return;
    }
    if (
      !Object.values(allowedBulkTaggingSources(sources, bulkConfig)).some(
        Boolean
      )
    ) {
      setBulkError(intl.formatMessage({ id: "media_tagging.error_no_source" }));
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
        intl.formatMessage(
          { id: "media_tagging.started" },
          { count: bulkIDs.length, job: result.jobID }
        )
      );
    } catch (error) {
      setBulkError(error instanceof Error ? error.message : String(error));
      setStarting(false);
      return;
    }
    // Closing unmounts this dialog; never reset its state after onHide().
    onHide();
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

  if (bulkChecking) {
    return (
      <Modal show centered onHide={onHide}>
        <Modal.Body className="text-center">
          <Spinner animation="border" size="sm" role="status" />{" "}
          {intl.formatMessage({ id: "media_tagging.checking" })}
        </Modal.Body>
      </Modal>
    );
  }

  // One chooser for every Image-only multi-selection: tag the selection in
  // one confirmed job with the sources the settings allow, or review each
  // Image. A lone Image or mixed media always uses individual review.
  if (bulkIDs && bulkConfig && !manualReview) {
    const available = availableBulkTaggingSources(bulkConfig);
    const chosen = allowedBulkTaggingSources(sources, bulkConfig);
    const anyAvailable = Object.values(available).some(Boolean);
    const usesModels = chosen.useCamie || chosen.useEva02;
    const message = (id: string, values?: Record<string, unknown>) =>
      intl.formatMessage({ id: `media_tagging.${id}` }, values as never);
    const sourceOptions: {
      key: keyof BulkTaggingSources;
      id: string;
      label: string;
    }[] = [
      {
        key: "useFilename",
        id: "bulk-tagging-use-filename",
        label: message("source_filename"),
      },
      {
        key: "useCamie",
        id: "bulk-tagging-use-camie",
        label: message("source_camie"),
      },
      {
        key: "useEva02",
        id: "bulk-tagging-use-eva02",
        label: message("source_eva02"),
      },
    ];
    const categoryOptions: {
      key: keyof BulkTaggingCategories;
      label: string;
    }[] = [
      { key: "applyCharacters", label: message("category_characters") },
      { key: "applyArtist", label: message("category_artists") },
      { key: "applyCopyright", label: message("category_copyrights") },
      { key: "applyTags", label: message("category_tags") },
    ];
    const count = bulkIDs.length;
    return (
      <Modal show centered onHide={starting ? () => {} : onHide}>
        <Modal.Header>
          <Modal.Title>{message("bulk_title", { count })}</Modal.Title>
        </Modal.Header>
        <Modal.Body className="bulk-tagging-chooser">
          <p>{message("bulk_intro")}</p>
          <p className="mb-2">
            <strong>{message("sources")}</strong>
          </p>
          {sourceOptions.map(({ key, id, label }) => (
            <Form.Check
              key={key}
              type="checkbox"
              id={id}
              label={label}
              checked={chosen[key]}
              disabled={starting || !available[key]}
              onChange={(event) => {
                const checked = event.currentTarget.checked;
                setSources((current) => ({ ...current, [key]: checked }));
              }}
            />
          ))}
          {!available.useFilename && (
            <p className="small text-muted mb-1">
              {message("source_filename_off")}
            </p>
          )}
          {!bulkConfig.bulkTaggingEnabled && (
            <p className="small text-muted mb-1">{message("models_off")}</p>
          )}
          {usesModels && (
            <p className="small text-muted mb-1">
              {message("model_settings", {
                camie: bulkConfig.threshold,
                eva02: bulkConfig.eva02Threshold,
                limit: bulkConfig.limit,
              })}
            </p>
          )}
          <p className="small mb-3">
            <Link
              target="_blank"
              rel="noreferrer"
              to="/settings?tab=processing#visual-similarity"
            >
              {message("setup_link")}
            </Link>
          </p>
          {anyAvailable ? (
            <>
              <p className="mb-2">
                <strong>{message("categories")}</strong>
              </p>
              {categoryOptions.map(({ key, label }) => (
                <Form.Check
                  key={key}
                  type="checkbox"
                  id={`bulk-tagging-${key}`}
                  label={label}
                  checked={categories[key]}
                  disabled={starting}
                  onChange={(event) => {
                    const checked = event.currentTarget.checked;
                    setCategories((current) => ({
                      ...current,
                      [key]: checked,
                    }));
                  }}
                />
              ))}
              <Alert className="mt-3 mb-0" variant="warning">
                {message("bulk_warning")}
              </Alert>
              <p className="small text-muted mt-2 mb-0">
                {message("manual_only")}
              </p>
            </>
          ) : (
            <Alert variant="info" className="mb-0">
              {message("no_bulk_source")}
            </Alert>
          )}
          {bulkError && (
            <Alert className="mt-3" variant="danger">
              {bulkError}
            </Alert>
          )}
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant={anyAvailable ? "secondary" : "primary"}
            disabled={starting}
            onClick={() => setManualReview(true)}
          >
            {message("review_individually")}
          </Button>
          <Button variant="secondary" disabled={starting} onClick={onHide}>
            {intl.formatMessage({ id: "actions.cancel" })}
          </Button>
          {anyAvailable && (
            <Button
              variant="warning"
              disabled={
                starting ||
                !Object.values(categories).some(Boolean) ||
                !Object.values(chosen).some(Boolean)
              }
              onClick={() => void startBulkTagging()}
            >
              {starting ? message("starting") : message("confirm", { count })}
            </Button>
          )}
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
  onHide: () => void,
  intl: IntlShape
) {
  return {
    text: intl.formatMessage({
      id: `media_tagging.action_${taggingActionKind(items, MediaKind.Image)}`,
    }),
    isDisplayed: () => items.length > 0,
    onClick: () =>
      showModal(<MetadataReviewDialog items={items} onHide={onHide} />),
  };
}
