import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, Button, Form, Modal, Spinner } from "react-bootstrap";
import { faExternalLinkAlt, faSearch } from "@fortawesome/free-solid-svg-icons";
import { useHistory } from "react-router-dom";

import { Icon } from "src/components/Shared/Icon";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";

interface TargetCandidate {
  id: number;
  name: string;
  disambiguation?: string;
}

interface TagPrediction {
  name: string;
  category: string;
  score: number;
  rawName?: string;
  source?: string;
  targetPath?: string;
  targetExists?: boolean;
  targetCandidates?: TargetCandidate[];
}

type MetadataSourceKind = "local" | "booru" | "frames";

interface MetadataSource {
  kind: MetadataSourceKind;
  label: string;
  detail?: string;
  priority: number;
}

interface MetadataPrediction extends TagPrediction {
  provenance: MetadataSource[];
}

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
const IDENTITY_CATEGORIES = new Set(["character", "artist", "copyright"]);
const SOURCE_PRIORITY: Record<MetadataSourceKind, number> = {
  local: 0,
  booru: 10,
  frames: 20,
};
const CHARACTER_IDENTITY_PATTERN = /^(.+?)\s*\(([^()]*)\)\s*$/;
const POSSIBLE_CHARACTER_TARGET_PATTERN = /^\/performers\/\d+\/?$/;

function normalizePredictionValue(value?: string) {
  return (value ?? "")
    .trim()
    .replaceAll("_", " ")
    .replace(/\s+/g, " ")
    .toLocaleLowerCase();
}

function predictionIdentityKeys(prediction: TagPrediction) {
  const category = prediction.category.trim().toLocaleLowerCase();
  const values = [prediction.name, prediction.rawName]
    .map(normalizePredictionValue)
    .filter(Boolean);
  return [...new Set(values)].map((value) => `${category}\u0000${value}`);
}

function predictionKey(prediction: TagPrediction) {
  return (
    predictionIdentityKeys(prediction)[0] ??
    `${prediction.category}\u0000${prediction.name}`
  );
}

function filterBooruPredictionsForLocalPriority(
  localPredictions: TagPrediction[],
  booruPredictions: TagPrediction[]
) {
  const localIdentityCategories = new Set(
    localPredictions
      .map((prediction) => prediction.category.trim().toLocaleLowerCase())
      .filter((category) => IDENTITY_CATEGORIES.has(category))
  );
  const localIdentityKeys = new Set(
    localPredictions.flatMap(predictionIdentityKeys)
  );

  return booruPredictions.filter((prediction) => {
    const category = prediction.category.trim().toLocaleLowerCase();
    if (!IDENTITY_CATEGORIES.has(category)) return true;
    if (!localIdentityCategories.has(category)) return true;

    return predictionIdentityKeys(prediction).some((key) =>
      localIdentityKeys.has(key)
    );
  });
}

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

function sourceKey(source: MetadataSource) {
  return `${source.kind}\u0000${source.detail ?? ""}`;
}

function mergeSources(
  current: MetadataSource[],
  incoming: MetadataSource[]
): MetadataSource[] {
  const merged = new Map<string, MetadataSource>();
  for (const source of [...current, ...incoming]) {
    merged.set(sourceKey(source), source);
  }
  return [...merged.values()].sort((left, right) => {
    if (left.priority !== right.priority) return left.priority - right.priority;
    return left.label.localeCompare(right.label);
  });
}

