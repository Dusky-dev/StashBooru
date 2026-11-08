import { useEffect, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Card,
  Form,
  Modal,
  Spinner,
} from "react-bootstrap";
import { Link, useLocation } from "react-router-dom";
import { Helmet } from "react-helmet";

import { useImagesDestroy } from "src/core/StashService";
import { useToast } from "src/hooks/Toast";
import { DuplicateComparison } from "./DuplicateComparison";
import {
  defaultDuplicateMethod,
  duplicateClusterURL,
  duplicateGroupKey,
  type DuplicateMatchMethod,
  duplicateReviewPlan,
  keptImageIDs,
  readableFileSize,
  similarityMetadataCopyPlan,
  type SimilarImageGroup,
  type SimilarImageMember,
} from "./review";

import "./styles.scss";

interface ClusterResponse {
  method: DuplicateMatchMethod;
  indexedImages: number;
  totalImages: number;
  clusters: SimilarImageGroup[];
}

interface MetadataCopyCounts {
  characters: number;
  artists: number;
  copyrights: number;
  tags: number;
}

interface MetadataCopyResponse {
  results: {
    imageID: number;
    added: MetadataCopyCounts;
    error?: string;
  }[];
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
  const { search: locationSearch } = useLocation();
  const initialMethod =
    new URLSearchParams(locationSearch).get("method") === "embedding"
      ? "embedding"
      : defaultDuplicateMethod;
  const [search, setSearch] = useState({
    method: initialMethod as DuplicateMatchMethod,
    distance: "5",
    threshold: "0.94",
  });
  const [appliedSearch, setAppliedSearch] = useState(search);
  const [reloadToken, setReloadToken] = useState(0);
  const [result, setResult] = useState<ClusterResponse>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [shown, setShown] = useState(pageSize);
  const [hidden, setHidden] = useState<string[]>([]);
  const [selections, setSelections] = useState<Record<string, number[]>>({});
  const [previewGroup, setPreviewGroup] = useState<{
    group: SimilarImageGroup;
    selectedID: number;
  }>();
  const [reviewGroup, setReviewGroup] = useState<SimilarImageGroup>();
  const [deleteFiles, setDeleteFiles] = useState(false);
  const [copyMetadata, setCopyMetadata] = useState(true);
  const [busy, setBusy] = useState(false);
  const [metadataReview, setMetadataReview] = useState<{
    group: SimilarImageGroup;
    sourceID: number;
    targetIDs: number[];
  }>();
  const [metadataBusy, setMetadataBusy] = useState(false);
  const [metadataCopyError, setMetadataCopyError] = useState("");
  const [metadataCopyReport, setMetadataCopyReport] = useState<MetadataCopyResponse>();

