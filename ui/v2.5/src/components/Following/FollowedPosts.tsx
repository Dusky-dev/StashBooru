import { useEffect, useRef, useState } from "react";
import { Alert, Button, Form } from "react-bootstrap";
import { useIntl } from "react-intl";
import { Link } from "react-router-dom";
import type { BooruSource } from "../BooruBrowser/browser";
import { SourceIcon } from "../BooruBrowser/SourceIcon";
import { notifyFollowingChanged, useMarkSeenOnView } from "./FollowAction";
import {
  type FeedPage,
  FollowingPostGrid,
  followingRequest,
} from "./FollowingPosts";

interface EntityFollow {
  id: string;
  version: number;
  name: string;
  kind: string;
  instance: string;
  sourceURL: string;
  active: boolean;
  syncSupported: boolean;
  connection?: { entityID: number };
  unseen?: number;
}

// One check stops after this many pages per site; the next press continues.
const maxPagesPerSite = 10;

export type FollowedKind = "artist" | "character" | "copyright";

/** Active follows whose site name is connected to this Artist, Character
 * or Copyright. */
export function useEntityFollows(
  kind: FollowedKind,
  entityID: string,
  epoch = 0
) {
  // undefined until loaded, so the tab doesn't flash its empty state.
  const [follows, setFollows] = useState<EntityFollow[]>();
  // biome-ignore lint/correctness/useExhaustiveDependencies: epoch reloads the list.
  useEffect(() => {
    const controller = new AbortController();
    const load = () =>
      void followingRequest<EntityFollow[]>(
        "following/follows",
        controller.signal
      )
        .then((rows) => {
          if (controller.signal.aborted) return;
          setFollows(
            rows.filter(
              (row) =>
                row.active &&
                row.kind === kind &&
                String(row.connection?.entityID ?? "") === entityID
            )
          );
        })
        .catch(() => {});
    load();
    window.addEventListener("stashbooru-following-changed", load);
    return () => {
      controller.abort();
      window.removeEventListener("stashbooru-following-changed", load);
    };
  }, [kind, entityID, epoch]);
  return follows;
}

/** New posts across this record's follows, for its tab counter. */
export function useFollowedUnseen(kind: FollowedKind, entityID: string) {
  return (useEntityFollows(kind, entityID) ?? []).reduce(
    (count, follow) => count + (follow.unseen ?? 0),
    0
  );
}

/**
 * A profile's Followed posts tab: check every site the Artist, Character or
 * Copyright is followed on, and review the saved posts with an icon for each
 * site. Without follows it explains how to add one.
 */
