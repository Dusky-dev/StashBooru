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
}

interface ImageHeadingOptions {
  hideImageFilenames?: boolean;
  showImageAssociationTitles?: boolean;
}

function namesForHeading(entries: readonly NamedAssociation[] | null | undefined) {
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

/** Human-readable display label only; never renames or modifies the source. */
export function imageAssociationHeading(image: ImageHeadingMetadata): string {
  const copyright =
    image.primary_copyright?.name?.trim() ||
    namesForHeading(image.copyrights);
  const characters = namesForHeading(image.performers);
  const artists = namesForHeading(
    image.artists?.length ? image.artists : image.studio ? [image.studio] : []
  );
  return (
    [copyright, characters, artists].filter(Boolean).join(" · ") ||
    (image.id != null ? `Image #${image.id}` : "Image")
  );
}

/** Keep authored titles; only replace the filename-generated fallback. */
export function imageDisplayHeading(
  image: ImageHeadingMetadata,
  filenameHeading: string,
  options?: ImageHeadingOptions | null
): string | null {
  if (image.title) return image.title;
  if (options?.showImageAssociationTitles) {
    return imageAssociationHeading(image);
  }
  if (options?.hideImageFilenames) return null;
  return filenameHeading;
}
