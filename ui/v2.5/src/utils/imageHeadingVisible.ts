import { parseImageFilenameLayout } from "./mediaFilenameLayout.ts";

/**
 * Image headings use the filename when no title was supplied by the user.
 * Keep explicit titles, even when the filename display preference is enabled.
 */
export function imageHeadingVisible(
  image: { title?: string | null },
  hideImageFilenames = false
): boolean {
  return !hideImageFilenames || Boolean(image.title);
}

interface NamedAssociation {
  name?: string | null;
}

export interface ImageHeadingMetadata {
  id?: string | number | null;
  title?: string | null;
  primary_copyright?: NamedAssociation | null;
  copyrights?: readonly NamedAssociation[] | null;
  performers?: readonly NamedAssociation[] | null;
  artists?: readonly NamedAssociation[] | null;
  studio?: NamedAssociation | null;
  visual_files?: readonly { path: string }[] | null;
}

interface ImageHeadingOptions {
  hideImageFilenames?: boolean;
  showImageAssociationTitles?: boolean;
  showFormattedImageFilenameFallback?: boolean;
  imageFilenameLayout?: string;
}

function namesForHeading(
  entries: readonly NamedAssociation[] | null | undefined
) {
  const names = Array.from(
    new Set(
      (entries ?? [])
        .map((entry) => entry?.name?.trim())
        .filter((name): name is string => Boolean(name))
    )
  );
  const shown = names.slice(0, 2).join(", ");
  return names.length > 2 ? `${shown} +${names.length - 2}` : shown;
}

function associationHeading(image: ImageHeadingMetadata): string {
  const copyright =
    image.primary_copyright?.name?.trim() || namesForHeading(image.copyrights);
  const characters = namesForHeading(image.performers);
  const artists = namesForHeading(
    image.artists?.length ? image.artists : image.studio ? [image.studio] : []
  );
  return [copyright, characters, artists].filter(Boolean).join(" · ");
}

/** Human-readable display label only; never renames or modifies the source. */
export function imageAssociationHeading(image: ImageHeadingMetadata): string {
  return (
    associationHeading(image) ||
    (image.id != null ? `Image #${image.id}` : "Image")
  );
}

/** Make a filename readable without changing the original file or its title. */
export function formattedImageFilenameHeading(filename: string): string {
  const base = filename.trim().replace(/^.*[\\/]/, "");
  const stem = base.replace(
    /\.(?:apng|avif|avifs|bmp|gif|heic|heif|jpe?g|jxl|png|tiff?|webp|mp4|m4v|mov|mkv|webm)$/i,
    ""
  );
  if (!stem || stem.toLowerCase() === "no file name") return "";

  // Remove a trailing full-file hash when another descriptive name remains.
  // A filename consisting entirely of a hash retains a shortened identifier.
  const name = /^[a-f\d]{32,64}$/i.test(stem)
    ? `${stem.slice(0, 12)}…`
    : stem.replace(/[-_. ]+[a-f\d]{32,64}$/i, "");
  const readable = name
    .replace(/_+/g, " ")
    .replace(/-{2,}/g, " ")
    .replace(/([A-Za-z])-(?=[A-Za-z])/g, "$1 ")
    .replace(/\s+/g, " ")
    .trim();
  return readable.replace(/^\p{Ll}/u, (initial) => initial.toUpperCase());
}

/** Keep authored titles; only replace the filename-generated fallback. */
export function imageDisplayHeading(
  image: ImageHeadingMetadata,
  filenameHeading: string,
  options?: ImageHeadingOptions | null
): string | null {
  if (image.title) return image.title;
  if (options?.showImageAssociationTitles) {
    if (
      options.showFormattedImageFilenameFallback &&
      options.imageFilenameLayout
    ) {
      // A complete path suffix must follow the configured layout. Filename-only
      // guessing is never used to assign a missing Copyright/Character/Artist.
      const parsed = parseImageFilenameLayout(
        image.visual_files?.[0]?.path ?? filenameHeading,
        options.imageFilenameLayout
      );
      if (parsed) {
        const copyright =
          image.primary_copyright?.name?.trim() ||
          namesForHeading(image.copyrights) ||
          parsed.copyright;
        const characters =
          namesForHeading(image.performers) || parsed.character;
        const artists =
          namesForHeading(
            image.artists?.length
              ? image.artists
              : image.studio
                ? [image.studio]
                : []
          ) || parsed.artist;
        const complete = [copyright, characters, artists]
          .filter(Boolean)
          .join(" · ");
        if (complete) return complete;
      }
    }
    const labels = associationHeading(image);
    if (labels) return labels;
    if (options.showFormattedImageFilenameFallback) {
      const filename = formattedImageFilenameHeading(filenameHeading);
      if (filename) return filename;
    }
    return imageAssociationHeading(image);
  }
  if (options?.hideImageFilenames) return null;
  return filenameHeading;
}
