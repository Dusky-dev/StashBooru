import {
  type ReactNode,
  type KeyboardEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Alert, Badge, Button, Form, Modal, Spinner } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import { useHistory, useLocation } from "react-router-dom";
import {
  faArrowLeft,
  faArrowRight,
  faBan,
  faGear,
  faSort,
  faShieldHalved,
  faImages,
  faTags,
  faPlay,
  faStop,
} from "@fortawesome/free-solid-svg-icons";
import { Icon } from "../Shared/Icon";
import { PageSizeSelector } from "../List/ListFilter";
import { BooruIconButton, BooruIconMenu } from "./BooruControls";
import { motionPreviewURL } from "./browser";
import { ExternalLink } from "../Shared/ExternalLink";
import {
  type BooruPost,
  type BooruResult,
  type BooruSource,
  type BrowserSearch,
  postMediaKind,
  browserResultSignature,
  browserSearchURL,
  readBooruResponse,
  readBrowserSearch,
  remoteURL,
  booruMediaURL,
} from "./browser";
import { BooruReview, ReviewEvidence, type ReviewState } from "./BooruReview";
import { TagAutocomplete } from "./TagAutocomplete";
import { BooruDownload } from "./BooruDownload";
import { readBrowserMemory, saveBrowserMemory } from "./browser";
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
  review,
  onRefresh,
  onPrevious,
  onNext,
  onIgnore,
  reviewBusy,
}: {
  post: BooruPost;
  source: BooruSource;
  onHide: () => void;
  review?: ReviewState;
  onRefresh: () => void;
  onPrevious?: () => void;
  onNext?: () => void;
  onIgnore: () => void;
  reviewBusy: boolean;
}) {
  const groups = [...new Set(post.tags.map((tag) => tag.category))];
  const sample = booruMediaURL(post, "sample");
  const download = useRef<HTMLButtonElement>(null);
  const ignore = useRef<HTMLButtonElement>(null);
  return (
    <Modal
      show
      onHide={onHide}
      size="lg"
      className="booru-browser-inspection"
      onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => {
        const target = event.target as HTMLElement;
        if (
          event.altKey ||
          event.ctrlKey ||
          event.metaKey ||
          event.shiftKey ||
          !event.currentTarget.contains(target) ||
          target.closest(
            'input, textarea, select, video, audio, [contenteditable="true"], [role="combobox"], [role="menu"]'
          )
        )
          return;
        if (
          !["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(
            event.key
          )
        )
          return;
        event.preventDefault();
        event.stopPropagation();
        if (event.key === "ArrowLeft") onPrevious?.();
        if (event.key === "ArrowRight") onNext?.();
        if (!event.repeat && event.key === "ArrowUp") download.current?.click();
        if (!event.repeat && event.key === "ArrowDown") ignore.current?.click();
      }}
    >
      <Modal.Header closeButton>
        <Modal.Title>
          {source.name} #{post.id}
        </Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <nav
          className="booru-preview-navigation"
          aria-label="Preview navigation"
        >
          <BooruIconButton
            icon={faArrowLeft}
            label="Previous preview (Left arrow)"
            disabled={!onPrevious}
            onClick={onPrevious}
          />
          <span className="small text-muted">
            ← / → Browse · ↑ Download · ↓ Ignore
          </span>
          <BooruIconButton
            icon={faArrowRight}
            label="Next preview (Right arrow)"
            disabled={!onNext}
            onClick={onNext}
          />
        </nav>
        <ReviewEvidence state={review} />
        <MotionPreview key={`motion:${post.id}:${post.mediaKey}`} post={post}>
          {sample ? (
            <RemoteImage
              key={sample}
              className="booru-sample"
              src={sample}
              fallbackSrc={remoteURL(post.directSampleURL)}
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
        </MotionPreview>
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
        <BooruDownload
          key={`download:${post.id}:${post.mediaKey}`}
          downloadRef={download}
          post={post}
          instance={source.instance}
        />
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
        <Button
          ref={ignore}
          variant="secondary"
          className="minimal booru-icon-button"
          aria-label="Ignore post (Down arrow)"
          title="Ignore post (Down arrow)"
          disabled={reviewBusy || !!review?.ignoredAt}
          onClick={onIgnore}
        >
          <Icon icon={faBan} />
        </Button>
        <Button variant="secondary" onClick={onRefresh}>
          Refresh previews
        </Button>
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

function MotionPreview({
  post,
  children,
}: {
  post: BooruPost;
  children: ReactNode;
}) {
  const [playing, setPlaying] = useState(false);
  const [failed, setFailed] = useState(false);
  const video = useRef<HTMLVideoElement>(null);
  const motion = motionPreviewURL(post);
  useEffect(() => {
    if (!playing) return;
    const player = video.current;
    return () => {
      if (player) {
        player.pause();
        player.removeAttribute("src");
        player.load();
      }
    };
  }, [playing]);
  if (!motion) return <>{children}</>;
  return (
    <section className="booru-motion-preview" aria-label="Animated preview">
      <Button
        variant="secondary"
        size="sm"
        onClick={() => {
          setPlaying((value) => !value);
          setFailed(false);
        }}
      >
        <Icon icon={playing ? faStop : faPlay} />{" "}
        {playing ? "Stop animated preview" : "Play animated preview"}
      </Button>
      {playing &&
        (failed ? (
          <Alert variant="warning">
            Animated preview could not be played. Stop and retry, download the
            file, or open the remote post.
          </Alert>
        ) : ["gif", "apng"].includes(post.format?.toLowerCase() ?? "") ? (
          <img
            src={motion}
            className="booru-sample"
            alt={`Animated ${post.sourceID} #${post.id}`}
            onError={() => setFailed(true)}
          />
        ) : (
          <video
            ref={video}
            className="booru-sample"
            src={motion}
            controls
            autoPlay
            muted
            loop
            playsInline
            preload="none"
            aria-label={`Video preview ${post.sourceID} #${post.id}`}
            onError={() => setFailed(true)}
          />
        ))}
      {!playing && (
        <>
          <small className="d-block text-muted">
            On-demand preview · up to 24 MiB · video starts muted
          </small>
          {children}
        </>
      )}
    </section>
  );
}

function RemoteImage({
  src,
  alt,
  className,
  fallbackSrc,
}: {
  src: string;
  alt: string;
  className?: string;
  fallbackSrc?: string;
}) {
  const [direct, setDirect] = useState(false);
  const [failed, setFailed] = useState(false);
  const [reason, setReason] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    if (failed)
      void fetch(src, { signal: controller.signal })
        .then(async (response) => {
          if (!response.ok) await readBooruResponse(response);
        })
        .catch((cause: Error) => {
          if (!controller.signal.aborted) setReason(cause.message);
        });
    return () => controller.abort();
  }, [failed, src]);
  return failed ? (
    <span>
      Remote preview could not be loaded.{" "}
      {reason || "Use Refresh previews or open the remote post."}
    </span>
  ) : (
    <img
      src={direct ? fallbackSrc : src}
      alt={alt}
      className={className}
      loading="lazy"
      referrerPolicy="no-referrer"
      onError={() => {
        if (!direct && fallbackSrc) setDirect(true);
        else setFailed(true);
      }}
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
  showIgnored,
  filters,
  thumbnails,
}: {
  source: BooruSource;
  query: string;
  showIgnored: boolean;
  filters: Pick<BrowserSearch, "sort" | "rating" | "media" | "pageSize">;
  thumbnails: boolean;
  page: number;
  inspect?: string;
  onPage: (page: number) => void;
  onInspect: (id: string) => void;
  onClose: () => void;
  onResult: (id: string, signature: string, posts: BooruPost[]) => void;
}) {
  const [result, setResult] = useState<BooruResult>();
  const visiblePosts = useMemo(
    () =>
      result?.posts.filter(
        (post) => !filters.media || postMediaKind(post) === filters.media
      ) ?? [],
    [result, filters.media]
  );
  const signature = browserResultSignature(
    query,
    page,
    filters.sort,
    filters.rating,
    filters.media,
    filters.pageSize
  );
  useEffect(
    () => onResult(source.id, signature, visiblePosts),
    [onResult, source.id, signature, visiblePosts]
  );
  const [review, setReview] = useState<ReviewState[]>([]);
  const onReview = useCallback((value: ReviewState[]) => setReview(value), []);
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
      limit: String(filters.pageSize ?? 24),
      ...(filters.sort ? { sort: filters.sort } : {}),
      ...(filters.rating ? { rating: filters.rating } : {}),
    });
    void fetch(`booru-browser/search?${params}`, { signal: request.signal })
      .then(readBooruResponse<BooruResult>)
      .then((value) => {
        if (!request.signal.aborted) {
          setResult(value);
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
  }, [
    source.id,
    query,
    page,
    retry,
    filters.sort,
    filters.rating,
    filters.pageSize,
  ]);
  const post = result?.posts.find((item) => item.id === inspect);
  const navigation = visiblePosts.filter(
    (item) =>
      item.id === inspect ||
      showIgnored ||
      !review.find((state) => state.id === item.id)?.ignoredAt
  );
  const previewIndex = navigation.findIndex((item) => item.id === inspect);
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
          {filters.media && (
            <p className="small text-muted">
              Showing {visiblePosts.length} of {result.posts.length} posts on
              this loaded page. Provider totals and pagination include all
              media.
            </p>
          )}
          <BooruReview
            source={source}
            posts={visiblePosts}
            showIgnored={showIgnored}
            onStates={onReview}
            preview={({ ignore, busy }) =>
              post && (
                <PostInspection
                  post={post}
                  source={source}
                  onHide={onClose}
                  review={review.find((state) => state.id === post.id)}
                  reviewBusy={busy}
                  onIgnore={() => ignore(post.id)}
                  onPrevious={
                    previewIndex > 0
                      ? () => onInspect(navigation[previewIndex - 1].id)
                      : undefined
                  }
                  onNext={
                    previewIndex >= 0 && previewIndex < navigation.length - 1
                      ? () => onInspect(navigation[previewIndex + 1].id)
                      : undefined
                  }
                  onRefresh={() => setRetry((value) => value + 1)}
                />
              )
            }
          >
            {(item, controls, isSelected) => (
              <article
                key={`${source.id}:${item.id}`}
                className={`booru-post ${isSelected ? "selected" : ""}`}
              >
                <Button
                  variant="link"
                  className="booru-preview-button"
                  onClick={() => onInspect(item.id)}
                  aria-label={`Inspect ${source.name} post ${item.id}`}
                >
                  {item.rating && (
                    <span
                      className={`booru-rating rating-${["s", "safe", "g", "general"].includes(item.rating) ? "safe" : ["q", "questionable", "sensitive"].includes(item.rating) ? "questionable" : "explicit"}`}
                    >
                      {item.rating}
                    </span>
                  )}
                  {!thumbnails ? (
                    <span>Thumbnails hidden</span>
                  ) : booruMediaURL(item, "preview") ? (
                    <RemoteImage
                      key={item.previewURL}
                      src={booruMediaURL(item, "preview") || ""}
                      fallbackSrc={remoteURL(item.directPreviewURL)}
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
                </div>
                <div className="booru-card-actions">
                  <BooruDownload
                    post={item}
                    instance={source.instance}
                    compact
                  />
                  {controls}
                </div>
              </article>
            )}
          </BooruReview>
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
    </section>
  );
}

export default function BooruBrowser() {
  const location = useLocation();
  const history = useHistory();
  const remembered = useMemo(() => readBrowserMemory(), []);
  const effectiveSearch = location.search || remembered?.search || "";
  const restoreDraft = !!remembered && effectiveSearch === remembered.search;
  const active = useMemo(
    () => readBrowserSearch(effectiveSearch),
    [effectiveSearch]
  );
  useEffect(() => {
    if (!location.search && remembered?.search)
      history.replace({ pathname: "/booru", search: remembered.search });
  }, [location.search, remembered, history]);
  const [sources, setSources] = useState<BooruSource[]>();
  const [sourceError, setSourceError] = useState("");
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [showSettings, setShowSettings] = useState(
    restoreDraft ? (remembered?.settings ?? false) : false
  );
  const [thumbnails, setThumbnails] = useState(remembered?.thumbnails ?? true);
  const [autocomplete, setAutocomplete] = useState<"live" | "database">(
    remembered?.autocomplete ?? "live"
  );
  const [tagSource, setTagSource] = useState(
    restoreDraft ? (remembered?.tagSource ?? "") : ""
  );
  const [tagPages, setTagPages] = useState<
    Record<string, { signature: string; posts: BooruPost[] }>
  >({});
  const onResult = useCallback(
    (id: string, signature: string, posts: BooruPost[]) => {
      setTagPages((previous) => ({ ...previous, [id]: { signature, posts } }));
    },
    []
  );
  const [query, setQuery] = useState(
    restoreDraft ? (remembered?.query ?? active.query) : active.query
  );
  const [selected, setSelected] = useState(
    restoreDraft ? (remembered?.sources ?? active.sources) : active.sources
  );
  const lastQuery = useRef(active.query);
  const sourceSignature = active.sources.join(",");
  const lastSources = useRef(sourceSignature);
  useEffect(() => {
    if (lastQuery.current !== active.query) {
      setQuery(active.query);
      lastQuery.current = active.query;
    }
  }, [active.query]);
  useEffect(() => {
    if (lastSources.current !== sourceSignature) {
      setSelected(sourceSignature.split(","));
      lastSources.current = sourceSignature;
    }
  }, [sourceSignature]);
  const memory = useRef({
    search: effectiveSearch,
    query,
    sources: selected,
    settings: showSettings,
    tagSource,
    scroll: restoreDraft ? (remembered?.scroll ?? 0) : 0,
    thumbnails,
    autocomplete,
  });
  useEffect(() => {
    memory.current = {
      ...memory.current,
      search: effectiveSearch,
      query,
      sources: selected,
      settings: showSettings,
      tagSource,
      thumbnails,
      autocomplete,
    };
    saveBrowserMemory(memory.current);
  }, [
    effectiveSearch,
    query,
    selected,
    showSettings,
    tagSource,
    thumbnails,
    autocomplete,
  ]);
  useEffect(() => {
    const rememberScroll = () => {
      if (document.body.classList.contains("modal-open")) return;
      memory.current.scroll = window.scrollY;
      saveBrowserMemory(memory.current);
    };
    window.addEventListener("scroll", rememberScroll, { passive: true });
    return () => window.removeEventListener("scroll", rememberScroll);
  }, []);
  const restoreScroll = useRef(restoreDraft);
  useEffect(() => {
    if (
      restoreScroll.current &&
      (Object.keys(tagPages).length >= active.sources.length || !active.query)
    ) {
      restoreScroll.current = false;
      window.requestAnimationFrame(() =>
        window.scrollTo(0, remembered?.scroll ?? 0)
      );
    }
  }, [tagPages, active, remembered]);
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
        <div className="d-flex flex-wrap">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setShowSettings((value) => !value)}
          >
            <Icon icon={showSettings ? faArrowLeft : faGear} />{" "}
            {showSettings ? "Back to posts" : "Settings"}
          </Button>
        </div>
      </header>
      {showSettings && (
        <section aria-label="Booru settings" className="booru-settings-panel">
          <h3>Booru settings</h3>
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
              className="booru-search-card"
              onSubmit={(event) => {
                event.preventDefault();
                history.push({
                  pathname: "/booru",
                  search: browserSearchURL({
                    ...active,
                    sources: selected,
                    query: query.trim(),
                    pages: {},
                    inspect: undefined,
                    showIgnored: active.showIgnored,
                  }),
                });
              }}
            >
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
                      type="switch"
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
              <Form.Group controlId="booru-query" className="mb-2">
                <Form.Label>Search</Form.Label>
                <TagAutocomplete
                  mode={autocomplete}
                  value={query}
                  onChange={setQuery}
                  sources={sources.filter((source) =>
                    selected.includes(source.id)
                  )}
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
              <div
                className="booru-filter-icons"
                aria-label="Filters and display"
              >
                <BooruIconMenu
                  label="Sort by"
                  icon={faSort}
                  value={active.sort ?? ""}
                  options={[
                    { value: "", label: "Provider default" },
                    { value: "newest", label: "Newest" },
                    { value: "popular", label: "Popular" },
                    {
                      value: "updated",
                      label: "Updated",
                      disabled: sources.some(
                        (s) =>
                          selected.includes(s.id) &&
                          ["gelbooru", "philomena"].includes(s.engine)
                      ),
                    },
                  ]}
                  onChange={(value) =>
                    history.push({
                      search: browserSearchURL({
                        ...active,
                        sort: (value || undefined) as BrowserSearch["sort"],
                        pages: {},
                        inspect: undefined,
                      }),
                    })
                  }
                />
                <BooruIconMenu
                  label="Rating"
                  icon={faShieldHalved}
                  value={active.rating ?? ""}
                  options={[
                    { value: "", label: "All ratings" },
                    { value: "safe", label: "Safe / General" },
                    { value: "questionable", label: "Questionable" },
                    { value: "explicit", label: "Explicit" },
                  ]}
                  onChange={(value) =>
                    history.push({
                      search: browserSearchURL({
                        ...active,
                        rating: (value || undefined) as BrowserSearch["rating"],
                        pages: {},
                        inspect: undefined,
                      }),
                    })
                  }
                />
                <BooruIconMenu
                  label="Media on this page"
                  icon={faImages}
                  value={active.media ?? ""}
                  options={[
                    { value: "", label: "All media" },
                    { value: "images", label: "Images" },
                    { value: "videos", label: "Videos" },
                  ]}
                  onChange={(value) =>
                    history.push({
                      search: browserSearchURL({
                        ...active,
                        media: (value || undefined) as BrowserSearch["media"],
                        inspect: undefined,
                      }),
                    })
                  }
                />
                <BooruIconMenu
                  label="Autocomplete"
                  icon={faTags}
                  value={autocomplete}
                  options={[
                    { value: "live", label: "Live, with CSV fallback" },
                    { value: "database", label: "Installed CSV only" },
                  ]}
                  onChange={(value) =>
                    setAutocomplete(value as "live" | "database")
                  }
                />
              </div>
              <div
                className="booru-display-switches"
                aria-label="Display options"
              >
                <Form.Check
                  type="switch"
                  id="booru-thumbnails"
                  label="Show thumbnails"
                  checked={thumbnails}
                  onChange={(event) =>
                    setThumbnails(event.currentTarget.checked)
                  }
                />
                <Form.Check
                  type="switch"
                  id="booru-show-ignored"
                  label="Show ignored posts"
                  checked={!!active.showIgnored}
                  onChange={(event) =>
                    history.push({
                      search: browserSearchURL({
                        ...active,
                        showIgnored: event.currentTarget.checked || undefined,
                      }),
                    })
                  }
                />
              </div>
              <details className="booru-help">
                <summary>Search help</summary>
                <p>
                  Your expression keeps its native syntax. Sort and rating
                  controls add provider filters. Leave controls at Provider
                  default / All ratings when entering your own order or rating
                  expressions. Ctrl+Enter searches directly.
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
            filters={active}
            onTag={setQuery}
          />
          <p className="small text-muted booru-library-note">
            Manual marks are history. Local links require current-file MD5
            verification.
          </p>
        </aside>
        <section className="booru-results" aria-label="Remote posts">
          <div className="booru-results-toolbar" aria-label="Result display">
            <span className="small text-muted">Posts per source page</span>
            <PageSizeSelector
              pageSize={active.pageSize ?? 24}
              maxPageSize={50}
              setPageSize={(value) =>
                history.push({
                  search: browserSearchURL({
                    ...active,
                    pageSize: value,
                    pages: {},
                    inspect: undefined,
                  }),
                })
              }
            />
          </div>
          {(!active.query.trim() || !sources) && !sourceError && (
            <div className="booru-empty">
              <h3>Discover posts</h3>
              <p>Choose your sources, enter tags, and search.</p>
              <p className="small text-muted">
                API keys, custom sites, downloads and autocomplete are available
                in Settings.
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
                    key={`${id}:${active.query}:${active.pages[id]}:${loadAttempt}:${active.sort}:${active.rating}:${active.pageSize}`}
                    source={source}
                    filters={active}
                    thumbnails={thumbnails}
                    showIgnored={!!active.showIgnored}
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
  filters,
  onTag,
}: {
  sources: BooruSource[];
  selected: string;
  onSource: (id: string) => void;
  pages: Record<string, { signature: string; posts: BooruPost[] }>;
  query: string;
  activePages: Record<string, number>;
  filters: Pick<BrowserSearch, "sort" | "rating" | "media" | "pageSize">;
  onTag: (name: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const source = sources.find((item) => item.id === selected) ?? sources[0];
  const loaded = source ? pages[source.id] : undefined;
  const posts =
    source &&
    loaded?.signature ===
      browserResultSignature(
        query,
        activePages[source.id],
        filters.sort,
        filters.rating,
        filters.media,
        filters.pageSize
      )
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
            className="btn-secondary"
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
