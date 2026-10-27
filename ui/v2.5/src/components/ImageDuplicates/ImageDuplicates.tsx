import React, { useCallback, useEffect, useState } from "react";
import { Alert, Badge, Button, Card, Form, Modal, Spinner } from "react-bootstrap";
import { Link } from "react-router-dom";
import { Helmet } from "react-helmet";

import { useImagesDestroy } from "src/core/StashService";
import { useToast } from "src/hooks/Toast";
import {
  duplicateGroupKey,
  duplicateReviewPlan,
  keptImageIDs,
  readableFileSize,
  type SimilarImageGroup,
  type SimilarImageMember,
} from "./review";

import "./styles.scss";

interface ClusterResponse {
  threshold: number;
  indexedImages: number;
  clusters: SimilarImageGroup[];
}

interface SimilarityStatus {
  indexedImages: number;
  totalImages: number;
}

interface MetadataCopyResponse {
  results: { imageID: number; error?: string }[];
}

const pageSize = 12;

async function jsonResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || "The duplicate review request failed.");
  }
  return (await response.json()) as T;
}

export default function ImageDuplicates() {
  const Toast = useToast();
  const [threshold, setThreshold] = useState("0.94");
  const [appliedThreshold, setAppliedThreshold] = useState("0.94");
  const [reloadToken, setReloadToken] = useState(0);
  const [result, setResult] = useState<ClusterResponse>();
  const [status, setStatus] = useState<SimilarityStatus>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [shown, setShown] = useState(pageSize);
  const [hidden, setHidden] = useState<string[]>([]);
  const [selections, setSelections] = useState<Record<string, number[]>>({});
  const [previewGroup, setPreviewGroup] = useState<SimilarImageGroup>();
  const [reviewGroup, setReviewGroup] = useState<SimilarImageGroup>();
  const [deleteFiles, setDeleteFiles] = useState(false);
  const [copyMetadata, setCopyMetadata] = useState(true);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setShown(pageSize);
    setHidden([]);
    setSelections({});
    void Promise.allSettled([
      fetch("image/visual-similarity/status", { signal: controller.signal }).then(
        jsonResponse<SimilarityStatus>
      ),
      fetch(
        "image/visual-similarity/clusters?threshold=" +
          appliedThreshold +
          "&neighbors=20",
        { signal: controller.signal }
      ).then(jsonResponse<ClusterResponse>),
    ]).then(([statusResponse, clustersResponse]) => {
      if (controller.signal.aborted) return;
      setStatus(statusResponse.status === "fulfilled" ? statusResponse.value : undefined);
      if (clustersResponse.status === "fulfilled") {
        setResult(clustersResponse.value);
      } else {
        setResult(undefined);
        setError(String(clustersResponse.reason));
      }
      setLoading(false);
    });
    return () => controller.abort();
  }, [appliedThreshold, reloadToken]);

  const visibleGroups = (result?.clusters ?? []).filter(
    (group) => !hidden.includes(duplicateGroupKey(group))
  );
  const displayedGroups = visibleGroups.slice(0, shown);
  const totalCandidates = visibleGroups.reduce(
    (total, group) => total + group.members.length,
    0
  );

  const selectionFor = useCallback(
    (group: SimilarImageGroup) =>
      keptImageIDs(group, selections[duplicateGroupKey(group)]),
    [selections]
  );

  const selectKeep = (group: SimilarImageGroup, imageID: number) => {
    const key = duplicateGroupKey(group);
    setSelections((old) => {
      const current = keptImageIDs(group, old[key]);
      const next = current.includes(imageID)
        ? current.filter((id) => id !== imageID)
        : [...current, imageID];
      // A review must always retain at least one image.
      return next.length ? { ...old, [key]: next } : old;
    });
  };

  const selectedPlan = reviewGroup
    ? duplicateReviewPlan(
        reviewGroup,
        selections[duplicateGroupKey(reviewGroup)]
      )
    : { keep: [], remove: [] };

  const [destroyImages] = useImagesDestroy({
    ids: selectedPlan.remove.map(String),
    delete_file: deleteFiles,
    delete_generated: true,
  });

  const dismissGroup = (group: SimilarImageGroup) => {
    const key = duplicateGroupKey(group);
    setHidden((current) => [...current, key]);
  };

  const confirmReview = async () => {
    if (!reviewGroup || busy || selectedPlan.keep.length === 0 || selectedPlan.remove.length === 0) {
      return;
    }
    setBusy(true);
    try {
      if (copyMetadata && selectedPlan.keep.length === 1) {
        // Copy source metadata before deleting it, and abort deletion if any
        // transfer fails. The existing API merges rather than replaces links.
        for (const imageID of selectedPlan.remove) {
          const copied = await fetch(
            "image/visual-similarity/clusters/copy-metadata",
            {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({
                sourceImageID: imageID,
                targetImageIDs: selectedPlan.keep,
              }),
            }
          ).then(jsonResponse<MetadataCopyResponse>);
          if (copied.results.some((item) => item.error)) {
            throw new Error(
              copied.results.find((item) => item.error)?.error ||
                "Could not transfer image metadata."
            );
          }
        }
      }
      const deleted = await destroyImages();
      if (!deleted.data?.imagesDestroy) {
        throw new Error("Stash did not confirm the selected images were deleted.");
      }
      dismissGroup(reviewGroup);
      setReviewGroup(undefined);
      Toast.success(
        selectedPlan.remove.length + " duplicate image(s) removed from the library."
      );
    } catch (cause) {
      Toast.error(cause);
    } finally {
      setBusy(false);
    }
  };

  function previewImage(member: SimilarImageMember, large = false) {
    return (
      <img
        src={"image/" + member.id + (large ? "/preview" : "/thumbnail")}
        alt={"Image " + member.id}
        loading="lazy"
      />
    );
  }

  return (
    <main id="image-duplicate-review" className="container-fluid">
      <Helmet>
        <title>Review similar images · StashBooru</title>
      </Helmet>
      <div className="duplicate-review-heading">
        <div>
          <h1>Review similar images</h1>
          <p className="text-muted">
            Compare visually similar images side by side. Choose which originals
            to keep before removing any others, like Immich's duplicate review.
          </p>
        </div>
        <Link to="/settings?tab=system" className="btn btn-secondary">
          Similarity settings
        </Link>
      </div>

      <Card className="duplicate-review-controls">
        <Card.Body>
          <div className="duplicate-review-toolbar">
            <Form.Group controlId="duplicate-threshold" className="mb-0">
              <Form.Label>Similarity threshold</Form.Label>
              <Form.Control
                as="select"
                value={threshold}
                disabled={loading || busy}
                onChange={(event) => setThreshold(event.currentTarget.value)}
              >
                <option value="0.97">Very close (97%)</option>
                <option value="0.95">Close (95%)</option>
                <option value="0.94">Balanced (94%)</option>
                <option value="0.92">Broader (92%)</option>
                <option value="0.88">More related images (88%)</option>
              </Form.Control>
            </Form.Group>
            <Button
              disabled={loading || busy}
              onClick={() => {
                if (appliedThreshold !== threshold) setAppliedThreshold(threshold);
                else setReloadToken((value) => value + 1);
              }}
            >
              {loading ? "Searching…" : "Find similar images"}
            </Button>
            <div className="duplicate-review-stat">
              {status
                ? status.indexedImages.toLocaleString() +
                  " / " +
                  status.totalImages.toLocaleString() +
                  " images indexed"
                : "Checking visual similarity index…"}
              {result && (
                <span>
                  {visibleGroups.length.toLocaleString()} groups ·{" "}
                  {totalCandidates.toLocaleString()} images
                </span>
              )}
            </div>
          </div>
        </Card.Body>
      </Card>

      <Alert variant="info" className="mt-3">
        Similarity uses the existing EVA02 visual index. Groups are suggestions,
        not proof of identical content. Review carefully before deleting.
        Missing results? <Link to="/settings?tab=system">Generate visual embeddings</Link>{" "}
        under Visual Similarity settings.
      </Alert>

      {loading && (
        <div className="duplicate-review-loading" role="status">
          <Spinner animation="border" className="mr-3" />
          Comparing indexed images and building groups…
        </div>
      )}
      {!loading && error && (
        <Alert variant="warning" role="alert">
          <strong>Duplicate search unavailable.</strong> {error}
          <p className="mt-2 mb-0">
            Install the EVA02 model and generate image embeddings in{" "}
            <Link to="/settings?tab=system">Similarity settings</Link>.
          </p>
        </Alert>
      )}
      {!loading && result && visibleGroups.length === 0 && (
        <Alert variant="success">
          No similar image groups to review at this threshold.
          {hidden.length > 0 && (
            <Button
              variant="link"
              onClick={() => setHidden([])}
              className="ml-2"
            >
              Show hidden groups
            </Button>
          )}
        </Alert>
      )}

      {!loading &&
        displayedGroups.map((group, index) => {
          const key = duplicateGroupKey(group);
          const plan = duplicateReviewPlan(group, selections[key]);
          return (
            <Card className="duplicate-review-group" key={key}>
              <Card.Header className="duplicate-review-group-heading">
                <div>
                  <strong>Group {index + 1}</strong>
                  <span className="text-muted ml-2">
                    {group.members.length} similar images
                  </span>
                </div>
                <div className="duplicate-review-group-actions">
                  <Button
                    size="sm"
                    variant="outline-secondary"
                    onClick={() =>
                      setSelections((old) => ({ ...old, [key]: [group.preferredID] }))
                    }
                  >
                    Keep suggested
                  </Button>
                  <Button
                    size="sm"
                    variant="outline-secondary"
                    onClick={() =>
                      setSelections((old) => ({
                        ...old,
                        [key]: group.members.map((member) => member.id),
                      }))
                    }
                  >
                    Keep all
                  </Button>
                  <Button
                    size="sm"
                    variant="outline-secondary"
                    onClick={() => dismissGroup(group)}
                    title="Hide this group until the next search; no files are changed"
                  >
                    Skip
                  </Button>
                </div>
              </Card.Header>
              <Card.Body>
                <div className="duplicate-review-images">
                  {group.members.map((member) => {
                    const kept = plan.keep.includes(member.id);
                    return (
                      <div
                        className={
                          "duplicate-review-image" +
                          (kept ? " duplicate-review-kept" : " duplicate-review-removed")
                        }
                        key={member.id}
                      >
                        <button
                          type="button"
                          className="duplicate-review-preview"
                          onClick={() => setPreviewGroup(group)}
                          title="Compare all full-size previews"
                          aria-label={"Compare image " + member.id}
                        >
                          {previewImage(member)}
                        </button>
                        <div className="duplicate-review-image-details">
                          <div className="duplicate-review-image-id">
                            <Link to={"/images/" + member.id} target="_blank">
                              Image #{member.id}
                            </Link>
                            {member.preferred && (
                              <Badge variant="success">Suggested</Badge>
                            )}
                          </div>
                          <span className="text-muted">
                            {member.width > 0 && member.height > 0
                              ? member.width + " × " + member.height
                              : "Unknown resolution"}
                            {" · "}
                            {readableFileSize(member.fileSize)}
                          </span>
                          <Form.Check
                            type="checkbox"
                            id={"duplicate-keep-" + key + "-" + member.id}
                            checked={kept}
                            label={kept ? "Keep" : "Remove"}
                            onChange={() => selectKeep(group, member.id)}
                          />
                        </div>
                      </div>
                    );
                  })}
                </div>
                <div className="duplicate-review-footer">
                  <span className="text-muted">
                    Keeping {plan.keep.length}, removing {plan.remove.length}
                  </span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => setPreviewGroup(group)}
                  >
                    Compare large previews
                  </Button>
                  <Button
                    size="sm"
                    variant="danger"
                    disabled={plan.remove.length === 0 || busy}
                    onClick={() => {
                      setDeleteFiles(false);
                      setCopyMetadata(true);
                      setReviewGroup(group);
                    }}
                  >
                    Review removal
                  </Button>
                </div>
              </Card.Body>
            </Card>
          );
        })}

      {!loading && visibleGroups.length > shown && (
        <div className="text-center mb-4">
          <Button variant="secondary" onClick={() => setShown((n) => n + pageSize)}>
            Show more groups ({visibleGroups.length - shown} remaining)
          </Button>
        </div>
      )}
      {!loading && hidden.length > 0 && (
        <p className="text-center text-muted">
          {hidden.length} group(s) skipped in this session.{" "}
          <Button size="sm" variant="link" onClick={() => setHidden([])}>
            Show skipped groups
          </Button>
        </p>
      )}

      <Modal
        size="xl"
        show={!!previewGroup}
        onHide={() => setPreviewGroup(undefined)}
        centered
        dialogClassName="duplicate-review-preview-modal"
      >
        <Modal.Header closeButton>
          <Modal.Title>Compare large previews</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <div className="duplicate-review-large-images">
            {previewGroup?.members.map((member) => (
              <div key={member.id}>
                {previewImage(member, true)}
                <Link to={"/images/" + member.id} target="_blank">
                  Image #{member.id} · {member.width} × {member.height} ·{" "}
                  {readableFileSize(member.fileSize)}
                </Link>
              </div>
            ))}
          </div>
        </Modal.Body>
      </Modal>

      <Modal
        show={!!reviewGroup}
        onHide={() => !busy && setReviewGroup(undefined)}
        centered
      >
        <Modal.Header closeButton={!busy}>
          <Modal.Title>Review duplicate removal</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p>
            <strong>Keep {selectedPlan.keep.length}</strong> image(s), and remove{" "}
            <strong>{selectedPlan.remove.length}</strong> selected image(s) from
            Stash. This action cannot be undone from the review page.
          </p>
          <p className="text-muted">
            Kept: {selectedPlan.keep.join(", ")}
            <br />
            Removed: {selectedPlan.remove.join(", ")}
          </p>
          {selectedPlan.keep.length === 1 && (
            <Form.Check
              type="checkbox"
              id="duplicate-copy-metadata"
              label="Copy tags, artists, characters and copyrights to the kept image first"
              checked={copyMetadata}
              disabled={busy}
              onChange={(event) => setCopyMetadata(event.currentTarget.checked)}
            />
          )}
          <Form.Check
            className="mt-3"
            type="checkbox"
            id="duplicate-delete-files"
            label="Also delete original image files (not just Stash records)"
            checked={deleteFiles}
            disabled={busy}
            onChange={(event) => setDeleteFiles(event.currentTarget.checked)}
          />
          <Alert variant={deleteFiles ? "danger" : "warning"} className="mt-3">
            {deleteFiles
              ? "The selected original files will be sent to Stash's configured trash, or permanently deleted if no trash is configured."
              : "Original files will remain on disk and may reappear during a future library scan. Enable file deletion to reclaim space."}
          </Alert>
        </Modal.Body>
        <Modal.Footer>
          <Button variant="secondary" disabled={busy} onClick={() => setReviewGroup(undefined)}>
            Cancel
          </Button>
          <Button
            variant="danger"
            disabled={busy || selectedPlan.remove.length === 0}
            onClick={() => void confirmReview()}
          >
            {busy ? "Removing…" : "Confirm removal"}
          </Button>
        </Modal.Footer>
      </Modal>
    </main>
  );
}
