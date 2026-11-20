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

interface ArtistFollow {
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

/** Active follows whose Artist tag is connected to this Artist. */
export function useArtistFollows(studioID: string, epoch = 0) {
  const [follows, setFollows] = useState<ArtistFollow[]>([]);
  // biome-ignore lint/correctness/useExhaustiveDependencies: epoch reloads the list.
  useEffect(() => {
    const controller = new AbortController();
    const load = () =>
      void followingRequest<ArtistFollow[]>(
        "following/follows",
        controller.signal
      )
        .then((rows) => {
          if (controller.signal.aborted) return;
          setFollows(
            rows.filter(
              (row) =>
                row.active &&
                row.kind === "artist" &&
                String(row.connection?.entityID ?? "") === studioID
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
  }, [studioID, epoch]);
  return follows;
}

/**
 * An Artist's Followed posts tab: check every site this Artist is followed
 * on, and review the saved posts with an icon for each site.
 */
export function ArtistFollowedPosts({ studioID }: { studioID: string }) {
  const intl = useIntl();
  const m = (id: string, values?: Record<string, string | number>) =>
    intl.formatMessage({ id: `following_action.artist.${id}` }, values);
  const [epoch, setEpoch] = useState(0);
  const follows = useArtistFollows(studioID, epoch);
  const markSeenOnView = useMarkSeenOnView();
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

  const siteName = (follow: ArtistFollow) =>
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
    const rows = await followingRequest<ArtistFollow[]>(
      "following/follows",
      c.signal
    ).catch(() => [] as ArtistFollow[]);
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

  if (!follows.length) return null;
  const items = page?.items ?? [];
  return (
    <section className="artist-followed-posts" aria-label={m("tab")}>
      <div className="following-actions mb-2">
        <span>{m("intro", { count: follows.length })}</span>
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
