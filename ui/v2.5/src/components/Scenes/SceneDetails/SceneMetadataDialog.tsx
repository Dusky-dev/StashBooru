import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Badge, Button, Form, Modal, Spinner } from "react-bootstrap";
import { faExternalLinkAlt, faSearch } from "@fortawesome/free-solid-svg-icons";
import { useHistory } from "react-router-dom";

import { Icon } from "src/components/Shared/Icon";
import { useToast } from "src/hooks/Toast";

interface MetadataPrediction {
  name: string;
  category: string;
  score: number;
  rawName?: string;
  source?: string;
  targetPath?: string;
  targetExists?: boolean;
}

interface LocalMetadataResponse {
  tags: MetadataPrediction[];
}

interface BooruMetadataResponse {
  source: string;
  postID: string;
  postURL?: string;
  md5: string;
  md5Source: "filename" | "file";
  tags: MetadataPrediction[];
}

interface AppliedEntity {
  id: number;
  name: string;
  category: string;
  score: number;
  created: boolean;
}

interface ApplyResponse {
  sceneID: number;
  characters: AppliedEntity[];
  artist?: AppliedEntity;
  copyrights: AppliedEntity[];
  tags: AppliedEntity[];
  preservedArtist: boolean;
  createdCharacters: number;
  createdArtists: number;
  createdCopyrights: number;
  createdTags: number;
}

type Provenance = "local" | "booru";

interface ReviewPrediction extends MetadataPrediction {
  provenance: Provenance[];
}

interface IProps {
  sceneId: string;
  onHide: () => void;
  onApplied: () => Promise<unknown>;
}

const CATEGORY_ORDER = ["character", "artist", "copyright", "general", "meta"];
const LOCAL_AUTHORITY_CATEGORIES = new Set([
  "character",
  "artist",
  "copyright",
]);

function normalizedValue(value?: string) {
  return (value ?? "")
    .trim()
    .replaceAll("_", " ")
    .replace(/\s+/g, " ")
    .toLocaleLowerCase();
}

function identityKeys(prediction: MetadataPrediction) {
  const category = prediction.category.trim().toLocaleLowerCase();
  return [prediction.name, prediction.rawName]
    .map(normalizedValue)
    .filter(Boolean)
    .map((value) => `${category}\u0000${value}`);
}

function predictionKey(prediction: MetadataPrediction) {
  return (
    identityKeys(prediction)[0] ??
    `${prediction.category}\u0000${prediction.name}`
  );
}

function addProvenance(
  current: Provenance[],
  source: Provenance
): Provenance[] {
  if (current.includes(source)) return current;
  return source === "local" ? [source, ...current] : [...current, source];
}

