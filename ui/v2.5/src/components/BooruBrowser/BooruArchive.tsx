import { type RefObject, useEffect, useRef, useState } from "react";
import { Alert } from "react-bootstrap";
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
export function BooruArchive({
  post,
  scrollBody,
}: {
  post: BooruPost;
  scrollBody: RefObject<HTMLDivElement | null>;
}) {
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
      { root, rootMargin: "240px 0px" }
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [listing, visibleCount, scrollBody]);

  return (
    <div className="booru-archive-media" aria-label="Archive media">
      <p className="small text-muted">
        ZIP/CBZ archive · Previewing supported media without saving extracted
        files
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
              This archive contains no supported previewable images or videos.
            </Alert>
          )}
          {listing.files.slice(0, visibleCount).map((file) => {
            const src = archiveURL(key, file.index);
            const broken = failed.includes(file.index);
            const video = ["mp4", "webm"].includes(file.format);
            return (
              <section key={file.index} className="booru-archive-file">
                <div className="booru-archive-filename">
                  {file.name} · {file.format.toUpperCase()} ·{" "}
                  {(file.bytes / 1048576).toFixed(2)} MiB
                </div>
                {broken ? (
                  <Alert variant="secondary">
                    Media cannot be decoded or exceeds the preview constraints.
                  </Alert>
                ) : video ? (
                  <video
                    className="booru-sample"
                    src={src}
                    controls
                    preload="none"
                    aria-label={file.name}
                    onError={() => setFailed((prev) => [...prev, file.index])}
                  />
                ) : (
                  <img
                    className="booru-sample"
                    src={src}
                    alt={file.name}
                    loading="lazy"
                    onError={() => setFailed((prev) => [...prev, file.index])}
                  />
                )}
              </section>
            );
          })}
          {visibleCount < listing.files.length && (
            <div ref={sentinel} className="booru-media-load-more" role="status">
              Loading more archive media ({visibleCount} of{" "}
              {listing.files.length})…
            </div>
          )}
        </>
      )}
    </div>
  );
}