  // biome-ignore lint/correctness/useExhaustiveDependencies: reloadToken explicitly refreshes the server-derived similarity groups.
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setShown(pageSize);
    setHidden([]);
    setSelections({});
    // pHash searches use existing fingerprints and never contact EVA02.
    // The cluster response includes index totals for the selected mode.
    void fetch(
      duplicateClusterURL(
        appliedSearch.method,
        appliedSearch.distance,
        appliedSearch.threshold
      ),
      { signal: controller.signal }
    )
      .then(jsonResponse<ClusterResponse>)
      .then((value) => {
        if (controller.signal.aborted) return;
        setResult(value);
        setLoading(false);
      })
      .catch((reason) => {
        if (controller.signal.aborted) return;
        setResult(undefined);
        setError(String(reason));
        setLoading(false);
      });
    return () => controller.abort();
  }, [appliedSearch, reloadToken]);

  const visibleGroups = (result?.clusters ?? []).filter(
    (group) => !hidden.includes(duplicateGroupKey(group))
  );
  const displayedGroups = visibleGroups.slice(0, shown);
  const totalCandidates = visibleGroups.reduce(
    (total, group) => total + group.members.length,
    0
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
    if (
      !reviewGroup ||
      busy ||
      selectedPlan.keep.length === 0 ||
      selectedPlan.remove.length === 0
    ) {
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
        throw new Error(
          "Stash did not confirm the selected images were deleted."
        );
      }
      dismissGroup(reviewGroup);
      setReviewGroup(undefined);
      Toast.success(
        selectedPlan.remove.length +
          " duplicate image(s) removed from the library."
      );
    } catch (cause) {
      Toast.error(cause);
    } finally {
      setBusy(false);
    }
  };

  const metadataPlan = metadataReview
    ? similarityMetadataCopyPlan(
        metadataReview.group,
        metadataReview.sourceID,
        metadataReview.targetIDs
      )
    : { sourceImageID: 0, targetImageIDs: [] };

  const startMetadataCopy = (group: SimilarImageGroup) => {
    setMetadataReview({
      group,
      sourceID: group.members.some((item) => item.id === group.preferredID)
        ? group.preferredID
        : group.members[0].id,
      targetIDs: [],
    });
    setMetadataCopyError("");
    setMetadataCopyReport(undefined);
  };

  const confirmMetadataCopy = async () => {
    if (
      !metadataReview ||
      metadataBusy ||
      busy ||
      metadataPlan.sourceImageID <= 0 ||
      metadataPlan.targetImageIDs.length === 0
    ) {
      return;
    }
    setMetadataBusy(true);
    setMetadataCopyError("");
    setMetadataCopyReport(undefined);
    try {
      const response = await fetch(
        "image/visual-similarity/clusters/copy-metadata",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(metadataPlan),
        }
      ).then(jsonResponse<MetadataCopyResponse>);
      const failures = response.results.filter((item) => item.error);
      const confirmed = new Set(response.results.map((item) => item.imageID));
      const missing = metadataPlan.targetImageIDs.filter(
        (id) => !confirmed.has(id)
      );
      if (failures.length > 0 || missing.length > 0) {
        setMetadataCopyReport(response);
        setMetadataCopyError(
          "Some images could not be updated. Review the failed targets before retrying."
        );
        return;
      }
      const totals = response.results.reduce(
        (sum, item) => ({
          characters: sum.characters + item.added.characters,
          artists: sum.artists + item.added.artists,
          copyrights: sum.copyrights + item.added.copyrights,
          tags: sum.tags + item.added.tags,
        }),
        { characters: 0, artists: 0, copyrights: 0, tags: 0 }
      );
      Toast.success(
        `Added ${totals.characters} Characters, ${totals.artists} Artists, ${totals.copyrights} Copyrights and ${totals.tags} Tags to ${response.results.length} image(s).`
      );
      setMetadataReview(undefined);
    } catch (cause) {
      setMetadataCopyError(String(cause));
    } finally {
      setMetadataBusy(false);
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
            <Form.Group controlId="duplicate-method" className="mb-0">
              <Form.Label>Match using</Form.Label>
              <Form.Control
                as="select"
                value={search.method}
                disabled={loading || busy}
                onChange={(event) => {
                  const method = event.currentTarget
                    .value as DuplicateMatchMethod;
                  const next = { ...search, method };
                  setSearch(next);
                  setAppliedSearch(next);
                  setResult(undefined);
                }}
              >
                <option value="phash">pHash — duplicate images</option>
                <option value="embedding">EVA02 — related content</option>
              </Form.Control>
            </Form.Group>
            {search.method === "phash" ? (
              <Form.Group controlId="duplicate-distance" className="mb-0">
                <Form.Label>Maximum pHash difference</Form.Label>
                <Form.Control
                  as="select"
                  value={search.distance}
                  disabled={loading || busy}
                  onChange={(event) =>
                    setSearch({
                      ...search,
                      distance: event.currentTarget.value,
                    })
                  }
                >
                  <option value="0">Exact pHash (0 bits)</option>
                  <option value="2">Very close (2 bits)</option>
                  <option value="5">Balanced (5 bits)</option>
                  <option value="8">More variants (8 bits)</option>
                </Form.Control>
              </Form.Group>
            ) : (
              <Form.Group controlId="duplicate-threshold" className="mb-0">
                <Form.Label>EVA02 similarity threshold</Form.Label>
                <Form.Control
                  as="select"
                  value={search.threshold}
                  disabled={loading || busy}
                  onChange={(event) =>
                    setSearch({
                      ...search,
                      threshold: event.currentTarget.value,
                    })
                  }
                >
                  <option value="0.97">Very close (97%)</option>
                  <option value="0.95">Close (95%)</option>
                  <option value="0.94">Balanced (94%)</option>
                  <option value="0.92">Broader (92%)</option>
                  <option value="0.88">More related images (88%)</option>
                </Form.Control>
              </Form.Group>
            )}
            <Button
              disabled={loading || busy}
              onClick={() => {
                if (
                  appliedSearch.method !== search.method ||
                  (search.method === "phash"
                    ? appliedSearch.distance !== search.distance
                    : appliedSearch.threshold !== search.threshold)
                ) {
                  setAppliedSearch({ ...search });
                } else {
                  setReloadToken((value) => value + 1);
                }
              }}
            >
              {loading ? "Searching…" : "Find similar images"}
            </Button>
            <div className="duplicate-review-stat">
              {result
                ? result.indexedImages.toLocaleString() +
                  " / " +
                  result.totalImages.toLocaleString() +
                  (result.method === "phash"
                    ? " images hashed"
                    : " images indexed")
                : loading
                  ? "Checking image index…"
                  : "Image index unavailable"}
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
        {appliedSearch.method === "phash" ? (
          <>
            pHash compares the current primary-file fingerprints to find exact
            and near-duplicate images, including resized or recompressed copies.
            No EVA02 model is required. Missing fingerprints? Generate Image
            perceptual hashes under <Link to="/settings?tab=tasks">Tasks</Link>.
          </>
        ) : (
          <>
            EVA02 finds related subjects and compositions, not necessarily
            duplicates. Approximate candidate search may miss borderline
            matches. Review carefully before deleting. Missing results? Generate
            visual embeddings in{" "}
            <Link to="/settings?tab=system">Similarity settings</Link>.
          </>
        )}
      </Alert>

      {loading && (
        <div className="duplicate-review-loading" role="status">
          <Spinner animation="border" className="mr-3" />
          {appliedSearch.method === "phash"
            ? "Grouping current image perceptual hashes…"
            : "Scanning indexed images and comparing likely matches…"}
        </div>
      )}
      {!loading && error && (
        <Alert variant="warning" role="alert">
          <strong>Duplicate search unavailable.</strong> {error}
          <p className="mt-2 mb-0">
            {appliedSearch.method === "phash" ? (
              <>
                Could not read Image perceptual hashes. Check server logs and
                retry; generate missing hashes under{" "}
                <Link to="/settings?tab=tasks">Tasks</Link>.
              </>
            ) : (
              <>
                The similarity index could not be read. Existing EVA02
                embeddings may still be available; check server logs and retry.
              </>
            )}
          </p>
        </Alert>
      )}
      {!loading && result && visibleGroups.length === 0 && (
        <Alert variant="success">
          {appliedSearch.method === "phash"
            ? result.indexedImages === 0
              ? "No primary-file pHashes found. Generate Image perceptual hashes under Tasks."
              : "No duplicate groups found at this pHash distance."
            : "No related image groups found at this EVA02 threshold."}
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
                      setSelections((old) => ({
                        ...old,
                        [key]: [group.preferredID],
                      }))
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
                          (kept
                            ? " duplicate-review-kept"
                            : " duplicate-review-removed")
                        }
                        key={member.id}
                      >
                        <button
                          type="button"
                          className="duplicate-review-preview"
                          onClick={() =>
                            setPreviewGroup({ group, selectedID: member.id })
                          }
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
                    onClick={() =>
                      setPreviewGroup({ group, selectedID: group.preferredID })
                    }
                  >
                    Compare large previews
                  </Button>
                  <Button
                    size="sm"
                    variant="outline-secondary"
                    disabled={busy || metadataBusy}
                    onClick={() => startMetadataCopy(group)}
                  >
                    Copy missing metadata
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
          <Button
            variant="secondary"
            onClick={() => setShown((n) => n + pageSize)}
          >
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

      {previewGroup && (
        <DuplicateComparison
          key={duplicateGroupKey(previewGroup.group)}
          group={previewGroup.group}
          initialImageID={previewGroup.selectedID}
          keptIDs={keptImageIDs(
            previewGroup.group,
            selections[duplicateGroupKey(previewGroup.group)]
          )}
          onToggleKeep={(id) => selectKeep(previewGroup.group, id)}
          onClose={() => setPreviewGroup(undefined)}
        />
      )}

      <Modal
        show={!!metadataReview}
        onHide={() => !metadataBusy && setMetadataReview(undefined)}
        centered
      >
        <Modal.Header closeButton={!metadataBusy}>
          <Modal.Title>Copy missing image metadata</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p className="text-muted">
            Copy existing Characters, Artists, Copyrights and Tags from one image
            to the selected images. This only adds missing relationships; it
            never replaces metadata, removes images or changes files.
          </p>
          {metadataReview && (
            <>
              <Form.Group controlId="similarity-metadata-source">
                <Form.Label>Copy metadata from</Form.Label>
                <Form.Control
                  as="select"
                  value={metadataReview.sourceID}
                  disabled={metadataBusy}
                  onChange={(event) => {
                    const sourceID = Number(event.currentTarget.value);
                    setMetadataReview((current) =>
                      current ? { ...current, sourceID, targetIDs: [] } : current
                    );
                    setMetadataCopyError("");
                    setMetadataCopyReport(undefined);
                  }}
                >
                  {metadataReview.group.members.map((member) => (
                    <option key={member.id} value={member.id}>
                      Image #{member.id}
                      {member.preferred ? " (suggested)" : ""}
                    </option>
                  ))}
                </Form.Control>
              </Form.Group>
              <div className="d-flex align-items-center justify-content-between mb-2">
                <strong>Copy missing metadata to</strong>
                <Button
                  variant="link"
                  size="sm"
                  disabled={metadataBusy}
                  onClick={() =>
                    setMetadataReview((current) =>
                      current
                        ? {
                            ...current,
                            targetIDs: current.group.members
                              .map((member) => member.id)
                              .filter((id) => id !== current.sourceID),
                          }
                        : current
                    )
                  }
                >
                  Select all others
                </Button>
              </div>
              {metadataReview.group.members
                .filter((member) => member.id !== metadataReview.sourceID)
                .map((member) => (
                  <Form.Check
                    key={member.id}
                    type="checkbox"
                    className="mb-2"
                    id={`similarity-metadata-target-${member.id}`}
                    label={`Image #${member.id}`}
                    checked={metadataReview.targetIDs.includes(member.id)}
                    disabled={metadataBusy}
                    onChange={(event) => {
                      const checked = event.currentTarget.checked;
                      setMetadataReview((current) =>
                        current
                          ? {
                              ...current,
                              targetIDs: checked
                                ? [...current.targetIDs, member.id]
                                : current.targetIDs.filter(
                                    (id) => id !== member.id
                                  ),
                            }
                          : current
                      );
                      setMetadataCopyError("");
                      setMetadataCopyReport(undefined);
                    }}
                  />
                ))}
            </>
          )}
          {metadataCopyError && (
            <Alert variant="danger" role="alert">
              {metadataCopyError}
              {metadataCopyReport?.results
                .filter((item) => item.error)
                .map((item) => (
                  <div key={item.imageID}>
                    Image #{item.imageID}: {item.error}
                  </div>
                ))}
            </Alert>
          )}
        </Modal.Body>
        <Modal.Footer>
          <Button
            variant="secondary"
            disabled={metadataBusy}
            onClick={() => setMetadataReview(undefined)}
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            disabled={metadataBusy || metadataPlan.targetImageIDs.length === 0}
            onClick={() => void confirmMetadataCopy()}
          >
            {metadataBusy
              ? "Copying…"
              : `Copy to ${metadataPlan.targetImageIDs.length} selected`}
          </Button>
        </Modal.Footer>
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
            <strong>Keep {selectedPlan.keep.length}</strong> image(s), and
            remove <strong>{selectedPlan.remove.length}</strong> selected
            image(s) from Stash. This action cannot be undone from the review
            page.
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
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => setReviewGroup(undefined)}
          >
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
