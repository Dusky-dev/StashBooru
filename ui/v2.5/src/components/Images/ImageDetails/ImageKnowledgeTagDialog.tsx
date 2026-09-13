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

type MetadataSourceKind = "local" | "booru" | "camie" | "eva02";

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

interface CamieConfig {
  threshold: number;
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

interface ImageTaggingApplyResponse {
  imageID: number;
  characters: AppliedEntity[];
  artists: AppliedEntity[];
  copyrights: AppliedEntity[];
  tags: AppliedEntity[];
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
const SOURCE_PRIORITY: Record<MetadataSourceKind, number> = {
  local: 0,
  booru: 10,
  camie: 20,
  eva02: 30,
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

function sourceKey(source: MetadataSource) {
  return `${source.kind}\u0000${source.detail ?? ""}`;
}

function predictionSource(
  prediction: TagPrediction,
  kind: MetadataSourceKind
): MetadataSource {
  switch (kind) {
    case "local":
      return {
        kind,
        label: "Local",
        detail: "Local filename metadata",
        priority: SOURCE_PRIORITY.local,
      };
    case "booru": {
      const provider = prediction.source?.startsWith("booru:")
        ? prediction.source.slice("booru:".length)
        : undefined;
      return {
        kind,
        label: provider || "Danbooru",
        detail: provider ? `Booru metadata: ${provider}` : "Danbooru metadata",
        priority: SOURCE_PRIORITY.booru,
      };
    }
    case "camie":
      return {
        kind,
        label: "Camie",
        detail: "Camie model prediction",
        priority: SOURCE_PRIORITY.camie,
      };
    case "eva02":
      return {
        kind,
        label: "EVA02",
        detail: "WD EVA02 fallback prediction",
        priority: SOURCE_PRIORITY.eva02,
      };
  }
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
  camiePredictions: TagPrediction[],
  eva02Predictions: TagPrediction[]
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
      const next: MetadataPrediction = {
        ...prediction,
        provenance,
      };
      const index = merged.length;
      merged.push(next);
      for (const key of keys) indexByKey.set(key, index);
      return;
    }

    // Sources are added in priority order. Preserve the earlier source's
    // prediction and score; later sources only contribute provenance and fill
    // optional target information that was missing from the winning source.
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
  for (const prediction of camiePredictions) add(prediction, "camie");
  for (const prediction of eva02Predictions) add(prediction, "eva02");
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
  switch (source) {
    case "local":
      return "success";
    case "booru":
      return "info";
    case "camie":
      return "warning";
    case "eva02":
      return "secondary";
  }
}

function parseInferenceOptions(threshold: string, limit: string) {
  const parsedThreshold = Number.parseFloat(threshold);
  const parsedLimit = Number.parseInt(limit, 10);
  if (!(parsedThreshold > 0 && parsedThreshold < 1)) {
    throw new Error("Threshold must be greater than 0 and less than 1.");
  }
  if (!(parsedLimit >= 1 && parsedLimit <= 200)) {
    throw new Error("Per-category limit must be between 1 and 200.");
  }
  return { parsedThreshold, parsedLimit };
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
  const [localPredictions, setLocalPredictions] = useState<TagPrediction[]>([]);
  const [booruPredictions, setBooruPredictions] = useState<TagPrediction[]>([]);
  const [camiePredictions, setCamiePredictions] = useState<TagPrediction[]>([]);
  const [eva02Predictions, setEva02Predictions] = useState<TagPrediction[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [camieBackend, setCamieBackend] = useState<string>();
  const [eva02Backend, setEva02Backend] = useState<string>();
  const [booruMetadata, setBooruMetadata] = useState<BooruMetadataResponse>();
  const [loading, setLoading] = useState(true);
  const [loadingSource, setLoadingSource] = useState<MetadataSourceKind>();
  const [applying, setApplying] = useState(false);
  const [replaceArtist, setReplaceArtist] = useState(false);
  const [error, setError] = useState<string>();

  const predictions = useMemo(
    () =>
      mergeMetadataPredictions(
        localPredictions,
        booruPredictions,
        camiePredictions,
        eva02Predictions
      ),
    [booruPredictions, camiePredictions, eva02Predictions, localPredictions]
  );

  const addSelected = useCallback((items: TagPrediction[]) => {
    setSelected((current) => {
      const next = new Set(current);
      for (const item of items) next.add(predictionKey(item));
      return next;
    });
  }, []);

  const loadBooruMetadata = useCallback(async () => {
    setLoadingSource("booru");
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
      setLoadingSource(undefined);
    }
  }, [addSelected, imageId]);

  const loadModelPredictions = useCallback(
    async (source: "camie" | "eva02") => {
      let options: ReturnType<typeof parseInferenceOptions>;
      try {
        options = parseInferenceOptions(threshold, limit);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : String(cause));
        return;
      }

      setLoadingSource(source);
      setError(undefined);
      try {
        const endpoint = source === "camie" ? "camie-tags" : "eva02-tags";
        const response = await fetch(
          `image/${imageId}/${endpoint}?threshold=${encodeURIComponent(options.parsedThreshold)}&limit=${encodeURIComponent(options.parsedLimit)}`,
          { method: "POST" }
        );
        const result = await readResponse<TagSourceResponse>(response);
        if (source === "camie") {
          setCamiePredictions(result.tags);
          setCamieBackend(result.backend);
        } else {
          setEva02Predictions(result.tags);
          setEva02Backend(result.backend);
        }
        addSelected(result.tags);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : String(cause));
      } finally {
        setLoadingSource(undefined);
      }
    },
    [addSelected, imageId, limit, threshold]
  );

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(undefined);

