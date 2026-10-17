import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Alert, Badge, Button, Form, Modal, Spinner } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import { useHistory, useLocation } from "react-router-dom";
import { ExternalLink } from "../Shared/ExternalLink";
import {
  type BooruPost,
  type BooruResult,
  type BooruSource,
  browserSearchURL,
  readBooruResponse,
  readBrowserSearch,
  remoteURL,
} from "./browser";
import { SourceSettings } from "./SourceSettings";
import "./styles.scss";

const message = (id: string, defaultMessage: string) => (
  <FormattedMessage
    id={`booru_browser.${id}`}
    defaultMessage={defaultMessage}
  />
);

function PostInspection({
  post,
  source,
  onHide,
}: {
  post: BooruPost;
  source: BooruSource;
  onHide: () => void;
}) {
  const groups = [...new Set(post.tags.map((tag) => tag.category))];
  const sample = remoteURL(post.sampleURL);
  return (
    <Modal show onHide={onHide} size="lg" className="booru-browser-inspection">
      <Modal.Header closeButton>
        <Modal.Title>
          {source.name} #{post.id}
        </Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <p>
          {message(
            "remote_preview",
            "Remote preview — this browser does not yet check whether the post is already in your library."
          )}
        </p>
        {sample ? (
          <RemoteImage
            key={sample}
            className="booru-sample"
            src={sample}
            alt={`${source.name} #${post.id}`}
          />
        ) : (
          <Alert variant="secondary">
            {message(
              "no_preview",
              "No supported preview is available. Open the remote post to inspect it."
            )}
          </Alert>
        )}
        <p className="mt-2">
          {post.width && post.height ? `${post.width} × ${post.height} · ` : ""}
          {post.bytes ? `${post.bytes.toLocaleString()} B · ` : ""}
          {post.format} {post.rating && ` · Rating: ${post.rating}`}
        </p>
        {post.md5 && (
          <p className="booru-hash">
            MD5: <code>{post.md5}</code>
          </p>
        )}
        {groups.length === 0 && (
          <p>{message("no_tags", "The provider supplied no tags.")}</p>
        )}
        {groups.map((category) => (
          <section key={category} className="mb-3">
            <h6>
              {category === "unknown"
                ? message("unknown_category", "Tags — category not supplied")
                : category}
            </h6>
            <div className="booru-tags">
              {post.tags
                .filter((tag) => tag.category === category)
                .map((tag) => (
                  <Badge key={`${category}:${tag.name}`} variant="secondary">
                    {tag.name}
                  </Badge>
                ))}
            </div>
          </section>
        ))}
      </Modal.Body>
      <Modal.Footer>
        {remoteURL(post.postURL) && (
          <ExternalLink
            className="btn btn-primary"
            href={remoteURL(post.postURL)}
          >
            {message("open_post", "Open remote post")}
          </ExternalLink>
        )}
        <Button variant="secondary" onClick={onHide}>
          {message("close", "Close")}
        </Button>
      </Modal.Footer>
    </Modal>
  );
}

function RemoteImage({
  src,
  alt,
  className,
}: {
  src: string;
  alt: string;
  className?: string;
}) {
  const [failed, setFailed] = useState(false);
  return failed ? (
    <span>
      {message(
        "preview_failed",
        "Remote preview could not be loaded. Use Open remote post."
      )}
    </span>
  ) : (
    <img
      src={src}
      alt={alt}
      className={className}
      loading="lazy"
      referrerPolicy="no-referrer"
      onError={() => setFailed(true)}
    />
  );
}