export function FollowedPosts({
  kind,
  entityID,
}: {
  kind: FollowedKind;
  entityID: string;
}) {
  const intl = useIntl();
  const m = (id: string, values?: Record<string, string | number>) =>
    intl.formatMessage({ id: `following_action.followed.${id}` }, values);
  const [epoch, setEpoch] = useState(0);
  const loaded = useEntityFollows(kind, entityID, epoch);
  const follows = loaded ?? [];
  const markSeenOnView = !!useMarkSeenOnView();
  const [sources, setSources] = useState<BooruSource[]>([]);
  const [newOnly, setNewOnly] = useState(true);
  const [before, setBefore] = useState("");
  const [page, setPage] = useState<FeedPage>();
  const [error, setError] = useState("");
  const [checking, setChecking] = useState("");
  const [messages, setMessages] = useState<string[]>([]);
  const controller = useRef<AbortController>();

  useEffect(() => () => controller.current?.abort(), []);

  useEffect(() => {
    const c = new AbortController();
    void followingRequest<BooruSource[]>("booru-browser/sources", c.signal)
      .then((value) => {
        if (!c.signal.aborted) setSources(value);
      })
      .catch(() => {});
    return () => c.abort();
  }, []);

  const ids = follows.map((follow) => follow.id).join(",");
  // biome-ignore lint/correctness/useExhaustiveDependencies: epoch reloads the posts.
  useEffect(() => {
    if (!ids) return;
    const c = new AbortController();
    const query = new URLSearchParams();
    for (const id of ids.split(",")) query.append("follow", id);
    if (newOnly) query.set("unseen", "1");
    if (before) query.set("before", before);
    setError("");
    void followingRequest<FeedPage>(`following/feed?${query}`, c.signal)
      .then((value) => {
        if (!c.signal.aborted) setPage(value);
      })
      .catch((e: Error) => {
        if (!c.signal.aborted) setError(e.message);
      });
    return () => c.abort();
  }, [ids, newOnly, before, epoch]);

  const siteName = (follow: EntityFollow) =>
    sources.find((s) => s.instance === follow.instance)?.name ??
    follow.sourceURL;

  async function check() {
    const c = new AbortController();
    controller.current = c;
    const lines: string[] = [];
    const unseenBefore = follows.reduce((n, f) => n + (f.unseen ?? 0), 0);
    setMessages([]);
    for (const follow of follows.filter((f) => f.syncSupported)) {
      setChecking(siteName(follow));
      try {
        for (let i = 0; i < maxPagesPerSite; i++) {
          const result = await followingRequest<{
            checkpoint: { hasMore: boolean };
          }>("following/sync", c.signal, {
            id: follow.id,
            version: follow.version,
          });
          if (!result.checkpoint?.hasMore) break;
        }
      } catch (e) {
        if (c.signal.aborted) return;
        lines.push(
          m("failed", { site: siteName(follow), error: (e as Error).message })
        );
      }
    }
    setChecking("");
    // Count what the check added from the refreshed follow list.
    const rows = await followingRequest<EntityFollow[]>(
      "following/follows",
      c.signal
    ).catch(() => [] as EntityFollow[]);
    const unseenAfter = rows
      .filter((row) => follows.some((f) => f.id === row.id))
      .reduce((n, f) => n + (f.unseen ?? 0), 0);
    lines.unshift(
      m("checked", { count: Math.max(0, unseenAfter - unseenBefore) })
    );
    setMessages(lines);
    setBefore("");
    setEpoch((value) => value + 1);
    notifyFollowingChanged();
  }

  if (!loaded) return null;
  if (!follows.length)
    return (
      <section className="artist-followed-posts" aria-label={m("tab")}>
        <p>{m("none", { kind })}</p>
        <Link
          className="btn btn-secondary btn-sm"
          to={`/identities?${new URLSearchParams({ kind })}`}
        >
          {m("open_identities")}
        </Link>
      </section>
    );
  const items = page?.items ?? [];
  return (
    <section className="artist-followed-posts" aria-label={m("tab")}>
      <div className="following-actions mb-2">
        <span>{m("intro", { count: follows.length, kind })}</span>
        {follows.map((follow) => {
          const source = sources.find((s) => s.instance === follow.instance);
          return (
            <span key={follow.id} className="following-site-chip">
              {source && <SourceIcon source={source} />} {siteName(follow)}
            </span>
          );
        })}
      </div>
      <div className="following-actions mb-2">
        <Button disabled={!!checking} onClick={() => void check()}>
          {m("check")}
        </Button>
        <Form.Check
          type="switch"
          id="artist-followed-new-only"
          label={m("new_only")}
          checked={newOnly}
          onChange={(e) => {
            setNewOnly(e.currentTarget.checked);
            setBefore("");
          }}
        />
        <Link className="btn btn-secondary btn-sm" to="/following">
          {m("manage")}
        </Link>
      </div>
      {checking && (
        <Alert variant="info" role="status">
          {m("checking", { site: checking })}
        </Alert>
      )}
      {messages.length > 0 && (
        <Alert variant="info" role="status">
          {messages.map((line) => (
            <div key={line}>{line}</div>
          ))}
        </Alert>
      )}
      {error && <Alert variant="danger">{error}</Alert>}
      {page && !items.length && (
        <p className="text-muted">{m(newOnly ? "empty_new" : "empty")}</p>
      )}
      <FollowingPostGrid
        items={items}
        sources={sources}
        scope={follows.map((follow) => follow.id)}
        markSeenOnView={markSeenOnView}
        disabled={!!checking}
        onChanged={() => setEpoch((value) => value + 1)}
      />
      <nav className="following-actions my-3">
        {before && (
          <Button variant="secondary" onClick={() => setBefore("")}>
            {m("newest")}
          </Button>
        )}
        {page?.next && (
          <Button variant="secondary" onClick={() => setBefore(page.next)}>
            {m("older")}
          </Button>
        )}
      </nav>
    </section>
  );
}