function mergeVideoMetadata(
  local: MetadataPrediction[],
  booru: MetadataPrediction[]
): ReviewPrediction[] {
  const merged: ReviewPrediction[] = [];
  const indexByKey = new Map<string, number>();
  const localIdentityCategories = new Set<string>();

  const add = (prediction: MetadataPrediction, source: Provenance) => {
    const keys = identityKeys(prediction);
    let existingIndex: number | undefined;
    for (const key of keys) {
      const index = indexByKey.get(key);
      if (index !== undefined) {
        existingIndex = index;
        break;
      }
    }

    if (existingIndex === undefined) {
      const index = merged.length;
      merged.push({ ...prediction, provenance: [source] });
      for (const key of keys) indexByKey.set(key, index);
      return;
    }

    const current = merged[existingIndex];
    const next: ReviewPrediction = {
      ...current,
      score: Math.max(current.score, prediction.score),
      rawName: current.rawName || prediction.rawName,
      targetExists: current.targetExists || prediction.targetExists,
      targetPath:
        current.targetExists && current.targetPath
          ? current.targetPath
          : prediction.targetPath || current.targetPath,
      provenance: addProvenance(current.provenance, source),
    };
    merged[existingIndex] = next;
    for (const key of [...identityKeys(current), ...keys, ...identityKeys(next)]) {
      indexByKey.set(key, existingIndex);
    }
  };

  for (const prediction of local) {
    const category = prediction.category.trim().toLocaleLowerCase();
    if (LOCAL_AUTHORITY_CATEGORIES.has(category)) {
      localIdentityCategories.add(category);
    }
    add(prediction, "local");
  }

  for (const prediction of booru) {
    const category = prediction.category.trim().toLocaleLowerCase();
    if (localIdentityCategories.has(category)) {
      const overlapsLocal = identityKeys(prediction).some((key) => {
        const index = indexByKey.get(key);
        return index !== undefined && merged[index].provenance.includes("local");
      });
      if (!overlapsLocal) continue;
    }
    add(prediction, "booru");
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

export const SceneMetadataDialog: React.FC<IProps> = ({
  sceneId,
  onHide,
  onApplied,
}) => {
  const Toast = useToast();
  const history = useHistory();
  const [localPredictions, setLocalPredictions] = useState<MetadataPrediction[]>(
    []
  );
  const [booruPredictions, setBooruPredictions] = useState<MetadataPrediction[]>(
    []
  );
  const [booruMetadata, setBooruMetadata] = useState<BooruMetadataResponse>();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [replaceArtist, setReplaceArtist] = useState(false);
  const [loading, setLoading] = useState(true);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState<string>();

  const predictions = useMemo(
    () => mergeVideoMetadata(localPredictions, booruPredictions),
    [booruPredictions, localPredictions]
  );

  const grouped = useMemo(() => {
    const groups = new Map<string, ReviewPrediction[]>();
    for (const prediction of predictions) {
      const items = groups.get(prediction.category) ?? [];
      items.push(prediction);
      groups.set(prediction.category, items);
    }
    return [...groups.entries()].sort(([left], [right]) => {
      const leftIndex = CATEGORY_ORDER.indexOf(left);
      const rightIndex = CATEGORY_ORDER.indexOf(right);
      if (leftIndex === -1 && rightIndex === -1) return left.localeCompare(right);
      if (leftIndex === -1) return 1;
      if (rightIndex === -1) return -1;
      return leftIndex - rightIndex;
    });
  }, [predictions]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoading(true);
      try {
        const response = await fetch(`scene/${sceneId}/metadata`);
        const result = await readResponse<LocalMetadataResponse>(response);
        if (cancelled) return;
        setLocalPredictions(result.tags);
        setSelected(new Set(result.tags.map(predictionKey)));
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
  }, [sceneId]);

  const fetchBooru = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const response = await fetch(`scene/${sceneId}/booru-metadata`);
      const result = await readResponse<BooruMetadataResponse>(response);
      setBooruMetadata(result);
      setBooruPredictions(result.tags);
      setSelected((current) => {
        const next = new Set(current);
        for (const item of result.tags) next.add(predictionKey(item));
        return next;
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }, [sceneId]);

  const visibleSelectedCount = useMemo(
    () =>
      predictions.filter((prediction) => selected.has(predictionKey(prediction)))
        .length,
    [predictions, selected]
  );

  const toggle = useCallback((prediction: MetadataPrediction) => {
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
      const response = await fetch(`scene/${sceneId}/metadata`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tags, replaceArtist }),
      });
      const result = await readResponse<ApplyResponse>(response);
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
      Toast.success(
        `Applied ${appliedCount} metadata item${appliedCount === 1 ? "" : "s"}; created ${createdCount} new entr${createdCount === 1 ? "y" : "ies"}.${result.preservedArtist ? " Existing Artist was preserved." : ""}`
      );
      await onApplied();
      onHide();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      Toast.error(cause);
    } finally {
      setApplying(false);
    }
  }, [Toast, onApplied, onHide, predictions, replaceArtist, sceneId, selected]);

  return (
    <Modal show onHide={onHide} size="lg" centered>
      <Modal.Header closeButton>
        <Modal.Title>Video metadata</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <div className="d-flex flex-wrap align-items-center mb-3">
          <Button
            variant="secondary"
            disabled={loading || applying}
            onClick={() => void fetchBooru()}
          >
            Fetch from booru
          </Button>
          <span className="ml-3 text-muted">
            Local filename metadata is loaded automatically. No Camie inference is
            run for videos.
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

        <div className="mb-3 text-muted">
          Local filename metadata has priority for Characters, Artist and
          Copyright. If the booru reports the same item, its row shows both
          sources. A conflicting booru item in a locally populated identity
          category is not mixed in. Booru can fill identity categories that the
          filename does not provide, and can add general/meta tags.
        </div>

        <Form.Check
          className="mb-3"
          type="checkbox"
          id="video-metadata-replace-artist"
          checked={replaceArtist}
          onChange={(event) => setReplaceArtist(event.currentTarget.checked)}
          label="Replace the existing Artist if this video already has one"
        />

        {error ? <div className="alert alert-danger">{error}</div> : null}

        {loading ? (
          <div className="text-center py-5">
            <Spinner animation="border" role="status" />
          </div>
        ) : predictions.length === 0 && !error ? (
          <div className="text-muted">
            No local metadata was found. You can still try an exact booru lookup.
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
                Clear selection
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
                          id={`video-metadata-${category}-${index}`}
                          checked={selected.has(key)}
                          onChange={() => toggle(prediction)}
                          label={prediction.name}
                        />
                        {prediction.provenance.map((source) => (
                          <Badge
                            className="ml-2"
                            key={source}
                            variant={source === "local" ? "success" : "info"}
                          >
                            {source}
                          </Badge>
                        ))}
                        {prediction.targetExists ? (
                          <Badge className="ml-2" variant="secondary">
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
