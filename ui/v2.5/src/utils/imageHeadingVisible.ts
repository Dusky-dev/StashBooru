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
