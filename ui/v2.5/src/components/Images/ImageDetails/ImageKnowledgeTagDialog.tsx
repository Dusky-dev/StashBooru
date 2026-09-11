import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, Button, Form, Modal, Spinner } from "react-bootstrap";
import { faExternalLinkAlt, faSearch } from "@fortawesome/free-solid-svg-icons";
import { useHistory } from "react-router-dom";

import { Icon } from "src/components/Shared/Icon";
import { useToast } from "src/hooks/Toast";

interface CamiePrediction {
  name: string;
  category: string;
  score: number;
  rawName?: string;
  source?: string;
  targetPath?: string;
  targetExists?: boolean;
}

type MetadataOrigin = "camie" | "booru";
type MetadataSourceKind = "file" | "booru" | "camie";

interface MetadataSource {
  kind: MetadataSourceKind;
  label: string;
  detail?: string;
  priority: number;
}

interface MetadataPrediction extends CamiePrediction {
  provenance: MetadataSource[];
}

interface CamieTagsResponse {
  backend: "local" | "remote";
  model: string;
  threshold: number;
  limit: number;
  tags: CamiePrediction[];
}

interface BooruMetadataResponse {
  source: string;
  postID: string;
  postURL?: string;
  md5: string;
  md5Source: "filename" | "file";
  tags: CamiePrediction[];
}

interface CamieConfig {
  threshold: number;
  limit: number;
  filenameEnabled: boolean;
  filenameLayout: string;
}

interface CamieAppliedEntity {
  id: number;
  name: string;
  category: string;
  score: number;
  created: boolean;
}

interface CamieApplyResponse {
  imageID: number;
  characters: CamieAppliedEntity[];
  artist?: CamieAppliedEntity;
  copyrights: CamieAppliedEntity[];
  tags: CamieAppliedEntity[];
  skippedArtists?: CamiePrediction[];
  preservedArtist: boolean;
  createdCharacters: number;
  createdArtists: number;
  createdCopyrights: number;
  createdTags: number;
}

interface IProps {
  imageId: string;
  onHide: () => void;
  onApplied: () => Promise<unknown>;
}

const CATEGORY_ORDER = ["character", "artist", "copyright", "general", "meta"];
const LOCAL_AUTHORITY_CATEGORIES = new Set([
  "character",
  "artist",
  "copyright",
]);
const SOURCE_PRIORITY: Record<MetadataSourceKind, number> = {
  file: 0,
  booru: 10,
  camie: 20,
};

function normalizePredictionValue(value?: string) {
  return (value ?? "")
    .trim()
    .replaceAll("_", " ")
    .replace(/\s+/g, " ")
    .toLocaleLowerCase();
}

function predictionIdentityKeys(prediction: CamiePrediction) {
  const category = prediction.category.trim().toLocaleLowerCase();
  const values = [prediction.name, prediction.rawName]
    .map(normalizePredictionValue)
    .filter(Boolean);
  return [...new Set(values)].map((value) => `${category}\u0000${value}`);
}

function predictionKey(prediction: CamiePrediction) {
  return (
    predictionIdentityKeys(prediction)[0] ??
    `${prediction.category}\u0000${prediction.name}`
  );
}

function sourceKey(source: MetadataSource) {
  return `${source.kind}\u0000${source.detail ?? ""}`;
}

function hasFilenameSource(prediction: CamiePrediction) {
  return prediction.source?.includes("filename") ?? false;
}

function hasCamieModelSource(prediction: CamiePrediction) {
  const source = prediction.source ?? "model";
  return source.includes("model") || !source.includes("filename");
}

