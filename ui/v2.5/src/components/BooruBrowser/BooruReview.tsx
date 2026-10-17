import { type ReactNode, useEffect, useRef, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { Link } from "react-router-dom";
import { type BooruPost, type BooruSource, readBooruResponse } from "./browser";

export interface ReviewState {
  id: string;
  version: number;
  ignoredAt: string;
  downloadedAt: string;
  updatedAt: string;
  checkedAt?: string;
  matches?: {
    kind: "image" | "scene";
    id: number;
    fileID: string;
    evidence: string;
  }[];
  matchStatus?: string;
}
interface ReviewChange {
  id: string;
  version: number;
  ignored?: boolean;
  downloaded?: boolean;
}

export function ReviewEvidence({ state }: { state?: ReviewState }) {
  if (!state)
    return <small className="text-muted">Review status not loaded</small>;
  return (
    <div className="booru-review-evidence">
      {state.ignoredAt && <span title={state.ignoredAt}>Ignored</span>}
      {state.downloadedAt && (
        <span title={state.downloadedAt}>Marked downloaded · manual</span>
      )}
      {state.matches?.map((match) => (
        <Link
          key={`${match.kind}:${match.id}:${match.fileID}`}
          to={`/${match.kind === "image" ? "images" : "scenes"}/${match.id}`}
          title={`Current-file MD5 verified ${state.checkedAt ?? ""}`}
        >
          In library · {match.kind === "image" ? "Image" : "Video"} #{match.id}
        </Link>
      ))}
      {!state.matches?.length && (
        <small className="text-muted">
          {state.matchStatus === "no-remote-md5"
            ? "Local match unknown · no remote MD5"
            : state.matchStatus === "no-indexed-match"
              ? "No indexed primary-file MD5 match"
              : "Local check incomplete"}
        </small>
      )}
    </div>
  );
}

export function BooruReview({
  source,
  posts,
  showIgnored,
  onStates,
  children,
}: {
  source: BooruSource;
  posts: BooruPost[];
  showIgnored: boolean;
  onStates: (states: ReviewState[]) => void;
  children: (post: BooruPost, controls: ReactNode) => ReactNode;
}) {
  const [states, setStates] = useState<ReviewState[]>();
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [undo, setUndo] = useState<ReviewChange[]>();
  const [notice, setNotice] = useState("");
  const request = useRef<AbortController>();
  const saving = useRef(false);
  const changed = useRef(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      request.current?.abort();
    };
  }, []);
  // biome-ignore lint/correctness/useExhaustiveDependencies: Explicit reload/focus refreshes durable state and re-verifies current local files without searching the provider again.
  useEffect(() => {
    if (saving.current) return;
    const controller = new AbortController();
    request.current?.abort();
    request.current = controller;
    setBusy(true);
    setError("");
    void fetch("booru-browser/review/read", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      signal: controller.signal,
      body: JSON.stringify({
        source: source.id,
        instance: source.instance,
        posts: posts.map((post) => ({ id: post.id, md5: post.md5 ?? "" })),
      }),
    })
      .then(readBooruResponse<ReviewState[]>)
      .then((value) => {
        if (!controller.signal.aborted) {
          setStates(value);
          onStates(value);
        }
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          setError(cause instanceof Error ? cause.message : String(cause));
          setStates(undefined);
          onStates([]);
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setBusy(false);
      });
    return () => controller.abort();
  }, [source.id, source.instance, posts, attempt, onStates]);
  useEffect(() => {
    const refresh = () => {
      if (!saving.current) setAttempt((value) => value + 1);
    };
    window.addEventListener("focus", refresh);
    return () => window.removeEventListener("focus", refresh);
  }, []);
  const visible = posts.filter(
    (post) =>
      showIgnored || !states?.find((state) => state.id === post.id)?.ignoredAt
  );
  useEffect(() => {
    setSelected((ids) => {
      const visibleIDs = new Set(
        posts
          .filter(
            (post) =>
              showIgnored ||
              !states?.find((state) => state.id === post.id)?.ignoredAt
          )
          .map((post) => post.id)
      );
      const next = ids.filter((id) => visibleIDs.has(id));
      return next.length === ids.length ? ids : next;
    });
  }, [posts, showIgnored, states]);
  const targets = selected.filter((id) =>
    visible.some((post) => post.id === id)
  );

  async function apply(changes: ReviewChange[], isUndo = false) {
    if (busy || !states || !changes.length) return;
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    saving.current = true;
    changed.current = false;
    setBusy(true);
    setError("");
    setNotice("");
    const previous = states;
    try {
      const response = await fetch("booru-browser/review/update", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        signal: controller.signal,
        body: JSON.stringify({
          source: source.id,
          instance: source.instance,
          changes,
        }),
      });
      const value = await readBooruResponse<ReviewState[]>(response);
      if (controller.signal.aborted || !mounted.current) return;
      changed.current = true;
      const next = previous.map((state) => ({
        ...state,
        ...value.find((item) => item.id === state.id),
      }));
      setStates(next);
      onStates(next);
      setSelected([]);
      setUndo(
        isUndo
          ? undefined
          : value.map((state) => {
              const before = previous.find((item) => item.id === state.id)!;
              const change = changes.find((item) => item.id === state.id)!;
              return {
                id: state.id,
                version: state.version,
                ...(change.ignored !== undefined
                  ? { ignored: !!before.ignoredAt }
                  : {}),
                ...(change.downloaded !== undefined
                  ? { downloaded: !!before.downloadedAt }
                  : {}),
              };
            })
      );
      setNotice(
        isUndo
          ? "Review change undone."
          : `Saved review state for ${value.length} post${value.length === 1 ? "" : "s"}.`
      );
    } catch (cause: unknown) {
      if (!controller.signal.aborted && mounted.current)
        setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      saving.current = false;
      if (!controller.signal.aborted && mounted.current) {
        setBusy(false);
        if (changed.current) setAttempt((value) => value + 1);
      }
    }
  }
  function mark(
    ids: string[],
    dimension: "ignored" | "downloaded",
    value: boolean
  ) {
    if (!states) return;
    void apply(
      ids.map((id) => ({
        id,
        version: states.find((state) => state.id === id)!.version,
        [dimension]: value,
      }))
    );
  }
  return (
    <>
      <div className="booru-review-toolbar">
        <span>
          {visible.length} shown / {posts.length} provider posts ·{" "}
          {posts.length - visible.length} ignored hidden
        </span>
        <Button
          variant="secondary"
          size="sm"
          disabled={busy}
          onClick={() => setAttempt((value) => value + 1)}
        >
          Reload review status
        </Button>
        {busy && <span role="status">Checking review status…</span>}
      </div>
      {error && (
        <Alert variant="warning">
          {error} Reload review status before retrying. Review changes are
          atomic.
        </Alert>
      )}
      {notice && (
        <div role="status" className="booru-review-toolbar">
          <span>{notice}</span>
          {undo && (
            <Button
              size="sm"
              variant="secondary"
              disabled={busy || !states}
              onClick={() => void apply(undo, true)}
            >
              Undo last review change
            </Button>
          )}
        </div>
      )}
      <div className="booru-review-toolbar" aria-label="Review selection">
        <Button
          size="sm"
          variant="secondary"
          disabled={busy || !states || !visible.length}
          onClick={() => setSelected(visible.map((post) => post.id))}
        >
          Select visible
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={!targets.length}
          onClick={() => setSelected([])}
        >
          Clear selection
        </Button>
        <span>{targets.length} selected</span>
        <Button
          size="sm"
          variant="secondary"
          disabled={busy || !states || !targets.length}
          onClick={() => mark(targets, "ignored", true)}
        >
          Ignore selected
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={busy || !states || !targets.length}
          onClick={() => mark(targets, "ignored", false)}
        >
          Unignore selected
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={busy || !states || !targets.length}
          onClick={() => mark(targets, "downloaded", true)}
        >
          Mark selected downloaded
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={busy || !states || !targets.length}
          onClick={() => mark(targets, "downloaded", false)}
        >
          Clear selected manual marks
        </Button>
      </div>
      {!visible.length && posts.length > 0 && (
        <p>
          All posts on this provider page are ignored. Show ignored posts or use
          Next to continue.
        </p>
      )}
      <div className="booru-grid">
        {visible.map((post) => {
          const state = states?.find((item) => item.id === post.id);
          return children(
            post,
            <>
              <Form.Check
                id={`booru-select-${source.id}-${post.id}`}
                aria-label={`Select ${source.name} post ${post.id}`}
                label="Select"
                checked={targets.includes(post.id)}
                disabled={busy || !state}
                onChange={() =>
                  setSelected((ids) =>
                    ids.includes(post.id)
                      ? ids.filter((id) => id !== post.id)
                      : [...ids, post.id]
                  )
                }
              />
              <ReviewEvidence state={state} />
              <div className="booru-review-actions">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy || !state}
                  onClick={() => mark([post.id], "ignored", !state?.ignoredAt)}
                >
                  {state?.ignoredAt ? "Unignore" : "Ignore"}
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={busy || !state}
                  onClick={() =>
                    mark([post.id], "downloaded", !state?.downloadedAt)
                  }
                >
                  {state?.downloadedAt
                    ? "Clear manual mark"
                    : "Mark downloaded"}
                </Button>
              </div>
            </>
          );
        })}
      </div>
    </>
  );
}
