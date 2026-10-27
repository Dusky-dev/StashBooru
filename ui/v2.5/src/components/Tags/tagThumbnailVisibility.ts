/**
 * Image visibility is a display preference only. Keep the stored image intact.
 * Copyrights share the Tag detail component but are a separate native category.
 */
export function shouldHideTagThumbnail(
  perTagHidden: boolean,
  hideGeneralTagThumbnails: boolean,
  isCopyright: boolean
): boolean {
  return perTagHidden || (hideGeneralTagThumbnails && !isCopyright);
}
