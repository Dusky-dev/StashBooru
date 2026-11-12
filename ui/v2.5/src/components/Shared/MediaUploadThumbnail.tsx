import { useEffect, useState } from "react";
import { faFilm, faImage, faLink } from "@fortawesome/free-solid-svg-icons";
import { Icon } from "./Icon";

const previewSize = { width: "54px", height: "54px" };

export function MediaUploadThumbnail({
  file,
  name,
}: {
  file?: File;
  name: string;
}) {
  const [objectURL, setObjectURL] = useState("");
  const [failed, setFailed] = useState(false);
  const isVideo = !!file && (
    file.type.startsWith("video/") ||
    /\.(mp4|m4v|webm|mkv|mov|avi|wmv|mpg|mpeg|flv|ts)$/i.test(file.name)
  );
  const isImage = !!file && (
    file.type.startsWith("image/") ||
    /\.(jpe?g|png|gif|webp|avif|jxl|bmp|tiff?|heic|heif)$/i.test(file.name)
  );

  useEffect(() => {
    setFailed(false);
    if (!file) {
      setObjectURL("");
      return;
    }
    const preview = URL.createObjectURL(file);
    setObjectURL(preview);
    return () => URL.revokeObjectURL(preview);
  }, [file]);

  const frameStyle = {
    ...previewSize,
    objectFit: "cover" as const,
    borderRadius: "4px",
  };

  if (objectURL && !failed && isImage) {
    return (
      <img
        src={objectURL}
        alt=""
        aria-hidden="true"
        style={frameStyle}
        onError={() => setFailed(true)}
      />
    );
  }
  if (objectURL && !failed && isVideo) {
    return (
      <video
        src={objectURL}
        muted
        playsInline
        preload="metadata"
        aria-hidden="true"
        style={frameStyle}
        onLoadedMetadata={(event) => {
          const player = event.currentTarget;
          if (Number.isFinite(player.duration) && player.duration > 0.2) {
            player.currentTime = 0.1;
          }
        }}
        onError={() => setFailed(true)}
      />
    );
  }
  // A remote URL is never fetched just to make a preview. The server validates
  // and downloads it on import. Unsupported local formats get a safe icon.
  return (
    <span
      className="d-inline-flex justify-content-center align-items-center"
      style={previewSize}
      title={name}
      aria-hidden="true"
    >
      <Icon icon={file ? (isVideo ? faFilm : faImage) : faLink} />
    </span>
  );
}
