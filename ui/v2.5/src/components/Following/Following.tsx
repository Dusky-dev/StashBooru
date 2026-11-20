import { useEffect, useRef, useState } from "react";
import { Alert, Badge, Button, Form } from "react-bootstrap";
import { useIntl } from "react-intl";
import { Link, useHistory, useLocation } from "react-router-dom";
import type { BooruSource } from "../BooruBrowser/browser";
import {
  type IdentityKind,
  type IdentityPage,
  type ProviderIdentity,
  identityPaths,
  identityRequest,
} from "../Identities/identityApi";
import {
  notifyFollowingChanged,
  useFollowingUnseen,
  useMarkSeenOnView,
} from "./FollowAction";
import {
  type FeedPage,
  FollowingPostGrid,
  followingRequest as request,
} from "./FollowingPosts";
import { FollowScheduleSettings } from "./FollowScheduleSettings";
import {
  canFollowIdentity,
  canSyncEngine,
  sourceForIdentity,
} from "./followContext";
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
  connection?: { identityID: string; entityID: number; entityName: string };
  unseen?: number;
}

const feedParams = [
  "follow",
  "instance",
  "kind",
  "unseen",
  "history",
  "ignored",
  "downloaded",
  "before",
];

const followKinds = [
  ["artist", "Artist"],
  ["character", "Character"],
  ["copyright", "Copyright / series"],
  ["query", "Provider query"],
];

