import {
  type ReactNode,
  type KeyboardEvent,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  Alert,
  Button,
  ButtonGroup,
  Dropdown,
  Form,
  Modal,
  Spinner,
} from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import { useHistory, useLocation } from "react-router-dom";
import {
  faArrowLeft,
  faArrowRight,
  faBan,
  faGear,
  faStar,
  faTableCells,
  faGrip,
  faSliders,
  faShieldHalved,
  faImages,
  faTags,
  faPlay,
  faStop,
  faSortAmountDown,
} from "@fortawesome/free-solid-svg-icons";
import { Icon } from "../Shared/Icon";
import { ZoomSelect } from "../List/ZoomSlider";
import {
  SavedSearchFilters,
  CoreTagActions,
  type Bookmarks,
  type BookmarkAction,
} from "./BooruBookmarks";
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
import { SourceSelector } from "./SourceSelector";
import { CreatorSearch } from "./CreatorSearch";
import { appendSearchTag } from "./completion";
import TextUtils from "src/utils/text";
import { FileSize } from "../Shared/FileSize";
import { normalizedRating, supportsSensitiveRating } from "./presentation";
import "./styles.scss";

const tagCategoryLabels: Record<string, string> = {
  artist: "Artists",
  copyright: "Copyrights",
  character: "Characters",
  general: "General",
  meta: "Metadata",
  species: "Species",
};

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
  onTag,
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
  onTag: (name: string, mode: "add" | "replace") => void;
}) {
  const groups = [...new Set(post.tags.map((tag) => tag.category))].sort(
    (a, b) => {
      const order = [
        "artist",
        "copyright",
        "character",
        "general",
        "species",
        "meta",
        "unknown",
      ];
      return order.indexOf(a) - order.indexOf(b);
    }
  );
  const identity = `${source.instance}:${post.id}`;
  const [assetSelection, setAssetSelection] = useState({ identity, index: 0 });
  const assetIndex =
    assetSelection.identity === identity ? assetSelection.index : 0;
  // Keep the modal mounted so consecutive arrow keys retain their focus target.
  useLayoutEffect(() => {
    setAssetSelection({ identity, index: 0 });
  }, [identity]);
  const assets = [post, ...(post.attachments ?? [])];
  const asset = assets[assetIndex] ?? post;
  const sample = booruMediaURL(asset, "sample");
  const download = useRef<HTMLButtonElement>(null);
  const ignore = useRef<HTMLButtonElement>(null);
  return (
    <Modal
      show
      onHide={onHide}
      size="lg"
      className="booru-browser-inspection"
      onEntered={(node: HTMLElement) => {
        const content = node.querySelector<HTMLElement>(".modal-content");
        if (content) {
          content.tabIndex = -1;
          content.focus({ preventScroll: true });
        }
      }}
      onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => {
        const target = event.target as HTMLElement;
        if (
          event.defaultPrevented ||
          event.altKey ||
          event.ctrlKey ||
          event.metaKey ||
          event.shiftKey ||
          !event.currentTarget.contains(target) ||
          target.closest(
            'input, textarea, select, video, audio, .dropdown, [contenteditable="true"], [role="combobox"], [role="menu"]'
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
          {source.name} #{post.remoteID ?? post.id}
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
        {post.mediaNotice && <Alert variant="info">{post.mediaNotice}</Alert>}
        {assets.length > 1 && (
          <Form.Group>
            <Form.Label htmlFor="booru-asset">Media file</Form.Label>
            <Form.Control
              as="select"
              id="booru-asset"
              className="btn-secondary"
              value={assetIndex}
              onChange={(event) =>
                setAssetSelection({
                  identity,
                  index: Number(event.currentTarget.value),
                })
              }
            >
              {assets.map((item, index) => (
                <option key={item.id} value={index}>
                  {index + 1} / {assets.length} · {item.format || "media"}
                </option>
              ))}
            </Form.Control>
          </Form.Group>
        )}
        <MotionPreview
          key={`motion:${asset.id}:${asset.mediaKey}`}
          post={asset}
        >
          {sample ? (
            <RemoteImage
              key={sample}
              className="booru-sample"
              src={sample}
              fallbackSrc={remoteURL(asset.directSampleURL)}
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
        <div className="booru-file-details mt-2">
          {!!asset.width && !!asset.height && (
            <span>
              {TextUtils.resolution(asset.width, asset.height)} ({asset.width} ×{" "}
              {asset.height})
            </span>
          )}
          {!!asset.bytes && (
            <span>
              <FileSize size={asset.bytes} />
            </span>
          )}
          {asset.format && <span>{asset.format.toUpperCase()}</span>}
          {post.rating && (
            <span
              className={`booru-rating-label rating-${normalizedRating(post.rating, source)}`}
            >
              {normalizedRating(post.rating, source)}
            </span>
          )}
        </div>
        <BooruDownload
          key={`download:${asset.id}:${asset.mediaKey}`}
          downloadRef={download}
          post={asset}
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
                : (tagCategoryLabels[category] ?? category)}
            </h6>
            <div className="booru-tags">
              {post.tags
                .filter((tag) => tag.category === category)
                .map((tag) => (
                  <Dropdown
                    key={`${category}:${tag.name}`}
                    className="booru-preview-tag"
                  >
                    <Dropdown.Toggle
                      variant="link"
                      className={`booru-tag tag-${category}`}
                      aria-label={`Tag ${tag.label ?? tag.name}`}
                    >
                      {tag.label ?? tag.name}
                    </Dropdown.Toggle>
                    <Dropdown.Menu role="menu">
                      <Dropdown.Item
                        role="menuitem"
                        onClick={() => {
                          onTag(tag.name, "add");
                        }}
                      >
                        Add to search
                      </Dropdown.Item>
                      <Dropdown.Item
                        role="menuitem"
                        onClick={() => {
                          onTag(tag.name, "replace");
                        }}
                      >
                        Replace search
                      </Dropdown.Item>
                    </Dropdown.Menu>
                  </Dropdown>
                ))}
            </div>
          </section>
        ))}
        <div className="booru-hash">
          {asset.md5 && (
            <span>
              MD5: <code>{asset.md5}</code>
            </span>
          )}
          {assetIndex === 0 && <ReviewEvidence state={review} />}
        </div>
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
  suspended,
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
  favorites,
  displayMode,
  zoom,
  onTag,
}: {
  source: BooruSource;
  suspended: boolean;
  query: string;
  showIgnored: boolean;
  filters: Pick<
    BrowserSearch,
    "sort" | "rating" | "media" | "pageSize" | "favorites"
  >;
  thumbnails: boolean;
  favorites: boolean;
  displayMode: "grid" | "wall";
  zoom: number;
  onTag: (name: string, mode?: "add" | "replace") => void;
  page: number;
  inspect?: string;
  onPage: (page: number) => void;
  onInspect: (id: string) => void;
  onClose: () => void;
  onResult: (
    id: string,
    signature: string,
    posts: BooruPost[],
    ready: boolean
  ) => void;
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
    filters.pageSize,
    favorites
  );
  const [review, setReview] = useState<ReviewState[]>([]);
  const onReview = useCallback((value: ReviewState[]) => setReview(value), []);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  // Results are not ready for scroll restoration until the provider finishes.
  // The initial empty posts list is only a loading placeholder.
  useEffect(
    () => onResult(source.id, signature, visiblePosts, !loading),
    [onResult, source.id, signature, visiblePosts, loading]
  );
  const [retry, setRetry] = useState(0);
  const controller = useRef<AbortController>();
  // biome-ignore lint/correctness/useExhaustiveDependencies: Retry intentionally repeats the same read after a source failure.
  useEffect(() => {
    if (suspended) return;
    const request = new AbortController();
    controller.current = request;
    setResult(undefined);
    setError("");
    setLoading(true);

    const params = new URLSearchParams({
      source: source.id,
      instance: source.instance,
      q: query,
      page: String(page),
      limit: String(filters.pageSize ?? 24),
      ...(filters.sort ? { sort: filters.sort } : {}),
      ...(filters.rating ? { rating: filters.rating } : {}),
    });
    void fetch(
      `booru-browser/${favorites ? "account-favorites" : "search"}?${params}`,
      { signal: request.signal }
    )
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
    source.instance,
    favorites,
    query,
    page,
    retry,
    filters.sort,
    filters.rating,
    filters.pageSize,
    suspended,
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
            displayMode={displayMode}
            zoom={zoom}
            source={source}
            posts={visiblePosts}
            showIgnored={showIgnored}
            onStates={onReview}
            preview={({ ignore, busy }) =>
              post && (
                <PostInspection
                  post={post}
                  source={source}
                  onTag={onTag}
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
            {(item, controls, isSelected, selection) => (
              <article
                key={`${source.id}:${item.id}`}
                className={`booru-post ${isSelected ? "selected" : ""}`}
                onClickCapture={(event) => {
                  if (
                    selection.active &&
                    !(event.target as HTMLElement).closest(".card-check")
                  ) {
                    event.preventDefault();
                    event.stopPropagation();
                    selection.toggle();
                  }
                }}
              >
                <Button
                  variant="link"
                  className="booru-preview-button"
                  onClick={() => onInspect(item.id)}
                  aria-label={`Inspect ${source.name} post ${item.id}`}
                >
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
                    #{item.remoteID ?? item.id}
                  </Button>
                  {item.rating && (
                    <span
                      className={`booru-rating rating-${normalizedRating(item.rating, source)}`}
                    >
                      {normalizedRating(item.rating, source)}
                    </span>
                  )}
                </div>
                <CoreTagActions post={item} onTag={onTag} />
                <div className="booru-card-actions">
                  <BooruDownload
                    post={item}
                    instance={source.instance}
                    compact
                  />
                </div>
                {controls}
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
    if (
      location.pathname === "/booru" &&
      !location.search &&
      remembered?.search
    )
      history.replace({ pathname: "/booru", search: remembered.search });
  }, [location.pathname, location.search, remembered, history]);
  const hasSearch =
    new URLSearchParams(effectiveSearch).has("q") || active.favorites;
  const [sources, setSources] = useState<BooruSource[]>();
  const [sourceError, setSourceError] = useState("");
  const [loadAttempt, setLoadAttempt] = useState(0);
  const showSettings = location.pathname === "/booru/settings";
  const [displayMode, setDisplayMode] = useState<"grid" | "wall">(
    remembered?.displayMode ?? "grid"
  );
  const [zoom, setZoom] = useState(remembered?.zoom ?? 0);
  const [sourceOrder, setSourceOrder] = useState(remembered?.sourceOrder ?? []);
  const [searchOnTagClick, setSearchOnTagClick] = useState(
    remembered?.searchOnTagClick ?? true
  );
  const [bookmarks, setBookmarks] = useState<Bookmarks>();
  const [bookmarkBusy, setBookmarkBusy] = useState(false);
  const [bookmarkError, setBookmarkError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void fetch("booru-browser/bookmarks", { signal: controller.signal })
      .then(readBooruResponse<Bookmarks>)
      .then((value) => {
        if (!controller.signal.aborted) setBookmarks(value);
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) setBookmarkError(String(error));
      });
    return () => controller.abort();
  }, []);
  async function saveBookmark(action: BookmarkAction) {
    if (bookmarkBusy) return;
    setBookmarkBusy(true);
    setBookmarkError("");
    try {
      setBookmarks(
        await fetch("booru-browser/bookmarks", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(action),
        }).then(readBooruResponse<Bookmarks>)
      );
    } catch (error: unknown) {
      setBookmarkError(String(error));
    } finally {
      setBookmarkBusy(false);
    }
  }
  const [thumbnails, setThumbnails] = useState(remembered?.thumbnails ?? true);
  const [autocomplete, setAutocomplete] = useState<"live" | "database">(
    remembered?.autocomplete ?? "live"
  );
  const [tagSource, setTagSource] = useState(
    restoreDraft ? (remembered?.tagSource ?? "") : ""
  );
  const [tagPages, setTagPages] = useState<
    Record<string, { signature: string; posts: BooruPost[]; ready: boolean }>
  >({});
  const onResult = useCallback(
    (id: string, signature: string, posts: BooruPost[], ready: boolean) => {
      setTagPages((previous) => ({
        ...previous,
        [id]: { signature, posts, ready },
      }));
    },
    []
  );
  const [query, setQuery] = useState(
    restoreDraft ? (remembered?.query ?? active.query) : active.query
  );
  const [selected, setSelected] = useState(active.sources.slice(0, 1));
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
      setSelected(sourceSignature.split(",").slice(0, 1));
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
    displayMode,
    zoom,
    autocomplete,
    sourceOrder,
    searchOnTagClick,
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
      displayMode,
      zoom,
      autocomplete,
      sourceOrder,
      searchOnTagClick,
    };
    saveBrowserMemory(memory.current);
  }, [
    effectiveSearch,
    query,
    selected,
    showSettings,
    tagSource,
    thumbnails,
    displayMode,
    zoom,
    autocomplete,
    sourceOrder,
    searchOnTagClick,
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
    // An explicit q= is a submitted search even when its query is blank.
    // Require the matching source results to finish loading before scrolling;
    // simply counting mounted SourceResults reports loading placeholders.
    const resultsReady = active.sources.every((id) => {
      const page = tagPages[id];
      return (
        page?.ready &&
        page.signature ===
          browserResultSignature(
            active.query,
            active.pages[id],
            active.sort,
            active.rating,
            active.media,
            active.pageSize,
            active.favorites
          )
      );
    });
    if (
      restoreScroll.current &&
      (!hasSearch || !!active.error || (sources && resultsReady))
    ) {
      restoreScroll.current = false;
      window.requestAnimationFrame(() =>
        window.scrollTo(0, remembered?.scroll ?? 0)
      );
    }
  }, [tagPages, active, remembered, hasSearch, sources]);
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
            current
              .filter((id) => value.some((source) => source.id === id))
              .slice(0, 1)
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
  const selectedSource = sources?.find((source) => source.id === selected[0]);
  const unsupportedRating =
    !!active.favorites ||
    selected.some((id) =>
      ["shimmie", "pixiv", "pawchive", "kemono"].includes(
        sources?.find((source) => source.id === id)?.engine ?? ""
      )
    );
  function searchTag(name: string, mode: "add" | "replace" = "replace") {
    const value =
      mode === "replace"
        ? name
        : appendSearchTag(query, name, selectedSource?.engine === "philomena");
    if (new TextEncoder().encode(value).length > 1024) {
      setBookmarkError("Search would exceed 1024 bytes.");
      return;
    }
    setQuery(value);
    if (searchOnTagClick) {
      history.push({
        pathname: "/booru",
        search: browserSearchURL({
          ...active,
          sources: selected,
          query: value,
          pages: {},
          inspect: undefined,
          favorites: undefined,
        }),
      });
    } else {
      history.replace({
        search: browserSearchURL({ ...active, inspect: undefined }),
      });
      window.requestAnimationFrame(() =>
        document.getElementById("booru-query")?.focus()
      );
    }
  }
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
            onClick={() => {
              if (
                showSettings &&
                (location.state as { booruSettingsFrom?: boolean } | undefined)
                  ?.booruSettingsFrom
              )
                history.goBack();
              else
                history.push({
                  pathname: showSettings ? "/booru" : "/booru/settings",
                  search: effectiveSearch,
                  state: { booruSettingsFrom: !showSettings },
                });
            }}
          >
            <Icon icon={showSettings ? faArrowLeft : faGear} />{" "}
            {showSettings ? "Back to posts" : "Settings"}
          </Button>
        </div>
      </header>
      {showSettings && (
        <section aria-label="Booru settings" className="booru-settings-panel">
          <h3>Booru settings</h3>
          <Form.Check
            type="switch"
            id="booru-search-on-tag"
            label="Search immediately when clicking tags"
            checked={searchOnTagClick}
            onChange={(event) =>
              setSearchOnTagClick(event.currentTarget.checked)
            }
          />
          <p className="small text-muted">
            Disable to only fill the search box. This preference and source
            order are saved in this browser.
          </p>
          <SourceSettings
            onSaved={() => setLoadAttempt((value) => value + 1)}
          />
        </section>
      )}
      {sources && (
        <Form
          className="booru-search-card"
          hidden={showSettings}
          onSubmit={(event) => {
            event.preventDefault();
            history.push({
              pathname: "/booru",
              search: browserSearchURL({
                ...active,
                favorites: undefined,
                sources: selected,
                query: query.trim(),
                pages: {},
                inspect: undefined,
                showIgnored: active.showIgnored,
              }),
            });
          }}
        >
          <div className="filtered-list-toolbar btn-toolbar">
            <SourceSelector
              sources={sources}
              selected={selectedSource}
              order={sourceOrder}
              onOrder={setSourceOrder}
              onSelect={(source) => {
                const nextQuery =
                  selected[0] !== source.id &&
                  (query.startsWith("creator:") || source.engine === "pawchive")
                    ? ""
                    : query.trim();
                setSelected([source.id]);
                setQuery(nextQuery);
                history.push({
                  pathname: "/booru",
                  search: browserSearchURL({
                    ...active,
                    sources: [source.id],
                    query: nextQuery,
                    pages: {},
                    inspect: undefined,
                    rating: undefined,
                    sort: undefined,
                    favorites: undefined,
                  }),
                });
              }}
            />
            <Form.Group
              controlId="booru-query"
              className="booru-toolbar-query mb-0"
            >
              <Form.Label>Search</Form.Label>
              <TagAutocomplete
                mode={autocomplete}
                value={query}
                submittedValue={active.query}
                submissionKey={location.key}
                onChange={setQuery}
                sources={sources.filter((source) =>
                  selected.includes(source.id)
                )}
              />
            </Form.Group>
            <Button
              type="submit"
              size="sm"
              className="booru-search-submit"
              aria-label="Search selected sources"
              disabled={selected.length === 0}
            >
              Search
            </Button>
            <div
              className="booru-filter-icons"
              aria-label="Filters and display"
            >
              <BooruIconMenu
                label="Rating"
                icon={faShieldHalved}
                value={active.rating ?? ""}
                options={[
                  { value: "", label: "All ratings" },
                  {
                    value: "safe",
                    label: "Safe / General",
                    className: "rating-safe",
                    disabled: unsupportedRating,
                  },
                  {
                    value: "sensitive",
                    label: "Sensitive",
                    className: "rating-sensitive",
                    disabled:
                      unsupportedRating ||
                      !selectedSource ||
                      !supportsSensitiveRating(selectedSource),
                  },
                  {
                    value: "questionable",
                    label: "Questionable",
                    className: "rating-questionable",
                    disabled: unsupportedRating,
                  },
                  {
                    value: "explicit",
                    label: "Explicit",
                    className: "rating-explicit",
                    disabled: unsupportedRating,
                  },
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
            <Dropdown className="list-operations btn-group">
              <Dropdown.Toggle variant="secondary" aria-label="Display options">
                <Icon icon={faSliders} />
              </Dropdown.Toggle>
              <Dropdown.Menu
                renderOnMount
                className="px-3 booru-display-switches"
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
              </Dropdown.Menu>
            </Dropdown>
            <Dropdown className="saved-filter-dropdown dropdown btn-group">
              <Dropdown.Toggle
                variant="secondary"
                title="Favorites"
                aria-label="Favorites"
              >
                <Icon icon={faStar} />
              </Dropdown.Toggle>
              <Dropdown.Menu>
                <Dropdown.Header>
                  {selectedSource?.name} account favorites
                </Dropdown.Header>
                <Dropdown.Item
                  disabled={!selectedSource?.accountFavorites}
                  onClick={() =>
                    history.push({
                      search: browserSearchURL({
                        ...active,
                        sources: selected,
                        favorites: true,
                        query: "",
                        rating: undefined,
                        sort: undefined,
                        pages: {},
                        inspect: undefined,
                      }),
                    })
                  }
                >
                  Browse account favorites
                </Dropdown.Item>
                {!selectedSource?.accountFavorites && (
                  <Dropdown.ItemText>
                    {["danbooru", "gelbooru", "e621"].includes(
                      selectedSource?.engine ?? ""
                    )
                      ? "Save your account login/user ID and API key in Settings."
                      : "This engine has no supported account favorites API."}
                  </Dropdown.ItemText>
                )}
              </Dropdown.Menu>
            </Dropdown>
            <SavedSearchFilters
              bookmarks={bookmarks}
              busy={bookmarkBusy}
              search={browserSearchURL({
                ...active,
                query: active.favorites ? "" : query.trim(),
                sources: selected,
                pages: {},
                inspect: undefined,
              })}
              onSave={(action) => void saveBookmark(action)}
              onSelect={(search) => history.push({ search })}
            />
            <BooruIconMenu
              label="Sort by"
              icon={faSortAmountDown}
              className="sort-by-select dropdown btn-group"
              value={active.sort ?? ""}
              options={[
                { value: "", label: "Provider default" },
                {
                  value: "newest",
                  label: "Recent",
                  disabled:
                    !!active.favorites ||
                    (selectedSource?.engine === "pixiv" &&
                      !active.query.trim()),
                },
                {
                  value: "oldest",
                  label: "Oldest",
                  disabled:
                    !!active.favorites ||
                    ["shimmie", "pixiv", "pawchive", "kemono"].includes(
                      selectedSource?.engine ?? ""
                    ),
                },
                {
                  value: "popular",
                  label: "Popular",
                  disabled:
                    !!active.favorites ||
                    ["shimmie", "pixiv", "pawchive", "kemono"].includes(
                      selectedSource?.engine ?? ""
                    ),
                },
                {
                  value: "score_desc",
                  label: "Score: high to low",
                  disabled:
                    !!active.favorites ||
                    ["shimmie", "pixiv", "pawchive", "kemono"].includes(
                      selectedSource?.engine ?? ""
                    ),
                },
                {
                  value: "score_asc",
                  label: "Score: low to high",
                  disabled:
                    !!active.favorites ||
                    ["shimmie", "pixiv", "pawchive", "kemono"].includes(
                      selectedSource?.engine ?? ""
                    ),
                },
                {
                  value: "updated",
                  label: "Updated",
                  disabled:
                    !!active.favorites ||
                    !["danbooru", "e621", "moebooru"].includes(
                      selectedSource?.engine ?? ""
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
            <ButtonGroup aria-label="View mode">
              <Button
                variant="secondary"
                active={displayMode === "grid"}
                aria-label="Grid view"
                onClick={() => setDisplayMode("grid")}
              >
                <Icon icon={faTableCells} />
              </Button>
              <Button
                variant="secondary"
                active={displayMode === "wall"}
                aria-label="Wall view"
                onClick={() => setDisplayMode("wall")}
              >
                <Icon icon={faGrip} />
              </Button>
            </ButtonGroup>
            <div className="zoom-slider-container" aria-label="Thumbnail size">
              <label className="mb-0 w-100">
                <span className="sr-only">Thumbnail size</span>
                <ZoomSelect zoomIndex={zoom} onChangeZoom={setZoom} />
              </label>
            </div>
          </div>
        </Form>
      )}
      {bookmarkError && <Alert variant="warning">{bookmarkError}</Alert>}
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
          {selectedSource &&
            ["pawchive", "kemono"].includes(selectedSource.engine) && (
              <CreatorSearch
                key={selectedSource.instance}
                source={selectedSource}
                onChoose={(value) => {
                  setQuery(value);
                  history.push({
                    pathname: "/booru",
                    search: browserSearchURL({
                      ...active,
                      sources: [selectedSource.id],
                      query: value,
                      pages: {},
                      inspect: undefined,
                      favorites: undefined,
                      rating: undefined,
                      sort: undefined,
                    }),
                  });
                }}
              />
            )}
          <details className="booru-help">
            <summary>Search help</summary>
            <p>
              Your expression keeps its native syntax. Leave All ratings
              selected when entering native rating expressions. Pixiv and
              archive engines currently require All ratings. Ctrl+Enter searches
              directly.
            </p>
            {(sources ?? [])
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
            onTag={searchTag}
          />
          <p className="small text-muted booru-library-note">
            Manual marks are history. Local links require current-file MD5
            verification.
          </p>
        </aside>
        <section className="booru-results" aria-label="Remote posts">
          {(!hasSearch || !sources) && !sourceError && (
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
          {hasSearch &&
            !active.error &&
            !unknown &&
            sources &&
            active.sources.map((id) => {
              const source = sources.find((item) => item.id === id);
              return (
                source && (
                  <SourceResults
                    key={`${id}:${active.favorites}:${active.query}:${active.pages[id]}:${loadAttempt}:${active.sort}:${active.rating}:${active.pageSize}`}
                    source={source}
                    suspended={showSettings}
                    favorites={!!active.favorites}
                    onTag={searchTag}
                    displayMode={displayMode}
                    zoom={zoom}
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
  filters: Pick<
    BrowserSearch,
    "sort" | "rating" | "media" | "pageSize" | "favorites"
  >;
  onTag: (name: string, mode?: "add" | "replace") => void;
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
        filters.pageSize,
        filters.favorites
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
          Counts cover this loaded page only. Tag-click search behavior can be
          changed in Settings.
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
