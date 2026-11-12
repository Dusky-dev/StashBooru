import { useEffect, useState } from "react";
import { faFilm, faImage, faLink } from "@fortawesome/free-solid-svg-icons";
import { Icon } from "./Icon";

type ImportedMedia = {
  kind: "image" | "scene";
  id: string;
};

export function MediaUploadThumbnail({
  file,
  name,
  result,
  size = 76,
}: {
  file?: File;
  name: string;
  result?: ImportedMedia;
  size?: number;
}) {
  const [localURL, setLocalURL] = useState("");
  const [localFailed, setLocalFailed] = useState(false);
  const [importedFailed, setImportedFailed] = useState(false);
  const isVideo =
    result?.kind === "scene" ||
    (!!file &&
      (file.type.startsWith("video/") ||
        /\.(mp4|m4v|webm|mkv|mov|avi|wmv|mpg|mpeg|flv|ts)$/i.test(
          file.name
        )));
  const isImage =
    result?.kind === "image" ||
    (!!file &&
      (file.type.startsWith("image/") ||
        /\.(jpe?g|png|gif|webp|avif|jxl|bmp|tiff?|heic|heif)$/i.test(
          file.name
        )));

  useEffect(() => {
    setLocalFailed(false);
    if (!file) {
      setLocalURL("");
      return;
    }
    // Browser-native JPEG, PNG, GIF, WebP and AVIF render before the upload
    // finishes. Revoke the URL when a queued file is removed or dismissed.
    const url = URL.createObjectURL(file);
    setLocalURL(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  useEffect(() => {
    setImportedFailed(false);
  }, [result?.id]);

  // Once Stash has imported a URL or a browser-unsupported format (e.g. JXL
  // or HEIC), prefer Stash's generated thumbnail instead of an image icon.
  // Use the document's base URL to respect deployments under a subpath.
  const importedThumbnail =
    result?.kind === "image" && /^\d+$/.test(result.id)
      ? new URL(`image/${result.id}/thumbnail`, document.baseURI).href
      : "";
  const source = isImage
    ? !localFailed && localURL
      ? localURL
      : !importedFailed
        ? importedThumbnail
        : ""
    : "";
  const frameStyle = {
    width: size,
    height: size,
    objectFit: "cover" as const,
    borderRadius: 4,
    maxWidth: "100%",
  };

  if (source) {
    return (
      <img
        src={source}
        alt={`Preview of ${name}`}
        title={name}
        style={frameStyle}
        onError={() => {
          if (source === localURL) setLocalFailed(true);
          else setImportedFailed(true);
        }}
      />
    );
  }

  if (localURL && !localFailed && isVideo) {
    return (
      <video
        src={localURL}
        muted
        playsInline
        preload="metadata"
        title={name}
        aria-label={`Preview of ${name}`}
        style={frameStyle}
        onLoadedMetadata={(event) => {
          const player = event.currentTarget;
          if (Number.isFinite(player.duration) && player.duration > 0.2) {
            player.currentTime = 0.1;
          }
        }}
        onError={() => setLocalFailed(true)}
      />
    );
  }

  // Remote media cannot be previewed safely before the SSRF-protected import;
  // never load an unvalidated URL in the browser just for a thumbnail.
  const hint = file
    ? "Preview unavailable in this browser"
    : result
      ? "Open imported media to preview"
      : "Preview after import";
  return (
    <span
      className="d-inline-flex flex-column justify-content-center align-items-center text-muted border rounded text-center"
      style={{ width: size, height: size, fontSize: size <= 80 ? 9 : 13 }}
      title={name}
      role="img"
      aria-label={`${name}: ${hint}`}
    >
      <Icon icon={isVideo ? faFilm : isImage ? faImage : faLink} />
      <span className="mt-1">{hint}</span>
    </span>
  );
}
