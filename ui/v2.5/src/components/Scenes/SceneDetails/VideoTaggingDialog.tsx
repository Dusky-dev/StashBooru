import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, Button, Form, Modal, Spinner } from "react-bootstrap";
import { faExternalLinkAlt, faSearch } from "@fortawesome/free-solid-svg-icons";
import { useHistory } from "react-router-dom";

import { Icon } from "src/components/Shared/Icon";
import { ModalComponent } from "src/components/Shared/Modal";
import {
  addPredictionSelection,
  ambiguousCharacterCandidates,
  characterCandidateLabel,
  filterBooruPredictionsForLocalPriority,
  mergeMetadataPredictions as mergeTaggingMetadataPredictions,
  nextCharacterResolutionIndex,
  possibleBareCharacterMatch,
  predictionKey,
  reuseCharacterCandidate,
  reusePossibleBareCharacter,
  selectedPredictionCount,
  sourceKey,
  togglePredictionSelection,
  type MetadataPrediction,
  type MetadataSource,
  type TagPrediction,
  type TargetCandidate,
} from "src/components/Tagging/taggingReviewPolicy";
import { useToast } from "src/hooks/Toast";
import {
  TaggingChangePlan,
  TaggingChangePlanModal,
} from "src/components/Tagging/TaggingChangePlanModal";

type MetadataSourceKind = "local" | "booru" | "frames";

interface TagSourceResponse {
  backend: "local" | "remote";
  model: string;
  threshold: number;
  limit: number;
  tags: TagPrediction[];
}

interface BooruMetadataResponse {
  source: string;
  postID: string;
  postURL?: string;
  md5: string;
  md5Source: "filename" | "file";
  tags: TagPrediction[];
}

interface FrameMetadataResponse {
  backend: "local" | "remote";
  model: string;
  threshold: number;
  limit: number;
  sampleTimes: number[];
  tags: TagPrediction[];
}

interface TaggingConfig {
  threshold: number;
  eva02Threshold: number;
  limit: number;
  filenameEnabled: boolean;
  filenameLayout: string;
}

interface AppliedEntity {
  id: number;
  name: string;
  category: string;
  score: number;
  created: boolean;
}

interface VideoTaggingApplyResponse {
  sceneID: number;
  characters: AppliedEntity[] | null;
  artists: AppliedEntity[] | null;
  copyrights: AppliedEntity[] | null;
  tags: AppliedEntity[] | null;
  createdCharacters: number;
  createdArtists: number;
  createdCopyrights: number;
  createdTags: number;
}

interface PendingCharacterResolution {
  tags: TagPrediction[];
  index: number;
}

interface IProps {
  sceneId: string;
  onHide: () => void;
  onApplied: () => Promise<unknown>;
}

const CATEGORY_ORDER = ["character", "artist", "copyright", "general", "meta"];
const SOURCE_PRIORITY: Record<MetadataSourceKind, number> = {
  local: 0,
  booru: 10,
  frames: 20,
};

function predictionSource(
  prediction: TagPrediction,
  kind: MetadataSourceKind
): MetadataSource {
  if (kind === "local") {
    return {
      kind,
      label: "Local",
      detail: "Local filename metadata",
      priority: SOURCE_PRIORITY.local,
    };
  }

  if (kind === "frames") {
    const consensus = prediction.source?.match(/^frame-analysis:(\d+)\/(\d+)$/);
    return {
      kind,
      label: "Frames",
      detail: consensus
        ? `Frame analysis: ${consensus[1]}/${consensus[2]} sampled frames`
        : "Frame analysis",
      priority: SOURCE_PRIORITY.frames,
    };
  }

  const provider = prediction.source?.startsWith("booru:")
    ? prediction.source.slice("booru:".length)
    : undefined;
  return {
    kind,
    label: provider || "Booru",
    detail: provider ? `Booru metadata: ${provider}` : "Booru metadata",
    priority: SOURCE_PRIORITY.booru,
  };
}

