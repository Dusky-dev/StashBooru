import { useEffect, useRef, useState } from "react";
import { Alert, Badge, Button, Form } from "react-bootstrap";
import { Link, useHistory, useLocation } from "react-router-dom";
import type { BooruSource } from "../BooruBrowser/browser";
import "./styles.scss";

interface Follow {
  id: string;
  version: number;
  queryRevision: number;
  name: string;
  kind: string;
  target: string;
  source: string;
  sourceURL: string;
  instance: string;
  active: boolean;
  status: string;
  resolvedSource?: string;
  syncSupported: boolean;
  feed?: { hasMore: boolean; checkedAt: string; lastError: string };
}

interface FeedItem {
  sequence: string;
  postID: string;
  retrievedAt: string;
  seenAt: string;
  seenVersion: number;
  ignoredAt: string;
  downloadedAt: string;
  matchesTruncated: boolean;
  matches: {
    followID: string;
    name: string;
    revision: number;
    target: string;
    current: boolean;
  }[];
  content: {
    title: string;
    url?: string;
    thumbnail?: string;
    browseURL?: string;
    rating?: string;
    tags?: { name: string; category: string }[];
  };
}

interface FeedPage {
  items: FeedItem[];
  next: string;
}

async function request<T>(path: string, signal: AbortSignal, input?: unknown) {
  const response = await fetch(new URL(path, document.baseURI), {
    signal,
    credentials: "same-origin",
    ...(input === undefined
      ? {}
      : {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(input),
        }),
  });
  const value = await response.json();
  if (!response.ok) {
    const retry = response.headers.get("Retry-After");
    throw new Error(
      (typeof value.error === "string" ? value.error : "Request failed.") +
        (retry ? ` Retry after ${retry} seconds.` : "")
    );
  }
  return value as T;
}

function Snapshot({
  item,
  disabled,
  seen,
  filter,
}: {
  item: FeedItem;
  disabled: boolean;
  seen: () => void;
  filter: (id: string, historic: boolean) => void;
}) {
  const [failed, setFailed] = useState(false);
  const content = item.content;
  useEffect(() => setFailed(false), [content.thumbnail]);
  return (
    <article className="following-post card" aria-label={content.title}>
      <div className="following-thumbnail">
        {content.thumbnail && !failed ? (
          <img
            src={new URL(content.thumbnail, document.baseURI).toString()}
            alt=""
            loading="lazy"
            onError={() => setFailed(true)}
          />
        ) : (
          <span>{failed ? "Preview unavailable" : "No cached preview"}</span>
        )}
      </div>
      <div className="card-body">
        <h3>
          {content.url ? (
            <a href={content.url} target="_blank" rel="noreferrer">
              {content.title}
            </a>
          ) : (
            content.title
          )}
        </h3>
        <div className="following-actions">
          {content.rating && (
            <Badge variant="secondary">{content.rating}</Badge>
          )}
          {item.ignoredAt && <Badge variant="secondary">Ignored</Badge>}
          {item.downloadedAt && (
            <Badge variant="secondary">Marked downloaded</Badge>
          )}
          <Button
            size="sm"
            variant={item.seenAt ? "outline-secondary" : "secondary"}
            disabled={disabled}
            onClick={seen}
          >
            {item.seenAt ? "Mark unseen" : "Mark seen"}
          </Button>
          {content.browseURL && (
            <Link className="btn btn-secondary btn-sm" to={content.browseURL}>
              Browse / Download
            </Link>
          )}
        </div>
        <div className="following-matches">
          Matched follows:
          {item.matches.map((match) => (
            <Button
              key={`${match.followID}:${match.revision}`}
              variant="link"
              size="sm"
              title={match.target}
              onClick={() => filter(match.followID, !match.current)}
            >
              {match.name}
              {!match.current && " (history)"}
            </Button>
          ))}
          {item.matchesTruncated && (
            <small>Additional matches omitted (50 shown).</small>
          )}
        </div>
        {!!content.tags?.length && (
          <details>
            <summary>Tags ({content.tags.length})</summary>
            <div className="following-tags">
              {content.tags.map((tag) => (
                <span key={`${tag.category}:${tag.name}`}>
                  {tag.name} <small>({tag.category})</small>
                </span>
              ))}
            </div>
          </details>
        )}
        <small className="text-muted">
          Saved snapshot ·{" "}
          <time dateTime={item.retrievedAt}>
            {new Date(item.retrievedAt).toLocaleString()}
          </time>
        </small>
      </div>
    </article>
  );
}

