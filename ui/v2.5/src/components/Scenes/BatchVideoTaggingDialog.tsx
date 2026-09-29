import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Badge,
  Button,
  Form,
  Modal,
  ProgressBar,
  Spinner,
} from "react-bootstrap";

import { queryFindScenes } from "src/core/StashService";
import { ListFilterModel } from "src/models/list-filter/filter";
import { useToast } from "src/hooks/Toast";
import { CalculatedTagsSummary } from "src/components/Tagging/TaggingChangePlanModal";
import type { TaggingInheritedTag } from "src/components/Tagging/TaggingChangePlanModal";
import { collectFilteredSceneIDs } from "./batchVideoTaggingScope";

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

interface BatchReviewItem {
  prediction: TagPrediction;
  source: string;
  reason: string;
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
  inheritedTags?: TaggingInheritedTag[];
  createdCharacters: number;
  createdArtists: number;
  createdCopyrights: number;
  createdTags: number;
}

interface BatchSceneResult {
  sceneID: number;
  label: string;
  autoApply: TagPrediction[];
  needsReview: BatchReviewItem[];
  applied?: VideoTaggingApplyResponse;
  error?: string;
  booruMatched: boolean;
}

interface BatchStatusResponse {
  jobID: number;
  status: "queued" | "running" | "complete" | "failed";
  total: number;
  processed: number;
  items: BatchSceneResult[];
  error?: string;
}

interface TaggingPlanPreviewResponse {
  inheritedTags?: TaggingInheritedTag[];
}

type BatchScope = "selected" | "filtered" | "all";

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return response.json() as Promise<T>;
}

function candidateLabel(candidate: TargetCandidate) {
  const disambiguation = candidate.disambiguation?.trim();
  return disambiguation
    ? `${candidate.name} (${disambiguation})`
    : candidate.name;
}

function appliedCount(response?: VideoTaggingApplyResponse) {
  if (!response) return 0;
  return (
    (response.characters?.length ?? 0) +
    (response.artists?.length ?? 0) +
    (response.copyrights?.length ?? 0) +
    (response.tags?.length ?? 0)
  );
}

function reviewKey(sceneID: number, index: number) {
  return `${sceneID}:${index}`;
}

function resolveReviewPrediction(
  item: BatchReviewItem,
  candidateID?: number
): TagPrediction | undefined {
  if (item.reason === "local-identity-conflict") return undefined;
  if (item.reason !== "ambiguous-character") return item.prediction;

  const candidate = item.prediction.targetCandidates?.find(
    (value) => value.id === candidateID
  );
  if (!candidate) return undefined;
  const name = candidateLabel(candidate);
  return {
    ...item.prediction,
    name,
    rawName: name,
    targetPath: "/performers/" + candidate.id,
    targetExists: true,
    targetCandidates: undefined,
  };
}

function mergeAppliedEntities(
  existing: AppliedEntity[] | null | undefined,
  incoming: AppliedEntity[] | null | undefined
) {
  const byIdentity = new Map<string, AppliedEntity>();
  for (const entity of [...(existing ?? []), ...(incoming ?? [])]) {
    byIdentity.set(entity.category + ":" + entity.id, entity);
  }
  return [...byIdentity.values()];
}

function mergeInheritedTags(
  existing: TaggingInheritedTag[] | undefined,
  incoming: TaggingInheritedTag[] | undefined
) {
  const byID = new Map<number, TaggingInheritedTag>();
  for (const tag of [...(existing ?? []), ...(incoming ?? [])]) {
    const current = byID.get(tag.id);
    if (!current) {
      byID.set(tag.id, { ...tag, origins: [...tag.origins] });
      continue;
    }

    const origins = new Map(
      current.origins.map((origin) => [JSON.stringify(origin), origin])
    );
    for (const origin of tag.origins) {
      origins.set(JSON.stringify(origin), origin);
    }
    byID.set(tag.id, { ...current, origins: [...origins.values()] });
  }
  return [...byID.values()];
}