function mergeVideoMetadataPredictions(
  localPredictions: TagPrediction[],
  booruPredictions: TagPrediction[],
  framePredictions: TagPrediction[]
) {
  return mergeTaggingMetadataPredictions([
    {
      predictions: localPredictions,
      source: (prediction) => predictionSource(prediction, "local"),
    },
    {
      predictions: booruPredictions,
      source: (prediction) => predictionSource(prediction, "booru"),
    },
    {
      predictions: framePredictions,
      source: (prediction) => predictionSource(prediction, "frames"),
    },
  ]);
}

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return response.json() as Promise<T>;
}

function categoryLabel(category: string) {
  switch (category) {
    case "character":
      return "Characters";
    case "artist":
      return "Artists";
    case "copyright":
      return "Copyrights";
    case "general":
      return "General tags";
    case "meta":
      return "Meta tags";
    default:
      return `${category || "Other"} tags`;
  }
}

function sourceBadgeVariant(source: string) {
  if (source === "local") return "success";
  if (source === "frames") return "warning";
  return "info";
}

function parseFrameOptions(threshold: string, limit: string) {
  const parsedThreshold = Number.parseFloat(threshold);
  const parsedLimit = Number.parseInt(limit, 10);
  if (!(parsedThreshold > 0 && parsedThreshold < 1)) {
    throw new Error("Frame threshold must be greater than 0 and less than 1.");
  }
  if (!(parsedLimit >= 1 && parsedLimit <= 200)) {
    throw new Error("Per-category limit must be between 1 and 200.");
  }
  return { parsedThreshold, parsedLimit };
}

