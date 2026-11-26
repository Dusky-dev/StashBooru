import {
  FollowAction,
  notifyFollowingChanged,
  useMarkSeenOnView,
} from "../Following/FollowAction";
import { MarkSeenOnViewSwitch } from "../Following/FollowScheduleSettings";
import { IdentityLink } from "../Identities/identity";
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
  Collapse,
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
  faChevronDown,
  faChevronRight,
  faBan,
  faGear,
  faStar,
  faTableCells,
  faGrip,
  faSliders,
  faShieldHalved,
  faImages,
  faRobot,
  faTags,
  faPlay,
  faStop,
  faLink,
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
import { Pagination } from "../List/Pagination";
import { BooruIconButton, BooruIconMenu } from "./BooruControls";
import { mediaReviewIDs, motionPreviewURL } from "./browser";
import { ExternalLink } from "../Shared/ExternalLink";
import {
  type BooruPost,
  type BooruResult,
  type BooruSource,
  type BrowserSearch,
  MAX_BOORU_PAGE,
  postMediaKind,
  isAIGeneratedPost,
  browserResultSignature,
  browserSearchURL,
  readBooruResponse,
  readBrowserSearch,
  remoteURL,
  booruMediaURL,
  inspectionMedia,
  sharedLinkCount,
} from "./browser";
import { BooruReview, ReviewEvidence, type ReviewState } from "./BooruReview";
import { TagAutocomplete } from "./TagAutocomplete";
import { BooruDownload } from "./BooruDownload";
import { BooruFilenameMigration } from "./BooruFilenameMigration";
import { BooruArchive } from "./BooruArchive";
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

// Posts with more media than this get a collapsible media group header.
const LARGE_MEDIA_GROUP = 4;

const isArchiveFormat = (format?: string) =>
  ["zip", "cbz", "7z"].includes(format?.toLowerCase() ?? "");

const message = (id: string, defaultMessage: string) => (
  <FormattedMessage
    id={`booru_browser.${id}`}
    defaultMessage={defaultMessage}
  />
);

// The inspection dialog's focusable content, and whether keyboard focus has
// left it (dropped to the page, or onto a control that became disabled).
const dialogContent = (body: HTMLElement | null) =>
  body?.closest<HTMLElement>(".modal-content") ?? null;
