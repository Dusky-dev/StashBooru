import React, {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import cloneDeep from "lodash-es/cloneDeep";
import { Badge, Button, Form, Modal, ProgressBar, Spinner } from "react-bootstrap";

import { after } from "src/patch";
import { queryFindScenes } from "src/core/StashService";
import { ListFilterModel } from "src/models/list-filter/filter";
import { useToast } from "src/hooks/Toast";
import {
  FilteredListToolbar,
  IFilteredListToolbar,
} from "src/components/List/FilteredListToolbar";
import {
  IListFilterOperation,
  ListOperations,
} from "src/components/List/ListOperationButtons";

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

type BatchScope = "selected" | "filtered" | "all";

type ListOperationsProps = React.ComponentProps<typeof ListOperations>;

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return response.json() as Promise<T>;
}

async function collectFilteredSceneIDs(
  filter: ListFilterModel,
  restrictedSceneIDs?: number[]
): Promise<string[]> {
  if (restrictedSceneIDs) {
    return restrictedSceneIDs.map(String);
  }

  const pageSize = 250;
  const ids: string[] = [];
  const seen = new Set<string>();
  let page = 1;
  let total = Number.POSITIVE_INFINITY;

  while (ids.length < total) {
    const pageFilter = cloneDeep(filter);
    pageFilter.currentPage = page;
    pageFilter.itemsPerPage = pageSize;
    const result = await queryFindScenes(pageFilter);
    const found = result.data.findScenes.scenes;
    total = result.data.findScenes.count;

    for (const scene of found) {
      if (!seen.has(scene.id)) {
        seen.add(scene.id);
        ids.push(scene.id);
      }
    }

    if (found.length === 0 || found.length < pageSize) break;
    page += 1;
  }

  return ids;
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

const BatchVideoTaggingDialog: React.FC<{
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
  const [anchorSceneID, setAnchorSceneID] = useState<string>();
  const [jobID, setJobID] = useState<number>();
  const [status, setStatus] = useState<BatchStatusResponse>();
  const [error, setError] = useState<string>();
  const [selectedReview, setSelectedReview] = useState<Set<string>>(new Set());
  const [characterResolutions, setCharacterResolutions] = useState<
    Record<string, number>
  >({});

  const postBatch = useCallback(
    async <T,>(anchor: string, body: unknown) => {
      const response = await fetch(`scene/${anchor}/knowledge-tags?batch=1`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      return readResponse<T>(response);
    },
    []
  );

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
        targetIDs = await collectFilteredSceneIDs(filter, restrictedSceneIDs);
      }

      let anchor = targetIDs[0] ?? [...selectedIds][0];
      if (!anchor) {
        const filtered = await collectFilteredSceneIDs(filter, restrictedSceneIDs);
        anchor = filtered[0];
      }
      if (!anchor) {
        throw new Error("No videos are available to anchor the batch request.");
      }

      const result = await postBatch<{ jobID: number }>(anchor, {
        action: "start",
        sceneIDs: scope === "all" ? [] : targetIDs.map(Number),
        all: scope === "all",
        includeBooru,
        autoApply,
        replaceArtist: replaceArtists,
      });
      setAnchorSceneID(anchor);
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
    if (!jobID || !anchorSceneID) return;

    let cancelled = false;
    let timeout: ReturnType<typeof setTimeout> | undefined;

    const poll = async () => {
      try {
        const next = await postBatch<BatchStatusResponse>(anchorSceneID, {
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
  }, [anchorSceneID, jobID, postBatch]);

  const forgetAndClose = useCallback(async () => {
    if (jobID && anchorSceneID && status?.status !== "running") {
      try {
        await postBatch(anchorSceneID, { action: "forget", jobID });
      } catch {
        // Cleanup is best-effort; closing the review should never be blocked.
      }
    }
    onHide();
  }, [anchorSceneID, jobID, onHide, postBatch, status?.status]);

  const reviewCount = useMemo(
    () =>
      status?.items.reduce((sum, item) => sum + item.needsReview.length, 0) ?? 0,
    [status]
  );

  const autoApplied = useMemo(
    () => status?.items.reduce((sum, item) => sum + appliedCount(item.applied), 0) ?? 0,
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

  const applySelectedReview = useCallback(async () => {
    if (!status || selectedReview.size === 0 || unresolvedSelected > 0) return;
    setApplyingReview(true);
    setError(undefined);

    try {
      const appliedKeys = new Set<string>();
      for (const scene of status.items) {
        const tags: TagPrediction[] = [];
        scene.needsReview.forEach((item, index) => {
          const key = reviewKey(scene.sceneID, index);
          if (!selectedReview.has(key)) return;

          let prediction = item.prediction;
          if (item.reason === "ambiguous-character") {
            const candidateID = characterResolutions[key];
            const candidate = prediction.targetCandidates?.find(
              (value) => value.id === candidateID
            );
            if (!candidate) return;
            const name = candidateLabel(candidate);
            prediction = {
              ...prediction,
              name,
              rawName: name,
              targetPath: `/performers/${candidate.id}`,
              targetExists: true,
              targetCandidates: undefined,
            };
          }
          tags.push(prediction);
          appliedKeys.add(key);
        });

        if (tags.length === 0) continue;
        const response = await fetch(`scene/${scene.sceneID}/knowledge-tags`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ tags, replaceArtist: replaceArtists }),
        });
        await readResponse<VideoTaggingApplyResponse>(response);
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
    <Modal show onHide={() => void forgetAndClose()} size="lg" backdrop="static">
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
              onChange={(event) => setReplaceArtists(event.currentTarget.checked)}
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
                {status.processed} video{status.processed === 1 ? "" : "s"}; {autoApplied}{" "}
                metadata item{autoApplied === 1 ? "" : "s"} auto-applied; {reviewCount}{" "}
                item{reviewCount === 1 ? "" : "s"} need review.
              </span>
            </div>

            {status.items.map((scene) => (
              <div key={scene.sceneID} className="card bg-secondary mb-2">
                <div className="card-body py-2">
                  <div className="d-flex flex-wrap align-items-center mb-1">
                    <strong className="mr-2">{scene.label || `Video ${scene.sceneID}`}</strong>
                    {scene.booruMatched && (
                      <Badge variant="info" className="mr-2">
                        booru match
                      </Badge>
                    )}
                    {scene.applied && (
                      <Badge variant="success">
                        {appliedCount(scene.applied)} auto-applied
                      </Badge>
                    )}
                  </div>

                  {scene.error && <div className="text-danger mb-2">{scene.error}</div>}

                  {scene.needsReview.map((item, index) => {
                    const key = reviewKey(scene.sceneID, index);
                    const ambiguous = item.reason === "ambiguous-character";
                    const resolved = !ambiguous || Boolean(characterResolutions[key]);
                    return (
                      <div key={key} className="border-top py-2">
                        <div className="d-flex align-items-center flex-wrap">
                          <Form.Check
                            type="checkbox"
                            id={`batch-review-${key}`}
                            className="mr-2"
                            checked={selectedReview.has(key)}
                            disabled={!resolved}
                            onChange={() => toggleReview(key)}
                          />
                          <strong className="mr-2">{item.prediction.name}</strong>
                          <Badge variant="secondary" className="mr-2">
                            {item.prediction.category}
                          </Badge>
                          <Badge variant={item.source === "local" ? "success" : "info"}>
                            {item.source}
                          </Badge>
                        </div>
                        <small className="text-muted d-block mt-1">
                          {item.reason === "local-identity-conflict"
                            ? "Conflicts with authoritative local filename identity. Select only if you explicitly want the lower-priority identity too."
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
                            {(item.prediction.targetCandidates ?? []).map((candidate) => (
                              <option key={candidate.id} value={candidate.id}>
                                {candidateLabel(candidate)}
                              </option>
                            ))}
                          </Form.Control>
                        )}
                      </div>
                    );
                  })}
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
          <Button variant="primary" onClick={() => void start()} disabled={starting}>
            {starting ? <Spinner animation="border" size="sm" /> : "Start batch"}
          </Button>
        )}
        {status?.status === "complete" && reviewCount > 0 && (
          <Button
            variant="primary"
            disabled={
              applyingReview || selectedReview.size === 0 || unresolvedSelected > 0
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

const BatchVideoTaggingOperations: React.FC<{
  original: React.ReactElement<ListOperationsProps>;
  toolbarProps: IFilteredListToolbar;
  restrictedSceneIDs?: number[];
}> = ({ original, toolbarProps, restrictedSceneIDs }) => {
  const [show, setShow] = useState(false);
  const selectedIds = toolbarProps.listSelect.selectedIds;
  const originalOperations = original.props.operations ?? [];

  const operations = useMemo<IListFilterOperation[]>(
    () => [
      ...originalOperations,
      {
        text: "Batch Video Tagging…",
        onClick: () => setShow(true),
      },
    ],
    [originalOperations]
  );

  return (
    <>
      {React.cloneElement(original, { operations })}
      {show && (
        <BatchVideoTaggingDialog
          filter={toolbarProps.filter}
          selectedIds={selectedIds}
          restrictedSceneIDs={restrictedSceneIDs}
          onHide={() => setShow(false)}
        />
      )}
    </>
  );
};

function enhanceSceneListTree(
  node: React.ReactNode,
  restrictedSceneIDs?: number[]
): React.ReactNode {
  if (!React.isValidElement(node)) return node;

  if (node.type === FilteredListToolbar) {
    const toolbarProps = node.props as IFilteredListToolbar;
    const original = toolbarProps.operationComponent;
    if (React.isValidElement<ListOperationsProps>(original)) {
      const originalProps = original.props;
      if (originalProps.operationsMenuClassName === "scene-list-operations-dropdown") {
        return React.cloneElement(node, {
          operationComponent: (
            <BatchVideoTaggingOperations
              original={original}
              toolbarProps={toolbarProps}
              restrictedSceneIDs={restrictedSceneIDs}
            />
          ),
        });
      }
    }
    return node;
  }

  const props = node.props as { children?: React.ReactNode };
  if (props.children === undefined) return node;
  return React.cloneElement(
    node,
    undefined,
    React.Children.map(props.children, (child) =>
      enhanceSceneListTree(child, restrictedSceneIDs)
    )
  );
}

const SceneBatchEnhancer: React.FC<{
  result: React.ReactNode;
  restrictedSceneIDs?: number[];
}> = ({ result, restrictedSceneIDs }) => (
  <>{enhanceSceneListTree(result, restrictedSceneIDs)}</>
);

after(
  "FilteredSceneList",
  (
    props: { sceneIDs?: number[] },
    result: React.ReactNode
  ): React.ReactNode => (
    <SceneBatchEnhancer result={result} restrictedSceneIDs={props.sceneIDs} />
  )
);