function SourceResults({
  source,
  query,
  page,
  inspect,
  onPage,
  onInspect,
  onClose,
  onResult,
}: {
  source: BooruSource;
  query: string;
  page: number;
  inspect?: string;
  onPage: (page: number) => void;
  onInspect: (id: string) => void;
  onClose: () => void;
  onResult: (id: string, signature: string, posts: BooruPost[]) => void;
}) {
  const [result, setResult] = useState<BooruResult>();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [retry, setRetry] = useState(0);
  const controller = useRef<AbortController>();
  // biome-ignore lint/correctness/useExhaustiveDependencies: Retry intentionally repeats the same read after a source failure.
  useEffect(() => {
    const request = new AbortController();
    controller.current = request;
    setResult(undefined);
    setError("");
    setLoading(true);
    onResult(source.id, `${query}:${page}`, []);
    const params = new URLSearchParams({
      source: source.id,
      q: query,
      page: String(page),
      limit: "24",
    });
    void fetch(`booru-browser/search?${params}`, { signal: request.signal })
      .then(readBooruResponse<BooruResult>)
      .then((value) => {
        if (!request.signal.aborted) {
          setResult(value);
          onResult(source.id, `${query}:${page}`, value.posts);
        }
      })
      .catch((cause: unknown) => {
        if (!request.signal.aborted)
          setError(cause instanceof Error ? cause.message : String(cause));
      })
      .finally(() => {
        if (!request.signal.aborted) setLoading(false);
      });
    return () => request.abort();
  }, [source.id, query, page, retry, onResult]);
  const post = result?.posts.find((item) => item.id === inspect);
  return (
    <section
      className="booru-source"
      aria-label={source.name}
      aria-busy={loading}
    >
      <header className="booru-results-header">
        <h3>{source.name}</h3>
        <span className="text-muted">
          {source.engine} · {message("page", "Page")} {page}
        </span>
        {result && (
          <span className="text-muted">
            {result.posts.length} posts
            {result.total !== undefined
              ? ` / ${result.total.toLocaleString()} matches`
              : " · total unknown"}
          </span>
        )}
        {remoteURL(source.url) && (
          <ExternalLink className="ml-auto" href={source.url}>
            Visit source ↗
          </ExternalLink>
        )}
      </header>
      {loading && (
        <p role="status">
          <Spinner animation="border" size="sm" className="mr-2" />
          {message("searching", "Searching…")}{" "}
          <Button
            variant="secondary"
            size="sm"
            onClick={() => {
              controller.current?.abort();
              setLoading(false);
              setError("Search cancelled.");
            }}
          >
            {message("cancel", "Cancel")}
          </Button>
        </p>
      )}
      {error && (
        <Alert variant="warning">
          <span>{error}</span>{" "}
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setRetry((value) => value + 1)}
          >
            {message("retry", "Retry source")}
          </Button>
        </Alert>
      )}
      {result && (
        <>
          {result.posts.length === 0 && (
            <Alert variant="info">
              {message("empty", "No posts returned for this page and query.")}
            </Alert>
          )}
          <div className="booru-grid">
            {result.posts.map((item) => (
              <article key={`${source.id}:${item.id}`} className="booru-post">
                <Button
                  variant="link"
                  className="booru-preview-button"
                  onClick={() => onInspect(item.id)}
                  aria-label={`Inspect ${source.name} post ${item.id}`}
                >
                  {remoteURL(item.previewURL) ? (
                    <RemoteImage
                      key={item.previewURL}
                      src={remoteURL(item.previewURL) || ""}
                      alt={`${source.name} #${item.id}`}
                    />
                  ) : (
                    <span>
                      {message("missing_thumbnail", "Preview unavailable")}
                    </span>
                  )}
                </Button>
                <div className="booru-post-caption">
                  <Button
                    variant="link"
                    className="p-0"
                    onClick={() => onInspect(item.id)}
                  >
                    #{item.id}
                  </Button>
                  <small>
                    {item.width && item.height
                      ? `${item.width} × ${item.height}`
                      : message(
                          "unknown_dimensions",
                          "Dimensions unknown"
                        )}{" "}
                    {item.rating && ` · ${item.rating}`}
                  </small>
                </div>
              </article>
            ))}
          </div>
          <div className="d-flex flex-wrap mt-3 booru-pagination">
            <Button
              variant="secondary"
              size="sm"
              disabled={page <= 1}
              onClick={() => onPage(page - 1)}
            >
              {message("previous", "Previous")}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={!result.hasMore || page >= 750}
              onClick={() => onPage(page + 1)}
            >
              {message("next", "Next")}
            </Button>
          </div>
          {page === 750 && (
            <Alert variant="info" className="mt-2">
              {message(
                "page_limit",
                "Numbered browsing ends at page 750. Narrow the query to continue."
              )}
            </Alert>
          )}
          {inspect && !post && (
            <Alert variant="info">
              {message(
                "post_moved",
                "The selected post is no longer on this page. The remote listing may have changed."
              )}{" "}
              <Button size="sm" variant="secondary" onClick={onClose}>
                {message("close", "Close")}
              </Button>
            </Alert>
          )}
        </>
      )}
      {post && <PostInspection post={post} source={source} onHide={onClose} />}
    </section>
  );
}