function mergeApplyResponses(
  existing: VideoTaggingApplyResponse | undefined,
  incoming: VideoTaggingApplyResponse | undefined
): VideoTaggingApplyResponse | undefined {
  if (!existing) return incoming;
  if (!incoming) return existing;
  return {
    ...existing,
    ...incoming,
    characters: mergeAppliedEntities(existing.characters, incoming.characters),
    artists: mergeAppliedEntities(existing.artists, incoming.artists),
    copyrights: mergeAppliedEntities(existing.copyrights, incoming.copyrights),
    tags: mergeAppliedEntities(existing.tags, incoming.tags),
    inheritedTags: mergeInheritedTags(
      existing.inheritedTags,
      incoming.inheritedTags
    ),
    createdCharacters: existing.createdCharacters + incoming.createdCharacters,
    createdArtists: existing.createdArtists + incoming.createdArtists,
    createdCopyrights: existing.createdCopyrights + incoming.createdCopyrights,
    createdTags: existing.createdTags + incoming.createdTags,
  };
}

export const BatchVideoTaggingDialog: React.FC<{
  filter: ListFilterModel;
  selectedIds: Set<string>;
  restrictedSceneIDs?: number[];
  onHide: () => void;
}> = ({ filter, selectedIds, restrictedSceneIDs, onHide }) => {
  const Toast = useToast();
  const [scope, setScope] = useState<BatchScope>(
    selectedIds.size > 0 ? "selected" : "filtered"
  );
  const [includeBooru, setIncludeBooru] = useState(true);
  const [autoApply, setAutoApply] = useState(true);
  const [replaceArtists, setReplaceArtists] = useState(false);
  const [starting, setStarting] = useState(false);
  const [applyingReview, setApplyingReview] = useState(false);
  const [jobID, setJobID] = useState<number>();
  const [status, setStatus] = useState<BatchStatusResponse>();
  const [error, setError] = useState<string>();
  const [selectedReview, setSelectedReview] = useState<Set<string>>(new Set());
  const [reviewInheritedTags, setReviewInheritedTags] = useState<
    Record<number, TaggingInheritedTag[]>
  >({});
  const [characterResolutions, setCharacterResolutions] = useState<
    Record<string, number>
  >({});

  const postBatch = useCallback(async <T,>(body: unknown) => {
    const response = await fetch("scene/tagging-batch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return readResponse<T>(response);
  }, []);

  const start = useCallback(async () => {
    setStarting(true);
    setError(undefined);
    setStatus(undefined);
    setSelectedReview(new Set());
    setCharacterResolutions({});

    try {
      let targetIDs: string[] = [];
      if (scope === "selected") {
        targetIDs = [...selectedIds];
      } else if (scope === "filtered") {
        targetIDs = await collectFilteredSceneIDs(
          filter,
          queryFindScenes,
          restrictedSceneIDs
        );
      }

      const result = await postBatch<{ jobID: number }>({
        action: "start",
        sceneIDs: scope === "all" ? [] : targetIDs.map(Number),
        all: scope === "all",
        includeBooru,
        autoApply,
        replaceArtist: replaceArtists,
      });
      setJobID(result.jobID);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setStarting(false);
    }
  }, [
    autoApply,
    filter,
    includeBooru,
    postBatch,
    replaceArtists,
    restrictedSceneIDs,
    scope,
    selectedIds,
  ]);

  useEffect(() => {
    if (!jobID) return;

    let cancelled = false;
    let timeout: ReturnType<typeof setTimeout> | undefined;

    const poll = async () => {
      try {
        const next = await postBatch<BatchStatusResponse>({
          action: "status",
          jobID,
        });
        if (cancelled) return;
        setStatus(next);
        if (next.status === "queued" || next.status === "running") {
          timeout = setTimeout(() => void poll(), 750);
        }
      } catch (cause) {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : String(cause));
        }
      }
    };

    void poll();
    return () => {
      cancelled = true;
      if (timeout) clearTimeout(timeout);
    };
  }, [jobID, postBatch]);

  const forgetAndClose = useCallback(async () => {
    if (jobID && status?.status !== "running") {
      try {
        await postBatch({ action: "forget", jobID });
      } catch {
        // Cleanup is best-effort; abandoned running state also expires server-side.
      }
    }
    onHide();
  }, [jobID, onHide, postBatch, status?.status]);

  const reviewCount = useMemo(
    () =>
      status?.items.reduce((sum, item) => sum + item.needsReview.length, 0) ??
      0,
    [status]
  );

  const autoApplied = useMemo(
    () =>
      status?.items.reduce(
        (sum, item) => sum + appliedCount(item.applied),
        0
      ) ?? 0,
    [status]
  );

  const unresolvedSelected = useMemo(() => {
    if (!status) return 0;
    let count = 0;
    for (const scene of status.items) {
      scene.needsReview.forEach((item, index) => {
        const key = reviewKey(scene.sceneID, index);
        if (
          selectedReview.has(key) &&
          item.reason === "ambiguous-character" &&
          !characterResolutions[key]
        ) {
          count += 1;
        }
      });
    }
    return count;
  }, [characterResolutions, selectedReview, status]);

  const toggleReview = useCallback((key: string) => {
    setSelectedReview((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }, []);

  useEffect(() => {
    if (status?.status !== "complete" || selectedReview.size === 0) {
      setReviewInheritedTags({});
      return;
    }

    let cancelled = false;
    setReviewInheritedTags({});

    const preview = async () => {
      const next: Record<number, TaggingInheritedTag[]> = {};
      const scenes = status.items.filter((scene) =>
        scene.needsReview.some((_item, index) =>
          selectedReview.has(reviewKey(scene.sceneID, index))
        )
      );
      let nextScene = 0;
      await Promise.all(
        Array.from({ length: Math.min(scenes.length, 4) }, async () => {
          while (nextScene < scenes.length) {
            const scene = scenes[nextScene];
            nextScene += 1;
            const tags: TagPrediction[] = [];
            scene.needsReview.forEach((item, index) => {
              const key = reviewKey(scene.sceneID, index);
              if (!selectedReview.has(key)) return;
              const prediction = resolveReviewPrediction(
                item,
                characterResolutions[key]
              );
              if (prediction) tags.push(prediction);
            });
            if (tags.length === 0) return;

            try {
              const response = await fetch(
                "scene/" + scene.sceneID + "/knowledge-tags?preview=1",
                {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ tags, replaceArtist: replaceArtists }),
                }
              );
              const plan =
                await readResponse<TaggingPlanPreviewResponse>(response);
              next[scene.sceneID] = plan.inheritedTags ?? [];
            } catch {
              // Preview is advisory; the apply request still reports any error.
            }
          }
        })
      );
      if (!cancelled) setReviewInheritedTags(next);
    };

    void preview();
    return () => {
      cancelled = true;
    };
  }, [characterResolutions, replaceArtists, selectedReview, status]);

  const applySelectedReview = useCallback(async () => {
    if (!status || selectedReview.size === 0 || unresolvedSelected > 0) return;
    setApplyingReview(true);
    setError(undefined);

    try {
      const appliedKeys = new Set<string>();
      const reviewedApplications = new Map<number, VideoTaggingApplyResponse>();
      for (const scene of status.items) {
        const tags: TagPrediction[] = [];
        scene.needsReview.forEach((item, index) => {
          const key = reviewKey(scene.sceneID, index);
          if (!selectedReview.has(key)) return;
          const prediction = resolveReviewPrediction(
            item,
            characterResolutions[key]
          );
          if (!prediction) return;
          tags.push(prediction);
          appliedKeys.add(key);
        });

        if (tags.length === 0) continue;
        const response = await fetch(`scene/${scene.sceneID}/knowledge-tags`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ tags, replaceArtist: replaceArtists }),
        });
        reviewedApplications.set(
          scene.sceneID,
          await readResponse<VideoTaggingApplyResponse>(response)
        );
      }

      setStatus((current) =>
        current
          ? {
              ...current,
              items: current.items.map((scene) => ({
                ...scene,
                needsReview: scene.needsReview.filter(
                  (_item, index) =>
                    !appliedKeys.has(reviewKey(scene.sceneID, index))
                ),
                applied: mergeApplyResponses(
                  scene.applied,
                  reviewedApplications.get(scene.sceneID)
                ),
              })),
            }
          : current
      );
      setSelectedReview(new Set());
      Toast.success(
        `Applied ${appliedKeys.size} reviewed metadata item${
          appliedKeys.size === 1 ? "" : "s"
        }.`
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
      Toast.error(cause);
    } finally {
      setApplyingReview(false);
    }
  }, [
    Toast,
    characterResolutions,
    replaceArtists,
    selectedReview,
    status,
    unresolvedSelected,
  ]);

  const progress = status?.total
    ? Math.min(100, Math.round((status.processed / status.total) * 100))
    : 0;
  const running = status?.status === "queued" || status?.status === "running";

  return (
    <Modal
      show
      onHide={() => void forgetAndClose()}
      size="lg"
      backdrop="static"
    >
      <Modal.Header closeButton>
        <Modal.Title>Batch Video Tagging</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        {!jobID && (
          <>
            <Form.Group className="mb-3">
              <Form.Label>Scope</Form.Label>
              <Form.Check
                type="radio"
                name="batch-video-tagging-scope"
                id="batch-video-tagging-selected"
                label={`Selected videos (${selectedIds.size})`}
                checked={scope === "selected"}
                disabled={selectedIds.size === 0}
                onChange={() => setScope("selected")}
              />
              <Form.Check
                type="radio"
                name="batch-video-tagging-scope"
                id="batch-video-tagging-filtered"
                label="Current filter"
                checked={scope === "filtered"}
                onChange={() => setScope("filtered")}
              />
              <Form.Check
                type="radio"
                name="batch-video-tagging-scope"
                id="batch-video-tagging-all"
                label="All videos"
                checked={scope === "all"}
                onChange={() => setScope("all")}
              />
            </Form.Group>

            <Form.Check
              className="mb-2"
              type="checkbox"
              id="batch-video-tagging-booru"
              label="Fetch booru metadata when an MD5 match is available"
              checked={includeBooru}
              onChange={(event) => setIncludeBooru(event.currentTarget.checked)}
            />
            <Form.Check
              className="mb-2"
              type="checkbox"
              id="batch-video-tagging-auto-apply"
              label="Auto-apply unambiguous metadata"
              checked={autoApply}
              onChange={(event) => setAutoApply(event.currentTarget.checked)}
            />
            <Form.Check
              className="mb-3"
              type="checkbox"
              id="batch-video-tagging-replace-artists"
              label="Replace existing Artists when applying Artist metadata"
              checked={replaceArtists}
              onChange={(event) =>
                setReplaceArtists(event.currentTarget.checked)
              }
            />
          </>
        )}

        {running && (
          <div className="mb-3">
            <div className="d-flex align-items-center mb-2">
              <Spinner animation="border" size="sm" className="mr-2" />
              <span>
                Processing {status?.processed ?? 0} / {status?.total ?? 0}
              </span>
            </div>
            <ProgressBar now={progress} label={`${progress}%`} />
          </div>
        )}

        {status?.status === "complete" && (
          <div className="mb-3">
            <div className="mb-2">
              <Badge variant="success" className="mr-2">
                Complete
              </Badge>
              <span>
                {status.processed} video{status.processed === 1 ? "" : "s"};{" "}
                {autoApplied} metadata item{autoApplied === 1 ? "" : "s"}{" "}
                auto-applied; {reviewCount} item{reviewCount === 1 ? "" : "s"}{" "}
                need review.
              </span>
            </div>

            {status.items.map((scene) => (
              <div key={scene.sceneID} className="card bg-secondary mb-2">
                <div className="card-body py-2">
                  <div className="d-flex flex-wrap align-items-center mb-1">
                    <strong className="mr-2">
                      {scene.label || `Video ${scene.sceneID}`}
                    </strong>
                    {scene.booruMatched && (
                      <Badge variant="info" className="mr-2">
                        booru match
                      </Badge>
                    )}
                    {scene.applied && (
                      <Badge variant="success">
                        {appliedCount(scene.applied)} applied
                      </Badge>
                    )}
                  </div>

                  {scene.error && (
                    <div className="text-danger mb-2">{scene.error}</div>
                  )}
                  <CalculatedTagsSummary
                    tags={scene.applied?.inheritedTags ?? []}
                    heading="Inherited Tags from applied metadata"
                  />

                  {scene.needsReview.map((item, index) => {
                    const key = reviewKey(scene.sceneID, index);
                    const ambiguous = item.reason === "ambiguous-character";
                    const localConflict =
                      item.reason === "local-identity-conflict";
                    const resolved =
                      !ambiguous || Boolean(characterResolutions[key]);
                    return (
                      <div key={key} className="border-top py-2">
                        <div className="d-flex align-items-center flex-wrap">
                          <Form.Check
                            type="checkbox"
                            id={`batch-review-${key}`}
                            className="mr-2"
                            checked={selectedReview.has(key)}
                            disabled={!resolved || localConflict}
                            onChange={() => toggleReview(key)}
                          />
                          <strong className="mr-2">
                            {item.prediction.name}
                          </strong>
                          <Badge variant="secondary" className="mr-2">
                            {item.prediction.category}
                          </Badge>
                          <Badge
                            variant={
                              item.source === "local" ? "success" : "info"
                            }
                          >
                            {item.source}
                          </Badge>
                        </div>
                        <small className="text-muted d-block mt-1">
                          {item.reason === "local-identity-conflict"
                            ? "Suppressed by authoritative Local filename identity. It is shown for review but cannot be applied as an override."
                            : "Same-name Character is ambiguous. Choose the intended native Character before applying."}
                        </small>

                        {ambiguous && (
                          <Form.Control
                            as="select"
                            className="mt-2"
                            value={characterResolutions[key] ?? ""}
                            onChange={(event) => {
                              const value = Number(event.currentTarget.value);
                              setCharacterResolutions((current) => ({
                                ...current,
                                [key]: value,
                              }));
                            }}
                          >
                            <option value="">Choose Character…</option>
                            {(item.prediction.targetCandidates ?? []).map(
                              (candidate) => (
                                <option key={candidate.id} value={candidate.id}>
                                  {candidateLabel(candidate)}
                                </option>
                              )
                            )}
                          </Form.Control>
                        )}
                      </div>
                    );
                  })}
                  <CalculatedTagsSummary
                    tags={reviewInheritedTags[scene.sceneID] ?? []}
                    heading="Calculated Tags from selected review items"
                  />
                </div>
              </div>
            ))}
          </div>
        )}

        {status?.status === "failed" && (
          <div className="text-danger mb-3">
            Batch failed: {status.error || "Unknown batch error"}
          </div>
        )}
        {error && <div className="text-danger mb-3">{error}</div>}
      </Modal.Body>
      <Modal.Footer>
        {!jobID && (
          <Button
            variant="primary"
            onClick={() => void start()}
            disabled={starting}
          >
            {starting ? (
              <Spinner animation="border" size="sm" />
            ) : (
              "Start batch"
            )}
          </Button>
        )}
        {status?.status === "complete" && reviewCount > 0 && (
          <Button
            variant="primary"
            disabled={
              applyingReview ||
              selectedReview.size === 0 ||
              unresolvedSelected > 0
            }
            onClick={() => void applySelectedReview()}
          >
            {applyingReview
              ? "Applying…"
              : `Apply selected review items (${selectedReview.size})`}
          </Button>
        )}
        <Button variant="secondary" onClick={() => void forgetAndClose()}>
          Close
        </Button>
      </Modal.Footer>
    </Modal>
  );
};