export default function Following() {
  const intl = useIntl();
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const fromIdentity = params.get("fromIdentity") ?? "";
  // Follows and settings are the default view; the post list opens with
  // view=posts, or any feed filter from an older or shared link.
  const view = params.get("view");
  const managing =
    !!fromIdentity ||
    view === "manage" ||
    location.hash === "#following-manager" ||
    (view !== "posts" && !feedParams.some((key) => params.has(key)));
  const feedSource = params.get("instance") ?? "";
  const feedKind = params.get("kind") ?? "";
  const feedFollow = params.get("follow") ?? "";
  const showHistory = params.get("history") === "1";
  const [identity, setIdentity] = useState<ProviderIdentity>();
  const [identityError, setIdentityError] = useState("");
  const [epoch, setEpoch] = useState(0);
  const [sources, setSources] = useState<BooruSource[]>([]);
  const [sourcesLoaded, setSourcesLoaded] = useState(false);
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
  const markSeenOnView = !!useMarkSeenOnView();
  const totalUnseen = useFollowingUnseen();

  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
      operation.current?.abort();
    };
  }, []);

  useEffect(() => {
    setIdentity(undefined);
    setIdentityError("");
    if (!fromIdentity) return;
    if (!/^[1-9]\d*$/.test(fromIdentity)) {
      setIdentityError("This provider connection link is invalid.");
      return;
    }
    const controller = new AbortController();
    void identityRequest<IdentityPage>(
      `identities/list?identity=${encodeURIComponent(fromIdentity)}`,
      controller.signal
    )
      .then((value) => {
        if (controller.signal.aborted) return;
        const connection = value.items.find((row) => row.id === fromIdentity);
        if (!connection)
          throw new Error("This provider connection no longer exists.");
        if (!canFollowIdentity(connection))
          throw new Error(
            intl.formatMessage({
              id: "following_action.unsupported_connection",
            })
          );
        setIdentity(connection);
        setName(connection.entityName || connection.label || connection.target);
        setKind(connection.kind);
        setTarget(connection.target);
      })
      .catch((error: Error) => {
        if (!controller.signal.aborted) setIdentityError(error.message);
      });
    return () => controller.abort();
  }, [fromIdentity, intl]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: epoch explicitly reloads definitions and source settings after mutations.
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
              current || value.find((s) => canSyncEngine(s.engine))?.id || ""
          );
        }
      })
      .catch((error: Error) => {
        if (!controller.signal.aborted) setSourceError(error.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setSourcesLoaded(true);
      });
    return () => controller.abort();
  }, [epoch]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: epoch explicitly reloads snapshots and preview grants after mutations.
  useEffect(() => {
    if (managing) return;
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
  }, [location.search, epoch, managing]);

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
        notifyFollowingChanged();
      }
    }
  }

  function viewLink(manage: boolean) {
    const next = new URLSearchParams(location.search);
    next.delete("fromIdentity");
    next.set("view", manage ? "manage" : "posts");
    return { pathname: "/following", search: next.toString() };
  }

  function checkPosts(follow: Follow) {
    void mutate("Checking one page…", "following/sync", {
      id: follow.id,
      version: follow.version,
    });
  }

  function changeFilter(key: string, value: string) {
    const next = new URLSearchParams(location.search);
    next.delete("before");
    if (key === "instance" || key === "kind") next.delete("follow");
    if (value) next.set(key, value);
    else next.delete(key);
    next.set("view", "posts");
    history.push({ pathname: "/following", search: next.toString() });
  }

  function selectMatch(id: string, historic: boolean) {
    const next = new URLSearchParams(location.search);
    next.delete("before");
    next.delete("instance");
    next.delete("kind");
    next.set("follow", id);
    if (historic) next.set("history", "1");
    next.set("view", "posts");
    history.push({ pathname: "/following", search: next.toString() });
  }

  function editFollow(follow: Follow) {
    setEditing(follow);
    setName(follow.name);
    setKind(follow.kind);
    setTarget(follow.target);
    setSourceID(follow.resolvedSource ?? follow.source);
  }

  function resetForm() {
    setEditing(undefined);
    setName("");
    setTarget("");
    setKind("artist");
    setSourceID(sources.find((s) => canSyncEngine(s.engine))?.id ?? "");
  }

  const selectedSource = fromIdentity
    ? identity?.id === fromIdentity
      ? sourceForIdentity(sources, identity)
      : undefined
    : sources.find((source) => source.id === sourceID);
  const contextBlocked = !!fromIdentity && (!identity || !selectedSource);
  const selectedSourceID = selectedSource?.id;
  const identityID = identity?.id;
  useEffect(() => {
    if (!fromIdentity || !identityID || !selectedSourceID) return;
    document
      .getElementById("following-manager")
      ?.scrollIntoView({ block: "start" });
    document.getElementById("following-name")?.focus({ preventScroll: true });
  }, [fromIdentity, identityID, selectedSourceID]);

  function clearContext() {
    resetForm();
    if (!fromIdentity) return;
    const current = history.location;
    const next = new URLSearchParams(current.search);
    next.delete("fromIdentity");
    next.set("view", "manage");
    history.replace({ ...current, search: next.toString() });
  }

  const duplicate =
    !editing &&
    selectedSource &&
    follows.find(
      (follow) =>
        follow.instance === selectedSource.instance &&
        follow.kind === kind &&
        follow.target === target
    );
  const currentEdit = follows.find((follow) => follow.id === editing?.id);
  const editChanged = !!editing && currentEdit?.version !== editing.version;
  const locked = !!busy;
  const sourceNames = new Map<string, string>();
  for (const follow of follows) {
    const source = sources.find((row) => row.instance === follow.instance);
    sourceNames.set(follow.instance, source?.name ?? follow.sourceURL);
  }
  const selectableFollows = follows.filter(
    (follow) =>
      follow.id === feedFollow ||
      ((!feedSource || follow.instance === feedSource) &&
        (!feedKind || follow.kind === feedKind || showHistory))
  );
  const checkable = selectableFollows.filter(
    (follow) =>
      follow.active &&
      follow.syncSupported &&
      (!feedSource || follow.instance === feedSource) &&
      (!feedKind || follow.kind === feedKind)
  );
  const followToCheck = feedFollow
    ? checkable.find((follow) => follow.id === feedFollow)
    : checkable.length === 1
      ? checkable[0]
      : undefined;
  // "Mark all seen" covers the follows this view is filtered to.
  const feedScope =
    feedFollow || feedSource || feedKind
      ? selectableFollows
          .filter(
            (follow) =>
              (!feedFollow || follow.id === feedFollow) &&
              (!feedSource || follow.instance === feedSource) &&
              (!feedKind || follow.kind === feedKind)
          )
          .map((follow) => follow.id)
      : undefined;
  const next = new URLSearchParams(location.search);
  if (page?.next) next.set("before", page.next);

  return (
    <div className="following-page">
      <header>
        <h2>Following</h2>
        <nav className="following-actions" aria-label="Following views">
          <Link
            className={managing ? "btn btn-secondary" : "btn btn-primary"}
            aria-current={managing ? undefined : "page"}
            to={viewLink(false)}
            onClick={() => {
              if (fromIdentity) resetForm();
            }}
          >
            {intl.formatMessage({ id: "following_action.view_posts" })}
          </Link>
          <Link
            className={managing ? "btn btn-primary" : "btn btn-secondary"}
            aria-current={managing ? "page" : undefined}
            to={fromIdentity ? location : viewLink(true)}
          >
            {intl.formatMessage({ id: "following_action.view_manage" })}
          </Link>
          <Link className="btn btn-secondary" to="/booru">
            Content browser
          </Link>
        </nav>
      </header>
      {[listError, sourceError, !managing && feedError, actionError]
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
      {managing ? (
        <section id="following-manager" aria-label="Follow management">
          <p className="text-muted">
            {intl.formatMessage({ id: "following_action.manager_intro" })}
          </p>
          {totalUnseen > 0 && (
            <p>
              <Link
                className="btn btn-primary"
                to={{
                  pathname: "/following",
                  search: new URLSearchParams({
                    view: "posts",
                    unseen: "1",
                  }).toString(),
                }}
              >
                {intl.formatMessage(
                  { id: "following_action.navbar_new" },
                  { count: totalUnseen }
                )}
              </Link>
            </p>
          )}
          <FollowScheduleSettings
            onFinished={() => setEpoch((value) => value + 1)}
          />
          <div className="following-actions mb-3">
            <Button
              variant="secondary"
              onClick={() => setEpoch((value) => value + 1)}
            >
              Reload follows
            </Button>
            <Link className="btn btn-secondary" to="/booru/settings">
              Source settings
            </Link>
          </div>
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
                    source: selectedSource?.id,
                    instance: selectedSource?.instance,
                    name,
                    kind,
                    target,
                  };
              if (
                locked ||
                contextBlocked ||
                duplicate ||
                (!editing && !canSyncEngine(selectedSource?.engine))
              )
                return;
              if (await mutate("Saving follow…", "following/follows", input))
                clearContext();
            }}
          >
            <h3>{editing ? "Edit follow" : "Add follow"}</h3>
            {fromIdentity && (
              <Alert variant={contextBlocked ? "warning" : "info"}>
                {identityError ||
                  (identity
                    ? selectedSource
                      ? "Review this saved provider connection, then press Follow. Checking posts is a separate action."
                      : !sourcesLoaded
                        ? "Loading source settings…"
                        : "The matching source is disabled, changed or unavailable. Enable it in Source settings; no substitute was selected."
                    : "Loading provider connection…")}
                <div className="mt-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={locked}
                    onClick={clearContext}
                  >
                    Start a blank follow
                  </Button>
                </div>
              </Alert>
            )}
            {!fromIdentity &&
              !editing &&
              sourcesLoaded &&
              !sourceError &&
              !sources.some((source) => canSyncEngine(source.engine)) && (
                <Alert variant="warning">
                  {intl.formatMessage({
                    id: "following_action.no_supported_source",
                  })}
                </Alert>
              )}
            {duplicate && (
              <Alert variant="info">
                This exact target already has a saved follow.{" "}
                <a href={`#saved-follow-${duplicate.id}`}>
                  Manage saved follow
                </a>
              </Alert>
            )}
            {editChanged && (
              <Alert variant="warning">
                This follow changed elsewhere. Reload its current definition
                before saving. Your draft is kept until you reload or cancel.
              </Alert>
            )}
            <Form.Group controlId="following-source">
              <Form.Label>Source</Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={fromIdentity ? (selectedSource?.id ?? "") : sourceID}
                disabled={locked || !!editing || !!fromIdentity}
                onChange={(e) => setSourceID(e.target.value)}
              >
                {!selectedSource && !editing && (
                  <option value="">No supported source selected</option>
                )}
                {editing && !sources.some((s) => s.id === sourceID) && (
                  <option value={sourceID}>{editing.sourceURL}</option>
                )}
                {sources
                  .filter((s) => canSyncEngine(s.engine) || s.id === sourceID)
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
              </Form.Control>
              <Form.Text muted>
                {intl.formatMessage({ id: "following_action.source_help" })}
              </Form.Text>
            </Form.Group>
            <Form.Group controlId="following-name">
              <Form.Label>Follow name</Form.Label>
              <Form.Control
                required
                className="text-input"
                value={name}
                disabled={locked || contextBlocked}
                onChange={(e) => setName(e.target.value)}
              />
            </Form.Group>
            <Form.Group controlId="following-kind">
              <Form.Label>Target type</Form.Label>
              <Form.Control
                as="select"
                className="input-control"
                value={kind}
                disabled={locked || contextBlocked}
                onChange={(e) => setKind(e.target.value)}
              >
                {followKinds.map(([value, label]) => (
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
                className="text-input"
                value={target}
                disabled={locked || contextBlocked}
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
                  editChanged ||
                  !!sourceError ||
                  contextBlocked ||
                  !!duplicate ||
                  !selectedSource ||
                  (!editing && !canSyncEngine(selectedSource.engine))
                }
              >
                {editing ? "Save changes" : "Follow"}
              </Button>
              {editChanged && currentEdit && (
                <Button
                  variant="secondary"
                  disabled={locked}
                  onClick={() => editFollow(currentEdit)}
                >
                  Reload latest follow
                </Button>
              )}
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
              id={`saved-follow-${follow.id}`}
              aria-label={`Follow ${follow.name}`}
            >
              <h4>
                {follow.name}{" "}
                {!follow.active && <Badge variant="secondary">Inactive</Badge>}
                {follow.active && !!follow.unseen && (
                  <Link
                    className="badge badge-primary"
                    to={`/following?${new URLSearchParams({ view: "posts", follow: follow.id })}`}
                  >
                    {intl.formatMessage(
                      { id: "following_action.new_badge" },
                      { count: follow.unseen }
                    )}
                  </Link>
                )}
              </h4>
              <small>
                {follow.kind} · {follow.sourceURL}
              </small>
              <code>{follow.target}</code>
              {follow.connection && follow.connection.entityID > 0 && (
                <small>
                  <Link
                    to={`${identityPaths[follow.kind as IdentityKind] ?? "/studios"}/${follow.connection.entityID}`}
                  >
                    {intl.formatMessage(
                      { id: "following_action.connected_to" },
                      { name: follow.connection.entityName }
                    )}
                  </Link>
                </small>
              )}
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
                  onClick={() => checkPosts(follow)}
                >
                  {follow.feed?.hasMore ? "Continue check" : "Check new posts"}
                </Button>
                {follow.status === "available" && follow.resolvedSource && (
                  <Link
                    className="btn btn-sm btn-secondary"
                    to={`/booru?${new URLSearchParams({ source: follow.resolvedSource, q: follow.target })}`}
                  >
                    {intl.formatMessage({
                      id: "following_action.open_in_booru",
                    })}
                  </Link>
                )}
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={
                    locked || follow.status !== "available" || !!listError
                  }
                  onClick={() => {
                    if (fromIdentity) clearContext();
                    editFollow(follow);
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
          <details className="mt-3">
            <summary>Feed cache</summary>
            <p className="mt-2">
              Remove cached snapshots you marked seen. Seen decisions, follows,
              and Stash files are kept.
            </p>
            <Button
              size="sm"
              variant="secondary"
              disabled={locked}
              onClick={() =>
                void mutate("Removing seen snapshots…", "following/prune", {
                  action: "remove-seen-snapshots",
                })
              }
            >
              Remove seen snapshots
            </Button>
          </details>
        </section>
      ) : (
        <section id="following-feed" aria-label="Following feed">
          <div className="following-filters card card-body mb-3">
            <div className="following-filter-fields">
              <Form.Group controlId="following-feed-source">
                <Form.Label>Feed source</Form.Label>
                <Form.Control
                  as="select"
                  value={feedSource}
                  onChange={(e) => changeFilter("instance", e.target.value)}
                >
                  <option value="">All sources</option>
                  {!!feedSource && !sourceNames.has(feedSource) && (
                    <option value={feedSource}>Unavailable source</option>
                  )}
                  {[...sourceNames].map(([instance, label]) => (
                    <option key={instance} value={instance}>
                      {label}
                    </option>
                  ))}
                </Form.Control>
              </Form.Group>
              <Form.Group controlId="following-feed-kind">
                <Form.Label>Follow type</Form.Label>
                <Form.Control
                  as="select"
                  value={feedKind}
                  onChange={(e) => changeFilter("kind", e.target.value)}
                >
                  <option value="">All types</option>
                  {followKinds.map(([value, label]) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </Form.Control>
              </Form.Group>
              <Form.Group controlId="following-filter">
                <Form.Label>Matched follow</Form.Label>
                <Form.Control
                  as="select"
                  value={feedFollow}
                  onChange={(e) => changeFilter("follow", e.target.value)}
                >
                  <option value="">All follows</option>
                  {selectableFollows.map((follow) => (
                    <option key={follow.id} value={follow.id}>
                      {follow.name}
                    </option>
                  ))}
                </Form.Control>
              </Form.Group>
            </div>
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
              <Button
                size="sm"
                variant="secondary"
                onClick={() => setEpoch((value) => value + 1)}
              >
                Reload saved feed
              </Button>
              {followToCheck && (
                <Button
                  size="sm"
                  disabled={locked || !!listError}
                  onClick={() => checkPosts(followToCheck)}
                >
                  {followToCheck.feed?.hasMore
                    ? "Continue check for "
                    : "Check new posts for "}
                  {followToCheck.name}
                </Button>
              )}
              <span className="text-muted">
                Latest discovered · saved snapshots
              </span>
            </div>
          </div>
          {!page && !feedError && <p role="status">Loading saved feed…</p>}
          {page?.items.length === 0 && (
            <p>
              No saved posts match these filters. Adjust the filters or{" "}
              <Link to={viewLink(true)}>manage your follows</Link> to check for
              posts.
            </p>
          )}
          <FollowingPostGrid
            items={page?.items ?? []}
            sources={sources}
            scope={feedScope}
            markSeenOnView={markSeenOnView}
            disabled={locked}
            onChanged={() => setEpoch((value) => value + 1)}
            onFilter={selectMatch}
          />
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
      )}
    </div>
  );
}