export default function BooruBrowser() {
  const location = useLocation();
  const history = useHistory();
  const active = useMemo(
    () => readBrowserSearch(location.search),
    [location.search]
  );
  const [sources, setSources] = useState<BooruSource[]>();
  const [sourceError, setSourceError] = useState("");
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [showSettings, setShowSettings] = useState(false);
  const [tagSource, setTagSource] = useState("");
  const [tagPages, setTagPages] = useState<
    Record<string, { signature: string; posts: BooruPost[] }>
  >({});
  const onResult = useCallback(
    (id: string, signature: string, posts: BooruPost[]) => {
      setTagPages((previous) => ({ ...previous, [id]: { signature, posts } }));
    },
    []
  );
  const [query, setQuery] = useState(active.query);
  const [selected, setSelected] = useState(active.sources);
  const sourceSelection = active.sources.join(",");
  useEffect(() => {
    setQuery(active.query);
  }, [active.query]);
  useEffect(() => {
    setSelected(sourceSelection.split(","));
  }, [sourceSelection]);
  // biome-ignore lint/correctness/useExhaustiveDependencies: Reload intentionally repeats the source-list request after a configuration failure or settings save.
  useEffect(() => {
    const request = new AbortController();
    setSourceError("");
    void fetch("booru-browser/sources", { signal: request.signal })
      .then(readBooruResponse<BooruSource[]>)
      .then((value) => {
        if (!request.signal.aborted) {
          setSources(value);
          setSelected((current) =>
            current.filter((id) => value.some((source) => source.id === id))
          );
        }
      })
      .catch((cause: unknown) => {
        if (!request.signal.aborted)
          setSourceError(
            cause instanceof Error ? cause.message : String(cause)
          );
      });
    return () => request.abort();
  }, [loadAttempt]);
  const unknown =
    sources &&
    active.sources.some((id) => !sources.some((source) => source.id === id));
  return (
    <div className="booru-browser">
      <header className="booru-heading">
        <div>
          <h2>{message("title", "Booru browser")}</h2>
          <span className="text-muted">Remote posts · Browse and inspect</span>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => setShowSettings((value) => !value)}
        >
          {showSettings ? "Back to posts" : "Source settings"}
        </Button>
      </header>
      {showSettings && (
        <section aria-label="Source settings" className="booru-settings-panel">
          <h3>Source settings</h3>
          <SourceSettings
            onSaved={() => setLoadAttempt((value) => value + 1)}
          />
        </section>
      )}
      <div className="booru-layout" hidden={showSettings}>
        <aside className="booru-sidebar" aria-label="Search and tags">
          {sourceError && (
            <Alert variant="danger">
              <span>{sourceError}</span>{" "}
              <Button
                variant="secondary"
                onClick={() => setLoadAttempt((value) => value + 1)}
              >
                {message("retry", "Retry source")}
              </Button>
            </Alert>
          )}
          {!sources && !sourceError && (
            <p role="status">
              {message("loading_sources", "Loading sources…")}
            </p>
          )}
          {sources && (
            <Form
              onSubmit={(event) => {
                event.preventDefault();
                history.push({
                  pathname: "/booru",
                  search: browserSearchURL({
                    sources: selected,
                    query: query.trim(),
                    pages: {},
                  }),
                });
              }}
            >
              <Form.Group controlId="booru-query" className="mb-2">
                <Form.Label>Search</Form.Label>
                <Form.Control
                  aria-label="Search tags (native source syntax)"
                  value={query}
                  onChange={(event) => setQuery(event.currentTarget.value)}
                  maxLength={1024}
                  placeholder="Search tags…"
                />
              </Form.Group>
              <Button
                type="submit"
                size="sm"
                className="w-100 mb-3"
                aria-label="Search selected sources"
                disabled={
                  !query.trim() || selected.length === 0 || selected.length > 8
                }
              >
                Search
              </Button>
              <fieldset>
                <legend>
                  {message("sources", "Sources")}{" "}
                  <small className="text-muted">{selected.length}/8</small>
                </legend>
                <div className="booru-source-options">
                  {sources.map((source) => (
                    <Form.Check
                      key={source.id}
                      id={`booru-source-${source.id}`}
                      type="checkbox"
                      label={
                        <span>
                          {source.name}
                          <small className="text-muted d-block">
                            {source.engine}
                            {source.credentialsConfigured ? " · key saved" : ""}
                          </small>
                        </span>
                      }
                      aria-label={`${source.name} · ${source.engine}`}
                      checked={selected.includes(source.id)}
                      disabled={
                        !selected.includes(source.id) && selected.length >= 8
                      }
                      onChange={() =>
                        setSelected((value) =>
                          value.includes(source.id)
                            ? value.filter((id) => id !== source.id)
                            : [...value, source.id]
                        )
                      }
                    />
                  ))}
                </div>
              </fieldset>
              <details className="booru-help">
                <summary>Search help</summary>
                <p>
                  The same query is sent unchanged to selected sources. Syntax
                  and account limits vary.
                </p>
                {sources
                  .filter((source) => selected.includes(source.id))
                  .map((source) => (
                    <p key={source.id}>
                      <strong>{source.name}</strong>
                      <br />
                      {source.queryHelp}
                      <br />
                      {source.authentication}
                    </p>
                  ))}
              </details>
            </Form>
          )}
          <PageTags
            sources={[
              ...(sources?.filter((source) =>
                active.sources.includes(source.id)
              ) ?? []),
            ].sort(
              (a, b) =>
                active.sources.indexOf(a.id) - active.sources.indexOf(b.id)
            )}
            selected={tagSource}
            onSource={setTagSource}
            pages={tagPages}
            query={active.query}
            activePages={active.pages}
            onTag={setQuery}
          />
          <p className="small text-muted booru-library-note">
            Remote listings. Local download status is not checked.
          </p>
        </aside>
        <section className="booru-results" aria-label="Remote posts">
          {(!active.query.trim() || !sources) && !sourceError && (
            <div className="booru-empty">
              <h3>Discover posts</h3>
              <p>Choose your sources, enter tags, and search.</p>
              <p className="small text-muted">
                API keys and custom sites are available in Source settings.
              </p>
            </div>
          )}
          {(active.error || unknown) && (
            <Alert className="mt-3" variant="warning" role="alert">
              {active.error ||
                message(
                  "unknown_source",
                  "This search includes a source that is no longer enabled. Choose sources and search again."
                )}
            </Alert>
          )}
          {active.query.trim() &&
            !active.error &&
            !unknown &&
            sources &&
            active.sources.map((id) => {
              const source = sources.find((item) => item.id === id);
              return (
                source && (
                  <SourceResults
                    key={`${id}:${active.query}:${active.pages[id]}:${loadAttempt}`}
                    source={source}
                    onResult={onResult}
                    query={active.query}
                    page={active.pages[id]}
                    inspect={
                      active.inspect?.source === id
                        ? active.inspect.id
                        : undefined
                    }
                    onPage={(page) =>
                      history.push({
                        search: browserSearchURL({
                          ...active,
                          pages: { ...active.pages, [id]: page },
                          inspect: undefined,
                        }),
                      })
                    }
                    onInspect={(postID) =>
                      history.push({
                        search: browserSearchURL({
                          ...active,
                          inspect: { source: id, id: postID },
                        }),
                      })
                    }
                    onClose={() =>
                      history.replace({
                        search: browserSearchURL({
                          ...active,
                          inspect: undefined,
                        }),
                      })
                    }
                  />
                )
              );
            })}
        </section>
      </div>
    </div>
  );
}