function applyLocalAuthority(predictions: CamiePrediction[]) {
  const authoritativeCategories = new Set<string>();
  for (const prediction of predictions) {
    const category = prediction.category.trim().toLocaleLowerCase();
    if (
      LOCAL_AUTHORITY_CATEGORIES.has(category) &&
      hasFilenameSource(prediction)
    ) {
      authoritativeCategories.add(category);
    }
  }

  return predictions.filter((prediction) => {
    const category = prediction.category.trim().toLocaleLowerCase();
    if (!authoritativeCategories.has(category)) return true;

    // Filename/local metadata owns Character, Artist and Copyright as a whole
    // category. Camie may confirm a local value (model+filename), but a
    // different model-only value is not mixed into that category.
    return hasFilenameSource(prediction);
  });
}

function predictionSources(
  prediction: CamiePrediction,
  origin: MetadataOrigin
): MetadataSource[] {
  if (origin === "booru") {
    const provider = prediction.source?.startsWith("booru:")
      ? prediction.source.slice("booru:".length)
      : undefined;
    return [
      {
        kind: "booru",
        label: "booru",
        detail: provider ? `Booru: ${provider}` : "Booru metadata",
        priority: SOURCE_PRIORITY.booru,
      },
    ];
  }

  const result: MetadataSource[] = [];
  if (hasFilenameSource(prediction)) {
    result.push({
      kind: "file",
      label: "local",
      detail: "Local filename metadata",
      priority: SOURCE_PRIORITY.file,
    });
  }
  if (hasCamieModelSource(prediction)) {
    result.push({
      kind: "camie",
      label: "Camie",
      detail: "Camie model prediction",
      priority: SOURCE_PRIORITY.camie,
    });
  }
  return result;
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
  camiePredictions: CamiePrediction[],
  booruPredictions: CamiePrediction[]
): MetadataPrediction[] {
  const merged: MetadataPrediction[] = [];
  const indexByKey = new Map<string, number>();
  const authoritativeCamie = applyLocalAuthority(camiePredictions);

  const add = (prediction: CamiePrediction, origin: MetadataOrigin) => {
    const keys = predictionIdentityKeys(prediction);
    let existingIndex: number | undefined;
    for (const key of keys) {
      const index = indexByKey.get(key);
      if (index !== undefined) {
        existingIndex = index;
        break;
      }
    }

    const provenance = predictionSources(prediction, origin);
    if (existingIndex === undefined) {
      const next: MetadataPrediction = {
        ...prediction,
        provenance: mergeSources([], provenance),
      };
      const index = merged.length;
      merged.push(next);
      for (const key of keys) indexByKey.set(key, index);
      return;
    }

    const current = merged[existingIndex];
    const next: MetadataPrediction = {
      ...current,
      score: Math.max(current.score, prediction.score),
      rawName: current.rawName || prediction.rawName,
      targetExists: current.targetExists || prediction.targetExists,
      targetPath:
        current.targetExists && current.targetPath
          ? current.targetPath
          : prediction.targetPath || current.targetPath,
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

  // Local filename metadata is the preferred representation. Booru is still
  // shown for provenance/overlap, followed by Camie-only categories where no
  // authoritative local Character/Artist/Copyright value exists.
  for (const prediction of authoritativeCamie) {
    if (hasFilenameSource(prediction)) add(prediction, "camie");
  }
  for (const prediction of booruPredictions) add(prediction, "booru");
  for (const prediction of authoritativeCamie) {
    if (!hasFilenameSource(prediction)) add(prediction, "camie");
  }
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
      return "Artist";
    case "copyright":
      return "Copyright";
    case "general":
      return "General tags";
    case "meta":
      return "Meta tags";
    default:
      return `${category || "Other"} tags`;
  }
}

function sourceBadgeVariant(source: MetadataSourceKind) {
  switch (source) {
    case "file":
      return "success";
    case "booru":
      return "info";
    case "camie":
      return "warning";
  }
}

export const ImageKnowledgeTagDialog: React.FC<IProps> = ({
  imageId,
  onHide,
  onApplied,
}) => {
  const Toast = useToast();
  const history = useHistory();
  const [threshold, setThreshold] = useState("0.492");
  const [limit, setLimit] = useState("50");
  const [camiePredictions, setCamiePredictions] = useState<CamiePrediction[]>(
    []
  );
  const [booruPredictions, setBooruPredictions] = useState<CamiePrediction[]>(
    []
  );
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [backend, setBackend] = useState<string>();
  const [booruMetadata, setBooruMetadata] = useState<BooruMetadataResponse>();
  const [loading, setLoading] = useState(true);
  const [applying, setApplying] = useState(false);
  const [replaceArtist, setReplaceArtist] = useState(false);
  const [error, setError] = useState<string>();

  const predictions = useMemo(
    () => mergeMetadataPredictions(camiePredictions, booruPredictions),
    [booruPredictions, camiePredictions]
  );

  const addSelected = useCallback((items: CamiePrediction[]) => {
    setSelected((current) => {
      const next = new Set(current);
      for (const item of items) next.add(predictionKey(item));
      return next;
    });
  }, []);

  const loadPredictions = useCallback(
    async (nextThreshold: string, nextLimit: string) => {
      const parsedThreshold = Number.parseFloat(nextThreshold);
      const parsedLimit = Number.parseInt(nextLimit, 10);
      if (!(parsedThreshold > 0 && parsedThreshold < 1)) {
        setError("Threshold must be greater than 0 and less than 1.");
        return;
      }
      if (!(parsedLimit >= 1 && parsedLimit <= 200)) {
        setError("Per-category limit must be between 1 and 200.");
        return;
      }

      setLoading(true);
      setError(undefined);
      try {
        const response = await fetch(
          `image/${imageId}/knowledge-tags?threshold=${encodeURIComponent(parsedThreshold)}&limit=${encodeURIComponent(parsedLimit)}`,
          { method: "POST" }
        );
        const result = await readResponse<CamieTagsResponse>(response);
        setCamiePredictions(result.tags);
        setBackend(result.backend);
        addSelected(result.tags);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : String(cause));
      } finally {
        setLoading(false);
      }
    },
    [addSelected, imageId]
  );

  const loadBooruMetadata = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const response = await fetch(`image/${imageId}/booru-metadata`);
      const result = await readResponse<BooruMetadataResponse>(response);
      setBooruMetadata(result);
      setBooruPredictions(result.tags);
      addSelected(result.tags);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }, [addSelected, imageId]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const response = await fetch("image/visual-similarity/camie/config");
        const config = await readResponse<CamieConfig>(response);
        if (cancelled) return;
        const savedThreshold = String(config.threshold);
        const savedLimit = String(config.limit);
        setThreshold(savedThreshold);
        setLimit(savedLimit);
        await loadPredictions(savedThreshold, savedLimit);
      } catch (cause) {
        if (!cancelled) {
          setLoading(false);
          setError(cause instanceof Error ? cause.message : String(cause));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [loadPredictions]);

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
      if (leftIndex === -1 && rightIndex === -1) {
        return left.localeCompare(right);
      }
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

  const togglePrediction = useCallback((prediction: CamiePrediction) => {
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
      const response = await fetch(
        `image/${imageId}/knowledge-tags?apply=true`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ tags, replaceArtist }),
        }
      );
      const result = await readResponse<CamieApplyResponse>(response);
      const appliedCount =
        result.characters.length +
        result.copyrights.length +
        result.tags.length +
        (result.artist ? 1 : 0);
      const createdCount =
        result.createdCharacters +
        result.createdArtists +
        result.createdCopyrights +
        result.createdTags;
      const preserved = result.preservedArtist
        ? " Existing Artist was preserved."
        : "";
      Toast.success(
        `Applied ${appliedCount} metadata item${appliedCount === 1 ? "" : "s"}; created ${createdCount} new entr${createdCount === 1 ? "y" : "ies"}.${preserved}`
      );
      await onApplied();
      onHide();
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setError(message);
      Toast.error(cause);
    } finally {
      setApplying(false);
    }
  }, [Toast, imageId, onApplied, onHide, predictions, replaceArtist, selected]);

  return (
    <Modal show onHide={onHide} size="lg" centered>
      <Modal.Header closeButton>
        <Modal.Title>Image metadata</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <div className="d-flex flex-wrap align-items-end mb-3">
          <Form.Group className="mr-3 mb-2">
            <Form.Label>Camie threshold</Form.Label>
            <Form.Control
              type="number"
              min="0.001"
              max="0.999"
              step="0.01"
              value={threshold}
              onChange={(event) => setThreshold(event.currentTarget.value)}
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
            disabled={loading || applying}
            onClick={() => void loadPredictions(threshold, limit)}
          >
            Analyze with Camie
          </Button>
          <Button
            className="mb-2"
            variant="secondary"
            disabled={loading || applying}
            onClick={() => void loadBooruMetadata()}
          >
            Fetch from booru
          </Button>
          {backend ? (
            <Badge className="ml-2 mb-2" variant="secondary">
              {backend === "remote"
                ? "Camie remote worker"
                : "Camie local worker"}
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
          Matching metadata from different methods is merged into one row.
          Local means metadata parsed from this file's filename; an existing
          Stash entry is not treated as a source. For Characters, Artist and
          Copyright, local values are authoritative: Camie can confirm those
          exact values, but conflicting Camie-only values in that category are
          ignored. If the filename has no value for a category, Camie can supply
          it. Booru overlap remains visible separately.
        </div>

        <Form.Check
          className="mb-3"
          type="checkbox"
          id="camie-replace-artist"
          checked={replaceArtist}
          onChange={(event) => setReplaceArtist(event.currentTarget.checked)}
          label="Replace the existing Artist if this image already has one"
        />

        {error ? <div className="alert alert-danger">{error}</div> : null}

        {loading ? (
          <div className="text-center py-5">
            <Spinner animation="border" role="status" />
          </div>
        ) : predictions.length === 0 && !error ? (
          <div className="text-muted">No metadata was returned.</div>
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
                Clear selection
              </Button>
              <span className="ml-auto text-muted align-self-center">
                {visibleSelectedCount} / {predictions.length} selected
              </span>
            </div>

            <div style={{ maxHeight: "55vh", overflowY: "auto" }}>
              {grouped.map(([category, items]) => {
                const localAuthority =
                  LOCAL_AUTHORITY_CATEGORIES.has(category) &&
                  items.some((prediction) =>
                    prediction.provenance.some(
                      (source) => source.kind === "file"
                    )
                  );
                return (
                  <div className="mb-4" key={category}>
                    <h5>{categoryLabel(category)}</h5>
                    {localAuthority ? (
                      <div className="small text-muted mb-2">
                        Local filename metadata is authoritative for this
                        category; conflicting Camie predictions are ignored.
                      </div>
                    ) : null}
                    {category === "artist" && items.length > 1 ? (
                      <div className="small text-muted mb-2">
                        Images support one Artist. If several non-local sources
                        disagree, the selected metadata is reviewed here before
                        applying.
                      </div>
                    ) : null}
                    {items.map((prediction, index) => {
                      const key = predictionKey(prediction);
                      return (
                        <div
                          className="d-flex align-items-center py-1 border-bottom"
                          key={key}
                        >
                          <Form.Check
                            type="checkbox"
                            id={`metadata-${category}-${index}`}
                            checked={selected.has(key)}
                            onChange={() => togglePrediction(prediction)}
                            label={prediction.name}
                          />
                          {prediction.provenance.map((source) => (
                            <Badge
                              className="ml-2"
                              key={sourceKey(source)}
                              title={source.detail}
                              variant={sourceBadgeVariant(source.kind)}
                            >
                              {source.label}
                            </Badge>
                          ))}
                          {prediction.targetExists ? (
                            <Badge
                              className="ml-2"
                              variant="secondary"
                              title="This metadata entity already exists in Stash"
                            >
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
                );
              })}
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