    void (async () => {
      try {
        const response = await fetch(`image/${imageId}/local-metadata`);
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

      // Camie's saved inference defaults are only used to initialize the
      // controls. Reading them is not inference and does not start Camie.
      try {
        const response = await fetch("image/visual-similarity/camie/config");
        const config = await readResponse<CamieConfig>(response);
        if (!cancelled) {
          setThreshold(String(config.threshold));
          setLimit(String(config.limit));
        }
      } catch {
        // Keep the UI defaults if the optional Camie config is unavailable.
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [addSelected, imageId]);

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
  const allVisibleSelected =
    predictions.length > 0 && visibleSelectedCount === predictions.length;
  const busy = loading || loadingSource !== undefined;

  const togglePrediction = useCallback((prediction: TagPrediction) => {
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
      const result = await readResponse<ImageTaggingApplyResponse>(response);
      const appliedCount =
        result.characters.length +
        result.artists.length +
        result.copyrights.length +
        result.tags.length;
      const createdCount =
        result.createdCharacters +
        result.createdArtists +
        result.createdCopyrights +
        result.createdTags;
      Toast.success(
        `Applied ${appliedCount} metadata item${appliedCount === 1 ? "" : "s"}; created ${createdCount} new entr${createdCount === 1 ? "y" : "ies"}.`
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
        <Modal.Title>Image tagging</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <div className="d-flex flex-wrap align-items-end mb-3">
          <Form.Group className="mr-3 mb-2">
            <Form.Label>Inference threshold</Form.Label>
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
            disabled={busy || applying}
            onClick={() => void loadBooruMetadata()}
          >
            {loadingSource === "booru" ? "Fetching…" : "Fetch from Danbooru"}
          </Button>
          <Button
            className="mr-2 mb-2"
            variant="secondary"
            disabled={busy || applying}
            onClick={() => void loadModelPredictions("camie")}
          >
            {loadingSource === "camie" ? "Analyzing…" : "Analyze with Camie"}
          </Button>
          <Button
            className="mb-2"
            variant="secondary"
            disabled={busy || applying}
            onClick={() => void loadModelPredictions("eva02")}
          >
            {loadingSource === "eva02" ? "Analyzing…" : "Analyze with EVA02"}
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
          {camieBackend ? (
            <Badge className="mr-2 mb-1" variant="warning">
              Camie {camieBackend}
            </Badge>
          ) : null}
          {eva02Backend ? (
            <Badge className="mb-1" variant="secondary">
              EVA02 {eva02Backend}
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
          Sources are merged in priority order: Local → Danbooru → Camie →
          EVA02. Opening this dialog loads Local metadata only; every network or
          model source is explicit. When the same metadata item is predicted by
          multiple sources, the earlier source keeps its value and score while
          later sources are shown as additional provenance. EVA02 is the
          fallback source. Multiple selected Artists can be attached to the same
          image.
        </div>

        <Form.Check
          className="mb-3"
          type="checkbox"
          id="image-tagging-replace-artists"
          checked={replaceArtist}
          onChange={(event) => setReplaceArtist(event.currentTarget.checked)}
          label="Replace existing Artists instead of adding the selected Artists"
        />

        {error ? <div className="alert alert-danger">{error}</div> : null}

        {busy ? (
          <div className="text-center py-5">
            <Spinner animation="border" role="status" />
          </div>
        ) : predictions.length === 0 && !error ? (
          <div className="text-muted">
            No Local metadata was found. Fetch Danbooru or run Camie/EVA02
            explicitly to add predictions.
          </div>
        ) : (
          <>
            <div className="d-flex mb-3">
              <Button
                size="sm"
                variant="outline-secondary"
                onClick={() =>
                  setSelected(
                    allVisibleSelected
                      ? new Set()
                      : new Set(predictions.map(predictionKey))
                  )
                }
              >
                {allVisibleSelected ? "Unselect all" : "Select all"}
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
          onClick={() => void applySelected()}
        >
          {applying ? "Applying…" : "Apply selected metadata"}
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