function PageTags({
  sources,
  selected,
  onSource,
  pages,
  query,
  activePages,
  onTag,
}: {
  sources: BooruSource[];
  selected: string;
  onSource: (id: string) => void;
  pages: Record<string, { signature: string; posts: BooruPost[] }>;
  query: string;
  activePages: Record<string, number>;
  onTag: (name: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const source = sources.find((item) => item.id === selected) ?? sources[0];
  const loaded = source ? pages[source.id] : undefined;
  const posts =
    source && loaded?.signature === `${query}:${activePages[source.id]}`
      ? loaded.posts
      : [];
  const counts = new Map<
    string,
    { name: string; category: string; count: number }
  >();
  for (const post of posts)
    for (const tag of post.tags) {
      const key = `${tag.category}:${tag.name}`;
      const old = counts.get(key);
      counts.set(key, { ...tag, count: (old?.count ?? 0) + 1 });
    }
  const categories = [
    "artist",
    "copyright",
    "character",
    "species",
    "general",
    "meta",
    "lore",
    "unknown",
  ];
  const groups = [
    ...new Set([
      ...categories,
      ...[...counts.values()].map((tag) => tag.category),
    ]),
  ];
  return (
    <section className="booru-page-tags" aria-label="Tags on this page">
      <h4>Tags on this page</h4>
      <Button
        variant="secondary"
        size="sm"
        className="booru-tags-toggle"
        aria-expanded={expanded}
        aria-controls="booru-page-tag-body"
        onClick={() => setExpanded((value) => !value)}
      >
        {expanded ? "Hide page tags" : "Show page tags"}
      </Button>
      <div
        id="booru-page-tag-body"
        className={`booru-tag-body ${expanded ? "is-expanded" : ""}`}
      >
        {sources.length > 1 && (
          <Form.Control
            as="select"
            size="sm"
            aria-label="Tag source"
            value={source?.id}
            onChange={(e) => onSource(e.currentTarget.value)}
          >
            {sources.map((item) => (
              <option value={item.id} key={item.id}>
                {item.name}
              </option>
            ))}
          </Form.Control>
        )}
        <p className="small text-muted">
          Select a tag to prepare a new search. Counts cover this loaded page
          only.
        </p>
        {posts.length === 0 && (
          <p className="small text-muted">No loaded tags from this source.</p>
        )}
        {groups.map((category) => {
          const tags = [...counts.values()]
            .filter((tag) => tag.category === category)
            .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
            .slice(0, 20);
          return (
            tags.length > 0 && (
              <div
                key={category}
                className={`booru-tag-category tag-${categories.includes(category) ? category : "unknown"}`}
              >
                <h5>{category === "unknown" ? "Uncategorized" : category}</h5>
                <ul>
                  {tags.map((tag) => (
                    <li key={tag.name}>
                      <Button
                        variant="link"
                        size="sm"
                        title={`Use tag ${tag.name}`}
                        onClick={() => onTag(tag.name)}
                      >
                        {tag.name.replace(/_/g, " ")}
                      </Button>
                      <span className="text-muted">{tag.count}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )
          );
        })}
      </div>
    </section>
  );
}