export default function Following() {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const [epoch, setEpoch] = useState(0);
  const [sources, setSources] = useState<BooruSource[]>([]);
  const [follows, setFollows] = useState<Follow[]>([]);
  const [page, setPage] = useState<FeedPage>();
  const [listError, setListError] = useState("");
  const [sourceError, setSourceError] = useState("");
  const [feedError, setFeedError] = useState("");
  const [actionError, setActionError] = useState("");
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<Follow>();
  const [name, setName] = useState("");
  const [kind, setKind] = useState("artist");
  const [target, setTarget] = useState("");
  const [sourceID, setSourceID] = useState("");
  const [busy, setBusy] = useState<{
    label: string;
    controller: AbortController;
  }>();
  const active = useRef(true);
  const operation = useRef<AbortController>();

  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
      operation.current?.abort();
    };
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    request<Follow[]>("following/follows", controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) {
          setFollows(value);
          setListError("");
        }
      })
      .catch((error: Error) => {
        if (!controller.signal.aborted) setListError(error.message);
      });
    request<BooruSource[]>("booru-browser/sources", controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) {
          setSources(value);
          setSourceError("");
          setSourceID(
            (current) =>
              current || value.find((s) => s.engine === "danbooru")?.id || ""
          );
        }
      })
      .catch((error: Error) => {
        if (!controller.signal.aborted) setSourceError(error.message);
      });
    return () => controller.abort();
  }, [epoch]);

  useEffect(() => {
    const controller = new AbortController();
    setPage(undefined);
    setFeedError("");
    request<FeedPage>(`following/feed${location.search}`, controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) setPage(value);
      })
      .catch((error: Error) => {
        if (!controller.signal.aborted) setFeedError(error.message);
      });
    return () => controller.abort();
  }, [location.search, epoch]);

  async function mutate(label: string, path: string, input: unknown) {
    if (operation.current) return false;
    const controller = new AbortController();
    operation.current = controller;
    setBusy({ label, controller });
    setActionError("");
    setMessage("");
    try {
      const result = await request<{ notice?: string; removed?: number }>(
        path,
        controller.signal,
        input
      );
      if (!active.current || controller.signal.aborted) return false;
      setMessage(
        result.notice ??
          (result.removed === undefined
            ? "Saved."
            : `Removed ${result.removed} seen snapshots. Seen decisions and Stash files are retained.`)
      );
      return true;
    } catch (error) {
      if (active.current)
        setActionError(
          controller.signal.aborted
            ? "Request cancelled. Reloaded state includes any page that completed before cancellation."
            : (error as Error).message
        );
      return false;
    } finally {
      operation.current = undefined;
      if (active.current) {
        setBusy(undefined);
        setEpoch((value) => value + 1);
      }
    }
  }

  function changeFilter(key: string, value: string) {
    const next = new URLSearchParams(location.search);
    next.delete("before");
    if (value) next.set(key, value);
    else next.delete(key);
    history.push({ pathname: "/following", search: next.toString() });
  }

  function selectMatch(id: string, historic: boolean) {
    const next = new URLSearchParams(location.search);
    next.delete("before");
    next.set("follow", id);
    if (historic) next.set("history", "1");
    history.push({ pathname: "/following", search: next.toString() });
  }

  function resetForm() {
    setEditing(undefined);
    setName("");
    setTarget("");
    setKind("artist");
    setSourceID(sources.find((s) => s.engine === "danbooru")?.id ?? "");
  }

  const locked = !!busy;
  const next = new URLSearchParams(location.search);
  if (page?.next) next.set("before", page.next);

  return (
    <div className="following-page">
      <header>
        <h2>Following</h2>
        <div className="following-actions">
          <a className="btn btn-secondary" href="#following-manager">
            Manage follows
          </a>
          <Link className="btn btn-secondary" to="/booru">
            Content browser
          </Link>
          <Link className="btn btn-secondary" to="/booru/settings">
            Source settings
          </Link>
          <Button onClick={() => setEpoch((value) => value + 1)}>
            Reload saved feed
          </Button>
        </div>
      </header>
      <p className="text-muted">
        Check follows on demand. Your first check saves up to 50 recent posts;
        later checks collect newer uploads. Feed checks currently support
        Danbooru-compatible sources.
      </p>
      {[listError, sourceError, feedError, actionError]
        .filter(Boolean)
        .map((error, i) => (
          <Alert key={`${i}:${error}`} variant="danger">
            {error}
          </Alert>
        ))}
      {message && <Alert variant="info">{message}</Alert>}
      {busy && (
        <Alert variant="info" role="status">
          {busy.label}{" "}
          <Button size="sm" onClick={() => busy.controller.abort()}>
            Cancel request
          </Button>
        </Alert>
      )}
      <div className="following-layout">
        <aside id="following-manager">
          <a className="btn btn-link d-md-none" href="#following-feed">
            Back to feed
          </a>
          <Form
            className="card card-body mb-3"
            onSubmit={async (event) => {
              event.preventDefault();
              const input = editing
                ? {
                    action: "update",
                    id: editing.id,
                    version: editing.version,
                    name,
                    kind,
                    target,
                  }
                : {
                    action: "create",
                    source: sourceID,
                    instance: sources.find((s) => s.id === sourceID)?.instance,
                    name,
                    kind,
                    target,
                  };
              if (await mutate("Saving follow…", "following/follows", input))
                resetForm();
            }}
          >
            <h3>{editing ? "Edit follow" : "Add follow"}</h3>
            <Form.Group controlId="following-source">
              <Form.Label>Source</Form.Label>
              <Form.Control
                as="select"
                value={sourceID}
                disabled={locked || !!editing}
                onChange={(e) => setSourceID(e.target.value)}
              >
                {editing && !sources.some((s) => s.id === sourceID) && (
                  <option value={sourceID}>{editing.sourceURL}</option>
                )}
                {sources
                  .filter((s) => s.engine === "danbooru" || s.id === sourceID)
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
              </Form.Control>
            </Form.Group>
            <Form.Group controlId="following-name">
              <Form.Label>Follow name</Form.Label>
              <Form.Control
                required
                value={name}
                disabled={locked}
                onChange={(e) => setName(e.target.value)}
              />
            </Form.Group>
            <Form.Group controlId="following-kind">
              <Form.Label>Target type</Form.Label>
              <Form.Control
                as="select"
                value={kind}
                disabled={locked}
                onChange={(e) => setKind(e.target.value)}
              >
                {[
                  ["artist", "Artist"],
                  ["character", "Character"],
                  ["copyright", "Copyright / series"],
                  ["query", "Provider query"],
                ].map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
            <Form.Group controlId="following-target">
              <Form.Label>Exact provider tag or query</Form.Label>
              <Form.Control
                required
                value={target}
                disabled={locked}
                onChange={(e) => setTarget(e.target.value)}
              />
              <Form.Text muted>
                Use an exact tag or query, including series constraints and
                exclusions. Omit sorting and page controls.
                {editing &&
                  " Changing a target starts a new recent window; previous matches remain in History."}
                {kind === "query" &&
                  " Changes on older posts do not resurface automatically."}
              </Form.Text>
            </Form.Group>
            <div className="following-actions">
              <Button
                type="submit"
                disabled={
                  locked ||
                  !!sourceError ||
                  !sources.some((s) => s.id === sourceID)
                }
              >
                {editing ? "Save changes" : "Follow"}
              </Button>
              {editing && (
                <Button
                  variant="secondary"
                  disabled={locked}
                  onClick={resetForm}
                >
                  Cancel edit
                </Button>
              )}
            </div>
          </Form>
          <h3>Saved follows ({follows.length})</h3>
          {follows.length === 0 && (
            <p>Add a follow to start collecting posts.</p>
          )}
          {follows.map((follow) => (
            <section
              className="card card-body mb-2"
              key={follow.id}
              aria-label={`Follow ${follow.name}`}
            >
              <h4>
                {follow.name}{" "}
                {!follow.active && <Badge variant="secondary">Inactive</Badge>}
              </h4>
              <small>
                {follow.kind} · {follow.sourceURL}
              </small>
              <code>{follow.target}</code>
              <small>
                {follow.status !== "available"
                  ? follow.status
                  : !follow.syncSupported
                    ? "Feed checks not yet supported"
                    : follow.feed?.checkedAt
                      ? `Last check: ${new Date(follow.feed.checkedAt).toLocaleString()}`
                      : "Not checked yet"}
              </small>
              {follow.feed?.lastError && (
                <small className="text-danger" role="status">
                  Last check failed: {follow.feed.lastError}
                </small>
              )}
              <div className="following-actions mt-2">
                <Button
                  size="sm"
                  disabled={
                    locked ||
                    !follow.active ||
                    !follow.syncSupported ||
                    !!listError
                  }
                  onClick={() =>
                    void mutate("Checking one page…", "following/sync", {
                      id: follow.id,
                      version: follow.version,
                    })
                  }
                >
                  {follow.feed?.hasMore ? "Continue check" : "Check new posts"}
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={
                    locked || follow.status !== "available" || !!listError
                  }
                  onClick={() => {
                    setEditing(follow);
                    setName(follow.name);
                    setKind(follow.kind);
                    setTarget(follow.target);
                    setSourceID(follow.resolvedSource ?? follow.source);
                  }}
                >
                  Edit
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={
                    locked ||
                    !!listError ||
                    (!follow.active && follow.status !== "available")
                  }
                  onClick={() =>
                    void mutate("Saving follow state…", "following/follows", {
                      action: follow.active ? "unfollow" : "refollow",
                      id: follow.id,
                      version: follow.version,
                    })
                  }
                >
                  {follow.active ? "Unfollow" : "Refollow"}
                </Button>
              </div>
            </section>
          ))}
        </aside>
        <section id="following-feed" aria-label="Following feed">
          <div className="following-filters card card-body mb-3">
            <Form.Group controlId="following-filter">
              <Form.Label>Matched follow</Form.Label>
              <Form.Control
                as="select"
                value={params.get("follow") ?? ""}
                onChange={(e) => changeFilter("follow", e.target.value)}
              >
                <option value="">All follows</option>
                {follows.map((f) => (
                  <option key={f.id} value={f.id}>
                    {f.name}
                  </option>
                ))}
              </Form.Control>
            </Form.Group>
            <div className="following-actions">
              {[
                ["unseen", "Unseen only", "1"],
                ["history", "Include History", "1"],
                ["ignored", "Show ignored", "1"],
                ["downloaded", "Hide marked downloaded", "hide"],
              ].map(([key, label, value]) => (
                <Form.Check
                  key={key}
                  id={`following-${key}`}
                  type="switch"
                  label={label}
                  checked={params.get(key) === value}
                  onChange={(e) =>
                    changeFilter(key, e.target.checked ? value : "")
                  }
                />
              ))}
            </div>
            <div className="following-actions mt-2">
              <span>Latest discovered · saved snapshots</span>
              <Button
                size="sm"
                variant="secondary"
                disabled={locked}
                title="Remove seen snapshots from the feed cache; retain seen decisions and Stash files."
                onClick={() =>
                  void mutate("Removing seen snapshots…", "following/prune", {
                    action: "remove-seen-snapshots",
                  })
                }
              >
                Remove seen snapshots
              </Button>
            </div>
          </div>
          {!page && !feedError && <p role="status">Loading saved feed…</p>}
          {page?.items.length === 0 && (
            <p>
              No saved posts match these filters. Check a follow, continue a
              pending check, or adjust the filters.
            </p>
          )}
          <div className="following-grid">
            {page?.items.map((item) => (
              <Snapshot
                key={item.sequence}
                item={item}
                disabled={locked}
                filter={selectMatch}
                seen={() =>
                  void mutate("Saving seen decision…", "following/seen", {
                    sequence: item.sequence,
                    version: item.seenVersion,
                    seen: !item.seenAt,
                  })
                }
              />
            ))}
          </div>
          <nav aria-label="Feed pages" className="following-actions my-3">
            {params.has("before") && (
              <Button
                variant="secondary"
                onClick={() => changeFilter("before", "")}
              >
                Newest saved
              </Button>
            )}
            {page?.next && (
              <Link
                className="btn btn-secondary"
                to={{ pathname: "/following", search: next.toString() }}
              >
                Older saved
              </Link>
            )}
          </nav>
        </section>
      </div>
    </div>
  );
}
