import { useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Card,
  Form,
  Modal,
  Spinner,
} from "react-bootstrap";
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
}: {
  source: BooruSource;
  query: string;
  page: number;
  inspect?: string;
  onPage: (page: number) => void;
  onInspect: (id: string) => void;
  onClose: () => void;
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
    const params = new URLSearchParams({
      source: source.id,
      q: query,
      page: String(page),
      limit: "24",
    });
    void fetch(`booru-browser/search?${params}`, { signal: request.signal })
      .then(readBooruResponse<BooruResult>)
      .then((value) => {
        if (!request.signal.aborted) setResult(value);
      })
      .catch((cause: unknown) => {
        if (!request.signal.aborted)
          setError(cause instanceof Error ? cause.message : String(cause));
      })
      .finally(() => {
        if (!request.signal.aborted) setLoading(false);
      });
    return () => request.abort();
  }, [source.id, query, page, retry]);
  const post = result?.posts.find((item) => item.id === inspect);
  return (
    <section
      className="booru-source mb-4"
      aria-label={source.name}
      aria-busy={loading}
    >
      <div className="d-flex flex-wrap align-items-center mb-2">
        <h3 className="mr-3 mb-1">{source.name}</h3>
        <Badge variant="secondary" className="mr-2">
          {source.engine}
        </Badge>
        <span>
          {message("page", "Page")} {page}
        </span>
      </div>
      <p className="small text-muted mb-1">{source.queryHelp}</p>
      <p className="small text-muted">
        {source.authentication}{" "}
        {source.credentialsConfigured &&
          message("configured", "Credentials configured.")}
      </p>
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
          <p className="small text-muted">
            <FormattedMessage
              id="booru_browser.returned_posts"
              defaultMessage="{count, plural, one {# post on this page.} other {# posts on this page.}}"
              values={{ count: result.posts.length }}
            />{" "}
            {result.total !== undefined ? (
              <>
                {message("reported_total", "Provider-reported matches:")}{" "}
                {result.total.toLocaleString()}.
              </>
            ) : (
              message("unknown_total", "Total matches unknown.")
            )}
          </p>
          {result.posts.length === 0 && (
            <Alert variant="info">
              {message("empty", "No posts returned for this page and query.")}
            </Alert>
          )}
          <div className="booru-grid">
            {result.posts.map((item) => (
              <Card key={`${source.id}:${item.id}`} className="booru-post">
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
                <Card.Body className="p-2">
                  <Button
                    variant="link"
                    className="p-0"
                    onClick={() => onInspect(item.id)}
                  >
                    #{item.id}
                  </Button>
                  <small className="d-block">
                    {item.width && item.height
                      ? `${item.width} × ${item.height}`
                      : message(
                          "unknown_dimensions",
                          "Dimensions unknown"
                        )}{" "}
                    {item.rating && ` · ${item.rating}`}
                  </small>
                  {remoteURL(item.postURL) && (
                    <ExternalLink href={remoteURL(item.postURL)}>
                      {message("open_post", "Open remote post")}
                    </ExternalLink>
                  )}
                </Card.Body>
              </Card>
            ))}
          </div>
          <div className="d-flex flex-wrap mt-3 booru-pagination">
            <Button
              variant="secondary"
              disabled={page <= 1}
              onClick={() => onPage(page - 1)}
            >
              {message("previous", "Previous")}
            </Button>
            <Button
              variant="secondary"
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
  const [query, setQuery] = useState(active.query);
  const [selected, setSelected] = useState(active.sources);
  const sourceSelection = active.sources.join(",");
  useEffect(() => {
    setQuery(active.query);
  }, [active.query]);
  useEffect(() => {
    setSelected(sourceSelection.split(","));
  }, [sourceSelection]);
  // biome-ignore lint/correctness/useExhaustiveDependencies: Reload intentionally repeats the source-list request after a configuration failure.
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
      <h2>{message("title", "Booru browser")}</h2>
      <p>
        {message(
          "intro",
          "Search remote sources and inspect their tags. Searches do not import media or change your catalogue."
        )}
      </p>
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
        <p role="status">{message("loading_sources", "Loading sources…")}</p>
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
          <fieldset>
            <legend>
              {message("sources", "Sources — choose up to eight")}
            </legend>
            <div className="booru-source-options">
              {sources.map((source) => (
                <Form.Check
                  key={source.id}
                  id={`booru-source-${source.id}`}
                  type="checkbox"
                  label={`${source.name} · ${source.engine}`}
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
          <Form.Group controlId="booru-query" className="mt-3">
            <Form.Label>
              {message("query", "Search tags (native source syntax)")}
            </Form.Label>
            <Form.Control
              value={query}
              onChange={(event) => setQuery(event.currentTarget.value)}
              maxLength={1024}
              placeholder="landscape rating:safe"
            />
            <Form.Text className="text-muted">
              {message(
                "syntax_help",
                "The same query is sent unchanged to each selected source. Search syntax, ratings and account limits differ by engine; see each source's help below."
              )}
            </Form.Text>
          </Form.Group>
          <Button
            type="submit"
            disabled={
              !query.trim() || selected.length === 0 || selected.length > 8
            }
          >
            {message("search", "Search selected sources")}
          </Button>
        </Form>
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
      {active.query.trim() && !active.error && !unknown && sources && (
        <>
          <p className="mt-4 text-muted">
            {message(
              "listing_help",
              "Results are grouped and paged separately by source. Duplicate remote posts are retained with their provenance. Live listings can change between pages; counts are not a saved snapshot."
            )}
          </p>
          {active.sources.map((id) => {
            const source = sources.find((item) => item.id === id);
            return (
              source && (
                <SourceResults
                  key={`${id}:${active.query}:${active.pages[id]}`}
                  source={source}
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
        </>
      )}
    </div>
  );
}
