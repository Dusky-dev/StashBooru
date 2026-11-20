import { useEffect, useRef, useState } from "react";
import { Alert, Badge, Button, Form } from "react-bootstrap";
import { useIntl } from "react-intl";
import { Link } from "react-router-dom";
import type { BooruSource } from "../BooruBrowser/browser";
import { SourceIcon } from "../BooruBrowser/SourceIcon";
import { notifyFollowingChanged } from "./FollowAction";
import "./styles.scss";

export interface FeedItem {
  sequence: string;
  postID: string;
  instance: string;
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

export interface FeedPage {
  items: FeedItem[];
  next: string;
}

export async function followingRequest<T>(
  path: string,
  signal: AbortSignal,
  input?: unknown
) {
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

function PostCard({
  item,
  source,
  sourceName,
  isNew,
  seen,
  selected,
  disabled,
  onSelect,
  onSeen,
  onFilter,
}: {
  item: FeedItem;
  source?: BooruSource;
  sourceName?: string;
  isNew: boolean;
  seen: boolean;
  selected: boolean;
  disabled: boolean;
  onSelect: (selected: boolean) => void;
  onSeen: () => void;
  onFilter?: (id: string, historic: boolean) => void;
}) {
  const intl = useIntl();
  const m = (id: string, values?: Record<string, string | number>) =>
    intl.formatMessage({ id: `following_action.posts.${id}` }, values);
  const [failedThumbnail, setFailedThumbnail] = useState<string>();
  const content = item.content;
  const failed = !!content.thumbnail && failedThumbnail === content.thumbnail;
  return (
    <article
      className={`following-post card${selected ? " selected" : ""}${isNew ? " unseen" : ""}`}
      aria-label={content.title}
    >
      <div className="following-thumbnail">
        <Form.Check
          className="following-select"
          id={`following-select-${item.sequence}`}
          aria-label={m("select", { title: content.title })}
          checked={selected}
          onChange={(e) => onSelect(e.currentTarget.checked)}
        />
        {content.thumbnail && !failed ? (
          <img
            src={new URL(content.thumbnail, document.baseURI).toString()}
            alt=""
            loading="lazy"
            referrerPolicy="no-referrer"
            onError={() => setFailedThumbnail(content.thumbnail)}
          />
        ) : (
          <span>{m(failed ? "preview_failed" : "no_preview")}</span>
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
          {isNew && <Badge variant="primary">{m("new")}</Badge>}
          {sourceName && (
            <Badge variant="secondary" className="following-site">
              {source && <SourceIcon source={source} />}
              {sourceName}
            </Badge>
          )}
          {content.rating && (
            <Badge variant="secondary">{content.rating}</Badge>
          )}
          {item.ignoredAt && <Badge variant="secondary">{m("ignored")}</Badge>}
          {item.downloadedAt && (
            <Badge variant="secondary">{m("downloaded")}</Badge>
          )}
          <Button
            size="sm"
            variant={seen ? "outline-secondary" : "secondary"}
            disabled={disabled}
            onClick={onSeen}
          >
            {m(seen ? "mark_unseen" : "mark_seen")}
          </Button>
          {content.browseURL && (
            <Link
              className="btn btn-secondary btn-sm"
              to={content.browseURL}
              target="_blank"
              rel="noopener noreferrer"
              title={m("browse_title")}
            >
              {m("browse")}
            </Link>
          )}
        </div>
        {onFilter && (
          <div className="following-matches">
            {m("matched")}
            {item.matches.map((match) => (
              <Button
                key={`${match.followID}:${match.revision}`}
                variant="link"
                size="sm"
                title={match.target}
                onClick={() => onFilter(match.followID, !match.current)}
              >
                {match.name}
                {!match.current && ` (${m("history")})`}
              </Button>
            ))}
            {item.matchesTruncated && <small>{m("matches_truncated")}</small>}
          </div>
        )}
        {!!content.tags?.length && (
          <details>
            <summary>{m("tags", { count: content.tags.length })}</summary>
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
          <time dateTime={item.retrievedAt}>
            {intl.formatDate(item.retrievedAt, {
              dateStyle: "medium",
              timeStyle: "short",
            })}
          </time>
        </small>
      </div>
    </article>
  );
}

/**
 * Saved posts from follows, with selection and seen actions. Used by the
 * Following page (all follows) and an Artist's Followed posts tab.
 *
 * scope: the follows "Mark all seen" applies to (undefined = every follow).
 * markSeenOnView: posts are marked seen once shown; they stay highlighted as
 * new until the list reloads.
 */
export function FollowingPostGrid({
  items,
  sources,
  scope,
  markSeenOnView,
  disabled,
  onChanged,
  onFilter,
}: {
  items: FeedItem[];
  sources: BooruSource[];
  scope?: string[];
  markSeenOnView: boolean;
  disabled?: boolean;
  onChanged: () => void;
  onFilter?: (id: string, historic: boolean) => void;
}) {
  const intl = useIntl();
  const m = (id: string, values?: Record<string, string | number>) =>
    intl.formatMessage({ id: `following_action.posts.${id}` }, values);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const viewed = useRef(new Set<string>());
  // Posts explicitly marked unseen are not marked seen again on view.
  const kept = useRef(new Set<string>());

  // A new page starts with nothing selected.
  const shown = items.map((item) => item.sequence).join(",");
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset when the shown posts change.
  useEffect(() => {
    setSelected((current) => (current.size ? new Set() : current));
  }, [shown]);

  // Mark shown posts seen without reloading, so they keep their New badge
  // during this visit.
  useEffect(() => {
    if (!markSeenOnView) return;
    const sequences = items
      .filter(
        (item) =>
          !item.seenAt &&
          !viewed.current.has(item.sequence) &&
          !kept.current.has(item.sequence)
      )
      .map((item) => item.sequence);
    if (!sequences.length) return;
    for (const sequence of sequences) viewed.current.add(sequence);
    const controller = new AbortController();
    void followingRequest("following/seen-batch", controller.signal, {
      sequences,
      seen: true,
    })
      .then(() => notifyFollowingChanged())
      .catch(() => {
        for (const sequence of sequences) viewed.current.delete(sequence);
      });
  }, [items, markSeenOnView]);

  async function mark(input: Record<string, unknown>) {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const result = await followingRequest<{ changed: number }>(
        "following/seen-batch",
        new AbortController().signal,
        input
      );
      setMessage(m("updated", { count: result.changed }));
      notifyFollowingChanged();
      onChanged();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  // Marked seen on view: seen on the server, not yet in this copy.
  const isSeen = (item: FeedItem) =>
    !!item.seenAt || viewed.current.has(item.sequence);
  const sourceByInstance = new Map(sources.map((s) => [s.instance, s]));
  const chosen = items.filter((item) => selected.has(item.sequence));
  const newest = Math.max(0, ...items.map((item) => Number(item.sequence)));
  const locked = disabled || busy;
  return (
    <div className="following-posts">
      {items.length > 0 && (
        <div className="following-actions following-selection mb-2">
          <Button
            size="sm"
            variant="secondary"
            disabled={locked}
            onClick={() =>
              setSelected(
                chosen.length === items.length
                  ? new Set()
                  : new Set(items.map((item) => item.sequence))
              )
            }
          >
            {m(
              chosen.length === items.length ? "clear_selection" : "select_page"
            )}
          </Button>
          {chosen.length > 0 && (
            <>
              <span>{m("selected", { count: chosen.length })}</span>
              <Button
                size="sm"
                disabled={locked}
                onClick={() =>
                  void mark({
                    sequences: chosen.map((item) => item.sequence),
                    seen: true,
                  })
                }
              >
                {m("mark_selected_seen")}
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={locked}
                onClick={() => {
                  for (const item of chosen) {
                    viewed.current.delete(item.sequence);
                    kept.current.add(item.sequence);
                  }
                  void mark({
                    sequences: chosen.map((item) => item.sequence),
                    seen: false,
                  });
                }}
              >
                {m("mark_selected_unseen")}
              </Button>
            </>
          )}
          <Button
            size="sm"
            variant="secondary"
            className="ml-auto"
            disabled={locked || (!!scope && !scope.length)}
            title={m("mark_all_title")}
            onClick={() =>
              void mark({
                upTo: String(newest),
                seen: true,
                ...(scope ? { followIDs: scope } : {}),
              })
            }
          >
            {m("mark_all_seen")}
          </Button>
        </div>
      )}
      {error && <Alert variant="danger">{error}</Alert>}
      {message && (
        <Alert variant="info" role="status">
          {message}
        </Alert>
      )}
      <div className="following-grid">
        {items.map((item) => {
          const source = sourceByInstance.get(item.instance);
          return (
            <PostCard
              key={item.sequence}
              item={item}
              source={source}
              sourceName={source?.name}
              isNew={!item.seenAt || viewed.current.has(item.sequence)}
              seen={isSeen(item)}
              selected={selected.has(item.sequence)}
              disabled={locked}
              onSelect={(value) =>
                setSelected((current) => {
                  const next = new Set(current);
                  if (value) next.add(item.sequence);
                  else next.delete(item.sequence);
                  return next;
                })
              }
              onSeen={() => {
                // An explicit choice ends the "shown this visit" highlight.
                const seen = !isSeen(item);
                viewed.current.delete(item.sequence);
                if (seen) kept.current.delete(item.sequence);
                else kept.current.add(item.sequence);
                void mark({ sequences: [item.sequence], seen });
              }}
              onFilter={onFilter}
            />
          );
        })}
      </div>
    </div>
  );
}