function mergeMetadataPredictions(
  localPredictions: TagPrediction[],
  booruPredictions: TagPrediction[],
  framePredictions: TagPrediction[]
): MetadataPrediction[] {
  const merged: MetadataPrediction[] = [];
  const indexByKey = new Map<string, number>();

  const add = (prediction: TagPrediction, sourceKind: MetadataSourceKind) => {
    const keys = predictionIdentityKeys(prediction);
    let existingIndex: number | undefined;
    for (const key of keys) {
      const index = indexByKey.get(key);
      if (index !== undefined) {
        existingIndex = index;
        break;
      }
    }

    const provenance = [predictionSource(prediction, sourceKind)];
    if (existingIndex === undefined) {
      const next: MetadataPrediction = { ...prediction, provenance };
      const index = merged.length;
      merged.push(next);
      for (const key of keys) indexByKey.set(key, index);
      return;
    }

    const current = merged[existingIndex];
    const next: MetadataPrediction = {
      ...current,
      rawName: current.rawName || prediction.rawName,
      targetExists:
        current.targetExists === undefined
          ? prediction.targetExists
          : current.targetExists,
      targetPath: current.targetPath || prediction.targetPath,
      targetCandidates: current.targetCandidates?.length
        ? current.targetCandidates
        : prediction.targetCandidates,
      provenance: mergeSources(current.provenance, provenance),
    };
    merged[existingIndex] = next;
    for (const key of [
      ...predictionIdentityKeys(current),
      ...keys,
      ...predictionIdentityKeys(next),
    ]) {
      indexByKey.set(key, existingIndex);
    }
  };

  for (const prediction of localPredictions) add(prediction, "local");
  for (const prediction of booruPredictions) add(prediction, "booru");
  for (const prediction of framePredictions) add(prediction, "frames");
  return merged;
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

function sourceBadgeVariant(source: MetadataSourceKind) {
  if (source === "local") return "success";
  if (source === "frames") return "warning";
  return "info";
}

function possibleBareCharacterMatch(prediction: TagPrediction) {
  if (
    prediction.category !== "character" ||
    prediction.targetExists ||
    !POSSIBLE_CHARACTER_TARGET_PATTERN.test(prediction.targetPath ?? "")
  ) {
    return undefined;
  }

  const match = prediction.name.match(CHARACTER_IDENTITY_PATTERN);
  if (!match) return undefined;

  const bareName = match[1].trim();
  const disambiguation = match[2].trim();
  if (!bareName || !disambiguation) return undefined;

  return { bareName, disambiguation };
}

function ambiguousCharacterCandidates(prediction: TagPrediction) {
  if (prediction.category !== "character" || prediction.targetExists) {
    return [];
  }
  return prediction.targetCandidates ?? [];
}

function characterCandidateLabel(candidate: TargetCandidate) {
  const disambiguation = candidate.disambiguation?.trim();
  return disambiguation
    ? `${candidate.name} (${disambiguation})`
    : candidate.name;
}

function reusePossibleBareCharacter(prediction: TagPrediction): TagPrediction {
  const possible = possibleBareCharacterMatch(prediction);
  if (!possible) return prediction;

  return {
    ...prediction,
    name: possible.bareName,
    rawName: possible.bareName,
    targetExists: true,
    targetCandidates: undefined,
  };
}

function reuseCharacterCandidate(
  prediction: TagPrediction,
  candidate: TargetCandidate
): TagPrediction {
  const name = characterCandidateLabel(candidate);
  return {
    ...prediction,
    name,
    rawName: name,
    targetPath: `/performers/${candidate.id}`,
    targetExists: true,
    targetCandidates: undefined,
  };
}

export const VideoTaggingDialog: React.FC<IProps> = ({
  sceneId,
  onHide,
  onApplied,
}) => {
  const Toast = useToast();
  const history = useHistory();
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

  const predictions = useMemo(
    () =>
      mergeMetadataPredictions(
        localPredictions,
        booruPredictions,
        framePredictions
      ),
    [booruPredictions, framePredictions, localPredictions]
  );

  const addSelected = useCallback((items: TagPrediction[]) => {
    setSelected((current) => {
      const next = new Set(current);
      for (const item of items) next.add(predictionKey(item));
      return next;
    });
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(undefined);

    void (async () => {
      try {
        const response = await fetch(`scene/${sceneId}/local-metadata`);
        const result = await readResponse<TagSourceResponse>(response);
        if (!cancelled) {
          setLocalPredictions(result.tags);
          addSelected(result.tags);
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
    setLoadingFrames(true);
    setError(undefined);
    try {
      const response = await fetch(`scene/${sceneId}/frame-metadata`);
      const result = await readResponse<FrameMetadataResponse>(response);
      setFrameMetadata(result);
      setFramePredictions(result.tags);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoadingFrames(false);
    }
  }, [sceneId]);

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
    () =>
      predictions.filter((prediction) =>
        selected.has(predictionKey(prediction))
      ).length,
    [predictions, selected]
  );

  const toggle = useCallback((prediction: TagPrediction) => {
    const key = predictionKey(prediction);
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
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

  const continueCharacterResolution = useCallback(
    (tags: TagPrediction[], startIndex: number) => {
      const nextIndex = tags.findIndex(
        (prediction, index) =>
          index >= startIndex &&
          (possibleBareCharacterMatch(prediction) ||
            ambiguousCharacterCandidates(prediction).length > 1)
      );
      if (nextIndex === -1) {
        setPendingCharacterResolution(undefined);
        if (tags.length === 0) {
          setApplying(false);
          setError("No metadata items remain to apply.");
          return;
        }
        void submitTags(tags);
        return;
      }
      setPendingCharacterResolution({ tags, index: nextIndex });
    },
    [submitTags]
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

  return (
    <>
      <Modal
        show={!pendingCharacterResolution}
        onHide={onHide}
        size="lg"
        centered
      >
        <Modal.Header closeButton>
          <Modal.Title>Video Tagging</Modal.Title>
        </Modal.Header>
        <Modal.Body>
          <div className="d-flex flex-wrap align-items-center mb-3">
            <Button
              variant="secondary"
              disabled={loading || loadingBooru || loadingFrames || applying}
              onClick={() => void loadBooruMetadata()}
            >
              {loadingBooru ? "Loading booru…" : "Load booru metadata"}
            </Button>
            <Button
              className="ml-2"
              variant="secondary"
              disabled={loading || loadingBooru || loadingFrames || applying}
              onClick={() => void loadFrameMetadata()}
            >
              {loadingFrames ? "Analyzing frames…" : "Analyze frames"}
            </Button>
            <span className="ml-3 text-muted">
              Local filename metadata loads automatically. Frame inference runs
              only when requested.
            </span>
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

          {frameMetadata ? (
            <div className="alert alert-warning py-2">
              Analyzed {frameMetadata.sampleTimes.length} representative frame
              {frameMetadata.sampleTimes.length === 1 ? "" : "s"}. Frame
              suggestions start unselected and cannot override Local filename
              identity.
            </div>
          ) : null}

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
              No local metadata was found. You can still try an exact booru
              lookup or explicitly analyze representative frames.
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
            disabled={
              loading ||
              loadingBooru ||
              loadingFrames ||
              applying ||
              visibleSelectedCount === 0
            }
            onClick={applySelected}
          >
            {applying ? "Applying…" : "Apply selected metadata"}
          </Button>
        </Modal.Footer>
      </Modal>

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
              Video Tagging found <strong>{pendingPrediction.name}</strong>, but
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
            Video Tagging found <strong>{pendingPrediction?.name}</strong>, but
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
