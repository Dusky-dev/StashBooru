import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, Button, Form, Modal, Spinner } from "react-bootstrap";
import { faExternalLinkAlt, faSearch } from "@fortawesome/free-solid-svg-icons";
import { useHistory } from "react-router-dom";

import { Icon } from "src/components/Shared/Icon";
import { useToast } from "src/hooks/Toast";

interface TagPrediction {
  name: string;
  category: string;
  score: number;
  rawName?: string;
  source?: string;
  targetPath?: string;
  targetExists?: boolean;
}

type MetadataSourceKind = "local" | "booru";

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

interface IProps {
  sceneId: string;
  onHide: () => void;
  onApplied: () => Promise<unknown>;
}

const CATEGORY_ORDER = ["character", "artist", "copyright", "general", "meta"];
const SOURCE_PRIORITY: Record<MetadataSourceKind, number> = {
  local: 0,
  booru: 10,
};

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
  booruPredictions: TagPrediction[]
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
  return source === "local" ? "success" : "info";
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
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [replaceArtists, setReplaceArtists] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingBooru, setLoadingBooru] = useState(false);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState<string>();

  const predictions = useMemo(
    () => mergeMetadataPredictions(localPredictions, booruPredictions),
    [booruPredictions, localPredictions]
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
      setBooruMetadata(result);
      setBooruPredictions(result.tags);
      addSelected(result.tags);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoadingBooru(false);
    }
  }, [addSelected, sceneId]);

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

  const applySelected = useCallback(async () => {
    const tags = predictions
      .filter((prediction) => selected.has(predictionKey(prediction)))
      .map(({ provenance: _provenance, ...prediction }) => prediction);
    if (!tags.length) {
      setError("Select at least one metadata item to apply.");
      return;
    }

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
  }, [
    Toast,
    onApplied,
    onHide,
    predictions,
    replaceArtists,
    sceneId,
    selected,
  ]);

  return (
    <Modal show onHide={onHide} size="lg" centered>
      <Modal.Header closeButton>
        <Modal.Title>Video Tagging</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <div className="d-flex flex-wrap align-items-center mb-3">
          <Button
            variant="secondary"
            disabled={loading || loadingBooru || applying}
            onClick={() => void loadBooruMetadata()}
          >
            {loadingBooru ? "Loading booru…" : "Load booru metadata"}
          </Button>
          <span className="ml-3 text-muted">
            Local filename metadata loads automatically. Camie and EVA02 image
            inference are not run for Videos.
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
            lookup.
          </div>
        ) : (
          <>
            <div className="d-flex mb-3">
              <Button
                className="mr-2"
                size="sm"
                variant="outline-secondary"
                onClick={() =>
                  setSelected(new Set(predictions.map(predictionKey)))
                }
              >
                Select all
              </Button>
              <Button
                size="sm"
                variant="outline-secondary"
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
                          <Badge className="ml-2" variant="primary">
                            exists
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
          disabled={loading || applying || visibleSelectedCount === 0}
          onClick={() => void applySelected()}
        >
          {applying ? "Applying…" : "Apply selected metadata"}
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