function lostFocus(content: HTMLElement) {
  const active = document.activeElement as HTMLElement | null;
  return (
    !active ||
    active === document.body ||
    !content.contains(active) ||
    (active as HTMLButtonElement).disabled === true
  );
}

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
  onBlacklistTag,
  blacklistBusy,
  blacklistError,
  detailsLoading,
  detailsError,
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
  onBlacklistTag: (name: string) => void;
  blacklistBusy: boolean;
  blacklistError: string;
  detailsLoading?: boolean;
  detailsError?: string;
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
  const assets = useMemo(() => inspectionMedia(post), [post]);
  const multiMedia = assets.length > 1;
  // Each media of a post with several can be ignored on its own; its
  // review key is the attachment ID, or a derived key for the post's own
  // file (the post ID means the whole post).
  const mediaKeys = useMemo(() => mediaReviewIDs(post, assets), [post, assets]);
  const [mediaIgnored, setMediaIgnored] = useState<{
    identity: string;
    ignored: ReadonlySet<string>;
  }>({ identity, ignored: new Set() });
  const ignoredKeys =
    mediaIgnored.identity === identity
      ? mediaIgnored.ignored
      : new Set<string>();
  const ignoredKeysRef = useRef(ignoredKeys);
  ignoredKeysRef.current = ignoredKeys;
  const [mediaError, setMediaError] = useState("");
  const isMediaIgnored = (index: number) =>
    ignoredKeys.has(mediaKeys[index] ?? "");
  useEffect(() => {
    setMediaError("");
    if (!multiMedia) return;
    const controller = new AbortController();
    const chunks: string[][] = [];
    for (let i = 0; i < mediaKeys.length; i += 50)
      chunks.push(mediaKeys.slice(i, i + 50));
    void Promise.all(
      chunks.map((ids) =>
        fetch("booru-browser/review/read", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          signal: controller.signal,
          body: JSON.stringify({
            source: source.id,
            instance: source.instance,
            posts: ids.map((id) => ({ id, md5: "" })),
          }),
        }).then(readBooruResponse<ReviewState[]>)
      )
    )
      .then((values) => {
        if (controller.signal.aborted) return;
        setMediaIgnored({
          identity,
          ignored: new Set(
            values
              .flat()
              .filter((state) => state.ignoredAt)
              .map((state) => state.id)
          ),
        });
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted)
          setMediaError(cause instanceof Error ? cause.message : String(cause));
      });
    return () => controller.abort();
  }, [identity, multiMedia, mediaKeys, source.id, source.instance]);
  // Writes run one after another, each reading the current version first,
  // so quick repeated presses neither race nor conflict.
  const mediaWrites = useRef(Promise.resolve());
  function writeMediaIgnored(index: number, ignored: boolean) {
    const key = mediaKeys[index];
    if (!key) return;
    const update = (apply: (next: Set<string>) => void) =>
      setMediaIgnored((current) => {
        const next = new Set(
          current.identity === identity ? current.ignored : []
        );
        apply(next);
        return { identity, ignored: next };
      });
    update((next) => (ignored ? next.add(key) : next.delete(key)));
    setMediaError("");
    const body = (extra: object) =>
      JSON.stringify({
        source: source.id,
        instance: source.instance,
        ...extra,
      });
    mediaWrites.current = mediaWrites.current
      .then(async () => {
        const [state] = await fetch("booru-browser/review/read", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: body({ posts: [{ id: key, md5: "" }] }),
        }).then(readBooruResponse<ReviewState[]>);
        await fetch("booru-browser/review/update", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: body({
            changes: [{ id: key, version: state?.version ?? 0, ignored }],
          }),
        }).then(readBooruResponse<ReviewState[]>);
      })
      .catch((cause: unknown) => {
        update((next) => (ignored ? next.delete(key) : next.add(key)));
        setMediaError(
          `Could not save the media's ignore mark: ${cause instanceof Error ? cause.message : String(cause)}`
        );
      });
  }
  // Incremental rendering prevents posts with many attachments from creating
  // hundreds of image/player/download components at once.
  const [stream, setStream] = useState({ identity, count: 3 });
  const visibleCount = stream.identity === identity ? stream.count : 3;
  const scrollBody = useRef<HTMLDivElement>(null);
  const loadMore = useRef<HTMLDivElement>(null);
  // One explicit, highlighted keyboard selection over every visible media
  // target: post media, archive headers and extracted files, and the media
  // group header. Shift+Up/Down move it; Up downloads it.
  const [selection, setSelection] = useState<{
    identity: string;
    target: string;
  }>();
  const [mediaOpen, setMediaOpen] = useState({ identity, open: true });
  const groupMedia = assets.length > LARGE_MEDIA_GROUP;
  const mediaExpanded = mediaOpen.identity !== identity || mediaOpen.open;
  const firstTarget = groupMedia
    ? "media"
    : isArchiveFormat(assets[0]?.format)
      ? "archive:0"
      : "asset:0";
  const selected =
    selection?.identity === identity ? selection.target : firstTarget;
  const select = useCallback(
    (target: string) => setSelection({ identity, target }),
    [identity]
  );
  // Only one post media file is expanded at a time; the rest are one-line
  // rows that mount no image or player. It follows explicit choices (keys,
  // clicks, focus), never the scroll tracking below, so expanding a row
  // cannot scroll the highlight away and reselect in a loop.
  const [openAsset, setOpenAsset] = useState({ identity, index: 0 });
  const openIndex = openAsset.identity === identity ? openAsset.index : 0;
  const choose = useCallback(
    (target: string) => {
      setSelection({ identity, target });
      const match = /^asset:(\d+)$/.exec(target);
      if (match) setOpenAsset({ identity, index: Number(match[1]) });
    },
    [identity]
  );
  useLayoutEffect(() => {
    setStream({ identity, count: 3 });
    if (scrollBody.current) scrollBody.current.scrollTop = 0;
  }, [identity]);
  const selectedIndex = (() => {
    const match = /^(?:asset|archive):(\d+)/.exec(selected);
    return match ? Number(match[1]) : undefined;
  })();
  // The gallery (and Q) open a media file: mount it, open its group, select
  // it, then scroll it into view once rendered.
  const [reveal, setReveal] = useState<{ identity: string; index: number }>();
  function showMedia(index: number) {
    if (!assets[index]) return;
    if (!mediaExpanded) setMediaOpen({ identity, open: true });
    setStream((current) => ({
      identity,
      count: Math.max(
        current.identity === identity ? current.count : 3,
        index + 1
      ),
    }));
    choose(
      isArchiveFormat(assets[index].format)
        ? `archive:${index}`
        : `asset:${index}`
    );
    setReveal({ identity, index });
  }
  useEffect(() => {
    if (!reveal || reveal.identity !== identity) return;
    const frame = requestAnimationFrame(() =>
      scrollBody.current
        ?.querySelector(`[data-asset-index="${reveal.index}"]`)
        ?.scrollIntoView({ block: "start" })
    );
    return () => cancelAnimationFrame(frame);
  }, [reveal, identity]);
  function visibleTargets() {
    const body = scrollBody.current;
    if (!body) return [];
    return [
      ...body.querySelectorAll<HTMLElement>("[data-booru-target]"),
    ].filter((element) => element.getClientRects().length > 0);
  }
  function targetElement(target: string) {
    return visibleTargets().find(
      (element) => element.dataset.booruTarget === target
    );
  }
  function focusTarget(element: HTMLElement) {
    choose(element.dataset.booruTarget ?? firstTarget);
    element.focus({ preventScroll: true });
    element.scrollIntoView({ block: "nearest" });
  }
  function moveSelection(delta: 1 | -1) {
    const body = scrollBody.current;
    if (!body) return;
    const targets = visibleTargets();
    const current = targets.findIndex(
      (element) => element.dataset.booruTarget === selected
    );
    const next = current < 0 ? 0 : current + delta;
    if (delta > 0 && current >= 0) {
      // Media that is not mounted yet comes before the next mounted target:
      // reveal the load-more marker so the next press reaches it.
      const pending = [
        ...body.querySelectorAll<HTMLElement>(".booru-media-load-more"),
      ].find(
        (marker) =>
          marker.getClientRects().length > 0 &&
          targets[current].compareDocumentPosition(marker) &
            Node.DOCUMENT_POSITION_FOLLOWING &&
          (!targets[next] ||
            marker.compareDocumentPosition(targets[next]) &
              Node.DOCUMENT_POSITION_FOLLOWING)
      );
      if (pending) {
        pending.scrollIntoView({ block: "end" });
        return;
      }
    }
    if (targets[next]) focusTarget(targets[next]);
  }
  function keepSelectionVisible() {
    const body = scrollBody.current;
    if (!body) return;
    const view = body.getBoundingClientRect();
    const current = targetElement(selected);
    if (current) {
      const bounds = current.getBoundingClientRect();
      if (bounds.bottom > view.top && bounds.top < view.bottom) return;
    }
    // The selection scrolled out of view: select the target nearest the top.
    const top = view.top + 24;
    let closest = Number.POSITIVE_INFINITY;
    let nearest: string | undefined;
    for (const item of visibleTargets()) {
      const bounds = item.getBoundingClientRect();
      const distance =
        bounds.bottom < top
          ? top - bounds.bottom
          : bounds.top > top
            ? bounds.top - top
            : 0;
      if (distance < closest) {
        closest = distance;
        nearest = item.dataset.booruTarget;
      }
    }
    if (nearest) select(nearest);
  }
  useEffect(() => {
    if (visibleCount >= assets.length) return;
    const root = scrollBody.current;
    const target = loadMore.current;
    if (!root || !target) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting))
          setStream((current) =>
            current.identity === identity
              ? { identity, count: Math.min(assets.length, current.count + 3) }
              : { identity, count: 3 }
          );
      },
      { root, rootMargin: "240px 0px" }
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [identity, visibleCount, assets.length]);
  const ignore = useRef<HTMLButtonElement>(null);
  // E and Q act once their button is usable: a file that was just opened
  // is still loading its download settings, and Ignore waits for a previous
  // review write. A press is dropped if the post changes meanwhile.
  const currentIdentity = useRef(identity);
  currentIdentity.current = identity;
  // Q pressed again while the previous ignore is still moving to the next
  // post is meant for that next post: carry it over once the post changes.
  const lastIgnore = useRef({ identity: "", at: 0 });
  const queuedIgnores = useRef(0);
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs once per shown post; pressWhenReady reads refs.
  useEffect(() => {
    if (queuedIgnores.current < 1) return;
    queuedIgnores.current -= 1;
    if (multiMedia) {
      ignoreSelectedMedia();
      return;
    }
    lastIgnore.current = { identity, at: Date.now() };
    pressWhenReady(() => ignore.current);
  }, [identity]);
  // Q on a post with several media ignores the selected one and moves to
  // the next media not ignored; ignoring the last one ignores the post.
  function ignoreSelectedMedia() {
    if (!multiMedia) {
      ignorePost();
      return;
    }
    const ignoredNow = new Set(ignoredKeysRef.current);
    const open = (index: number) => !ignoredNow.has(mediaKeys[index]);
    const index = selectedIndex ?? assets.findIndex((_, item) => open(item));
    if (index >= 0 && open(index)) {
      writeMediaIgnored(index, true);
      ignoredNow.add(mediaKeys[index]);
    }
    const remaining = assets
      .map((_, item) => item)
      .filter((item) => open(item));
    if (remaining.length === 0) {
      ignorePost();
      return;
    }
    showMedia(remaining.find((item) => item > index) ?? remaining[0]);
  }
  function ignorePost() {
    const button = ignore.current;
    const justIgnored =
      lastIgnore.current.identity === identity &&
      Date.now() - lastIgnore.current.at < 3000;
    if (button && !button.disabled) {
      lastIgnore.current = { identity, at: Date.now() };
      button.click();
    } else if (justIgnored) {
      // Still showing the post just ignored: the press is for the next one.
      queuedIgnores.current += 1;
    } else {
      lastIgnore.current = { identity, at: Date.now() };
      pressWhenReady(() => ignore.current);
    }
  }

  function pressWhenReady(
    button: () => HTMLButtonElement | null | undefined,
    tries = 60
  ) {
    const pressedOn = identity;
    const attempt = (left: number) => {
      if (currentIdentity.current !== pressedOn) return;
      const target = button();
      if (target && !target.disabled) {
        target.click();
        return;
      }
      if (left > 0) window.setTimeout(() => attempt(left - 1), 50);
    };
    attempt(tries);
  }
  function downloadSelected() {
    const target = selected;
    // Open a collapsed row so the download's progress is visible.
    if (targetElement(target)?.classList.contains("booru-asset-collapsed"))
      choose(target);
    pressWhenReady(() =>
      targetElement(target)?.querySelector<HTMLButtonElement>(
        ".booru-download-button"
      )
    );
  }
  function handleKey(
    event: Pick<
      globalThis.KeyboardEvent,
      | "key"
      | "altKey"
      | "ctrlKey"
      | "metaKey"
      | "shiftKey"
      | "repeat"
      | "defaultPrevented"
      | "preventDefault"
      | "stopPropagation"
    >,
    target: HTMLElement
  ) {
    if (
      event.defaultPrevented ||
      event.altKey ||
      event.ctrlKey ||
      event.metaKey ||
      target.closest(
        'input, textarea, select, video, audio, .dropdown, [contenteditable="true"], [role="combobox"], [role="menu"]'
      )
    )
      return;
    const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
    if (
      !["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "e", "q"].includes(
        key
      ) ||
      (event.shiftKey && !["ArrowUp", "ArrowDown"].includes(key))
    )
      return;
    // Also stops the matching keypress, so page-wide single-key shortcuts
    // never see E or Q.
    event.preventDefault();
    event.stopPropagation();
    if (key === "ArrowUp" || key === "ArrowDown") {
      moveSelection(key === "ArrowDown" ? 1 : -1);
      return;
    }
    if (key === "ArrowLeft") onPrevious?.();
    if (key === "ArrowRight") onNext?.();
    if (event.repeat) return;
    if (key === "e") downloadSelected();
    if (key === "q") ignoreSelectedMedia();
  }
  // Switching posts can remove or disable the focused control, which drops
  // focus to the page and the dialog's keys stop working. Bring focus back
  // to the dialog, and handle a key pressed while focus was lost.
  // biome-ignore lint/correctness/useExhaustiveDependencies: checks once per shown post and media list.
  useEffect(() => {
    const content = dialogContent(scrollBody.current);
    if (content && lostFocus(content)) content.focus({ preventScroll: true });
  }, [identity, assets.length]);
  const handleKeyRef = useRef(handleKey);
  handleKeyRef.current = handleKey;
  useEffect(() => {
    const onKey = (event: globalThis.KeyboardEvent) => {
      const content = dialogContent(scrollBody.current);
      if (!content || !lostFocus(content)) return;
      // Another dialog above this one owns its keys.
      const open = [...document.querySelectorAll(".modal.show")];
      if (open.length && !open.at(-1)?.contains(content)) return;
      content.focus({ preventScroll: true });
      handleKeyRef.current(event, content);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);
  return (
    <Modal
      show
      onHide={onHide}
      size="lg"
      scrollable
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
        if (event.currentTarget.contains(target)) handleKey(event, target);
      }}
    >
      <Modal.Header closeButton>
        <Modal.Title>
          {source.name} #{post.remoteID ?? post.id}
        </Modal.Title>
      </Modal.Header>
      <Modal.Body
        ref={scrollBody}
        onScroll={keepSelectionVisible}
        onFocusCapture={(event) => {
          const target = (event.target as HTMLElement).closest<HTMLElement>(
            "[data-booru-target]"
          );
          if (target?.dataset.booruTarget) select(target.dataset.booruTarget);
        }}
        onMouseDown={(event) => {
          const target = (event.target as HTMLElement).closest<HTMLElement>(
            "[data-booru-target]"
          );
          if (target?.dataset.booruTarget) choose(target.dataset.booruTarget);
        }}
      >
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
            {message(
              "inspection_keys",
              "← / → Browse posts · ↑ / ↓ Select media · E Download selected · Q Ignore selected media"
            )}
          </span>
          <BooruIconButton
            icon={faArrowRight}
            label="Next preview (Right arrow)"
            disabled={!onNext}
            onClick={onNext}
          />
        </nav>
        {detailsLoading && (
          <p role="status">
            <Spinner animation="border" size="sm" className="mr-2" />
            Checking full Pawchive post for shared links…
          </p>
        )}
        {detailsError && (
          <Alert variant="warning">
            {detailsError} The listed Pawchive media remains available.
          </Alert>
        )}
        {post.mediaNotice && <Alert variant="info">{post.mediaNotice}</Alert>}
        {mediaError && (
          <Alert variant="warning" role="alert">
            {mediaError}
          </Alert>
        )}
        {multiMedia && (
          <MediaGallery
            assets={assets}
            selectedIndex={selectedIndex}
            isIgnored={isMediaIgnored}
            onShow={showMedia}
          />
        )}
        {groupMedia && (
          <Button
            className={`minimal booru-group-toggle mb-2${selected === "media" ? " booru-target-selected" : ""}`}
            data-booru-target="media"
            aria-expanded={mediaExpanded}
            onClick={() => {
              setMediaOpen({ identity, open: !mediaExpanded });
              // Hidden media cannot stay selected; select the header instead.
              if (mediaExpanded) select("media");
            }}
          >
            <Icon
              icon={mediaExpanded ? faChevronDown : faChevronRight}
              fixedWidth
            />{" "}
            <FormattedMessage
              id="booru_browser.media_toggle"
              defaultMessage="{count, plural, one {# media file} other {# media files}}"
              values={{ count: assets.length }}
            />
          </Button>
        )}
        <Collapse in={mediaExpanded}>
          <div className="booru-inspection-media" aria-label="Post media">
            {assets.slice(0, visibleCount).map((asset, index) => {
              const sample = booruMediaURL(asset, "sample");
              const archive = isArchiveFormat(asset.format);
              const assetTarget = `asset:${index}`;
              const open =
                archive || assets.length === 1 || index === openIndex;
              const heading = (
                <>
                  {asset.sourceID === "shared-links" ? "Linked media" : "Media"}{" "}
                  {index + 1} of {assets.length}
                  {asset.format && ` · ${asset.format.toUpperCase()}`}
                  {multiMedia && isMediaIgnored(index) && " · Ignored"}
                </>
              );
              return (
                <section
                  key={asset.id}
                  className={`booru-inspection-asset${!archive && selected === assetTarget ? " booru-target-selected" : ""}${open ? "" : " booru-asset-collapsed"}${multiMedia && isMediaIgnored(index) ? " booru-media-ignored" : ""}`}
                  data-asset-index={index}
                  data-booru-target={archive ? undefined : assetTarget}
                  tabIndex={-1}
                  aria-label={`Media ${index + 1} of ${assets.length}`}
                  aria-expanded={archive ? undefined : open}
                >
                  {!open ? (
                    <Button
                      variant="minimal"
                      className="booru-asset-heading booru-asset-expand"
                      onClick={() => choose(assetTarget)}
                    >
                      <Icon icon={faChevronRight} fixedWidth /> {heading}
                    </Button>
                  ) : (
                    assets.length > 1 && (
                      <h6 className="booru-asset-heading">
                        {!archive && <Icon icon={faChevronDown} fixedWidth />}{" "}
                        {heading}
                      </h6>
                    )
                  )}
                  {open && (
                    <>
                      {asset.format === "folder" ? (
                        <div className="booru-shared-folder">
                          {post.previewURL ? (
                            <>
                              <RemoteImage
                                src={booruMediaURL(post, "preview") || ""}
                                className="booru-sample"
                                alt="Pawchive post cover (not a folder file preview)"
                              />
                              <p className="small text-muted">
                                Pawchive post cover — folder thumbnails are
                                available on MEGA.
                              </p>
                            </>
                          ) : (
                            <Alert variant="secondary">
                              MEGA folder — open the provider to browse
                              image/video thumbnails.
                            </Alert>
                          )}
                        </div>
                      ) : archive ? (
                        <BooruArchive
                          post={asset}
                          scrollBody={scrollBody}
                          instance={source.instance}
                          target={`archive:${index}`}
                          selected={selected}
                          onSelect={select}
                        />
                      ) : (
                        <MotionPreview post={asset}>
                          {sample ? (
                            <RemoteImage
                              key={sample}
                              className="booru-sample"
                              src={sample}
                              fallbackSrc={remoteURL(asset.directSampleURL)}
                              alt={`${source.name} #${post.id} media ${index + 1}`}
                            />
                          ) : (
                            <Alert variant="secondary">
                              {asset.format?.toLowerCase() === "swf"
                                ? "Flash animation · Download the original to play it in a Flash emulator."
                                : asset.sourceID === "shared-links"
                                  ? "This shared file has no preview. Use Download to Stash to inspect the original."
                                  : message(
                                      "no_preview",
                                      "No supported preview is available. Open the remote post to inspect it."
                                    )}
                            </Alert>
                          )}
                        </MotionPreview>
                      )}
                      {asset.sourceID === "shared-links" && (
                        <p className="small mb-1">
                          <Icon icon={faLink} />{" "}
                          {asset.title || "External shared media"}
                        </p>
                      )}
                      {asset.sourceID === "shared-links" &&
                        /^[a-f0-9]{48}$/.test(asset.mediaKey ?? "") && (
                          <ExternalLink
                            className="btn btn-outline-secondary btn-sm mb-2"
                            href={`booru-browser/shared-open?key=${asset.mediaKey}`}
                          >
                            <Icon icon={faLink} />{" "}
                            {asset.format === "folder"
                              ? "Browse thumbnails and download on MEGA"
                              : "Open original on Dropbox / MEGA"}
                          </ExternalLink>
                        )}
                      {asset.mediaNotice &&
                        asset.mediaNotice !== post.mediaNotice && (
                          <Alert variant="info">{asset.mediaNotice}</Alert>
                        )}
                      {multiMedia && (
                        <Button
                          variant="secondary"
                          size="sm"
                          className="booru-media-ignore mt-2"
                          aria-pressed={isMediaIgnored(index)}
                          onClick={() =>
                            writeMediaIgnored(index, !isMediaIgnored(index))
                          }
                        >
                          <Icon icon={faBan} />{" "}
                          {isMediaIgnored(index)
                            ? message("unignore_media", "Unignore this media")
                            : message("ignore_media", "Ignore this media (Q)")}
                        </Button>
                      )}
                      <div className="booru-file-details mt-2">
                        {!!asset.width && !!asset.height && (
                          <span>
                            {TextUtils.resolution(asset.width, asset.height)} (
                            {asset.width} × {asset.height})
                          </span>
                        )}
                        {!!asset.bytes && (
                          <span>
                            <FileSize size={asset.bytes} />
                          </span>
                        )}
                        {asset.format && (
                          <span>{asset.format.toUpperCase()}</span>
                        )}
                        {post.rating && (
                          <span
                            className={`booru-rating-label rating-${normalizedRating(post.rating, source)}`}
                          >
                            {normalizedRating(post.rating, source)}
                          </span>
                        )}
                      </div>
                    </>
                  )}
                  {/* Download controls stay mounted (hidden) when the file is
                      collapsed, so a started download keeps its progress and
                      a queued E press still reaches its button. */}
                  {!["zip", "cbz", "7z", "folder"].includes(
                    asset.format?.toLowerCase() ?? ""
                  ) && (
                    <div className={open ? undefined : "d-none"}>
                      <BooruDownload
                        key={`download:${asset.id}:${asset.mediaKey}`}
                        post={asset}
                        instance={source.instance}
                      />
                    </div>
                  )}
                  {/* The post's library evidence stays visible when its
                      first file is collapsed. */}
                  {(open || index === 0) && (
                    <div className="booru-hash">
                      {asset.md5 && (
                        <span>
                          MD5: <code>{asset.md5}</code>
                        </span>
                      )}
                      {index === 0 && <ReviewEvidence state={review} />}
                    </div>
                  )}
                </section>
              );
            })}
            {visibleCount < assets.length && (
              <div
                ref={loadMore}
                className="booru-media-load-more"
                role="status"
              >
                Loading more media ({Math.min(visibleCount, assets.length)} of{" "}
                {assets.length})…
              </div>
            )}
          </div>
        </Collapse>
        {blacklistError && (
          <Alert variant="danger" role="alert">
            {blacklistError}
          </Alert>
        )}
        {blacklistBusy && <span role="status">Updating tag blacklist…</span>}
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
                      aria-label={`Tag ${tag.canonicalName ?? tag.label ?? tag.name}`}
                    >
                      {tag.canonicalName ?? tag.label ?? tag.name}
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
                      <Dropdown.Item
                        role="menuitem"
                        disabled={blacklistBusy}
                        onClick={() => onBlacklistTag(tag.name)}
                      >
                        Add to blacklist
                      </Dropdown.Item>
                      <IdentityLink
                        source={source}
                        kind={tag.category}
                        target={tag.name}
                        label={tag.label}
                        identityID={tag.identityID}
                      />
                      <FollowAction
                        as="menu"
                        instance={source.instance}
                        engine={source.engine}
                        kind={tag.category}
                        target={tag.name}
                        identityID={tag.identityID}
                        siteName={source.name}
                      />
                    </Dropdown.Menu>
                  </Dropdown>
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
          aria-label={multiMedia ? "Ignore post" : "Ignore post (Q)"}
          title={
            multiMedia
              ? "Ignore post (Q ignores the selected media; the last one ignores the post)"
              : "Ignore post (Q)"
          }
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

// A strip of small thumbnails for a post with several media: another way to
// move through them. Not keyboard targets (↑ / ↓ keep walking the list).
function MediaGallery({
  assets,
  selectedIndex,
  isIgnored,
  onShow,
}: {
  assets: BooruPost[];
  selectedIndex?: number;
  isIgnored: (index: number) => boolean;
  onShow: (index: number) => void;
}) {
  const strip = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const container = strip.current;
    const thumb = container?.querySelector<HTMLElement>(
      `[data-gallery-index="${selectedIndex}"]`
    );
    if (!container || !thumb) return;
    // Scroll the strip sideways only: scrollIntoView would also scroll the
    // dialog back up to the strip and pull the selection away.
    const start = thumb.offsetLeft;
    const end = start + thumb.offsetWidth;
    if (start < container.scrollLeft) container.scrollLeft = start;
    else if (end > container.scrollLeft + container.clientWidth)
      container.scrollLeft = end - container.clientWidth;
  }, [selectedIndex]);
  return (
    <div
      ref={strip}
      className="booru-media-gallery mb-2"
      role="group"
      aria-label={`Post media gallery, ${assets.length} files`}
    >
      {assets.map((asset, index) => {
        const thumbnail =
          booruMediaURL(asset, "preview") ?? booruMediaURL(asset, "sample");
        const ignored = isIgnored(index);
        const active = index === selectedIndex;
        return (
          <button
            type="button"
            key={asset.id}
            data-gallery-index={index}
            className={`booru-media-thumb${active ? " active" : ""}${ignored ? " ignored" : ""}`}
            aria-current={active || undefined}
            aria-label={`Show media ${index + 1} of ${assets.length}${ignored ? " (ignored)" : ""}`}
            title={`Media ${index + 1}${asset.format ? ` · ${asset.format.toUpperCase()}` : ""}${ignored ? " · Ignored" : ""}`}
            onClick={() => onShow(index)}
          >
            {thumbnail ? (
              <img src={thumbnail} alt="" loading="lazy" decoding="async" />
            ) : (
              <span className="booru-media-thumb-placeholder">
                {asset.format?.toUpperCase() ?? "?"}
              </span>
            )}
            <span className="booru-media-thumb-index">{index + 1}</span>
            {ignored && (
              <span className="booru-media-thumb-ignored">
                <Icon icon={faBan} />
              </span>
            )}
          </button>
        );
      })}
    </div>
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
  onFailure,
}: {
  src: string;
  alt: string;
  className?: string;
  fallbackSrc?: string;
  onFailure?: () => void;
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
        if (!direct && fallbackSrc) {
          setDirect(true);
        } else {
          setFailed(true);
          onFailure?.();
        }
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
  hidePawchiveWithoutPreview,
  markSeenOnView,
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
    | "sort"
    | "rating"
    | "media"
    | "pageSize"
    | "favorites"
    | "hideAI"
    | "showBlacklisted"
  >;
  thumbnails: boolean;
  hidePawchiveWithoutPreview: boolean;
  markSeenOnView: boolean;
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
  const [failures, setFailures] = useState<{
    result: BooruResult;
    urls: Set<string>;
  }>();
  const isPawchive = source.engine === "pawchive";
  const hideMissingPreview = isPawchive && hidePawchiveWithoutPreview;
  const pawchiveThumbnail = useCallback(
    (post: BooruPost) =>
      [post, ...(post.attachments ?? [])].find((asset) => {
        const url = booruMediaURL(asset, "preview");
        return (
          url &&
          !(failures && failures.result === result && failures.urls.has(url))
        );
      }),
    [failures, result]
  );
  function recordThumbnailFailure(url: string) {
    if (!result) return;
    setFailures((previous) => {
      if (previous?.result !== result) return { result, urls: new Set([url]) };
      if (previous.urls.has(url)) return previous;
      return { result, urls: new Set([...previous.urls, url]) };
    });
  }
  const matchingPosts = useMemo(
    () =>
      result?.posts.filter(
        (post) =>
          (!filters.media || postMediaKind(post) === filters.media) &&
          (!filters.hideAI || !isAIGeneratedPost(post))
      ) ?? [],
    [result, filters.media, filters.hideAI]
  );
  const visiblePosts = useMemo(
    () =>
      hideMissingPreview
        ? matchingPosts.filter(pawchiveThumbnail)
        : matchingPosts,
    [hideMissingPreview, matchingPosts, pawchiveThumbnail]
  );
  const hiddenWithoutPreview = matchingPosts.length - visiblePosts.length;
  // "Mark followed posts seen once shown": posts on this page that a follow
  // check already saved count as seen in Following. Others are unaffected.
  const shownIDs = visiblePosts.map((post) => post.id).join("\n");
  useEffect(() => {
    if (!markSeenOnView || suspended || !shownIDs) return;
    const controller = new AbortController();
    void fetch(new URL("following/seen-batch", document.baseURI), {
      method: "POST",
      credentials: "same-origin",
      signal: controller.signal,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        instance: source.instance,
        postIDs: shownIDs.split("\n").slice(0, 500),
        seen: true,
      }),
    })
      .then(async (response) => {
        const value = (await response.json()) as { changed?: number };
        if (response.ok && value.changed) notifyFollowingChanged();
      })
      .catch(() => {});
    return () => controller.abort();
  }, [markSeenOnView, suspended, shownIDs, source.instance]);
  const signature = browserResultSignature(
    query,
    page,
    filters.sort,
    filters.rating,
    filters.media,
    filters.pageSize,
    favorites,
    filters.hideAI,
    filters.showBlacklisted
  );
  const [review, setReview] = useState<ReviewState[]>([]);
  const [retry, setRetry] = useState(0);
  const onReview = useCallback((value: ReviewState[]) => setReview(value), []);
  const priorIgnored = useRef<Set<string>>(new Set());
  useEffect(() => {
    const ignored = new Set(
      review.filter((item) => item.ignoredAt).map((item) => item.id)
    );
    if (
      !showIgnored &&
      [...ignored].some((id) => !priorIgnored.current.has(id))
    ) {
      // Saved Ignore actions free a displayed slot. Refill once per new mark.
      setRetry((current) => current + 1);
    }
    priorIgnored.current = ignored;
  }, [review, showIgnored]);
  const [error, setError] = useState("");
  const pawchiveSearchLink =
    source.engine === "pawchive" &&
    !favorites &&
    query.trim() &&
    !query.startsWith("creator:")
      ? remoteURL(
          `${source.url.replace(/\/$/, "")}/posts?q=${encodeURIComponent(query)}`
        )
      : undefined;
  const [loading, setLoading] = useState(true);
  // Results are not ready for scroll restoration until the provider finishes.
  // The initial empty posts list is only a loading placeholder.
  useEffect(
    () => onResult(source.id, signature, visiblePosts, !loading),
    [onResult, source.id, signature, visiblePosts, loading]
  );
  const [blacklistBusy, setBlacklistBusy] = useState(false);
  const [blacklistError, setBlacklistError] = useState("");
  const [blacklistNotice, setBlacklistNotice] = useState("");
  async function blacklistTag(tag: string) {
    if (blacklistBusy) return;
    setBlacklistBusy(true);
    setBlacklistError("");
    try {
      await fetch("booru-browser/settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "blacklist-add",
          source: { id: source.id },
          instance: source.instance,
          tag,
        }),
      }).then(readBooruResponse);
      setBlacklistNotice(`${tag} is blacklisted on ${source.name}.`);
      onClose();
      setRetry((value) => value + 1);
    } catch (cause: unknown) {
      setBlacklistError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBlacklistBusy(false);
    }
  }
  const controller = useRef<AbortController>();
  useEffect(() => {
    if (suspended) return;
    const request = new AbortController();
    controller.current = request;
    // Keep review controls mounted during a refill, preserving the Undo action.
    if (retry === 0) setResult(undefined);
    setError("");
    setLoading(true);

    const params = new URLSearchParams({
      source: source.id,
      instance: source.instance,
      q: query,
      page: String(page),
      limit: String(filters.pageSize ?? 24),
      fill: "1",
      ...(filters.media ? { media: filters.media } : {}),
      ...(filters.hideAI ? { ai: "hide" } : {}),
      ...(!showIgnored ? { ignored: "hide" } : {}),
      ...(filters.sort ? { sort: filters.sort } : {}),
      ...(filters.rating ? { rating: filters.rating } : {}),
      ...(filters.showBlacklisted ? { blacklist: "show" } : {}),
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
    filters.media,
    filters.hideAI,
    showIgnored,
    filters.showBlacklisted,
    suspended,
  ]);
  const post = result?.posts.find((item) => item.id === inspect);
  const [details, setDetails] = useState<{ key: string; post: BooruPost }>();
  const [detailsLoading, setDetailsLoading] = useState(false);
  // Posts ignored from the preview leave navigation at once, before the
  // review write returns, so a quick ← does not land back on them.
  const [ignoredHere, setIgnoredHere] = useState<ReadonlySet<string>>(
    () => new Set()
  );
  // Once the review state changes (the write's result arrived), it is the
  // source of truth again; a failed write lets the post come back.
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset on each review update.
  useEffect(() => setIgnoredHere(new Set()), [review]);
  const [detailsError, setDetailsError] = useState("");
  useEffect(() => {
    setDetails(undefined);
    setDetailsError("");
    if (source.engine !== "pawchive" || suspended || !post?.mediaKey) {
      setDetailsLoading(false);
      return;
    }
    const key = post.mediaKey;
    const postID = post.id;
    const controller = new AbortController();
    setDetailsLoading(true);
    void fetch(`booru-browser/post-details?${new URLSearchParams({ key })}`, {
      signal: controller.signal,
    })
      .then(readBooruResponse<{ post: BooruPost }>)
      .then((result) => {
        if (!controller.signal.aborted && result.post.id === postID)
          setDetails({ key, post: result.post });
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted)
          setDetailsError(
            cause instanceof Error ? cause.message : String(cause)
          );
      })
      .finally(() => {
        if (!controller.signal.aborted) setDetailsLoading(false);
      });
    return () => controller.abort();
  }, [post?.id, post?.mediaKey, source.engine, suspended]);
  const inspectedPost =
    post && details && details.key === post.mediaKey ? details.post : post;
  const inspectedReview = review.find((state) => state.id === post?.id);
  const visibleIDs = new Set(visiblePosts.map((item) => item.id));
  const navigation = matchingPosts.filter(
    (item) =>
      item.id === inspect ||
      (visibleIDs.has(item.id) &&
        (showIgnored ||
          (!ignoredHere.has(item.id) &&
            !review.find((state) => state.id === item.id)?.ignoredAt)))
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
          {pawchiveSearchLink && (
            <ExternalLink
              className="btn btn-link btn-sm"
              href={pawchiveSearchLink}
            >
              Open search on Pawchive
            </ExternalLink>
          )}
        </Alert>
      )}
      {blacklistNotice && (
        <Alert
          variant="success"
          role="status"
          dismissible
          onClose={() => setBlacklistNotice("")}
        >
          {blacklistNotice}
        </Alert>
      )}
      {result && (
        <>
          {result.searchNotice && (
            <Alert variant="info">{result.searchNotice}</Alert>
          )}
          {source.id === "rule34" && result.categoryStatus === "loading" && (
            <Alert variant="info">
              Rule34 tag categories are being installed automatically from a
              public catalog. Run the search again after the catalog is ready;
              no CSV upload is needed.
            </Alert>
          )}
          {result.posts.length === 0 && (
            <Alert variant="info">
              {message("empty", "No posts returned for this page and query.")}
            </Alert>
          )}
          {!!result.blacklistedCount && (
            <p className="small text-muted">
              Skipped {result.blacklistedCount} blacklisted{" "}
              {result.blacklistedCount === 1 ? "post" : "posts"} while loading
              these results.
            </p>
          )}
          {hiddenWithoutPreview > 0 && (
            <p className="small text-muted" role="status">
              Hidden without previews on this Pawchive page:{" "}
              {hiddenWithoutPreview}.{" "}
              <Button
                variant="link"
                className="p-0"
                size="sm"
                disabled={loading}
                onClick={() => setRetry((value) => value + 1)}
              >
                Refresh previews
              </Button>
            </p>
          )}
          {(filters.media || filters.hideAI) && (
            <p className="small text-muted">
              Showing {matchingPosts.length} posts after the selected filters.
              Hidden posts are replaced from subsequent provider pages when
              available; AI-generated detection uses explicit tags only.
            </p>
          )}
          <BooruReview
            displayMode={displayMode}
            zoom={zoom}
            source={source}
            posts={visiblePosts}
            reviewPosts={result.posts}
            showIgnored={showIgnored}
            onStates={onReview}
            preview={({ ignore, busy }) =>
              post && (
                <PostInspection
                  post={inspectedPost ?? post}
                  detailsLoading={detailsLoading}
                  detailsError={detailsError}
                  source={source}
                  onTag={onTag}
                  onBlacklistTag={(name) => void blacklistTag(name)}
                  blacklistBusy={blacklistBusy}
                  blacklistError={blacklistError}
                  onHide={onClose}
                  review={inspectedReview}
                  reviewBusy={busy || !inspectedReview}
                  onIgnore={() => {
                    // Ignoring hides the post at once: move to the next one.
                    ignore(post.id);
                    setIgnoredHere((current) => new Set(current).add(post.id));
                    if (
                      previewIndex >= 0 &&
                      previewIndex < navigation.length - 1
                    )
                      onInspect(navigation[previewIndex + 1].id);
                    else if (previewIndex > 0)
                      onInspect(navigation[previewIndex - 1].id);
                    else onClose();
                  }}
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
            {(item, controls, isSelected, selection) => {
              const thumbnail = isPawchive ? pawchiveThumbnail(item) : item;
              const previewURL =
                thumbnail && booruMediaURL(thumbnail, "preview");
              return (
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
                    ) : previewURL ? (
                      <RemoteImage
                        key={previewURL}
                        src={previewURL}
                        fallbackSrc={remoteURL(thumbnail?.directPreviewURL)}
                        alt={`${source.name} #${item.id}`}
                        onFailure={
                          isPawchive
                            ? () => recordThumbnailFailure(previewURL)
                            : undefined
                        }
                      />
                    ) : (
                      <span>
                        {sharedLinkCount(item) > 0 ? (
                          <>
                            <Icon icon={faLink} /> Shared media — inspect post
                          </>
                        ) : (
                          message("missing_thumbnail", "Preview unavailable")
                        )}
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
                    {sharedLinkCount(item) > 0 && (
                      <span
                        className="small text-muted"
                        title="External Dropbox/MEGA links detected; inspect post"
                      >
                        <Icon icon={faLink} /> {sharedLinkCount(item)}
                      </span>
                    )}
                    {item.format?.toLowerCase() === "swf" && (
                      <span className="small text-muted">SWF</span>
                    )}
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
                    {!["zip", "cbz", "7z"].includes(
                      item.format?.toLowerCase() ?? ""
                    ) && (
                      <BooruDownload
                        post={item}
                        instance={source.instance}
                        compact
                      />
                    )}
                  </div>
                  {controls}
                </article>
              );
            }}
          </BooruReview>
          <div className="pagination-footer-container booru-pagination">
            <div className="pagination-footer">
              <Pagination
                currentPage={page}
                unknownTotal
                hasMore={result.hasMore}
                maxPage={MAX_BOORU_PAGE}
                pagePopupPlacement="top"
                onChangePage={(target) => {
                  if (
                    Number.isInteger(target) &&
                    target >= 1 &&
                    target <= MAX_BOORU_PAGE &&
                    target !== page
                  ) {
                    onPage(target);
                  }
                }}
              />
            </div>
          </div>
          {page === MAX_BOORU_PAGE && (
            <Alert variant="info" className="mt-2">
              {message(
                "page_limit",
                `Numbered browsing ends at page ${MAX_BOORU_PAGE}. Narrow the query to continue.`
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
  const markSeenOnView = !!useMarkSeenOnView();
  const [hidePawchiveWithoutPreview, setHidePawchiveWithoutPreview] = useState(
    remembered?.hidePawchiveWithoutPreview ?? false
  );
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
    hidePawchiveWithoutPreview,
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
      hidePawchiveWithoutPreview,
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
    hidePawchiveWithoutPreview,
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
            active.favorites,
            active.hideAI,
            active.showBlacklisted
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
      ["shimmie", "pixiv", "pawchive", "kemono", "twitter"].includes(
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
      <BooruFilenameMigration showSettings={showSettings} />
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
          <Form.Check
            type="switch"
            id="booru-hide-pawchive-without-preview"
            label="Pawchive: hide posts without previews"
            checked={hidePawchiveWithoutPreview}
            onChange={(event) =>
              setHidePawchiveWithoutPreview(event.currentTarget.checked)
            }
          />
          <p className="small text-muted">
            Filter the current Pawchive page when a post has no thumbnail or its
            thumbnails fail. This preference is saved in this browser.
          </p>
          <MarkSeenOnViewSwitch id="booru-mark-seen-on-view" />
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
                // Creator identities are source-specific; ordinary text
                // searches should survive switching to Pawchive, now that
                // its API supports server-side post search.
                const nextQuery =
                  selected[0] !== source.id && query.startsWith("creator:")
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
                label="Media type"
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
                      pages: {},
                      inspect: undefined,
                    }),
                  })
                }
              />
              <BooruIconButton
                icon={faBan}
                label="Hide blacklisted posts"
                crossedOut={!active.showBlacklisted}
                aria-pressed={!active.showBlacklisted}
                onClick={() =>
                  history.push({
                    search: browserSearchURL({
                      ...active,
                      showBlacklisted: !active.showBlacklisted,
                      pages: {},
                      inspect: undefined,
                    }),
                  })
                }
              />
              <BooruIconButton
                icon={faRobot}
                label="Hide AI-generated posts"
                crossedOut={!!active.hideAI}
                aria-pressed={!!active.hideAI}
                onClick={() =>
                  history.push({
                    search: browserSearchURL({
                      ...active,
                      hideAI: !active.hideAI,
                      pages: {},
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
                        pages: {},
                        inspect: undefined,
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
                    [
                      "shimmie",
                      "pixiv",
                      "pawchive",
                      "kemono",
                      "twitter",
                    ].includes(selectedSource?.engine ?? ""),
                },
                {
                  value: "popular",
                  label: "Popular",
                  disabled:
                    !!active.favorites ||
                    [
                      "shimmie",
                      "pixiv",
                      "pawchive",
                      "kemono",
                      "twitter",
                    ].includes(selectedSource?.engine ?? ""),
                },
                {
                  value: "score_desc",
                  label: "Score: high to low",
                  disabled:
                    !!active.favorites ||
                    [
                      "shimmie",
                      "pixiv",
                      "pawchive",
                      "kemono",
                      "twitter",
                    ].includes(selectedSource?.engine ?? ""),
                },
                {
                  value: "score_asc",
                  label: "Score: low to high",
                  disabled:
                    !!active.favorites ||
                    [
                      "shimmie",
                      "pixiv",
                      "pawchive",
                      "kemono",
                      "twitter",
                    ].includes(selectedSource?.engine ?? ""),
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
              maxPageSize={250}
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
                    key={`${id}:${active.favorites}:${active.query}:${active.pages[id]}:${loadAttempt}:${active.sort}:${active.rating}:${active.pageSize}:${active.showBlacklisted}`}
                    source={source}
                    markSeenOnView={markSeenOnView}
                    suspended={showSettings}
                    favorites={!!active.favorites}
                    onTag={searchTag}
                    displayMode={displayMode}
                    zoom={zoom}
                    filters={active}
                    thumbnails={thumbnails}
                    hidePawchiveWithoutPreview={hidePawchiveWithoutPreview}
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
    | "sort"
    | "rating"
    | "media"
    | "pageSize"
    | "favorites"
    | "hideAI"
    | "showBlacklisted"
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
        filters.favorites,
        filters.hideAI,
        filters.showBlacklisted
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
