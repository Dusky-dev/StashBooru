export type PreviewMediaType = "image" | "video";

export interface PreviewSource {
  src?: string | null;
  mediaType: PreviewMediaType;
}

export interface SelectedPreviewSource {
  src: string;
  mediaType: PreviewMediaType;
}

export function getScenePreviewSources(
  paths: {
    screenshot?: string | null;
    webp?: string | null;
    preview?: string | null;
  },
  playback?: string | null
): PreviewSource[] {
  if (playback === "image")
    return [{ src: paths.screenshot, mediaType: "image" }];
  return [
    {
      src: playback === "animation" ? paths.webp : paths.preview,
      mediaType: playback === "animation" ? "image" : "video",
    },
    { src: paths.screenshot, mediaType: "image" },
  ];
}

export function getWallDimensions(
  file?: { width?: number | null; height?: number | null },
  defaults = { width: 1, height: 1 }
) {
  const valid = (value: number | null | undefined, fallback: number) =>
    typeof value === "number" && Number.isFinite(value) && value > 0
      ? value
      : fallback;
  return {
    width: valid(file?.width, defaults.width),
    height: valid(file?.height, defaults.height),
  };
}

export function getFirstValidPreviewSource(
  srcSet: readonly PreviewSource[],
  invalidSrcSet: string[]
): SelectedPreviewSource {
  const validSrcSet = srcSet.filter((s) => s.src);

  if (!validSrcSet.length) {
    return { src: "", mediaType: "image" };
  }

  const selected =
    validSrcSet.find(({ src }) => !invalidSrcSet.includes(src!)) ??
    ([...validSrcSet].pop() as PreviewSource);

  return {
    src: selected.src!,
    mediaType: selected.mediaType,
  };
}