export const VideoTaggingDialog: React.FC<IProps> = ({
  sceneId,
  onHide,
  onApplied,
}) => {
  const Toast = useToast();
  const history = useHistory();
  const [frameThreshold, setFrameThreshold] = useState("0.492");
  const [limit, setLimit] = useState("50");
  const [localPredictions, setLocalPredictions] = useState<TagPrediction[]>([]);
  const [booruPredictions, setBooruPredictions] = useState<TagPrediction[]>([]);
  const [booruMetadata, setBooruMetadata] = useState<BooruMetadataResponse>();
  const [framePredictions, setFramePredictions] = useState<TagPrediction[]>([]);
  const [frameMetadata, setFrameMetadata] = useState<FrameMetadataResponse>();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [replaceArtists, setReplaceArtists] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingBooru, setLoadingBooru] = useState(false);
  const [loadingFrames, setLoadingFrames] = useState(false);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState<string>();
  const [pendingCharacterResolution, setPendingCharacterResolution] =
    useState<PendingCharacterResolution>();
  const [changePlan, setChangePlan] = useState<TaggingChangePlan>();
  const [changePlanTags, setChangePlanTags] = useState<TagPrediction[]>([]);

  const predictions = useMemo(
    () =>
      mergeVideoMetadataPredictions(
        localPredictions,
        booruPredictions,
        framePredictions
      ),
    [booruPredictions, framePredictions, localPredictions]
  );

  const addSelected = useCallback((items: TagPrediction[]) => {
    setSelected((current) => addPredictionSelection(current, items));
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(undefined);

    void (async () => {
      try {
        const [localResponse, configResponse] = await Promise.all([
          fetch(`scene/${sceneId}/local-metadata`),
          fetch("image/visual-similarity/camie/config"),
        ]);
        const result = await readResponse<TagSourceResponse>(localResponse);
        const config = await readResponse<TaggingConfig>(configResponse);
        if (!cancelled) {
          setLocalPredictions(result.tags);
          addSelected(result.tags);
          setFrameThreshold(String(config.threshold));
          setLimit(String(config.limit));
        }
      } catch (cause) {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : String(cause));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [addSelected, sceneId]);

  const loadBooruMetadata = useCallback(async () => {
    setLoadingBooru(true);
    setError(undefined);
    try {
      const response = await fetch(`scene/${sceneId}/booru-metadata`);
      const result = await readResponse<BooruMetadataResponse>(response);
      const allowedTags = filterBooruPredictionsForLocalPriority(
        localPredictions,
        result.tags
      );
      setBooruMetadata(result);
      setBooruPredictions(allowedTags);
      addSelected(allowedTags);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoadingBooru(false);
    }
  }, [addSelected, localPredictions, sceneId]);

  const loadFrameMetadata = useCallback(async () => {
    let options: ReturnType<typeof parseFrameOptions>;
    try {
      options = parseFrameOptions(frameThreshold, limit);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      return;
    }

    setLoadingFrames(true);
    setError(undefined);
    try {
      const response = await fetch(
        `scene/${sceneId}/frame-metadata?threshold=${encodeURIComponent(options.parsedThreshold)}&limit=${encodeURIComponent(options.parsedLimit)}`
      );
      const result = await readResponse<FrameMetadataResponse>(response);
      setFrameMetadata(result);
      setFramePredictions(result.tags);
      // Frame-assisted suggestions are review-only and start unselected.
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoadingFrames(false);
    }
  }, [frameThreshold, limit, sceneId]);

  const grouped = useMemo(() => {
    const groups = new Map<string, MetadataPrediction[]>();
    for (const prediction of predictions) {
      const items = groups.get(prediction.category) ?? [];
      items.push(prediction);
      groups.set(prediction.category, items);
    }
    return [...groups.entries()].sort(([left], [right]) => {
      const leftIndex = CATEGORY_ORDER.indexOf(left);
      const rightIndex = CATEGORY_ORDER.indexOf(right);
      if (leftIndex === -1 && rightIndex === -1)
        return left.localeCompare(right);
      if (leftIndex === -1) return 1;
      if (rightIndex === -1) return -1;
      return leftIndex - rightIndex;
    });
  }, [predictions]);

  const visibleSelectedCount = useMemo(
    () => selectedPredictionCount(predictions, selected),
    [predictions, selected]
  );

  const toggle = useCallback((prediction: TagPrediction) => {
    setSelected((current) => togglePredictionSelection(current, prediction));
  }, []);

  const submitTags = useCallback(
    async (tags: TagPrediction[]) => {
      setApplying(true);
      setError(undefined);
      try {
        const response = await fetch(`scene/${sceneId}/knowledge-tags`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ tags, replaceArtist: replaceArtists }),
        });
        const result = await readResponse<VideoTaggingApplyResponse>(response);
        const characters = result.characters ?? [];
        const artists = result.artists ?? [];
        const copyrights = result.copyrights ?? [];
        const appliedTags = result.tags ?? [];
        const appliedCount =
          characters.length +
          artists.length +
          copyrights.length +
          appliedTags.length;
        const createdCount =
          (result.createdCharacters ?? 0) +
          (result.createdArtists ?? 0) +
          (result.createdCopyrights ?? 0) +
          (result.createdTags ?? 0);
        Toast.success(
          `Applied ${appliedCount} metadata item${appliedCount === 1 ? "" : "s"}; created ${createdCount} new entr${createdCount === 1 ? "y" : "ies"}.`
        );
        await onApplied();
        onHide();
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : String(cause));
        Toast.error(cause);
      } finally {
        setApplying(false);
      }
    },
    [Toast, onApplied, onHide, replaceArtists, sceneId]
  );

  const previewTags = useCallback(
    async (tags: TagPrediction[]) => {
      setApplying(true);
      setError(undefined);
      try {
        const response = await fetch(
          `scene/${sceneId}/knowledge-tags?preview=1`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ tags, replaceArtist: replaceArtists }),
          }
        );
        const plan = await readResponse<TaggingChangePlan>(response);
        setChangePlan(plan);
        setChangePlanTags(tags);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : String(cause));
        Toast.error(cause);
      } finally {
        setApplying(false);
      }
    },
    [Toast, replaceArtists, sceneId]
  );

  const continueCharacterResolution = useCallback(
    (tags: TagPrediction[], startIndex: number) => {
      const nextIndex = nextCharacterResolutionIndex(tags, startIndex);
      if (nextIndex === -1) {
        setPendingCharacterResolution(undefined);
        if (tags.length === 0) {
          setApplying(false);
          setError("No metadata items remain to apply.");
          return;
        }
        void previewTags(tags);
        return;
      }
      setPendingCharacterResolution({ tags, index: nextIndex });
    },
    [previewTags]
  );

  const resolvePendingCharacter = useCallback(
    (reuseExisting: boolean) => {
      if (!pendingCharacterResolution) return;

      const { tags, index } = pendingCharacterResolution;
      const nextTags = [...tags];
      if (reuseExisting) {
        nextTags[index] = reusePossibleBareCharacter(nextTags[index]);
      }
      continueCharacterResolution(nextTags, index + 1);
    },
    [continueCharacterResolution, pendingCharacterResolution]
  );

  const choosePendingCharacterCandidate = useCallback(
    (candidate: TargetCandidate) => {
      if (!pendingCharacterResolution) return;
      const { tags, index } = pendingCharacterResolution;
      const nextTags = [...tags];
      nextTags[index] = reuseCharacterCandidate(nextTags[index], candidate);
      continueCharacterResolution(nextTags, index + 1);
    },
    [continueCharacterResolution, pendingCharacterResolution]
  );

  const skipPendingAmbiguousCharacter = useCallback(() => {
    if (!pendingCharacterResolution) return;
    const { tags, index } = pendingCharacterResolution;
    const nextTags = tags.filter((_, tagIndex) => tagIndex !== index);
    continueCharacterResolution(nextTags, index);
  }, [continueCharacterResolution, pendingCharacterResolution]);

  const cancelCharacterResolution = useCallback(() => {
    setPendingCharacterResolution(undefined);
    setApplying(false);
  }, []);

  const applySelected = useCallback(() => {
    const tags = predictions
      .filter((prediction) => selected.has(predictionKey(prediction)))
      .map(({ provenance: _provenance, ...prediction }) => prediction);
    if (!tags.length) {
      setError("Select at least one metadata item to apply.");
      return;
    }

    setApplying(true);
    setError(undefined);
    continueCharacterResolution(tags, 0);
  }, [continueCharacterResolution, predictions, selected]);

  const pendingPrediction = pendingCharacterResolution
    ? pendingCharacterResolution.tags[pendingCharacterResolution.index]
    : undefined;
  const pendingMatch = pendingPrediction
    ? possibleBareCharacterMatch(pendingPrediction)
    : undefined;
  const pendingCandidates = pendingPrediction
    ? ambiguousCharacterCandidates(pendingPrediction)
    : [];

  const busy = loading || loadingBooru || loadingFrames;

  return (
    <>
      <Modal
        show={!pendingCharacterResolution && !changePlan}
        onHide={onHide}
        size="lg"
        centered
      >
        <Modal.Header closeButton>
          <Modal.Title>Video metadata</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <div className="d-flex flex-wrap align-items-end mb-3">
            <Form.Group className="mr-3 mb-2">
              <Form.Label>Frame threshold</Form.Label>
              <Form.Control
                type="number"
                min="0.001"
                max="0.999"
                step="0.01"
                value={frameThreshold}
                onChange={(event) => setFrameThreshold(event.currentTarget.value)}
                style={{ width: "8rem" }}
              />
            </Form.Group>
            <Form.Group className="mr-3 mb-2">
              <Form.Label>Per-category limit</Form.Label>
              <Form.Control
                type="number"
                min="1"
                max="200"
                value={limit}
                onChange={(event) => setLimit(event.currentTarget.value)}
                style={{ width: "8rem" }}
              />
            </Form.Group>
            <Button
              className="mr-2 mb-2"
              variant="secondary"
              disabled={busy || applying}
              onClick={() => void loadBooruMetadata()}
            >
              {loadingBooru ? "Fetching…" : "Fetch from Danbooru"}
            </Button>
            <Button
              className="mb-2"
              variant="secondary"
              disabled={busy || applying}
              onClick={() => void loadFrameMetadata()}
            >
              {loadingFrames ? "Analyzing…" : "Analyze frames"}
            </Button>
          </div>

          <div className="d-flex flex-wrap align-items-center mb-3">
            <Badge className="mr-2 mb-1" variant="success">
              Local loaded
            </Badge>
            {booruMetadata ? (
              <Badge className="mr-2 mb-1" variant="info">
                {booruMetadata.source}
              </Badge>
            ) : null}
            {frameMetadata ? (
              <Badge className="mb-1" variant="warning">
                Frames {frameMetadata.sampleTimes.length} samples · {frameMetadata.backend}
              </Badge>
            ) : null}
          </div>

          {booruMetadata ? (
            <div className="alert alert-info py-2">
              <div className="d-flex flex-wrap align-items-center">
                <strong>{booruMetadata.source}</strong>
                <Badge className="ml-2" variant="secondary">
                  MD5 from {booruMetadata.md5Source}
                </Badge>
                <code className="ml-2">{booruMetadata.md5}</code>
                {booruMetadata.postURL ? (
                  <a
                    className="ml-auto"
                    href={booruMetadata.postURL}
                    target="_blank"
                    rel="noreferrer"
                  >
                    Post {booruMetadata.postID || "match"}{" "}
                    <Icon icon={faExternalLinkAlt} />
                  </a>
                ) : null}
              </div>
            </div>
          ) : null}

          <div className="mb-3 text-muted">
            Sources are merged in priority order: Local → Danbooru → Frames.
            Opening this dialog loads Local metadata only. Danbooru lookup and
            representative-frame analysis are explicit. Frame analysis uses the
            configured Camie/frame threshold and per-category limit, samples the
            Video through ffmpeg, and starts its suggestions unselected for
            review. Frame predictions cannot override authoritative Local
            Character, Artist, or Copyright identity.
          </div>

          <Form.Check
            className="mb-3"
            type="checkbox"
            id="video-tagging-replace-artists"
            checked={replaceArtists}
            onChange={(event) => setReplaceArtists(event.currentTarget.checked)}
            label="Replace existing Artists instead of adding selected Artists"
          />

          {error ? <div className="alert alert-danger">{error}</div> : null}

          {loading ? (
            <div className="text-center py-5">
              <Spinner animation="border" role="status" />
            </div>
          ) : predictions.length === 0 && !error ? (
            <div className="text-muted">
              No Local metadata was found. Fetch Danbooru or explicitly analyze
              representative frames to add predictions.
            </div>
          ) : (
            <>
              <div className="d-flex mb-3">
                <Button
                  className="mr-2"
                  size="sm"
                  variant="outline-secondary"
                  disabled={visibleSelectedCount === predictions.length}
                  onClick={() =>
                    setSelected(new Set(predictions.map(predictionKey)))
                  }
                >
                  Select all
                </Button>
                <Button
                  size="sm"
                  variant="outline-secondary"
                  disabled={visibleSelectedCount === 0}
                  onClick={() => setSelected(new Set())}
                >
                  Unselect all
                </Button>
                <span className="ml-auto text-muted align-self-center">
                  {visibleSelectedCount} / {predictions.length} selected
                </span>
              </div>

              <div style={{ maxHeight: "55vh", overflowY: "auto" }}>
                {grouped.map(([category, items]) => (
                  <div className="mb-4" key={category}>
                    <h5>{categoryLabel(category)}</h5>
                    {items.map((prediction, index) => {
                      const key = predictionKey(prediction);
                      const possibleMatch =
                        possibleBareCharacterMatch(prediction);
                      const candidates =
                        ambiguousCharacterCandidates(prediction);
                      return (
                        <div
                          className="d-flex align-items-center py-1 border-bottom"
                          key={key}
                        >
                          <Form.Check
                            type="checkbox"
                            id={`video-tagging-${category}-${index}`}
                            checked={selected.has(key)}
                            onChange={() => toggle(prediction)}
                            label={prediction.name}
                          />
                          {prediction.provenance.map((source) => (
                            <Badge
                              className="ml-2"
                              key={sourceKey(source)}
                              variant={sourceBadgeVariant(source.kind)}
                              title={source.detail}
                            >
                              {source.label}
                            </Badge>
                          ))}
                          {prediction.targetExists ? (
                            <Badge
                              className="ml-2"
                              variant="primary"
                              title="This metadata entity already exists in Stash"
                            >
                              exists
                            </Badge>
                          ) : null}
                          {possibleMatch ? (
                            <Badge
                              className="ml-2"
                              variant="secondary"
                              title={`Possible existing Character: ${possibleMatch.bareName}`}
                            >
                              possible match
                            </Badge>
                          ) : null}
                          {candidates.length > 1 ? (
                            <Badge
                              className="ml-2"
                              variant="warning"
                              title={`${candidates.length} existing Characters share this name`}
                            >
                              choose Character
                            </Badge>
                          ) : null}
                          {prediction.rawName &&
                          prediction.rawName !== prediction.name ? (
                            <span className="ml-2 small text-muted">
                              alias: {prediction.rawName}
                            </span>
                          ) : null}
                          {prediction.targetPath ? (
                            <Button
                              className="ml-auto mr-2 py-0 px-2"
                              size="sm"
                              variant="outline-secondary"
                              title={
                                prediction.targetExists
                                  ? "Open metadata page"
                                  : possibleMatch
                                    ? `Open possible existing Character “${possibleMatch.bareName}”`
                                    : "Search for this metadata"
                              }
                              onClick={() => {
                                onHide();
                                history.push(prediction.targetPath!);
                              }}
                            >
                              <Icon icon={faSearch} />
                            </Button>
                          ) : null}
                          <Badge
                            className={prediction.targetPath ? "" : "ml-auto"}
                            variant="secondary"
                          >
                            {(prediction.score * 100).toFixed(1)}%
                          </Badge>
                        </div>
                      );
                    })}
                  </div>
                ))}
              </div>
            </>
          )}
        </Modal.Body>
        <Modal.Footer>
          <Button variant="secondary" disabled={applying} onClick={onHide}>
            Cancel
          </Button>
          <Button
            variant="primary"
            disabled={busy || applying || visibleSelectedCount === 0}
            onClick={applySelected}
          >
            {applying ? "Applying…" : "Apply selected metadata"}
          </Button>
        </Modal.Footer>
      </Modal>

      <TaggingChangePlanModal
        show={!!changePlan}
        title="Review Video Metadata changes"
        plan={changePlan}
        busy={applying}
        onBack={() => {
          setChangePlan(undefined);
          setChangePlanTags([]);
        }}
        onHide={onHide}
        onApply={() => {
          const tags = changePlanTags;
          setChangePlan(undefined);
          setChangePlanTags([]);
          void submitTags(tags);
        }}
      />

      <ModalComponent
        show={
          !!pendingCharacterResolution &&
          !!pendingPrediction &&
          !!pendingMatch &&
          pendingCandidates.length === 0
        }
        header="Resolve Character"
        modalProps={{ centered: true }}
        cancel={{
          text: "Keep separate",
          variant: "secondary",
          onClick: () => resolvePendingCharacter(false),
        }}
        accept={{
          text: "Reuse existing Character",
          onClick: () => resolvePendingCharacter(true),
        }}
      >
        {pendingPrediction && pendingMatch ? (
          <>
            <p>
              Video Metadata found <strong>{pendingPrediction.name}</strong>, but
              an existing Character named{" "}
              <strong>{pendingMatch.bareName}</strong> has no disambiguation.
            </p>
            <p>Is this the same Character?</p>
            <p className="text-muted mb-0">
              Reuse existing keeps the current Character entry. Keep separate
              preserves <strong>{pendingPrediction.name}</strong> as its own
              disambiguated Character.
            </p>
          </>
        ) : null}
      </ModalComponent>

      <Modal
        show={
          !!pendingCharacterResolution &&
          !!pendingPrediction &&
          pendingCandidates.length > 1
        }
        onHide={cancelCharacterResolution}
        centered
      >
        <Modal.Header closeButton>
          <Modal.Title>Resolve Character</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <p>
            Video Metadata found <strong>{pendingPrediction?.name}</strong>, but
            multiple existing Characters share that name. Choose the Character
            this metadata refers to.
          </p>
          <div className="d-flex flex-column">
            {pendingCandidates.map((candidate) => (
              <Button
                className="mb-2 text-left"
                key={candidate.id}
                variant="outline-primary"
                onClick={() => choosePendingCharacterCandidate(candidate)}
              >
                {characterCandidateLabel(candidate)}
                <span className="ml-2 text-muted">#{candidate.id}</span>
              </Button>
            ))}
          </div>
          <p className="text-muted mb-0">
            The selected existing Character is reused explicitly; StashBooru no
            longer guesses between same-name Characters.
          </p>
        </Modal.Body>
        <Modal.Footer>
          <Button variant="secondary" onClick={cancelCharacterResolution}>
            Back
          </Button>
          <Button
            variant="outline-secondary"
            onClick={skipPendingAmbiguousCharacter}
          >
            Skip this Character
          </Button>
        </Modal.Footer>
      </Modal>
    </>
  );
};
