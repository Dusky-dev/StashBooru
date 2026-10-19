import {
  type ReactNode,
  type CSSProperties,
  useEffect,
  useRef,
  useState,
} from "react";
import { Alert, Button, Form } from "react-bootstrap";
import Gallery from "react-photo-gallery";
import { Link } from "react-router-dom";
import {
  faBan,
  faCheck,
  faRotateRight,
  faXmark,
  faCheckDouble,
  faArrowUpRightFromSquare,
} from "@fortawesome/free-solid-svg-icons";
import { BooruIconButton } from "./BooruControls";
import { Icon } from "../Shared/Icon";
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
    id: string;
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

export function ReviewEvidence({
  state,
  compact = false,
}: {
  state?: ReviewState;
  compact?: boolean;
}) {
  if (compact)
    return (
      <div className="booru-review-evidence booru-evidence-compact">
        {state?.ignoredAt && (
          <span
            role="img"
            aria-label="Ignored"
            title={`Ignored ${state.ignoredAt}`}
          >
            <Icon icon={faBan} />
          </span>
        )}
        {state?.downloadedAt && (
          <span
            role="img"
            aria-label="Marked downloaded · manual"
            title={`Marked downloaded · manual ${state.downloadedAt}`}
          >
            <Icon icon={faCheck} />
          </span>
        )}
        {state?.matches?.map((match) => (
          <Link
            key={`${match.kind}:${match.id}:${match.fileID}`}
            to={`/${match.kind === "image" ? "images" : "scenes"}/${match.id}`}
            aria-label={`In library · ${match.kind === "image" ? "Image" : "Video"} #${match.id}`}
            title={`Current-file MD5 verified · ${match.kind} #${match.id}`}
          >
            <Icon icon={faArrowUpRightFromSquare} />
          </Link>
        ))}
      </div>
    );
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
  preview,
  displayMode = "grid",
  zoom = 0,
}: {
  source: BooruSource;
  displayMode?: "grid" | "wall";
  zoom?: number;
  posts: BooruPost[];
  showIgnored: boolean;
  onStates: (states: ReviewState[]) => void;
  children: (
    post: BooruPost,
    controls: ReactNode,
    selected: boolean
  ) => ReactNode;
  preview?: (actions: {
    ignore: (id: string) => void;
    busy: boolean;
  }) => ReactNode;
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
    if (busy || error || !states || !changes.length) return;
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
  const renderPost = (post: BooruPost) => {
    const state = states?.find((item) => item.id === post.id);
    return children(
      post,
      <>
        <Form.Control
          type="checkbox"
          className="card-check mousetrap"
          aria-label={`Select ${source.name} post ${post.id}`}
          checked={targets.includes(post.id)}
          disabled={busy || !!error || !state}
          onChange={() =>
            setSelected((ids) =>
              ids.includes(post.id)
                ? ids.filter((id) => id !== post.id)
                : [...ids, post.id]
            )
          }
        />
        <div className="booru-review-actions">
          <BooruIconButton
            icon={faBan}
            label={state?.ignoredAt ? "Unignore" : "Ignore"}
            aria-pressed={!!state?.ignoredAt}
            disabled={busy || !!error || !state}
            onClick={() => mark([post.id], "ignored", !state?.ignoredAt)}
          />
          <BooruIconButton
            icon={faCheck}
            label={
              state?.downloadedAt ? "Clear manual mark" : "Mark downloaded"
            }
            aria-pressed={!!state?.downloadedAt}
            disabled={busy || !!error || !state}
            onClick={() => mark([post.id], "downloaded", !state?.downloadedAt)}
          />
          <ReviewEvidence state={state} compact />
        </div>
      </>,
      targets.includes(post.id)
    );
  };
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
              disabled={busy || !!error || !states}
              onClick={() => void apply(undo, true)}
            >
              Undo last review change
            </Button>
          )}
        </div>
      )}
      <div className="booru-review-toolbar" aria-label="Review selection">
        {targets.length > 0 ? (
          <>
            <div className="selected-items-info">
              <BooruIconButton
                icon={faXmark}
                label="Clear selection"
                onClick={() => setSelected([])}
              />
              <span className="selected-count" aria-live="polite">
                {targets.length} selected
              </span>
              <BooruIconButton
                icon={faCheckDouble}
                label="Select visible"
                disabled={busy || !!error || !states}
                onClick={() => setSelected(visible.map((post) => post.id))}
              />
            </div>
            <BooruIconButton
              icon={faBan}
              label="Ignore selected"
              disabled={busy || !!error || !states}
              onClick={() => mark(targets, "ignored", true)}
            />
            <BooruIconButton
              icon={faRotateRight}
              label="Unignore selected"
              disabled={busy || !!error || !states}
              onClick={() => mark(targets, "ignored", false)}
            />
            <BooruIconButton
              icon={faCheck}
              label="Mark selected downloaded"
              disabled={busy || !!error || !states}
              onClick={() => mark(targets, "downloaded", true)}
            />
            <BooruIconButton
              icon={faXmark}
              label="Clear selected manual marks"
              disabled={busy || !!error || !states}
              onClick={() => mark(targets, "downloaded", false)}
            />
          </>
        ) : (
          <BooruIconButton
            icon={faCheckDouble}
            label="Select visible"
            disabled={busy || !!error || !states || !visible.length}
            onClick={() => setSelected(visible.map((post) => post.id))}
          />
        )}
      </div>
      {!visible.length && posts.length > 0 && (
        <p>
          All posts on this provider page are ignored. Show ignored posts or use
          Next to continue.
        </p>
      )}
      <div
        className={displayMode === "wall" ? "booru-wall" : "booru-grid"}
        style={
          displayMode === "grid"
            ? ({
                gridTemplateColumns: `repeat(auto-fill, minmax(min(100%, ${[140, 180, 240, 320][zoom]}px), 1fr))`,
                "--booru-thumbnail-height": `${[180, 240, 310, 390][zoom]}px`,
              } as CSSProperties)
            : undefined
        }
      >
        {displayMode === "wall" && visible.length ? (
          <Gallery
            photos={visible.map((post) => ({
              key: post.id,
              src: post.previewURL ?? "",
              width:
                Math.min(
                  8,
                  Math.max(0.125, (post.width || 1) / (post.height || 1))
                ) * 1000,
              height: 1000,
            }))}
            direction="row"
            margin={2}
            targetRowHeight={[140, 190, 260, 340][zoom]}
            renderImage={({ photo, margin }) => (
              <div
                key={photo.key}
                className="booru-wall-item"
                style={{ width: photo.width, height: photo.height, margin }}
              >
                {renderPost(visible.find((post) => post.id === photo.key)!)}
              </div>
            )}
          />
        ) : (
          visible.map(renderPost)
        )}
      </div>
      {preview?.({
        ignore: (id) => mark([id], "ignored", true),
        busy: busy || !!error || !states,
      })}
    </>
  );
}
