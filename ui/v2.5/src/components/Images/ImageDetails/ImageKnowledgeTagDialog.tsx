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

function predictionKey(prediction: CamiePrediction) {
  return `${prediction.category}\u0000${prediction.name}`;
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

const CATEGORY_ORDER = ["character", "artist", "copyright", "general", "meta"];

export const ImageKnowledgeTagDialog: React.FC<IProps> = ({
  imageId,
  onHide,
  onApplied,
}) => {
  const Toast = useToast();
  const history = useHistory();
  const [threshold, setThreshold] = useState("0.492");
  const [limit, setLimit] = useState("50");
  const [predictions, setPredictions] = useState<CamiePrediction[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [backend, setBackend] = useState<string>();
  const [booruMetadata, setBooruMetadata] = useState<BooruMetadataResponse>();
  const [loading, setLoading] = useState(true);
  const [applying, setApplying] = useState(false);
  const [replaceArtist, setReplaceArtist] = useState(false);
  const [error, setError] = useState<string>();

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
      setBooruMetadata(undefined);
      try {
        const response = await fetch(
          `image/${imageId}/knowledge-tags?threshold=${encodeURIComponent(parsedThreshold)}&limit=${encodeURIComponent(parsedLimit)}`,
          { method: "POST" }
        );
        const result = await readResponse<CamieTagsResponse>(response);
        setPredictions(result.tags);
        setBackend(result.backend);
        setSelected(new Set(result.tags.map(predictionKey)));
      } catch (cause) {
        setPredictions([]);
        setSelected(new Set());
        setError(cause instanceof Error ? cause.message : String(cause));
      } finally {
        setLoading(false);
      }
    },
    [imageId]
  );

  const loadBooruMetadata = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    setBackend(undefined);
    try {
      const response = await fetch(`image/${imageId}/booru-metadata`);
      const result = await readResponse<BooruMetadataResponse>(response);
      setBooruMetadata(result);
      setPredictions(result.tags);
      setSelected(new Set(result.tags.map(predictionKey)));
    } catch (cause) {
      setBooruMetadata(undefined);
      setPredictions([]);
      setSelected(new Set());
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }, [imageId]);

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
    const groups = new Map<string, CamiePrediction[]>();
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
    const tags = predictions.filter((prediction) =>
      selected.has(predictionKey(prediction))
    );
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
              {backend === "remote" ? "Remote worker" : "Local worker"}
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
          Analyze locally/remotely with Camie, or look up the image on supported
          boorus by MD5 and re-extract the post metadata. A filename MD5 is
          tried first; if it is stale or unmatched, StashBooru hashes the actual
          file and retries. Characters become Characters, the highest selected
          artist becomes the image Artist, Copyright stays in the Copyright
          namespace, and general/meta entries become Tags.
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
                {selected.size} / {predictions.length} selected
              </span>
            </div>

            <div style={{ maxHeight: "55vh", overflowY: "auto" }}>
              {grouped.map(([category, items]) => (
                <div className="mb-4" key={category}>
                  <h5>{categoryLabel(category)}</h5>
                  {category === "artist" && items.length > 1 ? (
                    <div className="small text-muted mb-2">
                      Images support one Artist. If several artists stay
                      selected, the highest-confidence one is used.
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
                        {prediction.source?.includes("filename") ? (
                          <Badge className="ml-2" variant="info">
                            filename
                          </Badge>
                        ) : prediction.source?.startsWith("booru:") ? (
                          <Badge className="ml-2" variant="info">
                            {prediction.source.slice("booru:".length)}
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
          disabled={loading || applying || selected.size === 0}
          onClick={() => void applySelected()}
        >
          {applying ? "Applying…" : "Apply selected metadata"}
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
