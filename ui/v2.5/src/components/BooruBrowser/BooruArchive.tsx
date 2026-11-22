import {
  faChevronDown,
  faChevronRight,
} from "@fortawesome/free-solid-svg-icons";
import { type RefObject, useEffect, useRef, useState } from "react";
import { Alert, Button, Collapse } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import { Icon } from "src/components/Shared/Icon";
import { BooruDownload } from "./BooruDownload";
import { type BooruPost, readBooruResponse } from "./browser";

interface ArchiveFile {
  index: number;
  name: string;
  bytes: number;
  format: string;
}

interface ArchiveListing {
  files: ArchiveFile[];
  skipped: number;
}

const archiveURL = (key: string, file?: number) =>
  `booru-browser/archive?` +
  new URLSearchParams({
    key,
    ...(file === undefined ? {} : { file: String(file) }),
  });

// Archive bytes are fetched and validated on the authenticated Stash server,
// never extracted in the browser. Mount only three archive previews per batch.
// The archive header and each extracted file are keyboard selection targets
// (data-booru-target). Collapsing keeps the listing mounted, so it is not
// fetched again.
export function BooruArchive({
  post,
  scrollBody,
  instance,
  target,
  selected,
  onSelect,
}: {
  post: BooruPost;
  scrollBody: RefObject<HTMLDivElement | null>;
  instance: string;
  target: string;
  selected?: string;
  onSelect: (target: string) => void;
}) {
  const [open, setOpen] = useState(true);
  const [listing, setListing] = useState<ArchiveListing>();
  const [error, setError] = useState("");
  const [visibleCount, setVisibleCount] = useState(3);
  const [failed, setFailed] = useState<number[]>([]);
  const sentinel = useRef<HTMLDivElement>(null);
  const key = post.mediaKey ?? "";

  useEffect(() => {
    const controller = new AbortController();
    setListing(undefined);
    setError("");
    setVisibleCount(3);
    setFailed([]);
    setOpen(true);
    if (!/^[a-f0-9]{48}$/.test(key)) {
      setError("Archive preview unavailable. Refresh the search.");
      return () => controller.abort();
    }
    void fetch(archiveURL(key), { signal: controller.signal })
      .then(readBooruResponse<ArchiveListing>)
      .then((value) => {
        if (!controller.signal.aborted) setListing(value);
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted)
          setError(cause instanceof Error ? cause.message : String(cause));
      });
    return () => controller.abort();
  }, [key]);

  useEffect(() => {
    if (!listing || visibleCount >= listing.files.length) return;
    const root = scrollBody.current;
    const target = sentinel.current;
    if (!root || !target) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setVisibleCount((count) => Math.min(listing.files.length, count + 3));
        }
      },
      { root, rootMargin: "0px" }
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [listing, visibleCount, scrollBody]);

  return (
    <div className="booru-archive-media" aria-label="Archive media">
      <Button
        className={`minimal booru-group-toggle${selected === target ? " booru-target-selected" : ""}`}
        data-booru-target={target}
        aria-expanded={open}
        onClick={() => {
          const next = !open;
          setOpen(next);
          // A hidden file cannot stay selected; select the header instead.
          if (!next && selected?.startsWith(`${target}:`)) onSelect(target);
        }}
      >
        <Icon icon={open ? faChevronDown : faChevronRight} fixedWidth />{" "}
        <FormattedMessage
          id="booru_browser.archive_toggle"
          defaultMessage="{format} archive · {count, plural, one {# media file} other {# media files}}"
          values={{
            format: post.format?.toUpperCase() ?? "ZIP",
            count: listing?.files.length ?? 0,
          }}
        />
      </Button>
      <Collapse in={open}>
        <div>
          <p className="small text-muted">
            {post.format?.toUpperCase() ?? "Archive"} archive · Preview and save
            individual extracted media files
          </p>
          {!listing && !error && <p role="status">Reading archive contents…</p>}
          {error && <Alert variant="warning">{error}</Alert>}
          {listing && (
            <>
              <p className="small">
                {listing.files.length} media files in archive
                {listing.skipped > 0 &&
                  ` · ${listing.skipped} unsupported or oversized entries skipped`}
              </p>
              {listing.files.length === 0 && (
                <Alert variant="secondary">
                  This archive contains no supported previewable images or
                  videos.
                </Alert>
              )}
              {listing.files.slice(0, visibleCount).map((file) => {
                const src = archiveURL(key, file.index);
                const broken = failed.includes(file.index);
                const video = ["mp4", "webm"].includes(file.format);
                const fileTarget = `${target}:file:${file.index}`;
                return (
                  <section
                    key={file.index}
                    className={`booru-archive-file${selected === fileTarget ? " booru-target-selected" : ""}`}
                    data-booru-target={fileTarget}
                    tabIndex={-1}
                  >
                    <div className="booru-archive-filename">
                      {file.name} · {file.format.toUpperCase()} ·{" "}
                      {(file.bytes / 1048576).toFixed(2)} MiB
                    </div>
                    <BooruDownload
                      post={post}
                      instance={instance}
                      archiveFile={file.index}
                    />
                    {broken ? (
                      <Alert variant="secondary">
                        Media cannot be decoded or exceeds the preview
                        constraints.
                      </Alert>
                    ) : video ? (
                      <video
                        className="booru-sample"
                        src={src}
                        controls
                        preload="none"
                        aria-label={file.name}
                        onError={() =>
                          setFailed((prev) => [...prev, file.index])
                        }
                      />
                    ) : (
                      <img
                        className="booru-sample"
                        src={src}
                        alt={file.name}
                        loading="lazy"
                        onError={() =>
                          setFailed((prev) => [...prev, file.index])
                        }
                      />
                    )}
                  </section>
                );
              })}
              {visibleCount < listing.files.length && (
                <div
                  ref={sentinel}
                  className="booru-media-load-more"
                  role="status"
                >
                  Loading more archive media ({visibleCount} of{" "}
                  {listing.files.length})…
                </div>
              )}
            </>
          )}
        </div>
      </Collapse>
    </div>
  );
}
